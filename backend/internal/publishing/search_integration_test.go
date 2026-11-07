//go:build integration

package publishing

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"testing"
)

func TestPublicSearchRankingPaginationAndPrivacy(t *testing.T) {
	f := newImageFixture(t)
	category := createdLabel(t, f.call("POST", "/api/v1/admin/categories", `{"name":"Engineering","slug":"engineering"}`, true))
	tag := createdLabel(t, f.call("POST", "/api/v1/admin/tags", `{"name":"Go","slug":"go"}`, true))
	create := func(filename, title, summary, body, language string, categoryID *string, tags []string, public, publish bool) *Article {
		t.Helper()
		job := f.upload(t, filename, []byte(body), nil)
		if job.Status != "succeeded" {
			t.Fatal(job)
		}
		base := "/api/v1/admin/articles/" + *job.ArticleID
		a := imageArticle(t, f.call("GET", base, "", true))
		a = imageArticle(t, f.call("PATCH", base+"/draft", testJSON(map[string]any{"expected_version": a.Version, "title": title, "summary": summary, "written_at": "2026-11-07", "language": language, "category_id": categoryID, "tag_ids": tags}), true))
		if publish {
			a = imageArticle(t, f.call("POST", base+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
		}
		if public {
			a = imageArticle(t, f.call("PATCH", base, fmt.Sprintf(`{"expected_version":%d,"visibility":"public"}`, a.Version), true))
		}
		return a
	}
	title := create("title.md", "Go notes", "", "ordinary text", "en", &category.ID, []string{}, true, true)
	metadata := create("metadata.md", "Metadata notes", "Go systems", "ordinary text", "en", &category.ID, []string{}, true, true)
	body := create("body.md", "Body notes", "", strings.Repeat("Introduction ", 150)+"Go practical details", "en", nil, []string{}, true, true)
	tagged := create("tagged.md", "Tagged notes", "", "ordinary text", "en", nil, []string{tag.ID}, true, true)
	formatted := create("format.md", "Formatting", "", "G**o** with [useful link](https://example.com/hiddenurlneedle)", "en", nil, []string{}, true, true)
	chinese := create("chinese.md", "数据库笔记", "", "事务和索引", "zh", nil, []string{}, true, true)
	create("golang.md", "Golang gophers", "", "unrelated text", "en", nil, []string{}, true, true)
	private := create("private.md", "Private Go diary", "", "private_only_needle", "en", nil, []string{}, false, true)
	draft := create("draft.md", "Draft Go piece", "", "draft_only_needle", "en", nil, []string{}, true, false)
	code := create("code.md", "Code example", "", "```go\ncode_only_needle()\n```", "en", nil, []string{}, true, true)
	create("html.md", "Markup study", "", "<script>hidden_script_needle</script>", "en", nil, []string{}, true, true)
	historical := create("past.md", "Past Go story", "", "past_only_needle", "en", nil, []string{}, true, true)
	historicalBase := "/api/v1/admin/articles/" + historical.ID
	job := f.upload(t, "past-next.md", []byte("current unrelated text"), historical)
	if job.Status != "succeeded" {
		t.Fatal(job)
	}
	historical = imageArticle(t, f.call("GET", historicalBase, "", true))
	historical = imageArticle(t, f.call("PATCH", historicalBase+"/draft", fmt.Sprintf(`{"expected_version":%d,"title":"Fresh story"}`, historical.Version), true))
	imageArticle(t, f.call("POST", historicalBase+"/publish", fmt.Sprintf(`{"expected_version":%d}`, historical.Version), true))
	if _, err := f.db.ExecContext(f.ctx, `INSERT INTO comments(id,article_id,display_name,body) VALUES(gen_random_uuid(),$1,'Guest','comment_only_needle')`, title.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(f.ctx, `UPDATE articles SET published_at='2026-11-01T08:00:00Z' WHERE visibility='public' AND published_revision_id IS NOT NULL`); err != nil {
		t.Fatal(err)
	}
	type resultPage struct {
		Items []publicArticleSummary `json:"items"`
		Next  *string                `json:"next_cursor"`
	}
	search := func(query string) resultPage {
		t.Helper()
		w := f.call("GET", "/api/v1/articles/search?"+query, "", true)
		if w.Code != 200 {
			t.Fatalf("search %q: %d %s", query, w.Code, w.Body)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("search response cached")
		}
		if strings.Contains(w.Body.String(), private.ID) || strings.Contains(w.Body.String(), draft.ID) {
			t.Fatal("private/draft identity leaked")
		}
		var page resultPage
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		return page
	}
	metaIDs := []string{metadata.ID, tagged.ID}
	sort.Sort(sort.Reverse(sort.StringSlice(metaIDs)))
	bodyIDs := []string{body.ID, formatted.ID}
	sort.Sort(sort.Reverse(sort.StringSlice(bodyIDs)))
	expected := append([]string{title.ID}, metaIDs...)
	expected = append(expected, bodyIDs...)
	complete := search("q=GO")
	if len(complete.Items) != len(expected) {
		t.Fatalf("Go matches: got %+v", complete.Items)
	}
	for i, item := range complete.Items {
		if item.ID != expected[i] {
			t.Fatal("title/metadata/body ranking or tie ordering incorrect")
		}
		if item.ID == body.ID && !strings.Contains(item.Summary, "Go practical") {
			t.Fatal("body snippet missed query")
		}
	}
	seen := []string{}
	cursor := ""
	firstCursor := ""
	for {
		query := "q=go&limit=1"
		if cursor != "" {
			query += "&cursor=" + url.QueryEscape(cursor)
		}
		page := search(query)
		for _, item := range page.Items {
			seen = append(seen, item.ID)
		}
		if page.Next == nil {
			break
		}
		cursor = *page.Next
		if firstCursor == "" {
			firstCursor = cursor
		}
	}
	if len(seen) != len(expected) {
		t.Fatal("ranked cursor lost results")
	}
	for i, id := range seen {
		if id != expected[i] {
			t.Fatal("rank cursor repeated/reordered tied results")
		}
	}
	if got := search("q=" + url.QueryEscape("  GO  ") + "&limit=2&cursor=" + url.QueryEscape(firstCursor)); len(got.Items) != 2 || got.Items[0].ID != expected[1] {
		t.Fatal("normalized query/changed page size cursor failed")
	}
	for _, query := range []string{"q=other&cursor=" + url.QueryEscape(firstCursor), "q=go&language=zh&cursor=" + url.QueryEscape(firstCursor), "q=go&category=engineering&cursor=" + url.QueryEscape(firstCursor)} {
		if f.call("GET", "/api/v1/articles/search?"+query, "", false).Code != 400 {
			t.Fatal("cursor reused with different search/filter")
		}
	}
	for _, test := range []struct {
		query string
		want  []string
	}{
		{"q=" + url.QueryEscape("数据库"), []string{chinese.ID}},
		{"q=" + url.QueryEscape("数据库") + "&language=en", nil},
		{"q=go&category=engineering", []string{title.ID, metadata.ID}},
		{"q=go&tag=go", []string{tagged.ID}},
		{"q=Go+systems", []string{metadata.ID}},
		{"q=CODE_ONLY_NEEDLE", []string{code.ID}},
		{"q=Engineering", nil}, {"q=private_only_needle", nil}, {"q=draft_only_needle", nil},
		{"q=past_only_needle", nil}, {"q=comment_only_needle", nil}, {"q=hidden_script_needle", nil},
		{"q=hiddenurlneedle", nil}, {"q=" + url.QueryEscape(".*"), nil}, {"q=" + url.QueryEscape("' OR 1=1 --"), nil},
	} {
		got := search(test.query)
		if len(got.Items) != len(test.want) {
			t.Fatalf("query %q returned %d, want %d", test.query, len(got.Items), len(test.want))
		}
		for i, item := range got.Items {
			if item.ID != test.want[i] {
				t.Fatalf("query %q wrong result", test.query)
			}
		}
	}
	for _, query := range []string{"", "q=", "q=+++", "q=" + strings.Repeat("a", 201), "q=%ff", "q=go&q=other", "q=go&limit=0", "q=go&limit=101", "q=go&cursor=bad", "q=go&unknown=1", "q=go&language=fr"} {
		if f.call("GET", "/api/v1/articles/search?"+query, "", false).Code != 400 {
			t.Fatalf("invalid query accepted: %s", query)
		}
	}
	// Renames and publications are visible on the next search, without an index.
	createdLabel(t, f.call("PATCH", "/api/v1/admin/tags/"+tag.ID, `{"expected_version":1,"name":"Tooling"}`, true))
	if got := search("q=Tooling"); len(got.Items) != 1 || got.Items[0].ID != tagged.ID {
		t.Fatal("tag rename not immediately searchable")
	}
	if got := search("q=go"); len(got.Items) != len(expected)-1 {
		t.Fatal("old tag name remained indexed")
	}
	titleBase := "/api/v1/admin/articles/" + title.ID
	title = imageArticle(t, f.call("GET", titleBase, "", true))
	job = f.upload(t, "title-next.md", []byte("new_live_needle"), title)
	if job.Status != "succeeded" {
		t.Fatal(job)
	}
	title = imageArticle(t, f.call("GET", titleBase, "", true))
	title = imageArticle(t, f.call("PATCH", titleBase+"/draft", fmt.Sprintf(`{"expected_version":%d,"title":"Replacement story"}`, title.Version), true))
	if len(search("q=new_live_needle").Items) != 0 {
		t.Fatal("replacement draft became searchable")
	}
	title = imageArticle(t, f.call("POST", titleBase+"/publish", fmt.Sprintf(`{"expected_version":%d}`, title.Version), true))
	if len(search("q=new_live_needle").Items) != 1 {
		t.Fatal("published replacement absent from search")
	}
	title = imageArticle(t, f.call("PATCH", titleBase, fmt.Sprintf(`{"expected_version":%d,"visibility":"private"}`, title.Version), true))
	if len(search("q=new_live_needle").Items) != 0 {
		t.Fatal("privacy change left search content visible")
	}
	taggedBase := "/api/v1/admin/articles/" + tagged.ID
	tagged = imageArticle(t, f.call("GET", taggedBase, "", true))
	imageArticle(t, f.call("POST", taggedBase+"/unpublish", fmt.Sprintf(`{"expected_version":%d}`, tagged.Version), true))
	if len(search("q=Tooling").Items) != 0 {
		t.Fatal("unpublished content still searchable")
	}
	f.s.searchSlots <- struct{}{}
	f.s.searchSlots <- struct{}{}
	busy := f.call("GET", "/api/v1/articles/search?q=go", "", false)
	<-f.s.searchSlots
	<-f.s.searchSlots
	if busy.Code != 503 || busy.Header().Get("Retry-After") != "1" {
		t.Fatal("expensive search concurrency is unbounded")
	}
}

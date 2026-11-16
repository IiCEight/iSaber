//go:build integration

package publishing

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestPinLifecyclePrivacyAndContentDates(t *testing.T) {
	f := newImageFixture(t)
	job := f.upload(t, "pin.md", []byte("---\nwritten_at: 2024-01-01\nlanguage: en\n---\n# Pin lifecycle\nOriginal content"), nil)
	base := "/api/v1/admin/articles/" + *job.ArticleID
	a := imageArticle(t, f.call("GET", base, "", true))
	change := func(fields string) *Article {
		t.Helper()
		return imageArticle(t, f.call("PATCH", base, fmt.Sprintf(`{"expected_version":%d,%s}`, a.Version, fields), true))
	}
	if w := f.call("PATCH", base, fmt.Sprintf(`{"expected_version":%d,"pinned":true}`, a.Version), true); w.Code != 422 || !strings.Contains(w.Body.String(), "pin_requires_publication") {
		t.Fatalf("draft pin accepted: %d %s", w.Code, w.Body)
	}
	a = imageArticle(t, f.call("POST", base+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	if a.Visibility != "private" || a.Pinned {
		t.Fatal("unexpected publication defaults")
	}
	before := *a
	a = change(`"pinned":true`)
	if !a.Pinned || a.Version != before.Version+1 || !a.PublishedAt.Equal(*before.PublishedAt) || !a.UpdatedAt.Equal(*before.UpdatedAt) || *a.Published.WrittenAt != *before.Published.WrittenAt {
		t.Fatal("private pin failed or changed content dates")
	}
	if w := f.call("PATCH", base, fmt.Sprintf(`{"expected_version":%d,"pinned":false}`, before.Version), true); w.Code != 409 {
		t.Fatal("stale pin update accepted")
	}
	for _, raw := range []string{`null`, `"true"`, `1`, `{}`} {
		if w := f.call("PATCH", base, fmt.Sprintf(`{"expected_version":%d,"pinned":%s}`, a.Version, raw), true); w.Code != 422 {
			t.Fatalf("invalid pin accepted: %s %d", raw, w.Code)
		}
	}
	if w := f.call("PATCH", base, fmt.Sprintf(`{"expected_version":%d,"pinned":false}`, a.Version), false); w.Code != 401 {
		t.Fatal("anonymous pin control allowed")
	}
	for _, path := range []string{"/api/v1/articles?limit=4", "/api/v1/articles?page=1", "/api/v1/articles/search?q=lifecycle&page=1", "/api/v1/articles/search?q=lifecycle"} {
		for _, authenticated := range []bool{false, true} {
			w := f.call("GET", path, "", authenticated)
			if w.Code != 200 || strings.Contains(w.Body.String(), a.ID) || strings.Contains(w.Body.String(), "Pin lifecycle") {
				t.Fatalf("private pin leaked: %s %d %s", path, w.Code, w.Body)
			}
		}
	}
	if w := f.call("GET", "/api/v1/articles/"+*a.Slug, "", true); w.Code != 404 {
		t.Fatal("private pinned detail leaked")
	}
	for _, path := range []string{"articles?limit=4", "articles?page=1", "articles/search?q=lifecycle&page=1", "articles/search?q=lifecycle", "articles/" + *a.Slug} {
		w := f.call("GET", "/api/v1/admin/reading/"+path, "", true)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"pinned":true`) || !strings.Contains(w.Body.String(), `"visibility":"private"`) || w.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("private owner pin missing: %s %d %s", path, w.Code, w.Body)
		}
	}
	for _, path := range []string{"/api/v1/admin/articles?page=1", "/api/v1/admin/articles"} {
		w := f.call("GET", path, "", true)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"pinned":true`) {
			t.Fatal("admin list pin missing")
		}
	}
	a = change(`"visibility":"public"`)
	if !a.Pinned {
		t.Fatal("visibility change removed pin")
	}
	w := f.call("GET", "/api/v1/articles/"+*a.Slug, "", false)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"pinned":true`) {
		t.Fatal("public pin detail missing")
	}
	before = *a
	a = change(`"pinned":false`)
	if a.Pinned || !a.UpdatedAt.Equal(*before.UpdatedAt) || !a.PublishedAt.Equal(*before.PublishedAt) {
		t.Fatal("unpin changed content dates")
	}
	a = change(`"pinned":true`)
	firstRevision := a.Published.ID
	f.upload(t, "replacement.md", []byte("---\nwritten_at: 2025-01-01\n---\n# New draft\nReplacement content"), a)
	a = imageArticle(t, f.call("GET", base, "", true))
	if !a.Pinned || a.Published.ID != firstRevision {
		t.Fatal("import changed pin/publication")
	}
	a = imageArticle(t, f.call("POST", base+"/revisions/"+firstRevision+"/restore", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	if !a.Pinned {
		t.Fatal("revision restore removed pin")
	}
	a = imageArticle(t, f.call("POST", base+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	if !a.Pinned {
		t.Fatal("publishing changes removed pin")
	}
	a = imageArticle(t, f.call("POST", base+"/unpublish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	if a.Pinned || a.Published != nil {
		t.Fatal("unpublish retained pin")
	}
	a = imageArticle(t, f.call("POST", base+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	if a.Pinned {
		t.Fatal("republishing restored a removed pin")
	}
}

func TestPinnedBrowsePaginationFiltersAndSearchOrder(t *testing.T) {
	f := newImageFixture(t)
	label := createdLabel(t, f.call("POST", "/api/v1/admin/categories", `{"name":"Technology","slug":"technology"}`, true))
	tag := createdLabel(t, f.call("POST", "/api/v1/admin/tags", `{"name":"Go","slug":"go"}`, true))
	corpus := []publicArticleSummary{}
	for i := 0; i < 10; i++ {
		id, revision := fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1), fmt.Sprintf("10000000-0000-4000-8000-%012d", i+1)
		visibility := "public"
		if i == 8 {
			visibility = "private"
		}
		item := publicArticleSummary{ID: id, Slug: fmt.Sprintf("pin-entry-%d", i), Title: fmt.Sprintf("Entry %d", i), Language: "en", WrittenAt: fmt.Sprintf("2026-01-%02d", i+1), PublishedAt: time.Date(2026, 1, i+1, 0, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 2, 10-i, 0, 0, 0, 0, time.UTC), Pinned: i == 0 || i == 2 || i == 4 || i == 8, Visibility: visibility}
		if i == 2 {
			item.WrittenAt, item.PublishedAt, item.UpdatedAt = corpus[0].WrittenAt, corpus[0].PublishedAt, corpus[0].UpdatedAt
		}
		if i%2 == 1 || i == 4 {
			item.Language = "zh"
			item.Category = &label.Label
			item.Tags = []Label{tag.Label}
		}
		if i == 1 {
			item.Title = "Needle title match"
		}
		if _, err := f.db.ExecContext(f.ctx, `INSERT INTO articles(id,slug,visibility,published_at,updated_at) VALUES($1,$2,$3,$4,$5)`, id, item.Slug, visibility, item.PublishedAt, item.UpdatedAt); err != nil {
			t.Fatal(err)
		}
		var category *string
		if item.Category != nil {
			category = &label.ID
		}
		if _, err := f.db.ExecContext(f.ctx, `INSERT INTO revisions(id,article_id,title,language,written_at,category_id,markdown) VALUES($1,$2,$3,$4,$5,$6,'needle body match')`, revision, id, item.Title, item.Language, item.WrittenAt, category); err != nil {
			t.Fatal(err)
		}
		if len(item.Tags) > 0 {
			if _, err := f.db.ExecContext(f.ctx, `INSERT INTO revision_tags(revision_id,tag_id) VALUES($1,$2)`, revision, tag.ID); err != nil {
				t.Fatal(err)
			}
		}
		pointer := "published_revision_id"
		if i == 9 {
			pointer = "draft_revision_id"
		}
		if _, err := f.db.ExecContext(f.ctx, `UPDATE articles SET `+pointer+`=$1,pinned=$2 WHERE id=$3`, revision, item.Pinned, id); err != nil {
			t.Fatal(err)
		}
		if i != 9 {
			corpus = append(corpus, item)
		}
	}
	read := func(path string, owner bool) numberedArticlePage {
		t.Helper()
		w := f.call("GET", path, "", owner)
		var page numberedArticlePage
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil {
			t.Fatalf("browse: %s %d %s", path, w.Code, w.Body)
		}
		return page
	}
	for _, owner := range []bool{false, true} {
		root := "/api/v1/"
		if owner {
			root += "admin/reading/"
		}
		for _, filter := range []string{"", "&language=zh", "&category=technology", "&tag=go"} {
			for _, order := range []string{"written", "written_oldest", "updated", "updated_oldest", "newest", "oldest"} {
				expected := []publicArticleSummary{}
				for _, item := range corpus {
					if !owner && item.Visibility == "private" {
						continue
					}
					if filter != "" && item.Language != "zh" {
						continue
					}
					expected = append(expected, item)
				}
				sort.Slice(expected, func(i, j int) bool {
					a, b := expected[i], expected[j]
					if a.Pinned != b.Pinned {
						return a.Pinned
					}
					av, bv := a.WrittenAt, b.WrittenAt
					if strings.HasPrefix(order, "updated") {
						av, bv = a.UpdatedAt.Format(time.RFC3339), b.UpdatedAt.Format(time.RFC3339)
					} else if order == "newest" || order == "oldest" {
						av, bv = a.PublishedAt.Format(time.RFC3339), b.PublishedAt.Format(time.RFC3339)
					}
					if av == bv {
						av, bv = a.ID, b.ID
					}
					if strings.HasSuffix(order, "oldest") {
						return av < bv
					}
					return av > bv
				})
				found := []publicArticleSummary{}
				for page := 1; ; page++ {
					got := read(fmt.Sprintf("%sarticles?page=%d&limit=2&sort=%s%s", root, page, order, filter), owner)
					if got.Total != int64(len(expected)) {
						t.Fatal("pin changed total")
					}
					found = append(found, got.Items...)
					if page == got.TotalPages {
						break
					}
				}
				if len(found) != len(expected) {
					t.Fatal("numbered pages omitted/duplicated articles")
				}
				for i := range expected {
					if found[i].ID != expected[i].ID || found[i].Pinned != expected[i].Pinned {
						t.Fatalf("pin/date/filter order: owner=%t sort=%s filter=%s position=%d", owner, order, filter, i)
					}
				}
			}
		}
		for _, filter := range []string{"", "&language=zh", "&category=technology", "&tag=go"} {
			found := map[string]bool{}
			after := ""
			unpinned := false
			for {
				got := read(root+"articles?limit=2"+filter+after, owner)
				for _, item := range got.Items {
					if found[item.ID] || unpinned && item.Pinned {
						t.Fatal("cursor duplicated or misplaced pin")
					}
					found[item.ID] = true
					unpinned = unpinned || !item.Pinned
				}
				if got.NextCursor == nil {
					break
				}
				after = "&cursor=" + url.QueryEscape(*got.NextCursor)
			}
			want := 0
			for _, item := range corpus {
				if (owner || item.Visibility != "private") && (filter == "" || item.Language == "zh") {
					want++
				}
			}
			if len(found) != want {
				t.Fatal("cursor omitted articles across pin boundary")
			}
		}
		home := read(root+"articles?limit=4", owner)
		if len(home.Items) != 4 || !home.Items[0].Pinned {
			t.Fatal("home cap/order incorrect")
		}
		dateSearch := read(root+"articles/search?q=needle&limit=10&page=1&sort=written", owner)
		expectedFirst := corpus[7].ID
		if owner {
			expectedFirst = corpus[8].ID
		}
		if len(dateSearch.Items) == 0 || dateSearch.Items[0].ID != expectedFirst {
			t.Fatal("pin overrode selected search date")
		}
		for _, tail := range []string{"&page=1&sort=relevance", ""} {
			search := read(root+"articles/search?q=needle&limit=10"+tail, owner)
			if len(search.Items) == 0 || search.Items[0].Title != "Needle title match" || search.Items[0].Pinned {
				t.Fatal("pin overrode relevance")
			}
			pins := 0
			for _, item := range search.Items {
				if item.Pinned {
					pins++
				}
			}
			want := 3
			if owner {
				want = 4
			}
			if pins != want {
				t.Fatal("search omitted pin flags or leaked private pin")
			}
		}
	}
}

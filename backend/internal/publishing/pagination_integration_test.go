//go:build integration

package publishing

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestNumberedPublicPagesCountsRankingAndPrivacy(t *testing.T) {
	f := newImageFixture(t)
	category := createdLabel(t, f.call("POST", "/api/v1/admin/categories", `{"name":"Notes","slug":"notes"}`, true))
	tag := createdLabel(t, f.call("POST", "/api/v1/admin/tags", `{"name":"Systems","slug":"systems"}`, true))
	publicIDs := []string{}
	for i := 0; i < 39; i++ {
		id, revision := uuid.NewString(), uuid.NewString()
		visibility, published := "public", i < 37
		if i >= 35 && i < 37 {
			visibility = "private"
		}
		language := "en"
		if i%2 != 0 {
			language = "zh"
		}
		title, summary, body := fmt.Sprintf("Article %02d", i), "Summary", "Body"
		switch {
		case i < 10:
			title = "Go " + title
		case i < 20:
			summary = "Go summary"
		default:
			body = "Go body"
		}
		if _, err := f.db.ExecContext(f.ctx, `INSERT INTO articles(id,slug,visibility,published_at,updated_at)
 VALUES($1,$2,$3,'2026-11-01T08:00:00Z','2026-11-01T08:00:00Z')`, id, fmt.Sprintf("article-%d", i), visibility); err != nil {
			t.Fatal(err)
		}
		var categoryID *string
		if i%2 == 0 {
			categoryID = &category.ID
		}
		if _, err := f.db.ExecContext(f.ctx, `INSERT INTO revisions(id,article_id,title,summary,language,written_at,category_id,markdown)
 VALUES($1,$2,$3,$4,$5,'2026-11-01',$6,$7)`, revision, id, title, summary, language, categoryID, body); err != nil {
			t.Fatal(err)
		}
		if i%3 == 0 {
			if _, err := f.db.ExecContext(f.ctx, `INSERT INTO revision_tags(revision_id,tag_id) VALUES($1,$2)`, revision, tag.ID); err != nil {
				t.Fatal(err)
			}
		}
		pointer := "draft_revision_id"
		if published {
			pointer = "published_revision_id"
		}
		if _, err := f.db.ExecContext(f.ctx, `UPDATE articles SET `+pointer+`=$1 WHERE id=$2`, revision, id); err != nil {
			t.Fatal(err)
		}
		if i < 35 {
			publicIDs = append(publicIDs, id)
		}
	}
	if _, err := f.db.ExecContext(f.ctx, `UPDATE articles SET published_at=CASE WHEN id=$1 THEN '2026-10-01T08:00:00Z'::timestamptz ELSE '2026-11-07T08:00:00Z'::timestamptz END WHERE id IN ($1,$2)`, publicIDs[0], publicIDs[34]); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(f.ctx, `UPDATE revisions SET written_at=CASE WHEN article_id=$1 THEN '2001-01-01'::date ELSE '2026-11-08'::date END WHERE article_id IN ($1,$2)`, publicIDs[1], publicIDs[33]); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(f.ctx, `UPDATE articles SET updated_at=CASE WHEN id=$1 THEN '2020-01-01T08:00:00Z'::timestamptz ELSE '2026-12-01T08:00:00Z'::timestamptz END WHERE id IN ($1,$2)`, publicIDs[2], publicIDs[32]); err != nil {
		t.Fatal(err)
	}
	draftID := uuid.NewString()
	if _, err := f.db.ExecContext(f.ctx, `INSERT INTO revisions(id,article_id,title,language,written_at,updated_at,markdown) VALUES($1,$2,'Future draft','en','2099-01-01','2099-01-01T08:00:00Z','Go draft only')`, draftID, publicIDs[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(f.ctx, `UPDATE articles SET draft_revision_id=$1 WHERE id=$2`, draftID, publicIDs[0]); err != nil {
		t.Fatal(err)
	}
	read := func(path string) numberedArticlePage {
		t.Helper()
		w := f.call("GET", path, "", true)
		var page numberedArticlePage
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil {
			t.Fatalf("page %s: %d %s", path, w.Code, w.Body)
		}
		return page
	}
	ids := func(items []publicArticleSummary) []string {
		result := []string{}
		for _, item := range items {
			result = append(result, item.ID)
		}
		return result
	}
	for _, path := range []string{"/api/v1/articles?", "/api/v1/articles/search?q=go&"} {
		first, second := read(path+"page=1"), read(path+"page=2")
		if first.Total != 35 || first.TotalPages != 2 || first.PageSize != 20 || first.Page != 1 || len(first.Items) != 20 || second.Page != 2 || second.Total != 35 || len(second.Items) != 15 {
			t.Fatalf("35 article pagination: first=%+v second=%+v", first, second)
		}
		whole := read(path + "page=1&limit=100")
		newest := read(path + "page=1&limit=100&sort=newest")
		oldest := read(path + "page=1&limit=100&sort=oldest")
		written := read(path + "page=1&limit=100&sort=written")
		writtenOldest := read(path + "page=1&limit=100&sort=written_oldest")
		updated := read(path + "page=1&limit=100&sort=updated")
		updatedOldest := read(path + "page=1&limit=100&sort=updated_oldest")
		if !reflect.DeepEqual(ids(whole.Items), ids(written.Items)) || whole.Items[0].ID != publicIDs[33] || writtenOldest.Items[0].ID != publicIDs[1] {
			t.Fatal("written date is not the default, or unpublished draft dates affected sorting")
		}
		if updated.Items[0].ID != publicIDs[32] || updatedOldest.Items[0].ID != publicIDs[2] {
			t.Fatal("last updated sorting used the wrong date or unpublished draft metadata")
		}
		if newest.Total != 35 || oldest.Total != 35 || newest.Items[0].ID != publicIDs[34] || oldest.Items[0].ID != publicIDs[0] {
			t.Fatal("publication-date order or public totals are incorrect")
		}
		for _, pair := range [][2]numberedArticlePage{{newest, oldest}, {written, writtenOldest}, {updated, updatedOldest}} {
			for i := range pair[0].Items {
				if pair[0].Items[i].ID != pair[1].Items[len(pair[1].Items)-1-i].ID {
					t.Fatal("date direction or ID tie ordering is inconsistent")
				}
			}
		}
		for mode, expected := range map[string]numberedArticlePage{"oldest": oldest, "written_oldest": writtenOldest, "updated": updated} {
			paged := []string{}
			for number := 1; number <= 4; number++ {
				part := read(path + fmt.Sprintf("page=%d&limit=10&sort=%s", number, mode))
				paged = append(paged, ids(part.Items)...)
			}
			if !reflect.DeepEqual(paged, ids(expected.Items)) {
				t.Fatalf("%s pages lost or repeated results", mode)
			}
		}
		combined := append(ids(first.Items), ids(second.Items)...)
		if !reflect.DeepEqual(combined, ids(whole.Items)) {
			t.Fatal("numbered pages lost, repeated, or reordered results")
		}
		seen := map[string]bool{}
		for _, id := range combined {
			if seen[id] {
				t.Fatal("duplicate article across pages")
			}
			seen[id] = true
		}
		for _, id := range publicIDs {
			if !seen[id] {
				t.Fatal("public article missing")
			}
		}
		last := read(path + "page=2147483647")
		if last.Page != 2 || !reflect.DeepEqual(ids(last.Items), ids(second.Items)) {
			t.Fatal("out-of-range page did not resolve to the last available page")
		}
		filtered := read(path + "page=2&limit=5&language=zh")
		if filtered.Total != 17 || filtered.TotalPages != 4 || len(filtered.Items) != 5 {
			t.Fatal("language filters were not applied to totals and page content")
		}
		filtered = read(path + "page=1&category=notes&tag=systems")
		if filtered.Total != 6 || len(filtered.Items) != 6 {
			t.Fatal("combined filters were not applied to public totals")
		}
		empty := read(path + "page=3&category=unpublished-label")
		if empty.Total != 0 || empty.Page != 1 || empty.TotalPages != 1 || len(empty.Items) != 0 {
			t.Fatal("empty filtered collection has incorrect pagination")
		}
		for _, query := range []string{"page=0", "page=-1", "page=", "page=01", "page=1.5", "page=NaN", "page=2147483648", "page=2&page=1", "page=1&cursor=anything", "page=1&limit=0", "page=1&limit=101", "page=1&sort=unknown", "page=1&sort=", "page=1&sort=oldest&sort=newest", "sort=oldest"} {
			if w := f.call("GET", path+query, "", false); w.Code != 400 {
				t.Fatalf("invalid page accepted: %s %d %s", query, w.Code, w.Body)
			}
		}
	}
	if w := f.call("GET", "/api/v1/articles?page=1&sort=relevance", "", false); w.Code != 400 {
		t.Fatal("relevance sort accepted without a search query")
	}
	if !reflect.DeepEqual(ids(read("/api/v1/articles/search?q=go&page=1&limit=100&sort=newest").Items), ids(read("/api/v1/articles?page=1&limit=100&sort=newest").Items)) {
		t.Fatal("search date sorting still prioritized relevance over publication time")
	}
	if !reflect.DeepEqual(ids(read("/api/v1/articles/search?q=go&page=1&limit=100&sort=written").Items), ids(read("/api/v1/articles/search?q=go&page=1&limit=100").Items)) {
		t.Fatal("numbered search did not default to written date")
	}
	// Existing cursor pagination still works and preserves the exact ranking.
	w := f.call("GET", "/api/v1/articles/search?q=go&limit=100", "", false)
	var legacy struct {
		Items []publicArticleSummary `json:"items"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &legacy) != nil || !reflect.DeepEqual(ids(legacy.Items), ids(read("/api/v1/articles/search?q=go&page=1&limit=100&sort=relevance").Items)) {
		t.Fatal("numbered search ranking differs from existing cursor search")
	}
	if _, err := f.db.ExecContext(f.ctx, `UPDATE articles SET visibility='private' WHERE id=$1`, publicIDs[0]); err != nil {
		t.Fatal(err)
	}
	if read("/api/v1/articles?page=2").Total != 34 || read("/api/v1/articles/search?q=go&page=2").Total != 34 {
		t.Fatal("public counts were stale after a privacy change")
	}
}

//go:build integration

package publishing

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNumberedOwnerPagesFiltersAndCursorCompatibility(t *testing.T) {
	f := newImageFixture(t)
	for i := 0; i < 23; i++ {
		id, revision := uuid.NewString(), uuid.NewString()
		visibility := "public"
		if i%2 == 0 {
			visibility = "private"
		}
		if _, err := f.db.ExecContext(f.ctx, `INSERT INTO articles(id,slug,visibility,modified_at,published_at,updated_at) VALUES($1,$2,$3,'2026-11-14T08:00:00Z','2026-11-14T08:00:00Z','2026-11-14T08:00:00Z')`, id, fmt.Sprintf("owner-%d", i), visibility); err != nil {
			t.Fatal(err)
		}
		if _, err := f.db.ExecContext(f.ctx, `INSERT INTO revisions(id,article_id,title,language,written_at,markdown) VALUES($1,$2,$3,'en','2026-11-14','Private body')`, revision, id, fmt.Sprintf("Owner article %02d", i)); err != nil {
			t.Fatal(err)
		}
		pointer := "published_revision_id"
		if i >= 20 {
			pointer = "draft_revision_id"
		}
		if _, err := f.db.ExecContext(f.ctx, `UPDATE articles SET `+pointer+`=$1 WHERE id=$2`, revision, id); err != nil {
			t.Fatal(err)
		}
	}
	type ownerPage struct {
		Items      []ArticleSummary `json:"items"`
		Next       *string          `json:"next_cursor"`
		Page       int              `json:"page"`
		Size       int              `json:"page_size"`
		Total      int              `json:"total"`
		TotalPages int              `json:"total_pages"`
	}
	read := func(query string) ownerPage {
		t.Helper()
		w := f.call("GET", "/api/v1/admin/articles?"+query, "", true)
		var page ownerPage
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil {
			t.Fatalf("owner page %q: %d %s", query, w.Code, w.Body)
		}
		if w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "Private body") {
			t.Fatal("Owner listing must be uncached and omit article content")
		}
		return page
	}
	whole := read("limit=100")
	for _, size := range []int{5, 10, 20} {
		joined := []ArticleSummary{}
		pages := (23 + size - 1) / size
		for number := 1; number <= pages; number++ {
			page := read(fmt.Sprintf("page=%d&limit=%d", number, size))
			if page.Total != 23 || page.TotalPages != pages || page.Page != number || page.Size != size || page.Next != nil {
				t.Fatalf("Incorrect owner pagination: %+v", page)
			}
			joined = append(joined, page.Items...)
		}
		if !reflect.DeepEqual(joined, whole.Items) {
			t.Fatal("Numbered owner pages differ from cursor summaries or ordering")
		}
	}
	first := read("limit=5")
	second := read("limit=5&cursor=" + url.QueryEscape(*first.Next))
	if !reflect.DeepEqual(second.Items, whole.Items[5:10]) || second.Page != 0 {
		t.Fatal("Legacy cursor pagination changed")
	}
	for query, total := range map[string]int{
		"status=draft": 3, "status=published": 20, "visibility=private": 12,
		"visibility=public": 11, "status=draft&visibility=private": 2,
		"q=OWNER%20ARTICLE%200": 10, "q=no-such-title": 0,
	} {
		page := read("page=9999&limit=5&" + query)
		if page.Total != total || page.Page != max(1, (total+4)/5) || (total == 0 && len(page.Items) != 0) {
			t.Fatalf("Filtered totals or page clamping: %s %+v", query, page)
		}
	}
	for _, query := range []string{"page=0", "page=-1", "page=1.5", "page=2147483648", "page=1&page=2", "page=1&cursor=x", "page=1&limit=0", "page=1&limit=101", "page=1&limit=5&limit=10", "page=1&sort=written", "page=1&status=unknown", "page=1&visibility=unknown", "page=1&q=" + strings.Repeat("x", 201)} {
		if w := f.call("GET", "/api/v1/admin/articles?"+query, "", true); w.Code != 400 {
			t.Fatalf("Invalid owner page accepted: %q %d %s", query, w.Code, w.Body)
		}
	}
	if w := f.call("GET", "/api/v1/admin/articles?page=1", "", false); w.Code != 401 {
		t.Fatal("Owner pages must require authentication")
	}
}

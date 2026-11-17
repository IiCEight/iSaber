//go:build integration

package publishing

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestCategoryPagesPinsPrivacyAndDescriptions(t *testing.T) {
	f := newImageFixture(t)
	create := func(slug string) adminLabel {
		t.Helper()
		w := f.call("POST", "/api/v1/admin/categories", fmt.Sprintf(`{"name":%q,"slug":%q,"description_en":"Systems and ideas","description_zh":"技术与探索"}`, slug, slug), true)
		if w.Code != 201 {
			t.Fatalf("create: %d %s", w.Code, w.Body)
		}
		var v adminLabel
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	category := create("technology")
	hidden := create("private-only")
	other := create("other")
	seed := func(slug, categoryID, visibility, written string, global, local bool) string {
		t.Helper()
		id, revision := uuid.NewString(), uuid.NewString()
		if _, err := f.db.ExecContext(f.ctx, `INSERT INTO articles(id,slug,visibility,published_at,updated_at) VALUES($1,$2,$3,'2026-01-01','2026-01-02')`, id, slug, visibility); err != nil {
			t.Fatal(err)
		}
		if _, err := f.db.ExecContext(f.ctx, `INSERT INTO revisions(id,article_id,title,language,written_at,markdown,category_id,updated_at) VALUES($1,$2,$3,'en',$4,'systems body',$5,'2026-01-02')`, revision, id, slug, written, categoryID); err != nil {
			t.Fatal(err)
		}
		if _, err := f.db.ExecContext(f.ctx, `UPDATE articles SET published_revision_id=$2,pinned=$3 WHERE id=$1`, id, revision, global); err != nil {
			t.Fatal(err)
		}
		if _, err := f.db.ExecContext(f.ctx, `UPDATE articles SET category_pinned=$2 WHERE id=$1`, id, local); err != nil {
			t.Fatal(err)
		}
		return id
	}
	local := seed("local-pin", category.ID, "public", "2024-01-01", false, true)
	global := seed("global-pin", category.ID, "public", "2025-01-01", true, false)
	newest := seed("newest", category.ID, "public", "2026-01-01", false, false)
	seed("private-pin", category.ID, "private", "2025-05-01", false, true)
	seed("secret", hidden.ID, "private", "2025-01-01", false, true)
	seed("another-category", other.ID, "public", "2026-01-01", false, true)
	getPage := func(path string, owner bool) numberedArticlePage {
		t.Helper()
		w := f.call("GET", path, "", owner)
		if w.Code != 200 {
			t.Fatalf("page: %d %s", w.Code, w.Body)
		}
		var page numberedArticlePage
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		return page
	}
	route := "/api/v1/categories/technology/articles"
	page := getPage(route+"?page=1&limit=2", false)
	if page.Total != 3 || page.TotalPages != 2 || len(page.Items) != 2 || page.Items[0].ID != local || page.Items[1].ID != newest {
		t.Fatalf("category ordering: %+v", page)
	}
	page = getPage(route+"?page=2&limit=2", false)
	if page.Items[0].ID != global {
		t.Fatal("wrong second page")
	}
	page = getPage("/api/v1/articles?page=1&category=technology", false)
	if page.Items[0].ID != global {
		t.Fatal("global ordering changed")
	}
	page = getPage(route+"?q=systems&sort=written", false)
	if page.Items[0].ID != newest {
		t.Fatal("search promoted category pin")
	}
	page = getPage(route+"?language=zh", false)
	if page.Total != 0 {
		t.Fatal("filter escaped category")
	}
	page = getPage("/api/v1/admin/reading/categories/technology/articles?limit=2", true)
	if page.Total != 4 || !page.Items[0].CategoryPinned || page.Items[0].Visibility != "private" {
		t.Fatal("owner scope lost private category pin")
	}
	for _, path := range []string{"/api/v1/categories/private-only", "/api/v1/categories/private-only/articles", "/api/v1/admin/reading/categories/technology"} {
		w := f.call("GET", path, "", false)
		if w.Code != 404 && w.Code != 401 {
			t.Fatalf("metadata leaked: %d %s", w.Code, w.Body)
		}
	}
	for _, query := range []string{"?category=other", "?category=technology", "?cursor=x", "?page=0", "?page=1&page=2"} {
		if w := f.call("GET", route+query, "", false); w.Code != 400 {
			t.Fatalf("invalid query accepted %s: %d", query, w.Code)
		}
	}
	w := f.call("GET", "/api/v1/categories/technology", "", false)
	var metadata categoryPage
	json.Unmarshal(w.Body.Bytes(), &metadata)
	if w.Code != 200 || metadata.Total != 3 || metadata.DescriptionZH != "技术与探索" {
		t.Fatal("bad public metadata")
	}
	before := imageArticle(t, f.call("GET", "/api/v1/admin/articles/"+local, "", true))
	body := fmt.Sprintf(`{"name":"technology","expected_version":%d,"description_en":"Updated description","description_zh":"新的介绍"}`, category.Version)
	w = f.call("PATCH", "/api/v1/admin/categories/"+category.ID, body, true)
	if w.Code != 200 {
		t.Fatalf("description update %s", w.Body)
	}
	if stale := f.call("PATCH", "/api/v1/admin/categories/"+category.ID, body, true); stale.Code != 409 {
		t.Fatal("stale category update accepted")
	}
	after := imageArticle(t, f.call("GET", "/api/v1/admin/articles/"+local, "", true))
	if after.Version != before.Version || !after.UpdatedAt.Equal(*before.UpdatedAt) {
		t.Fatal("description changed article dates/version")
	}
	pin := func(value bool) *Article {
		t.Helper()
		w := f.call("PATCH", "/api/v1/admin/articles/"+local, fmt.Sprintf(`{"expected_version":%d,"category_pinned":%t}`, after.Version, value), true)
		return imageArticle(t, w)
	}
	after = pin(false)
	after = pin(true)
	if !after.UpdatedAt.Equal(*before.UpdatedAt) || !after.PublishedAt.Equal(*before.PublishedAt) || after.Pinned {
		t.Fatal("category pin changed content dates or global pin")
	}
	if w := f.call("PATCH", "/api/v1/admin/articles/"+local, fmt.Sprintf(`{"expected_version":%d,"category_pinned":null}`, after.Version), true); w.Code != 422 {
		t.Fatal("null pin accepted")
	}
	// Import a replacement draft before publishing; metadata edits require a draft.
	source := []byte("---\nlanguage: en\nwritten_at: 2024-01-01\ncategory: technology\n---\n# Systems update\nUpdated content")
	f.upload(t, "same-category.md", source, after)
	after = imageArticle(t, f.call("GET", "/api/v1/admin/articles/"+local, "", true))
	// Publish same-category and changed-category drafts through real service paths.
	after = imageArticle(t, f.call("PATCH", "/api/v1/admin/articles/"+local+"/draft", fmt.Sprintf(`{"expected_version":%d,"summary":"changed summary"}`, after.Version), true))
	after = imageArticle(t, f.call("POST", "/api/v1/admin/articles/"+local+"/publish", fmt.Sprintf(`{"expected_version":%d}`, after.Version), true))
	if !after.CategoryPinned {
		t.Fatal("same category lost pin")
	}
	f.upload(t, "changed-category.md", source, after)
	after = imageArticle(t, f.call("GET", "/api/v1/admin/articles/"+local, "", true))
	after = imageArticle(t, f.call("PATCH", "/api/v1/admin/articles/"+local+"/draft", fmt.Sprintf(`{"expected_version":%d,"category_id":%q}`, after.Version, other.ID), true))
	if !after.CategoryPinned {
		t.Fatal("draft category prematurely cleared pin")
	}
	after = imageArticle(t, f.call("POST", "/api/v1/admin/articles/"+local+"/publish", fmt.Sprintf(`{"expected_version":%d}`, after.Version), true))
	if after.CategoryPinned {
		t.Fatal("changed category retained pin")
	}
	after = pin(true)
	after = imageArticle(t, f.call("POST", "/api/v1/admin/articles/"+local+"/unpublish", fmt.Sprintf(`{"expected_version":%d}`, after.Version), true))
	if after.CategoryPinned {
		t.Fatal("unpublished pin retained")
	}
	bad := fmt.Sprintf(`{"name":"technology","expected_version":2,"description_en":%q}`, strings.Repeat("a", 2001))
	if w := f.call("PATCH", "/api/v1/admin/categories/"+category.ID, bad, true); w.Code != 422 {
		t.Fatal("oversize description accepted")
	}
}

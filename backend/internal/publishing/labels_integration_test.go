//go:build integration

package publishing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testJSON(value any) string { data, _ := json.Marshal(value); return string(data) }
func createdLabel(t *testing.T, w *httptest.ResponseRecorder) adminLabel {
	t.Helper()
	if w.Code != 201 && w.Code != 200 {
		t.Fatalf("label: %d %s", w.Code, w.Body)
	}
	var result adminLabel
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func TestLabelManagementAssignmentAndPublicPrivacy(t *testing.T) {
	f := newImageFixture(t)
	create := func(kind, name, slug string) adminLabel {
		return createdLabel(t, f.call("POST", "/api/v1/admin/"+kind, testJSON(map[string]string{"name": name, "slug": slug}), true))
	}
	category := create("categories", "Technology", "technology")
	tag := create("tags", "Go", "go")
	privateCategory := create("categories", "Private journal", "private-journal")
	privateTag := create("tags", "Hidden", "hidden")
	create("categories", "中文分类", "chinese")
	create("categories", "Search", "search") // Only article URL search is reserved.
	if f.call("POST", "/api/v1/admin/categories", `{"name":"Duplicate","slug":"technology"}`, true).Code != 409 {
		t.Fatal("duplicate slug accepted")
	}
	if f.call("POST", "/api/v1/admin/categories", `{"name":"No access","slug":"no-access"}`, false).Code != 401 {
		t.Fatal("anonymous category mutation")
	}
	for _, name := range []string{"", " \t\n", strings.Repeat("a", 121), "bad\x00name"} {
		if f.call("POST", "/api/v1/admin/tags", testJSON(map[string]string{"name": name, "slug": "bad"}), true).Code != 422 {
			t.Fatalf("invalid name accepted: %q", name)
		}
	}
	for _, slug := range []string{"", "UPPER", "a/b", "中文", strings.Repeat("a", 161)} {
		if f.call("POST", "/api/v1/admin/tags", testJSON(map[string]string{"name": "Valid", "slug": slug}), true).Code != 422 {
			t.Fatalf("invalid slug accepted: %q", slug)
		}
	}
	if f.call("POST", "/api/v1/admin/tags", `{"name":"Extra","slug":"extra","version":1}`, true).Code != 422 {
		t.Fatal("create accepted unknown fields")
	}
	list := func(kind string, admin bool) []adminLabel {
		t.Helper()
		prefix := "/api/v1/"
		if admin {
			prefix += "admin/"
		}
		w := f.call("GET", prefix+kind+"?limit=100", "", true)
		if w.Code != 200 {
			t.Fatalf("list: %d %s", w.Code, w.Body)
		}
		var page struct {
			Items []adminLabel `json:"items"`
		}
		json.Unmarshal(w.Body.Bytes(), &page)
		if !admin && strings.Contains(w.Body.String(), `"version"`) {
			t.Fatal("public labels exposed admin versions")
		}
		return page.Items
	}
	if len(list("categories", false)) != 0 || len(list("tags", false)) != 0 {
		t.Fatal("unassigned labels exposed publicly")
	}
	if f.call("GET", "/api/v1/admin/tags", "", false).Code != 401 {
		t.Fatal("anonymous admin labels")
	}
	job := f.upload(t, "article.md", []byte("---\nwritten_at: 2026-11-07\n---\n# Public article"), nil)
	base := "/api/v1/admin/articles/" + *job.ArticleID
	a := imageArticle(t, f.call("GET", base, "", true))
	a = imageArticle(t, f.call("PATCH", base+"/draft", testJSON(map[string]any{"expected_version": a.Version, "category_id": category.ID, "tag_ids": []string{tag.ID}}), true))
	if a.Draft.Category.ID != category.ID || len(a.Draft.Tags) != 1 || a.Draft.Tags[0].ID != tag.ID {
		t.Fatal("draft label assignment lost")
	}
	if len(list("categories", false)) != 0 {
		t.Fatal("draft-only labels public")
	}
	if f.call("PATCH", base+"/draft", testJSON(map[string]any{"expected_version": a.Version, "tag_ids": []string{tag.ID, tag.ID}}), true).Code != 422 {
		t.Fatal("duplicate tag IDs accepted")
	}
	if f.call("PATCH", base+"/draft", testJSON(map[string]any{"expected_version": a.Version, "category_id": tag.ID}), true).Code != 422 {
		t.Fatal("tag accepted as category")
	}
	a = imageArticle(t, f.call("POST", base+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	firstRevision := a.Published.ID
	if len(list("categories", false)) != 0 {
		t.Fatal("private published labels public even with owner cookie")
	}
	a = imageArticle(t, f.call("PATCH", base, fmt.Sprintf(`{"expected_version":%d,"visibility":"public"}`, a.Version), true))
	if got := list("categories", false); len(got) != 1 || got[0].ID != category.ID {
		t.Fatal("public categories incorrect")
	}
	if got := list("tags", false); len(got) != 1 || got[0].ID != tag.ID {
		t.Fatal("public tags incorrect")
	}
	privateJob := f.upload(t, "private.md", []byte("---\nwritten_at: 2026-11-07\n---\n# Private"), nil)
	privateBase := "/api/v1/admin/articles/" + *privateJob.ArticleID
	p := imageArticle(t, f.call("GET", privateBase, "", true))
	p = imageArticle(t, f.call("PATCH", privateBase+"/draft", testJSON(map[string]any{"expected_version": p.Version, "category_id": privateCategory.ID, "tag_ids": []string{privateTag.ID}}), true))
	p = imageArticle(t, f.call("POST", privateBase+"/publish", fmt.Sprintf(`{"expected_version":%d}`, p.Version), true))
	if len(list("categories", false)) != 1 || len(list("tags", false)) != 1 {
		t.Fatal("private-only labels discoverable")
	}
	job = f.upload(t, "draft.md", []byte("# Pending labels"), a)
	if job.Status != "succeeded" {
		t.Fatal(job)
	}
	a = imageArticle(t, f.call("GET", base, "", true))
	a = imageArticle(t, f.call("PATCH", base+"/draft", testJSON(map[string]any{"expected_version": a.Version, "category_id": privateCategory.ID, "tag_ids": []string{privateTag.ID}}), true))
	if got := list("categories", false); len(got) != 1 || got[0].ID != category.ID {
		t.Fatal("new draft changed public taxonomy")
	}
	before := *a
	privateBefore := *p
	rename := func(kind string, label adminLabel, name string) adminLabel {
		return createdLabel(t, f.call("PATCH", "/api/v1/admin/"+kind+"/"+label.ID, testJSON(map[string]any{"name": name, "expected_version": label.Version}), true))
	}
	oldTag := tag
	tag = rename("tags", tag, "Golang")
	a = imageArticle(t, f.call("GET", base, "", true))
	p = imageArticle(t, f.call("GET", privateBase, "", true))
	if tag.ID != oldTag.ID || tag.Slug != oldTag.Slug || tag.Version != oldTag.Version+1 || a.Version != before.Version+1 || !a.UpdatedAt.After(*before.UpdatedAt) || !a.PublishedAt.Equal(*before.PublishedAt) || *a.Slug != *before.Slug || p.Version != privateBefore.Version {
		t.Fatal("rename did not preserve identity or atomically update only affected live articles")
	}
	if live := f.call("GET", "/api/v1/articles/"+*a.Slug, "", false); !strings.Contains(live.Body.String(), "Golang") {
		t.Fatal("public metadata did not rename")
	}
	if f.call("PATCH", base+"/draft", fmt.Sprintf(`{"expected_version":%d,"title":"stale"}`, before.Version), true).Code != 409 {
		t.Fatal("rename failed to invalidate stale article edit")
	}
	if f.call("PATCH", "/api/v1/admin/tags/"+tag.ID, `{"expected_version":1,"name":"stale"}`, true).Code != 409 {
		t.Fatal("stale label rename accepted")
	}
	if f.call("PATCH", "/api/v1/admin/tags/"+tag.ID, testJSON(map[string]any{"expected_version": tag.Version, "name": "No slug changes", "slug": "new"}), true).Code != 422 {
		t.Fatal("rename accepted slug change")
	}
	before = *a
	privateBefore = *p
	privateTag = rename("tags", privateTag, "Hidden renamed")
	a = imageArticle(t, f.call("GET", base, "", true))
	p = imageArticle(t, f.call("GET", privateBase, "", true))
	if a.Version != before.Version+1 || !a.UpdatedAt.Equal(*before.UpdatedAt) || p.Version != privateBefore.Version+1 || !p.UpdatedAt.After(*privateBefore.UpdatedAt) {
		t.Fatal("draft-only and live rename date rules incorrect")
	}
	// Old published label identities stay associated, but names resolve globally.
	history := f.call("GET", base+"/revisions/"+firstRevision, "", true)
	if history.Code != 200 || !strings.Contains(history.Body.String(), "Golang") {
		t.Fatal("historical association lost on rename")
	}
	a = imageArticle(t, f.call("POST", base+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	if got := list("categories", false); len(got) != 1 || got[0].ID != privateCategory.ID {
		t.Fatal("old history-only category exposed")
	}
	if got := list("tags", false); len(got) != 1 || got[0].ID != privateTag.ID {
		t.Fatal("old history-only tag exposed")
	}
	before = *a
	category = rename("categories", category, "Technology renamed")
	a = imageArticle(t, f.call("GET", base, "", true))
	if a.Version != before.Version || !a.UpdatedAt.Equal(*before.UpdatedAt) {
		t.Fatal("history-only rename changed unrelated current state")
	}
	same := rename("categories", category, category.Name)
	if same.Version != category.Version {
		t.Fatal("same-name update was not a no-op")
	}
	for _, filter := range []string{"category=private-journal", "tag=hidden", "category=private-journal&tag=hidden"} {
		w := f.call("GET", "/api/v1/articles?"+filter, "", false)
		if w.Code != 200 || !strings.Contains(w.Body.String(), a.ID) || strings.Contains(w.Body.String(), p.ID) {
			t.Fatalf("public label filter leaked/missed articles: %s", filter)
		}
	}
	a = imageArticle(t, f.call("PATCH", base, fmt.Sprintf(`{"expected_version":%d,"visibility":"private"}`, a.Version), true))
	if len(list("categories", false)) != 0 || len(list("tags", false)) != 0 {
		t.Fatal("privacy change left labels public")
	}
	// Ambiguous names are allowed; selection by ID resolves imported ambiguity.
	duplicate := create("categories", "Ambiguous", "ambiguous-a")
	create("categories", "Ambiguous", "ambiguous-b")
	ambiguous := f.upload(t, "ambiguous.md", []byte("---\ncategory: Ambiguous\nwritten_at: 2026-11-07\n---\n# Ambiguous"), nil)
	if ambiguous.Status != "succeeded" || ambiguous.Publishable {
		t.Fatal("ambiguous category silently selected")
	}
	ambBase := "/api/v1/admin/articles/" + *ambiguous.ArticleID
	amb := imageArticle(t, f.call("GET", ambBase, "", true))
	amb = imageArticle(t, f.call("PATCH", ambBase+"/draft", testJSON(map[string]any{"expected_version": amb.Version, "category_id": duplicate.ID}), true))
	if !amb.Publishable {
		t.Fatal("explicit category selection failed to resolve import issue")
	}
	// Deterministic name/ID ordering and kind/audience cursor binding.
	all := list("categories", true)
	sort.Slice(all, func(i, j int) bool {
		if all[i].Name == all[j].Name {
			return all[i].ID < all[j].ID
		}
		return all[i].Name < all[j].Name
	})
	cursor := ""
	seen := []string{}
	for {
		query := "?limit=1"
		if cursor != "" {
			query += "&cursor=" + url.QueryEscape(cursor)
		}
		w := f.call("GET", "/api/v1/admin/categories"+query, "", true)
		if w.Code != 200 {
			t.Fatal(w.Body)
		}
		var page struct {
			Items []adminLabel `json:"items"`
			Next  *string      `json:"next_cursor"`
		}
		json.Unmarshal(w.Body.Bytes(), &page)
		for _, l := range page.Items {
			seen = append(seen, l.ID)
		}
		if page.Next == nil {
			break
		}
		cursor = *page.Next
	}
	if len(seen) != len(all) {
		t.Fatal("label pagination omitted rows")
	}
	for i, label := range all {
		if seen[i] != label.ID {
			t.Fatal("name/ID cursor ordering incorrect")
		}
	}
	if f.call("GET", "/api/v1/admin/tags?cursor="+url.QueryEscape(cursor), "", true).Code != 400 || f.call("GET", "/api/v1/categories?cursor="+url.QueryEscape(cursor), "", true).Code != 400 {
		t.Fatal("label cursor reused for another kind/audience")
	}
	for _, query := range []string{"?limit=", "?limit=101", "?limit=1&limit=2", "?cursor=broken", "?unknown=1"} {
		if f.call("GET", "/api/v1/tags"+query, "", false).Code != 400 {
			t.Fatalf("invalid label query accepted: %s", query)
		}
	}
	originalCSRF := f.csrf
	f.csrf = ""
	if f.call("POST", "/api/v1/admin/tags", `{"name":"No CSRF","slug":"csrf"}`, true).Code != 403 {
		t.Fatal("label mutation bypassed CSRF")
	}
	f.csrf = originalCSRF
}

func TestLabelRenameWaitsForAssociationTransaction(t *testing.T) {
	f := newImageFixture(t)
	category := createdLabel(t, f.call("POST", "/api/v1/admin/categories", `{"name":"Before","slug":"before"}`, true))
	job := f.upload(t, "article.md", []byte("---\nwritten_at: 2026-11-07\n---\n# Article"), nil)
	base := "/api/v1/admin/articles/" + *job.ArticleID
	a := imageArticle(t, f.call("GET", base, "", true))
	tx, err := f.db.BeginTx(f.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	locked, err := lockArticle(f.ctx, tx, a.ID, a.Version)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := *locked.Draft
	snapshot.Category = &category.Label
	if err := insertRevision(f.ctx, tx, &snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(f.ctx, `UPDATE articles SET draft_revision_id=$1,version=version+1 WHERE id=$2`, snapshot.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	// A short deadline proves rename cannot pass the active association writer.
	timeout, cancel := context.WithTimeout(f.ctx, 100*time.Millisecond)
	_, err = f.s.renameLabel(timeout, "categories", category.ID, map[string]json.RawMessage{"name": json.RawMessage(`"After"`), "expected_version": json.RawMessage(`1`)})
	cancel()
	if err == nil {
		t.Fatal("rename passed active association transaction")
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	updated, err := f.s.renameLabel(f.ctx, "categories", category.ID, map[string]json.RawMessage{"name": json.RawMessage(`"After"`), "expected_version": json.RawMessage(`1`)})
	if err != nil || updated.Version != 2 {
		t.Fatalf("rename after writer commit: %v", err)
	}
	after := imageArticle(t, f.call("GET", base, "", true))
	if after.Version != a.Version+2 || after.Draft.Category.Name != "After" {
		t.Fatal("rename missed newly committed association/version")
	}
	if f.call("PATCH", "/api/v1/admin/categories/"+uuid.NewString(), `{"name":"Missing","expected_version":1}`, true).Code != 404 {
		t.Fatal("missing label rename did not return 404")
	}
}

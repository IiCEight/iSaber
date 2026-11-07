//go:build integration

package publishing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestPublishedHistoryAndRestore(t *testing.T) {
	f := newImageFixture(t)
	category, tag := uuid.NewString(), uuid.NewString()
	if _, err := f.db.ExecContext(f.ctx, `INSERT INTO categories(id,name,slug) VALUES($1,'Technology','technology')`, category); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(f.ctx, `INSERT INTO tags(id,name,slug) VALUES($1,'Go','go')`, tag); err != nil {
		t.Fatal(err)
	}
	oldBytes := testPNG(t, 20)
	firstSource := "---\ntitle: First 中文\nsummary: First summary\nlanguage: zh\nwritten_at: 2026-11-01\ncategory: Technology\ntags: [Go]\n---\n# First\n![old](old.png)"
	job := f.upload(t, "first.zip", testZIP(t, zipEntry{"post.md", []byte(firstSource), 0}, zipEntry{"old.png", oldBytes, 0}), nil)
	if job.Status != "succeeded" || !job.Publishable {
		t.Fatalf("first import: %+v", job)
	}
	base := "/api/v1/admin/articles/" + *job.ArticleID
	a := imageArticle(t, f.call("GET", base, "", true))
	history := base + "/revisions"
	empty := f.call("GET", history, "", true)
	if empty.Code != 200 || !strings.Contains(empty.Body.String(), `"items":[]`) {
		t.Fatalf("empty history: %d %s", empty.Code, empty.Body)
	}
	if f.call("GET", history, "", false).Code != 401 {
		t.Fatal("anonymous history access")
	}
	draftID := a.Draft.ID
	if f.call("GET", history+"/"+draftID, "", true).Code != 404 {
		t.Fatal("draft listed as published history")
	}
	a = imageArticle(t, f.call("POST", base+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	firstID := a.Published.ID
	firstSnapshot, err := loadRevision(f.ctx, f.db, firstID)
	if err != nil {
		t.Fatal(err)
	}
	oldURL := assetURL(a.ID, firstSnapshot.Assets[0].ID)
	a = imageArticle(t, f.call("PATCH", base, fmt.Sprintf(`{"expected_version":%d,"visibility":"public","comments_enabled":false}`, a.Version), true))
	secondSource := "---\ntitle: Second\nsummary: Second summary\nlanguage: en\nwritten_at: 2026-11-02\ncategory: null\ntags: []\n---\n# Second\n![new](new.png)"
	job = f.upload(t, "second.zip", testZIP(t, zipEntry{"post.md", []byte(secondSource), 0}, zipEntry{"new.png", testPNG(t, 80), 0}), a)
	if job.Status != "succeeded" {
		t.Fatal(job)
	}
	a = imageArticle(t, f.call("GET", base, "", true))
	secondDraft := a.Draft.ID
	if f.call("GET", history+"/"+secondDraft, "", true).Code != 404 {
		t.Fatal("unpublished replacement exposed in history")
	}
	a = imageArticle(t, f.call("POST", base+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	secondID := a.Published.ID
	secondSnapshot, err := loadRevision(f.ctx, f.db, secondID)
	if err != nil {
		t.Fatal(err)
	}
	newURL := assetURL(a.ID, secondSnapshot.Assets[0].ID)
	// Deliberately tie timestamps: ID ordering must prevent omissions/duplicates.
	if _, err := f.db.ExecContext(f.ctx, `UPDATE revisions SET first_published_at='2026-11-01T08:00:00Z' WHERE article_id=$1`, a.ID); err != nil {
		t.Fatal(err)
	}
	type revisionPage struct {
		Items []Revision `json:"items"`
		Next  *string    `json:"next_cursor"`
	}
	page := func(query string) revisionPage {
		t.Helper()
		w := f.call("GET", history+query, "", true)
		if w.Code != 200 {
			t.Fatalf("history: %d %s", w.Code, w.Body)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("history cached")
		}
		var result revisionPage
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	one := page("?limit=1")
	if len(one.Items) != 1 || one.Next == nil {
		t.Fatal("missing history cursor")
	}
	two := page("?limit=1&cursor=" + url.QueryEscape(*one.Next))
	expected := []string{firstID, secondID}
	sort.Sort(sort.Reverse(sort.StringSlice(expected)))
	if len(two.Items) != 1 || two.Next != nil || one.Items[0].ID != expected[0] || two.Items[0].ID != expected[1] {
		t.Fatal("history cursor order skipped/repeated tied revisions")
	}
	for _, query := range []string{"?limit=101", "?limit=1&limit=2", "?cursor=invalid", "?unexpected=true"} {
		if f.call("GET", history+query, "", true).Code != 400 {
			t.Fatalf("invalid history query accepted: %s", query)
		}
	}
	foreignJob := f.upload(t, "foreign.md", []byte("---\nwritten_at: 2026-11-01\n---\n# Foreign"), nil)
	foreignBase := "/api/v1/admin/articles/" + *foreignJob.ArticleID
	if f.call("GET", foreignBase+"/revisions?cursor="+url.QueryEscape(*one.Next), "", true).Code != 400 {
		t.Fatal("cursor reused across articles")
	}
	if f.call("GET", foreignBase+"/revisions/"+firstID, "", true).Code != 404 {
		t.Fatal("foreign revision read")
	}
	if f.call("GET", "/api/v1/admin/articles/"+uuid.NewString()+"/revisions", "", true).Code != 404 {
		t.Fatal("unknown article history did not return 404")
	}
	historical := f.call("GET", history+"/"+firstID, "", true)
	if historical.Code != 200 || !strings.Contains(historical.Body.String(), oldURL) || strings.Contains(historical.Body.String(), ".asset") {
		t.Fatal("historical revision content or image representation incorrect")
	}
	var source Revision
	json.Unmarshal(historical.Body.Bytes(), &source)
	// An existing draft must be replaced explicitly, with no effect on live state.
	job = f.upload(t, "work.md", []byte("# Work in progress"), a)
	if job.Status != "succeeded" {
		t.Fatal(job)
	}
	a = imageArticle(t, f.call("GET", base, "", true))
	oldDraft := a.Draft.ID
	original := *a
	liveBefore := f.call("GET", "/api/v1/articles/"+*a.Slug, "", false).Body.String()
	callRestore := func(target, revision, key, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", target+"/revisions/"+revision+"/restore", strings.NewReader(body))
		req.Header.Set("Origin", "https://blog.example")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", f.csrf)
		req.Header.Set("Idempotency-Key", key)
		req.AddCookie(f.cookie)
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, req)
		return w
	}
	body := fmt.Sprintf(`{"expected_version":%d}`, a.Version)
	if callRestore(base, oldDraft, uuid.NewString(), body).Code != 404 {
		t.Fatal("draft-only restore allowed")
	}
	if callRestore(foreignBase, firstID, uuid.NewString(), `{"expected_version":1}`).Code != 404 {
		t.Fatal("cross-article restore allowed")
	}
	if callRestore(base, uuid.NewString(), uuid.NewString(), body).Code != 404 {
		t.Fatal("unknown revision restore allowed")
	}
	if callRestore(base, firstID, uuid.NewString(), `{"expected_version":1}`).Code != 409 {
		t.Fatal("stale restore overwrote current draft")
	}
	if callRestore(base, firstID, uuid.NewString(), fmt.Sprintf(`{"expected_version":%d,"visibility":"private"}`, a.Version)).Code != 422 {
		t.Fatal("restore accepted extra controls")
	}
	if callRestore(base, firstID, "", body).Code != 400 {
		t.Fatal("restore accepted missing retry key")
	}
	savedCSRF := f.csrf
	f.csrf = ""
	if callRestore(base, firstID, uuid.NewString(), body).Code != 403 {
		t.Fatal("restore accepted missing CSRF")
	}
	f.csrf = savedCSRF
	key := uuid.NewString()
	accepted := callRestore(base, firstID, key, body)
	a = imageArticle(t, accepted)
	restoredID := a.Draft.ID
	if restoredID == firstID || restoredID == oldDraft || a.Draft.FirstPublishedAt != nil || a.Draft.RestoredFromRevision == nil || *a.Draft.RestoredFromRevision != firstID {
		t.Fatal("restore did not create new unpublished snapshot/provenance")
	}
	if a.Draft.Markdown != source.Markdown || *a.Draft.Title != *source.Title || *a.Draft.Summary != *source.Summary || *a.Draft.Language != *source.Language || *a.Draft.WrittenAt != *source.WrittenAt || a.Draft.Category.ID != category || len(a.Draft.Tags) != 1 || a.Draft.Tags[0].ID != tag {
		t.Fatal("restore lost historical metadata or body")
	}
	if a.Published.ID != secondID || a.Visibility != original.Visibility || a.CommentsEnabled != original.CommentsEnabled || *a.Slug != *original.Slug || !a.PublishedAt.Equal(*original.PublishedAt) || !a.UpdatedAt.Equal(*original.UpdatedAt) || a.Version != original.Version+1 || !a.ModifiedAt.After(original.ModifiedAt) {
		t.Fatal("restore changed live state, controls, dates, or version incorrectly")
	}
	if got := f.call("GET", "/api/v1/articles/"+*a.Slug, "", false); got.Body.String() != liveBefore {
		t.Fatal("restore changed live article")
	}
	if f.call("GET", oldURL, "", false).Code != 404 || f.call("GET", oldURL, "", true).Code != 200 || f.call("GET", newURL, "", false).Code != 200 {
		t.Fatal("restore changed public image access")
	}
	if replay := callRestore(base, firstID, key, body); replay.Code != 200 || !bytes.Equal(replay.Body.Bytes(), accepted.Body.Bytes()) {
		t.Fatal("restore retry changed response or ran twice")
	}
	if callRestore(base, secondID, key, body).Code != 409 {
		t.Fatal("same retry key accepted a different history target")
	}
	// A subsequent edit must not be undone by retrying an uncertain restoration.
	a = imageArticle(t, f.call("PATCH", base+"/draft", fmt.Sprintf(`{"expected_version":%d,"title":"Edited restored draft"}`, a.Version), true))
	if a.Draft.RestoredFromRevision == nil || *a.Draft.RestoredFromRevision != firstID {
		t.Fatal("metadata edit lost restoration provenance")
	}
	currentDraft := a.Draft.ID
	if callRestore(base, firstID, key, body).Code != 200 {
		t.Fatal("retry rejected after later edit")
	}
	after := imageArticle(t, f.call("GET", base, "", true))
	if after.Draft.ID != currentDraft || after.Version != a.Version {
		t.Fatal("replay overwrote later edit")
	}
	if len(page("").Items) != 2 {
		t.Fatal("unpublished restored draft added to publication history")
	}
	a = imageArticle(t, f.call("POST", base+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	if a.Published.ID != currentDraft || a.Draft != nil || a.Published.RestoredFromRevision == nil || !a.PublishedAt.Equal(*original.PublishedAt) {
		t.Fatal("restored publication lost identity/provenance/date")
	}
	if f.call("GET", oldURL, "", false).Code != 200 || f.call("GET", newURL, "", false).Code != 404 {
		t.Fatal("restored publication did not switch image membership")
	}
	if len(page("").Items) != 3 {
		t.Fatal("newly published restoration absent from history")
	}
	unchanged := f.call("GET", history+"/"+firstID, "", true)
	if !bytes.Equal(unchanged.Body.Bytes(), historical.Body.Bytes()) {
		t.Fatal("source historical snapshot was modified")
	}
	// A private/unpublished article retains owner-readable history and restores.
	a = imageArticle(t, f.call("PATCH", base, fmt.Sprintf(`{"expected_version":%d,"visibility":"private"}`, a.Version), true))
	a = imageArticle(t, f.call("POST", base+"/unpublish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	a = imageArticle(t, callRestore(base, secondID, uuid.NewString(), fmt.Sprintf(`{"expected_version":%d}`, a.Version)))
	if a.Published != nil || a.Visibility != "private" || a.Draft.RestoredFromRevision == nil || *a.Draft.RestoredFromRevision != secondID || len(page("").Items) != 3 {
		t.Fatal("private unpublished restore changed access/history")
	}
	if f.call("GET", newURL, "", false).Code != 404 || f.call("GET", newURL, "", true).Code != 200 {
		t.Fatal("private restoration media leaked")
	}
}

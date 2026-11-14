//go:build integration

package publishing

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestImportedUpdatedAtPublicationAndDraftIsolation(t *testing.T) {
	f := newImageFixture(t)
	category := f.call("POST", "/api/v1/admin/categories", `{"name":"Old notes","slug":"old-notes"}`, true)
	var label adminLabel
	if category.Code != 201 || json.Unmarshal(category.Body.Bytes(), &label) != nil {
		t.Fatalf("create category: %d %s", category.Code, category.Body)
	}
	job := f.upload(t, "old.md", []byte("---\nwritten_at: 2024-06-01\nupdated_at: 2024-06-15T18:30:00+08:00\ncategory: Old notes\n---\n# Older article"), nil)
	base := "/api/v1/admin/articles/" + *job.ArticleID
	a := imageArticle(t, f.call("GET", base, "", true))
	want, _ := time.Parse(time.RFC3339, "2024-06-15T10:30:00Z")
	if !a.Publishable || a.Draft.UpdatedAt == nil || !a.Draft.UpdatedAt.Equal(want) || a.UpdatedAt != nil {
		t.Fatalf("import timestamp: %+v", a)
	}
	a = imageArticle(t, f.call("POST", base+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	firstPublished := *a.PublishedAt
	firstRevision := a.Published.ID
	if a.UpdatedAt == nil || !a.UpdatedAt.Equal(want) || !firstPublished.After(want) {
		t.Fatal("publication did not preserve the imported last updated time")
	}
	// Making the private publication public must not erase its chosen date.
	a = imageArticle(t, f.call("PATCH", base, fmt.Sprintf(`{"expected_version":%d,"visibility":"public"}`, a.Version), true))
	publicDate := func() time.Time {
		t.Helper()
		w := f.call("GET", "/api/v1/articles/"+*a.Slug, "", false)
		var value publicArticle
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &value) != nil {
			t.Fatalf("public article: %d %s", w.Code, w.Body)
		}
		return value.UpdatedAt
	}
	if !publicDate().Equal(want) {
		t.Fatal("visibility change overwrote the chosen timestamp")
	}
	if w := f.call("PATCH", "/api/v1/admin/categories/"+label.ID, fmt.Sprintf(`{"expected_version":%d,"name":"Earlier notes"}`, label.Version), true); w.Code != 200 {
		t.Fatalf("rename category: %d %s", w.Code, w.Body)
	}
	a = imageArticle(t, f.call("GET", base, "", true))
	if !a.UpdatedAt.Equal(want) || !publicDate().Equal(want) {
		t.Fatal("label rename overwrote the chosen timestamp")
	}
	// A subsequent upload without updated_at returns to automatic dating.
	job = f.upload(t, "old.md", []byte("# New content"), a)
	a = imageArticle(t, f.call("GET", base, "", true))
	if job.Status != "succeeded" || a.Draft.UpdatedAt != nil || !publicDate().Equal(want) {
		t.Fatal("replacement import changed the live timestamp or retained the old override")
	}
	for _, invalid := range []string{`"2024-06-15"`, `"invalid"`, `"0000-01-01T00:00:00Z"`, `123`, `{}`} {
		w := f.call("PATCH", base+"/draft", fmt.Sprintf(`{"expected_version":%d,"updated_at":%s}`, a.Version, invalid), true)
		if w.Code != 422 {
			t.Fatalf("invalid timestamp accepted: %d %s", w.Code, w.Body)
		}
	}
	a = imageArticle(t, f.call("PATCH", base+"/draft", fmt.Sprintf(`{"expected_version":%d,"updated_at":"2024-07-01T09:00:00+08:00"}`, a.Version), true))
	if !publicDate().Equal(want) {
		t.Fatal("draft metadata changed the public timestamp")
	}
	a = imageArticle(t, f.call("POST", base+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	secondDate, _ := time.Parse(time.RFC3339, "2024-07-01T01:00:00Z")
	if !publicDate().Equal(secondDate) || !a.PublishedAt.Equal(firstPublished) {
		t.Fatal("metadata timestamp was not published or first publication changed")
	}
	// Restoration copies the historical choice into a draft and leaves live dates alone.
	a = imageArticle(t, f.call("POST", base+"/revisions/"+firstRevision+"/restore", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	if a.Draft.UpdatedAt == nil || !a.Draft.UpdatedAt.Equal(want) || !publicDate().Equal(secondDate) {
		t.Fatal("restoration lost the timestamp or changed the live article")
	}
	a = imageArticle(t, f.call("PATCH", base+"/draft", fmt.Sprintf(`{"expected_version":%d,"updated_at":null}`, a.Version), true))
	before := time.Now().UTC()
	a = imageArticle(t, f.call("POST", base+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	if a.UpdatedAt.Before(before) || !a.PublishedAt.Equal(firstPublished) {
		t.Fatal("clearing updated_at did not restore automatic dating")
	}
}

func TestInvalidImportedUpdatedAtCanBeCorrected(t *testing.T) {
	f := newImageFixture(t)
	job := f.upload(t, "invalid.md", []byte("---\nwritten_at: 2024-06-01\nupdated_at: yesterday\n---\n# Fixable timestamp"), nil)
	if job.Status != "succeeded" || job.Publishable {
		t.Fatalf("invalid timestamp should be a fixable draft: %+v", job)
	}
	base := "/api/v1/admin/articles/" + *job.ArticleID
	a := imageArticle(t, f.call("GET", base, "", true))
	if w := f.call("POST", base+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true); w.Code != 422 {
		t.Fatal("invalid imported timestamp allowed publication")
	}
	a = imageArticle(t, f.call("PATCH", base+"/draft", fmt.Sprintf(`{"expected_version":%d,"updated_at":null}`, a.Version), true))
	if !a.Publishable || len(a.Issues) != 0 {
		t.Fatal("clearing invalid timestamp did not resolve its issue")
	}
}

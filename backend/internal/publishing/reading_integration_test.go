//go:build integration

package publishing

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestOwnerReadingIncludesOnlyPublishedContentAndKeepsPublicAPIsPublic(t *testing.T) {
	f := newImageFixture(t)
	privateCategory := createdLabel(t, f.call("POST", "/api/v1/admin/categories", `{"name":"Private category","slug":"private-category"}`, true))
	draftCategory := createdLabel(t, f.call("POST", "/api/v1/admin/categories", `{"name":"Draft category","slug":"draft-category"}`, true))
	privateTag := createdLabel(t, f.call("POST", "/api/v1/admin/tags", `{"name":"Private tag","slug":"private-tag"}`, true))
	for i := 0; i < 13; i++ {
		id, revision := uuid.NewString(), uuid.NewString()
		visibility, body := "private", "Private corpus 私密内容"
		if i < 2 {
			visibility, body = "public", "Public corpus"
		}
		if _, err := f.db.ExecContext(f.ctx, `INSERT INTO articles(id,slug,visibility,published_at,updated_at) VALUES($1,$2,$3,'2026-11-01T08:00:00Z','2026-11-01T08:00:00Z')`, id, fmt.Sprintf("entry-%d", i), visibility); err != nil {
			t.Fatal(err)
		}
		var category *string
		if i == 2 {
			category = &privateCategory.ID
		} else if i == 12 {
			category = &draftCategory.ID
		}
		if _, err := f.db.ExecContext(f.ctx, `INSERT INTO revisions(id,article_id,title,summary,language,written_at,category_id,markdown) VALUES($1,$2,$3,'Published summary','en',$4,$5,$6)`, revision, id, fmt.Sprintf("Entry %02d", i), fmt.Sprintf("2026-01-%02d", i+1), category, body); err != nil {
			t.Fatal(err)
		}
		pointer := "published_revision_id"
		if i == 12 {
			pointer = "draft_revision_id"
		}
		if _, err := f.db.ExecContext(f.ctx, `UPDATE articles SET `+pointer+`=$1 WHERE id=$2`, revision, id); err != nil {
			t.Fatal(err)
		}
		if i == 2 {
			if _, err := f.db.ExecContext(f.ctx, `INSERT INTO revision_tags(revision_id,tag_id) VALUES($1,$2)`, revision, privateTag.ID); err != nil {
				t.Fatal(err)
			}
			draft := uuid.NewString()
			if _, err := f.db.ExecContext(f.ctx, `INSERT INTO revisions(id,article_id,title,language,written_at,markdown) VALUES($1,$2,'Unpublished secret','en','2099-01-01','unpublished-marker')`, draft, id); err != nil {
				t.Fatal(err)
			}
			if _, err := f.db.ExecContext(f.ctx, `UPDATE articles SET draft_revision_id=$1 WHERE id=$2`, draft, id); err != nil {
				t.Fatal(err)
			}
		}
	}
	const root = "/api/v1/admin/reading/"
	read := func(path string) numberedArticlePage {
		t.Helper()
		response := f.call("GET", path, "", true)
		var page numberedArticlePage
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &page) != nil {
			t.Fatalf("Reading page %s: %d %s", path, response.Code, response.Body)
		}
		if !strings.Contains(response.Header().Get("Cache-Control"), "no-store") {
			t.Fatal("Reading responses may not be stored")
		}
		if strings.HasPrefix(path, root) && response.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
			t.Fatal("Protected reader must not be indexed")
		}
		return page
	}
	for _, path := range []string{"articles?page=1", "articles/search?q=private&page=1", "articles/entry-2", "categories", "tags"} {
		if response := f.call("GET", root+path, "", false); response.Code != 401 {
			t.Fatalf("Anonymous owner reading access: %s %d", path, response.Code)
		}
	}
	public := read("/api/v1/articles?page=1")
	if public.Total != 2 || len(public.Items) != 2 {
		t.Fatal("Public API expanded its scope for an owner cookie")
	}
	for _, item := range read(root + "articles?limit=4").Items {
		if item.Visibility != "private" && item.Visibility != "public" {
			t.Fatal("Owner Home feed omitted publication visibility")
		}
	}
	for _, item := range public.Items {
		if item.Visibility != "" {
			t.Fatal("Public response shape changed")
		}
	}
	if response := f.call("GET", "/api/v1/articles/entry-2", "", true); response.Code != 404 {
		t.Fatal("Public detail exposed private content to an owner cookie")
	}
	for _, order := range []string{"written", "written_oldest", "newest", "oldest", "updated", "updated_oldest"} {
		seen := map[string]bool{}
		for number := 1; number <= 3; number++ {
			page := read(root + fmt.Sprintf("articles?page=%d&limit=5&sort=%s", number, order))
			if page.Total != 12 || page.TotalPages != 3 || page.Page != number {
				t.Fatalf("Owner totals/order incorrect: %+v", page)
			}
			for _, item := range page.Items {
				if seen[item.ID] || (item.Visibility != "private" && item.Visibility != "public") || item.Title == "Unpublished secret" {
					t.Fatal("Missing visibility, duplicate article, or draft in owner reader")
				}
				seen[item.ID] = true
			}
		}
		if len(seen) != 12 {
			t.Fatal("Incomplete owner collection")
		}
	}
	for query, total := range map[string]int64{"q=private": 10, "q=" + url.QueryEscape("私密内容"): 10, "q=unpublished-marker": 0} {
		if page := read(root + "articles/search?page=1&" + query); page.Total != total {
			t.Fatalf("Owner published-only search: %s %+v", query, page)
		}
	}
	if page := read("/api/v1/articles/search?q=private&page=1"); page.Total != 0 {
		t.Fatal("Private search results leaked through public endpoint")
	}
	if page := read(root + "articles?page=1&category=private-category&tag=private-tag"); page.Total != 1 || page.Items[0].Slug != "entry-2" {
		t.Fatal("Owner filters excluded private published labels")
	}
	for _, kind := range []string{"categories", "tags"} {
		owner := f.call("GET", root+kind, "", true)
		guest := f.call("GET", "/api/v1/"+kind, "", true)
		if owner.Code != 200 || !strings.Contains(owner.Body.String(), "Private") || strings.Contains(owner.Body.String(), "Draft category") || strings.Contains(guest.Body.String(), "Private") {
			t.Fatal("Reading labels crossed the published/private boundary")
		}
	}
	for _, path := range []string{"articles", "articles/search?q=entry"} {
		separator := "?"
		if strings.Contains(path, "?") {
			separator = "&"
		}
		owned := read(root + path + separator + "limit=1")
		if owned.NextCursor == nil {
			t.Fatal("Owner cursor continuation missing")
		}
		if response := f.call("GET", "/api/v1/"+path+separator+"limit=1&cursor="+url.QueryEscape(*owned.NextCursor), "", true); response.Code != 400 {
			t.Fatal("Owner cursor accepted in public scope")
		}
	}
	detail := f.call("GET", root+"articles/entry-2", "", true)
	if detail.Code != 200 || !strings.Contains(detail.Body.String(), "Private corpus") || strings.Contains(detail.Body.String(), "unpublished-marker") {
		t.Fatal("Owner detail failed to select the current published revision")
	}
	if response := f.call("GET", root+"articles/entry-12", "", true); response.Code != 404 {
		t.Fatal("Draft-only article was readable as a publication")
	}
	job := f.upload(t, "private-image.zip", testZIP(t, zipEntry{"private-image.md", []byte("---\ntitle: Private image\nwritten_at: 2026-01-01\nlanguage: en\n---\n# Private image\n![sample](sample.png)"), 0}, zipEntry{"sample.png", testPNG(t, 50), 0}), nil)
	article := imageArticle(t, f.call("GET", "/api/v1/admin/articles/"+*job.ArticleID, "", true))
	article = imageArticle(t, f.call("POST", "/api/v1/admin/articles/"+article.ID+"/publish", testJSON(map[string]any{"expected_version": article.Version}), true))
	revision, err := loadRevision(f.ctx, f.db, article.Published.ID)
	if err != nil || len(revision.Assets) != 1 {
		t.Fatal("Missing published image membership")
	}
	asset := revision.Assets[0]
	imagePath := assetURL(article.ID, asset.ID)
	if f.call("GET", root+"articles/"+*article.Slug, "", true).Code != 200 || f.call("GET", imagePath, "", true).Code != 200 || f.call("GET", imagePath, "", false).Code != 404 {
		t.Fatal("Owner reader image authorization failed")
	}
	if response := f.call("POST", "/api/v1/auth/logout", "", true); response.Code != 204 {
		t.Fatal("Fixture logout failed")
	}
	if f.call("GET", root+"articles/entry-2", "", true).Code != 401 || f.call("GET", imagePath, "", true).Code != 404 {
		t.Fatal("Revoked session retained private reading access")
	}
}

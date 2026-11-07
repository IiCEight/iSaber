//go:build integration

package publishing

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/IiCEight/iSaber/backend/internal/auth"
	"github.com/IiCEight/iSaber/backend/internal/httpapi"
	"github.com/google/uuid"
)

type imageFixture struct {
	s           *Service
	db          *sql.DB
	ctx         context.Context
	handler     http.Handler
	cookie      *http.Cookie
	csrf, owner string
}

func newImageFixture(t *testing.T) *imageFixture {
	t.Helper()
	db, ctx := publishingDatabase(t)
	store := auth.NewStore(db)
	password := "synthetic image testing password"
	if err := store.Setup(ctx, "owner", password); err != nil {
		t.Fatal(err)
	}
	secret := bytes.Repeat([]byte{9}, 32)
	access, err := auth.NewService(store, secret)
	if err != nil {
		t.Fatal(err)
	}
	cfg := auth.Config{Origin: "https://blog.example", CookieName: "__Host-isaber_session", Secure: true, Secret: secret}
	session, token, err := access.Login(ctx, "owner", password)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewService(db, t.TempDir()+"/uploads", WithCommentOrigin(cfg.Origin))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	handler := httpapi.NewHandler(nil, httpapi.WithAuthentication(auth.NewHTTPHandler(access, cfg, s.AdminHandler())), httpapi.WithPublicArticles(s.PublicHandler(auth.OptionalOwner(access, cfg, s.AssetHandler()))))
	return &imageFixture{s, db, ctx, handler, &http.Cookie{Name: cfg.CookieName, Value: token}, session.CSRFToken, session.Owner.ID}
}
func (f *imageFixture) call(method, path, body string, owner bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://blog.example")
	req.Header.Set("Idempotency-Key", uuid.NewString())
	if owner {
		req.AddCookie(f.cookie)
		req.Header.Set("X-CSRF-Token", f.csrf)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, req)
	return w
}
func (f *imageFixture) upload(t *testing.T, filename string, data []byte, a *Article) Job {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, _ := writer.CreateFormFile("file", filename)
	file.Write(data)
	if a != nil {
		writer.WriteField("article_id", a.ID)
		writer.WriteField("expected_version", fmt.Sprint(a.Version))
	}
	writer.Close()
	req := httptest.NewRequest("POST", "/api/v1/admin/imports", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Origin", "https://blog.example")
	req.Header.Set("X-CSRF-Token", f.csrf)
	req.Header.Set("Idempotency-Key", uuid.NewString())
	req.AddCookie(f.cookie)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, req)
	if w.Code != 202 {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	var job Job
	if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if done, err := f.s.ProcessNext(f.ctx); !done || err != nil {
		t.Fatalf("worker: %v %v", done, err)
	}
	job, err := f.s.GetJob(f.ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	return job
}
func imageArticle(t *testing.T, w *httptest.ResponseRecorder) *Article {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("article: %d %s", w.Code, w.Body)
	}
	var a Article
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	return &a
}

func TestImageMembershipPrivacyAndRevisionTransitions(t *testing.T) {
	f := newImageFixture(t)
	png := testPNG(t, 50)
	source := "---\ntitle: 中文 images\nwritten_at: 2026-11-07\n---\n# Images\n\n![inline](../images/a%20b.png)\n\n![ref][图]\n\n[图]: <../images/a%20b.png> \"Title\"\n\n```md\n![example](../images/a%20b.png)\n```\n"
	job := f.upload(t, "article.zip", testZIP(t, zipEntry{"notes/中文.md", []byte(source), 0}, zipEntry{"images/a b.png", png, 0}), nil)
	if job.Status != "succeeded" || !job.Publishable {
		t.Fatalf("image job: %+v", job)
	}
	path := "/api/v1/admin/articles/" + *job.ArticleID
	a := imageArticle(t, f.call("GET", path, "", true))
	revision, err := loadRevision(f.ctx, f.db, a.Draft.ID)
	if err != nil || len(revision.Assets) != 1 {
		t.Fatalf("image membership: %+v %v", revision, err)
	}
	item := revision.Assets[0]
	url := assetURL(a.ID, item.ID)
	if strings.Count(a.Draft.Markdown, url) != 2 || !strings.Contains(a.Draft.Markdown, "![example](../images/a%20b.png)") {
		t.Fatalf("rewrite damaged Markdown: %s", a.Draft.Markdown)
	}
	if strings.Contains(a.Draft.Markdown, item.StorageKey) {
		t.Fatal("private storage key leaked")
	}
	if f.call("GET", url, "", false).Code != 404 {
		t.Fatal("private draft image leaked")
	}
	for _, cookies := range [][]*http.Cookie{
		{{Name: f.cookie.Name, Value: "invalid-session"}},
		{f.cookie, f.cookie},
		{{Name: "isaber_dev_session", Value: f.cookie.Value}},
	} {
		req := httptest.NewRequest("GET", url, nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, req)
		if w.Code != 404 {
			t.Fatal("invalid, ambiguous, or wrong-namespace cookie granted media access")
		}
	}
	if f.call("GET", assetURL(a.ID, "invalid-id"), "", true).Code != 404 {
		t.Fatal("invalid asset identifier revealed a distinct resource response")
	}
	info, err := f.s.root.Stat(item.StorageKey)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("image bytes do not have private file permissions")
	}
	owner := f.call("GET", url, "", true)
	if owner.Code != 200 || !bytes.Equal(owner.Body.Bytes(), png) || owner.Header().Get("Content-Type") != "image/png" || owner.Header().Get("Cache-Control") != "no-store" || owner.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("owner image: %d %s", owner.Code, owner.Body)
	}
	if f.call("GET", assetURL(uuid.NewString(), item.ID), "", true).Code != 404 {
		t.Fatal("cross-article image exposed")
	}
	a = imageArticle(t, f.call("PATCH", path+"/draft", fmt.Sprintf(`{"expected_version":%d,"title":"Reviewed"}`, a.Version), true))
	if f.call("GET", url, "", true).Code != 200 || !a.Publishable {
		t.Fatal("metadata edit lost asset membership")
	}
	a = imageArticle(t, f.call("POST", path+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	if f.call("GET", url, "", false).Code != 404 {
		t.Fatal("private publication image leaked")
	}
	a = imageArticle(t, f.call("PATCH", path, fmt.Sprintf(`{"expected_version":%d,"visibility":"public"}`, a.Version), true))
	if f.call("GET", url, "", false).Code != 200 {
		t.Fatal("live public image missing")
	}
	head := f.call("HEAD", url, "", false)
	if head.Code != 200 || head.Body.Len() != 0 || head.Header().Get("Content-Length") != fmt.Sprint(len(png)) {
		t.Fatal("HEAD image response incorrect")
	}
	// Client validators must never bypass a fresh authorization query.
	req := httptest.NewRequest("GET", url, nil)
	req.Header.Set("If-None-Match", "*")
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, req)
	if w.Code != 200 || w.Header().Get("ETag") != "" {
		t.Fatal("cached validator path enabled")
	}
	next := testPNG(t, 100)
	job = f.upload(t, "next.zip", testZIP(t, zipEntry{"post.md", []byte("# New\n![next](new.png)"), 0}, zipEntry{"new.png", next, 0}), a)
	if job.Status != "succeeded" || !job.Publishable {
		t.Fatalf("next job: %+v", job)
	}
	a = imageArticle(t, f.call("GET", path, "", true))
	revision, err = loadRevision(f.ctx, f.db, a.Draft.ID)
	if err != nil || len(revision.Assets) != 1 {
		t.Fatal(err)
	}
	nextURL := assetURL(a.ID, revision.Assets[0].ID)
	if f.call("GET", nextURL, "", false).Code != 404 || f.call("GET", nextURL, "", true).Code != 200 || f.call("GET", url, "", false).Code != 200 {
		t.Fatal("draft/live image isolation failed")
	}
	a = imageArticle(t, f.call("POST", path+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	if f.call("GET", nextURL, "", false).Code != 200 || f.call("GET", url, "", false).Code != 404 || f.call("GET", url, "", true).Code != 200 {
		t.Fatal("retained/live image transition incorrect")
	}
	a = imageArticle(t, f.call("PATCH", path, fmt.Sprintf(`{"expected_version":%d,"visibility":"private"}`, a.Version), true))
	if f.call("GET", nextURL, "", false).Code != 404 {
		t.Fatal("privacy change left media public")
	}
	a = imageArticle(t, f.call("POST", path+"/unpublish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	if f.call("GET", nextURL, "", false).Code != 404 || f.call("GET", nextURL, "", true).Code != 200 {
		t.Fatal("unpublished media authorization failed")
	}
	// An invalid ZIP cannot replace the current draft or leave SQL asset rows.
	var before int
	f.db.QueryRowContext(f.ctx, "SELECT count(*) FROM assets").Scan(&before)
	failed := f.upload(t, "unsafe.zip", testZIP(t, zipEntry{"post.md", []byte("# Bad"), 0}, zipEntry{"../escape.png", png, 0}), a)
	after := imageArticle(t, f.call("GET", path, "", true))
	var count int
	f.db.QueryRowContext(f.ctx, "SELECT count(*) FROM assets").Scan(&count)
	if failed.Status != "failed" || after.Version != a.Version || after.Draft.ID != a.Draft.ID || count != before {
		t.Fatal("failed import changed article/assets")
	}
	// A route copied from another article never becomes trusted membership.
	foreign := f.upload(t, "forged.md", []byte("---\nwritten_at: 2026-11-07\n---\n# Forged\n![foreign]("+nextURL+")"), nil)
	if foreign.Status != "succeeded" || foreign.Publishable {
		t.Fatal("forged managed route allowed publication")
	}
	tampered := append([]byte(nil), png...)
	tampered[len(tampered)-1] ^= 255
	file, err := f.s.root.OpenFile(item.StorageKey, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(tampered); err != nil {
		t.Fatal(err)
	}
	file.Close()
	if f.call("GET", url, "", true).Code != 503 {
		t.Fatal("corrupt image bytes were served")
	}
	// Losing private bytes blocks publication and returns a generic storage error.
	if err := f.s.root.Remove(revision.Assets[0].StorageKey); err != nil {
		t.Fatal(err)
	}
	if publish := f.call("POST", path+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true); publish.Code != 422 {
		t.Fatalf("missing media publication: %d", publish.Code)
	}
	if f.call("GET", nextURL, "", true).Code != 503 {
		t.Fatal("missing image not reported safely")
	}
	// Revoked sessions lose retained-media access, including direct URL requests.
	if _, err := f.db.ExecContext(f.ctx, "UPDATE sessions SET revoked_at=clock_timestamp()"); err != nil {
		t.Fatal(err)
	}
	if f.call("GET", url, "", true).Code != 404 {
		t.Fatal("revoked session read retained image")
	}
}

//go:build integration

package publishing

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/IiCEight/iSaber/backend/internal/auth"
	"github.com/IiCEight/iSaber/backend/internal/database"
	"github.com/IiCEight/iSaber/backend/internal/httpapi"
	"github.com/IiCEight/iSaber/backend/internal/migrations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func publishingDatabase(t *testing.T) (*sql.DB, context.Context) {
	t.Helper()
	connection := os.Getenv("TEST_DATABASE_URL")
	if connection == "" {
		t.Fatal("TEST_DATABASE_URL is required for integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	admin, err := database.Open(connection)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	schema := fmt.Sprintf("isaber_publishing_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+quoted); err != nil {
		admin.Close()
		t.Fatal("cannot create test schema")
	}
	config, err := pgx.ParseConfig(connection)
	if err != nil {
		t.Fatal("invalid test connection configuration")
	}
	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	t.Cleanup(func() {
		db.Close()
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, err := admin.ExecContext(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error("could not clean up auth test schema")
		}
		admin.Close()
	})
	provider, err := migrations.Provider(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func TestPublishingHTTPFlow(t *testing.T) {
	db, ctx := publishingDatabase(t)
	store := auth.NewStore(db)
	password := "synthetic publishing test password"
	if err := store.Setup(ctx, "owner", password); err != nil {
		t.Fatal(err)
	}
	secret := bytes.Repeat([]byte{9}, 32)
	access, err := auth.NewService(store, secret)
	if err != nil {
		t.Fatal(err)
	}
	cfg := auth.Config{Origin: "https://blog.example", CookieName: "__Host-isaber_session", Secure: true, Secret: secret}
	directory := t.TempDir() + "/uploads"
	service, err := NewService(db, directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { service.Close() })
	handler := httpapi.NewHandler(func(context.Context) error { return nil }, httpapi.WithAuthentication(auth.NewHTTPHandler(access, cfg, service.AdminHandler())), httpapi.WithPublicArticles(service.PublicHandler()))
	var cookie *http.Cookie
	csrf := ""
	call := func(method, path, body, key string, authenticated bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", cfg.Origin)
		req.Header.Set("Idempotency-Key", key)
		if authenticated && cookie != nil {
			req.AddCookie(cookie)
			req.Header.Set("X-CSRF-Token", csrf)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	logged := call("POST", "/api/v1/auth/login", `{"username":"owner","password":"`+password+`"}`, "", false)
	if logged.Code != 200 {
		t.Fatalf("login: %d %s", logged.Code, logged.Body)
	}
	var session auth.Session
	json.Unmarshal(logged.Body.Bytes(), &session)
	csrf = session.CSRFToken
	cookie = logged.Result().Cookies()[0]
	upload := func(source, key string, article *Article) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, _ := writer.CreateFormFile("file", "journal.md")
		part.Write([]byte(source))
		if article != nil {
			writer.WriteField("article_id", article.ID)
			writer.WriteField("expected_version", fmt.Sprint(article.Version))
		}
		writer.Close()
		req := httptest.NewRequest("POST", "/api/v1/admin/imports", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Origin", cfg.Origin)
		req.Header.Set("X-CSRF-Token", csrf)
		req.Header.Set("Idempotency-Key", key)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	articleFrom := func(w *httptest.ResponseRecorder) *Article {
		t.Helper()
		if w.Code != 200 {
			t.Fatalf("article request: %d %s", w.Code, w.Body)
		}
		var a Article
		if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
			t.Fatal(err)
		}
		return &a
	}
	source := "---\ntitle: First 中文\nwritten_at: 2026-11-07\nlanguage: zh\nvisibility: public\n---\n# First\nOriginal text. $x^2$\n"
	key := uuid.NewString()
	accepted := upload(source, key, nil)
	if accepted.Code != 202 {
		t.Fatalf("accept: %d %s", accepted.Code, accepted.Body)
	}
	var job Job
	json.Unmarshal(accepted.Body.Bytes(), &job)
	replay := upload(source, key, nil)
	if replay.Code != 202 || replay.Body.String() != accepted.Body.String() {
		t.Fatal("upload retry created another acceptance")
	}
	if changed := upload(source+"changed", key, nil); changed.Code != 409 {
		t.Fatalf("changed upload key: %d %s", changed.Code, changed.Body)
	}
	// A queued upload survives reopening the filesystem root/service.
	service.Close()
	service, err = NewService(db, directory)
	if err != nil {
		t.Fatal(err)
	}
	handler = httpapi.NewHandler(nil, httpapi.WithAuthentication(auth.NewHTTPHandler(access, cfg, service.AdminHandler())), httpapi.WithPublicArticles(service.PublicHandler()))
	if claimed, err := service.ProcessNext(ctx); !claimed || err != nil {
		t.Fatalf("worker: %v %v", claimed, err)
	}
	job, err = service.GetJob(ctx, job.ID)
	if err != nil || job.Status != "succeeded" || !job.Publishable {
		t.Fatalf("job: %+v %v", job, err)
	}
	path := "/api/v1/admin/articles/" + *job.ArticleID
	if call("GET", path, "", "", false).Code != 401 {
		t.Fatal("anonymous draft access")
	}
	a := articleFrom(call("GET", path, "", "", true))
	if a.Visibility != "private" || a.Published != nil || a.Draft == nil || !a.Publishable {
		t.Fatalf("draft-first rules: %+v", a)
	}
	publicPath := "/api/v1/articles/" + *a.Slug
	if call("GET", publicPath, "", "", true).Code != 404 {
		t.Fatal("owner cookie exposed private content on public endpoint")
	}
	originalRevision := a.Draft.ID
	payload := fmt.Sprintf(`{"expected_version":%d}`, a.Version)
	publishKey := uuid.NewString()
	published := call("POST", path+"/publish", payload, publishKey, true)
	a = articleFrom(published)
	if replay := call("POST", path+"/publish", payload, publishKey, true); replay.Code != 200 || replay.Body.String() != published.Body.String() {
		t.Fatal("publish retry was not stable")
	}
	if call("GET", publicPath, "", "", false).Code != 404 {
		t.Fatal("private publication leaked")
	}
	a = articleFrom(call("PATCH", path, fmt.Sprintf(`{"expected_version":%d,"visibility":"public"}`, a.Version), "", true))
	live := call("GET", publicPath, "", "", false)
	if live.Code != 200 || !strings.Contains(live.Body.String(), "Original text") || live.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("public read: %d %s", live.Code, live.Body)
	}
	firstDate := *a.PublishedAt
	update := upload("# Revised\nUnpublished secret draft.", uuid.NewString(), a)
	if update.Code != 202 {
		t.Fatalf("update: %d %s", update.Code, update.Body)
	}
	if _, err := service.ProcessNext(ctx); err != nil {
		t.Fatal(err)
	}
	a = articleFrom(call("GET", path, "", "", true))
	if a.Draft == nil || a.Published.ID != originalRevision || *a.Draft.WrittenAt != "2026-11-07" {
		t.Fatal("update replaced live snapshot or lost writing date")
	}
	if live := call("GET", publicPath, "", "", false); strings.Contains(live.Body.String(), "Unpublished secret") || !strings.Contains(live.Body.String(), "Original text") {
		t.Fatal("draft leaked to public")
	}
	if stale := call("PATCH", path+"/draft", `{"expected_version":1,"title":"stale"}`, "", true); stale.Code != 409 {
		t.Fatalf("stale update: %d %s", stale.Code, stale.Body)
	}
	if slug := call("PATCH", path+"/draft", fmt.Sprintf(`{"expected_version":%d,"slug":"new-url"}`, a.Version), "", true); slug.Code != 409 {
		t.Fatalf("locked slug: %d %s", slug.Code, slug.Body)
	}
	if invalid := call("PATCH", path+"/draft", fmt.Sprintf(`{"expected_version":%d,"written_at":"0000-01-01"}`, a.Version), "", true); invalid.Code != 422 {
		t.Fatalf("invalid date: %d", invalid.Code)
	}
	// A racing accepted import cannot overwrite a subsequent metadata edit.
	staleImport := upload("# Stale file", uuid.NewString(), a)
	if staleImport.Code != 202 {
		t.Fatal(staleImport.Body)
	}
	a = articleFrom(call("PATCH", path+"/draft", fmt.Sprintf(`{"expected_version":%d,"title":"Reviewed title"}`, a.Version), "", true))
	service.ProcessNext(ctx)
	var staleJob Job
	json.Unmarshal(staleImport.Body.Bytes(), &staleJob)
	staleJob, _ = service.GetJob(ctx, staleJob.ID)
	if staleJob.Status != "failed" {
		t.Fatal("stale import overwrote draft")
	}
	a = articleFrom(call("POST", path+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), uuid.NewString(), true))
	if !a.PublishedAt.Equal(firstDate) || a.Draft != nil || a.Published.ID == originalRevision {
		t.Fatal("revision publication/date transition incorrect")
	}
	a = articleFrom(call("PATCH", path, fmt.Sprintf(`{"expected_version":%d,"visibility":"private"}`, a.Version), "", true))
	if call("GET", publicPath, "", "", false).Code != 404 {
		t.Fatal("privacy change did not hide publication")
	}
	a = articleFrom(call("POST", path+"/unpublish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), uuid.NewString(), true))
	if a.Published != nil || a.Draft == nil {
		t.Fatal("unpublish did not retain draft")
	}
	blocked := upload("# Images\n![missing](./private.png)", uuid.NewString(), a)
	if blocked.Code != 202 {
		t.Fatal(blocked.Body)
	}
	service.ProcessNext(ctx)
	a = articleFrom(call("GET", path, "", "", true))
	if a.Publishable || call("POST", path+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), uuid.NewString(), true).Code != 422 {
		t.Fatal("missing local image allowed publication")
	}
	invalidImport := upload(string([]byte{0xff}), uuid.NewString(), a)
	service.ProcessNext(ctx)
	var invalidJob Job
	json.Unmarshal(invalidImport.Body.Bytes(), &invalidJob)
	invalidJob, _ = service.GetJob(ctx, invalidJob.ID)
	after := articleFrom(call("GET", path, "", "", true))
	if invalidJob.Status != "failed" || after.Version != a.Version || after.Draft.ID != a.Draft.ID {
		t.Fatal("failed import changed article")
	}
	if list := call("GET", "/api/v1/articles", "", "", false); list.Code != 200 || strings.Contains(list.Body.String(), a.ID) {
		t.Fatal("private article listed publicly")
	}
	badMetadata := upload("---\nlanguage: de\nsummary: "+strings.Repeat("x", 2001)+"\n---\n# Metadata", uuid.NewString(), a)
	if badMetadata.Code != 202 {
		t.Fatal(badMetadata.Body)
	}
	service.ProcessNext(ctx)
	a = articleFrom(call("GET", path, "", "", true))
	inherited := upload("# Keep unresolved metadata", uuid.NewString(), a)
	if inherited.Code != 202 {
		t.Fatal(inherited.Body)
	}
	service.ProcessNext(ctx)
	a = articleFrom(call("GET", path, "", "", true))
	found := map[string]bool{}
	for _, item := range a.Issues {
		found[item.Code] = true
	}
	if a.Publishable || a.Draft.Language != nil || !found["invalid_language"] || !found["metadata_too_long"] {
		t.Fatalf("replacement forgot inherited invalid metadata: %+v", a.Issues)
	}

	a = articleFrom(call("PATCH", path+"/draft", fmt.Sprintf(`{"expected_version":%d,"language":"en","summary":null}`, a.Version), "", true))
	if !a.Publishable {
		t.Fatalf("explicit metadata correction did not clear issues: %+v", a.Issues)
	}
	if missing := call("GET", path+"/not-implemented", "", "", true); missing.Code != 404 || missing.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatal("unknown admin route lost problem format")
	}
	if method := call("DELETE", path, "", "", true); method.Code != 405 || method.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatal("unsupported method lost problem format")
	}
	savedCSRF := csrf
	csrf = ""
	if call("PATCH", path, fmt.Sprintf(`{"expected_version":%d,"visibility":"public"}`, a.Version), "", true).Code != 403 {
		t.Fatal("mutation accepted without CSRF")
	}
	csrf = savedCSRF
}

func TestConcurrentIdempotencyAndRollback(t *testing.T) {
	db, ctx := publishingDatabase(t)
	s, err := NewService(db, t.TempDir()+"/uploads")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	key := uuid.NewString()
	entered, release := make(chan struct{}), make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		_, err := s.idempotent(ctx, "concurrency", key, []byte("same payload"), func(*sql.Tx) (int, any, error) {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return 0, nil, ctx.Err()
			}
			return 202, map[string]bool{"accepted": true}, nil
		})
		finished <- err
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("claim timed out")
	}
	_, concurrentErr := s.idempotent(ctx, "concurrency", key, []byte("same payload"), func(*sql.Tx) (int, any, error) { t.Error("concurrent action ran twice"); return 200, nil, nil })
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	var problem *Fault
	if !errors.As(concurrentErr, &problem) || problem.Code != "request_in_progress" {
		t.Fatalf("concurrent request: %v", concurrentErr)
	}
	replayed, err := s.idempotent(ctx, "concurrency", key, []byte("same payload"), func(*sql.Tx) (int, any, error) { t.Error("replay ran action"); return 200, nil, nil })
	if err != nil || !replayed.Replayed || replayed.Status != 202 {
		t.Fatalf("replay: %+v %v", replayed, err)
	}
	_, err = s.idempotent(ctx, "concurrency", key, []byte("different payload"), func(*sql.Tx) (int, any, error) { return 200, nil, nil })
	if !errors.As(err, &problem) || problem.Code != "idempotency_conflict" {
		t.Fatalf("changed payload: %v", err)
	}
	rollbackKey := uuid.NewString()
	_, err = s.idempotent(ctx, "rollback", rollbackKey, []byte("payload"), func(*sql.Tx) (int, any, error) { return 0, nil, fault(422, "invalid") })
	if err == nil {
		t.Fatal("validation error lost")
	}
	committed, err := s.idempotent(ctx, "rollback", rollbackKey, []byte("payload"), func(*sql.Tx) (int, any, error) { return 200, map[string]bool{"fixed": true}, nil })
	if err != nil || committed.Replayed {
		t.Fatal("rolled-back key prevented corrected retry")
	}
}

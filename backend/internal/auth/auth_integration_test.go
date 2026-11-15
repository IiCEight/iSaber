//go:build integration

package auth

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/IiCEight/iSaber/backend/internal/database"
	"github.com/IiCEight/iSaber/backend/internal/httpapi"
	"github.com/IiCEight/iSaber/backend/internal/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

const fixturePassword = "a synthetic owner passphrase"

func authDatabase(t *testing.T) (*sql.DB, context.Context) {
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
	schema := fmt.Sprintf("isaber_auth_test_%d", time.Now().UnixNano())
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

func fixture(t *testing.T) (*Service, *Store, http.Handler, Config, context.Context) {
	t.Helper()
	db, ctx := authDatabase(t)
	store := NewStore(db)
	if err := store.Setup(ctx, "owner", fixturePassword); err != nil {
		t.Fatal(err)
	}
	config := Config{Origin: "https://blog.example", CookieName: "__Host-isaber_session", Secure: true, Secret: bytes.Repeat([]byte{7}, 32)}
	service, err := NewService(store, config.Secret)
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.NewHandler(func(ctx context.Context) error { return migrations.CheckReady(ctx, db) }, httpapi.WithAuthentication(NewHTTPHandler(service, config)))
	return service, store, handler, config, ctx
}

func call(handler http.Handler, method, path, origin, csrf string, cookie *http.Cookie, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Origin", origin)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", csrf)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func login(t *testing.T, handler http.Handler, config Config) (Session, *http.Cookie) {
	t.Helper()
	response := call(handler, "POST", "/api/v1/auth/login", config.Origin, "", nil,
		`{"username":"owner","password":"`+fixturePassword+`"}`)
	if response.Code != 200 {
		t.Fatalf("login status: %d", response.Code)
	}
	var session Session
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("expected one session cookie")
	}
	if bytes.Contains(response.Body.Bytes(), []byte(cookies[0].Value)) {
		t.Fatal("session bearer token leaked in JSON")
	}
	return session, cookies[0]
}

func TestOwnerHTTPFlowAndCSRF(t *testing.T) {
	_, store, handler, config, ctx := fixture(t)
	if err := store.Setup(ctx, "second-owner", fixturePassword); !errors.Is(err, ErrOwnerExists) {
		t.Fatal("setup replaced or duplicated the existing owner")
	}
	for _, credentials := range []string{`{"username":"owner","password":"wrong password"}`, `{"username":"missing","password":"wrong password"}`} {
		if response := call(handler, "POST", "/api/v1/auth/login", config.Origin, "", nil, credentials); response.Code != 401 {
			t.Fatal("invalid credentials did not produce a generic 401")
		}
	}
	session, cookie := login(t, handler, config)
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Domain != "" || cookie.Path != "/" || cookie.MaxAge != int(AbsoluteLifetime.Seconds()) {
		t.Fatal("unsafe production cookie attributes")
	}
	if session.Owner.Username != "owner" || session.CSRFToken == "" {
		t.Fatal("invalid session response")
	}
	if remaining := time.Until(session.ExpiresAt); remaining < AbsoluteLifetime-time.Minute || remaining > AbsoluteLifetime+time.Minute {
		t.Fatal("stored session lifetime differs from the configured cookie lifetime")
	}
	duplicate := httptest.NewRequest("GET", "/api/v1/auth/session", nil)
	duplicate.AddCookie(cookie)
	duplicate.AddCookie(cookie)
	duplicateResponse := httptest.NewRecorder()
	handler.ServeHTTP(duplicateResponse, duplicate)
	if duplicateResponse.Code != 401 {
		t.Fatal("ambiguous duplicate session cookies were accepted")
	}
	wrongNamespace := &http.Cookie{Name: "isaber_dev_session", Value: cookie.Value}
	if call(handler, "GET", "/api/v1/auth/session", "", "", wrongNamespace, "").Code != 401 {
		t.Fatal("production trusted an insecure development cookie")
	}
	var tokenHash, csrfHash []byte
	if err := store.db.QueryRowContext(ctx, "SELECT token_hash,csrf_token_hash FROM sessions").Scan(&tokenHash, &csrfHash); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(tokenHash, digest(cookie.Value)) || !bytes.Equal(csrfHash, digest(session.CSRFToken)) {
		t.Fatal("session secrets were not stored as hashes")
	}
	for _, test := range []struct {
		method, path, origin, csrf string
		cookie                     *http.Cookie
		status                     int
	}{
		{"GET", "/api/v1/auth/session", "", "", cookie, 200},
		{"GET", "/api/v1/admin/articles", "", "", nil, 401},
		{"GET", "/api/v1/admin/articles", "", "", cookie, 404},
		{"POST", "/api/v1/admin/imports", config.Origin, "", cookie, 403},
		{"POST", "/api/v1/admin/imports", config.Origin, session.CSRFToken, cookie, 404},
		{"POST", "/api/v1/auth/logout", "https://other.example", session.CSRFToken, cookie, 403},
		{"POST", "/api/v1/auth/logout", config.Origin, "wrong-token", cookie, 403},
	} {
		response := call(handler, test.method, test.path, test.origin, test.csrf, test.cookie, "")
		if response.Code != test.status || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s %s: status %d, expected %d", test.method, test.path, response.Code, test.status)
		}
	}
	response := call(handler, "POST", "/api/v1/auth/logout", config.Origin, session.CSRFToken, cookie, "")
	if response.Code != 204 || response.Body.Len() != 0 || response.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatal("logout did not revoke and clear the session")
	}
	if response := call(handler, "GET", "/api/v1/auth/session", "", "", cookie, ""); response.Code != 401 {
		t.Fatal("revoked cookie still authenticated")
	}
}

func TestRecoveryExpiryAndKeyRotation(t *testing.T) {
	service, store, _, config, ctx := fixture(t)
	_, oldToken, err := service.Login(ctx, "OWNER", fixturePassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(ctx, "owner", "a replacement owner passphrase"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, oldToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("recovery did not invalidate existing sessions")
	}
	if _, _, err := service.Login(ctx, "owner", fixturePassword); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("old password still worked after recovery")
	}
	_, token, err := service.Login(ctx, "owner", "a replacement owner passphrase")
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := NewService(store, config.Secret)
	if err != nil {
		t.Fatal(err)
	}
	if session, err := restarted.Authenticate(ctx, token); err != nil || session.CSRFToken != service.csrf(token) {
		t.Fatal("session did not survive a service restart with the same key")
	}
	rotated, err := NewService(store, bytes.Repeat([]byte{8}, 32))
	if err != nil {
		t.Fatal(err)
	}
	var before, after time.Time
	if err := store.db.QueryRowContext(ctx, "SELECT last_seen_at FROM sessions WHERE token_hash=$1", digest(token)).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := rotated.Authenticate(ctx, token); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("CSRF key rotation did not invalidate old sessions")
	}
	if err := store.db.QueryRowContext(ctx, "SELECT last_seen_at FROM sessions WHERE token_hash=$1", digest(token)).Scan(&after); err != nil || !before.Equal(after) {
		t.Fatal("a rejected session renewed its idle deadline")
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE sessions SET last_seen_at=now()-interval '31 minutes' WHERE token_hash=$1", digest(token)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, token); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("idle session still authenticated")
	}
	_, token, err = service.Login(ctx, "owner", "a replacement owner passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE sessions SET created_at=now()-interval '9 hours',expires_at=now()-interval '1 minute' WHERE token_hash=$1", digest(token)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, token); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("absolute session expiry was not enforced")
	}
}

func TestLoginOriginValidationAndRateLimit(t *testing.T) {
	_, _, handler, config, _ := fixture(t)
	body := `{"username":"owner","password":"wrong password"}`
	if response := call(handler, "POST", "/api/v1/auth/login", "", "", nil, body); response.Code != 403 {
		t.Fatal("missing Origin was accepted")
	}
	if response := call(handler, "POST", "/api/v1/auth/login", config.Origin, "", nil, body+" {}"); response.Code != 400 {
		t.Fatal("multiple JSON payloads were accepted")
	}
	if response := call(handler, "POST", "/api/v1/auth/login", config.Origin, "", nil, `{"username":"owner","password":"x","extra":true}`); response.Code != 400 {
		t.Fatal("unknown fields were accepted")
	}
	for _, value := range []string{`null`, `"true"`, `1`, `{}`, `[]`} {
		payload := `{"username":"owner","password":"x","remember_me":` + value + `}`
		if response := call(handler, "POST", "/api/v1/auth/login", config.Origin, "", nil, payload); response.Code != 400 {
			t.Fatalf("Invalid remember_me accepted: %s", value)
		}
	}
	for range 5 {
		if response := call(handler, "POST", "/api/v1/auth/login", config.Origin, "", nil, body); response.Code != 401 {
			t.Fatalf("unexpected login status %d", response.Code)
		}
	}
	response := call(handler, "POST", "/api/v1/auth/login", config.Origin, "", nil, body)
	if response.Code != 429 || response.Header().Get("Retry-After") == "" {
		t.Fatal("repeated login attempts were not rate limited")
	}
}

func TestRememberedSessionSurvivesAbsenceAndRestartButRemainsRevocable(t *testing.T) {
	_, store, handler, config, ctx := fixture(t)
	signIn := func(remember bool) (Session, *http.Cookie) {
		t.Helper()
		payload := fmt.Sprintf(`{"username":"owner","password":%q,"remember_me":%t}`, fixturePassword, remember)
		response := call(handler, "POST", "/api/v1/auth/login", config.Origin, "", nil, payload)
		var session Session
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &session) != nil || len(response.Result().Cookies()) != 1 {
			t.Fatalf("Remembered login failed: %d", response.Code)
		}
		cookie := response.Result().Cookies()[0]
		if cookie.MaxAge != int(sessionLifetime(remember).Seconds()) || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Domain != "" {
			t.Fatal("Remembered login changed cookie protections or lifetime")
		}
		if time.Until(session.ExpiresAt) < sessionLifetime(remember)-time.Minute || time.Until(session.ExpiresAt) > sessionLifetime(remember)+time.Minute || session.ExpiresAt.Sub(cookie.Expires) >= time.Second {
			t.Fatal("Browser cookie and stored expiry disagree")
		}
		var persisted bool
		if err := store.db.QueryRowContext(ctx, `SELECT remember_me FROM sessions WHERE token_hash=$1`, digest(cookie.Value)).Scan(&persisted); err != nil || persisted != remember {
			t.Fatal("Login preference was not persisted")
		}
		if bytes.Contains(response.Body.Bytes(), []byte(cookie.Value)) {
			t.Fatal("Session bearer token leaked in JSON")
		}
		return session, cookie
	}
	_, short := signIn(false)
	_, remembered := signIn(true)
	if _, err := store.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at=now()-interval '31 minutes' WHERE token_hash=$1`, digest(short.Value)); err != nil {
		t.Fatal(err)
	}
	if response := call(handler, "GET", "/api/v1/auth/session", "", "", short, ""); response.Code != 401 {
		t.Fatal("Unchecking remember me removed the short-session idle limit")
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE sessions SET created_at=now()-interval '3 days',last_seen_at=now()-interval '3 days',expires_at=now()+interval '27 days' WHERE token_hash=$1`, digest(remembered.Value)); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewService(store, config.Secret)
	if err != nil {
		t.Fatal(err)
	}
	var fixedExpiry time.Time
	if err := store.db.QueryRowContext(ctx, `SELECT expires_at FROM sessions WHERE token_hash=$1`, digest(remembered.Value)).Scan(&fixedExpiry); err != nil {
		t.Fatal(err)
	}
	session, err := restarted.Authenticate(ctx, remembered.Value)
	if err != nil || !session.ExpiresAt.Equal(fixedExpiry) {
		t.Fatal("Remembered session did not survive absence/restart or its expiry was extended")
	}
	rotated, err := NewService(store, bytes.Repeat([]byte{8}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rotated.Authenticate(ctx, remembered.Value); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("Remembered session bypassed key rotation")
	}
	if response := call(handler, "POST", "/api/v1/auth/logout", config.Origin, "wrong-token", remembered, ""); response.Code != 403 {
		t.Fatal("Remembered session bypassed logout CSRF validation")
	}
	if response := call(handler, "POST", "/api/v1/auth/logout", config.Origin, session.CSRFToken, remembered, ""); response.Code != 204 || response.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatal("Remembered session was not revoked and cleared on logout")
	}
	if _, err := restarted.Authenticate(ctx, remembered.Value); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("Logged-out remembered session still authenticated")
	}
	_, expired := signIn(true)
	if _, err := store.db.ExecContext(ctx, `UPDATE sessions SET created_at=now()-interval '31 days',expires_at=now()-interval '1 minute' WHERE token_hash=$1`, digest(expired.Value)); err != nil {
		t.Fatal(err)
	}
	if response := call(handler, "GET", "/api/v1/auth/session", "", "", expired, ""); response.Code != 401 || response.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatal("Remembered session exceeded its absolute expiry")
	}
	_, recovered := signIn(true)
	if err := store.Recover(ctx, "owner", "a new synthetic owner passphrase"); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Authenticate(ctx, recovered.Value); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("Owner recovery did not revoke remembered sessions")
	}
}

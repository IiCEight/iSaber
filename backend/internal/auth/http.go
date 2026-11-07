package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"sync"
	"time"
)

type loginLimiter struct {
	mu       sync.Mutex
	start    time.Time
	attempts int
}

func (l *loginLimiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if time.Since(l.start) >= time.Minute {
		l.start, l.attempts = time.Now(), 0
	}
	if l.attempts >= 5 {
		return false
	}
	l.attempts++
	return true
}

type httpHandler struct {
	service    *Service
	config     Config
	limiter    loginLimiter
	slots      chan struct{}
	management http.Handler
}

func NewHTTPHandler(service *Service, config Config, management ...http.Handler) http.Handler {
	handler := &httpHandler{service: service, config: config, slots: make(chan struct{}, 2)}
	if len(management) > 0 {
		handler.management = management[0]
	}
	return handler
}

type ownerContextKey struct{}

func OwnerFromContext(ctx context.Context) (Owner, bool) {
	owner, ok := ctx.Value(ownerContextKey{}).(Owner)
	return owner, ok
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	original := r
	w.Header().Set("Cache-Control", "no-store")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	switch r.URL.Path {
	case "/api/v1/auth/login":
		if !method(w, r, http.MethodPost) || !h.origin(w, r) {
			return
		}
		h.login(w, r)
	case "/api/v1/auth/session":
		if !method(w, r, http.MethodGet) {
			return
		}
		session, _, ok := h.authenticate(w, r)
		if ok {
			writeJSON(w, http.StatusOK, session)
		}
	case "/api/v1/auth/logout":
		if !method(w, r, http.MethodPost) || !h.origin(w, r) {
			return
		}
		session, token, ok := h.authenticate(w, r)
		if !ok || !csrf(w, r, session.CSRFToken) {
			return
		}
		if err := h.service.Logout(r.Context(), token); err != nil {
			authProblem(w, 503, "authentication_unavailable")
			return
		}
		h.cookie(w, "", time.Unix(1, 0), -1)
		w.WriteHeader(http.StatusNoContent)
	default:
		// Every management endpoint passes this session/origin/CSRF boundary.
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions && !h.origin(w, r) {
			return
		}
		session, _, ok := h.authenticate(w, r)
		if !ok {
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions && !csrf(w, r, session.CSRFToken) {
			return
		}
		if h.management != nil {
			h.management.ServeHTTP(w, original.WithContext(context.WithValue(original.Context(), ownerContextKey{}, session.Owner)))
			return
		}
		authProblem(w, 404, "not_found")
	}
}

func (h *httpHandler) login(w http.ResponseWriter, r *http.Request) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		authProblem(w, 415, "unsupported_media_type")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	decoder := json.NewDecoder(r.Body)
	var fields map[string]json.RawMessage
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decoder.Decode(&fields); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			authProblem(w, 413, "request_too_large")
		} else {
			authProblem(w, 400, "invalid_json")
		}
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			authProblem(w, 413, "request_too_large")
		} else {
			authProblem(w, 400, "invalid_json")
		}
		return
	}
	if len(fields) != 2 || json.Unmarshal(fields["username"], &input.Username) != nil || json.Unmarshal(fields["password"], &input.Password) != nil {
		authProblem(w, 400, "invalid_credentials_payload")
		return
	}
	username, err := normalizeUsername(input.Username)
	if err != nil || !validPassword(input.Password, 1) {
		authProblem(w, 422, "invalid_credentials")
		return
	}
	if !h.limiter.allow() {
		w.Header().Set("Retry-After", "60")
		authProblem(w, 429, "login_rate_limited")
		return
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		w.Header().Set("Retry-After", "1")
		authProblem(w, 429, "login_busy")
		return
	}
	session, token, err := h.service.Login(r.Context(), username, input.Password)
	if errors.Is(err, ErrUnauthorized) {
		authProblem(w, 401, "unauthorized")
		return
	}
	if err != nil {
		authProblem(w, 503, "authentication_unavailable")
		return
	}
	h.cookie(w, token, session.ExpiresAt, int(AbsoluteLifetime.Seconds()))
	writeJSON(w, http.StatusOK, session)
}

func (h *httpHandler) authenticate(w http.ResponseWriter, r *http.Request) (Session, string, bool) {
	var token string
	count := 0
	for _, cookie := range r.Cookies() {
		if cookie.Name == h.config.CookieName {
			token, count = cookie.Value, count+1
		}
	}
	if count != 1 || len(token) > 128 {
		authProblem(w, 401, "unauthorized")
		return Session{}, "", false
	}
	session, err := h.service.Authenticate(r.Context(), token)
	if errors.Is(err, ErrUnauthorized) {
		h.cookie(w, "", time.Unix(1, 0), -1)
		authProblem(w, 401, "unauthorized")
		return Session{}, "", false
	}
	if err != nil {
		authProblem(w, 503, "authentication_unavailable")
		return Session{}, "", false
	}
	return session, token, true
}

func (h *httpHandler) cookie(w http.ResponseWriter, token string, expires time.Time, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: h.config.CookieName, Value: token, Path: "/", Secure: h.config.Secure,
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: maxAge})
}

func (h *httpHandler) origin(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Origin") != h.config.Origin {
		authProblem(w, 403, "origin_rejected")
		return false
	}
	return true
}

func csrf(w http.ResponseWriter, r *http.Request, expected string) bool {
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(expected)) != 1 {
		authProblem(w, 403, "csrf_rejected")
		return false
	}
	return true
}

func method(w http.ResponseWriter, r *http.Request, expected string) bool {
	if r.Method != expected {
		w.Header().Set("Allow", expected)
		authProblem(w, 405, "method_not_allowed")
		return false
	}
	return true
}

func authProblem(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": http.StatusText(status),
		"status": status, "code": code, "request_id": w.Header().Get("X-Request-ID")})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

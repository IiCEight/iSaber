// Package httpapi provides the HTTP entry point. Business endpoints will be
// implemented against contracts/openapi.yaml in subsequent milestones.
package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type problem struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Code      string `json:"code"`
	RequestID string `json:"request_id"`
}

type options struct {
	authentication http.Handler
	public         http.Handler
}
type Option func(*options)

func WithAuthentication(handler http.Handler) Option {
	return func(config *options) { config.authentication = handler }
}
func WithPublicArticles(handler http.Handler) Option {
	return func(config *options) { config.public = handler }
}

// NewHandler exposes probes and configured authentication. Unimplemented business
// endpoints remain unavailable; owner operations pass through authorization.
func NewHandler(checkReady func(context.Context) error, opts ...Option) http.Handler {
	config := options{}
	for _, option := range opts {
		option(&config)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := rand.Text()
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if config.authentication != nil && (strings.HasPrefix(r.URL.Path, "/api/v1/auth/") ||
			r.URL.Path == "/api/v1/admin" || strings.HasPrefix(r.URL.Path, "/api/v1/admin/")) {
			config.authentication.ServeHTTP(w, r)
			return
		}
		if config.public != nil && (r.URL.Path == "/api/v1/articles" || strings.HasPrefix(r.URL.Path, "/api/v1/articles/")) {
			config.public.ServeHTTP(w, r)
			return
		}

		switch r.URL.Path {
		case "/healthz", "/readyz":
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", requestID)
				return
			}
			if r.URL.Path == "/readyz" {
				ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
				defer cancel()
				if checkReady == nil || checkReady(ctx) != nil {
					writeProblem(w, http.StatusServiceUnavailable, "not_ready", requestID)
					return
				}
			}
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		default:
			writeProblem(w, http.StatusNotFound, "not_found", requestID)
		}
	})
}

func writeProblem(w http.ResponseWriter, status int, code, requestID string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem{
		Type: "about:blank", Title: http.StatusText(status),
		Status: status, Code: code, RequestID: requestID,
	})
}

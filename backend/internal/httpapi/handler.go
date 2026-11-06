// Package httpapi provides the HTTP entry point. Business endpoints will be
// implemented against contracts/openapi.yaml in subsequent milestones.
package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"time"
)

type problem struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Code      string `json:"code"`
	RequestID string `json:"request_id"`
}

// NewHandler exposes operational probes only. Unimplemented business routes
// return a problem response instead of pretending to serve blog content.
func NewHandler(checkReady func(context.Context) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := rand.Text()
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")

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

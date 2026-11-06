package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbesAndUnimplementedRoutes(t *testing.T) {
	tests := []struct {
		name, method, path, contentType, code string
		status                                int
	}{
		{"liveness", "GET", "/healthz", "application/json", "", 200},
		{"readiness", "GET", "/readyz", "application/json", "", 200},
		{"probe method", "POST", "/healthz", "application/problem+json", "method_not_allowed", 405},
		{"unknown article", "GET", "/api/v1/articles/missing", "application/problem+json", "not_found", 404},
		{"unimplemented owner action", "POST", "/api/v1/admin/imports", "application/problem+json", "not_found", 404},
	}
	handler := NewHandler(func(context.Context) error { return nil })
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, httptest.NewRequest(tt.method, tt.path, nil))
			if rr.Code != tt.status {
				t.Fatalf("status = %d, want %d", rr.Code, tt.status)
			}
			if rr.Header().Get("Content-Type") != tt.contentType {
				t.Fatalf("unexpected content type: %s", rr.Header().Get("Content-Type"))
			}
			if rr.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("response must not be cached")
			}
			if tt.code != "" {
				var p problem
				if err := json.Unmarshal(rr.Body.Bytes(), &p); err != nil {
					t.Fatal(err)
				}
				if p.Code != tt.code || p.Status != tt.status || p.RequestID == "" || p.RequestID != rr.Header().Get("X-Request-ID") {
					t.Fatalf("invalid problem response: %+v", p)
				}
			}
			if tt.status == http.StatusMethodNotAllowed && rr.Header().Get("Allow") != "GET, HEAD" {
				t.Fatal("method error must advertise allowed methods")
			}
		})
	}
}

func TestHeadProbeHasNoBody(t *testing.T) {
	rr := httptest.NewRecorder()
	NewHandler(nil).ServeHTTP(rr, httptest.NewRequest("HEAD", "/healthz", nil))
	if rr.Code != http.StatusOK || rr.Body.Len() != 0 {
		t.Fatalf("HEAD returned status %d and %d body bytes", rr.Code, rr.Body.Len())
	}
}

func TestDependencyFailureAffectsOnlyReadiness(t *testing.T) {
	handler := NewHandler(func(ctx context.Context) error {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("readiness must have a deadline")
		}
		return errors.New("sensitive database diagnostic")
	})
	for _, path := range []string{"/healthz", "/readyz"} {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest("GET", path, nil))
		if path == "/healthz" && rr.Code != 200 {
			t.Fatal("dependency failures must not fail liveness")
		}
		if path == "/readyz" {
			if rr.Code != 503 {
				t.Fatalf("readiness status = %d", rr.Code)
			}
			var p problem
			if err := json.Unmarshal(rr.Body.Bytes(), &p); err != nil || p.Code != "not_ready" {
				t.Fatalf("unexpected readiness response: %s", rr.Body.String())
			}
			if rr.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("readiness failures must not be cached")
			}
			if strings.Contains(rr.Body.String(), "sensitive") {
				t.Fatal("readiness leaked a dependency diagnostic")
			}
		}
	}
}

func TestMissingReadinessCheckIsNotReady(t *testing.T) {
	rr := httptest.NewRecorder()
	NewHandler(nil).ServeHTTP(rr, httptest.NewRequest("GET", "/readyz", nil))
	if rr.Code != 503 {
		t.Fatalf("missing readiness dependency returned %d", rr.Code)
	}
}

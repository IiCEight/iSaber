package publishing

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/IiCEight/iSaber/backend/internal/auth"
)

type ownerReadingKey struct{}

// This scope is attached only to protected reader routes, never public routes.
func ownerReading(ctx context.Context) bool {
	_, authenticated := auth.OwnerFromContext(ctx)
	return authenticated && ctx.Value(ownerReadingKey{}) == true
}

func (s *Service) readingRoutes(mux *http.ServeMux) {
	protect := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if _, ok := auth.OwnerFromContext(r.Context()); !ok {
				reply(w, nil, fault(401, "unauthorized"))
				return
			}
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			next(w, r.WithContext(context.WithValue(ctx, ownerReadingKey{}, true)))
		}
	}
	s.categoryRoutes(mux, "/api/v1/admin/reading", protect)
	mux.HandleFunc("GET /api/v1/admin/reading/articles", protect(s.listPublic))
	mux.HandleFunc("GET /api/v1/admin/reading/articles/search", protect(s.searchPublic))
	mux.HandleFunc("GET /api/v1/admin/reading/articles/{slug}", protect(func(w http.ResponseWriter, r *http.Request) {
		article, err := s.publicArticle(r.Context(), r.PathValue("slug"))
		reply(w, article, err)
	}))
	for _, kind := range []string{"categories", "tags"} {
		mux.HandleFunc("GET /api/v1/admin/reading/"+kind, protect(func(w http.ResponseWriter, r *http.Request) {
			s.listLabels(w, r, kind, false)
		}))
	}
}

func readingCollectionSQL(ctx context.Context) string {
	if ownerReading(ctx) {
		return publishedCollectionSQL
	}
	return publicCollectionSQL
}

func readingSearchSQL(ctx context.Context) string {
	if ownerReading(ctx) {
		return publishedSearchSQL
	}
	return publicSearchSQL
}

func readingPageOptions(ctx context.Context, values url.Values) (int, *cursor, string, error) {
	if !ownerReading(ctx) {
		return pageOptions(values)
	}
	return scopedPageOptions(values, "owner-reading")
}

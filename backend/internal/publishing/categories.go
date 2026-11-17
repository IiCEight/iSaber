package publishing

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"
)

type categoryReadingKey struct{}
type categoryPage struct {
	Label
	DescriptionEN string `json:"description_en"`
	DescriptionZH string `json:"description_zh"`
	Total         int64  `json:"total"`
}

func categoryDescriptions(kind string, fields map[string]json.RawMessage) (map[string]string, error) {
	values := map[string]string{}
	for key, raw := range fields {
		if key == "description_en" || key == "description_zh" {
			var value string
			if kind != "categories" || string(raw) == "null" || json.Unmarshal(raw, &value) != nil || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 2000 || strings.ContainsRune(value, 0) {
				return nil, fault(422, "invalid_category_description")
			}
			values[key] = value
		}
	}
	return values, nil
}
func (s *Service) categoryRoutes(mux *http.ServeMux, prefix string, protect func(http.HandlerFunc) http.HandlerFunc) {
	wrap := func(h http.HandlerFunc) http.HandlerFunc {
		if protect != nil {
			return protect(h)
		}
		return h
	}
	mux.HandleFunc("GET "+prefix+"/categories/{slug}", wrap(func(w http.ResponseWriter, r *http.Request) {
		category, err := s.readCategory(r.Context(), r.PathValue("slug"))
		reply(w, category, err)
	}))
	mux.HandleFunc("GET "+prefix+"/categories/{slug}/articles", wrap(func(w http.ResponseWriter, r *http.Request) {
		category, err := s.readCategory(r.Context(), r.PathValue("slug"))
		if err != nil {
			reply(w, nil, err)
			return
		}
		values := r.URL.Query()
		if values.Has("category") || values.Has("cursor") {
			reply(w, nil, fault(400, "invalid_category_parameters"))
			return
		}
		values.Set("category", category.Slug)
		if !values.Has("page") {
			values.Set("page", "1")
		}
		r.URL.RawQuery = values.Encode()
		r = r.WithContext(context.WithValue(r.Context(), categoryReadingKey{}, category.ID))
		if values.Has("q") {
			s.searchPublic(w, r)
		} else {
			s.listPublic(w, r)
		}
	}))
}
func (s *Service) readCategory(ctx context.Context, slug string) (categoryPage, error) {
	var value categoryPage
	if len(slug) > 160 || !slugPattern.MatchString(slug) {
		return value, fault(404, "not_found")
	}
	err := s.db.QueryRowContext(ctx, `SELECT c.id,c.name,c.slug,c.description_en,c.description_zh,
 (SELECT count(*) FROM articles a JOIN revisions r ON r.id=a.published_revision_id WHERE r.category_id=c.id AND (a.visibility='public' OR $2))
 FROM categories c WHERE c.slug=$1 AND ($2 OR EXISTS(SELECT 1 FROM articles a JOIN revisions r ON r.id=a.published_revision_id WHERE r.category_id=c.id AND a.visibility='public'))`, slug, ownerReading(ctx)).Scan(&value.ID, &value.Name, &value.Slug, &value.DescriptionEN, &value.DescriptionZH, &value.Total)
	if errors.Is(err, sql.ErrNoRows) {
		return value, fault(404, "not_found")
	}
	return value, err
}

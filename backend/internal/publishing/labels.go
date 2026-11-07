package publishing

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type adminLabel struct {
	Label
	Version int64 `json:"version"`
}

// Article mutations share this transaction lock before locking an article row.
// Rename takes it exclusively so its affected-article set cannot change midway.
// Shared writers still run concurrently on different articles.
func lockLabels(ctx context.Context, tx *sql.Tx, exclusive bool) error {
	query := `SELECT pg_advisory_xact_lock_shared(7311171)`
	if exclusive {
		query = `SELECT pg_advisory_xact_lock(7311171)`
	}
	_, err := tx.ExecContext(ctx, query)
	return err
}
func labelTable(kind string) string {
	switch kind {
	case "categories", "tags":
		return kind
	default:
		panic("invalid internal label kind")
	}
}
func labelAssociation(kind, revision string) string {
	if kind == "categories" {
		return revision + ".category_id=$1"
	}
	return "EXISTS(SELECT 1 FROM revision_tags rt WHERE rt.revision_id=" + revision + ".id AND rt.tag_id=$1)"
}
func labelName(fields map[string]json.RawMessage) (string, error) {
	var name string
	if json.Unmarshal(fields["name"], &name) != nil || strings.TrimSpace(name) == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 120 || strings.ContainsRune(name, 0) {
		return "", fault(422, "invalid_label_name")
	}
	return name, nil
}
func (s *Service) createLabel(ctx context.Context, kind string, fields map[string]json.RawMessage) (adminLabel, error) {
	var result adminLabel
	name, err := labelName(fields)
	if err != nil {
		return result, err
	}
	var slug string
	if len(fields) != 2 || json.Unmarshal(fields["slug"], &slug) != nil || len(slug) > 160 || !slugPattern.MatchString(slug) {
		return result, fault(422, "invalid_label")
	}
	err = s.db.QueryRowContext(ctx, `INSERT INTO `+labelTable(kind)+`(id,name,slug) VALUES($1,$2,$3) RETURNING id,name,slug,version`, uuid.NewString(), name, slug).Scan(&result.ID, &result.Name, &result.Slug, &result.Version)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return result, fault(409, "slug_conflict")
	}
	return result, err
}
func (s *Service) renameLabel(ctx context.Context, kind, id string, fields map[string]json.RawMessage) (adminLabel, error) {
	var result adminLabel
	name, err := labelName(fields)
	if err != nil {
		return result, err
	}
	version, err := expectedVersion(fields)
	if err != nil || len(fields) != 2 {
		return result, fault(422, "invalid_label")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	if err := lockLabels(ctx, tx, true); err != nil {
		return result, err
	}
	err = tx.QueryRowContext(ctx, `SELECT id,name,slug,version FROM `+labelTable(kind)+` WHERE id=$1 FOR NO KEY UPDATE`, id).Scan(&result.ID, &result.Name, &result.Slug, &result.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return result, fault(404, "not_found")
	}
	if err != nil {
		return result, err
	}
	if result.Version != version {
		return result, fault(409, "version_conflict")
	}
	if name == result.Name {
		return result, tx.Commit()
	}
	if err := tx.QueryRowContext(ctx, `UPDATE `+labelTable(kind)+` SET name=$1,version=version+1 WHERE id=$2 RETURNING name,version`, name, id).Scan(&result.Name, &result.Version); err != nil {
		return result, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE articles a SET version=version+1,modified_at=clock_timestamp(),
 updated_at=CASE WHEN EXISTS(SELECT 1 FROM revisions r WHERE r.id=a.published_revision_id AND `+labelAssociation(kind, "r")+`) THEN clock_timestamp() ELSE updated_at END
 WHERE EXISTS(SELECT 1 FROM revisions r WHERE r.id IN (a.draft_revision_id,a.published_revision_id) AND `+labelAssociation(kind, "r")+`)`, id)
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}

func (s *Service) adminLabelRoutes(mux *http.ServeMux) {
	for _, kind := range []string{"categories", "tags"} {
		mux.HandleFunc("GET /api/v1/admin/"+kind, func(w http.ResponseWriter, r *http.Request) { s.listLabels(w, r, kind, true) })
		mux.HandleFunc("POST /api/v1/admin/"+kind, func(w http.ResponseWriter, r *http.Request) {
			fields, err := readFields(w, r)
			if err != nil {
				reply(w, nil, err)
				return
			}
			label, err := s.createLabel(r.Context(), kind, fields)
			if err != nil {
				reply(w, nil, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			_ = json.NewEncoder(w).Encode(label)
		})
		mux.HandleFunc("PATCH /api/v1/admin/"+kind+"/{label_id}", func(w http.ResponseWriter, r *http.Request) {
			id, ok := pathID(w, r, "label_id")
			if !ok {
				return
			}
			fields, err := readFields(w, r)
			if err != nil {
				reply(w, nil, err)
				return
			}
			label, err := s.renameLabel(r.Context(), kind, id, fields)
			reply(w, label, err)
		})
	}
}
func (s *Service) publicLabelRoutes(mux *http.ServeMux) {
	for _, kind := range []string{"categories", "tags"} {
		mux.HandleFunc("GET /api/v1/"+kind, func(w http.ResponseWriter, r *http.Request) { s.listLabels(w, r, kind, false) })
	}
}

type labelCursor struct {
	Kind  string `json:"kind"`
	Admin bool   `json:"admin"`
	Name  string `json:"name"`
	ID    string `json:"id"`
}

func (s *Service) listLabels(w http.ResponseWriter, r *http.Request, kind string, admin bool) {
	values := r.URL.Query()
	limit := 20
	for key, items := range values {
		if (key != "limit" && key != "cursor") || len(items) != 1 {
			reply(w, nil, fault(400, "invalid_label_parameters"))
			return
		}
	}
	if raw := values.Get("limit"); values.Has("limit") {
		number, err := strconv.Atoi(raw)
		if err != nil || number < 1 || number > 100 {
			reply(w, nil, fault(400, "invalid_limit"))
			return
		}
		limit = number
	}
	var after *labelCursor
	if values.Has("cursor") {
		raw := values.Get("cursor")
		if len(raw) > 4096 {
			reply(w, nil, fault(400, "invalid_cursor"))
			return
		}
		data, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			reply(w, nil, fault(400, "invalid_cursor"))
			return
		}
		after = &labelCursor{}
		if json.Unmarshal(data, after) != nil || after.Kind != kind || after.Admin != admin || utf8.RuneCountInString(after.Name) > 120 || strings.TrimSpace(after.Name) == "" || strings.ContainsRune(after.Name, 0) {
			reply(w, nil, fault(400, "invalid_cursor"))
			return
		}
		if _, err := canonicalID(after.ID); err != nil {
			reply(w, nil, fault(400, "invalid_cursor"))
			return
		}
	}
	var name, id *string
	if after != nil {
		name = &after.Name
		id = &after.ID
	}
	association := "r.category_id=l.id"
	if kind == "tags" {
		association = "EXISTS(SELECT 1 FROM revision_tags rt WHERE rt.revision_id=r.id AND rt.tag_id=l.id)"
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT l.id,l.name,l.slug,l.version FROM `+labelTable(kind)+` l
 WHERE ($1 OR EXISTS(SELECT 1 FROM articles a JOIN revisions r ON r.id=a.published_revision_id WHERE a.visibility='public' AND `+association+`))
 AND ($2::text IS NULL OR (l.name COLLATE "C",l.id)>($2 COLLATE "C",$3::uuid)) ORDER BY l.name COLLATE "C",l.id LIMIT $4`, admin, name, id, limit+1)
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer rows.Close()
	labels := []adminLabel{}
	for rows.Next() {
		var label adminLabel
		if err := rows.Scan(&label.ID, &label.Name, &label.Slug, &label.Version); err != nil {
			reply(w, nil, err)
			return
		}
		labels = append(labels, label)
	}
	if err := rows.Err(); err != nil {
		reply(w, nil, err)
		return
	}
	more := len(labels) > limit
	if more {
		labels = labels[:limit]
	}
	var next *string
	if more && len(labels) > 0 {
		last := labels[len(labels)-1]
		encoded, _ := json.Marshal(labelCursor{kind, admin, last.Name, last.ID})
		value := base64.RawURLEncoding.EncodeToString(encoded)
		next = &value
	}
	if admin {
		reply(w, map[string]any{"items": labels, "next_cursor": next}, nil)
		return
	}
	public := []Label{}
	for _, label := range labels {
		public = append(public, label.Label)
	}
	reply(w, map[string]any{"items": public, "next_cursor": next}, nil)
}

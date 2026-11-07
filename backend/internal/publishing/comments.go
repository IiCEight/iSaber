package publishing

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type publicComment struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}
type adminComment struct {
	publicComment
	ArticleID   string     `json:"article_id"`
	Status      string     `json:"status"`
	Version     int64      `json:"version"`
	ModeratedAt *time.Time `json:"moderated_at"`
}

func commentText(fields map[string]json.RawMessage) (string, string, error) {
	var name, body string
	if len(fields) != 2 || json.Unmarshal(fields["display_name"], &name) != nil || json.Unmarshal(fields["body"], &body) != nil {
		return "", "", fault(422, "invalid_comment")
	}
	for _, item := range []struct {
		value string
		max   int
	}{{name, 80}, {body, 5000}} {
		if !utf8.ValidString(item.value) || strings.TrimSpace(item.value) == "" || utf8.RuneCountInString(item.value) > item.max || strings.ContainsRune(item.value, 0) {
			return "", "", fault(422, "invalid_comment")
		}
	}
	return name, body, nil
}
func (s *Service) submitComment(ctx context.Context, article, key string, fields map[string]json.RawMessage) (response, error) {
	name, body, err := commentText(fields)
	if err != nil {
		return response{}, err
	}
	payload, _ := json.Marshal(map[string]string{"display_name": name, "body": body})
	guard := func(tx *sql.Tx) error {
		var visibility string
		var published *string
		var enabled bool
		err := tx.QueryRowContext(ctx, `SELECT visibility,published_revision_id,comments_enabled FROM articles WHERE id=$1 FOR SHARE`, article).Scan(&visibility, &published, &enabled)
		if errors.Is(err, sql.ErrNoRows) {
			return fault(404, "not_found")
		}
		if err != nil {
			return err
		}
		if visibility != "public" || published == nil {
			return fault(404, "not_found")
		}
		if !enabled {
			return fault(409, "comments_disabled")
		}
		return nil
	}
	return s.idempotent(ctx, "guest-comment:"+article, key, payload, func(tx *sql.Tx) (int, any, error) {
		// Serialize the quota check with the insertion, across API processes. Replays
		// skip this action and consume no further quota. No reader IPs are stored.
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(7311172)`); err != nil {
			return 0, nil, err
		}
		var global, local int
		if err := tx.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE article_id=$1) FROM comments WHERE created_at>=clock_timestamp()-interval '1 minute'`, article).Scan(&global, &local); err != nil {
			return 0, nil, err
		}
		if global >= 20 || local >= 5 {
			return 0, nil, fault(429, "comment_rate_limited")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO comments(id,article_id,display_name,body,created_at) VALUES($1,$2,$3,$4,clock_timestamp())`, uuid.NewString(), article, name, body); err != nil {
			return 0, nil, err
		}
		return 202, map[string]string{"status": "pending"}, nil
	}, guard)
}
func (s *Service) publicCommentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/articles/{article_id}/comments", s.listPublicComments)
	mux.HandleFunc("POST /api/v1/articles/{article_id}/comments", func(w http.ResponseWriter, r *http.Request) {
		if s.commentOrigin == "" {
			reply(w, nil, fault(503, "comments_unavailable"))
			return
		}
		if r.Header.Get("Origin") != s.commentOrigin {
			reply(w, nil, fault(403, "invalid_origin"))
			return
		}
		select {
		case s.commentSlots <- struct{}{}:
			defer func() { <-s.commentSlots }()
		default:
			w.Header().Set("Retry-After", "1")
			reply(w, nil, fault(503, "comments_busy"))
			return
		}
		// Bound body reads as well as SQL work on real HTTP connections. Recorder
		// transports may not support connection deadlines; the server also has one.
		controller := http.NewResponseController(w)
		_ = controller.SetReadDeadline(time.Now().Add(5 * time.Second))
		defer controller.SetReadDeadline(time.Time{})
		article, ok := pathID(w, r, "article_id")
		if !ok {
			return
		}
		key, ok := requestKey(w, r)
		if !ok {
			return
		}
		fields, err := readFields(w, r, 32*1024)
		if err != nil {
			reply(w, nil, err)
			return
		}
		result, err := s.submitComment(r.Context(), article, key, fields)
		var problem *Fault
		if errors.As(err, &problem) && problem.Status == 429 {
			w.Header().Set("Retry-After", "60")
		}
		sendResult(w, result, err)
	})
}
func (s *Service) adminCommentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/comments", s.listAdminComments)
	mux.HandleFunc("PATCH /api/v1/admin/comments/{comment_id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "comment_id")
		if !ok {
			return
		}
		fields, err := readFields(w, r)
		if err != nil {
			reply(w, nil, err)
			return
		}
		comment, err := s.moderateComment(r.Context(), id, fields)
		reply(w, comment, err)
	})
}
func commentStatus(value string) bool {
	return value == "pending" || value == "approved" || value == "rejected" || value == "removed"
}
func loadComment(ctx context.Context, q queryer, id string) (adminComment, error) {
	var result adminComment
	err := q.QueryRowContext(ctx, `SELECT id,article_id,display_name,body,status,version,created_at,moderated_at FROM comments WHERE id=$1`, id).Scan(&result.ID, &result.ArticleID, &result.DisplayName, &result.Body, &result.Status, &result.Version, &result.CreatedAt, &result.ModeratedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return result, fault(404, "not_found")
	}
	result.CreatedAt = result.CreatedAt.UTC()
	utc(result.ModeratedAt)
	return result, err
}
func (s *Service) moderateComment(ctx context.Context, id string, fields map[string]json.RawMessage) (adminComment, error) {
	var result adminComment
	version, err := expectedVersion(fields)
	if err != nil {
		return result, err
	}
	var status string
	if len(fields) != 2 || json.Unmarshal(fields["status"], &status) != nil || !commentStatus(status) || status == "pending" {
		return result, fault(422, "invalid_moderation")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	var found string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM comments WHERE id=$1 FOR UPDATE`, id).Scan(&found); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return result, fault(404, "not_found")
		}
		return result, err
	}
	result, err = loadComment(ctx, tx, id)
	if err != nil {
		return result, err
	}
	if result.Version != version {
		return result, fault(409, "version_conflict")
	}
	if result.Status == status {
		return result, tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `UPDATE comments SET status=$1,version=version+1,moderated_at=clock_timestamp() WHERE id=$2`, status, id); err != nil {
		return result, err
	}
	result, err = loadComment(ctx, tx, id)
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}
func (s *Service) listPublicComments(w http.ResponseWriter, r *http.Request) {
	article, ok := pathID(w, r, "article_id")
	if !ok {
		return
	}
	values := r.URL.Query()
	for key, items := range values {
		if (key != "limit" && key != "cursor") || len(items) != 1 {
			reply(w, nil, fault(400, "invalid_comment_parameters"))
			return
		}
	}
	values.Set("_comments", "public:"+article)
	limit, after, filter, err := pageOptions(values)
	if err != nil {
		reply(w, nil, err)
		return
	}
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer tx.Rollback()
	var found string
	err = tx.QueryRowContext(r.Context(), `SELECT id FROM articles WHERE id=$1 AND visibility='public' AND published_revision_id IS NOT NULL`, article).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		reply(w, nil, fault(404, "not_found"))
		return
	}
	if err != nil {
		reply(w, nil, err)
		return
	}
	var at *time.Time
	var id *string
	if after != nil {
		at = &after.Time
		id = &after.ID
	}
	rows, err := tx.QueryContext(r.Context(), `SELECT id,display_name,body,created_at FROM comments WHERE article_id=$1 AND status='approved'
 AND ($2::timestamptz IS NULL OR (created_at,id)>($2,$3::uuid)) ORDER BY created_at,id LIMIT $4`, article, at, id, limit+1)
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer rows.Close()
	items := []publicComment{}
	for rows.Next() {
		var item publicComment
		if err := rows.Scan(&item.ID, &item.DisplayName, &item.Body, &item.CreatedAt); err != nil {
			reply(w, nil, err)
			return
		}
		item.CreatedAt = item.CreatedAt.UTC()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		reply(w, nil, err)
		return
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	var next *string
	if more && len(items) > 0 {
		last := items[len(items)-1]
		next = encodeCursor(last.CreatedAt, last.ID, filter)
	}
	reply(w, map[string]any{"items": items, "next_cursor": next}, nil)
}
func (s *Service) listAdminComments(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	for key, items := range values {
		if (key != "limit" && key != "cursor" && key != "article_id" && key != "status") || len(items) != 1 {
			reply(w, nil, fault(400, "invalid_comment_parameters"))
			return
		}
	}
	status := values.Get("status")
	if values.Has("status") && !commentStatus(status) {
		reply(w, nil, fault(400, "invalid_comment_status"))
		return
	}
	var article *string
	if values.Has("article_id") {
		id, err := canonicalID(values.Get("article_id"))
		if err != nil {
			reply(w, nil, fault(400, "invalid_article_id"))
			return
		}
		article = &id
		values.Set("article_id", id)
	}
	values.Set("_comments", "admin")
	limit, after, filter, err := pageOptions(values)
	if err != nil {
		reply(w, nil, err)
		return
	}
	var at *time.Time
	var id *string
	if after != nil {
		at = &after.Time
		id = &after.ID
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT id,article_id,display_name,body,status,version,created_at,moderated_at FROM comments
 WHERE ($1::uuid IS NULL OR article_id=$1) AND ($2='' OR status=$2) AND ($3::timestamptz IS NULL OR (created_at,id)<($3,$4::uuid))
 ORDER BY created_at DESC,id DESC LIMIT $5`, article, status, at, id, limit+1)
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer rows.Close()
	items := []adminComment{}
	for rows.Next() {
		var item adminComment
		if err := rows.Scan(&item.ID, &item.ArticleID, &item.DisplayName, &item.Body, &item.Status, &item.Version, &item.CreatedAt, &item.ModeratedAt); err != nil {
			reply(w, nil, err)
			return
		}
		item.CreatedAt = item.CreatedAt.UTC()
		utc(item.ModeratedAt)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		reply(w, nil, err)
		return
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	var next *string
	if more && len(items) > 0 {
		last := items[len(items)-1]
		next = encodeCursor(last.CreatedAt, last.ID, filter)
	}
	reply(w, map[string]any{"items": items, "next_cursor": next}, nil)
}

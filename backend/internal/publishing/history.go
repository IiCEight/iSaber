package publishing

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/IiCEight/iSaber/backend/internal/auth"
)

// Draft-only snapshots are not historical publications, even when their IDs are
// known to the caller. Both article ownership and publication history are checked.
func publishedRevision(ctx context.Context, q queryer, article, id string) (*Revision, error) {
	var found string
	err := q.QueryRowContext(ctx, `SELECT id FROM revisions WHERE id=$1 AND article_id=$2 AND first_published_at IS NOT NULL`, id, article).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fault(404, "not_found")
	}
	if err != nil {
		return nil, err
	}
	return loadRevision(ctx, q, found)
}

func (s *Service) Restore(ctx context.Context, owner, article, revision, key string, fields map[string]json.RawMessage) (response, error) {
	version, err := expectedVersion(fields)
	if err != nil || len(fields) != 1 {
		return response{}, fault(422, "invalid_restore_request")
	}
	payload, _ := json.Marshal(map[string]any{"revision_id": revision, "expected_version": version})
	return s.idempotent(ctx, owner+":restore:"+article, key, payload, func(tx *sql.Tx) (int, any, error) {
		a, err := lockArticle(ctx, tx, article, version)
		if err != nil {
			return 0, nil, err
		}
		source, err := publishedRevision(ctx, tx, article, revision)
		if err != nil {
			return 0, nil, err
		}
		snapshot := *source
		snapshot.ID = ""
		snapshot.FirstPublishedAt = nil
		snapshot.RestoredFromRevision = &source.ID
		if err := insertRevision(ctx, tx, &snapshot); err != nil {
			return 0, nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE articles SET draft_revision_id=$1,version=version+1,modified_at=clock_timestamp() WHERE id=$2`, snapshot.ID, a.ID); err != nil {
			return 0, nil, err
		}
		updated, err := loadArticle(ctx, tx, a.ID)
		return 200, updated, err
	})
}

func (s *Service) historyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/articles/{article_id}/revisions", s.listRevisions)
	mux.HandleFunc("GET /api/v1/admin/articles/{article_id}/revisions/{revision_id}", func(w http.ResponseWriter, r *http.Request) {
		article, revision, ok := historyIDs(w, r)
		if !ok {
			return
		}
		snapshot, err := publishedRevision(r.Context(), s.db, article, revision)
		reply(w, snapshot, err)
	})
	mux.HandleFunc("POST /api/v1/admin/articles/{article_id}/revisions/{revision_id}/restore", func(w http.ResponseWriter, r *http.Request) {
		article, revision, ok := historyIDs(w, r)
		if !ok {
			return
		}
		key, ok := requestKey(w, r)
		if !ok {
			return
		}
		fields, err := readFields(w, r)
		if err != nil {
			reply(w, nil, err)
			return
		}
		owner, _ := auth.OwnerFromContext(r.Context())
		result, err := s.Restore(r.Context(), owner.ID, article, revision, key, fields)
		sendResult(w, result, err)
	})
}
func historyIDs(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	article, ok := pathID(w, r, "article_id")
	if !ok {
		return "", "", false
	}
	revision, ok := pathID(w, r, "revision_id")
	return article, revision, ok
}
func (s *Service) listRevisions(w http.ResponseWriter, r *http.Request) {
	article, ok := pathID(w, r, "article_id")
	if !ok {
		return
	}
	values := r.URL.Query()
	for key, items := range values {
		if (key != "limit" && key != "cursor") || len(items) != 1 {
			reply(w, nil, fault(400, "invalid_history_parameters"))
			return
		}
	}
	// A cursor from another article is not valid even when ordering positions match.
	values.Set("article_id", article)
	limit, after, filter, err := pageOptions(values)
	if err != nil {
		reply(w, nil, err)
		return
	}
	var exists string
	err = s.db.QueryRowContext(r.Context(), `SELECT id FROM articles WHERE id=$1`, article).Scan(&exists)
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
	rows, err := s.db.QueryContext(r.Context(), `SELECT id FROM revisions WHERE article_id=$1 AND first_published_at IS NOT NULL
  AND ($2::timestamptz IS NULL OR (first_published_at,id)<($2,$3::uuid)) ORDER BY first_published_at DESC,id DESC LIMIT $4`, article, at, id, limit+1)
	if err != nil {
		reply(w, nil, err)
		return
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			reply(w, nil, err)
			return
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		reply(w, nil, err)
		return
	}
	more := len(ids) > limit
	if more {
		ids = ids[:limit]
	}
	items := []*Revision{}
	for _, id := range ids {
		snapshot, err := publishedRevision(r.Context(), s.db, article, id)
		if err != nil {
			reply(w, nil, err)
			return
		}
		items = append(items, snapshot)
	}
	var next *string
	if more && len(items) > 0 {
		last := items[len(items)-1]
		next = encodeCursor(*last.FirstPublishedAt, last.ID, filter)
	}
	reply(w, map[string]any{"items": items, "next_cursor": next}, nil)
}

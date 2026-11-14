package publishing

import (
	"database/sql"
	"net/http"
	"net/url"
	"unicode/utf8"
)

const adminCollectionSQL = ` FROM articles a LEFT JOIN revisions d ON d.id=a.draft_revision_id
 LEFT JOIN revisions p ON p.id=a.published_revision_id
 WHERE ($1='' OR (CASE WHEN a.published_revision_id IS NULL THEN 'draft' ELSE 'published' END)=$1)
 AND ($2='' OR a.visibility=$2) AND ($3='' OR strpos(lower(COALESCE(d.title,p.title,'')),lower($3))>0)`

// Numbered owner pages keep the existing modified-at order and cursor API.
func (s *Service) listAdminNumbered(w http.ResponseWriter, r *http.Request, values url.Values) {
	options, err := parseNumberedOptions(values)
	if err != nil {
		reply(w, nil, err)
		return
	}
	for key, items := range values {
		if (key != "page" && key != "limit" && key != "status" && key != "visibility" && key != "q") || len(items) != 1 {
			reply(w, nil, fault(400, "invalid_page_parameters"))
			return
		}
	}
	status, visibility, query := values.Get("status"), values.Get("visibility"), values.Get("q")
	if (status != "" && status != "draft" && status != "published") || (visibility != "" && visibility != "public" && visibility != "private") || utf8.RuneCountInString(query) > 200 {
		reply(w, nil, fault(400, "invalid_filters"))
		return
	}
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer tx.Rollback()
	args := []any{status, visibility, query}
	var total int64
	if err := tx.QueryRowContext(r.Context(), `SELECT count(*)`+adminCollectionSQL, args...).Scan(&total); err != nil {
		reply(w, nil, err)
		return
	}
	page := options.result(total)
	args = append(args, page.PageSize, page.offset())
	rows, err := tx.QueryContext(r.Context(), `SELECT a.id,a.slug,CASE WHEN d.id IS NULL THEN p.title ELSE d.title END,
 CASE WHEN a.published_revision_id IS NULL THEN 'draft' ELSE 'published' END,
 a.visibility,(a.draft_revision_id IS NOT NULL),a.version,a.published_at,a.updated_at,a.modified_at`+
		adminCollectionSQL+` ORDER BY a.modified_at DESC,a.id DESC LIMIT $4 OFFSET $5`, args...)
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer rows.Close()
	items := []ArticleSummary{}
	for rows.Next() {
		var item ArticleSummary
		if err := rows.Scan(&item.ID, &item.Slug, &item.Title, &item.Status, &item.Visibility, &item.HasDraft, &item.Version, &item.PublishedAt, &item.UpdatedAt, &item.ModifiedAt); err != nil {
			reply(w, nil, err)
			return
		}
		utc(item.PublishedAt)
		utc(item.UpdatedAt)
		item.ModifiedAt = item.ModifiedAt.UTC()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		reply(w, nil, err)
		return
	}
	reply(w, map[string]any{"items": items, "next_cursor": nil, "page": page.Page, "page_size": page.PageSize, "total": page.Total, "total_pages": page.TotalPages}, nil)
}

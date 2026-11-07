package publishing

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadRevision(ctx context.Context, q queryer, id string) (*Revision, error) {
	r := &Revision{Tags: []Label{}, SourceIssues: []Issue{}}
	var category *string
	var source []byte
	err := q.QueryRowContext(ctx, `SELECT id,article_id,title,summary,language,to_char(written_at,'YYYY-MM-DD'),
        category_id,markdown,created_at,first_published_at,restored_from_revision_id,source_issues FROM revisions WHERE id=$1`, id).
		Scan(&r.ID, &r.ArticleID, &r.Title, &r.Summary, &r.Language, &r.WrittenAt, &category, &r.Markdown, &r.CreatedAt, &r.FirstPublishedAt, &r.RestoredFromRevision, &source)
	if err != nil {
		return nil, err
	}
	r.CreatedAt = r.CreatedAt.UTC()
	utc(r.FirstPublishedAt)
	if err := json.Unmarshal(source, &r.SourceIssues); err != nil {
		return nil, err
	}
	if category != nil {
		r.Category = &Label{}
		if err := q.QueryRowContext(ctx, `SELECT id,name,slug FROM categories WHERE id=$1`, *category).Scan(&r.Category.ID, &r.Category.Name, &r.Category.Slug); err != nil {
			return nil, err
		}
	}
	rows, err := q.QueryContext(ctx, `SELECT t.id,t.name,t.slug FROM tags t JOIN revision_tags rt ON rt.tag_id=t.id WHERE rt.revision_id=$1 ORDER BY t.name,t.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var label Label
		if err := rows.Scan(&label.ID, &label.Name, &label.Slug); err != nil {
			return nil, err
		}
		r.Tags = append(r.Tags, label)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	assets, err := q.QueryContext(ctx, `SELECT a.id,a.article_id,a.storage_key,a.media_type,a.byte_size,a.checksum,ra.original_path FROM assets a JOIN revision_assets ra ON ra.asset_id=a.id WHERE ra.revision_id=$1 ORDER BY a.id`, id)
	if err != nil {
		return nil, err
	}
	defer assets.Close()
	for assets.Next() {
		var item asset
		if err := assets.Scan(&item.ID, &item.ArticleID, &item.StorageKey, &item.MediaType, &item.Size, &item.Checksum, &item.OriginalPath); err != nil {
			return nil, err
		}
		r.Assets = append(r.Assets, item)
	}
	return r, assets.Err()
}

func loadArticle(ctx context.Context, q queryer, id string) (*Article, error) {
	a := &Article{Issues: []Issue{}}
	var draft, published *string
	err := q.QueryRowContext(ctx, `SELECT id,slug,visibility,comments_enabled,draft_revision_id,published_revision_id,
        published_at,updated_at,version,modified_at FROM articles WHERE id=$1`, id).
		Scan(&a.ID, &a.Slug, &a.Visibility, &a.CommentsEnabled, &draft, &published, &a.PublishedAt, &a.UpdatedAt, &a.Version, &a.ModifiedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fault(404, "not_found")
	}
	if err != nil {
		return nil, err
	}
	a.ModifiedAt = a.ModifiedAt.UTC()
	utc(a.PublishedAt)
	utc(a.UpdatedAt)
	a.Status = "draft"
	if published != nil {
		a.Status = "published"
		a.Published, err = loadRevision(ctx, q, *published)
		if err != nil {
			return nil, err
		}
		a.Title = a.Published.Title
	}
	if draft != nil {
		a.HasDraft = true
		a.Draft, err = loadRevision(ctx, q, *draft)
		if err != nil {
			return nil, err
		}
		a.Title = a.Draft.Title
		a.Issues = revisionIssues(a.Draft, a.Slug)
		a.Publishable = !blocking(a.Issues)
	}
	return a, nil
}

func insertRevision(ctx context.Context, tx *sql.Tx, r *Revision) error {
	source, err := json.Marshal(r.SourceIssues)
	if err != nil {
		return err
	}
	var category *string
	if r.Category != nil {
		category = &r.Category.ID
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO revisions(id,article_id,title,summary,language,written_at,category_id,markdown,source_issues,restored_from_revision_id)
        VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,created_at`, r.ArticleID, r.Title, r.Summary, r.Language, r.WrittenAt, category, r.Markdown, source, r.RestoredFromRevision).Scan(&r.ID, &r.CreatedAt)
	if err != nil {
		return err
	}
	for _, tag := range r.Tags {
		if _, err := tx.ExecContext(ctx, `INSERT INTO revision_tags(revision_id,tag_id) VALUES($1,$2)`, r.ID, tag.ID); err != nil {
			return err
		}
	}
	for _, item := range r.Assets {
		if _, err := tx.ExecContext(ctx, `INSERT INTO revision_assets(article_id,revision_id,asset_id,original_path) VALUES($1,$2,$3,$4)`, r.ArticleID, r.ID, item.ID, item.OriginalPath); err != nil {
			return err
		}
	}
	return nil
}

func lockArticle(ctx context.Context, tx *sql.Tx, id string, version int64) (*Article, error) {
	var found string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM articles WHERE id=$1 FOR UPDATE`, id).Scan(&found); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fault(404, "not_found")
		}
		return nil, err
	}
	a, err := loadArticle(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if a.Version != version {
		return nil, fault(409, "version_conflict")
	}
	return a, nil
}

func utc(value *time.Time) {
	if value != nil {
		*value = value.UTC()
	}
}

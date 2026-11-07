package publishing

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type Service struct {
	db          *sql.DB
	root        *os.Root
	searchSlots chan struct{}
}

func NewService(db *sql.DB, directory string) (*Service, error) {
	if directory == "" {
		directory = ".data/uploads"
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, errors.New("cannot create private upload directory")
	}
	info, err := os.Stat(directory)
	if err != nil || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("upload directory must be private to its owner (0700)")
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, errors.New("cannot open private upload directory")
	}
	return &Service{db: db, root: root, searchSlots: make(chan struct{}, 2)}, nil
}
func (s *Service) Close() error { return s.root.Close() }

func (s *Service) PatchDraft(ctx context.Context, id string, fields map[string]json.RawMessage) (*Article, error) {
	version, err := expectedVersion(fields)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	a, err := lockArticle(ctx, tx, id, version)
	if err != nil {
		return nil, err
	}
	if a.Draft == nil {
		return nil, fault(409, "no_draft")
	}
	r := *a.Draft
	r.ID = ""
	r.FirstPublishedAt = nil
	r.SourceIssues = append([]Issue{}, a.Draft.SourceIssues...)
	for key, raw := range fields {
		switch key {
		case "expected_version":
		case "title", "summary", "language", "written_at":
			var value *string
			if json.Unmarshal(raw, &value) != nil {
				return nil, fault(422, "invalid_metadata")
			}
			if value != nil {
				switch key {
				case "title":
					if utf8.RuneCountInString(*value) > 300 {
						return nil, fault(422, "invalid_title")
					}
				case "summary":
					if utf8.RuneCountInString(*value) > 2000 {
						return nil, fault(422, "invalid_summary")
					}
				case "language":
					if *value != "en" && *value != "zh" {
						return nil, fault(422, "invalid_language")
					}
				case "written_at":
					if !validDate(*value) {
						return nil, fault(422, "invalid_written_at")
					}
				}
			}
			switch key {
			case "title":
				r.Title = value
			case "summary":
				r.Summary = value
			case "language":
				r.Language = value
			case "written_at":
				r.WrittenAt = value
			}
		case "slug":
			var value string
			if json.Unmarshal(raw, &value) != nil || !validSlug(value) {
				return nil, fault(422, "invalid_slug")
			}
			if a.PublishedAt != nil && (a.Slug == nil || value != *a.Slug) {
				return nil, fault(409, "slug_locked")
			}
			if _, err := tx.ExecContext(ctx, `UPDATE articles SET slug=$1 WHERE id=$2`, value, id); err != nil {
				var pg *pgconn.PgError
				if errors.As(err, &pg) && pg.Code == "23505" {
					return nil, fault(409, "slug_conflict")
				}
				return nil, err
			}
			a.Slug = &value
		case "category_id":
			var value *string
			if json.Unmarshal(raw, &value) != nil {
				return nil, fault(422, "invalid_category")
			}
			r.Category = nil
			if value != nil {
				if _, err := uuid.Parse(*value); err != nil {
					return nil, fault(422, "invalid_category")
				}
				label := Label{}
				if err := tx.QueryRowContext(ctx, `SELECT id,name,slug FROM categories WHERE id=$1`, *value).Scan(&label.ID, &label.Name, &label.Slug); err != nil {
					return nil, fault(422, "invalid_category")
				}
				r.Category = &label
			}
		case "tag_ids":
			var values []string
			if string(raw) == "null" || json.Unmarshal(raw, &values) != nil {
				return nil, fault(422, "invalid_tags")
			}
			r.Tags = []Label{}
			seen := map[string]bool{}
			for _, value := range values {
				parsed, err := uuid.Parse(value)
				if err != nil || seen[parsed.String()] {
					return nil, fault(422, "invalid_tags")
				}
				seen[parsed.String()] = true
				label := Label{}
				if err := tx.QueryRowContext(ctx, `SELECT id,name,slug FROM tags WHERE id=$1`, parsed.String()).Scan(&label.ID, &label.Name, &label.Slug); err != nil {
					return nil, fault(422, "invalid_tags")
				}
				r.Tags = append(r.Tags, label)
			}
		default:
			return nil, fault(400, "unknown_field")
		}
	}
	kept := []Issue{}
	for _, item := range r.SourceIssues {
		if item.Path == nil {
			kept = append(kept, item)
			continue
		}
		if _, changed := fields[*item.Path]; !changed {
			kept = append(kept, item)
		}
	}
	r.SourceIssues = kept
	if err := insertRevision(ctx, tx, &r); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE articles SET draft_revision_id=$1,version=version+1,modified_at=clock_timestamp() WHERE id=$2`, r.ID, id); err != nil {
		return nil, err
	}
	result, err := loadArticle(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func expectedVersion(fields map[string]json.RawMessage) (int64, error) {
	var version int64
	if json.Unmarshal(fields["expected_version"], &version) != nil || version < 1 {
		return 0, fault(422, "expected_version_required")
	}
	return version, nil
}

func (s *Service) Controls(ctx context.Context, id string, fields map[string]json.RawMessage) (*Article, error) {
	version, err := expectedVersion(fields)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	a, err := lockArticle(ctx, tx, id, version)
	if err != nil {
		return nil, err
	}
	for key, raw := range fields {
		switch key {
		case "expected_version":
		case "visibility":
			var value string
			if json.Unmarshal(raw, &value) != nil || (value != "public" && value != "private") {
				return nil, fault(422, "invalid_visibility")
			}
			a.Visibility = value
		case "comments_enabled":
			if string(raw) == "null" || json.Unmarshal(raw, &a.CommentsEnabled) != nil {
				return nil, fault(422, "invalid_controls")
			}
		default:
			return nil, fault(400, "unknown_field")
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE articles SET visibility=$1,comments_enabled=$2,version=version+1,modified_at=clock_timestamp(),
        updated_at=CASE WHEN published_revision_id IS NOT NULL THEN clock_timestamp() ELSE updated_at END WHERE id=$3`, a.Visibility, a.CommentsEnabled, id)
	if err != nil {
		return nil, err
	}
	result, err := loadArticle(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) Publish(ctx context.Context, owner, id, key string, fields map[string]json.RawMessage, unpublish bool) (response, error) {
	version, err := expectedVersion(fields)
	if err != nil || len(fields) != 1 {
		return response{}, fault(422, "invalid_publication_request")
	}
	payload, _ := json.Marshal(fields)
	operation := "publish"
	if unpublish {
		operation = "unpublish"
	}
	return s.idempotent(ctx, owner+":"+operation+":"+id, key, payload, func(tx *sql.Tx) (int, any, error) {
		a, err := lockArticle(ctx, tx, id, version)
		if err != nil {
			return 0, nil, err
		}
		if unpublish {
			if a.Published != nil {
				_, err = tx.ExecContext(ctx, `UPDATE articles SET draft_revision_id=COALESCE(draft_revision_id,published_revision_id),
                published_revision_id=NULL,version=version+1,modified_at=clock_timestamp(),updated_at=clock_timestamp() WHERE id=$1`, id)
			}
		} else {
			if a.Draft == nil {
				return 0, nil, fault(409, "no_draft")
			}
			if !a.Publishable {
				return 0, nil, fault(422, "publication_blocked")
			}
			if err := s.checkAssets(a.Draft); err != nil {
				return 0, nil, fault(422, "assets_unavailable")
			}
			if _, err = tx.ExecContext(ctx, `UPDATE revisions SET first_published_at=COALESCE(first_published_at,clock_timestamp()) WHERE id=$1`, a.Draft.ID); err == nil {
				_, err = tx.ExecContext(ctx, `UPDATE articles SET published_revision_id=draft_revision_id,draft_revision_id=NULL,
                    published_at=COALESCE(published_at,clock_timestamp()),updated_at=clock_timestamp(),modified_at=clock_timestamp(),version=version+1 WHERE id=$1`, id)
			}
		}
		if err != nil {
			return 0, nil, err
		}
		result, err := loadArticle(ctx, tx, id)
		return 200, result, err
	})
}

func titleOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

package publishing

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/IiCEight/iSaber/backend/internal/migrations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Service) AcceptImport(ctx context.Context, owner, key, filename string, data []byte, article *string, version *int64) (response, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return response{}, err
	}
	storage := id.String() + ".md"
	file, err := s.root.OpenFile(storage, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return response{}, err
	}
	_, writeErr := file.Write(data)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return response{}, writeErr
	}
	if closeErr != nil {
		return response{}, closeErr
	}
	// Persist the new directory entry as well as the staged file bytes before
	// committing acceptance. An unreferenced file is safer than a missing stage.
	directory, err := s.root.Open(".")
	if err != nil {
		return response{}, err
	}
	syncErr := directory.Sync()
	closeErr = directory.Close()
	if syncErr != nil {
		return response{}, syncErr
	}
	if closeErr != nil {
		return response{}, closeErr
	}
	hash := sha256.Sum256(data)
	payload, _ := json.Marshal(map[string]any{"filename": filename, "sha256": hex.EncodeToString(hash[:]), "article_id": article, "expected_version": version})
	result, err := s.idempotent(ctx, owner+":import", key, payload, func(tx *sql.Tx) (int, any, error) {
		if article != nil {
			if _, err := lockArticle(ctx, tx, *article, *version); err != nil {
				return 0, nil, err
			}
		}
		job := Job{ID: id.String(), Status: "queued", ArticleID: article, Issues: []Issue{}}
		if err := tx.QueryRowContext(ctx, `INSERT INTO import_jobs(id,article_id,expected_version,upload_storage_key,filename)
            VALUES($1,$2,$3,$4,$5) RETURNING created_at`, job.ID, article, version, storage, filename).Scan(&job.CreatedAt); err != nil {
			return 0, nil, err
		}
		job.CreatedAt = job.CreatedAt.UTC()
		return 202, job, nil
	})
	if result.Replayed {
		_ = s.root.Remove(storage)
	}
	// On an uncertain SQL commit, retain the private stage; the client retries the
	// same key to determine whether acceptance committed. Cleanup is a later task.
	return result, err
}

func (s *Service) GetJob(ctx context.Context, id string) (Job, error) {
	job := Job{Issues: []Issue{}}
	var issues []byte
	err := s.db.QueryRowContext(ctx, `SELECT id,status,article_id,result_revision_id,issues,created_at,finished_at FROM import_jobs WHERE id=$1`, id).
		Scan(&job.ID, &job.Status, &job.ArticleID, &job.RevisionID, &issues, &job.CreatedAt, &job.FinishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return job, fault(404, "not_found")
	}
	if err != nil {
		return job, err
	}
	if err := json.Unmarshal(issues, &job.Issues); err != nil {
		return job, err
	}
	job.CreatedAt = job.CreatedAt.UTC()
	utc(job.FinishedAt)
	job.Publishable = job.Status == "succeeded" && !blocking(job.Issues)
	return job, nil
}

// Run is a single durable queue consumer. A PostgreSQL advisory lock prevents a
// second process from recovering work that an active consumer is still handling.
func (s *Service) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if migrations.CheckReady(ctx, s.db) != nil {
			if !pause(ctx, time.Second) {
				return
			}
			continue
		}
		conn, err := s.db.Conn(ctx)
		if err != nil {
			if !pause(ctx, time.Second) {
				return
			}
			continue
		}
		var acquired bool
		err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(7311170)`).Scan(&acquired)
		if err != nil || !acquired {
			conn.Close()
			if !pause(ctx, time.Second) {
				return
			}
			continue
		}
		_, recoveryErr := s.db.ExecContext(ctx, `UPDATE import_jobs SET status='failed',finished_at=clock_timestamp(),
            issues='[{"severity":"blocking","code":"worker_interrupted","path":null,"message":"Import was interrupted. Upload again with a new request key."}]'::jsonb WHERE status='processing'`)
		for recoveryErr == nil && ctx.Err() == nil {
			if conn.PingContext(ctx) != nil {
				break
			}
			if _, err := s.ProcessNext(ctx); err != nil {
				if !pause(ctx, time.Second) {
					break
				}
			} else if !pause(ctx, 250*time.Millisecond) {
				break
			}
		}
		release, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, unlockErr := conn.ExecContext(release, `SELECT pg_advisory_unlock(7311170)`)
		cancel()
		if unlockErr != nil {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		conn.Close()
	}
}

func pause(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// ProcessNext claims at most one queued job. It is also used by integration tests
// to verify durable acceptance and completion without timing-based polling.
func (s *Service) ProcessNext(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	var id, storage, filename string
	var article *string
	var version *int64
	err = tx.QueryRowContext(ctx, `SELECT id,upload_storage_key,article_id,expected_version,filename FROM import_jobs WHERE status='queued' ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &storage, &article, &version, &filename)
	if errors.Is(err, sql.ErrNoRows) {
		tx.Rollback()
		return false, nil
	}
	if err != nil {
		tx.Rollback()
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE import_jobs SET status='processing' WHERE id=$1`, id); err != nil {
		tx.Rollback()
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	data, err := s.readStage(storage, MaxMarkdown+1)
	if err != nil {
		return true, s.failJob(ctx, id, "staged_upload_unavailable")
	}
	parsed, err := parseMarkdown(data, filename)
	if err != nil {
		return true, s.failJob(ctx, id, err.Error())
	}
	if err := s.commitImport(ctx, id, article, version, filename, parsed); err != nil {
		var known *Fault
		if errors.As(err, &known) {
			return true, s.failJob(ctx, id, known.Code)
		}
		return true, s.failJob(ctx, id, "import_failed")
	}
	return true, nil
}

func (s *Service) readStage(key string, maximum int64) ([]byte, error) {
	file, err := s.root.Open(key)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, maximum))
}
func (s *Service) failJob(ctx context.Context, id, code string) error {
	issues, _ := json.Marshal([]Issue{{Severity: "blocking", Code: code, Message: "The import could not complete. Check the Markdown/front matter, reload the article if it changed, and upload again."}})
	_, err := s.db.ExecContext(ctx, `UPDATE import_jobs SET status='failed',issues=$1,finished_at=clock_timestamp() WHERE id=$2 AND status='processing'`, issues, id)
	return err
}

func (s *Service) commitImport(ctx context.Context, jobID string, article *string, version *int64, filename string, parsed imported) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var state string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM import_jobs WHERE id=$1 FOR UPDATE`, jobID).Scan(&state); err != nil {
		return err
	}
	if state != "processing" {
		return fault(409, "job_claim_lost")
	}
	var a *Article
	if article == nil {
		var id string
		if err := tx.QueryRowContext(ctx, `INSERT INTO articles(id) VALUES(gen_random_uuid()) RETURNING id`).Scan(&id); err != nil {
			return err
		}
		a, err = loadArticle(ctx, tx, id)
	} else {
		a, err = lockArticle(ctx, tx, *article, *version)
	}
	if err != nil {
		return err
	}
	r := Revision{ArticleID: a.ID, Title: parsed.Title, Summary: parsed.Summary, Language: parsed.Language, WrittenAt: parsed.WrittenAt,
		Markdown: parsed.Markdown, Tags: []Label{}, SourceIssues: append([]Issue{}, parsed.Issues...)}
	base := a.Draft
	if base == nil {
		base = a.Published
	}
	if base != nil {
		if !parsed.Fields["summary"] {
			r.Summary = base.Summary
		}
		if !parsed.Fields["language"] {
			r.Language = base.Language
		}
		if !parsed.Fields["written_at"] {
			r.WrittenAt = base.WrittenAt
		}
		if !parsed.Fields["category"] {
			r.Category = base.Category
		}
		if !parsed.Fields["tags"] {
			r.Tags = base.Tags
		}
		inherited := map[string]string{"summary": "summary", "language": "language", "written_at": "written_at", "category_id": "category", "tag_ids": "tags"}
		for _, item := range base.SourceIssues {
			if item.Path != nil && inherited[*item.Path] != "" && !parsed.Fields[inherited[*item.Path]] {
				r.SourceIssues = append(r.SourceIssues, item)
			}
		}
	}
	if r.Language == nil && !parsed.Fields["language"] && base == nil {
		language := "en"
		r.Language = &language
	}
	if parsed.Fields["category"] && parsed.Category != nil {
		label, err := resolveLabel(ctx, tx, "categories", *parsed.Category)
		if err != nil {
			r.SourceIssues = append(r.SourceIssues, issue("blocking", "category_unresolved", "category_id", "Select or clear the imported category."))
		} else {
			r.Category = label
		}
	}
	if parsed.Fields["tags"] {
		unresolved := false
		for _, name := range parsed.Tags {
			label, err := resolveLabel(ctx, tx, "tags", name)
			if err != nil {
				if !unresolved {
					r.SourceIssues = append(r.SourceIssues, issue("blocking", "tag_unresolved", "tag_ids", "Select or clear the imported tags."))
					unresolved = true
				}
			} else {
				duplicate := false
				for _, existing := range r.Tags {
					if existing.ID == label.ID {
						duplicate = true
					}
				}
				if !duplicate {
					r.Tags = append(r.Tags, *label)
				}
			}
		}
	}
	if article == nil {
		candidate := strings.Trim(slugSeparators.ReplaceAllString(strings.ToLower(strings.TrimSuffix(filename, path.Ext(filename))), "-"), "-")
		if len(candidate) > 160 {
			candidate = strings.TrimRight(candidate[:160], "-")
		}
		if candidate == "" {
			candidate = "article-" + a.ID[:8]
		}
		if parsed.Slug != nil {
			candidate = *parsed.Slug
		}
		if validSlug(candidate) {
			if _, err := tx.ExecContext(ctx, `SAVEPOINT suggested_slug`); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `UPDATE articles SET slug=$1 WHERE id=$2`, candidate, a.ID)
			if err != nil {
				var pg *pgconn.PgError
				if !errors.As(err, &pg) || pg.Code != "23505" {
					return err
				}
				if _, err := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT suggested_slug`); err != nil {
					return err
				}
				r.SourceIssues = append(r.SourceIssues, issue("blocking", "slug_conflict", "slug", "This URL is already used. Choose another URL."))
			} else {
				a.Slug = &candidate
			}
		} else {
			r.SourceIssues = append(r.SourceIssues, issue("blocking", "invalid_slug", "slug", "Choose a valid article URL."))
		}
	} else if parsed.Slug != nil && (a.Slug == nil || *parsed.Slug != *a.Slug) {
		r.SourceIssues = append(r.SourceIssues, issue("warning", "slug_ignored", "slug", "An update keeps the existing article URL."))
	}
	if err := insertRevision(ctx, tx, &r); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE articles SET draft_revision_id=$1,version=CASE WHEN $3 THEN version ELSE version+1 END,modified_at=clock_timestamp() WHERE id=$2`, r.ID, a.ID, article == nil); err != nil {
		return err
	}
	issues, _ := json.Marshal(revisionIssues(&r, a.Slug))
	if _, err := tx.ExecContext(ctx, `UPDATE import_jobs SET status='succeeded',article_id=$1,result_revision_id=$2,issues=$3,finished_at=clock_timestamp() WHERE id=$4 AND status='processing'`, a.ID, r.ID, issues, jobID); err != nil {
		return err
	}
	return tx.Commit()
}

func resolveLabel(ctx context.Context, q queryer, table, name string) (*Label, error) {
	// table is selected only by internal calls, never from request input.
	rows, err := q.QueryContext(ctx, `SELECT id,name,slug FROM `+table+` WHERE lower(name)=lower($1) LIMIT 2`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var found []Label
	for rows.Next() {
		var label Label
		if err := rows.Scan(&label.ID, &label.Name, &label.Slug); err != nil {
			return nil, err
		}
		found = append(found, label)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(found) != 1 {
		return nil, errors.New("label_unresolved")
	}
	return &found[0], nil
}

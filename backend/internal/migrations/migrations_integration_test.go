//go:build integration

package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/IiCEight/iSaber/backend/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
)

// Each test owns a fresh schema; it never migrates the connection's default
// schema. The explicit test URL must point to a disposable development database.
func isolatedDatabase(t *testing.T) (*sql.DB, context.Context) {
	t.Helper()
	connection := os.Getenv("TEST_DATABASE_URL")
	if connection == "" {
		t.Fatal("integration tests require TEST_DATABASE_URL for a disposable PostgreSQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	admin, err := database.Open(connection)
	if err != nil {
		t.Fatal("invalid integration database configuration")
	}
	if err := admin.PingContext(ctx); err != nil {
		admin.Close()
		t.Fatal("integration database is unavailable")
	}
	schema := fmt.Sprintf("isaber_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+quoted); err != nil {
		admin.Close()
		t.Fatal("could not create isolated integration schema")
	}
	config, err := pgx.ParseConfig(connection)
	if err != nil {
		t.Fatal("invalid integration configuration")
	}
	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	db.SetMaxOpenConns(5)
	t.Cleanup(func() {
		db.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := admin.ExecContext(cleanupCtx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error("could not clean isolated integration schema")
		}
		admin.Close()
	})
	return db, ctx
}

func migratedDatabase(t *testing.T) (*sql.DB, context.Context) {
	t.Helper()
	db, ctx := isolatedDatabase(t)
	provider, err := Provider(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func mustExec(t *testing.T, db *sql.DB, ctx context.Context, query string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		t.Fatal(err)
	}
}

func mustReject(t *testing.T, db *sql.DB, ctx context.Context, code, query string, args ...any) {
	t.Helper()
	_, err := db.ExecContext(ctx, query, args...)
	var pgError *pgconn.PgError
	if !errors.As(err, &pgError) || pgError.Code != code {
		t.Fatalf("expected PostgreSQL error %s, got %v", code, err)
	}
}

func TestUpgradePreservesDataAndRepeatedUpIsSafe(t *testing.T) {
	db, ctx := isolatedDatabase(t)
	if err := CheckReady(ctx, db); err == nil {
		t.Fatal("unmigrated database must not be ready")
	}
	provider, err := Provider(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 1); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, ctx, `INSERT INTO owners(id, username, password_hash) VALUES
        ('11111111-1111-4111-8111-111111111111', 'fixture-owner', 'synthetic-hash-not-a-password')`)
	if err := CheckReady(ctx, db); err == nil {
		t.Fatal("partially migrated database must not be ready")
	}
	results, err := provider.Up(ctx)
	if err != nil || len(results) != int(RequiredVersion-1) {
		t.Fatalf("upgrade: applied %d migrations, error %v", len(results), err)
	}
	if err := CheckReady(ctx, db); err != nil {
		t.Fatal("fully migrated database must be ready")
	}
	version, err := provider.GetDBVersion(ctx)
	if err != nil || version != RequiredVersion {
		t.Fatalf("embedded migrations and required schema version disagree: version %d, error %v", version, err)
	}
	results, err = provider.Up(ctx)
	if err != nil || len(results) != 0 {
		t.Fatalf("repeat up: applied %d migrations, error %v", len(results), err)
	}
	var username string
	if err := db.QueryRowContext(ctx, "SELECT username FROM owners").Scan(&username); err != nil || username != "fixture-owner" {
		t.Fatal("upgrade did not preserve owner data")
	}
}

func TestArticleRevisionAndAssetOwnership(t *testing.T) {
	db, ctx := migratedDatabase(t)
	articleA := "11111111-1111-4111-8111-111111111111"
	articleB := "22222222-2222-4222-8222-222222222222"
	revisionA := "33333333-3333-4333-8333-333333333333"
	revisionB := "44444444-4444-4444-8444-444444444444"
	assetA := "55555555-5555-4555-8555-555555555555"
	assetB := "66666666-6666-4666-8666-666666666666"
	for _, id := range []string{articleA, articleB} {
		mustExec(t, db, ctx, "INSERT INTO articles(id) VALUES ($1)", id)
	}
	mustExec(t, db, ctx, "INSERT INTO revisions(id, article_id, markdown) VALUES ($1,$2,'draft A'),($3,$4,'draft B')", revisionA, articleA, revisionB, articleB)
	mustExec(t, db, ctx, "UPDATE articles SET draft_revision_id=$1 WHERE id=$2", revisionA, articleA)
	mustReject(t, db, ctx, "23503", "UPDATE articles SET draft_revision_id=$1 WHERE id=$2", revisionB, articleA)
	mustReject(t, db, ctx, "23503", "UPDATE articles SET slug='article-a', published_at=now(), updated_at=now(), published_revision_id=$1 WHERE id=$2", revisionB, articleA)
	mustReject(t, db, ctx, "23503", "UPDATE revisions SET restored_from_revision_id=$1 WHERE id=$2", revisionB, revisionA)
	for _, pair := range [][2]string{{assetA, articleA}, {assetB, articleB}} {
		mustExec(t, db, ctx, `INSERT INTO assets(id,article_id,storage_key,media_type,byte_size,checksum)
            VALUES ($1,$2,$1::uuid::text,'image/png',10,decode(repeat('00',32),'hex'))`, pair[0], pair[1])
	}
	mustReject(t, db, ctx, "23503", "INSERT INTO revision_assets(article_id,revision_id,asset_id,original_path) VALUES ($1,$2,$3,'image.png')", articleA, revisionA, assetB)
	mustExec(t, db, ctx, "INSERT INTO revision_assets(article_id,revision_id,asset_id,original_path) VALUES ($1,$2,$3,'image.png')", articleA, revisionA, assetA)
	var draft string
	if err := db.QueryRowContext(ctx, "SELECT draft_revision_id FROM articles WHERE id=$1", articleA).Scan(&draft); err != nil || draft != revisionA {
		t.Fatal("rejected cross-article writes changed the existing draft")
	}
}

func TestRevisionUpdatedAtUpgradePreservesArticleDates(t *testing.T) {
	db, ctx := isolatedDatabase(t)
	provider, err := Provider(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 5); err != nil {
		t.Fatal(err)
	}
	article := "11111111-1111-4111-8111-111111111111"
	revision := "22222222-2222-4222-8222-222222222222"
	mustExec(t, db, ctx, `INSERT INTO articles(id,slug,published_at,updated_at) VALUES($1,'existing','2024-06-01T00:00:00Z','2024-07-01T00:00:00Z')`, article)
	mustExec(t, db, ctx, `INSERT INTO revisions(id,article_id,markdown) VALUES($1,$2,'Existing content')`, revision, article)
	mustExec(t, db, ctx, `UPDATE articles SET published_revision_id=$1 WHERE id=$2`, revision, article)
	if err := CheckReady(ctx, db); err == nil {
		t.Fatal("version 5 must require the revision timestamp migration")
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var published, updated time.Time
	var override *time.Time
	var markdown string
	if err := db.QueryRowContext(ctx, `SELECT a.published_at,a.updated_at,r.updated_at,r.markdown FROM articles a JOIN revisions r ON r.id=a.published_revision_id WHERE a.id=$1`, article).Scan(&published, &updated, &override, &markdown); err != nil {
		t.Fatal(err)
	}
	if published.UTC().Format(time.RFC3339) != "2024-06-01T00:00:00Z" || updated.UTC().Format(time.RFC3339) != "2024-07-01T00:00:00Z" || override != nil || markdown != "Existing content" {
		t.Fatal("migration changed existing article dates or content")
	}
}

func TestImportResultAndIdempotencyConstraints(t *testing.T) {
	db, ctx := migratedDatabase(t)
	articleA := "11111111-1111-4111-8111-111111111111"
	articleB := "22222222-2222-4222-8222-222222222222"
	revision := "33333333-3333-4333-8333-333333333333"
	job := "44444444-4444-4444-8444-444444444444"
	mustExec(t, db, ctx, "INSERT INTO articles(id) VALUES ($1),($2)", articleA, articleB)
	mustExec(t, db, ctx, "INSERT INTO revisions(id,article_id,markdown) VALUES ($1,$2,'draft')", revision, articleB)
	mustReject(t, db, ctx, "23503", `INSERT INTO import_jobs(id,article_id,upload_storage_key,status,result_revision_id,finished_at)
        VALUES ($1,$2,'staged-upload','succeeded',$3,now())`, job, articleA, revision)
	mustExec(t, db, ctx, `INSERT INTO import_jobs(id,article_id,upload_storage_key,status,result_revision_id,finished_at)
        VALUES ($1,$2,'staged-upload','succeeded',$3,now())`, job, articleB, revision)
	key := "55555555-5555-4555-8555-555555555555"
	query := `INSERT INTO idempotency_records(scope,key,request_hash,state,expires_at)
        VALUES ($1,$2,decode(repeat('00',32),'hex'),'in_progress',now()+interval '24 hours')`
	mustExec(t, db, ctx, query, "owner:publish:article-a", key)
	mustReject(t, db, ctx, "23505", query, "owner:publish:article-a", key)
	mustExec(t, db, ctx, query, "owner:publish:article-b", key)
	mustReject(t, db, ctx, "23514", "UPDATE idempotency_records SET state='completed' WHERE scope=$1 AND key=$2", "owner:publish:article-a", key)
}

func TestOwnerPrivacyAndCommentConstraints(t *testing.T) {
	db, ctx := migratedDatabase(t)
	mustExec(t, db, ctx, `INSERT INTO owners(id,username,password_hash) VALUES
        ('11111111-1111-4111-8111-111111111111','owner','synthetic-hash')`)
	mustReject(t, db, ctx, "23505", `INSERT INTO owners(id,username,password_hash) VALUES
        ('22222222-2222-4222-8222-222222222222','another-owner','synthetic-hash')`)
	article := "33333333-3333-4333-8333-333333333333"
	mustExec(t, db, ctx, "INSERT INTO articles(id) VALUES ($1)", article)
	var visibility string
	if err := db.QueryRowContext(ctx, "SELECT visibility FROM articles WHERE id=$1", article).Scan(&visibility); err != nil || visibility != "private" {
		t.Fatal("new articles must default to private")
	}
	mustReject(t, db, ctx, "23514", "UPDATE articles SET visibility='members' WHERE id=$1", article)
	mustReject(t, db, ctx, "23514", "UPDATE articles SET version=0 WHERE id=$1", article)
	comment := "44444444-4444-4444-8444-444444444444"
	mustReject(t, db, ctx, "23514", "INSERT INTO comments(id,article_id,display_name,body) VALUES ($1,$2,'Reader',E' \\n\\t')", comment, article)
	mustExec(t, db, ctx, "INSERT INTO comments(id,article_id,display_name,body) VALUES ($1,$2,'Reader','中文 feedback')", comment, article)
	var status string
	if err := db.QueryRowContext(ctx, "SELECT status FROM comments WHERE id=$1", comment).Scan(&status); err != nil || status != "pending" {
		t.Fatal("new comments must await moderation")
	}
	mustReject(t, db, ctx, "23514", "UPDATE comments SET status='approved' WHERE id=$1", comment)
}

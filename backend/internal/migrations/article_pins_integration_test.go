//go:build integration

package migrations

import (
	"testing"
	"time"
)

func TestArticlePinsUpgradeAndPreviousUnpublishCompatibility(t *testing.T) {
	db, ctx := isolatedDatabase(t)
	provider, err := Provider(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 7); err != nil {
		t.Fatal(err)
	}
	const article = "11111111-1111-4111-8111-111111111111"
	const revision = "22222222-2222-4222-8222-222222222222"
	mustExec(t, db, ctx, `INSERT INTO articles(id,slug,visibility,published_at,updated_at) VALUES($1,'existing-private','private','2024-06-01T00:00:00Z','2024-07-01T00:00:00Z')`, article)
	mustExec(t, db, ctx, `INSERT INTO revisions(id,article_id,title,language,written_at,markdown) VALUES($1,$2,'Existing content','en','2024-01-01','Existing body')`, revision, article)
	mustExec(t, db, ctx, `UPDATE articles SET published_revision_id=$1 WHERE id=$2`, revision, article)
	if CheckReady(ctx, db) == nil {
		t.Fatal("new API accepted schema without pins")
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := CheckReady(ctx, db); err != nil {
		t.Fatal(err)
	}
	var pinned bool
	var published, updated time.Time
	var visibility, markdown string
	if err := db.QueryRowContext(ctx, `SELECT a.pinned,a.published_at,a.updated_at,a.visibility,r.markdown FROM articles a JOIN revisions r ON r.id=a.published_revision_id WHERE a.id=$1`, article).Scan(&pinned, &published, &updated, &visibility, &markdown); err != nil {
		t.Fatal(err)
	}
	if pinned || published.UTC().Format(time.RFC3339) != "2024-06-01T00:00:00Z" || updated.UTC().Format(time.RFC3339) != "2024-07-01T00:00:00Z" || visibility != "private" || markdown != "Existing body" {
		t.Fatal("migration changed existing article data")
	}
	mustExec(t, db, ctx, `UPDATE articles SET pinned=true WHERE id=$1`, article)
	// The preceding application does not know about pins and updates only the pointer.
	mustExec(t, db, ctx, `UPDATE articles SET published_revision_id=NULL WHERE id=$1`, article)
	if err := db.QueryRowContext(ctx, `SELECT pinned FROM articles WHERE id=$1`, article).Scan(&pinned); err != nil || pinned {
		t.Fatal("preceding binary cannot unpublish a pinned article")
	}
	if _, err := db.ExecContext(ctx, `UPDATE articles SET pinned=true WHERE id=$1`, article); err == nil {
		t.Fatal("database accepted a pinned draft")
	}
	if pending, err := provider.Up(ctx); err != nil || len(pending) != 0 {
		t.Fatal("migration is not repeatable")
	}
}

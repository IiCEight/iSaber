//go:build integration

package migrations

import "testing"

func TestCategoryPagesUpgradePreservesDataAndOldPublisher(t *testing.T) {
	db, ctx := isolatedDatabase(t)
	provider, err := Provider(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 8); err != nil {
		t.Fatal(err)
	}
	const article = "11111111-1111-4111-8111-111111111111"
	const revision = "22222222-2222-4222-8222-222222222222"
	const category = "33333333-3333-4333-8333-333333333333"
	mustExec(t, db, ctx, `INSERT INTO categories(id,name,slug) VALUES($1,'Existing category','existing')`, category)
	mustExec(t, db, ctx, `INSERT INTO articles(id,slug,visibility,published_at,updated_at) VALUES($1,'existing','private','2024-01-01','2024-02-01')`, article)
	mustExec(t, db, ctx, `INSERT INTO revisions(id,article_id,title,language,written_at,markdown,category_id) VALUES($1,$2,'Existing','en','2023-01-01','Existing Markdown',$3)`, revision, article, category)
	mustExec(t, db, ctx, `UPDATE articles SET published_revision_id=$2,pinned=true WHERE id=$1`, article, revision)
	if CheckReady(ctx, db) == nil {
		t.Fatal("schema 8 accepted")
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var description, markdown, dates string
	var global, local bool
	if err := db.QueryRowContext(ctx, `SELECT c.description_en,r.markdown,a.pinned,a.category_pinned,to_char(a.updated_at AT TIME ZONE 'UTC','YYYY-MM-DD') FROM articles a JOIN revisions r ON r.id=a.published_revision_id JOIN categories c ON c.id=r.category_id WHERE a.id=$1`, article).Scan(&description, &markdown, &global, &local, &dates); err != nil {
		t.Fatal(err)
	}
	if description != "" || markdown != "Existing Markdown" || !global || local || dates != "2024-02-01" {
		t.Fatal("migration changed existing content")
	}
	mustExec(t, db, ctx, `UPDATE articles SET category_pinned=true WHERE id=$1`, article)
	// An older binary changes only the published pointer: the DB clears both pins.
	mustExec(t, db, ctx, `UPDATE articles SET published_revision_id=NULL WHERE id=$1`, article)
	if err := db.QueryRowContext(ctx, `SELECT pinned,category_pinned FROM articles WHERE id=$1`, article).Scan(&global, &local); err != nil || global || local {
		t.Fatal("old publisher cannot unpublish")
	}
	if _, err := db.ExecContext(ctx, `UPDATE articles SET category_pinned=true WHERE id=$1`, article); err == nil {
		t.Fatal("draft pin accepted")
	}
	if results, err := provider.Up(ctx); err != nil || len(results) != 0 {
		t.Fatal("migration not repeatable")
	}
}

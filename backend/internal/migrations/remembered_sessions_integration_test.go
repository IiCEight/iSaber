//go:build integration

package migrations

import (
	"bytes"
	"testing"
	"time"
)

func TestRememberedSessionsUpgradeKeepsExistingSessionsShort(t *testing.T) {
	db, ctx := isolatedDatabase(t)
	provider, err := Provider(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 6); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, ctx, `INSERT INTO owners(id,username,password_hash) VALUES ('11111111-1111-4111-8111-111111111111','owner','synthetic-hash')`)
	mustExec(t, db, ctx, `INSERT INTO sessions(id,owner_id,token_hash,csrf_token_hash,expires_at)
 VALUES ('22222222-2222-4222-8222-222222222222','11111111-1111-4111-8111-111111111111',decode(repeat('01',32),'hex'),decode(repeat('02',32),'hex'),now()+interval '8 hours')`)
	var expiry, seen time.Time
	if err := db.QueryRowContext(ctx, `SELECT expires_at,last_seen_at FROM sessions`).Scan(&expiry, &seen); err != nil {
		t.Fatal(err)
	}
	if err := CheckReady(ctx, db); err == nil {
		t.Fatal("New API must require migration 00007")
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var afterExpiry, afterSeen time.Time
	var remembered bool
	var token, csrf []byte
	if err := db.QueryRowContext(ctx, `SELECT remember_me,expires_at,last_seen_at,token_hash,csrf_token_hash FROM sessions`).Scan(&remembered, &afterExpiry, &afterSeen, &token, &csrf); err != nil {
		t.Fatal(err)
	}
	if remembered || !expiry.Equal(afterExpiry) || !seen.Equal(afterSeen) || !bytes.Equal(token, bytes.Repeat([]byte{1}, 32)) || !bytes.Equal(csrf, bytes.Repeat([]byte{2}, 32)) {
		t.Fatal("Migration changed existing session credentials, dates, or idle policy")
	}
	if err := CheckReady(ctx, db); err != nil {
		t.Fatal(err)
	}
}

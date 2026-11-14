// Package migrations embeds forward-only PostgreSQL migrations in the binary.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// RequiredVersion is the minimum compatible schema for this application release.
// Increase it when adding a migration that a new binary requires. Future additive
// migrations must keep older binaries compatible during rolling deployments.
const RequiredVersion int64 = 6

//go:embed *.sql
var files embed.FS

func Provider(db *sql.DB) (*goose.Provider, error) {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, err
	}
	return goose.NewProvider(goose.DialectPostgres, db, files, goose.WithSessionLocker(locker))
}

// CheckReady is read-only; API processes never create migration tables or apply DDL.
func CheckReady(ctx context.Context, db *sql.DB) error {
	var version int64
	if err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version_id) FILTER (WHERE is_applied), 0) FROM goose_db_version`,
	).Scan(&version); err != nil {
		return fmt.Errorf("schema unavailable")
	}
	if version < RequiredVersion {
		return fmt.Errorf("schema migrations required")
	}
	return nil
}

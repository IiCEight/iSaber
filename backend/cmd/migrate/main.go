// migrate is an explicit operator command. The API never runs it automatically.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/IiCEight/iSaber/backend/internal/database"
	"github.com/IiCEight/iSaber/backend/internal/migrations"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		// SQL errors can include row data; keep failure output generic.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 || (args[0] != "up" && args[0] != "status") {
		return errors.New("usage: migrate up|status (requires MIGRATION_DATABASE_URL)")
	}
	if os.Getenv("MIGRATION_DATABASE_URL") == "" {
		return errors.New("MIGRATION_DATABASE_URL is required")
	}
	db, err := database.Open(os.Getenv("MIGRATION_DATABASE_URL"))
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	provider, err := migrations.Provider(db)
	if err != nil {
		return errors.New("could not initialize migrations")
	}
	if args[0] == "up" {
		results, err := provider.Up(ctx)
		if err != nil {
			return errors.New("migration failed; inspect database availability, migration history, and operator permissions without exposing connection secrets")
		}
		fmt.Printf("Applied %d migrations\n", len(results))
		return nil
	}
	statuses, err := provider.Status(ctx)
	if err != nil {
		return errors.New("could not read migration status")
	}
	for _, status := range statuses {
		fmt.Printf("%d %s\n", status.Source.Version, status.State)
	}
	return nil
}

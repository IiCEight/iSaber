// Package database configures the PostgreSQL pool without applying migrations.
package database

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func Open(connectionString string) (*sql.DB, error) {
	if strings.TrimSpace(connectionString) == "" {
		return nil, errors.New("database connection configuration is required")
	}
	config, err := pgx.ParseConfig(connectionString)
	if err != nil {
		// Driver errors may include connection strings. Never log credentials.
		return nil, errors.New("invalid database connection configuration")
	}
	config.ConnectTimeout = 3 * time.Second
	config.RuntimeParams["application_name"] = "isaber"
	db := stdlib.OpenDB(*config)
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	return db, nil
}

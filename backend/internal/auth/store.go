package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrUnauthorized = errors.New("invalid or expired owner credentials")
	ErrOwnerExists  = errors.New("an owner already exists; setup does not replace credentials")
	ErrOwnerMissing = errors.New("owner not found")
)

type Owner struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func normalizeUsername(username string) (string, error) {
	username = strings.TrimSpace(username)
	if !utf8.ValidString(username) || username == "" || utf8.RuneCountInString(username) > 120 {
		return "", errors.New("username must contain 1 to 120 characters")
	}
	return username, nil
}

func (s *Store) Setup(ctx context.Context, username, password string) error {
	username, err := normalizeUsername(username)
	if err != nil {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO owners(id,username,password_hash)
        VALUES(gen_random_uuid(),$1,$2)`, username, hash)
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		return ErrOwnerExists
	}
	if err != nil {
		return errors.New("could not create owner; verify the database schema and operator permissions")
	}
	return nil
}

// Recover changes credentials and revokes sessions in one transaction. Owner row
// locking also serializes recovery with a login that verified the previous hash.
func (s *Store) Recover(ctx context.Context, username, password string) error {
	username, err := normalizeUsername(username)
	if err != nil {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("could not begin owner recovery")
	}
	defer tx.Rollback()
	var ownerID string
	err = tx.QueryRowContext(ctx, `UPDATE owners SET password_hash=$1, credentials_changed_at=clock_timestamp()
        WHERE lower(username)=lower($2) RETURNING id`, hash, username).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrOwnerMissing
	}
	if err != nil {
		return errors.New("could not update owner credentials")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=clock_timestamp()
        WHERE owner_id=$1 AND revoked_at IS NULL`, ownerID); err != nil {
		return errors.New("could not revoke owner sessions")
	}
	if err := tx.Commit(); err != nil {
		return errors.New("could not commit owner recovery")
	}
	return nil
}

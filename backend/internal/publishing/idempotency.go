package publishing

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

type response struct {
	Status   int
	Body     []byte
	Replayed bool
}

// Only committed operations consume a key. Validation/transaction failures may
// be retried; an uncertain commit must be resolved by retrying the same key.
// Guards run before acceptance and replay, and can hold eligibility row locks.
func (s *Service) idempotent(ctx context.Context, scope, key string, payload []byte, action func(*sql.Tx) (int, any, error), guards ...func(*sql.Tx) error) (response, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return response{}, err
	}
	defer tx.Rollback()
	for _, guard := range guards {
		if err := guard(tx); err != nil {
			return response{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM idempotency_records WHERE scope=$1 AND key=$2 AND expires_at<=clock_timestamp()`, scope, key); err != nil {
		return response{}, err
	}
	if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout='250ms'`); err != nil {
		return response{}, err
	}
	hash := sha256.Sum256(payload)
	var inserted string
	err = tx.QueryRowContext(ctx, `INSERT INTO idempotency_records(scope,key,request_hash,state,expires_at)
        VALUES($1,$2,$3,'in_progress',clock_timestamp()+interval '24 hours') ON CONFLICT DO NOTHING RETURNING scope`, scope, key, hash[:]).Scan(&inserted)
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "55P03" {
		return response{}, fault(409, "request_in_progress")
	}
	if errors.Is(err, sql.ErrNoRows) {
		var stored []byte
		var state string
		var result response
		var status sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT request_hash,state,response_status,response_body FROM idempotency_records WHERE scope=$1 AND key=$2`, scope, key).Scan(&stored, &state, &status, &result.Body); err != nil {
			return response{}, err
		}
		if subtle.ConstantTimeCompare(stored, hash[:]) != 1 {
			return response{}, fault(409, "idempotency_conflict")
		}
		if state != "completed" {
			return response{}, fault(409, "request_in_progress")
		}
		result.Status = int(status.Int64)
		result.Replayed = true
		return result, nil
	}
	if err != nil {
		return response{}, err
	}
	if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout='0'`); err != nil {
		return response{}, err
	}
	status, value, err := action(tx)
	if err != nil {
		return response{}, err
	}
	body, err := json.Marshal(value)
	if err != nil {
		return response{}, err
	}
	// Return the stored JSON representation on both first acceptance and replay.
	if err := tx.QueryRowContext(ctx, `UPDATE idempotency_records SET state='completed',response_status=$3,response_body=$4 WHERE scope=$1 AND key=$2 RETURNING response_body`, scope, key, status, body).Scan(&body); err != nil {
		return response{}, err
	}
	if err := tx.Commit(); err != nil {
		return response{}, err
	}
	return response{Status: status, Body: body}, nil
}

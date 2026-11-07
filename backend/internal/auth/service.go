package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"
)

const (
	AbsoluteLifetime = 8 * time.Hour
	IdleLifetime     = 30 * time.Minute
)

type Session struct {
	Owner     Owner     `json:"owner"`
	CSRFToken string    `json:"csrf_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Service struct {
	store     *Store
	secret    []byte
	dummyHash string
}

func NewService(store *Store, secret []byte) (*Service, error) {
	if len(secret) != 32 {
		return nil, errors.New("CSRF key must be 32 bytes")
	}
	dummy, err := HashPassword("dummy-password-for-timing-only")
	if err != nil {
		return nil, err
	}
	return &Service{store: store, secret: append([]byte(nil), secret...), dummyHash: dummy}, nil
}

func (s *Service) csrf(token string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte("isaber-csrf-v1:" + token))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func digest(value string) []byte {
	result := sha256.Sum256([]byte(value))
	return result[:]
}

func (s *Service) Login(ctx context.Context, username, password string) (Session, string, error) {
	var owner Owner
	var hash string
	err := s.store.db.QueryRowContext(ctx, `SELECT id,username,password_hash FROM owners WHERE lower(username)=lower($1)`, username).
		Scan(&owner.ID, &owner.Username, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		verifyPassword(password, s.dummyHash)
		return Session{}, "", ErrUnauthorized
	}
	if err != nil {
		return Session{}, "", errors.New("owner storage unavailable")
	}
	if !verifyPassword(password, hash) {
		return Session{}, "", ErrUnauthorized
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return Session{}, "", errors.New("could not create owner session")
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	csrf := s.csrf(token)
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, "", errors.New("session storage unavailable")
	}
	defer tx.Rollback()
	var currentHash string
	// Recovery obtains an exclusive owner lock before changing credentials and
	// revoking sessions. Hold this shared lock through session creation.
	if err := tx.QueryRowContext(ctx, `SELECT password_hash FROM owners WHERE id=$1 FOR SHARE`, owner.ID).Scan(&currentHash); err != nil {
		return Session{}, "", errors.New("owner storage unavailable")
	}
	if currentHash != hash {
		return Session{}, "", ErrUnauthorized
	}
	var expires time.Time
	err = tx.QueryRowContext(ctx, `INSERT INTO sessions(id,owner_id,token_hash,csrf_token_hash,expires_at)
        VALUES(gen_random_uuid(),$1,$2,$3,now()+make_interval(secs => $4)) RETURNING expires_at`,
		owner.ID, digest(token), digest(csrf), AbsoluteLifetime.Seconds()).Scan(&expires)
	if err != nil {
		return Session{}, "", errors.New("could not store owner session")
	}
	if err := tx.Commit(); err != nil {
		return Session{}, "", errors.New("could not commit owner session")
	}
	return Session{Owner: owner, CSRFToken: csrf, ExpiresAt: expires.UTC()}, token, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (Session, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != 32 || base64.RawURLEncoding.EncodeToString(decoded) != token {
		return Session{}, ErrUnauthorized
	}
	var session Session
	csrf := s.csrf(token)
	err = s.store.db.QueryRowContext(ctx, `UPDATE sessions AS s SET last_seen_at=clock_timestamp()
        FROM owners AS o WHERE s.owner_id=o.id AND s.token_hash=$1 AND s.csrf_token_hash=$2 AND s.revoked_at IS NULL
        AND s.expires_at>clock_timestamp() AND s.last_seen_at>clock_timestamp()-make_interval(secs => $3)
        RETURNING o.id,o.username,s.expires_at`, digest(token), digest(csrf), IdleLifetime.Seconds()).
		Scan(&session.Owner.ID, &session.Owner.Username, &session.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrUnauthorized
	}
	if err != nil {
		return Session{}, errors.New("session storage unavailable")
	}
	// All validity predicates are checked in the same UPDATE. Rejected cookies,
	// including cookies derived under a different key, never renew the idle timer.
	session.CSRFToken = csrf
	session.ExpiresAt = session.ExpiresAt.UTC()
	return session, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	_, err := s.store.db.ExecContext(ctx, `UPDATE sessions SET revoked_at=clock_timestamp() WHERE token_hash=$1`, digest(token))
	if err != nil {
		return errors.New("session storage unavailable")
	}
	return nil
}

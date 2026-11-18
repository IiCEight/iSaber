package publishing

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type friendDetails struct {
	SiteName     string `json:"site_name"`
	SiteURL      string `json:"site_url"`
	Introduction string `json:"introduction"`
	IconURL      string `json:"icon_url"`
}
type publicFriend struct {
	ID string `json:"id"`
	friendDetails
}
type adminFriend struct {
	publicFriend
	Published bool      `json:"published"`
	Position  int       `json:"position"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type friendRequest struct {
	ID string `json:"id"`
	friendDetails
	Email       string     `json:"email"`
	Status      string     `json:"status"`
	FriendID    *string    `json:"friend_id"`
	Version     int64      `json:"version"`
	CreatedAt   time.Time  `json:"created_at"`
	ModeratedAt *time.Time `json:"moderated_at"`
}

const friendColumns = `id,site_name,site_url,introduction,icon_url,published,position,version,created_at,updated_at`
const requestColumns = `id,site_name,site_url,introduction,icon_url,email,status,friend_id,version,created_at,moderated_at`

func httpsFriendURL(value string, optional bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" && optional {
		return "", nil
	}
	u, err := url.Parse(value)
	if err != nil || len(value) > 2048 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || strings.ContainsAny(value, "\x00\r\n\t") {
		return "", fault(422, "invalid_friend_url")
	}
	u.Host = strings.ToLower(u.Host)
	return u.String(), nil
}
func parseFriendDetails(fields map[string]json.RawMessage, extra ...string) (friendDetails, error) {
	var d friendDetails
	allowed := map[string]bool{"site_name": true, "site_url": true, "introduction": true, "icon_url": true}
	for _, key := range extra {
		allowed[key] = true
	}
	for key := range fields {
		if !allowed[key] {
			return d, fault(422, "invalid_friend")
		}
	}
	for key, dest := range map[string]*string{"site_name": &d.SiteName, "site_url": &d.SiteURL, "introduction": &d.Introduction, "icon_url": &d.IconURL} {
		raw, ok := fields[key]
		if !ok && key == "icon_url" {
			continue
		}
		if !ok || string(raw) == "null" || json.Unmarshal(raw, dest) != nil || !utf8.ValidString(*dest) || strings.ContainsRune(*dest, 0) {
			return d, fault(422, "invalid_friend")
		}
		*dest = strings.TrimSpace(*dest)
	}
	if utf8.RuneCountInString(d.SiteName) < 1 || utf8.RuneCountInString(d.SiteName) > 80 || utf8.RuneCountInString(d.Introduction) < 1 || utf8.RuneCountInString(d.Introduction) > 300 {
		return d, fault(422, "invalid_friend")
	}
	var err error
	d.SiteURL, err = httpsFriendURL(d.SiteURL, false)
	if err != nil {
		return d, err
	}
	d.IconURL, err = httpsFriendURL(d.IconURL, true)
	return d, err
}
func scanFriend(row interface{ Scan(...any) error }) (adminFriend, error) {
	var f adminFriend
	err := row.Scan(&f.ID, &f.SiteName, &f.SiteURL, &f.Introduction, &f.IconURL, &f.Published, &f.Position, &f.Version, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = fault(404, "not_found")
	}
	f.CreatedAt = f.CreatedAt.UTC()
	f.UpdatedAt = f.UpdatedAt.UTC()
	return f, err
}
func scanFriendRequest(row interface{ Scan(...any) error }) (friendRequest, error) {
	var f friendRequest
	err := row.Scan(&f.ID, &f.SiteName, &f.SiteURL, &f.Introduction, &f.IconURL, &f.Email, &f.Status, &f.FriendID, &f.Version, &f.CreatedAt, &f.ModeratedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = fault(404, "not_found")
	}
	f.CreatedAt = f.CreatedAt.UTC()
	utc(f.ModeratedAt)
	return f, err
}

// Serialize changes to directory membership/order so stale reorder operations
// cannot silently overwrite another tab's edits or omit a newly created entry.
func lockFriends(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(7311174)`)
	return err
}
func insertFriend(ctx context.Context, tx *sql.Tx, d friendDetails) (adminFriend, error) {
	var count, position int
	if err := tx.QueryRowContext(ctx, `SELECT count(*),COALESCE(max(position),-1)+1 FROM friends`).Scan(&count, &position); err != nil {
		return adminFriend{}, err
	}
	if count >= 200 {
		return adminFriend{}, fault(409, "friend_directory_full")
	}
	return scanFriend(tx.QueryRowContext(ctx, `INSERT INTO friends(id,site_name,site_url,introduction,icon_url,position) VALUES($1,$2,$3,$4,$5,$6) RETURNING `+friendColumns, uuid.NewString(), d.SiteName, d.SiteURL, d.Introduction, d.IconURL, position))
}
func (s *Service) listFriends(w http.ResponseWriter, r *http.Request, admin bool) {
	if len(r.URL.Query()) != 0 {
		reply(w, nil, fault(400, "invalid_friend_parameters"))
		return
	}
	query := `SELECT ` + friendColumns + ` FROM friends`
	if !admin {
		query += ` WHERE published`
	}
	rows, err := s.db.QueryContext(r.Context(), query+` ORDER BY position,id`)
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer rows.Close()
	owners := []adminFriend{}
	public := []publicFriend{}
	for rows.Next() {
		f, err := scanFriend(rows)
		if err != nil {
			reply(w, nil, err)
			return
		}
		if admin {
			owners = append(owners, f)
		} else {
			public = append(public, f.publicFriend)
		}
	}
	if err := rows.Err(); err != nil {
		reply(w, nil, err)
		return
	}
	if admin {
		reply(w, map[string]any{"items": owners}, nil)
	} else {
		reply(w, map[string]any{"items": public}, nil)
	}
}
func (s *Service) publicFriendRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/friends", func(w http.ResponseWriter, r *http.Request) { s.listFriends(w, r, false) })
	mux.HandleFunc("POST /api/v1/friend-requests", func(w http.ResponseWriter, r *http.Request) {
		if s.commentOrigin == "" {
			reply(w, nil, fault(503, "friend_requests_unavailable"))
			return
		}
		if r.Header.Get("Origin") != s.commentOrigin {
			reply(w, nil, fault(403, "invalid_origin"))
			return
		}
		select {
		case s.friendSlots <- struct{}{}:
			defer func() { <-s.friendSlots }()
		default:
			w.Header().Set("Retry-After", "1")
			reply(w, nil, fault(503, "friend_requests_busy"))
			return
		}
		controller := http.NewResponseController(w)
		_ = controller.SetReadDeadline(time.Now().Add(5 * time.Second))
		defer controller.SetReadDeadline(time.Time{})
		key, ok := requestKey(w, r)
		if !ok {
			return
		}
		fields, err := readFields(w, r, 16*1024)
		if err != nil {
			reply(w, nil, err)
			return
		}
		d, err := parseFriendDetails(fields, "email")
		if err != nil {
			reply(w, nil, err)
			return
		}
		email := ""
		if raw, ok := fields["email"]; ok {
			if string(raw) == "null" || json.Unmarshal(raw, &email) != nil {
				reply(w, nil, fault(422, "invalid_reply_email"))
				return
			}
		}
		email = strings.TrimSpace(email)
		if email != "" {
			address, err := mail.ParseAddress(email)
			if err != nil || address.Address != email || len(email) > 254 || strings.ContainsAny(email, "\x00\r\n") {
				reply(w, nil, fault(422, "invalid_reply_email"))
				return
			}
		}
		payload, _ := json.Marshal(struct {
			friendDetails
			Email string `json:"email"`
		}{d, email})
		result, err := s.idempotent(r.Context(), "guest-friend-request", key, payload, func(tx *sql.Tx) (int, any, error) {
			if _, err := tx.ExecContext(r.Context(), `SELECT pg_advisory_xact_lock(7311175)`); err != nil {
				return 0, nil, err
			}
			var recent, pending int
			if err := tx.QueryRowContext(r.Context(), `SELECT count(*) FILTER(WHERE created_at>=clock_timestamp()-interval '1 minute'),count(*) FILTER(WHERE status='pending') FROM friend_requests`).Scan(&recent, &pending); err != nil {
				return 0, nil, err
			}
			if recent >= 5 {
				return 0, nil, fault(429, "friend_request_rate_limited")
			}
			if pending >= 1000 {
				return 0, nil, fault(503, "friend_requests_unavailable")
			}
			_, err := tx.ExecContext(r.Context(), `INSERT INTO friend_requests(id,site_name,site_url,introduction,icon_url,email) VALUES($1,$2,$3,$4,$5,$6)`, uuid.NewString(), d.SiteName, d.SiteURL, d.Introduction, d.IconURL, email)
			return 202, map[string]string{"status": "pending"}, err
		})
		var problem *Fault
		if errors.As(err, &problem) && problem.Status == 429 {
			w.Header().Set("Retry-After", "60")
		}
		sendResult(w, result, err)
	})
}
func (s *Service) adminFriendRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/friends", func(w http.ResponseWriter, r *http.Request) { s.listFriends(w, r, true) })
	mux.HandleFunc("POST /api/v1/admin/friends", func(w http.ResponseWriter, r *http.Request) {
		key, ok := requestKey(w, r)
		if !ok {
			return
		}
		fields, err := readFields(w, r)
		if err != nil {
			reply(w, nil, err)
			return
		}
		d, err := parseFriendDetails(fields)
		if err != nil {
			reply(w, nil, err)
			return
		}
		payload, _ := json.Marshal(d)
		result, err := s.idempotent(r.Context(), "owner-create-friend", key, payload, func(tx *sql.Tx) (int, any, error) {
			if err := lockFriends(r.Context(), tx); err != nil {
				return 0, nil, err
			}
			f, err := insertFriend(r.Context(), tx, d)
			return 201, f, err
		})
		sendResult(w, result, err)
	})
	mux.HandleFunc("PATCH /api/v1/admin/friends/{friend_id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "friend_id")
		if !ok {
			return
		}
		fields, err := readFields(w, r)
		if err != nil {
			reply(w, nil, err)
			return
		}
		f, err := s.updateFriend(r.Context(), id, fields)
		reply(w, f, err)
	})
	mux.HandleFunc("PUT /api/v1/admin/friends/order", s.reorderFriends)
	mux.HandleFunc("GET /api/v1/admin/friend-requests", s.listFriendRequests)
	mux.HandleFunc("PATCH /api/v1/admin/friend-requests/{request_id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "request_id")
		if !ok {
			return
		}
		fields, err := readFields(w, r)
		if err != nil {
			reply(w, nil, err)
			return
		}
		f, err := s.moderateFriendRequest(r.Context(), id, fields)
		reply(w, f, err)
	})
}
func (s *Service) updateFriend(ctx context.Context, id string, fields map[string]json.RawMessage) (adminFriend, error) {
	var empty adminFriend
	version, err := expectedVersion(fields)
	if err != nil {
		return empty, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	if err := lockFriends(ctx, tx); err != nil {
		return empty, err
	}
	old, err := scanFriend(tx.QueryRowContext(ctx, `SELECT `+friendColumns+` FROM friends WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return empty, err
	}
	if old.Version != version {
		return empty, fault(409, "version_conflict")
	}
	merged := map[string]json.RawMessage{}
	for key, value := range map[string]string{"site_name": old.SiteName, "site_url": old.SiteURL, "introduction": old.Introduction, "icon_url": old.IconURL} {
		merged[key], _ = json.Marshal(value)
	}
	published := old.Published
	if len(fields) < 2 {
		return empty, fault(422, "invalid_friend")
	}
	for key, raw := range fields {
		switch key {
		case "expected_version":
		case "published":
			if string(raw) == "null" || json.Unmarshal(raw, &published) != nil {
				return empty, fault(422, "invalid_friend")
			}
		default:
			merged[key] = raw
		}
	}
	d, err := parseFriendDetails(merged)
	if err != nil {
		return empty, err
	}
	f, err := scanFriend(tx.QueryRowContext(ctx, `UPDATE friends SET site_name=$2,site_url=$3,introduction=$4,icon_url=$5,published=$6,version=version+1,updated_at=clock_timestamp() WHERE id=$1 RETURNING `+friendColumns, id, d.SiteName, d.SiteURL, d.Introduction, d.IconURL, published))
	if err != nil {
		return empty, err
	}
	return f, tx.Commit()
}
func (s *Service) reorderFriends(w http.ResponseWriter, r *http.Request) {
	fields, err := readFields(w, r)
	if err != nil {
		reply(w, nil, err)
		return
	}
	var items []struct {
		ID      string `json:"id"`
		Version int64  `json:"expected_version"`
	}
	decoder := json.NewDecoder(bytes.NewReader(fields["items"]))
	decoder.DisallowUnknownFields()
	if len(fields) != 1 || decoder.Decode(&items) != nil || items == nil || len(items) > 200 {
		reply(w, nil, fault(422, "invalid_friend_order"))
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer tx.Rollback()
	if err := lockFriends(r.Context(), tx); err != nil {
		reply(w, nil, err)
		return
	}
	rows, err := tx.QueryContext(r.Context(), `SELECT id,version FROM friends FOR UPDATE`)
	if err != nil {
		reply(w, nil, err)
		return
	}
	versions := map[string]int64{}
	for rows.Next() {
		var id string
		var version int64
		if err := rows.Scan(&id, &version); err != nil {
			rows.Close()
			reply(w, nil, err)
			return
		}
		versions[id] = version
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		reply(w, nil, err)
		return
	}
	if len(items) != len(versions) {
		reply(w, nil, fault(409, "version_conflict"))
		return
	}
	for i, item := range items {
		version, ok := versions[item.ID]
		if !ok || version != item.Version {
			reply(w, nil, fault(409, "version_conflict"))
			return
		}
		delete(versions, item.ID)
		if _, err := tx.ExecContext(r.Context(), `UPDATE friends SET position=$2,version=version+1,updated_at=clock_timestamp() WHERE id=$1`, item.ID, i); err != nil {
			reply(w, nil, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		reply(w, nil, err)
		return
	}
	reply(w, map[string]string{"status": "reordered"}, nil)
}
func (s *Service) listFriendRequests(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	for key, items := range values {
		if (key != "page" && key != "limit" && key != "status") || len(items) != 1 {
			reply(w, nil, fault(400, "invalid_friend_parameters"))
			return
		}
	}
	status := values.Get("status")
	if status != "" && status != "pending" && status != "approved" && status != "rejected" {
		reply(w, nil, fault(400, "invalid_friend_parameters"))
		return
	}
	if !values.Has("page") {
		values.Set("page", "1")
	}
	options, err := parseNumberedOptions(values)
	if err != nil {
		reply(w, nil, err)
		return
	}
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer tx.Rollback()
	var total int64
	if err := tx.QueryRowContext(r.Context(), `SELECT count(*) FROM friend_requests WHERE ($1='' OR status=$1)`, status).Scan(&total); err != nil {
		reply(w, nil, err)
		return
	}
	page := options.result(total)
	rows, err := tx.QueryContext(r.Context(), `SELECT `+requestColumns+` FROM friend_requests WHERE ($1='' OR status=$1) ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, status, page.PageSize, page.offset())
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer rows.Close()
	items := []friendRequest{}
	for rows.Next() {
		item, err := scanFriendRequest(rows)
		if err != nil {
			reply(w, nil, err)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		reply(w, nil, err)
		return
	}
	reply(w, map[string]any{"items": items, "page": page.Page, "page_size": page.PageSize, "total": page.Total, "total_pages": page.TotalPages, "next_cursor": nil}, nil)
}
func (s *Service) moderateFriendRequest(ctx context.Context, id string, fields map[string]json.RawMessage) (friendRequest, error) {
	var empty friendRequest
	version, err := expectedVersion(fields)
	if err != nil {
		return empty, err
	}
	var status string
	if len(fields) != 2 || json.Unmarshal(fields["status"], &status) != nil || (status != "approved" && status != "rejected") {
		return empty, fault(422, "invalid_friend_request_status")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	if err := lockFriends(ctx, tx); err != nil {
		return empty, err
	}
	old, err := scanFriendRequest(tx.QueryRowContext(ctx, `SELECT `+requestColumns+` FROM friend_requests WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return empty, err
	}
	if old.Version != version {
		return empty, fault(409, "version_conflict")
	}
	if old.Status != "pending" {
		return empty, fault(409, "friend_request_reviewed")
	}
	var friendID *string
	if status == "approved" {
		friend, err := insertFriend(ctx, tx, old.friendDetails)
		if err != nil {
			return empty, err
		}
		friendID = &friend.ID
	}
	f, err := scanFriendRequest(tx.QueryRowContext(ctx, `UPDATE friend_requests SET status=$2,friend_id=$3,version=version+1,moderated_at=clock_timestamp() WHERE id=$1 RETURNING `+requestColumns, id, status, friendID))
	if err != nil {
		return empty, err
	}
	return f, tx.Commit()
}

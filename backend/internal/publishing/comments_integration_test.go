//go:build integration

package publishing

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func commentArticle(t *testing.T, f *imageFixture, filename string) *Article {
	t.Helper()
	job := f.upload(t, filename, []byte("---\nwritten_at: 2026-11-07\n---\n# Comment article"), nil)
	if job.Status != "succeeded" {
		t.Fatal(job)
	}
	base := "/api/v1/admin/articles/" + *job.ArticleID
	a := imageArticle(t, f.call("GET", base, "", true))
	a = imageArticle(t, f.call("POST", base+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	return imageArticle(t, f.call("PATCH", base, fmt.Sprintf(`{"expected_version":%d,"visibility":"public"}`, a.Version), true))
}
func guestComment(f *imageFixture, article, key, body, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/v1/articles/"+article+"/comments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	req.Header.Set("Origin", origin)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, req)
	return w
}
func decodeComment(t *testing.T, w *httptest.ResponseRecorder) adminComment {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("comment: %d %s", w.Code, w.Body)
	}
	var comment adminComment
	if err := json.Unmarshal(w.Body.Bytes(), &comment); err != nil {
		t.Fatal(err)
	}
	return comment
}
func TestGuestCommentsModerationAndPrivacy(t *testing.T) {
	f := newImageFixture(t)
	a := commentArticle(t, f, "comments.md")
	base := "/api/v1/admin/articles/" + a.ID
	public := "/api/v1/articles/" + a.ID + "/comments"
	original := *a
	payload := testJSON(map[string]string{"display_name": "读者 <b>Name</b>", "body": "First line\n<script>comment_only_needle</script>"})
	key := uuid.NewString()
	for _, origin := range []string{"", "https://other.example"} {
		if guestComment(f, a.ID, key, payload, origin).Code != 403 {
			t.Fatal("guest origin check bypassed")
		}
	}
	if guestComment(f, a.ID, "", payload, "https://blog.example").Code != 400 {
		t.Fatal("guest submission accepted no retry key")
	}
	accepted := guestComment(f, a.ID, key, payload, "https://blog.example")
	if accepted.Code != 202 || !bytes.Equal(accepted.Body.Bytes(), []byte(`{"status": "pending"}`)) {
		t.Fatalf("receipt exposes unexpected information: %d %s", accepted.Code, accepted.Body)
	}
	replay := guestComment(f, a.ID, key, payload, "https://blog.example")
	if replay.Code != 202 || !bytes.Equal(replay.Body.Bytes(), accepted.Body.Bytes()) {
		t.Fatal("guest retry not stable")
	}
	if guestComment(f, a.ID, key, `{"display_name":"Changed","body":"changed"}`, "https://blog.example").Code != 409 {
		t.Fatal("guest key accepted different text")
	}
	var count int
	if err := f.db.QueryRowContext(f.ctx, "SELECT count(*) FROM comments").Scan(&count); err != nil || count != 1 {
		t.Fatal("guest retry duplicated comment")
	}
	if got := f.call("GET", public, "", true); got.Code != 200 || strings.Contains(got.Body.String(), "comment_only_needle") {
		t.Fatal("pending comment publicly visible even with owner cookie")
	}
	if f.call("GET", "/api/v1/admin/comments", "", false).Code != 401 {
		t.Fatal("anonymous moderation list")
	}
	list := f.call("GET", "/api/v1/admin/comments?status=pending&article_id="+a.ID, "", true)
	var page struct {
		Items []adminComment `json:"items"`
	}
	json.Unmarshal(list.Body.Bytes(), &page)
	if list.Code != 200 || len(page.Items) != 1 || page.Items[0].Status != "pending" || page.Items[0].ModeratedAt != nil {
		t.Fatal("pending queue incorrect")
	}
	c := page.Items[0]
	path := "/api/v1/admin/comments/" + c.ID
	if f.call("PATCH", path, `{"expected_version":1,"status":"approved"}`, false).Code != 401 {
		t.Fatal("anonymous moderation mutation")
	}
	saved := f.csrf
	f.csrf = ""
	if f.call("PATCH", path, `{"expected_version":1,"status":"approved"}`, true).Code != 403 {
		t.Fatal("moderation bypassed CSRF")
	}
	f.csrf = saved
	for _, command := range []string{`{"expected_version":1,"status":"pending"}`, `{"expected_version":1,"status":"approved","body":"edited"}`, `{"expected_version":0,"status":"approved"}`} {
		if f.call("PATCH", path, command, true).Code != 422 {
			t.Fatal("invalid moderation accepted")
		}
	}
	c = decodeComment(t, f.call("PATCH", path, `{"expected_version":1,"status":"approved"}`, true))
	if c.Status != "approved" || c.Version != 2 || c.ModeratedAt == nil {
		t.Fatal("approval state/version incorrect")
	}
	approved := f.call("GET", public, "", false)
	if approved.Code != 200 || !strings.Contains(approved.Body.String(), "comment_only_needle") || strings.Contains(approved.Body.String(), `"status"`) || strings.Contains(approved.Body.String(), `"article_id"`) || strings.Contains(approved.Body.String(), `"version"`) || approved.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("approved public representation/privacy incorrect")
	}
	if f.call("PATCH", path, `{"expected_version":1,"status":"rejected"}`, true).Code != 409 {
		t.Fatal("stale moderator overwrote newer state")
	}
	same := decodeComment(t, f.call("PATCH", path, `{"expected_version":2,"status":"approved"}`, true))
	if same.Version != 2 || !same.ModeratedAt.Equal(*c.ModeratedAt) {
		t.Fatal("same moderation status was not a no-op")
	}
	after := imageArticle(t, f.call("GET", base, "", true))
	if after.Version != original.Version || !after.UpdatedAt.Equal(*original.UpdatedAt) {
		t.Fatal("comment actions changed article publication metadata")
	}
	if search := f.call("GET", "/api/v1/articles/search?q=comment_only_needle", "", false); search.Code != 200 || strings.Contains(search.Body.String(), a.ID) {
		t.Fatal("approved comment entered article search")
	}
	a = imageArticle(t, f.call("PATCH", base, fmt.Sprintf(`{"expected_version":%d,"comments_enabled":false}`, a.Version), true))
	if guestComment(f, a.ID, key, payload, "https://blog.example").Code != 409 {
		t.Fatal("receipt replay bypassed disabled-comment check")
	}
	if got := f.call("GET", public, "", false); got.Code != 200 || !strings.Contains(got.Body.String(), c.ID) {
		t.Fatal("disabled submission hid existing approved comment")
	}
	a = imageArticle(t, f.call("PATCH", base, fmt.Sprintf(`{"expected_version":%d,"visibility":"private"}`, a.Version), true))
	if f.call("GET", public, "", true).Code != 404 || guestComment(f, a.ID, key, payload, "https://blog.example").Code != 404 {
		t.Fatal("private comment history or replay leaked")
	}
	// Moderation remains owner-accessible while private; only publication controls visibility.
	c = decodeComment(t, f.call("PATCH", path, `{"expected_version":2,"status":"rejected"}`, true))
	if c.Status != "rejected" || c.Version != 3 {
		t.Fatal("private moderation failed")
	}
	a = imageArticle(t, f.call("PATCH", base, fmt.Sprintf(`{"expected_version":%d,"visibility":"public","comments_enabled":true}`, a.Version), true))
	if strings.Contains(f.call("GET", public, "", false).Body.String(), c.ID) {
		t.Fatal("rejected comment public")
	}
	c = decodeComment(t, f.call("PATCH", path, `{"expected_version":3,"status":"removed"}`, true))
	if strings.Contains(f.call("GET", public, "", false).Body.String(), c.ID) {
		t.Fatal("removed comment public")
	}
	if !strings.Contains(f.call("GET", "/api/v1/admin/comments?status=removed", "", true).Body.String(), c.ID) {
		t.Fatal("logical removal physically deleted owner data")
	}
	c = decodeComment(t, f.call("PATCH", path, `{"expected_version":4,"status":"approved"}`, true))
	// A new article revision retains comments on the stable article identity.
	job := f.upload(t, "comments-next.md", []byte("# Updated article"), a)
	if job.Status != "succeeded" {
		t.Fatal(job)
	}
	a = imageArticle(t, f.call("GET", base, "", true))
	a = imageArticle(t, f.call("POST", base+"/publish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	if !strings.Contains(f.call("GET", public, "", false).Body.String(), c.ID) {
		t.Fatal("revision update lost comments")
	}
	a = imageArticle(t, f.call("POST", base+"/unpublish", fmt.Sprintf(`{"expected_version":%d}`, a.Version), true))
	if f.call("GET", public, "", false).Code != 404 || guestComment(f, a.ID, key, payload, "https://blog.example").Code != 404 {
		t.Fatal("unpublished comments readable or replayed")
	}
	if !strings.Contains(f.call("GET", "/api/v1/admin/comments?article_id="+a.ID, "", true).Body.String(), c.ID) {
		t.Fatal("owner lost unpublished comments")
	}
	if guestComment(f, uuid.NewString(), uuid.NewString(), payload, "https://blog.example").Code != 404 {
		t.Fatal("unknown article accepted comment")
	}
}

func TestCommentValidationRateLimitsAndPagination(t *testing.T) {
	f := newImageFixture(t)
	a := commentArticle(t, f, "limits.md")
	b := commentArticle(t, f, "other.md")
	public := "/api/v1/articles/" + a.ID + "/comments"
	for _, fields := range []map[string]any{
		{"display_name": " ", "body": "valid"}, {"display_name": "Guest", "body": "\n "},
		{"display_name": strings.Repeat("中", 81), "body": "valid"}, {"display_name": "Guest", "body": strings.Repeat("文", 5001)},
		{"display_name": "Guest", "body": "bad\x00text"}, {"display_name": "Guest", "body": "valid", "status": "approved"},
	} {
		if guestComment(f, a.ID, uuid.NewString(), testJSON(fields), "https://blog.example").Code != 422 {
			t.Fatal("invalid guest text/fields accepted")
		}
	}
	if guestComment(f, a.ID, uuid.NewString(), `{"display_name":"Guest","body":"`+strings.Repeat("a", 40000)+`"}`, "https://blog.example").Code != 413 {
		t.Fatal("oversized JSON not bounded")
	}
	if guestComment(f, a.ID, uuid.NewString(), `{"display_name":"Guest","body":"valid"}`+strings.Repeat(" ", 40000), "https://blog.example").Code != 413 {
		t.Fatal("trailing whitespace bypassed request size cap")
	}
	long := testJSON(map[string]string{"display_name": strings.Repeat("中", 80), "body": strings.Repeat("文", 5000)})
	acceptedKey := uuid.NewString()
	if guestComment(f, a.ID, acceptedKey, long, "https://blog.example").Code != 202 {
		t.Fatal("Unicode boundary limit rejected valid comment")
	}
	for i := 0; i < 4; i++ {
		if got := guestComment(f, a.ID, uuid.NewString(), `{"display_name":"Guest","body":"text"}`, "https://blog.example"); got.Code != 202 {
			t.Fatalf("rate budget before limit: %d %s", got.Code, got.Body)
		}
	}
	limited := guestComment(f, a.ID, uuid.NewString(), `{"display_name":"Guest","body":"sixth"}`, "https://blog.example")
	if limited.Code != 429 || limited.Header().Get("Retry-After") != "60" {
		t.Fatal("per-article submission quota missing")
	}
	if guestComment(f, a.ID, acceptedKey, long, "https://blog.example").Code != 202 {
		t.Fatal("retry consumed quota")
	}
	// Approved fixtures also test old creation-time ties and logical states. Old
	// timestamps exclude them from the sliding submission window.
	ids := []string{}
	for i := 0; i < 4; i++ {
		id := uuid.NewString()
		ids = append(ids, id)
		if _, err := f.db.ExecContext(f.ctx, `INSERT INTO comments(id,article_id,display_name,body,status,created_at,moderated_at) VALUES($1,$2,'Fixture','approved','approved','2026-11-01T00:00:00Z','2026-11-01T01:00:00Z')`, id, a.ID); err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(ids)
	type pubPage struct {
		Items []publicComment `json:"items"`
		Next  *string         `json:"next_cursor"`
	}
	var cursor string
	seen := []string{}
	firstCursor := ""
	for {
		query := "?limit=1"
		if cursor != "" {
			query += "&cursor=" + url.QueryEscape(cursor)
		}
		response := f.call("GET", public+query, "", false)
		if response.Code != 200 {
			t.Fatal(response.Body)
		}
		var page pubPage
		json.Unmarshal(response.Body.Bytes(), &page)
		for _, item := range page.Items {
			seen = append(seen, item.ID)
		}
		if page.Next == nil {
			break
		}
		cursor = *page.Next
		if firstCursor == "" {
			firstCursor = cursor
		}
	}
	if len(seen) != len(ids) {
		t.Fatal("comment pagination lost rows")
	}
	for i, id := range seen {
		if id != ids[i] {
			t.Fatal("creation/ID tie ordering incorrect")
		}
	}
	if f.call("GET", "/api/v1/articles/"+b.ID+"/comments?cursor="+url.QueryEscape(firstCursor), "", false).Code != 400 || f.call("GET", "/api/v1/admin/comments?cursor="+url.QueryEscape(firstCursor), "", true).Code != 400 {
		t.Fatal("cursor reused across article/audience")
	}
	admin := f.call("GET", "/api/v1/admin/comments?limit=1&status=approved", "", true)
	var adminPage struct {
		Items []adminComment `json:"items"`
		Next  *string        `json:"next_cursor"`
	}
	json.Unmarshal(admin.Body.Bytes(), &adminPage)
	if admin.Code != 200 || len(adminPage.Items) != 1 || adminPage.Items[0].ID != ids[len(ids)-1] || adminPage.Next == nil {
		t.Fatal("admin newest-first pagination incorrect")
	}
	if f.call("GET", "/api/v1/admin/comments?status=rejected&cursor="+url.QueryEscape(*adminPage.Next), "", true).Code != 400 {
		t.Fatal("cursor reused across moderation status")
	}
	for _, query := range []string{"?status=secret", "?limit=101", "?limit=1&limit=2", "?unknown=1", "?article_id=bad"} {
		if f.call("GET", "/api/v1/admin/comments"+query, "", true).Code != 400 {
			t.Fatalf("invalid moderation filter: %s", query)
		}
	}
	// Fill the remaining global committed-submission budget on another article.
	for i := 0; i < 3; i++ {
		other := commentArticle(t, f, fmt.Sprintf("quota-%d.md", i))
		if _, err := f.db.ExecContext(f.ctx, `INSERT INTO comments(id,article_id,display_name,body) SELECT gen_random_uuid(),$1,'Fixture','recent' FROM generate_series(1,5)`, other.ID); err != nil {
			t.Fatal(err)
		}
	}
	if got := guestComment(f, b.ID, uuid.NewString(), `{"display_name":"Guest","body":"limited"}`, "https://blog.example"); got.Code != 429 {
		t.Fatal("global quota missing")
	}
	secondService, err := NewService(f.db, t.TempDir()+"/uploads", WithCommentOrigin("https://blog.example"))
	if err != nil {
		t.Fatal(err)
	}
	defer secondService.Close()
	_, err = secondService.submitComment(f.ctx, b.ID, uuid.NewString(), map[string]json.RawMessage{"display_name": json.RawMessage(`"Guest"`), "body": json.RawMessage(`"fresh service"`)})
	var rate *Fault
	if !errors.As(err, &rate) || rate.Status != 429 {
		t.Fatal("fresh service ignored persisted submission quota")
	}
	if _, err := f.db.ExecContext(f.ctx, `UPDATE comments SET created_at=clock_timestamp()-interval '2 minutes' WHERE created_at>=clock_timestamp()-interval '1 minute'`); err != nil {
		t.Fatal(err)
	}
	if got := guestComment(f, b.ID, uuid.NewString(), `{"display_name":"Guest","body":"after window"}`, "https://blog.example"); got.Code != 202 {
		t.Fatal("expired sliding window did not restore quota")
	}
	f.s.commentSlots <- struct{}{}
	f.s.commentSlots <- struct{}{}
	busy := guestComment(f, b.ID, uuid.NewString(), `{"display_name":"Guest","body":"busy"}`, "https://blog.example")
	<-f.s.commentSlots
	<-f.s.commentSlots
	if busy.Code != 503 || busy.Header().Get("Retry-After") != "1" {
		t.Fatal("comment admission concurrency unbounded")
	}
}

func TestSubmissionEligibilityLockAndGuardedReplay(t *testing.T) {
	f := newImageFixture(t)
	a := commentArticle(t, f, "lock.md")
	// Reuse the production idempotency path while an article mutation holds the
	// row. Submission must wait and see the new access policy after commit.
	tx, err := f.db.BeginTx(f.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := lockArticle(f.ctx, tx, a.ID, a.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(f.ctx, `UPDATE articles SET visibility='private',version=version+1 WHERE id=$1`, a.ID); err != nil {
		t.Fatal(err)
	}
	timeout, cancel := context.WithTimeout(f.ctx, 100*time.Millisecond)
	_, err = f.s.submitComment(timeout, a.ID, uuid.NewString(), map[string]json.RawMessage{"display_name": json.RawMessage(`"Guest"`), "body": json.RawMessage(`"body"`)})
	cancel()
	if err == nil {
		t.Fatal("submission passed an uncommitted privacy change")
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	got := guestComment(f, a.ID, uuid.NewString(), `{"display_name":"Guest","body":"body"}`, "https://blog.example")
	if got.Code != 404 {
		t.Fatal("submission ignored committed privacy change")
	}
	var count int
	if err := f.db.QueryRowContext(f.ctx, "SELECT count(*) FROM comments").Scan(&count); err != nil || count != 0 {
		t.Fatal("blocked submission wrote a comment")
	}
	// Guard errors must also apply before a completed response is replayed.
	guarded := func(*sql.Tx) error { return fault(404, "not_found") }
	result, err := f.s.idempotent(f.ctx, "guard-test", uuid.NewString(), []byte("payload"), func(*sql.Tx) (int, any, error) { t.Error("guarded action ran"); return 200, nil, nil }, guarded)
	var problem *Fault
	if !errors.As(err, &problem) || problem.Status != 404 || result.Status != 0 {
		t.Fatal("idempotency guard did not fail closed")
	}
}

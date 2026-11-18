//go:build integration

package publishing

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestFriendDirectoryReviewPrivacyOrderAndConflicts(t *testing.T) {
	f := newImageFixture(t)
	details := `{"site_name":"朋友 Notes","site_url":"https://friend.example/","introduction":"A bilingual corner. 你好","icon_url":"https://friend.example/icon.png"}`
	call := func(method, path, body string, owner bool, code int) *httptest.ResponseRecorder {
		t.Helper()
		w := f.call(method, path, body, owner)
		if w.Code != code {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body)
		}
		return w
	}
	decodeFriend := func(w *httptest.ResponseRecorder) adminFriend {
		t.Helper()
		var v adminFriend
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	call("POST", "/api/v1/admin/friends", details, false, 401)
	w := call("POST", "/api/v1/admin/friends", details, true, 201)
	first := decodeFriend(w)
	if first.Published {
		t.Fatal("new entry published without review")
	}
	public := call("GET", "/api/v1/friends", "", false, 200)
	if strings.Contains(public.Body.String(), first.ID) {
		t.Fatal("draft leaked")
	}
	first = decodeFriend(call("PATCH", "/api/v1/admin/friends/"+first.ID, `{"expected_version":1,"published":true}`, true, 200))
	call("PATCH", "/api/v1/admin/friends/"+first.ID, `{"expected_version":1,"site_name":"stale"}`, true, 409)
	public = call("GET", "/api/v1/friends", "", false, 200)
	var directory struct {
		Items []map[string]any `json:"items"`
	}
	json.Unmarshal(public.Body.Bytes(), &directory)
	if len(directory.Items) != 1 {
		t.Fatal("published entry missing")
	}
	for _, key := range []string{"published", "position", "version", "email", "created_at"} {
		if _, ok := directory.Items[0][key]; ok {
			t.Fatalf("owner field %s leaked", key)
		}
	}
	second := decodeFriend(call("POST", "/api/v1/admin/friends", strings.Replace(details, "朋友 Notes", "Second", 1), true, 201))
	second = decodeFriend(call("PATCH", "/api/v1/admin/friends/"+second.ID, `{"expected_version":1,"published":true}`, true, 200))
	order := fmt.Sprintf(`{"items":[{"id":%q,"expected_version":%d},{"id":%q,"expected_version":%d}]}`, second.ID, second.Version, first.ID, first.Version)
	call("PUT", "/api/v1/admin/friends/order", order, true, 200)
	public = call("GET", "/api/v1/friends", "", false, 200)
	json.Unmarshal(public.Body.Bytes(), &directory)
	if directory.Items[0]["id"] != second.ID {
		t.Fatal("wrong directory order")
	}
	call("PUT", "/api/v1/admin/friends/order", order, true, 409)
	// Duplicate/missing IDs must roll back the whole reorder.
	call("PUT", "/api/v1/admin/friends/order", fmt.Sprintf(`{"items":[{"id":%q,"expected_version":3},{"id":%q,"expected_version":3}]}`, second.ID, second.ID), true, 409)
	first = decodeFriend(call("PATCH", "/api/v1/admin/friends/"+first.ID, `{"expected_version":3,"published":false}`, true, 200))
	if first.Version != 4 {
		t.Fatal("failed reorder changed versions")
	}
	key := uuid.NewString()
	payload := strings.TrimSuffix(details, "}") + `,"email":"private-reply@example.com"}`
	submit := func(body, key, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/v1/friend-requests", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", origin)
		req.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, req)
		return w
	}
	if w := submit(payload, key, "https://evil.example"); w.Code != 403 {
		t.Fatal("cross-origin request accepted")
	}
	if w := submit(payload, key, "https://blog.example"); w.Code != 202 || strings.Contains(w.Body.String(), "email") {
		t.Fatalf("submit %d %s", w.Code, w.Body)
	}
	if w := submit(payload, key, "https://blog.example"); w.Code != 202 {
		t.Fatalf("retry %d %s", w.Code, w.Body)
	}
	if w := submit(strings.Replace(payload, "Notes", "Other", 1), key, "https://blog.example"); w.Code != 409 {
		t.Fatal("changed retry accepted")
	}
	call("GET", "/api/v1/admin/friend-requests", "", false, 401)
	queue := call("GET", "/api/v1/admin/friend-requests?status=pending&page=1", "", true, 200)
	var requests struct {
		Items []friendRequest `json:"items"`
	}
	json.Unmarshal(queue.Body.Bytes(), &requests)
	if len(requests.Items) != 1 || requests.Items[0].Email != "private-reply@example.com" {
		t.Fatal("request/reply email not retained privately")
	}
	item := requests.Items[0]
	review := call("PATCH", "/api/v1/admin/friend-requests/"+item.ID, `{"status":"approved","expected_version":1}`, true, 200)
	json.Unmarshal(review.Body.Bytes(), &item)
	if item.FriendID == nil {
		t.Fatal("approval did not create draft")
	}
	call("PATCH", "/api/v1/admin/friend-requests/"+item.ID, `{"status":"approved","expected_version":1}`, true, 409)
	var copies int
	f.db.QueryRow(`SELECT count(*) FROM friends WHERE id=$1`, item.FriendID).Scan(&copies)
	if copies != 1 {
		t.Fatal("approval duplicated draft")
	}
	public = call("GET", "/api/v1/friends", "", false, 200)
	if strings.Contains(public.Body.String(), *item.FriendID) || strings.Contains(public.Body.String(), "private-reply") {
		t.Fatal("approved draft or private email leaked")
	}
	// Actual auth boundary rejects missing CSRF on owner mutations.
	req := httptest.NewRequest("PATCH", "/api/v1/admin/friends/"+first.ID, strings.NewReader(`{"expected_version":4,"published":true}`))
	req.AddCookie(f.cookie)
	req.Header.Set("Origin", "https://blog.example")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	f.handler.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal("missing CSRF accepted")
	}
	for _, query := range []string{"?status=secret", "?status=pending&status=approved", "?page=0", "?limit=101", "?cursor=x"} {
		call("GET", "/api/v1/admin/friend-requests"+query, "", true, 400)
	}
	call("GET", "/api/v1/friends?published=false", "", false, 400)
}
func TestFriendRequestValidationAndAdmission(t *testing.T) {
	f := newImageFixture(t)
	for _, body := range []string{
		`{"site_name":" ","site_url":"https://x.example","introduction":"hi"}`,
		`{"site_name":"x","site_url":"javascript:alert(1)","introduction":"hi"}`,
		`{"site_name":"x","site_url":"https://name:secret@x.example","introduction":"hi"}`,
		`{"site_name":"x","site_url":"https://x.example","introduction":"hi","icon_url":"http://x.example/i.png"}`,
		`{"site_name":"x","site_url":"https://x.example","introduction":"hi","email":"Name <reply@example.com>"}`,
		`{"site_name":"x","site_url":"https://x.example","introduction":"hi","published":true}`,
		`{"site_name":"x","site_url":"https://x.example","introduction":"hi","icon_url":null}`,
	} {
		w := f.call("POST", "/api/v1/friend-requests", body, false)
		if w.Code != 422 {
			t.Fatalf("invalid request: %d %s", w.Code, w.Body)
		}
	}
	payload := `{"site_name":"x","site_url":"https://x.example","introduction":"hi"}`
	for i := 0; i < 5; i++ {
		if w := f.call("POST", "/api/v1/friend-requests", payload, false); w.Code != 202 {
			t.Fatalf("request %d: %d", i, w.Code)
		}
	}
	if w := f.call("POST", "/api/v1/friend-requests", payload, false); w.Code != 429 || w.Header().Get("Retry-After") != "60" {
		t.Fatal("request quota not enforced")
	}
	var count int
	f.db.QueryRow(`SELECT count(*) FROM friend_requests`).Scan(&count)
	if count != 5 {
		t.Fatal("invalid/rate-limited request stored")
	}
}

func TestFriendApprovalIsAtomicAndCapacityKeepsRequestPending(t *testing.T) {
	f := newImageFixture(t)
	body := `{"site_name":"Concurrent","site_url":"https://atomic.example","introduction":"Review me"}`
	if w := f.call("POST", "/api/v1/friend-requests", body, false); w.Code != 202 {
		t.Fatal(w.Body)
	}
	var id string
	if err := f.db.QueryRow(`SELECT id FROM friend_requests`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() {
			statuses <- f.call("PATCH", "/api/v1/admin/friend-requests/"+id, `{"expected_version":1,"status":"approved"}`, true).Code
		}()
	}
	a, b := <-statuses, <-statuses
	if !((a == 200 && b == 409) || (a == 409 && b == 200)) {
		t.Fatalf("concurrent review: %d, %d", a, b)
	}
	var count int
	f.db.QueryRow(`SELECT count(*) FROM friends`).Scan(&count)
	if count != 1 {
		t.Fatalf("duplicate drafts: %d", count)
	}
	if _, err := f.db.Exec(`INSERT INTO friends(id,site_name,site_url,introduction,position) SELECT gen_random_uuid(),'Capacity fixture','https://capacity.example','Capacity test',n FROM generate_series(1,199) n`); err != nil {
		t.Fatal(err)
	}
	if w := f.call("POST", "/api/v1/friend-requests", body, false); w.Code != 202 {
		t.Fatal(w.Body)
	}
	if err := f.db.QueryRow(`SELECT id FROM friend_requests WHERE status='pending'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if w := f.call("PATCH", "/api/v1/admin/friend-requests/"+id, `{"expected_version":1,"status":"approved"}`, true); w.Code != 409 {
		t.Fatalf("capacity: %d %s", w.Code, w.Body)
	}
	var status string
	var version int64
	var friendID *string
	f.db.QueryRow(`SELECT status,version,friend_id FROM friend_requests WHERE id=$1`, id).Scan(&status, &version, &friendID)
	if status != "pending" || version != 1 || friendID != nil {
		t.Fatal("failed approval mutated request")
	}
	if w := f.call("PATCH", "/api/v1/admin/friend-requests/"+id, `{"expected_version":1,"status":"rejected"}`, true); w.Code != 200 {
		t.Fatalf("reject: %d %s", w.Code, w.Body)
	}
	if w := f.call("POST", "/api/v1/admin/friends", body, true); w.Code != 409 {
		t.Fatal("directory capacity exceeded")
	}
}

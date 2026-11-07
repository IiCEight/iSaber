package auth

import (
	"bytes"
	"encoding/base64"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestPasswordHashingAndBounds(t *testing.T) {
	password := "a long mixed 密码 phrase"
	first, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || strings.Contains(first, password) || !verifyPassword(password, first) || verifyPassword("incorrect password", first) {
		t.Fatal("password hashing, salting, or verification failed")
	}
	for _, invalid := range []string{"short", strings.Repeat(" ", 20), strings.Repeat("x", 1025)} {
		if _, err := HashPassword(invalid); err != ErrPassword {
			t.Fatal("invalid password was accepted")
		}
	}
	for _, malformed := range []string{"not-a-hash", strings.Replace(first, "m=19456", "m=4294967295", 1), strings.Replace(first, "p=1", "p=99", 1)} {
		if verifyPassword(password, malformed) {
			t.Fatal("unsupported or corrupt hash was accepted")
		}
	}
}

func TestConfigFailsClosedAndIsolatesDevelopment(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	load := func(values map[string]string) (Config, error) {
		return LoadConfig(func(name string) string { return values[name] })
	}
	for _, values := range []map[string]string{
		{},
		{"SITE_ORIGIN": "http://blog.example", "CSRF_SECRET": key},
		{"SITE_ORIGIN": "https://blog.example", "CSRF_SECRET": "bad-key"},
		{"APP_ENV": "development", "SITE_ORIGIN": "http://public.example"},
		{"APP_ENV": "development", "SITE_ORIGIN": "http://127.0.0.1:3000/path"},
		{"APP_ENV": "unknown"},
	} {
		if _, err := load(values); err == nil {
			t.Fatal("unsafe configuration was accepted")
		}
	}
	production, err := load(map[string]string{"SITE_ORIGIN": "https://blog.example", "CSRF_SECRET": key})
	if err != nil || !production.Secure || production.CookieName != "__Host-isaber_session" {
		t.Fatal("invalid production cookie configuration")
	}
	development, err := load(map[string]string{"APP_ENV": "development"})
	if err != nil || development.Secure || development.CookieName == production.CookieName || len(development.Secret) != 32 {
		t.Fatal("development authentication was not isolated")
	}
	if development.ValidateListenAddress("0.0.0.0:8080") == nil || development.ValidateListenAddress(":8080") == nil || development.ValidateListenAddress("127.0.0.1:8080") != nil {
		t.Fatal("development API binding was not restricted to loopback")
	}
	other, err := load(map[string]string{"APP_ENV": "development"})
	if err != nil || bytes.Equal(other.Secret, development.Secret) {
		t.Fatal("local ephemeral keys were not independent")
	}
}

func TestLoginLimiterIsBoundedUnderConcurrency(t *testing.T) {
	var limiter loginLimiter
	var allowed atomic.Int32
	var workers sync.WaitGroup
	for range 50 {
		workers.Go(func() {
			if limiter.allow() {
				allowed.Add(1)
			}
		})
	}
	workers.Wait()
	if allowed.Load() != 5 {
		t.Fatalf("allowed %d simultaneous attempts, expected 5", allowed.Load())
	}
}

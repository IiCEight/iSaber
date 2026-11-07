package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const passwordMemory uint32 = 19 * 1024

var ErrPassword = errors.New("password must contain 15 to 1024 characters and cannot be blank")

// HashPassword uses Argon2id with an independent 128-bit salt per password.
func HashPassword(password string) (string, error) {
	if !validPassword(password, 15) {
		return "", ErrPassword
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", errors.New("could not generate password salt")
	}
	key := argon2.IDKey([]byte(password), salt, 2, passwordMemory, 1, 32)
	return fmt.Sprintf("$argon2id$v=19$m=19456,t=2,p=1$%s$%s",
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func validPassword(password string, minimum int) bool {
	length := utf8.RuneCountInString(password)
	return utf8.ValidString(password) && length >= minimum && length <= 1024 && strings.TrimSpace(password) != ""
}

func verifyPassword(password, encoded string) bool {
	if !validPassword(password, 1) {
		return false
	}
	parts := strings.Split(encoded, "$")
	// A fixed supported format bounds CPU/memory even if a stored value is corrupt.
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=19456,t=2,p=1" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) != 32 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, 2, passwordMemory, 1, 32)
	return subtle.ConstantTimeCompare(got, want) == 1
}

package database

import (
	"strings"
	"testing"
)

func TestConfigurationErrorsDoNotExposeSecrets(t *testing.T) {
	for _, connection := range []string{"", "postgres://user:secret-password@localhost:invalid/db"} {
		db, err := Open(connection)
		if db != nil || err == nil {
			t.Fatal("invalid configuration was accepted")
		}
		if strings.Contains(err.Error(), "secret-password") || strings.Contains(err.Error(), "postgres://") {
			t.Fatal("configuration error exposed credentials")
		}
	}
}

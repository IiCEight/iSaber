package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"net/url"
)

type Config struct {
	Origin     string
	CookieName string
	Secure     bool
	Secret     []byte
}

func (c Config) ValidateListenAddress(address string) error {
	if c.Secure {
		return nil
	}
	host, _, err := net.SplitHostPort(address)
	ip := net.ParseIP(host)
	if err != nil || (host != "localhost" && (ip == nil || !ip.IsLoopback())) {
		return errors.New("development authentication must bind the API to a loopback address")
	}
	return nil
}

func LoadConfig(getenv func(string) string) (Config, error) {
	mode := getenv("APP_ENV")
	if mode == "" {
		mode = "production"
	}
	if mode != "production" && mode != "development" {
		return Config{}, errors.New("APP_ENV must be production or development")
	}
	origin := getenv("SITE_ORIGIN")
	if origin == "" && mode == "development" {
		origin = "http://127.0.0.1:3000"
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return Config{}, errors.New("SITE_ORIGIN must be an HTTP(S) origin without credentials or a path")
	}
	config := Config{Origin: u.Scheme + "://" + u.Host, CookieName: "__Host-isaber_session", Secure: true}
	if mode == "development" {
		if (u.Scheme != "http" && u.Scheme != "https") || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1") {
			return Config{}, errors.New("development authentication requires a loopback SITE_ORIGIN")
		}
		config.CookieName = "isaber_dev_session"
		config.Secure = false
	} else if u.Scheme != "https" {
		return Config{}, errors.New("production SITE_ORIGIN requires HTTPS")
	}
	encoded := getenv("CSRF_SECRET")
	if encoded == "" && mode == "development" {
		config.Secret = make([]byte, 32)
		if _, err := rand.Read(config.Secret); err != nil {
			return Config{}, errors.New("could not create local CSRF key")
		}
		return config, nil
	}
	config.Secret, err = base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(config.Secret) != 32 {
		return Config{}, errors.New("CSRF_SECRET must be a base64-encoded 32-byte key")
	}
	return config, nil
}

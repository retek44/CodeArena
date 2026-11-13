package store

import (
	"errors"
	"net"
	"net/url"
	"os"
)

// DatabaseURLFromEnv returns DATABASE_URL, or builds it from parts. In AWS the
// password comes from the RDS-managed secret (ECS injects it as DB_PASSWORD), so it
// never appears in a task definition or in Terraform state.
//
//	DB_HOST, DB_NAME, DB_USER, DB_PASSWORD   required when DATABASE_URL is empty
//	DB_PORT                                  default 5432
//	DB_SSLMODE                               default require
func DatabaseURLFromEnv() (string, error) {
	if u := os.Getenv("DATABASE_URL"); u != "" {
		return u, nil
	}
	host, name, user, password := os.Getenv("DB_HOST"), os.Getenv("DB_NAME"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD")
	if host == "" || name == "" || user == "" || password == "" {
		return "", errors.New("set DATABASE_URL, or DB_HOST, DB_NAME, DB_USER and DB_PASSWORD")
	}
	port := envOr("DB_PORT", "5432")
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     net.JoinHostPort(host, port),
		Path:     "/" + name,
		RawQuery: url.Values{"sslmode": {envOr("DB_SSLMODE", "require")}}.Encode(),
	}
	return u.String(), nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

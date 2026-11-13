package store_test

import (
	"net/url"
	"testing"

	"github.com/retek44/CodeArena/backend/internal/store"
)

func TestDatabaseURLFromEnvPrefersDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://a:b@host/db")
	t.Setenv("DB_HOST", "ignored")

	got, err := store.DatabaseURLFromEnv()
	if err != nil || got != "postgres://a:b@host/db" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestDatabaseURLFromEnvBuildsFromParts(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", "db.internal")
	t.Setenv("DB_NAME", "codearena")
	t.Setenv("DB_USER", "app")
	// RDS-generated passwords contain characters that must be escaped in a URL.
	t.Setenv("DB_PASSWORD", "p@ss:w/rd?#%")
	t.Setenv("DB_PORT", "")
	t.Setenv("DB_SSLMODE", "")

	got, err := store.DatabaseURLFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("not a valid URL %q: %v", got, err)
	}
	password, _ := u.User.Password()
	if u.Host != "db.internal:5432" || u.Path != "/codearena" || u.User.Username() != "app" || password != "p@ss:w/rd?#%" {
		t.Errorf("parsed back as host=%s path=%s user=%s password=%q", u.Host, u.Path, u.User.Username(), password)
	}
	if u.Query().Get("sslmode") != "require" {
		t.Errorf("sslmode = %q, want require by default", u.Query().Get("sslmode"))
	}
}

func TestDatabaseURLFromEnvRequiresParts(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", "db.internal")
	t.Setenv("DB_NAME", "")
	t.Setenv("DB_USER", "app")
	t.Setenv("DB_PASSWORD", "x")

	if _, err := store.DatabaseURLFromEnv(); err == nil {
		t.Fatal("missing DB_NAME accepted")
	}
}

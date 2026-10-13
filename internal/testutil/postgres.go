// Package testutil provides isolated database fixtures for integration tests.
// Runtime packages must not depend on it.
package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	schemaSQL "github.com/gofxq/gaoming/deployments/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func Postgres(t testing.TB) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		if os.Getenv("REQUIRE_POSTGRES_TESTS") == "1" {
			t.Fatal("TEST_POSTGRES_DSN is required")
		}
		t.Skip("set TEST_POSTGRES_DSN for PostgreSQL integration tests")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("gaoming_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dsn, "://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		dsn = parsed.String()
	} else {
		dsn += " search_path=" + schema
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close(); _, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); _ = admin.Close() })
	if _, err := db.ExecContext(context.Background(), schemaSQL.InitSQL); err != nil {
		t.Fatal(err)
	}
	return db
}

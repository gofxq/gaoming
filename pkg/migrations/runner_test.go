package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Every integration test owns a fresh schema; it never truncates a user's tables.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		if os.Getenv("REQUIRE_POSTGRES_TESTS") == "1" {
			t.Fatal("TEST_POSTGRES_DSN is required")
		}
		t.Skip("set TEST_POSTGRES_DSN to run PostgreSQL integration tests")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	// pgx accepts search_path as a connection parameter for both URL and keyword DSNs.
	if strings.Contains(dsn, "://") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "search_path=" + schema
	} else {
		dsn += " search_path=" + schema
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		_ = admin.Close()
	})
	return db
}

func schemaColumns(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT table_name, column_name, data_type, is_nullable
		FROM information_schema.columns WHERE table_schema = current_schema()
		AND table_name <> 'schema_migrations' ORDER BY table_name, column_name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var table, column, typ, nullable string
		if err := rows.Scan(&table, &column, &typ, &nullable); err != nil {
			t.Fatal(err)
		}
		result = append(result, table+"."+column+":"+typ+":"+nullable)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestMigrationEmptyAndExistingDatabaseMatch(t *testing.T) {
	ctx := context.Background()
	empty, existing := testDB(t), testDB(t)
	initSQL, err := os.ReadFile("../../deployments/sql/init.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := existing.Exec(string(initSQL)); err != nil {
		t.Fatal(err)
	}
	// Emulate a deployed pre-adaptive-metrics database, including real retained data.
	if _, err := existing.Exec(`INSERT INTO tenants(tenant_code,name) VALUES ('retained','Existing tenant');
		ALTER TABLE host_status_current DROP COLUMN mem_available_bytes, DROP COLUMN swap_used_pct,
		DROP COLUMN disk_free_bytes, DROP COLUMN disk_inodes_used_pct, DROP COLUMN disk_read_iops,
		DROP COLUMN disk_write_iops, DROP COLUMN net_rx_packets_ps, DROP COLUMN net_tx_packets_ps`); err != nil {
		t.Fatal(err)
	}
	for _, db := range []*sql.DB{empty, existing} {
		if err := Run(ctx, db); err != nil {
			t.Fatal(err)
		}
		if err := Run(ctx, db); err != nil {
			t.Fatal("repeat migration:", err)
		}
		if err := Check(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(schemaColumns(t, empty), schemaColumns(t, existing)) {
		t.Fatal("empty and upgraded database schemas differ")
	}
	var count int
	if err := existing.QueryRow(`SELECT count(*) FROM tenants WHERE tenant_code='retained'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("data changed: count=%d error=%v", count, err)
	}
}

func TestMigrationFailureRollsBackAndCanRetry(t *testing.T) {
	db := testDB(t)
	source := fstest.MapFS{
		"001-start.sql": {Data: []byte(`CREATE TABLE marker(id integer)`)},
		"002-fail.sql":  {Data: []byte(`INSERT INTO nonexistent_table VALUES (1)`)},
	}
	if err := run(context.Background(), db, source); err == nil {
		t.Fatal("expected SQL failure")
	}
	var name sql.NullString
	if err := db.QueryRow(`SELECT to_regclass('marker')::text`).Scan(&name); err != nil || name.Valid {
		t.Fatalf("failed migration left table: %v %v", name, err)
	}
	source["002-fail.sql"] = &fstest.MapFile{Data: []byte(`INSERT INTO marker VALUES (42)`)}
	if err := run(context.Background(), db, source); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), db, source); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM marker WHERE id=42`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("retry duplicated data: %d %v", count, err)
	}
}

func TestMigrationChecksumAndFutureVersionRejected(t *testing.T) {
	db := testDB(t)
	if err := Check(context.Background(), db); err == nil {
		t.Fatal("unmigrated database accepted")
	}
	if err := Run(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE schema_migrations SET checksum='tampered' WHERE version=1`); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), db); err == nil {
		t.Fatal("changed applied migration accepted")
	}
	if err := Check(context.Background(), db); err == nil {
		t.Fatal("checksum mismatch accepted at startup")
	}
	db2 := testDB(t)
	if err := Run(context.Background(), db2); err != nil {
		t.Fatal(err)
	}
	if _, err := db2.Exec(`INSERT INTO schema_migrations(version,name,checksum) VALUES (999,'future','x')`); err != nil {
		t.Fatal(err)
	}
	if err := Check(context.Background(), db2); err == nil {
		t.Fatal("future schema accepted by old binary")
	}
}

func TestConcurrentMigrationsAndLockCancellation(t *testing.T) {
	db := testDB(t)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- Run(context.Background(), db) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock($1)`, lockID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := Run(ctx, db); err == nil {
		t.Fatal("lock did not respect cancellation")
	}
}

func TestMigrationFilesRequireContiguousVersions(t *testing.T) {
	for _, files := range []fstest.MapFS{
		{"001-one.sql": {Data: []byte("SELECT 1")}, "003-gap.sql": {Data: []byte("SELECT 1")}},
		{"001-one.sql": {Data: []byte("SELECT 1")}, "001-duplicate.sql": {Data: []byte("SELECT 1")}},
		{"bad.sql": {Data: []byte("SELECT 1")}},
		{},
	} {
		if _, err := load(files); err == nil {
			t.Fatal("invalid migration set accepted")
		}
	}
}

// Package testutil provides a self-migrating test database helper shared by
// internal/model and internal/service tests.
package testutil

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/Operations-ESB/dashboard-sales/api/migrations"
)

// SetupTestDB creates a throwaway, uniquely-named database on the same
// Postgres server as TEST_DATABASE_URL, migrates it up with goose, hands
// back a *gorm.DB, and drops it when the test finishes.
//
// Each call gets its OWN database rather than reusing TEST_DATABASE_URL's
// database directly: `go test ./...` runs different packages' test binaries
// concurrently, and internal/model and internal/service both call this --
// sharing one physical database across concurrent goose.Up/DownTo(0) runs
// causes them to step on each other's schema mid-test.
//
// Skips the test if TEST_DATABASE_URL isn't set or unreachable.
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping test that needs a real Postgres")
	}

	dsn, err := url.Parse(base)
	if err != nil {
		t.Skip("cannot parse TEST_DATABASE_URL: " + err.Error())
	}

	adminDSN := *dsn
	adminDSN.Path = "/postgres"
	adminDB, err := sql.Open("pgx", adminDSN.String())
	if err != nil {
		t.Skip("cannot open admin connection: " + err.Error())
	}
	if err := adminDB.Ping(); err != nil {
		adminDB.Close()
		t.Skip("cannot connect to Postgres server from TEST_DATABASE_URL: " + err.Error())
	}

	dbName := "apptest_" + randomHex(t, 8)
	if _, err := adminDB.Exec("CREATE DATABASE " + dbName); err != nil {
		adminDB.Close()
		t.Fatalf("create test database %s: %v", dbName, err)
	}
	// Registered before the test connection's own cleanup below, so it runs
	// AFTER that connection is closed (t.Cleanup runs LIFO) -- Postgres
	// refuses to drop a database that still has open connections.
	t.Cleanup(func() {
		defer adminDB.Close()
		if _, err := adminDB.Exec("DROP DATABASE IF EXISTS " + dbName); err != nil {
			t.Errorf("drop test database %s: %v", dbName, err)
		}
	})

	testDSN := *dsn
	testDSN.Path = "/" + dbName
	sqlDB, err := sql.Open("pgx", testDSN.String())
	if err != nil {
		t.Fatalf("open %s: %v", dbName, err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose.SetDialect: %v", err)
	}
	if err := goose.Up(sqlDB, "."); err != nil {
		t.Fatalf("goose.Up: %v", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return db
}

func randomHex(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n/2)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	return fmt.Sprintf("%x", b)
}

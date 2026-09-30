package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/pressly/goose/v3"

	"github.com/Operations-ESB/dashboard-sales/api/internal/config"
	"github.com/Operations-ESB/dashboard-sales/api/internal/db"
	"github.com/Operations-ESB/dashboard-sales/api/migrations"
)

// runMigrate implements `server migrate <up|down|status|has-pending>`.
//
// has-pending exits 0 when the embedded migrations include a version newer
// than what's recorded in the database, 1 otherwise — deploy.sh uses the
// exit code to decide whether a pre-migration pg_dump backup is needed.
func runMigrate(cfg config.Config, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: server migrate <up|down|status|has-pending>")
	}

	gormDB, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	switch args[0] {
	case "up":
		return goose.Up(sqlDB, ".")
	case "down":
		return goose.Down(sqlDB, ".")
	case "status":
		return goose.Status(sqlDB, ".")
	case "has-pending":
		pending, err := hasPending(sqlDB)
		if err != nil {
			return err
		}
		if pending {
			os.Exit(0)
		}
		os.Exit(1)
		return nil
	default:
		return fmt.Errorf("unknown migrate subcommand %q", args[0])
	}
}

// hasPending compares the highest embedded migration version against what's
// recorded in the database's goose version table.
func hasPending(sqlDB *sql.DB) (bool, error) {
	all, err := goose.CollectMigrations(".", 0, goose.MaxVersion)
	if err != nil {
		if err == goose.ErrNoMigrationFiles || len(all) == 0 {
			return false, nil
		}
		return false, err
	}
	if len(all) == 0 {
		return false, nil
	}

	last, err := all.Last()
	if err != nil {
		return false, nil
	}

	current, err := goose.GetDBVersion(sqlDB)
	if err != nil {
		return false, err
	}

	return last.Version > current, nil
}

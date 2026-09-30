package db

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open connects to Postgres via GORM. AutoMigrate is never used here —
// schema changes always go through goose migrations in api/migrations.
func Open(databaseURL string) (*gorm.DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is empty")
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	return db, nil
}

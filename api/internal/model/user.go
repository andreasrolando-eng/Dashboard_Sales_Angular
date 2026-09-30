package model

import "time"

// User is an app-managed account, upserted by email on first operations-sso
// login (see docs/2026-09-28-migrasi-tech-stack.md). SSO isn't wired up yet,
// so an admin can pre-provision a row by email before Sub is ever filled in.
type User struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Name      string    `json:"name"`
	Sub       *string   `gorm:"uniqueIndex" json:"-"`
	IsAdmin   bool      `gorm:"not null;default:false" json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

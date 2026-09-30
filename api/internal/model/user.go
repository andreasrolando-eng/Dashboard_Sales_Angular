package model

import (
	"time"

	"gorm.io/gorm"
)

// User is an app-managed account. An admin provisions it by email (Kelola
// User); it can sign in with email + password once PasswordHash is set (or,
// later, through operations-sso, which fills Sub on first login).
type User struct {
	ID    int64   `gorm:"primaryKey" json:"id"`
	Email string  `gorm:"uniqueIndex;not null" json:"email"`
	Name  string  `json:"name"`
	Sub   *string `gorm:"uniqueIndex" json:"-"`
	// PasswordHash is a bcrypt hash; never serialised. nil = no password set.
	PasswordHash *string    `gorm:"column:password_hash" json:"-"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	IsAdmin      bool       `gorm:"not null;default:false" json:"is_admin"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	// HasPassword is derived from PasswordHash (not a column); lets the API
	// tell the UI whether an account can sign in without exposing the hash.
	HasPassword bool `gorm:"-" json:"has_password"`
}

// Session is one server-side login. TokenHash is the SHA-256 (hex) of the
// random token held in the browser cookie.
type Session struct {
	ID        int64  `gorm:"primaryKey"`
	TokenHash string `gorm:"not null;uniqueIndex"`
	UserID    int64  `gorm:"not null;index"`
	CreatedAt time.Time
	ExpiresAt time.Time `gorm:"not null"`
	UserAgent *string
	IP        *string
}

// AfterFind fills HasPassword after every load so the flag is always
// consistent with PasswordHash.
func (u *User) AfterFind(_ *gorm.DB) error {
	u.HasPassword = u.PasswordHash != nil && *u.PasswordHash != ""
	return nil
}

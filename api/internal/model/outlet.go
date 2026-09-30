package model

import "time"

// Outlet mirrors a branch from the ESB OMS API.
type Outlet struct {
	BranchCode    string `gorm:"primaryKey"`
	BranchName    string `gorm:"not null"`
	ExtBranchCode *string
	FirstSeenAt   time.Time
	LastSeenAt    time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

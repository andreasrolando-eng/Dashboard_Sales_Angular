package model

import "time"

// SyncLog records one run of the ETL job (M3) that pulls sales/menu/member
// data from the ESB OMS API into the raw_* tables. One row per synced day.
// JobName is "sync-esb" for the nightly sync and "sync-esb-manual" for
// user-triggered runs.
type SyncLog struct {
	ID           int64      `gorm:"primaryKey" json:"id"`
	JobName      string     `gorm:"not null" json:"job_name"`
	TargetDate   *time.Time `json:"target_date"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	Status       string     `gorm:"not null;default:running" json:"status"`
	RowsSynced   *int       `json:"rows_synced"`
	ErrorMessage *string    `json:"error_message"`
	CreatedAt    time.Time  `json:"-"`
	UpdatedAt    time.Time  `json:"-"`
}

package service

import (
	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
)

// ListSyncLogs returns the most recent sync runs (nightly and manual), newest
// first. limit is clamped to 1..200.
func ListSyncLogs(db *gorm.DB, limit int) ([]model.SyncLog, error) {
	if limit < 1 {
		limit = 30
	}
	if limit > 200 {
		limit = 200
	}
	logs := []model.SyncLog{}
	err := db.Order("started_at desc, id desc").Limit(limit).Find(&logs).Error
	return logs, err
}

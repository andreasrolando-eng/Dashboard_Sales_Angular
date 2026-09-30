package service

import (
	"time"

	"gorm.io/gorm"
)

// OutletOption is a flat, unlimited outlet list for dropdowns -- unlike the
// MCP list_outlets tool, which applies a search filter and a limit.
type OutletOption struct {
	BranchCode string `json:"branch_code"`
	BranchName string `json:"branch_name"`
}

// CategoryOption is a flat category list. The display name is whichever
// menu_category_name appeared on that category_id's most recent Finished
// sale -- ported from the old v_categories view (distinct on category_id,
// ordered by sales_date desc), not the most common name.
type CategoryOption struct {
	CategoryID   string `json:"category_id"`
	CategoryName string `json:"category_name"`
}

// CategoryDetailOption mirrors CategoryOption one level down (menu category
// detail), same "latest sales_date wins" rule, ported from v_category_details.
type CategoryDetailOption struct {
	CategoryDetailID   string `json:"category_detail_id"`
	CategoryDetailName string `json:"category_detail_name"`
	CategoryID         string `json:"category_id"`
}

// LastSync is the most recent successful ETL run (M3). Returns nil until
// sync_logs has a 'success' row.
type LastSync struct {
	JobName    string     `json:"job_name"`
	FinishedAt *time.Time `json:"finished_at"`
	Status     string     `json:"status"`
	RowsSynced *int       `json:"rows_synced"`
}

func ListOutlets(db *gorm.DB) ([]OutletOption, error) {
	var outlets []OutletOption
	err := db.Table("outlets").
		Select("branch_code, branch_name").
		Order("branch_name").
		Find(&outlets).Error
	return outlets, err
}

// ListCategories ports v_categories: distinct on category_id, name taken
// from the row with the most recent sales_date, sourced from menu line
// items on Finished sales only. Ties on the same sales_date break in an
// undefined order (same as the original view).
func ListCategories(db *gorm.DB) ([]CategoryOption, error) {
	var categories []CategoryOption
	err := db.Raw(`
		select distinct on (category_id) category_id, category_name
		from (
			select
				m.menu_category_id as category_id,
				m.menu_category_name as category_name,
				m.sales_date
			from raw_sales_menu_items m
			join raw_sales s on s.sales_num = m.sales_num
			where s.status_name = 'Finished' and m.menu_category_id is not null
		) t
		order by category_id, sales_date desc
	`).Scan(&categories).Error
	return categories, err
}

// ListCategoryDetails ports v_category_details -- same "latest sales_date
// wins" rule as ListCategories, one level down the category hierarchy.
func ListCategoryDetails(db *gorm.DB) ([]CategoryDetailOption, error) {
	var details []CategoryDetailOption
	err := db.Raw(`
		select distinct on (category_detail_id) category_detail_id, category_detail_name, category_id
		from (
			select
				m.menu_category_detail_id as category_detail_id,
				m.menu_category_detail_name as category_detail_name,
				m.menu_category_id as category_id,
				m.sales_date
			from raw_sales_menu_items m
			join raw_sales s on s.sales_num = m.sales_num
			where s.status_name = 'Finished' and m.menu_category_detail_id is not null
		) t
		order by category_detail_id, sales_date desc
	`).Scan(&details).Error
	return details, err
}

// GetLastSync returns the most recent successful NIGHTLY sync_logs row, or nil
// if none exists yet. Manual syncs (job_name 'sync-esb-manual') are ignored on
// purpose: backfilling an old date by hand must not make the dashboard claim
// the data is fresh as of now.
func GetLastSync(db *gorm.DB) (*LastSync, error) {
	var sync LastSync
	err := db.Raw(`
		select job_name, finished_at, status, rows_synced
		from sync_logs
		where status = 'success' and job_name = 'sync-esb'
		order by finished_at desc
		limit 1
	`).Scan(&sync).Error
	if err != nil {
		return nil, err
	}
	if sync.JobName == "" {
		return nil, nil
	}
	return &sync, nil
}

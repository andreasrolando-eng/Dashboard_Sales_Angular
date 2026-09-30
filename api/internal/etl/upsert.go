package etl

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const upsertBatchSize = 500

// Mode decides what happens when a fetched row already exists (same primary
// key). Either way a row can never be duplicated -- every table is keyed on
// its natural ESB identifier.
type Mode int

const (
	// ModeUpsert overwrites existing rows with the fetched values
	// (ON CONFLICT DO UPDATE). Used by the nightly sync, so a bill whose
	// status changed since the last pull gets refreshed.
	ModeUpsert Mode = iota
	// ModeFillMissing only inserts rows that are not stored yet
	// (ON CONFLICT DO NOTHING) and leaves existing rows untouched. Used by
	// the manual sync ("lengkapi data yang belum ada").
	ModeFillMissing
)

// UpsertResult reports how many rows were written per table -- summed into
// sync_logs.rows_synced (outlets included in the total, matching the
// original). In ModeFillMissing it counts only rows that were actually
// inserted; rows that already existed are not counted.
type UpsertResult struct {
	Outlets  int `json:"outlets"`
	Sales    int `json:"sales"`
	Payments int `json:"payments"`
	Items    int `json:"items"`
}

// Total is the sum across all tables (what sync_logs.rows_synced stores).
func (u UpsertResult) Total() int { return u.Outlets + u.Sales + u.Payments + u.Items }

// Counts returns how many rows of each kind the result holds -- i.e. how many
// a sync attempted to write, before existing rows are taken into account.
func (r TransformResult) Counts() UpsertResult {
	return UpsertResult{Outlets: len(r.Outlets), Sales: len(r.Sales), Payments: len(r.Payments), Items: len(r.Items)}
}

// Upsert is the nightly-sync write path (ModeUpsert).
func Upsert(db *gorm.DB, r TransformResult) (UpsertResult, error) {
	return UpsertMode(db, r, ModeUpsert)
}

// UpsertMode writes one day's TransformResult in the same order as the
// original Edge Function (outlets, then sales, then payments, then menu
// items -- the FK dependency order), in batches of 500. Each write is
// idempotent, so re-running the same day after a partial failure is always
// safe -- nothing is deleted first.
//
// ModeUpsert: outlets only update branch_name/ext_branch_code on conflict
// (first_seen_at/last_seen_at are left alone -- DB default fires on insert
// only, matching the original, which never sends those columns); sales,
// payments and menu items update every column, matching the original's
// "DO UPDATE SET <all supplied columns>".
//
// ModeFillMissing: every table is ON CONFLICT DO NOTHING, and the result
// counts only newly inserted rows. Because payments and menu items are
// checked against their own keys, a bill that is already stored but is
// missing some of its payment/item rows still gets those filled in.
func UpsertMode(db *gorm.DB, r TransformResult, mode Mode) (UpsertResult, error) {
	var result UpsertResult

	conflict := func(cols []clause.Column, updateCols []string) clause.OnConflict {
		if mode == ModeFillMissing {
			return clause.OnConflict{Columns: cols, DoNothing: true}
		}
		if updateCols == nil {
			return clause.OnConflict{Columns: cols, UpdateAll: true}
		}
		return clause.OnConflict{Columns: cols, DoUpdates: clause.AssignmentColumns(updateCols)}
	}
	// written is what to report for a batch: everything in upsert mode, only
	// the rows Postgres actually inserted in fill-missing mode.
	written := func(attempted int, tx *gorm.DB) int {
		if mode == ModeFillMissing {
			return int(tx.RowsAffected)
		}
		return attempted
	}

	if len(r.Outlets) > 0 {
		// Omit first_seen_at/last_seen_at entirely -- Outlet's Go zero value
		// for these (time.Time{}, i.e. year 0001) would otherwise be sent
		// explicitly in the INSERT, which suppresses the column's
		// `default now()` just as surely as if we'd written the zero value
		// on purpose. Omit() drops them from the statement so the DB
		// default actually fires, matching "not sent" in the original.
		tx := db.Omit("FirstSeenAt", "LastSeenAt").
			Clauses(conflict([]clause.Column{{Name: "branch_code"}}, []string{"branch_name", "ext_branch_code"})).
			CreateInBatches(&r.Outlets, upsertBatchSize)
		if tx.Error != nil {
			return result, fmt.Errorf("upsert outlets: %w", tx.Error)
		}
		result.Outlets = written(len(r.Outlets), tx)
	}

	if len(r.Sales) > 0 {
		tx := db.Clauses(conflict([]clause.Column{{Name: "sales_num"}}, nil)).CreateInBatches(&r.Sales, upsertBatchSize)
		if tx.Error != nil {
			return result, fmt.Errorf("upsert raw_sales: %w", tx.Error)
		}
		result.Sales = written(len(r.Sales), tx)
	}

	if len(r.Payments) > 0 {
		tx := db.Clauses(conflict([]clause.Column{{Name: "sales_num"}, {Name: "sales_payment_backend_id"}}, nil)).
			CreateInBatches(&r.Payments, upsertBatchSize)
		if tx.Error != nil {
			return result, fmt.Errorf("upsert raw_sales_payments: %w", tx.Error)
		}
		result.Payments = written(len(r.Payments), tx)
	}

	if len(r.Items) > 0 {
		tx := db.Clauses(conflict([]clause.Column{{Name: "sales_num"}, {Name: "line_seq"}}, nil)).
			CreateInBatches(&r.Items, upsertBatchSize)
		if tx.Error != nil {
			return result, fmt.Errorf("upsert raw_sales_menu_items: %w", tx.Error)
		}
		result.Items = written(len(r.Items), tx)
	}

	return result, nil
}

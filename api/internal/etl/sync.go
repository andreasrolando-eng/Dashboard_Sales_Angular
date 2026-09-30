package etl

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
)

// YesterdayWIB mirrors the original's default date fallback: yesterday in
// Asia/Jakarta (a fixed UTC+7 offset, no DST, so no timezone database entry
// is needed). Getting this right matters -- the original had a real
// production bug where computing "yesterday" in UTC instead of WIB silently
// synced the wrong day for hours every night (see RUNBOOK.md).
func YesterdayWIB() string {
	wib := time.FixedZone("WIB", 7*60*60)
	return time.Now().In(wib).AddDate(0, 0, -1).Format("2006-01-02")
}

type DaySyncResult struct {
	Date           string `json:"date"`
	OK             bool   `json:"ok"`
	RecordsFetched int    `json:"records_fetched,omitempty"`
	// Outlets/Sales/Payments/MenuItems = rows written. In ModeFillMissing
	// that is only the rows that were newly inserted.
	Outlets   int `json:"outlets,omitempty"`
	Sales     int `json:"sales,omitempty"`
	Payments  int `json:"payments,omitempty"`
	MenuItems int `json:"menu_items,omitempty"`
	// Found is everything ESB returned for the day (after de-duplication),
	// and Existing (ModeFillMissing only) the part of it that was already
	// stored and therefore skipped: Found - written, per table.
	Found    UpsertResult  `json:"found"`
	Existing *UpsertResult `json:"existing,omitempty"`
	// SalesNew / SalesRefreshed split the bills ESB returned into ones that were
	// not stored before and ones that were (overwritten by a refresh, skipped by
	// fill-missing). StatusChanges and NotInESB are only filled by a refresh.
	SalesNew       int            `json:"sales_new"`
	SalesRefreshed int            `json:"sales_refreshed"`
	StatusChanges  []StatusChange `json:"status_changes,omitempty"`
	NotInESB       []BillRef      `json:"not_in_esb,omitempty"`
	Error          string         `json:"error,omitempty"`
}

// SyncOptions tunes SyncDayWith. The zero value is the nightly sync.
type SyncOptions struct {
	Mode    Mode
	JobName string // sync_logs.job_name; defaults to "sync-esb"
	// ReportChanges diffs the fetched bills against what is stored before
	// writing, to report new / refreshed bills, status changes and bills ESB no
	// longer returns. Costs one extra query per day.
	ReportChanges bool
}

// ManualJobName is the sync_logs.job_name of manual syncs. It is deliberately
// different from the nightly job so a manual run never counts as "this day
// was synced" for the scheduler's catch-up, nor as the "last sync" shown in
// the UI.
const ManualJobName = "sync-esb-manual"

// ManualRefreshJobName is the job_name of manual runs that overwrite existing
// data ("perbarui"). Like ManualJobName it never counts as a nightly sync.
const ManualRefreshJobName = "sync-esb-manual-refresh"

// SyncDay fetches, transforms and upserts one calendar date, recording a
// sync_logs row throughout -- ports syncOneDay from the original. There is
// no membership sync here: the old one was an unimplemented stub gated
// behind an env var that was never set (see the port notes) -- nothing to
// port, and raw_members stays populated only via manual/admin means for now.
func SyncDay(ctx context.Context, db *gorm.DB, client *Client, date string) DaySyncResult {
	return SyncDayWith(ctx, db, client, date, SyncOptions{})
}

// SyncDayWith is SyncDay with an explicit write mode and log job name.
func SyncDayWith(ctx context.Context, db *gorm.DB, client *Client, date string, opts SyncOptions) DaySyncResult {
	jobName := opts.JobName
	if jobName == "" {
		jobName = "sync-esb"
	}
	entry := model.SyncLog{JobName: jobName, Status: "running", StartedAt: time.Now()}
	if targetDate, err := time.Parse("2006-01-02", date); err == nil {
		entry.TargetDate = &targetDate
	}
	if err := db.Create(&entry).Error; err != nil {
		// Matches the original: a failed log insert is logged but not fatal.
		log.Printf("sync-esb: failed to write sync_log for %s: %v", date, err)
	}

	result := DaySyncResult{Date: date}

	records, err := client.FetchSalesDay(ctx, date)
	if err != nil {
		finishLog(db, entry.ID, "failed", nil, err)
		result.Error = err.Error()
		return result
	}
	result.RecordsFetched = len(records)

	transformed := Transform(records, time.Now())
	found := transformed.Counts()
	result.Found = found
	var cmp *Comparison
	if opts.ReportChanges {
		c, err := CompareWithStored(db, date, transformed.Sales)
		if err != nil {
			finishLog(db, entry.ID, "failed", nil, err)
			result.Error = err.Error()
			return result
		}
		cmp = &c
	}
	upserted, err := UpsertMode(db, transformed, opts.Mode)
	if err != nil {
		finishLog(db, entry.ID, "failed", nil, err)
		result.Error = err.Error()
		return result
	}

	result.OK = true
	result.Outlets = upserted.Outlets
	result.Sales = upserted.Sales
	result.Payments = upserted.Payments
	result.MenuItems = upserted.Items

	if opts.Mode == ModeFillMissing {
		result.Existing = &UpsertResult{
			Outlets:  found.Outlets - upserted.Outlets,
			Sales:    found.Sales - upserted.Sales,
			Payments: found.Payments - upserted.Payments,
			Items:    found.Items - upserted.Items,
		}
	}

	if cmp != nil {
		result.SalesNew, result.SalesRefreshed = cmp.New, cmp.Refreshed
		result.StatusChanges, result.NotInESB = cmp.StatusChanges, cmp.NotInESB
	} else if opts.Mode == ModeFillMissing {
		result.SalesNew, result.SalesRefreshed = upserted.Sales, found.Sales-upserted.Sales
	} else {
		result.SalesNew = found.Sales // nightly: not diffed, all counted as written
	}

	rowsSynced := upserted.Total()
	finishLog(db, entry.ID, "success", &rowsSynced, nil)
	return result
}

func finishLog(db *gorm.DB, id int64, status string, rowsSynced *int, syncErr error) {
	if id == 0 {
		return
	}
	updates := map[string]any{
		"finished_at": time.Now(),
		"status":      status,
	}
	if rowsSynced != nil {
		updates["rows_synced"] = *rowsSynced
	}
	if syncErr != nil {
		updates["error_message"] = syncErr.Error()
	}
	if err := db.Model(&model.SyncLog{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		log.Printf("sync-esb: failed to update sync_log %d: %v", id, err)
	}
}

// RangeSummary is the overall result of SyncRange, one entry per day.
type RangeSummary struct {
	OK       bool            `json:"ok"`
	DateFrom string          `json:"date_from"`
	DateTo   string          `json:"date_to"`
	Days     []DaySyncResult `json:"days"`
}

// SyncRange runs SyncDay for every date in [from, to] inclusive. A failed
// day does NOT stop the batch -- matches the original exactly. OK is false
// only when every day failed. There is no 31-day cap: that limit only
// existed because of the Supabase Edge Function's execution time budget,
// which a long-running Go process doesn't share.
func SyncRange(ctx context.Context, db *gorm.DB, client *Client, from, to time.Time) RangeSummary {
	summary := RangeSummary{DateFrom: from.Format("2006-01-02"), DateTo: to.Format("2006-01-02")}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		summary.Days = append(summary.Days, SyncDay(ctx, db, client, d.Format("2006-01-02")))
	}

	allFailed := len(summary.Days) > 0
	for _, d := range summary.Days {
		if d.OK {
			allFailed = false
			break
		}
	}
	summary.OK = !allFailed
	return summary
}

// PingHealthchecks posts a plain success/failure ping if pingURL is set --
// ports healthchecks.ts. A no-op when pingURL is empty; errors are
// swallowed (best-effort), matching the original.
func PingHealthchecks(pingURL, summaryText string, failed bool) {
	if pingURL == "" {
		return
	}
	target := pingURL
	if failed {
		target = strings.TrimRight(pingURL, "/") + "/fail"
	}
	_, _ = http.Post(target, "text/plain", strings.NewReader(summaryText))
}

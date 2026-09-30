package etl_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Operations-ESB/dashboard-sales/api/internal/etl"
	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

func TestSyncDay_Success_WritesSyncLogAndUpserts(t *testing.T) {
	db := testutil.SetupTestDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"salesNum": "SN1", "salesDate": "2026-02-03", "branchCode": "BR01", "branchName": "Cabang A", "grandTotal": 50000, "statusName": "Finished"},
		})
	}))
	defer server.Close()

	client := etl.NewClient(server.URL, "test-key")
	result := etl.SyncDay(context.Background(), db, client, "2026-02-03")

	if !result.OK {
		t.Fatalf("result.OK = false, want true; error = %q", result.Error)
	}
	if result.Sales != 1 || result.Outlets != 1 {
		t.Errorf("result = %+v, want 1 outlet and 1 sale", result)
	}

	var logEntry model.SyncLog
	if err := db.Where("job_name = ?", "sync-esb").First(&logEntry).Error; err != nil {
		t.Fatalf("sync_logs row not found: %v", err)
	}
	if logEntry.Status != "success" {
		t.Errorf("sync_log.status = %q, want success", logEntry.Status)
	}
	if logEntry.RowsSynced == nil || *logEntry.RowsSynced != 2 { // 1 outlet + 1 sale
		t.Errorf("sync_log.rows_synced = %v, want 2", logEntry.RowsSynced)
	}
	if logEntry.FinishedAt == nil {
		t.Error("sync_log.finished_at is nil, want set")
	}
}

func TestSyncDay_Failure_WritesFailedSyncLogAndDoesNotPanic(t *testing.T) {
	db := testutil.SetupTestDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := etl.NewClient(server.URL, "test-key")
	result := etl.SyncDay(context.Background(), db, client, "2026-02-03")

	if result.OK {
		t.Fatal("result.OK = true, want false (server returned 500)")
	}
	if result.Error == "" {
		t.Error("result.Error is empty, want a message")
	}

	var logEntry model.SyncLog
	if err := db.Where("job_name = ?", "sync-esb").First(&logEntry).Error; err != nil {
		t.Fatalf("sync_logs row not found: %v", err)
	}
	if logEntry.Status != "failed" {
		t.Errorf("sync_log.status = %q, want failed", logEntry.Status)
	}
	if logEntry.ErrorMessage == nil || *logEntry.ErrorMessage == "" {
		t.Error("sync_log.error_message is empty, want the fetch error")
	}
	if logEntry.RowsSynced != nil {
		t.Errorf("sync_log.rows_synced = %v, want nil on failure", logEntry.RowsSynced)
	}
}

func TestSyncRange_OneFailedDayDoesNotStopTheBatch(t *testing.T) {
	db := testutil.SetupTestDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		date := r.URL.Query().Get("salesDateFrom")
		if date == "2026-02-02" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"salesNum": "SN-" + date, "salesDate": date, "branchCode": "BR01", "grandTotal": 1000, "statusName": "Finished"},
		})
	}))
	defer server.Close()

	client := etl.NewClient(server.URL, "test-key")
	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC)
	summary := etl.SyncRange(context.Background(), db, client, from, to)

	if len(summary.Days) != 3 {
		t.Fatalf("len(Days) = %d, want 3", len(summary.Days))
	}
	if !summary.Days[0].OK || summary.Days[1].OK || !summary.Days[2].OK {
		t.Errorf("day results = %+v, want [ok, failed, ok]", summary.Days)
	}
	if !summary.OK {
		t.Error("summary.OK = false, want true (not every day failed)")
	}
}

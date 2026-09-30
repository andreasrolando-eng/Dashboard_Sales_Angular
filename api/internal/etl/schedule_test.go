package etl_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Operations-ESB/dashboard-sales/api/internal/etl"
	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

func TestNextRun(t *testing.T) {
	wib := etl.WIB
	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"before today's run", time.Date(2026, 2, 3, 5, 59, 0, 0, wib), time.Date(2026, 2, 3, 6, 0, 0, 0, wib)},
		{"exactly at run time goes to tomorrow", time.Date(2026, 2, 3, 6, 0, 0, 0, wib), time.Date(2026, 2, 4, 6, 0, 0, 0, wib)},
		{"after today's run", time.Date(2026, 2, 3, 20, 0, 0, 0, wib), time.Date(2026, 2, 4, 6, 0, 0, 0, wib)},
		// 23:30 UTC on Feb 2 is 06:30 WIB on Feb 3 -> next run is Feb 4, not Feb 3.
		{"UTC input is converted to WIB", time.Date(2026, 2, 2, 23, 30, 0, 0, time.UTC), time.Date(2026, 2, 4, 6, 0, 0, 0, wib)},
		{"month rollover", time.Date(2026, 2, 28, 12, 0, 0, 0, wib), time.Date(2026, 3, 1, 6, 0, 0, 0, wib)},
	}
	for _, c := range cases {
		if got := etl.NextRun(c.now, 6); !got.Equal(c.want) {
			t.Errorf("%s: NextRun = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestMissingDates_OnlySuccessfulRunsCount(t *testing.T) {
	db := testutil.SetupTestDB(t)
	now := time.Date(2026, 2, 10, 7, 0, 0, 0, etl.WIB) // yesterday = Feb 9

	for date, status := range map[string]string{
		"2026-02-07": "success",
		"2026-02-08": "failed",  // failed -> still missing
		"2026-02-09": "running", // never finished -> still missing
		"2026-01-01": "success", // outside window, irrelevant
	} {
		d, _ := time.Parse("2006-01-02", date)
		if err := db.Create(&model.SyncLog{JobName: "sync-esb", TargetDate: &d, Status: status, StartedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	// A different job succeeding on Feb 6 must not count.
	d6 := time.Date(2026, 2, 6, 0, 0, 0, 0, time.UTC)
	db.Create(&model.SyncLog{JobName: "other-job", TargetDate: &d6, Status: "success", StartedAt: now})

	got, err := etl.MissingDates(db, now, 4) // window: Feb 6..9
	if err != nil {
		t.Fatal(err)
	}
	var days []string
	for _, d := range got {
		days = append(days, d.Format("2006-01-02"))
	}
	want := "2026-02-06,2026-02-08,2026-02-09"
	if strings.Join(days, ",") != want {
		t.Errorf("MissingDates = %v, want %s", days, want)
	}
}

func esbServer(t *testing.T, failDate string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		date := r.URL.Query().Get("salesDateFrom")
		if date == failDate {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"salesNum": "SN-" + date, "salesDate": date, "branchCode": "BR01", "grandTotal": 1000, "statusName": "Finished"},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestScheduler_RunOnce_CatchesUpAndAlertsOnFailure(t *testing.T) {
	db := testutil.SetupTestDB(t)
	esb := esbServer(t, "2026-02-08")

	var mu sync.Mutex
	var alerts []string
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var p map[string]string
		_ = json.Unmarshal(b, &p)
		mu.Lock()
		alerts = append(alerts, p["text"])
		mu.Unlock()
	}))
	defer hook.Close()

	s := &etl.Scheduler{
		DB: db, Client: etl.NewClient(esb.URL, "k"),
		Cfg: etl.SchedulerConfig{Hour: 6, CatchupDays: 3, AlertWebhookURL: hook.URL},
		Now: func() time.Time { return time.Date(2026, 2, 10, 6, 0, 0, 0, etl.WIB) }, // window Feb 7..9
	}

	results, skipped, err := s.RunOnce(context.Background())
	if err != nil || skipped {
		t.Fatalf("RunOnce err=%v skipped=%v", err, skipped)
	}
	if len(results) != 3 {
		t.Fatalf("synced %d days, want 3 (catch-up window)", len(results))
	}
	if len(alerts) != 1 || !strings.Contains(alerts[0], "2026-02-08") {
		t.Errorf("alerts = %v, want exactly one mentioning the failed 2026-02-08", alerts)
	}

	// Second run: only the failed day is retried; the two successes are not re-fetched.
	results, _, _ = s.RunOnce(context.Background())
	if len(results) != 1 || results[0].Date != "2026-02-08" || results[0].OK {
		t.Errorf("second run results = %+v, want only failed 2026-02-08 retried", results)
	}
}

func TestScheduler_RunOnce_NoAlertWhenAllGood(t *testing.T) {
	db := testutil.SetupTestDB(t)
	esb := esbServer(t, "")
	called := false
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer hook.Close()

	s := &etl.Scheduler{
		DB: db, Client: etl.NewClient(esb.URL, "k"),
		Cfg: etl.SchedulerConfig{Hour: 6, CatchupDays: 2, AlertWebhookURL: hook.URL},
		Now: func() time.Time { return time.Date(2026, 2, 10, 6, 0, 0, 0, etl.WIB) },
	}
	if results, _, err := s.RunOnce(context.Background()); err != nil || len(results) != 2 {
		t.Fatalf("results=%d err=%v, want 2 days", len(results), err)
	}
	if called {
		t.Error("alert webhook called although every day succeeded")
	}
	// Nothing left to do -> empty run, still no alert.
	if results, _, _ := s.RunOnce(context.Background()); len(results) != 0 {
		t.Errorf("third run synced %d days, want 0", len(results))
	}
}

func TestScheduler_RunOnce_SkipsWhenAnotherInstanceHoldsLock(t *testing.T) {
	db := testutil.SetupTestDB(t)
	esb := esbServer(t, "")

	// Simulate instance A mid-run by holding the same advisory lock in an open tx.
	holder := db.Begin()
	defer holder.Rollback()
	if err := holder.Exec("select pg_advisory_xact_lock(7402119001)").Error; err != nil {
		t.Fatal(err)
	}

	s := &etl.Scheduler{
		DB: db, Client: etl.NewClient(esb.URL, "k"),
		Cfg: etl.SchedulerConfig{Hour: 6, CatchupDays: 2},
		Now: func() time.Time { return time.Date(2026, 2, 10, 6, 0, 0, 0, etl.WIB) },
	}
	results, skipped, err := s.RunOnce(context.Background())
	if err != nil || !skipped || len(results) != 0 {
		t.Errorf("results=%d skipped=%v err=%v, want skipped with nothing synced", len(results), skipped, err)
	}
}

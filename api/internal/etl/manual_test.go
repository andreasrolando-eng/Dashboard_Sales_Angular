package etl_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Operations-ESB/dashboard-sales/api/internal/etl"
	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

func d(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func TestValidateManualRange(t *testing.T) {
	today := "2026-02-10"
	cases := []struct {
		name     string
		from, to string
		wantErr  bool
	}{
		{"single past day", "2026-02-03", "2026-02-03", false},
		{"up to today", "2026-02-08", "2026-02-10", false},
		{"start after end", "2026-02-05", "2026-02-04", true},
		{"end in the future", "2026-02-09", "2026-02-11", true},
		{"exactly the max span", "2025-12-11", "2026-02-10", false}, // 62 days
		{"one day over the max span", "2025-12-10", "2026-02-10", true},
	}
	for _, c := range cases {
		err := etl.ValidateManualRange(d(c.from), d(c.to), today)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", c.name, err, c.wantErr)
		}
	}
}

func waitDone(t *testing.T, m *etl.ManualSyncer) etl.ManualJob {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if j := m.Current(); j != nil && j.Status != etl.StatusRunning {
			return *j
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("manual sync did not finish in time")
	return etl.ManualJob{}
}

func TestManualSyncer_FillsMissingDaysAndNeverDuplicates(t *testing.T) {
	db := testutil.SetupTestDB(t)
	esb := esbServer(t, "")
	m := etl.NewManualSyncer(db, etl.NewClient(esb.URL, "k"))

	// Feb 2 already stored, with a value that differs from what ESB returns now.
	stored := saleFixture("SN-2026-02-02")
	stored.SalesDate = d("2026-02-02")
	stored.GrandTotal = 55555
	if _, err := etl.Upsert(db, etl.TransformResult{
		Outlets: []model.Outlet{{BranchCode: "BR01", BranchName: "Cabang A"}},
		Sales:   []model.RawSale{stored},
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := m.Start(d("2026-02-01"), d("2026-02-03"), etl.ManualFill); err != nil {
		t.Fatal(err)
	}
	job := waitDone(t, m)
	if job.Status != etl.StatusDone || len(job.Days) != 3 {
		t.Fatalf("job = %+v, want done with 3 days", job)
	}
	if got := job.Days[1].Result; got == nil || got.Sales != 0 || got.Existing == nil || got.Existing.Sales != 1 {
		t.Errorf("Feb 2 result = %+v, want 0 inserted / 1 already existing", got)
	}
	if got := job.Days[0].Result; got == nil || got.Sales != 1 {
		t.Errorf("Feb 1 result = %+v, want 1 inserted", got)
	}

	var n int64
	db.Model(&model.RawSale{}).Count(&n)
	if n != 3 {
		t.Errorf("raw_sales rows = %d, want 3 (one per day, none doubled)", n)
	}
	var kept model.RawSale
	db.First(&kept, "sales_num = ?", "SN-2026-02-02")
	if kept.GrandTotal != 55555 {
		t.Errorf("stored bill GrandTotal = %v, want the untouched 55555", kept.GrandTotal)
	}

	// Same range again: nothing new, still no duplicates.
	if _, err := m.Start(d("2026-02-01"), d("2026-02-03"), etl.ManualFill); err != nil {
		t.Fatal(err)
	}
	job = waitDone(t, m)
	for _, day := range job.Days {
		if day.Result == nil || day.Result.Sales != 0 || day.Result.Existing.Sales != 1 {
			t.Errorf("rerun day %s = %+v, want 0 inserted / 1 existing", day.Date, day.Result)
		}
	}
	db.Model(&model.RawSale{}).Count(&n)
	if n != 3 {
		t.Errorf("raw_sales rows after rerun = %d, want 3", n)
	}
}

func TestManualSyncer_RejectsSecondStartWhileRunning(t *testing.T) {
	db := testutil.SetupTestDB(t)

	release := make(chan struct{})
	var once sync.Once
	esb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer esb.Close()
	defer once.Do(func() { close(release) })

	m := etl.NewManualSyncer(db, etl.NewClient(esb.URL, "k"))
	if _, err := m.Start(d("2026-02-01"), d("2026-02-01"), etl.ManualFill); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(d("2026-02-02"), d("2026-02-02"), etl.ManualFill); err != etl.ErrBusy {
		t.Errorf("second Start err = %v, want ErrBusy", err)
	}
	if j := m.Current(); j == nil || j.Status != etl.StatusRunning {
		t.Errorf("Current = %+v, want the running job", j)
	}

	once.Do(func() { close(release) })
	if job := waitDone(t, m); job.Status != etl.StatusDone {
		t.Errorf("final status = %q, want done (an empty day is a success)", job.Status)
	}
}

func TestManualSyncer_FailsCleanlyWhenNightlySyncHoldsTheLock(t *testing.T) {
	db := testutil.SetupTestDB(t)

	hits := 0
	esb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer esb.Close()

	holder := db.Begin()
	defer holder.Rollback()
	if err := holder.Exec("select pg_advisory_xact_lock(7402119001)").Error; err != nil {
		t.Fatal(err)
	}

	m := etl.NewManualSyncer(db, etl.NewClient(esb.URL, "k"))
	if _, err := m.Start(d("2026-02-01"), d("2026-02-01"), etl.ManualFill); err != nil {
		t.Fatal(err)
	}
	job := waitDone(t, m)
	if job.Status != etl.StatusFailed || job.Error == "" {
		t.Errorf("job = %+v, want failed with an explanatory error", job)
	}
	if hits != 0 {
		t.Errorf("ESB was called %d times, want 0 while the lock is held", hits)
	}
}

func TestManualSyncer_ReportsFailedDaysWithoutStoppingTheRest(t *testing.T) {
	db := testutil.SetupTestDB(t)
	esb := esbServer(t, "2026-02-02")
	m := etl.NewManualSyncer(db, etl.NewClient(esb.URL, "k"))

	if _, err := m.Start(d("2026-02-01"), d("2026-02-03"), etl.ManualFill); err != nil {
		t.Fatal(err)
	}
	job := waitDone(t, m)
	if job.Status != etl.StatusDone {
		t.Errorf("status = %q, want done (2 of 3 days succeeded)", job.Status)
	}
	got := []string{job.Days[0].Status, job.Days[1].Status, job.Days[2].Status}
	want := []string{etl.StatusDone, etl.StatusFailed, etl.StatusDone}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("day statuses = %v, want %v", got, want)
			break
		}
	}
	if job.Days[1].Result == nil || job.Days[1].Result.Error == "" {
		t.Errorf("failed day should carry its error, got %+v", job.Days[1].Result)
	}
}

func TestManualSyncer_RejectsInvalidRangeWithoutStarting(t *testing.T) {
	db := testutil.SetupTestDB(t)
	m := etl.NewManualSyncer(db, etl.NewClient("http://127.0.0.1:1", "k"))
	if _, err := m.Start(d("2026-02-05"), d("2026-02-01"), etl.ManualFill); err == nil {
		t.Error("expected a validation error for from > to")
	}
	if m.Current() != nil {
		t.Error("an invalid request must not create a job")
	}
}

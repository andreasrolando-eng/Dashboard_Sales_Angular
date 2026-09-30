package etl_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/Operations-ESB/dashboard-sales/api/internal/etl"
	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

// switchableESB serves a per-date list of bills that the test can change
// between syncs -- like ESB, where a bill voided later keeps its original
// salesDate but comes back with a new statusName.
type switchableESB struct {
	srv  *httptest.Server
	mu   sync.Mutex
	bill map[string][]map[string]any
}

func newSwitchableESB(t *testing.T) *switchableESB {
	t.Helper()
	e := &switchableESB{bill: map[string][]map[string]any{}}
	e.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.mu.Lock()
		defer e.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(e.bill[r.URL.Query().Get("salesDateFrom")])
	}))
	t.Cleanup(e.srv.Close)
	return e
}

func (e *switchableESB) set(date string, bills ...map[string]any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bill[date] = bills
}

func bill(num, date, status string, total float64) map[string]any {
	return map[string]any{"salesNum": num, "billNum": "B-" + num, "salesDate": date, "branchCode": "BR01", "grandTotal": total, "statusName": status}
}

func TestManualRefresh_PicksUpAVoidAndReportsTheStatusChange(t *testing.T) {
	db := testutil.SetupTestDB(t)
	esb := newSwitchableESB(t)
	m := etl.NewManualSyncer(db, etl.NewClient(esb.srv.URL, "k"))

	// Yesterday: two finished bills get synced normally.
	esb.set("2026-02-03", bill("SN1", "2026-02-03", "Finished", 100000), bill("SN2", "2026-02-03", "Finished", 50000))
	if _, err := m.Start(d("2026-02-03"), d("2026-02-03"), etl.ManualFill); err != nil {
		t.Fatal(err)
	}
	waitDone(t, m)

	// Today SN1 is voided (still dated yesterday) and a late bill SN3 appears.
	esb.set("2026-02-03",
		bill("SN1", "2026-02-03", "Void", 100000),
		bill("SN2", "2026-02-03", "Finished", 50000),
		bill("SN3", "2026-02-03", "Finished", 20000))

	// "Lengkapi" must NOT touch SN1 -- this is exactly why refresh exists.
	if _, err := m.Start(d("2026-02-03"), d("2026-02-03"), etl.ManualFill); err != nil {
		t.Fatal(err)
	}
	waitDone(t, m)
	var sn1 model.RawSale
	db.First(&sn1, "sales_num = ?", "SN1")
	if sn1.StatusName == nil || *sn1.StatusName != "Finished" {
		t.Fatalf("after fill-only SN1 status = %v, want it untouched (Finished)", sn1.StatusName)
	}

	// Refresh picks the void up.
	if _, err := m.Start(d("2026-02-03"), d("2026-02-03"), etl.ManualRefresh); err != nil {
		t.Fatal(err)
	}
	job := waitDone(t, m)
	if job.Mode != etl.ManualRefresh || job.Status != etl.StatusDone {
		t.Fatalf("job = %+v, want a finished refresh", job)
	}
	res := job.Days[0].Result
	if res.SalesNew != 0 || res.SalesRefreshed != 3 {
		// SN3 was already inserted by the fill run above, so all 3 exist now.
		t.Errorf("new=%d refreshed=%d, want 0 new and 3 refreshed", res.SalesNew, res.SalesRefreshed)
	}
	if len(res.StatusChanges) != 1 || res.StatusChanges[0].SalesNum != "SN1" ||
		res.StatusChanges[0].From != "Finished" || res.StatusChanges[0].To != "Void" {
		t.Fatalf("status changes = %+v, want SN1 Finished -> Void", res.StatusChanges)
	}
	if res.StatusChanges[0].BillNum != "B-SN1" || res.StatusChanges[0].GrandTotal != 100000 {
		t.Errorf("change carries bill_num/total = %q/%v, want B-SN1/100000", res.StatusChanges[0].BillNum, res.StatusChanges[0].GrandTotal)
	}

	db.First(&sn1, "sales_num = ?", "SN1")
	if sn1.StatusName == nil || *sn1.StatusName != "Void" {
		t.Errorf("SN1 status after refresh = %v, want Void", sn1.StatusName)
	}
	var n int64
	db.Model(&model.RawSale{}).Count(&n)
	if n != 3 {
		t.Errorf("raw_sales rows = %d, want 3 (refresh overwrites, never duplicates)", n)
	}

	// Logged as a refresh, which never counts as a nightly sync.
	var logs int64
	db.Model(&model.SyncLog{}).Where("job_name = ?", etl.ManualRefreshJobName).Count(&logs)
	if logs != 1 {
		t.Errorf("refresh sync_logs = %d, want 1", logs)
	}
}

func TestManualRefresh_CountsNewBillsAndFlagsBillsESBNoLongerReturns(t *testing.T) {
	db := testutil.SetupTestDB(t)
	esb := newSwitchableESB(t)
	m := etl.NewManualSyncer(db, etl.NewClient(esb.srv.URL, "k"))

	esb.set("2026-02-03", bill("SN1", "2026-02-03", "Finished", 100), bill("SN2", "2026-02-03", "Finished", 200))
	m.Start(d("2026-02-03"), d("2026-02-03"), etl.ManualFill)
	waitDone(t, m)

	// SN2 disappears from ESB's answer, SN9 is new.
	esb.set("2026-02-03", bill("SN1", "2026-02-03", "Finished", 100), bill("SN9", "2026-02-03", "Finished", 900))
	m.Start(d("2026-02-03"), d("2026-02-03"), etl.ManualRefresh)
	res := waitDone(t, m).Days[0].Result

	if res.SalesNew != 1 || res.SalesRefreshed != 1 || len(res.StatusChanges) != 0 {
		t.Errorf("new=%d refreshed=%d changes=%d, want 1 new, 1 refreshed, 0 changes", res.SalesNew, res.SalesRefreshed, len(res.StatusChanges))
	}
	if len(res.NotInESB) != 1 || res.NotInESB[0].SalesNum != "SN2" {
		t.Fatalf("not in ESB = %+v, want SN2", res.NotInESB)
	}

	// Reported only: SN2 stays exactly as stored.
	var sn2 model.RawSale
	if err := db.First(&sn2, "sales_num = ?", "SN2").Error; err != nil {
		t.Fatalf("SN2 was deleted: %v", err)
	}
	if sn2.StatusName == nil || *sn2.StatusName != "Finished" || sn2.GrandTotal != 200 {
		t.Errorf("SN2 = %+v, want untouched", sn2)
	}
}

func TestParseManualMode(t *testing.T) {
	for in, want := range map[string]etl.ManualMode{"": etl.ManualFill, "fill": etl.ManualFill, "refresh": etl.ManualRefresh} {
		if got, err := etl.ParseManualMode(in); err != nil || got != want {
			t.Errorf("ParseManualMode(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := etl.ParseManualMode("overwrite"); err == nil {
		t.Error("unknown mode must be rejected")
	}
}

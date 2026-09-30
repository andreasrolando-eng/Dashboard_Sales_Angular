package etl_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/Operations-ESB/dashboard-sales/api/internal/etl"
	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

func amount(v float64) *float64 { return &v }

func TestUpsertMode_FillMissing_KeepsExistingRowsAndInsertsOnlyNewOnes(t *testing.T) {
	db := testutil.SetupTestDB(t)

	stored := saleFixture("SN1")
	stored.GrandTotal = 100000
	if _, err := etl.Upsert(db, etl.TransformResult{
		Outlets: []model.Outlet{{BranchCode: "BR01", BranchName: "Nama Tersimpan"}},
		Sales:   []model.RawSale{stored},
	}); err != nil {
		t.Fatal(err)
	}

	// ESB now returns SN1 with DIFFERENT values plus a brand-new bill SN2.
	changed := saleFixture("SN1")
	changed.GrandTotal = 999999
	fresh := saleFixture("SN2")
	res, err := etl.UpsertMode(db, etl.TransformResult{
		Outlets: []model.Outlet{{BranchCode: "BR01", BranchName: "Nama Baru"}},
		Sales:   []model.RawSale{changed, fresh},
	}, etl.ModeFillMissing)
	if err != nil {
		t.Fatal(err)
	}

	if res.Sales != 1 || res.Outlets != 0 {
		t.Errorf("inserted = %+v, want exactly 1 new sale and 0 outlets (both SN1 and BR01 already existed)", res)
	}

	var sales []model.RawSale
	db.Order("sales_num").Find(&sales)
	if len(sales) != 2 {
		t.Fatalf("sales rows = %d, want 2 (no duplicates)", len(sales))
	}
	if sales[0].GrandTotal != 100000 {
		t.Errorf("SN1.GrandTotal = %v, want 100000 -- fill-missing must NOT overwrite an existing bill", sales[0].GrandTotal)
	}
	var outlet model.Outlet
	db.First(&outlet, "branch_code = ?", "BR01")
	if outlet.BranchName != "Nama Tersimpan" {
		t.Errorf("outlet name = %q, want the stored one -- existing outlets must not be overwritten", outlet.BranchName)
	}
}

func TestUpsertMode_FillMissing_CompletesMissingChildRowsOfAnExistingBill(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mustCreateOutlet(t, db)

	// Bill SN1 was stored earlier with only payment P1 and line 0.
	if _, err := etl.Upsert(db, etl.TransformResult{
		Sales:    []model.RawSale{saleFixture("SN1")},
		Payments: []model.RawSalesPayment{{SalesNum: "SN1", SalesPaymentBackendID: "P1", PaymentAmount: amount(50)}},
		Items:    []model.RawSalesMenuItem{{SalesNum: "SN1", LineSeq: 0, MenuID: "M-A", SalesDate: fixtureDate, BranchCode: "BR01", Qty: 1, Total: 100}},
	}); err != nil {
		t.Fatal(err)
	}

	// ESB returns the full bill: P1 (changed amount), new P2, line 0 (changed), new line 1.
	res, err := etl.UpsertMode(db, etl.TransformResult{
		Sales: []model.RawSale{saleFixture("SN1")},
		Payments: []model.RawSalesPayment{
			{SalesNum: "SN1", SalesPaymentBackendID: "P1", PaymentAmount: amount(9999)},
			{SalesNum: "SN1", SalesPaymentBackendID: "P2", PaymentAmount: amount(70)},
		},
		Items: []model.RawSalesMenuItem{
			{SalesNum: "SN1", LineSeq: 0, MenuID: "M-A", SalesDate: fixtureDate, BranchCode: "BR01", Qty: 1, Total: 12345},
			{SalesNum: "SN1", LineSeq: 1, MenuID: "M-B", SalesDate: fixtureDate, BranchCode: "BR01", Qty: 2, Total: 200},
		},
	}, etl.ModeFillMissing)
	if err != nil {
		t.Fatal(err)
	}

	if res.Sales != 0 || res.Payments != 1 || res.Items != 1 {
		t.Errorf("inserted = %+v, want 0 sales, 1 payment (P2), 1 item (line 1)", res)
	}

	var p1 model.RawSalesPayment
	db.First(&p1, "sales_num = ? and sales_payment_backend_id = ?", "SN1", "P1")
	if p1.PaymentAmount == nil || *p1.PaymentAmount != 50 {
		t.Errorf("P1 amount = %v, want the stored 50", p1.PaymentAmount)
	}
	var line0 model.RawSalesMenuItem
	db.First(&line0, "sales_num = ? and line_seq = ?", "SN1", 0)
	if line0.Total != 100 {
		t.Errorf("line 0 total = %v, want the stored 100", line0.Total)
	}
	var payments, items int64
	db.Model(&model.RawSalesPayment{}).Count(&payments)
	db.Model(&model.RawSalesMenuItem{}).Count(&items)
	if payments != 2 || items != 2 {
		t.Errorf("payments=%d items=%d, want 2 and 2 (missing rows filled in, none duplicated)", payments, items)
	}
}

// Counting relies on RowsAffected being summed across CreateInBatches
// batches (500 rows each) -- easy to get wrong, so cover >1 batch.
func TestUpsertMode_FillMissing_CountsAcrossMultipleBatches(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mustCreateOutlet(t, db)

	const total, preexisting = 1200, 300
	build := func(from, to int) []model.RawSale {
		out := make([]model.RawSale, 0, to-from)
		for i := from; i < to; i++ {
			out = append(out, saleFixture(fmt.Sprintf("SN%05d", i)))
		}
		return out
	}
	if _, err := etl.Upsert(db, etl.TransformResult{Sales: build(0, preexisting)}); err != nil {
		t.Fatal(err)
	}

	res, err := etl.UpsertMode(db, etl.TransformResult{Sales: build(0, total)}, etl.ModeFillMissing)
	if err != nil {
		t.Fatal(err)
	}
	if res.Sales != total-preexisting {
		t.Errorf("inserted sales = %d, want %d", res.Sales, total-preexisting)
	}
	var n int64
	db.Model(&model.RawSale{}).Count(&n)
	if n != total {
		t.Errorf("rows = %d, want %d", n, total)
	}

	// Running it again must insert nothing.
	res, err = etl.UpsertMode(db, etl.TransformResult{Sales: build(0, total)}, etl.ModeFillMissing)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total() != 0 {
		t.Errorf("second identical run inserted %+v, want nothing", res)
	}
}

func TestSyncDayWith_FillMissing_LogsUnderManualJobAndReportsExisting(t *testing.T) {
	db := testutil.SetupTestDB(t)
	esb := esbServer(t, "")
	client := etl.NewClient(esb.URL, "k")

	opts := etl.SyncOptions{Mode: etl.ModeFillMissing, JobName: etl.ManualJobName}
	first := etl.SyncDayWith(t.Context(), db, client, "2026-02-03", opts)
	if !first.OK || first.Sales != 1 || first.Existing == nil || first.Existing.Sales != 0 {
		t.Fatalf("first run = %+v, want 1 new sale and 0 existing", first)
	}
	if first.Found.Sales != 1 {
		t.Errorf("Found.Sales = %d, want 1", first.Found.Sales)
	}

	second := etl.SyncDayWith(t.Context(), db, client, "2026-02-03", opts)
	if !second.OK || second.Sales != 0 || second.Existing == nil || second.Existing.Sales != 1 {
		t.Fatalf("second run = %+v, want 0 new and 1 already existing", second)
	}

	var n int64
	db.Model(&model.RawSale{}).Count(&n)
	if n != 1 {
		t.Errorf("raw_sales rows = %d, want 1", n)
	}

	var manual, nightly int64
	db.Model(&model.SyncLog{}).Where("job_name = ?", etl.ManualJobName).Count(&manual)
	db.Model(&model.SyncLog{}).Where("job_name = ?", "sync-esb").Count(&nightly)
	if manual != 2 || nightly != 0 {
		t.Errorf("sync_logs manual=%d nightly=%d, want 2 and 0", manual, nightly)
	}

	// A manual run must not make the scheduler think the day is already synced.
	now := time.Date(2026, 2, 4, 6, 0, 0, 0, etl.WIB)
	missing, err := etl.MissingDates(db, now, 1)
	if err != nil || len(missing) != 1 || missing[0].Format("2006-01-02") != "2026-02-03" {
		t.Errorf("MissingDates = %v (err %v), want 2026-02-03 still missing after a manual run", missing, err)
	}
}

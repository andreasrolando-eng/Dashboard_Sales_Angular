package etl_test

import (
	"testing"
	"time"

	"github.com/Operations-ESB/dashboard-sales/api/internal/etl"
	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

func TestUpsert_IdempotentReUpsertUpdatesInPlace(t *testing.T) {
	db := testutil.SetupTestDB(t)

	first := etl.TransformResult{
		Outlets: []model.Outlet{{BranchCode: "BR01", BranchName: "Cabang Lama"}},
		Sales: []model.RawSale{{
			SalesNum: "SN1", SalesDate: time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC),
			BranchCode: "BR01", GrandTotal: 100000, Raw: []byte(`{}`), SyncedAt: time.Now(),
		}},
	}
	if _, err := etl.Upsert(db, first); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}

	// Re-sync the same day with updated values -- simulates ESB data
	// changing between syncs (e.g. a bill edited after the fact).
	second := etl.TransformResult{
		Outlets: []model.Outlet{{BranchCode: "BR01", BranchName: "Cabang Baru"}},
		Sales: []model.RawSale{{
			SalesNum: "SN1", SalesDate: time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC),
			BranchCode: "BR01", GrandTotal: 150000, Raw: []byte(`{}`), SyncedAt: time.Now(),
		}},
	}
	if _, err := etl.Upsert(db, second); err != nil {
		t.Fatalf("second Upsert: %v", err)
	}

	var outletCount, saleCount int64
	db.Model(&model.Outlet{}).Count(&outletCount)
	db.Model(&model.RawSale{}).Count(&saleCount)
	if outletCount != 1 || saleCount != 1 {
		t.Fatalf("outlet count = %d, sale count = %d, want 1 each (upsert must not duplicate rows)", outletCount, saleCount)
	}

	var outlet model.Outlet
	db.First(&outlet, "branch_code = ?", "BR01")
	if outlet.BranchName != "Cabang Baru" {
		t.Errorf("outlet.BranchName = %q, want updated value %q", outlet.BranchName, "Cabang Baru")
	}
	// Regression check: FirstSeenAt/LastSeenAt must come from the column's
	// `default now()`, not Go's zero time.Time{} (year 0001) -- that
	// happened once already because Create() sends every struct field
	// explicitly unless it's omitted.
	if outlet.FirstSeenAt.Year() < 2000 {
		t.Errorf("outlet.FirstSeenAt = %v, want the DB default (now()), not the Go zero value", outlet.FirstSeenAt)
	}
	if outlet.LastSeenAt.Year() < 2000 {
		t.Errorf("outlet.LastSeenAt = %v, want the DB default (now()), not the Go zero value", outlet.LastSeenAt)
	}

	var sale model.RawSale
	db.First(&sale, "sales_num = ?", "SN1")
	if sale.GrandTotal != 150000 {
		t.Errorf("sale.GrandTotal = %v, want updated value 150000", sale.GrandTotal)
	}
}

func TestUpsert_StaleChildRowsAreNotDeleted(t *testing.T) {
	db := testutil.SetupTestDB(t)

	mustCreateOutlet(t, db)

	// First sync: 2 menu line items.
	first := etl.TransformResult{
		Sales: []model.RawSale{saleFixture("SN1")},
		Items: []model.RawSalesMenuItem{
			{SalesNum: "SN1", LineSeq: 0, MenuID: "M-A", SalesDate: fixtureDate, BranchCode: "BR01", Qty: 1, Total: 100},
			{SalesNum: "SN1", LineSeq: 1, MenuID: "M-B", SalesDate: fixtureDate, BranchCode: "BR01", Qty: 1, Total: 200},
		},
	}
	if _, err := etl.Upsert(db, first); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}

	// Re-sync with only 1 line item now (ESB says the bill was edited down
	// to one line). The original Edge Function never deletes -- the stale
	// line_seq=1 row stays. This test documents that behavior, not
	// endorses it as ideal; see the Upsert doc comment.
	second := etl.TransformResult{
		Sales: []model.RawSale{saleFixture("SN1")},
		Items: []model.RawSalesMenuItem{
			{SalesNum: "SN1", LineSeq: 0, MenuID: "M-A", SalesDate: fixtureDate, BranchCode: "BR01", Qty: 1, Total: 100},
		},
	}
	if _, err := etl.Upsert(db, second); err != nil {
		t.Fatalf("second Upsert: %v", err)
	}

	var count int64
	db.Model(&model.RawSalesMenuItem{}).Where("sales_num = ?", "SN1").Count(&count)
	if count != 2 {
		t.Errorf("menu item count = %d, want 2 (stale line_seq=1 row is never deleted, matching the original)", count)
	}
}

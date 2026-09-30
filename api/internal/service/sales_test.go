package service_test

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/service"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

func mustCreate(t *testing.T, db *gorm.DB, v any) {
	t.Helper()
	if err := db.Create(v).Error; err != nil {
		t.Fatalf("create %T: %v", v, err)
	}
}

func f64ptr(v float64) *float64 { return &v }

// TestGetSalesSummary_NettFormulaAndFilters seeds two Finished sales (one
// with a member, one without) and one Cancelled sale, and checks the exact
// nett_sales formula (subtotal - other_tax_total - vat_total -
// other_vat_total - discount_total - rounding_total) plus that Cancelled
// sales are excluded entirely.
func TestGetSalesSummary_NettFormulaAndFilters(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mustCreate(t, db, &model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"})

	mustCreate(t, db, &model.RawSale{
		SalesNum: "SN-A", SalesDate: date(2026, 2, 3), BranchCode: "BR01",
		StatusName: strPtr("Finished"), GrandTotal: 113000, Raw: []byte(`{}`),
		Subtotal: f64ptr(100000), OtherTaxTotal: f64ptr(5000), VatTotal: f64ptr(10000),
		OtherVatTotal: f64ptr(0), DiscountTotal: f64ptr(2000), RoundingTotal: f64ptr(0),
	})
	mustCreate(t, db, &model.RawSale{
		SalesNum: "SN-B", SalesDate: date(2026, 2, 3), BranchCode: "BR01",
		StatusName: strPtr("Finished"), GrandTotal: 50000, Raw: []byte(`{}`),
		Subtotal: f64ptr(50000), OtherTaxTotal: f64ptr(0), VatTotal: f64ptr(0),
		OtherVatTotal: f64ptr(0), DiscountTotal: f64ptr(0), RoundingTotal: f64ptr(0),
		MemberCode: strPtr("M1"),
	})
	mustCreate(t, db, &model.RawSale{
		SalesNum: "SN-C", SalesDate: date(2026, 2, 3), BranchCode: "BR01",
		StatusName: strPtr("Cancelled"), GrandTotal: 999999, Raw: []byte(`{}`),
		Subtotal: f64ptr(999999),
	})

	summary, err := service.GetSalesSummary(db, date(2026, 2, 1), date(2026, 2, 7), nil)
	if err != nil {
		t.Fatalf("GetSalesSummary: %v", err)
	}
	if summary.Revenue != 163000 {
		t.Errorf("revenue = %v, want 163000 (Cancelled excluded)", summary.Revenue)
	}
	if summary.NettSales != 133000 {
		t.Errorf("nett_sales = %v, want 133000 (83000 + 50000)", summary.NettSales)
	}
	if summary.TransCount != 2 {
		t.Errorf("trans_count = %v, want 2", summary.TransCount)
	}
	if summary.MemberRevenue != 50000 {
		t.Errorf("member_revenue = %v, want 50000 (only SN-B has member_code)", summary.MemberRevenue)
	}
}

// TestGetRevenueByOutlet_IncludesZeroRevenueOutlet checks that an outlet
// with no sales in range still appears, with zeros -- the query is driven
// from `outlets`, not from `raw_sales`.
func TestGetRevenueByOutlet_IncludesZeroRevenueOutlet(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mustCreate(t, db, &model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"})
	mustCreate(t, db, &model.Outlet{BranchCode: "BR02", BranchName: "Cabang B"})

	mustCreate(t, db, &model.RawSale{
		SalesNum: "SN-A", SalesDate: date(2026, 2, 3), BranchCode: "BR01",
		StatusName: strPtr("Finished"), GrandTotal: 100000, Raw: []byte(`{}`),
		Subtotal: f64ptr(100000), OtherTaxTotal: f64ptr(0), VatTotal: f64ptr(0),
		OtherVatTotal: f64ptr(0), DiscountTotal: f64ptr(0), RoundingTotal: f64ptr(0),
	})

	rows, err := service.GetRevenueByOutlet(db, date(2026, 2, 1), date(2026, 2, 7))
	if err != nil {
		t.Fatalf("GetRevenueByOutlet: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2 (both outlets)", len(rows))
	}
	// Ordered by revenue desc: BR01 (100000) then BR02 (0).
	if rows[0].BranchCode != "BR01" || rows[0].Revenue != 100000 {
		t.Errorf("rows[0] = %+v, want BR01 with revenue 100000", rows[0])
	}
	if rows[1].BranchCode != "BR02" || rows[1].Revenue != 0 || rows[1].NettSales != 0 || rows[1].TransCount != 0 {
		t.Errorf("rows[1] = %+v, want BR02 with all-zero metrics", rows[1])
	}
}

func seedFinishedMenuLine(t *testing.T, db *gorm.DB, salesNum, branchCode string, salesDate time.Time, menuID, categoryID string, qty, revenue float64) {
	t.Helper()
	mustCreate(t, db, &model.RawSale{
		SalesNum: salesNum, SalesDate: salesDate, BranchCode: branchCode,
		StatusName: strPtr("Finished"), GrandTotal: revenue, Raw: []byte(`{}`),
	})
	mustCreate(t, db, &model.RawSalesMenuItem{
		SalesNum: salesNum, LineSeq: 1, MenuID: menuID, MenuName: strPtr(menuID),
		SalesDate: salesDate, BranchCode: branchCode,
		MenuCategoryID: strPtr(categoryID), MenuCategoryName: strPtr(categoryID),
		Qty: qty, Total: revenue,
	})
}

// TestGetMenuPerformance_TrendContributionAndTakeout covers every quirk
// documented for the ported fn_menu_performance: previous-period trend bands
// (naik/turun/stagnan, including "no previous data" -> stagnan),
// contribution_pct computed against the TOTAL of every product in scope
// (unaffected by the category filter), takeout-candidate threshold, and
// qty-ascending ordering.
func TestGetMenuPerformance_TrendContributionAndTakeout(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mustCreate(t, db, &model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"})

	current := date(2026, 2, 3)   // inside [2026-02-01, 2026-02-07]
	previous := date(2026, 1, 27) // inside the 7-day previous period [2026-01-25, 2026-01-31]

	// Category CAT1: menus A-D exercise every trend case.
	seedFinishedMenuLine(t, db, "CUR-A", "BR01", current, "A", "CAT1", 20, 100) // naik (20 > 10*1.1)
	seedFinishedMenuLine(t, db, "PREV-A", "BR01", previous, "A", "CAT1", 10, 100)
	seedFinishedMenuLine(t, db, "CUR-B", "BR01", current, "B", "CAT1", 5, 50) // turun (5 < 10*0.9)
	seedFinishedMenuLine(t, db, "PREV-B", "BR01", previous, "B", "CAT1", 10, 50)
	seedFinishedMenuLine(t, db, "CUR-C", "BR01", current, "C", "CAT1", 10, 30) // stagnan (equal)
	seedFinishedMenuLine(t, db, "PREV-C", "BR01", previous, "C", "CAT1", 10, 30)
	seedFinishedMenuLine(t, db, "CUR-D", "BR01", current, "D", "CAT1", 8, 20) // stagnan (no previous data)
	// Category CAT2: menu E, only affects the unfiltered total.
	seedFinishedMenuLine(t, db, "CUR-E", "BR01", current, "E", "CAT2", 1, 5)

	threshold := 15.0

	t.Run("unfiltered", func(t *testing.T) {
		rows, err := service.GetMenuPerformance(db, date(2026, 2, 1), date(2026, 2, 7), nil, nil, nil, threshold)
		if err != nil {
			t.Fatalf("GetMenuPerformance: %v", err)
		}
		if len(rows) != 5 {
			t.Fatalf("len(rows) = %d, want 5 (A,B,C,D,E)", len(rows))
		}
		// Ordered by qty asc: B(5), E(1)... wait E has qty 1, smallest.
		want := []struct {
			menuID             string
			trend              string
			isTakeoutCandidate bool
			contributionPct    float64
		}{
			{"E", "stagnan", true, 2.4},  // 5/205*100
			{"B", "turun", true, 24.4},   // 50/205*100
			{"D", "stagnan", true, 9.8},  // 20/205*100
			{"C", "stagnan", true, 14.6}, // 30/205*100
			{"A", "naik", false, 48.8},   // 100/205*100
		}
		for i, w := range want {
			r := rows[i]
			if r.MenuID != w.menuID {
				t.Errorf("rows[%d].MenuID = %q, want %q", i, r.MenuID, w.menuID)
				continue
			}
			if r.Trend != w.trend {
				t.Errorf("menu %s: trend = %q, want %q", w.menuID, r.Trend, w.trend)
			}
			if r.IsTakeoutCandidate != w.isTakeoutCandidate {
				t.Errorf("menu %s: is_takeout_candidate = %v, want %v", w.menuID, r.IsTakeoutCandidate, w.isTakeoutCandidate)
			}
			if r.ContributionPct == nil || !almostEqual(*r.ContributionPct, w.contributionPct, 0.05) {
				t.Errorf("menu %s: contribution_pct = %v, want ~%v", w.menuID, r.ContributionPct, w.contributionPct)
			}
		}
	})

	t.Run("filtered by category still uses the unfiltered total", func(t *testing.T) {
		cat1 := "CAT1"
		rows, err := service.GetMenuPerformance(db, date(2026, 2, 1), date(2026, 2, 7), nil, &cat1, nil, threshold)
		if err != nil {
			t.Fatalf("GetMenuPerformance: %v", err)
		}
		if len(rows) != 4 {
			t.Fatalf("len(rows) = %d, want 4 (E excluded by category filter)", len(rows))
		}
		for _, r := range rows {
			if r.MenuID == "A" && (r.ContributionPct == nil || !almostEqual(*r.ContributionPct, 48.8, 0.05)) {
				t.Errorf("menu A contribution_pct = %v, want ~48.8 even when filtered to CAT1 (denominator is the unfiltered total)", r.ContributionPct)
			}
		}
	})
}

func almostEqual(a, b, tolerance float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= tolerance
}

// TestGetTopProducts_SortDirectionAndLimit checks both sort directions
// (Top Seller = revenue desc, Slow Moving = qty asc) and that limit <= 0
// means unlimited.
func TestGetTopProducts_SortDirectionAndLimit(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mustCreate(t, db, &model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"})

	d := date(2026, 2, 3)
	seedFinishedMenuLine(t, db, "SN-A", "BR01", d, "A", "CAT1", 5, 300)
	seedFinishedMenuLine(t, db, "SN-B", "BR01", d, "B", "CAT1", 20, 100)
	seedFinishedMenuLine(t, db, "SN-C", "BR01", d, "C", "CAT1", 10, 200)

	t.Run("revenue desc, unlimited", func(t *testing.T) {
		rows, err := service.GetTopProducts(db, date(2026, 2, 1), date(2026, 2, 7), nil, nil, nil, "revenue", true, 0)
		if err != nil {
			t.Fatalf("GetTopProducts: %v", err)
		}
		if len(rows) != 3 {
			t.Fatalf("len(rows) = %d, want 3", len(rows))
		}
		if rows[0].MenuID != "A" || rows[1].MenuID != "C" || rows[2].MenuID != "B" {
			t.Errorf("order = %v, want A,C,B by revenue desc", menuIDs(rows))
		}
	})

	t.Run("qty asc for Slow Moving, limited", func(t *testing.T) {
		rows, err := service.GetTopProducts(db, date(2026, 2, 1), date(2026, 2, 7), nil, nil, nil, "qty", false, 2)
		if err != nil {
			t.Fatalf("GetTopProducts: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("len(rows) = %d, want 2 (limit)", len(rows))
		}
		if rows[0].MenuID != "A" || rows[1].MenuID != "C" {
			t.Errorf("order = %v, want A,C by qty asc", menuIDs(rows))
		}
	})
}

func menuIDs(rows []service.Product) []string {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.MenuID
	}
	return ids
}

func TestGetSalesBills_Pagination(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mustCreate(t, db, &model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"})

	for i, billNum := range []string{"B1", "B2", "B3"} {
		mustCreate(t, db, &model.RawSale{
			SalesNum: "SN-" + billNum, BillNum: strPtr(billNum),
			SalesDate: date(2026, 2, 1+i), BranchCode: "BR01",
			StatusName: strPtr("Finished"), GrandTotal: 1000, Raw: []byte(`{}`),
		})
	}

	rows, total, err := service.GetSalesBills(db, date(2026, 2, 1), date(2026, 2, 7), nil, 1, 2)
	if err != nil {
		t.Fatalf("GetSalesBills page 1: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2 (page size)", len(rows))
	}

	rows2, _, err := service.GetSalesBills(db, date(2026, 2, 1), date(2026, 2, 7), nil, 2, 2)
	if err != nil {
		t.Fatalf("GetSalesBills page 2: %v", err)
	}
	if len(rows2) != 1 {
		t.Fatalf("len(rows2) = %d, want 1 (last page)", len(rows2))
	}
}

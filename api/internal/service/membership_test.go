package service_test

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/service"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

func seedMemberVisit(t *testing.T, db *gorm.DB, salesNum, branchCode, memberCode string, salesDate time.Time, grandTotal float64) {
	t.Helper()
	mustCreate(t, db, &model.RawSale{
		SalesNum: salesNum, SalesDate: salesDate, BranchCode: branchCode,
		StatusName: strPtr("Finished"), GrandTotal: grandTotal, Raw: []byte(`{}`),
		MemberCode: strPtr(memberCode),
	})
}

// TestGetMembershipSummary_ActiveChurnRetentionAndFrequency exercises every
// quirk of the ported fn_membership_summary: total/active are lifetime
// counts (member visits outside the query range still count), the active
// cutoff is dateEnd-60 days, retention splits the range at its midpoint, and
// visit_frequency averages only visits inside the range.
func TestGetMembershipSummary_ActiveChurnRetentionAndFrequency(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mustCreate(t, db, &model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"})

	dateStart := date(2026, 1, 1)
	dateEnd := date(2026, 1, 30) // range is 29 days; mid = Jan1 + floor(29/2)=14 -> Jan15

	// M1: visits in both halves -> retained, active (last_seen Jan20).
	seedMemberVisit(t, db, "S1", "BR01", "M1", date(2026, 1, 5), 10000)
	seedMemberVisit(t, db, "S2", "BR01", "M1", date(2026, 1, 20), 20000)
	// M2: first half only -> not retained, active (last_seen Jan3).
	seedMemberVisit(t, db, "S3", "BR01", "M2", date(2026, 1, 3), 5000)
	// M3: second half only -> active (last_seen Jan25), not in first_half at all.
	seedMemberVisit(t, db, "S4", "BR01", "M3", date(2026, 1, 25), 8000)
	// M4: last visit is far outside the query range and before the active
	// cutoff (dateEnd-60 = ~2025-12-01) -> counts toward total_members but
	// NOT active_members, and doesn't affect retention/visit_frequency since
	// it falls outside [dateStart, dateEnd].
	seedMemberVisit(t, db, "S5", "BR01", "M4", date(2025, 11, 1), 1000)

	summary, err := service.GetMembershipSummary(db, dateStart, dateEnd, nil)
	if err != nil {
		t.Fatalf("GetMembershipSummary: %v", err)
	}
	if summary.TotalMembers != 4 {
		t.Errorf("total_members = %d, want 4", summary.TotalMembers)
	}
	if summary.ActiveMembers != 3 {
		t.Errorf("active_members = %d, want 3 (M4 is inactive)", summary.ActiveMembers)
	}
	if summary.ActivePct == nil || !almostEqual(*summary.ActivePct, 75.0, 0.05) {
		t.Errorf("active_pct = %v, want ~75.0", summary.ActivePct)
	}
	if summary.ChurnPct == nil || !almostEqual(*summary.ChurnPct, 25.0, 0.05) {
		t.Errorf("churn_pct = %v, want ~25.0", summary.ChurnPct)
	}
	// first_half = {M1, M2}, second_half = {M1, M3}; retained = {M1} -> 1/2 = 50%.
	if summary.RetentionPct == nil || !almostEqual(*summary.RetentionPct, 50.0, 0.05) {
		t.Errorf("retention_pct = %v, want ~50.0", summary.RetentionPct)
	}
	// period_visits (within range): M1=2, M2=1, M3=1 -> avg 1.333, months=1 -> ~1.3.
	if summary.VisitFrequency == nil || !almostEqual(*summary.VisitFrequency, 1.3, 0.05) {
		t.Errorf("visit_frequency = %v, want ~1.3", summary.VisitFrequency)
	}
}

// TestGetTopMembers_TierBracketFavoriteMenuAndOrdering checks the lifetime
// spending-bracket tier fallback (Gold >= 5,000,000, Silver >= 2,000,000,
// else Bronze), favorite_menu tie-break (qty desc then menu_name asc), and
// that member_name IS included (unlike the MCP tool, which strips it).
func TestGetTopMembers_TierBracketFavoriteMenuAndOrdering(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mustCreate(t, db, &model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"})

	d := date(2026, 2, 3)
	mustCreate(t, db, &model.RawSale{
		SalesNum: "S-GOLD", SalesDate: d, BranchCode: "BR01", StatusName: strPtr("Finished"),
		GrandTotal: 5000000, Raw: []byte(`{}`), MemberCode: strPtr("GOLD"), MemberName: strPtr("Member Gold"),
	})
	// Two menus with the SAME qty on the same bill -- "Apple" should win the
	// favorite_menu tie over "Zebra" (menu_name ascending tiebreak).
	mustCreate(t, db, &model.RawSalesMenuItem{
		SalesNum: "S-GOLD", LineSeq: 1, MenuID: "MA", MenuName: strPtr("Apple"),
		SalesDate: d, BranchCode: "BR01", Qty: 2, Total: 100,
	})
	mustCreate(t, db, &model.RawSalesMenuItem{
		SalesNum: "S-GOLD", LineSeq: 2, MenuID: "MZ", MenuName: strPtr("Zebra"),
		SalesDate: d, BranchCode: "BR01", Qty: 2, Total: 100,
	})

	mustCreate(t, db, &model.RawSale{
		SalesNum: "S-SILVER", SalesDate: d, BranchCode: "BR01", StatusName: strPtr("Finished"),
		GrandTotal: 2500000, Raw: []byte(`{}`), MemberCode: strPtr("SILVER"), MemberName: strPtr("Member Silver"),
	})
	mustCreate(t, db, &model.RawSale{
		SalesNum: "S-BRONZE", SalesDate: d, BranchCode: "BR01", StatusName: strPtr("Finished"),
		GrandTotal: 100000, Raw: []byte(`{}`), MemberCode: strPtr("BRONZE"), MemberName: strPtr("Member Bronze"),
	})

	rows, err := service.GetTopMembers(db, date(2026, 2, 1), date(2026, 2, 7), nil, 10)
	if err != nil {
		t.Fatalf("GetTopMembers: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}
	// Ordered by spending desc: GOLD, SILVER, BRONZE.
	if rows[0].MemberCode != "GOLD" || rows[0].Tier == nil || *rows[0].Tier != "Gold" {
		t.Errorf("rows[0] = %+v, want GOLD with tier Gold", rows[0])
	}
	if rows[0].MemberName == nil || *rows[0].MemberName != "Member Gold" {
		t.Errorf("rows[0].MemberName = %v, want %q (dashboard API must include member_name)", rows[0].MemberName, "Member Gold")
	}
	if rows[0].FavoriteMenu == nil || *rows[0].FavoriteMenu != "Apple" {
		t.Errorf("rows[0].FavoriteMenu = %v, want %q (tie broken by menu_name asc)", rows[0].FavoriteMenu, "Apple")
	}
	if rows[1].MemberCode != "SILVER" || rows[1].Tier == nil || *rows[1].Tier != "Silver" {
		t.Errorf("rows[1] = %+v, want SILVER with tier Silver", rows[1])
	}
	if rows[2].MemberCode != "BRONZE" || rows[2].Tier == nil || *rows[2].Tier != "Bronze" {
		t.Errorf("rows[2] = %+v, want BRONZE with tier Bronze", rows[2])
	}
}

func TestGetMemberOptions(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mustCreate(t, db, &model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"})
	seedMemberVisit(t, db, "S1", "BR01", "M1", date(2026, 2, 3), 10000)

	opts, err := service.GetMemberOptions(db, nil)
	if err != nil {
		t.Fatalf("GetMemberOptions: %v", err)
	}
	if len(opts) != 1 || opts[0].MemberCode != "M1" {
		t.Errorf("opts = %+v, want one row for M1", opts)
	}
}

func TestGetMemberMenuPurchases(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mustCreate(t, db, &model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"})

	d := date(2026, 2, 3)
	mustCreate(t, db, &model.RawSale{
		SalesNum: "S1", SalesDate: d, BranchCode: "BR01", StatusName: strPtr("Finished"),
		GrandTotal: 50000, Raw: []byte(`{}`), MemberCode: strPtr("M1"),
	})
	mustCreate(t, db, &model.RawSalesMenuItem{
		SalesNum: "S1", LineSeq: 1, MenuID: "M-A", MenuName: strPtr("Nasi Goreng"),
		SalesDate: d, BranchCode: "BR01", Qty: 2, Total: 50000,
	})

	rows, err := service.GetMemberMenuPurchases(db, "M1", date(2026, 2, 1), date(2026, 2, 7), nil)
	if err != nil {
		t.Fatalf("GetMemberMenuPurchases: %v", err)
	}
	if len(rows) != 1 || rows[0].Qty != 2 || rows[0].Revenue != 50000 {
		t.Errorf("rows = %+v, want one row qty=2 revenue=50000", rows)
	}
}

func TestGetMembershipNewWeekly_WindowedToEightWeeks(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mustCreate(t, db, &model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"})

	dateEnd := date(2026, 3, 1)
	// Inside the 56-day window.
	seedMemberVisit(t, db, "S1", "BR01", "RECENT", date(2026, 2, 1), 10000)
	// Outside the window (first_seen way before dateEnd-56).
	seedMemberVisit(t, db, "S2", "BR01", "OLD", date(2025, 1, 1), 10000)

	rows, err := service.GetMembershipNewWeekly(db, dateEnd)
	if err != nil {
		t.Fatalf("GetMembershipNewWeekly: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1 (only RECENT's week is in the 56-day window)", len(rows))
	}
}

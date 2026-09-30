package service_test

import (
	"testing"

	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/service"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

// TestGetPromoPerformance_LiftRoiAndStatus builds a baseline of two non-promo
// bills (avg 110000, one of them using the '0' sentinel instead of NULL --
// both must count as non-promo) and three promos exercising every branch of
// the "Efektif" rule (lift >= 15% AND roi >= 2, checked on the UNROUNDED
// values): one that clears both thresholds, one that clears neither, and one
// with high lift but low ROI to prove the AND (not OR) semantics.
func TestGetPromoPerformance_LiftRoiAndStatus(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mustCreate(t, db, &model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"})

	d := date(2026, 2, 3)
	seedSale := func(salesNum string, grandTotal float64, promoID, promoName *string, discountTotal float64) {
		mustCreate(t, db, &model.RawSale{
			SalesNum: salesNum, SalesDate: d, BranchCode: "BR01",
			StatusName: strPtr("Finished"), GrandTotal: grandTotal, Raw: []byte(`{}`),
			PromotionID: promoID, PromotionName: promoName, DiscountTotal: f64ptr(discountTotal),
		})
	}

	// Baseline: avg = (100000+120000)/2 = 110000. N2 uses the '0' sentinel,
	// which must be treated the same as NULL (no promo).
	seedSale("N1", 100000, nil, nil, 0)
	seedSale("N2", 120000, strPtr("0"), nil, 0)

	// Promo A: avg bill 150000 -> lift 36.4%; ROI (300000-220000)/20000=4.0 -> Efektif.
	seedSale("A1", 150000, strPtr("PROMO-A"), strPtr("Diskon A"), 10000)
	seedSale("A2", 150000, strPtr("PROMO-A"), strPtr("Diskon A"), 10000)

	// Promo B: avg bill 115000 -> lift 4.5% (below 15) and ROI 1.0 (below 2) -> Kurang Efektif.
	seedSale("B1", 115000, strPtr("PROMO-B"), strPtr("Diskon B"), 5000)

	// Promo C: avg bill 200000 -> lift 81.8% (clears 15) but discount_cost is
	// large enough that ROI is only 0.9 (below 2) -> still Kurang Efektif,
	// proving status requires BOTH conditions, not either one.
	seedSale("C1", 200000, strPtr("PROMO-C"), strPtr("Diskon C"), 100000)

	rows, err := service.GetPromoPerformance(db, date(2026, 2, 1), date(2026, 2, 7), nil)
	if err != nil {
		t.Fatalf("GetPromoPerformance: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3 (baseline bills excluded)", len(rows))
	}

	byID := map[string]service.PromoPerformance{}
	for _, r := range rows {
		byID[r.PromotionID] = r
	}

	a := byID["PROMO-A"]
	if a.Redemptions != 2 || a.PromoRevenue != 300000 || a.DiscountCost != 20000 {
		t.Errorf("PROMO-A = %+v, want redemptions=2 promo_revenue=300000 discount_cost=20000", a)
	}
	if a.LiftPct == nil || !almostEqual(*a.LiftPct, 36.4, 0.05) {
		t.Errorf("PROMO-A lift_pct = %v, want ~36.4", a.LiftPct)
	}
	if !almostEqual(a.ROI, 4.0, 0.01) {
		t.Errorf("PROMO-A roi = %v, want 4.0", a.ROI)
	}
	if a.Status != "Efektif" {
		t.Errorf("PROMO-A status = %q, want Efektif (lift>=15 and roi>=2)", a.Status)
	}

	b := byID["PROMO-B"]
	if b.Status != "Kurang Efektif" {
		t.Errorf("PROMO-B status = %q, want Kurang Efektif (both lift and roi below threshold)", b.Status)
	}

	c := byID["PROMO-C"]
	if c.LiftPct == nil || !almostEqual(*c.LiftPct, 81.8, 0.05) {
		t.Errorf("PROMO-C lift_pct = %v, want ~81.8 (high lift)", c.LiftPct)
	}
	if !almostEqual(c.ROI, 0.9, 0.01) {
		t.Errorf("PROMO-C roi = %v, want 0.9 (low roi)", c.ROI)
	}
	if c.Status != "Kurang Efektif" {
		t.Errorf("PROMO-C status = %q, want Kurang Efektif despite high lift -- ROI alone must not be enough", c.Status)
	}
}

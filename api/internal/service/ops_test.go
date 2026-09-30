package service_test

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/service"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

func seedOpsSale(t *testing.T, db *gorm.DB, salesNum, branchCode, status string, salesDateIn, salesDateOut *time.Time, pax *int) {
	t.Helper()
	mustCreate(t, db, &model.RawSale{
		SalesNum: salesNum, SalesDate: date(2026, 2, 3), BranchCode: branchCode,
		StatusName: strPtr(status), GrandTotal: 10000, Raw: []byte(`{}`),
		SalesDateIn: salesDateIn, SalesDateOut: salesDateOut, PaxTotal: pax,
		MenuDiscountTotal: f64ptr(0), PromotionDiscount: f64ptr(0), VoucherDiscountTotal: f64ptr(0),
	})
}

// TestGetOpsSummary_StatusCounts checks that trans_count_all counts every
// status while the per-status counters only count their own status.
func TestGetOpsSummary_StatusCounts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mustCreate(t, db, &model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"})

	seedOpsSale(t, db, "SN-1", "BR01", "Finished", nil, nil, nil)
	seedOpsSale(t, db, "SN-2", "BR01", "Cancelled", nil, nil, nil)
	seedOpsSale(t, db, "SN-3", "BR01", "Void", nil, nil, nil)
	seedOpsSale(t, db, "SN-4", "BR01", "New", nil, nil, nil)

	summary, err := service.GetOpsSummary(db, date(2026, 2, 1), date(2026, 2, 7), nil)
	if err != nil {
		t.Fatalf("GetOpsSummary: %v", err)
	}
	tot := summary.Totals
	if tot.TransCountAll != 4 {
		t.Errorf("trans_count_all = %d, want 4", tot.TransCountAll)
	}
	if tot.TransCountFinished != 1 || tot.CancelledCount != 1 || tot.VoidCount != 1 || tot.NewCount != 1 {
		t.Errorf("per-status counts = %+v, want 1 each", tot)
	}
}

// TestGetOpsSummary_DwellTimeCapAndExclusions checks the 8h (28800s) dwell
// cap and that bills with missing times or out < in are excluded from both
// the sum and the sample count.
func TestGetOpsSummary_DwellTimeCapAndExclusions(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mustCreate(t, db, &model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"})

	in1 := date(2026, 2, 3).Add(10 * time.Hour)
	out1 := in1.Add(30 * time.Minute) // 1800s, under the cap
	seedOpsSale(t, db, "SN-1", "BR01", "Finished", &in1, &out1, nil)

	in2 := date(2026, 2, 3).Add(9 * time.Hour)
	out2 := in2.Add(10 * time.Hour) // 36000s, over the 28800s cap
	seedOpsSale(t, db, "SN-2", "BR01", "Finished", &in2, &out2, nil)

	// Missing sales_date_out -- excluded from both sum and sample count.
	in3 := date(2026, 2, 3).Add(11 * time.Hour)
	seedOpsSale(t, db, "SN-3", "BR01", "Finished", &in3, nil, nil)

	// out before in -- excluded.
	in4 := date(2026, 2, 3).Add(12 * time.Hour)
	out4 := in4.Add(-1 * time.Hour)
	seedOpsSale(t, db, "SN-4", "BR01", "Finished", &in4, &out4, nil)

	summary, err := service.GetOpsSummary(db, date(2026, 2, 1), date(2026, 2, 7), nil)
	if err != nil {
		t.Fatalf("GetOpsSummary: %v", err)
	}
	if summary.Totals.DwellSampleCount != 2 {
		t.Fatalf("dwell_sample_count = %d, want 2 (SN-1, SN-2 only)", summary.Totals.DwellSampleCount)
	}
	want := 1800.0 + 28800.0 // SN-2 capped at 28800, not the raw 36000
	if summary.Totals.DwellSecondsSum != want {
		t.Errorf("dwell_seconds_sum = %v, want %v (SN-2 capped at 8h)", summary.Totals.DwellSecondsSum, want)
	}
}

// TestGetOpsSummary_ChannelNormalization checks the visit_purpose_name ->
// channel mapping: case-insensitive prefix/substring matches, null ->
// "Tidak Diketahui", and anything unrecognized passes through unchanged.
func TestGetOpsSummary_ChannelNormalization(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mustCreate(t, db, &model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"})

	seed := func(salesNum string, purpose *string) {
		mustCreate(t, db, &model.RawSale{
			SalesNum: salesNum, SalesDate: date(2026, 2, 3), BranchCode: "BR01",
			StatusName: strPtr("Finished"), GrandTotal: 10000, Raw: []byte(`{}`),
			VisitPurposeName: purpose,
		})
	}
	seed("SN-1", strPtr("DINE IN - AC"))
	seed("SN-2", strPtr("Grabfood Pick Up"))
	seed("SN-3", strPtr("Gojek Delivery"))
	seed("SN-4", nil)
	seed("SN-5", strPtr("Catering"))

	summary, err := service.GetOpsSummary(db, date(2026, 2, 1), date(2026, 2, 7), nil)
	if err != nil {
		t.Fatalf("GetOpsSummary: %v", err)
	}
	got := map[string]int64{}
	for _, c := range summary.ByChannel {
		got[c.Channel] = c.TransCount
	}
	want := map[string]int64{"Dine In": 1, "Pick Up": 1, "Delivery": 1, "Tidak Diketahui": 1, "Catering": 1}
	for channel, count := range want {
		if got[channel] != count {
			t.Errorf("channel %q trans_count = %d, want %d (got map: %v)", channel, got[channel], count, got)
		}
	}
}

// TestGetOpsSummary_PaymentMethodFromPayments checks that the breakdown
// reads raw_sales_payments.payment_amount (not grand_total), so a bill split
// across two payment methods counts once per payment row, and null method
// names become "Tidak Diketahui".
func TestGetOpsSummary_PaymentMethodFromPayments(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mustCreate(t, db, &model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"})

	mustCreate(t, db, &model.RawSale{
		SalesNum: "SN-1", SalesDate: date(2026, 2, 3), BranchCode: "BR01",
		StatusName: strPtr("Finished"), GrandTotal: 30000, Raw: []byte(`{}`),
	})
	// Split payment: 20000 cash + 10000 QRIS.
	mustCreate(t, db, &model.RawSalesPayment{
		SalesNum: "SN-1", SalesPaymentBackendID: "P1",
		PaymentMethodTypeName: strPtr("Tunai"), PaymentAmount: f64ptr(20000),
	})
	mustCreate(t, db, &model.RawSalesPayment{
		SalesNum: "SN-1", SalesPaymentBackendID: "P2",
		PaymentMethodTypeName: strPtr("QRIS"), PaymentAmount: f64ptr(10000),
	})

	mustCreate(t, db, &model.RawSale{
		SalesNum: "SN-2", SalesDate: date(2026, 2, 3), BranchCode: "BR01",
		StatusName: strPtr("Finished"), GrandTotal: 5000, Raw: []byte(`{}`),
	})
	mustCreate(t, db, &model.RawSalesPayment{
		SalesNum: "SN-2", SalesPaymentBackendID: "P1",
		PaymentMethodTypeName: nil, PaymentAmount: f64ptr(5000),
	})

	summary, err := service.GetOpsSummary(db, date(2026, 2, 1), date(2026, 2, 7), nil)
	if err != nil {
		t.Fatalf("GetOpsSummary: %v", err)
	}
	got := map[string]float64{}
	count := map[string]int64{}
	for _, p := range summary.ByPaymentMethod {
		got[p.PaymentMethodTypeName] = p.PaymentAmount
		count[p.PaymentMethodTypeName] = p.PaymentCount
	}
	if got["Tunai"] != 20000 || got["QRIS"] != 10000 || got["Tidak Diketahui"] != 5000 {
		t.Errorf("payment amounts = %v, want Tunai=20000, QRIS=10000, Tidak Diketahui=5000", got)
	}
	if count["Tunai"] != 1 || count["QRIS"] != 1 {
		t.Errorf("payment counts = %v, want 1 each (split payment counts per row, not per bill)", count)
	}
}

package service_test

import (
	"testing"

	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/service"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

// seedSalesAndNonSales: one ordinary sale (cash, 100.000) and one non sales
// bill (payment type 7 "Compliment", 40.000) for the same member, outlet and
// day, each with one menu line.
func seedSalesAndNonSales(t *testing.T, db *gorm.DB) {
	t.Helper()
	day := date(2026, 2, 3)
	mustCreate(t, db, &model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"})
	for _, b := range []struct {
		num, menu, payType, payName string
		total                       float64
		nonSales                    bool
	}{
		{"SN-SALES", "Nasi Goreng", "1", "Cash", 100000, false},
		{"SN-NONSALES", "Kopi Tamu", "7", "Compliment", 40000, true},
	} {
		mustCreate(t, db, &model.RawSale{
			SalesNum: b.num, BillNum: strPtr("B-" + b.num), SalesDate: day, BranchCode: "BR01",
			StatusName: strPtr("Finished"), GrandTotal: b.total, IsNonSales: b.nonSales, Raw: []byte(`{}`),
			Subtotal: f64ptr(b.total), OtherTaxTotal: f64ptr(0), VatTotal: f64ptr(0), OtherVatTotal: f64ptr(0),
			DiscountTotal: f64ptr(0), RoundingTotal: f64ptr(0),
			MemberCode: strPtr("M1"), MemberName: strPtr("Budi"),
		})
		mustCreate(t, db, &model.RawSalesMenuItem{
			SalesNum: b.num, LineSeq: 0, MenuID: b.menu, MenuName: strPtr(b.menu), SalesDate: day, BranchCode: "BR01",
			MenuCategoryID: strPtr("C1"), MenuCategoryName: strPtr("Makanan"), Qty: 2, Total: b.total,
		})
		mustCreate(t, db, &model.RawSalesPayment{
			SalesNum: b.num, SalesPaymentBackendID: "P-" + b.num,
			PaymentMethodTypeID: strPtr(b.payType), PaymentMethodTypeName: strPtr(b.payName),
			PaymentMethodName: strPtr(b.payName), PaymentAmount: f64ptr(b.total),
		})
	}
}

func TestNonSales_AreExcludedFromEverySalesMetric(t *testing.T) {
	db := testutil.SetupTestDB(t)
	seedSalesAndNonSales(t, db)
	from, to := date(2026, 2, 1), date(2026, 2, 7)

	sum, err := service.GetSalesSummary(db, from, to, nil)
	if err != nil || sum.Revenue != 100000 || sum.TransCount != 1 || sum.MemberRevenue != 100000 || sum.NettSales != 100000 {
		t.Errorf("sales summary = %+v, %v; want only the 100.000 sale", sum, err)
	}
	daily, _ := service.GetSalesDaily(db, from, to, nil)
	if len(daily) != 1 || daily[0].Revenue != 100000 || daily[0].TransCount != 1 {
		t.Errorf("sales daily = %+v", daily)
	}
	byOutlet, _ := service.GetRevenueByOutlet(db, from, to)
	if len(byOutlet) != 1 || byOutlet[0].Revenue != 100000 || byOutlet[0].TransCount != 1 {
		t.Errorf("revenue by outlet = %+v", byOutlet)
	}
	cat, _ := service.GetRevenueByCategory(db, from, to, nil, nil, nil)
	if cat.Revenue != 100000 {
		t.Errorf("revenue by category = %+v", cat)
	}
	top, _ := service.GetTopProducts(db, from, to, nil, nil, nil, "qty", true, 0)
	if len(top) != 1 || top[0].MenuID != "Nasi Goreng" {
		t.Errorf("top products = %+v, want only the sales menu", top)
	}
	perf, _ := service.GetMenuPerformance(db, from, to, nil, nil, nil, 300)
	if len(perf) != 1 || perf[0].MenuID != "Nasi Goreng" {
		t.Errorf("menu performance = %+v", perf)
	}
	bills, total, _ := service.GetSalesBills(db, from, to, nil, 1, 50)
	if total != 1 || len(bills) != 1 || *bills[0].BillNum != "B-SN-SALES" {
		t.Errorf("sales bills = %+v (total %d)", bills, total)
	}
	export, _ := service.GetAllSalesBills(db, from, to, nil)
	if len(export) != 1 {
		t.Errorf("sales export = %+v", export)
	}

	ops, _ := service.GetOpsSummary(db, from, to, nil)
	if ops.Totals.TransCountAll != 1 || ops.Totals.TransCountFinished != 1 {
		t.Errorf("ops totals = %+v", ops.Totals)
	}
	for _, p := range ops.ByPaymentMethod {
		if p.PaymentMethodTypeName == "Compliment" {
			t.Errorf("ops payment breakdown includes the non sales payment: %+v", ops.ByPaymentMethod)
		}
	}
	var channelRevenue float64
	for _, c := range ops.ByChannel {
		channelRevenue += c.Revenue
	}
	if channelRevenue != 100000 {
		t.Errorf("ops channel revenue = %v, want 100000", channelRevenue)
	}

	members, _ := service.GetTopMembers(db, from, to, nil, 10)
	if len(members) != 1 || members[0].Spending != 100000 || members[0].Visits != 1 {
		t.Errorf("top members = %+v, want 1 visit / 100.000 (non sales visit excluded)", members)
	}
	purchases, _ := service.GetMemberMenuPurchases(db, "M1", from, to, nil)
	if len(purchases) != 1 || purchases[0].MenuID != "Nasi Goreng" {
		t.Errorf("member menu purchases = %+v", purchases)
	}
}

func TestNonSales_Segment(t *testing.T) {
	db := testutil.SetupTestDB(t)
	seedSalesAndNonSales(t, db)
	from, to := date(2026, 2, 1), date(2026, 2, 7)

	s, err := service.GetNonSalesSummary(db, from, to, nil)
	if err != nil || s.Value != 40000 || s.TransCount != 1 || s.AvgValue != 40000 || s.OutletCount != 1 {
		t.Errorf("non sales summary = %+v, %v", s, err)
	}
	daily, _ := service.GetNonSalesDaily(db, from, to, nil)
	if len(daily) != 1 || daily[0].Value != 40000 {
		t.Errorf("non sales daily = %+v", daily)
	}
	outlets, _ := service.GetNonSalesByOutlet(db, from, to)
	if len(outlets) != 1 || outlets[0].BranchName != "Cabang A" || outlets[0].Value != 40000 {
		t.Errorf("non sales by outlet = %+v", outlets)
	}
	menus, _ := service.GetNonSalesTopMenus(db, from, to, nil, 10)
	if len(menus) != 1 || menus[0].MenuID != "Kopi Tamu" || menus[0].Qty != 2 {
		t.Errorf("non sales top menus = %+v", menus)
	}
	bills, total, _ := service.GetNonSalesBills(db, from, to, nil, 1, 50)
	if total != 1 || len(bills) != 1 || *bills[0].BillNum != "B-SN-NONSALES" ||
		bills[0].PaymentMethod == nil || *bills[0].PaymentMethod != "Compliment" {
		t.Errorf("non sales bills = %+v (total %d)", bills, total)
	}

	// Outlet filter and empty range behave.
	other := "BR99"
	if s, _ := service.GetNonSalesSummary(db, from, to, &other); s.TransCount != 0 || s.Value != 0 {
		t.Errorf("filtered to another outlet = %+v, want zeros", s)
	}
	if rows, _ := service.GetNonSalesDaily(db, date(2026, 3, 1), date(2026, 3, 2), nil); rows == nil || len(rows) != 0 {
		t.Errorf("empty range should give an empty (non-nil) list, got %#v", rows)
	}
}

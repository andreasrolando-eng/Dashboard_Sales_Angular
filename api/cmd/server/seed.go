package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/Operations-ESB/dashboard-sales/api/internal/config"
	"github.com/Operations-ESB/dashboard-sales/api/internal/db"
	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
)

// runSeed fills the local dev database with deterministic, realistic-looking
// sample data (outlets, members, 45 days of sales/menu-items/payments across
// 3 outlets) so the M2 API endpoints have something to show without needing
// the real ESB ETL (M3) wired up yet. Always truncates the raw_* tables and
// outlets first -- this is dev-only seed data, never run against production.
func runSeed(cfg config.Config) error {
	gormDB, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return err
	}

	if err := gormDB.Exec(`truncate raw_sales_payments, raw_sales_menu_items, raw_sales, raw_members, outlets, sync_logs cascade`).Error; err != nil {
		return fmt.Errorf("truncate before seed: %w", err)
	}

	outlets := []model.Outlet{
		{BranchCode: "BR01", BranchName: "Jakarta Pusat"},
		{BranchCode: "BR02", BranchName: "Bandung"},
		{BranchCode: "BR03", BranchName: "Surabaya"},
	}
	if err := gormDB.Create(&outlets).Error; err != nil {
		return fmt.Errorf("seed outlets: %w", err)
	}

	type menuDef struct {
		id, name, categoryID, categoryName, detailID, detailName string
		price                                                    float64
	}
	menus := []menuDef{
		{"M1", "Nasi Goreng", "CAT-FOOD", "Makanan", "DET-NASI", "Nasi", 35000},
		{"M2", "Nasi Ayam Bakar", "CAT-FOOD", "Makanan", "DET-NASI", "Nasi", 40000},
		{"M3", "Mie Ayam", "CAT-FOOD", "Makanan", "DET-MIE", "Mie", 30000},
		{"M4", "Mie Goreng", "CAT-FOOD", "Makanan", "DET-MIE", "Mie", 32000},
		{"M5", "Es Kopi Susu", "CAT-DRINK", "Minuman", "DET-KOPI", "Kopi", 22000},
		{"M6", "Cappuccino", "CAT-DRINK", "Minuman", "DET-KOPI", "Kopi", 28000},
		{"M7", "Es Teh Manis", "CAT-DRINK", "Minuman", "DET-TEH", "Teh", 12000},
		{"M8", "Jus Alpukat", "CAT-DRINK", "Minuman", "DET-JUS", "Jus", 20000},
	}

	members := make([]model.RawMember, 15)
	for i := range members {
		members[i] = model.RawMember{
			MemberCode: fmt.Sprintf("MBR-%02d", i+1),
			MemberName: strp(fmt.Sprintf("Member %02d", i+1)),
		}
	}
	if err := gormDB.Create(&members).Error; err != nil {
		return fmt.Errorf("seed members: %w", err)
	}

	channels := []string{"Dine In", "Pick Up", "Delivery"}
	paymentMethods := []struct{ id, name string }{
		{"CASH", "Tunai"}, {"QRIS", "QRIS"}, {"DEBIT", "Kartu Debit"}, {"CREDIT", "Kartu Kredit"},
	}
	promos := []struct{ id, name string }{
		{"PROMO1", "Diskon Akhir Pekan 20%"}, {"PROMO2", "Buy 1 Get 1 Kopi"},
	}
	statuses := []struct {
		name   string
		weight int
	}{
		{"Finished", 85}, {"Cancelled", 5}, {"Void", 5}, {"New", 5},
	}

	rng := rand.New(rand.NewSource(42))
	pickStatus := func() string {
		n := rng.Intn(100)
		acc := 0
		for _, s := range statuses {
			acc += s.weight
			if n < acc {
				return s.name
			}
		}
		return "Finished"
	}

	today := time.Now().Truncate(24 * time.Hour)
	const days = 45

	var sales []model.RawSale
	var items []model.RawSalesMenuItem
	var payments []model.RawSalesPayment
	salesCount := 0

	for d := 0; d < days; d++ {
		salesDate := today.AddDate(0, 0, -d)
		for _, o := range outlets {
			txCount := 15 + rng.Intn(26) // 15-40 transactions/day/outlet
			for n := 0; n < txCount; n++ {
				salesCount++
				salesNum := fmt.Sprintf("SEED-%s-%s-%03d", o.BranchCode, salesDate.Format("20060102"), n)
				status := pickStatus()

				lineCount := 1 + rng.Intn(3) // 1-3 items
				var subtotal float64
				lineItems := make([]model.RawSalesMenuItem, 0, lineCount)
				for seq := 1; seq <= lineCount; seq++ {
					m := menus[rng.Intn(len(menus))]
					qty := float64(1 + rng.Intn(3))
					total := m.price * qty
					subtotal += total
					lineItems = append(lineItems, model.RawSalesMenuItem{
						SalesNum: salesNum, LineSeq: seq, MenuID: m.id, MenuName: strp(m.name),
						SalesDate: salesDate, BranchCode: o.BranchCode,
						MenuCategoryID: strp(m.categoryID), MenuCategoryName: strp(m.categoryName),
						MenuCategoryDetailID: strp(m.detailID), MenuCategoryDetailName: strp(m.detailName),
						Qty: qty, Price: f64p(m.price), Total: total,
					})
				}

				menuDiscount := float64(rng.Intn(3000))
				var promoDiscount float64
				var promoID, promoName *string
				if status == "Finished" && rng.Intn(100) < 15 {
					p := promos[rng.Intn(len(promos))]
					promoID, promoName = strp(p.id), strp(p.name)
					promoDiscount = 5000 + float64(rng.Intn(10000))
				}
				discountTotal := menuDiscount + promoDiscount
				grandTotal := subtotal - discountTotal
				if grandTotal < 0 {
					grandTotal = subtotal
					discountTotal = 0
					menuDiscount = 0
					promoDiscount = 0
				}

				var memberCode, memberName *string
				if status == "Finished" && rng.Intn(100) < 30 {
					mb := members[rng.Intn(len(members))]
					memberCode, memberName = strp(mb.MemberCode), mb.MemberName
				}

				hourIn := 8 + rng.Intn(14) // 08:00-21:59
				salesDateIn := salesDate.Add(time.Duration(hourIn)*time.Hour + time.Duration(rng.Intn(60))*time.Minute)
				dwellMin := 10 + rng.Intn(80)
				salesDateOut := salesDateIn.Add(time.Duration(dwellMin) * time.Minute)

				channel := channels[rng.Intn(len(channels))]
				pax := 1 + rng.Intn(4)

				sales = append(sales, model.RawSale{
					SalesNum: salesNum, BillNum: strp(salesNum), SalesDate: salesDate,
					SalesDateIn: &salesDateIn, SalesDateOut: &salesDateOut,
					BranchCode: o.BranchCode, BranchName: strp(o.BranchName),
					MemberCode: memberCode, MemberName: memberName,
					VisitPurposeName: strp(channel), PaxTotal: &pax,
					Subtotal: f64p(subtotal), DiscountTotal: f64p(discountTotal),
					MenuDiscountTotal: f64p(menuDiscount), PromotionDiscount: f64p(promoDiscount),
					VoucherDiscountTotal: f64p(0), OtherTaxTotal: f64p(0), VatTotal: f64p(0),
					OtherVatTotal: f64p(0), RoundingTotal: f64p(0),
					GrandTotal:  grandTotal,
					PromotionID: promoID, PromotionName: promoName,
					StatusID: strp(status), StatusName: strp(status),
					Raw:      []byte(`{}`),
					SyncedAt: time.Now(),
				})
				items = append(items, lineItems...)

				payMethod := paymentMethods[rng.Intn(len(paymentMethods))]
				payments = append(payments, model.RawSalesPayment{
					SalesNum: salesNum, SalesPaymentBackendID: salesNum + "-P1",
					PaymentMethodTypeID: strp(payMethod.id), PaymentMethodTypeName: strp(payMethod.name),
					PaymentAmount: f64p(grandTotal), FullPaymentAmount: f64p(grandTotal),
				})
			}
		}
	}

	if err := gormDB.CreateInBatches(&sales, 500).Error; err != nil {
		return fmt.Errorf("seed sales: %w", err)
	}
	if err := gormDB.CreateInBatches(&items, 500).Error; err != nil {
		return fmt.Errorf("seed menu items: %w", err)
	}
	if err := gormDB.CreateInBatches(&payments, 500).Error; err != nil {
		return fmt.Errorf("seed payments: %w", err)
	}

	finishedAt := time.Now()
	if err := gormDB.Create(&model.SyncLog{
		JobName: "seed", Status: "success", StartedAt: finishedAt, FinishedAt: &finishedAt,
		RowsSynced: &salesCount,
	}).Error; err != nil {
		return fmt.Errorf("seed sync log: %w", err)
	}

	fmt.Printf("seeded %d outlets, %d members, %d sales (%d menu items, %d payments) across %d days ending %s\n",
		len(outlets), len(members), salesCount, len(items), len(payments), days, today.Format("2006-01-02"))
	return nil
}

func strp(s string) *string   { return &s }
func f64p(v float64) *float64 { return &v }

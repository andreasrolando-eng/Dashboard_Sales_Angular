package service_test

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/service"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

func TestListOutlets(t *testing.T) {
	db := testutil.SetupTestDB(t)

	if err := db.Create(&model.Outlet{BranchCode: "BR02", BranchName: "Cabang B"}).Error; err != nil {
		t.Fatalf("seed BR02: %v", err)
	}
	if err := db.Create(&model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"}).Error; err != nil {
		t.Fatalf("seed BR01: %v", err)
	}

	outlets, err := service.ListOutlets(db)
	if err != nil {
		t.Fatalf("ListOutlets: %v", err)
	}
	if len(outlets) != 2 {
		t.Fatalf("len(outlets) = %d, want 2", len(outlets))
	}
	// Ordered by branch_name, so "Cabang A" comes before "Cabang B".
	if outlets[0].BranchCode != "BR01" || outlets[1].BranchCode != "BR02" {
		t.Errorf("outlets = %+v, want BR01 then BR02", outlets)
	}
}

// seedFinishedSaleWithMenuItem inserts one Finished raw_sales row and one
// raw_sales_menu_items row on the given date, category/category-detail --
// the minimum fixture ListCategories/ListCategoryDetails need, since both
// derive purely from menu line items on Finished sales.
func seedFinishedSaleWithMenuItem(t *testing.T, db *gorm.DB, salesNum, branchCode string, salesDate time.Time, categoryID, categoryName, detailID, detailName string) {
	t.Helper()
	sale := model.RawSale{
		SalesNum:   salesNum,
		SalesDate:  salesDate,
		BranchCode: branchCode,
		StatusName: strPtr("Finished"),
		GrandTotal: 10000,
		Raw:        []byte(`{}`),
	}
	if err := db.Create(&sale).Error; err != nil {
		t.Fatalf("seed sale %s: %v", salesNum, err)
	}
	item := model.RawSalesMenuItem{
		SalesNum:               salesNum,
		LineSeq:                1,
		MenuID:                 "MENU-1",
		SalesDate:              salesDate,
		BranchCode:             branchCode,
		MenuCategoryID:         strPtr(categoryID),
		MenuCategoryName:       strPtr(categoryName),
		MenuCategoryDetailID:   strPtr(detailID),
		MenuCategoryDetailName: strPtr(detailName),
		Qty:                    1,
		Total:                  10000,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("seed menu item %s: %v", salesNum, err)
	}
}

func strPtr(s string) *string { return &s }

func TestListCategories(t *testing.T) {
	db := testutil.SetupTestDB(t)

	if err := db.Create(&model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"}).Error; err != nil {
		t.Fatalf("seed outlet: %v", err)
	}

	// Same category_id, two different names on two different dates -- the
	// most recent sales_date's name should win (ported v_categories rule).
	seedFinishedSaleWithMenuItem(t, db, "SN-1", "BR01", date(2026, 1, 1), "CAT-1", "Makanan (lama)", "DET-1", "Nasi (lama)")
	seedFinishedSaleWithMenuItem(t, db, "SN-2", "BR01", date(2026, 1, 5), "CAT-1", "Makanan", "DET-1", "Nasi")

	categories, err := service.ListCategories(db)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	if len(categories) != 1 {
		t.Fatalf("len(categories) = %d, want 1", len(categories))
	}
	if categories[0].CategoryName != "Makanan" {
		t.Errorf("category_name = %q, want %q (latest sales_date should win)", categories[0].CategoryName, "Makanan")
	}
}

func TestListCategoryDetails(t *testing.T) {
	db := testutil.SetupTestDB(t)

	if err := db.Create(&model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"}).Error; err != nil {
		t.Fatalf("seed outlet: %v", err)
	}
	seedFinishedSaleWithMenuItem(t, db, "SN-1", "BR01", date(2026, 1, 1), "CAT-1", "Makanan", "DET-1", "Nasi")

	details, err := service.ListCategoryDetails(db)
	if err != nil {
		t.Fatalf("ListCategoryDetails: %v", err)
	}
	if len(details) != 1 {
		t.Fatalf("len(details) = %d, want 1", len(details))
	}
	if details[0].CategoryID != "CAT-1" {
		t.Errorf("category_id = %q, want %q", details[0].CategoryID, "CAT-1")
	}
}

func TestGetLastSync_NoneYet(t *testing.T) {
	db := testutil.SetupTestDB(t)

	sync, err := service.GetLastSync(db)
	if err != nil {
		t.Fatalf("GetLastSync: %v", err)
	}
	if sync != nil {
		t.Errorf("sync = %+v, want nil (no successful sync yet)", sync)
	}
}

func TestGetLastSync_ReturnsLatestSuccess(t *testing.T) {
	db := testutil.SetupTestDB(t)

	oldFinished := date(2026, 1, 1)
	newFinished := date(2026, 1, 5)
	if err := db.Create(&model.SyncLog{
		JobName: "sync-esb", Status: "success", StartedAt: oldFinished, FinishedAt: &oldFinished,
	}).Error; err != nil {
		t.Fatalf("seed old sync: %v", err)
	}
	if err := db.Create(&model.SyncLog{
		JobName: "sync-esb", Status: "success", StartedAt: newFinished, FinishedAt: &newFinished,
	}).Error; err != nil {
		t.Fatalf("seed new sync: %v", err)
	}

	sync, err := service.GetLastSync(db)
	if err != nil {
		t.Fatalf("GetLastSync: %v", err)
	}
	if sync == nil {
		t.Fatal("sync = nil, want the most recent successful run")
	}
	if sync.FinishedAt == nil || !sync.FinishedAt.Equal(newFinished) {
		t.Errorf("finished_at = %v, want the newer run (%v)", sync.FinishedAt, newFinished)
	}
}

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestGetLastSync_IgnoresManualSyncs(t *testing.T) {
	db := testutil.SetupTestDB(t)
	nightly := time.Date(2026, 2, 3, 6, 0, 0, 0, time.UTC)
	manual := nightly.Add(48 * time.Hour) // later, but must not win

	rows := 5
	for _, l := range []model.SyncLog{
		{JobName: "sync-esb", Status: "success", StartedAt: nightly, FinishedAt: &nightly, RowsSynced: &rows},
		{JobName: "sync-esb-manual", Status: "success", StartedAt: manual, FinishedAt: &manual, RowsSynced: &rows},
	} {
		if err := db.Create(&l).Error; err != nil {
			t.Fatal(err)
		}
	}

	got, err := service.GetLastSync(db)
	if err != nil || got == nil {
		t.Fatalf("GetLastSync = %v, %v", got, err)
	}
	if got.JobName != "sync-esb" || !got.FinishedAt.Equal(nightly) {
		t.Errorf("last sync = %+v, want the nightly run (a manual backfill is not 'fresh data')", got)
	}
}

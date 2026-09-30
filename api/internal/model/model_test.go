package model_test

import (
	"testing"
	"time"

	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

var setupTestDB = testutil.SetupTestDB

func TestUserRoundTrip(t *testing.T) {
	db := setupTestDB(t)

	u := model.User{Email: "test.user@esb.co.id", Name: "Test User"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var got model.User
	if err := db.First(&got, u.ID).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if got.Email != u.Email {
		t.Errorf("email = %q, want %q", got.Email, u.Email)
	}
}

func TestSeededAdminExists(t *testing.T) {
	db := setupTestDB(t)

	var admin model.User
	if err := db.Where("email = ?", "andreas.rolando@esb.co.id").First(&admin).Error; err != nil {
		t.Fatalf("seeded admin not found: %v", err)
	}
	if !admin.IsAdmin {
		t.Errorf("seeded admin IsAdmin = false, want true")
	}
}

func TestOutletRoundTrip(t *testing.T) {
	db := setupTestDB(t)

	o := model.Outlet{BranchCode: "BR01", BranchName: "Outlet Satu"}
	if err := db.Create(&o).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var got model.Outlet
	if err := db.First(&got, "branch_code = ?", "BR01").Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if got.BranchName != "Outlet Satu" {
		t.Errorf("branch_name = %q, want %q", got.BranchName, "Outlet Satu")
	}
}

func TestRawSaleRoundTrip(t *testing.T) {
	db := setupTestDB(t)

	if err := db.Create(&model.Outlet{BranchCode: "BR01", BranchName: "Outlet Satu"}).Error; err != nil {
		t.Fatalf("create outlet: %v", err)
	}

	s := model.RawSale{
		SalesNum:   "SN-001",
		SalesDate:  time.Now(),
		BranchCode: "BR01",
		GrandTotal: 150000,
		Raw:        []byte(`{}`),
	}
	if err := db.Create(&s).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var got model.RawSale
	if err := db.First(&got, "sales_num = ?", "SN-001").Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if got.GrandTotal != 150000 {
		t.Errorf("grand_total = %v, want 150000", got.GrandTotal)
	}
}

func TestRawSalesPaymentRoundTrip(t *testing.T) {
	db := setupTestDB(t)

	if err := db.Create(&model.Outlet{BranchCode: "BR01", BranchName: "Outlet Satu"}).Error; err != nil {
		t.Fatalf("create outlet: %v", err)
	}
	if err := db.Create(&model.RawSale{
		SalesNum: "SN-001", SalesDate: time.Now(), BranchCode: "BR01", GrandTotal: 150000, Raw: []byte(`{}`),
	}).Error; err != nil {
		t.Fatalf("create sale: %v", err)
	}

	p := model.RawSalesPayment{SalesNum: "SN-001", SalesPaymentBackendID: "PAY-1"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var got model.RawSalesPayment
	if err := db.First(&got, "sales_num = ? AND sales_payment_backend_id = ?", "SN-001", "PAY-1").Error; err != nil {
		t.Fatalf("first: %v", err)
	}
}

func TestRawSalesMenuItemRoundTrip(t *testing.T) {
	db := setupTestDB(t)

	if err := db.Create(&model.Outlet{BranchCode: "BR01", BranchName: "Outlet Satu"}).Error; err != nil {
		t.Fatalf("create outlet: %v", err)
	}
	if err := db.Create(&model.RawSale{
		SalesNum: "SN-001", SalesDate: time.Now(), BranchCode: "BR01", GrandTotal: 150000, Raw: []byte(`{}`),
	}).Error; err != nil {
		t.Fatalf("create sale: %v", err)
	}

	item := model.RawSalesMenuItem{
		SalesNum:   "SN-001",
		LineSeq:    1,
		MenuID:     "MENU-1",
		SalesDate:  time.Now(),
		BranchCode: "BR01",
		Qty:        2,
		Total:      50000,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var got model.RawSalesMenuItem
	if err := db.First(&got, "sales_num = ? AND line_seq = ?", "SN-001", 1).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if got.Total != 50000 {
		t.Errorf("total = %v, want 50000", got.Total)
	}
}

func TestRawMemberRoundTrip(t *testing.T) {
	db := setupTestDB(t)

	m := model.RawMember{MemberCode: "MC-001"}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var got model.RawMember
	if err := db.First(&got, "member_code = ?", "MC-001").Error; err != nil {
		t.Fatalf("first: %v", err)
	}
}

func TestSyncLogRoundTrip(t *testing.T) {
	db := setupTestDB(t)

	l := model.SyncLog{JobName: "sync-esb", Status: "running", StartedAt: time.Now()}
	if err := db.Create(&l).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var got model.SyncLog
	if err := db.First(&got, l.ID).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if got.JobName != "sync-esb" {
		t.Errorf("job_name = %q, want %q", got.JobName, "sync-esb")
	}
}

package etl_test

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
)

var fixtureDate = time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC)

func mustCreateOutlet(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Create(&model.Outlet{BranchCode: "BR01", BranchName: "Cabang A"}).Error; err != nil {
		t.Fatalf("seed outlet: %v", err)
	}
}

func saleFixture(salesNum string) model.RawSale {
	return model.RawSale{
		SalesNum: salesNum, SalesDate: fixtureDate, BranchCode: "BR01",
		GrandTotal: 100000, Raw: []byte(`{}`), SyncedAt: time.Now(),
	}
}

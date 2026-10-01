package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/Operations-ESB/dashboard-sales/api/internal/handler"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

func TestSalesBillsExport_XLSX(t *testing.T) {
	db := testutil.SetupTestDB(t)
	for _, q := range []string{
		`insert into outlets (branch_code, branch_name) values ('BR01', 'Outlet Satu')`,
		`insert into raw_sales (sales_num, bill_num, sales_date, branch_code, grand_total, status_name, raw)
			values ('S1', 'B-001', '2026-02-02', 'BR01', 150000, 'Finished', '{}'),
			       ('S2', 'B-002', '2026-02-01', 'BR01', 50000, 'Finished', '{}'),
			       ('S3', 'B-003', '2026-02-01', 'BR01', 999999, 'Void', '{}')`,
	} {
		if err := db.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}

	rec := httptest.NewRecorder()
	handler.SalesBillsExport(db).ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/sales/bills/export?dateStart=2026-02-01&dateEnd=2026-02-02&format=xlsx", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "spreadsheetml") {
		t.Errorf("Content-Type = %q, want xlsx", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "bill-sales_2026-02-01_2026-02-02.xlsx") {
		t.Errorf("Content-Disposition = %q", cd)
	}

	f, err := excelize.OpenReader(rec.Body)
	if err != nil {
		t.Fatalf("not a valid xlsx: %v", err)
	}
	rows, err := f.GetRows("Bill", excelize.Options{RawCellValue: true})
	if err != nil {
		t.Fatal(err)
	}
	// Header + 2 Finished bills (Void excluded, oldest first) + total row.
	if len(rows) != 4 {
		t.Fatalf("rows = %v, want header, 2 bills, total", rows)
	}
	if rows[1][0] != "B-002" || rows[1][3] != "Outlet Satu" || rows[2][0] != "B-001" {
		t.Errorf("bill rows = %v", rows[1:3])
	}
	if got := rows[3][4]; got != "200000" {
		t.Errorf("total = %q, want 200000 (a number cell, not text)", got)
	}
	if typ, _ := f.GetCellType("Bill", "B2"); typ == excelize.CellTypeSharedString || typ == excelize.CellTypeInlineString {
		t.Errorf("date cell is stored as text (%v), want a date/number", typ)
	}
}

func TestSalesBillsExport_DefaultsToJSON(t *testing.T) {
	db := testutil.SetupTestDB(t)
	rec := httptest.NewRecorder()
	handler.SalesBillsExport(db).ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/sales/bills/export?dateStart=2026-02-01&dateEnd=2026-02-02", nil))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Errorf("status=%d content-type=%q, want JSON as before", rec.Code, rec.Header().Get("Content-Type"))
	}
}

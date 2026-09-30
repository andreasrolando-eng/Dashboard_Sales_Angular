package model_test

import (
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/Operations-ESB/dashboard-sales/api/migrations"
)

// The non sales migration must backfill bills that were synced BEFORE the flag
// existed, from their stored payments.
func TestNonSalesMigration_BackfillsExistingBills(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	u, _ := url.Parse(base)
	admin := *u
	admin.Path = "/postgres"
	adminDB, err := sql.Open("pgx", admin.String())
	if err != nil {
		t.Skip(err)
	}
	defer adminDB.Close()
	name := "apptest_ns_" + strings.ReplaceAll(time.Now().Format("150405.000000"), ".", "")
	if _, err := adminDB.Exec("create database " + name); err != nil {
		t.Fatal(err)
	}
	defer adminDB.Exec("drop database if exists " + name + " with (force)")

	u.Path = "/" + name
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpTo(db, ".", 20260930100001); err != nil { // everything before the non sales migration
		t.Fatal(err)
	}
	for _, q := range []string{
		`insert into outlets (branch_code, branch_name) values ('BR01', 'A')`,
		`insert into raw_sales (sales_num, sales_date, branch_code, grand_total, raw, synced_at) values
			('SN-CASH', '2026-02-03', 'BR01', 100, '{}', now()),
			('SN-NS',   '2026-02-03', 'BR01',  40, '{}', now()),
			('SN-NOPAY','2026-02-03', 'BR01',   0, '{}', now())`,
		`insert into raw_sales_payments (sales_num, sales_payment_backend_id, payment_method_type_id) values
			('SN-CASH', '1', '1'), ('SN-NS', '2', '7')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := goose.Up(db, "."); err != nil {
		t.Fatal(err)
	}

	rows, err := db.Query(`select sales_num, is_non_sales from raw_sales order by sales_num`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]bool{}
	for rows.Next() {
		var num string
		var ns bool
		if err := rows.Scan(&num, &ns); err != nil {
			t.Fatal(err)
		}
		got[num] = ns
	}
	if !got["SN-NS"] || got["SN-CASH"] || got["SN-NOPAY"] {
		t.Errorf("backfill = %v, want only SN-NS flagged", got)
	}
}

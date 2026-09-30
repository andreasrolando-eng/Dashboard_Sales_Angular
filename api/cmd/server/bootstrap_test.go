package main

import (
	"strings"
	"testing"
	"time"

	"github.com/Operations-ESB/dashboard-sales/api/internal/auth"
	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

func TestBootstrapAdmin(t *testing.T) {
	db := testutil.SetupTestDB(t)
	svc := auth.NewService(db, time.Hour)
	canLogin := func(email, pw string) bool {
		_, _, err := svc.Login(email, pw, "", "")
		return err == nil
	}

	// The seeded admin (migration 1) exists without a password: bootstrap sets it.
	if _, err := bootstrapAdmin(db, " Andreas.Rolando@esb.co.id ", "demo-pertama-1"); err != nil {
		t.Fatal(err)
	}
	if !canLogin("andreas.rolando@esb.co.id", "demo-pertama-1") {
		t.Fatal("seeded admin should be able to log in after bootstrap")
	}

	// Restart with different variables must NOT overwrite the existing password.
	msg, err := bootstrapAdmin(db, "andreas.rolando@esb.co.id", "percobaan-timpa-2")
	if err != nil || !strings.Contains(msg, "diabaikan") {
		t.Fatalf("second run = %q, %v; want it ignored", msg, err)
	}
	if canLogin("andreas.rolando@esb.co.id", "percobaan-timpa-2") || !canLogin("andreas.rolando@esb.co.id", "demo-pertama-1") {
		t.Error("bootstrap overwrote an existing password")
	}

	// A new email is created as an admin.
	if _, err := bootstrapAdmin(db, "baru@esb.co.id", "rahasia-baru-3"); err != nil {
		t.Fatal(err)
	}
	var u model.User
	db.First(&u, "email = ?", "baru@esb.co.id")
	if !u.IsAdmin || !canLogin("baru@esb.co.id", "rahasia-baru-3") {
		t.Errorf("new bootstrap user = %+v, want an admin that can log in", u)
	}

	// Bad input is rejected without touching anything.
	for name, c := range map[string][2]string{
		"missing password": {"x@esb.co.id", ""},
		"missing email":    {"", "rahasia-123"},
		"weak password":    {"x@esb.co.id", "pendek"},
	} {
		if _, err := bootstrapAdmin(db, c[0], c[1]); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	var n int64
	db.Model(&model.User{}).Where("email = ?", "x@esb.co.id").Count(&n)
	if n != 0 {
		t.Error("rejected bootstrap input created a user")
	}
}

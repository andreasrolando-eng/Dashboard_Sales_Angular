package service_test

import (
	"testing"

	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/service"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

func TestAddUser_CreatesNew(t *testing.T) {
	db := testutil.SetupTestDB(t)

	u, err := service.AddUser(db, "Budi@ESB.co.id", true)
	if err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	if u.Email != "budi@esb.co.id" {
		t.Errorf("email = %q, want lowercased %q", u.Email, "budi@esb.co.id")
	}
	if !u.IsAdmin {
		t.Errorf("is_admin = false, want true")
	}
}

func TestAddUser_UpsertOverwritesIsAdmin(t *testing.T) {
	db := testutil.SetupTestDB(t)

	if _, err := service.AddUser(db, "budi@esb.co.id", true); err != nil {
		t.Fatalf("first AddUser: %v", err)
	}

	// Re-adding the same email with is_admin=false demotes them -- this is
	// existing, documented behavior ported as-is (see AddUser's doc comment).
	u, err := service.AddUser(db, "budi@esb.co.id", false)
	if err != nil {
		t.Fatalf("second AddUser: %v", err)
	}
	if u.IsAdmin {
		t.Errorf("is_admin = true, want false after re-add with is_admin=false")
	}

	users, err := service.ListUsers(db)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	// The seeded admin from the users migration is already there, so
	// upserting budi@esb.co.id should bring the total to 2, not 3 -- if the
	// upsert created a second row instead of updating, this would be 3.
	if len(users) != 2 {
		t.Fatalf("len(users) = %d, want 2 (seeded admin + upserted budi, not a second budi row)", len(users))
	}
}

func TestRemoveUser(t *testing.T) {
	db := testutil.SetupTestDB(t)

	if _, err := service.AddUser(db, "budi@esb.co.id", false); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	if err := service.RemoveUser(db, "BUDI@esb.co.id"); err != nil {
		t.Fatalf("RemoveUser: %v", err)
	}

	var count int64
	if err := db.Model(&model.User{}).Where("email = ?", "budi@esb.co.id").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 after remove", count)
	}
}

func TestListUsers_NewestFirst(t *testing.T) {
	db := testutil.SetupTestDB(t)

	if _, err := service.AddUser(db, "first@esb.co.id", false); err != nil {
		t.Fatalf("AddUser first: %v", err)
	}
	if _, err := service.AddUser(db, "second@esb.co.id", false); err != nil {
		t.Fatalf("AddUser second: %v", err)
	}

	users, err := service.ListUsers(db)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	// Includes the seeded admin from migration 20260928170001_users.sql plus
	// the two just added.
	if len(users) != 3 {
		t.Fatalf("len(users) = %d, want 3", len(users))
	}
	if users[0].Email != "second@esb.co.id" {
		t.Errorf("users[0].Email = %q, want the most recently added user first", users[0].Email)
	}
}

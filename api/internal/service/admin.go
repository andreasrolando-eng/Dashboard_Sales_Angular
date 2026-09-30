package service

import (
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
)

// ListUsers returns every app user, newest-added first -- ported from the
// old fn_admin_list_users (ordered by added_at desc).
func ListUsers(db *gorm.DB) ([]model.User, error) {
	var users []model.User
	err := db.Order("created_at desc").Find(&users).Error
	return users, err
}

// AddUser upserts a user by email (case-insensitive). Re-adding an existing
// email overwrites IsAdmin -- this matches the old fn_admin_add_user
// behavior exactly, including the fact that re-adding an admin with the
// checkbox unchecked demotes them. Not "fixed" here on purpose: it's
// existing, known, documented behavior, not something M2 is scoped to change.
func AddUser(db *gorm.DB, email string, isAdmin bool) (model.User, error) {
	u := model.User{Email: strings.ToLower(strings.TrimSpace(email)), IsAdmin: isAdmin}
	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "email"}},
		DoUpdates: clause.AssignmentColumns([]string{"is_admin", "updated_at"}),
	}).Create(&u).Error
	return u, err
}

// RemoveUser deletes a user by email (case-insensitive); the user's sessions
// go with it (ON DELETE CASCADE). The "you can't remove yourself" rule lives in
// the HTTP handler, where the signed-in caller is known.
func RemoveUser(db *gorm.DB, email string) error {
	return db.Where("email = ?", strings.ToLower(strings.TrimSpace(email))).Delete(&model.User{}).Error
}

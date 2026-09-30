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

// RemoveUser deletes a user by email (case-insensitive).
//
// TODO(SSO): the old fn_admin_remove_user refused to let a caller remove
// their own access, checked against the caller's session email. There is no
// session yet (SSO deferred) -- this function does not accept a
// caller-supplied "actor email" as a substitute, since that would be trivial
// to spoof. Wire the self-removal check here once operations-sso sessions
// exist and the handler has a real caller identity.
func RemoveUser(db *gorm.DB, email string) error {
	return db.Where("email = ?", strings.ToLower(strings.TrimSpace(email))).Delete(&model.User{}).Error
}

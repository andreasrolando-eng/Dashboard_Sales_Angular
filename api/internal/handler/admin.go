package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/auth"
	"github.com/Operations-ESB/dashboard-sales/api/internal/service"
)

// All handlers in this file sit behind auth.Require + auth.RequireAdmin.

func ListUsers(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := service.ListUsers(db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, users)
	}
}

type addUserRequest struct {
	Email   string `json:"email"`
	IsAdmin bool   `json:"is_admin"`
	// Password is optional: without it the account exists but cannot sign in
	// with the form yet (an admin can set one later).
	Password string `json:"password"`
}

// AddUser handles POST /api/admin/users. Re-adding an existing email keeps
// the documented upsert behaviour (overwrites is_admin) and, if a password is
// given, resets it.
func AddUser(db *gorm.DB, authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req addUserRequest
		if !decodeBody(w, r, &req) {
			return
		}
		if !strings.Contains(req.Email, "@") {
			writeError(w, http.StatusBadRequest, fmt.Errorf("email tidak valid"))
			return
		}
		// Validate before creating anything, so a weak password doesn't leave
		// a half-created account behind.
		if req.Password != "" {
			if err := auth.ValidatePassword(req.Password); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		}

		user, err := service.AddUser(db, req.Email, req.IsAdmin)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if req.Password != "" {
			if err := authSvc.SetPassword(req.Email, req.Password); err != nil {
				writeError(w, http.StatusInternalServerError, fmt.Errorf("user dibuat tetapi password gagal diatur: %w", err))
				return
			}
			user.HasPassword = true
		}
		writeJSON(w, http.StatusOK, user)
	}
}

type setPasswordRequest struct {
	Password string `json:"password"`
}

// SetUserPassword handles POST /api/admin/users/{email}/password: an admin
// (re)sets someone's password. That user's sessions all end.
func SetUserPassword(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email, err := url.PathUnescape(chi.URLParam(r, "email"))
		if err != nil || email == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("email tidak ditemukan"))
			return
		}
		var req setPasswordRequest
		if !decodeBody(w, r, &req) {
			return
		}
		err = authSvc.SetPassword(email, req.Password)
		switch {
		case errors.Is(err, auth.ErrWeakPassword):
			writeError(w, http.StatusBadRequest, err)
		case errors.Is(err, gorm.ErrRecordNotFound):
			writeError(w, http.StatusNotFound, fmt.Errorf("user tidak ditemukan"))
		case err != nil:
			writeError(w, http.StatusInternalServerError, fmt.Errorf("gagal mengatur password"))
		default:
			writeJSON(w, http.StatusNoContent, nil)
		}
	}
}

// RemoveUser handles DELETE /api/admin/users/{email}. A caller cannot remove
// their own access (ported from the old fn_admin_remove_user), which also
// guarantees the last admin can't lock everyone out by accident.
func RemoveUser(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// chi returns the still percent-encoded segment when the client sent
		// %40 for @ (browsers do), so decode it or the delete silently matches nothing.
		email, err := url.PathUnescape(chi.URLParam(r, "email"))
		if err != nil || email == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("email tidak ditemukan"))
			return
		}
		if actor, ok := auth.UserFrom(r.Context()); ok && strings.EqualFold(strings.TrimSpace(email), actor.Email) {
			writeError(w, http.StatusConflict, fmt.Errorf("tidak bisa menghapus akses sendiri"))
			return
		}
		if err := service.RemoveUser(db, email); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusNoContent, nil)
	}
}

package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/service"
)

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
}

func AddUser(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req addUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if !strings.Contains(req.Email, "@") {
			writeError(w, http.StatusBadRequest, fmt.Errorf("email tidak valid"))
			return
		}

		user, err := service.AddUser(db, req.Email, req.IsAdmin)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, user)
	}
}

// RemoveUser handles DELETE /api/admin/users/{email}.
//
// TODO(SSO): once a real session exists, reject removing the caller's own
// access here (see service.RemoveUser's TODO for why that isn't faked now).
func RemoveUser(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// chi returns the still percent-encoded segment when the client sent
		// %40 for @ (browsers do), so decode it or the delete silently matches nothing.
		email, err := url.PathUnescape(chi.URLParam(r, "email"))
		if err != nil || email == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("email tidak ditemukan"))
			return
		}
		if err := service.RemoveUser(db, email); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusNoContent, nil)
	}
}

package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/service"
)

func MembershipSummary(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		summary, err := service.GetMembershipSummary(db, start, end, optionalString(r, "outlet"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, summary)
	}
}

func TopMembers(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		limit := intParam(r, "limit", 8)
		rows, err := service.GetTopMembers(db, start, end, optionalString(r, "outlet"), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

func MemberOptions(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := service.GetMemberOptions(db, optionalString(r, "outlet"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

func MemberMenuPurchases(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		memberCode := chi.URLParam(r, "memberCode")
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		rows, err := service.GetMemberMenuPurchases(db, memberCode, start, end, optionalString(r, "outlet"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

func MembershipNewWeekly(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		end, err := parseDate(r.URL.Query().Get("dateEnd"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		rows, err := service.GetMembershipNewWeekly(db, end)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

package handler

import (
	"net/http"

	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/service"
)

func NonSalesSummary(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		s, err := service.GetNonSalesSummary(db, start, end, optionalString(r, "outlet"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, s)
	}
}

func NonSalesDaily(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		rows, err := service.GetNonSalesDaily(db, start, end, optionalString(r, "outlet"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

func NonSalesByOutlet(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		rows, err := service.GetNonSalesByOutlet(db, start, end)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

func NonSalesTopMenus(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		rows, err := service.GetNonSalesTopMenus(db, start, end, optionalString(r, "outlet"), intParam(r, "limit", 10))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

func NonSalesBills(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		rows, total, err := service.GetNonSalesBills(db, start, end, optionalString(r, "outlet"),
			intParam(r, "page", 1), intParam(r, "pageSize", 50))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"rows": rows, "total_count": total})
	}
}

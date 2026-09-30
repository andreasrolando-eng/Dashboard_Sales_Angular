package handler

import (
	"net/http"

	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/service"
)

func PromoPerformance(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		rows, err := service.GetPromoPerformance(db, start, end, optionalString(r, "outlet"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

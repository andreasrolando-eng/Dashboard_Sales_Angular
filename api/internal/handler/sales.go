package handler

import (
	"fmt"
	"net/http"

	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/service"
)

func SalesSummary(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		summary, err := service.GetSalesSummary(db, start, end, optionalString(r, "outlet"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, summary)
	}
}

func SalesDaily(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		rows, err := service.GetSalesDaily(db, start, end, optionalString(r, "outlet"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

func SalesHourly(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		rows, err := service.GetSalesHourly(db, start, end, optionalString(r, "outlet"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

func RevenueByOutlet(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		rows, err := service.GetRevenueByOutlet(db, start, end)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

func RevenueByCategory(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		result, err := service.GetRevenueByCategory(db, start, end,
			optionalString(r, "outlet"), optionalString(r, "category"), optionalString(r, "categoryDetail"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func TopProducts(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		sortBy := r.URL.Query().Get("sortBy")
		sortDesc := r.URL.Query().Get("sortDir") != "asc"
		limit := intParam(r, "limit", 0)

		rows, err := service.GetTopProducts(db, start, end,
			optionalString(r, "outlet"), optionalString(r, "category"), optionalString(r, "categoryDetail"),
			sortBy, sortDesc, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

func MenuPerformance(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		threshold := floatParam(r, "threshold", 300)

		rows, err := service.GetMenuPerformance(db, start, end,
			optionalString(r, "outlet"), optionalString(r, "category"), optionalString(r, "categoryDetail"),
			threshold)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

func SalesBills(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		page := intParam(r, "page", 1)
		pageSize := intParam(r, "pageSize", 50)

		rows, total, err := service.GetSalesBills(db, start, end, optionalString(r, "outlet"), page, pageSize)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"rows": rows, "total_count": total})
	}
}

func SalesBillsExport(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start, end, err := dateRangeParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		rows, err := service.GetAllSalesBills(db, start, end, optionalString(r, "outlet"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if r.URL.Query().Get("format") != "xlsx" {
			writeJSON(w, http.StatusOK, rows)
			return
		}
		outlets, err := service.ListOutlets(db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		names := make(map[string]string, len(outlets))
		for _, o := range outlets {
			names[o.BranchCode] = o.BranchName
		}
		filename := fmt.Sprintf("bill-sales_%s_%s.xlsx", start.Format("2006-01-02"), end.Format("2006-01-02"))
		writeBillsXLSX(w, filename, rows, names)
	}
}

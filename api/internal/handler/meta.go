package handler

import (
	"net/http"

	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/service"
)

func ListOutlets(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		outlets, err := service.ListOutlets(db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, outlets)
	}
}

func ListCategories(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		categories, err := service.ListCategories(db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, categories)
	}
}

func ListCategoryDetails(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		details, err := service.ListCategoryDetails(db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, details)
	}
}

func GetLastSync(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sync, err := service.GetLastSync(db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if sync == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no successful sync yet"})
			return
		}
		writeJSON(w, http.StatusOK, sync)
	}
}

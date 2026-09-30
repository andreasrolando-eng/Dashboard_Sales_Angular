package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/etl"
	"github.com/Operations-ESB/dashboard-sales/api/internal/service"
)

type manualSyncRequest struct {
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
	// Mode is "fill" (default: only add what is missing) or "refresh"
	// (overwrite stored bills with the latest ESB data, e.g. to pick up voids).
	Mode string `json:"mode"`
}

// StartManualSync handles POST /api/admin/sync: starts a background sync of
// [date_from, date_to]. mode "fill" (default) only adds data that is not
// stored yet; mode "refresh" also overwrites stored bills with what ESB
// returns now and reports which bills changed status.
// syncer is nil when the ESB credentials are not configured.
//
// TODO(SSO): like the other /api/admin routes this has no caller identity yet;
// restrict it to admins once operations-sso sessions exist.
func StartManualSync(syncer *etl.ManualSyncer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if syncer == nil {
			writeError(w, http.StatusServiceUnavailable,
				fmt.Errorf("sinkron manual belum aktif: ESB_API_BASE_URL dan ESB_API_KEY belum diisi di server"))
			return
		}
		var req manualSyncRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("body tidak valid: %w", err))
			return
		}
		from, err := parseDate(req.DateFrom)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("date_from: %w", err))
			return
		}
		to, err := parseDate(req.DateTo)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("date_to: %w", err))
			return
		}

		mode, err := etl.ParseManualMode(req.Mode)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		job, err := syncer.Start(from, to, mode)
		switch {
		case errors.Is(err, etl.ErrBusy):
			writeError(w, http.StatusConflict, err)
		case err != nil:
			writeError(w, http.StatusBadRequest, err)
		default:
			writeJSON(w, http.StatusAccepted, job)
		}
	}
}

// GetManualSync handles GET /api/admin/sync: the latest manual sync job
// (running or finished), or null if none has run since the server started.
func GetManualSync(syncer *etl.ManualSyncer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var job *etl.ManualJob // typed nil so the body is the JSON literal null, not empty
		if syncer != nil {
			job = syncer.Current()
		}
		writeJSON(w, http.StatusOK, job)
	}
}

// ListSyncLogs handles GET /api/admin/sync/logs?limit=N.
func ListSyncLogs(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs, err := service.ListSyncLogs(db, intParam(r, "limit", 30))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, logs)
	}
}

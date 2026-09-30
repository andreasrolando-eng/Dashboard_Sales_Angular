package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Operations-ESB/dashboard-sales/api/internal/etl"
	"github.com/Operations-ESB/dashboard-sales/api/internal/handler"
	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/testutil"
)

func syncRouter(syncer *etl.ManualSyncer) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/admin/sync", handler.GetManualSync(syncer))
	r.Post("/api/admin/sync", handler.StartManualSync(syncer))
	return r
}

func post(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/sync", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func fakeESB(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		date := r.URL.Query().Get("salesDateFrom")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"salesNum": "SN-" + date, "salesDate": date, "branchCode": "BR01", "grandTotal": 1000, "statusName": "Finished"},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestStartManualSync_NotConfigured(t *testing.T) {
	rec := post(t, syncRouter(nil), `{"date_from":"2026-02-01","date_to":"2026-02-01"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 when ESB credentials are missing", rec.Code)
	}
	get := httptest.NewRecorder()
	syncRouter(nil).ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/admin/sync", nil))
	if get.Code != http.StatusOK || strings.TrimSpace(get.Body.String()) != "null" {
		t.Errorf("GET = %d %q, want 200 null", get.Code, get.Body.String())
	}
}

func TestStartManualSync_ValidatesInput(t *testing.T) {
	db := testutil.SetupTestDB(t)
	h := syncRouter(etl.NewManualSyncer(db, etl.NewClient("http://127.0.0.1:1", "k")))

	for name, body := range map[string]string{
		"not json":         `nope`,
		"missing dates":    `{}`,
		"bad date format":  `{"date_from":"01-02-2026","date_to":"2026-02-01"}`,
		"reversed":         `{"date_from":"2026-02-05","date_to":"2026-02-01"}`,
		"future end":       `{"date_from":"2026-02-01","date_to":"2999-01-01"}`,
		"longer than 62 d": `{"date_from":"2025-01-01","date_to":"2025-12-31"}`,
	} {
		if rec := post(t, h, body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d (%s), want 400", name, rec.Code, strings.TrimSpace(rec.Body.String()))
		}
	}
}

func TestStartManualSync_RunsAndReportsProgress(t *testing.T) {
	db := testutil.SetupTestDB(t)
	esb := fakeESB(t)
	h := syncRouter(etl.NewManualSyncer(db, etl.NewClient(esb.URL, "k")))

	rec := post(t, h, `{"date_from":"2026-02-01","date_to":"2026-02-02"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d (%s), want 202", rec.Code, rec.Body.String())
	}

	var job etl.ManualJob
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		get := httptest.NewRecorder()
		h.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/admin/sync", nil))
		job = etl.ManualJob{}
		if err := json.Unmarshal(get.Body.Bytes(), &job); err != nil {
			t.Fatal(err)
		}
		if job.Status != etl.StatusRunning {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if job.Status != etl.StatusDone || len(job.Days) != 2 || job.Days[0].Result == nil || job.Days[0].Result.Sales != 1 {
		t.Fatalf("job = %+v, want done with 2 days and 1 new sale on the first", job)
	}

	// The second POST for the same range is accepted but inserts nothing.
	if rec := post(t, h, `{"date_from":"2026-02-01","date_to":"2026-02-02"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("second POST status = %d, want 202", rec.Code)
	}
	var n int64
	deadline = time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		db.Model(&model.SyncLog{}).Where("job_name = ? and status = 'success'", etl.ManualJobName).Count(&n)
		if n == 4 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	var sales int64
	db.Model(&model.RawSale{}).Count(&sales)
	if n != 4 || sales != 2 {
		t.Errorf("manual log rows = %d, raw_sales = %d, want 4 and 2 (no duplicates)", n, sales)
	}
}

func TestStartManualSync_ModeValidation(t *testing.T) {
	db := testutil.SetupTestDB(t)
	esb := fakeESB(t)
	h := syncRouter(etl.NewManualSyncer(db, etl.NewClient(esb.URL, "k")))

	if rec := post(t, h, `{"date_from":"2026-02-01","date_to":"2026-02-01","mode":"overwrite-everything"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown mode: status = %d, want 400", rec.Code)
	}

	rec := post(t, h, `{"date_from":"2026-02-01","date_to":"2026-02-01","mode":"refresh"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("refresh: status = %d (%s), want 202", rec.Code, rec.Body.String())
	}
	var job etl.ManualJob
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil || job.Mode != etl.ManualRefresh {
		t.Errorf("job = %+v (%v), want mode refresh echoed back", job, err)
	}
}

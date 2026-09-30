package etl

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestClient_FetchSalesDay_PaginatesAndAuthenticates simulates ESB's exact
// contract (records in the body, pagination in x-pagination-* response
// headers, Bearer auth) with httptest -- this verifies the whole client
// without needing real ESB credentials or network access. Only a final
// live smoke test against the real staging server needs those.
func TestClient_FetchSalesDay_PaginatesAndAuthenticates(t *testing.T) {
	var gotPages []string
	var gotAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		page := r.URL.Query().Get("page")
		gotPages = append(gotPages, page)

		if r.URL.Query().Get("salesDateFrom") != "2026-02-03" || r.URL.Query().Get("salesDateTo") != "2026-02-03" {
			t.Errorf("unexpected date params: %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("sortBy") != "salesDateIn" || r.URL.Query().Get("sortOrder") != "asc" {
			t.Errorf("unexpected sort params: %s", r.URL.RawQuery)
		}

		w.Header().Set("x-pagination-page-count", "2")
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "1":
			w.Header().Set("x-pagination-current-page", "1")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]esbSaleRecord{
				{SalesNum: "SN1", SalesDate: "2026-02-03", BranchCode: "BR01"},
				{SalesNum: "SN2", SalesDate: "2026-02-03", BranchCode: "BR01"},
			})
		case "2":
			w.Header().Set("x-pagination-current-page", "2")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]esbSaleRecord{
				{SalesNum: "SN3", SalesDate: "2026-02-03", BranchCode: "BR01"},
			})
		default:
			t.Errorf("unexpected page requested: %q", page)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]esbSaleRecord{})
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	records, err := client.FetchSalesDay(context.Background(), "2026-02-03")
	if err != nil {
		t.Fatalf("FetchSalesDay: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("len(records) = %d, want 3 (all pages combined)", len(records))
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer test-key")
	}
	if len(gotPages) != 2 || gotPages[0] != "1" || gotPages[1] != "2" {
		t.Errorf("pages requested = %v, want [1 2] (fetched in order, stopping at page_count)", gotPages)
	}
}

func TestClient_FetchSalesDay_NonOKStatusFailsWholeDay(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad-key")
	_, err := client.FetchSalesDay(context.Background(), "2026-02-03")
	if err == nil {
		t.Fatal("FetchSalesDay: expected error on 401, got nil")
	}
}

func TestClient_BaseURLPathPrefixIsPreserved(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]esbSaleRecord{})
	}))
	defer server.Close()

	// Base URL carries its own path prefix, like the real
	// ".../api-fnb-backend/web" -- string concatenation must preserve it
	// rather than a URL-resolve that would drop it.
	client := NewClient(server.URL+"/api-fnb-backend/web/", "test-key")
	if _, err := client.FetchSalesDay(context.Background(), "2026-02-03"); err != nil {
		t.Fatalf("FetchSalesDay: %v", err)
	}
	want := "/api-fnb-backend/web/corev1/sales/sales-information"
	if gotPath != want {
		t.Errorf("request path = %q, want %q", gotPath, want)
	}
}

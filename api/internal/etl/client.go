package etl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Client talks to the ESB OMS API. Ported from
// supabase/functions/sync-esb/esb-client.ts.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{BaseURL: baseURL, APIKey: apiKey, HTTP: &http.Client{Timeout: 60 * time.Second}}
}

// FetchSalesDay fetches every page of sales records for one calendar date
// (salesDateFrom == salesDateTo == date). Page count is read only from page
// 1's response headers, matching the original -- there is no retry and any
// non-2xx response fails the whole day.
func (c *Client) FetchSalesDay(ctx context.Context, date string) ([]esbSaleRecord, error) {
	first, pagination, err := c.fetchSalesPage(ctx, date, 1)
	if err != nil {
		return nil, err
	}
	all := first

	currentPage := pagination.CurrentPage
	if currentPage == 0 {
		currentPage = 1
	}
	pageCount := pagination.PageCount
	if pageCount == 0 {
		pageCount = 1
	}

	for currentPage < pageCount {
		currentPage++
		records, _, err := c.fetchSalesPage(ctx, date, currentPage)
		if err != nil {
			return nil, err
		}
		all = append(all, records...)
	}
	return all, nil
}

func (c *Client) fetchSalesPage(ctx context.Context, date string, page int) ([]esbSaleRecord, paginationHeaders, error) {
	// String concatenation, not url.ResolveReference -- BaseURL carries its
	// own path prefix (e.g. /api-fnb-backend/web) that ResolveReference
	// would drop.
	base := strings.TrimRight(c.BaseURL, "/")
	url := base + "/corev1/sales/sales-information" +
		"?salesDateFrom=" + date +
		"&salesDateTo=" + date +
		"&page=" + strconv.Itoa(page) +
		"&sortBy=salesDateIn&sortOrder=asc"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, paginationHeaders{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, paginationHeaders{}, fmt.Errorf("ESB get-sales-information: %w (page %d, date %s)", err, page, date)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, paginationHeaders{}, fmt.Errorf("ESB get-sales-information failed: %d %s (page %d, date %s)", resp.StatusCode, resp.Status, page, date)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, paginationHeaders{}, fmt.Errorf("read ESB response body: %w", err)
	}
	var records []esbSaleRecord
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, paginationHeaders{}, fmt.Errorf("decode ESB response body: %w", err)
	}

	return records, paginationHeaders{
		TotalCount:  headerInt(resp.Header, "x-pagination-total-count", 0),
		PageCount:   headerInt(resp.Header, "x-pagination-page-count", 1),
		CurrentPage: headerInt(resp.Header, "x-pagination-current-page", 1),
		PerPage:     headerInt(resp.Header, "x-pagination-per-page", 20),
	}, nil
}

func headerInt(h http.Header, key string, fallback int) int {
	v := h.Get(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

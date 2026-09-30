package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func dateRangeParams(r *http.Request) (start, end time.Time, err error) {
	start, err = parseDate(r.URL.Query().Get("dateStart"))
	if err != nil {
		return start, end, fmt.Errorf("dateStart: %w", err)
	}
	end, err = parseDate(r.URL.Query().Get("dateEnd"))
	if err != nil {
		return start, end, fmt.Errorf("dateEnd: %w", err)
	}
	if start.After(end) {
		return start, end, fmt.Errorf("dateStart must be on or before dateEnd")
	}
	return start, end, nil
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("required, format YYYY-MM-DD")
	}
	return time.Parse("2006-01-02", s)
}

// optionalString returns nil for an absent/empty query param, so callers
// can pass it straight into a `col is null or col = ?`-style SQL filter.
func optionalString(r *http.Request, key string) *string {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	return &v
}

func intParam(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func floatParam(r *http.Request, key string, def float64) float64 {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return n
}

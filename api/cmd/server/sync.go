package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Operations-ESB/dashboard-sales/api/internal/config"
	"github.com/Operations-ESB/dashboard-sales/api/internal/db"
	"github.com/Operations-ESB/dashboard-sales/api/internal/etl"
)

// runSyncCommand implements `server sync run [--date=YYYY-MM-DD |
// --from=YYYY-MM-DD --to=YYYY-MM-DD]`, defaulting to yesterday in WIB when
// no date flag is given (matching the original Edge Function's default).
func runSyncCommand(cfg config.Config, args []string) error {
	if len(args) < 1 || args[0] != "run" {
		return fmt.Errorf("usage: server sync run [--date=YYYY-MM-DD | --from=YYYY-MM-DD --to=YYYY-MM-DD]")
	}

	fs := flag.NewFlagSet("sync run", flag.ContinueOnError)
	date := fs.String("date", "", "single date to sync (YYYY-MM-DD)")
	from := fs.String("from", "", "range start (YYYY-MM-DD)")
	to := fs.String("to", "", "range end (YYYY-MM-DD)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if cfg.ESBAPIBaseURL == "" || cfg.ESBAPIKey == "" {
		return fmt.Errorf("ESB_API_BASE_URL and ESB_API_KEY must be set in .env before running sync")
	}

	var fromDate, toDate time.Time
	var err error
	switch {
	case *from != "" && *to != "":
		fromDate, err = time.Parse("2006-01-02", *from)
		if err != nil {
			return fmt.Errorf("--from: %w", err)
		}
		toDate, err = time.Parse("2006-01-02", *to)
		if err != nil {
			return fmt.Errorf("--to: %w", err)
		}
	case *date != "":
		fromDate, err = time.Parse("2006-01-02", *date)
		if err != nil {
			return fmt.Errorf("--date: %w", err)
		}
		toDate = fromDate
	default:
		fromDate, err = time.Parse("2006-01-02", etl.YesterdayWIB())
		if err != nil {
			return err
		}
		toDate = fromDate
	}
	if toDate.Before(fromDate) {
		return fmt.Errorf("--from must be on or before --to")
	}

	gormDB, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return err
	}

	client := etl.NewClient(cfg.ESBAPIBaseURL, cfg.ESBAPIKey)
	summary := etl.SyncRange(context.Background(), gormDB, client, fromDate, toDate)

	totalRows := 0
	failed := 0
	for _, d := range summary.Days {
		totalRows += d.Outlets + d.Sales + d.Payments + d.MenuItems
		if !d.OK {
			failed++
		}
	}
	summaryText := fmt.Sprintf("sync-esb %s..%s: %d rows total, %d/%d day(s) failed",
		summary.DateFrom, summary.DateTo, totalRows, failed, len(summary.Days))
	etl.PingHealthchecks(cfg.HealthchecksPingURL, summaryText, !summary.OK)
	if failed > 0 {
		etl.SendAlert(cfg.SyncAlertWebhookURL, summaryText)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(summary)
	fmt.Println(summaryText)

	if !summary.OK {
		return fmt.Errorf("all days failed")
	}
	return nil
}

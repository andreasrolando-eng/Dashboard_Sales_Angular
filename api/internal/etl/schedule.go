package etl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
)

// WIB is a fixed UTC+7 zone (Asia/Jakarta has no DST), so no tzdata is needed.
var WIB = time.FixedZone("WIB", 7*60*60)

// syncAdvisoryLockKey guards against two API instances running the nightly
// sync at once (arbitrary constant, unique to this job).
const syncAdvisoryLockKey int64 = 7_402_119_001

// NextRun returns the next occurrence of hour:00 WIB strictly after now.
func NextRun(now time.Time, hour int) time.Time {
	n := now.In(WIB)
	next := time.Date(n.Year(), n.Month(), n.Day(), hour, 0, 0, 0, WIB)
	if !next.After(n) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// MissingDates returns, oldest first, every date in the lookbackDays days
// ending yesterday (WIB) that has no successful sync-esb run. This one query
// drives both the nightly run (yesterday is normally the only gap) and
// catch-up after downtime or failed days.
func MissingDates(db *gorm.DB, now time.Time, lookbackDays int) ([]time.Time, error) {
	yesterday := now.In(WIB).AddDate(0, 0, -1)
	first := yesterday.AddDate(0, 0, -(lookbackDays - 1))

	var done []string
	err := db.Raw(`select distinct to_char(target_date, 'YYYY-MM-DD') from sync_logs
		where job_name = 'sync-esb' and status = 'success' and target_date >= ?`,
		first.Format("2006-01-02")).Scan(&done).Error
	if err != nil {
		return nil, err
	}
	have := make(map[string]bool, len(done))
	for _, d := range done {
		have[d] = true
	}

	var missing []time.Time
	for i := 0; i < lookbackDays; i++ {
		d := first.AddDate(0, 0, i)
		if !have[d.Format("2006-01-02")] {
			// Plain calendar date, midnight UTC, like the CLI's time.Parse.
			missing = append(missing, time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC))
		}
	}
	return missing, nil
}

// RefreshDates returns, oldest first, the days in the refreshDays days ending
// yesterday (WIB) that are not in skip. The nightly run re-fetches them
// because ESB keeps returning a bill under its original transaction date
// after it is voided later, and catch-up alone never looks at a day again
// once it synced successfully.
func RefreshDates(now time.Time, refreshDays int, skip []time.Time) []time.Time {
	skipped := make(map[string]bool, len(skip))
	for _, d := range skip {
		skipped[d.Format("2006-01-02")] = true
	}
	yesterday := now.In(WIB).AddDate(0, 0, -1)
	var dates []time.Time
	for i := refreshDays - 1; i >= 0; i-- {
		d := yesterday.AddDate(0, 0, -i)
		if !skipped[d.Format("2006-01-02")] {
			dates = append(dates, time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC))
		}
	}
	return dates
}

// SchedulerConfig configures the in-process nightly sync.
type SchedulerConfig struct {
	Hour            int    // WIB hour of the daily run
	CatchupDays     int    // how far back to look for unsynced days
	RefreshDays     int    // already-synced recent days to re-fetch each run; 0 disables
	AlertWebhookURL string // Slack/Discord-compatible incoming webhook; empty disables alerts
	HealthchecksURL string // optional healthchecks.io ping
}

type Scheduler struct {
	DB     *gorm.DB
	Client *Client
	Cfg    SchedulerConfig
	Now    func() time.Time // injectable for tests; defaults to time.Now
}

func (s *Scheduler) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// WithSyncLock runs fn while holding the Postgres advisory lock that
// serialises every ESB sync (nightly scheduler on any instance, and manual
// syncs). acquired is false, and fn is not run, if another sync holds it.
func WithSyncLock(db *gorm.DB, fn func() error) (acquired bool, err error) {
	err = db.Transaction(func(tx *gorm.DB) error {
		var got bool
		if err := tx.Raw("select pg_try_advisory_xact_lock(?)", syncAdvisoryLockKey).Scan(&got).Error; err != nil {
			return err
		}
		if !got {
			return nil
		}
		acquired = true
		return fn()
	})
	return acquired, err
}

// RefreshJobName is the job_name of the nightly re-fetch of recent days. It
// differs from "sync-esb" so it never counts toward catch-up or "last sync".
const RefreshJobName = "sync-esb-refresh"

// RunOnce syncs every missing day in the catch-up window, then re-fetches
// (upsert, with change report) the RefreshDays most recent days that were
// already synced. Results hold the missing days first, then the refreshed
// ones. It returns nil summaries (skipped=true) if another instance holds the
// advisory lock.
func (s *Scheduler) RunOnce(ctx context.Context) (results []DaySyncResult, skipped bool, err error) {
	var refreshed []DaySyncResult
	acquired, err := WithSyncLock(s.DB, func() error {
		now := s.now()
		dates, err := MissingDates(s.DB, now, s.Cfg.CatchupDays)
		if err != nil {
			return err
		}
		for _, d := range dates {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			results = append(results, SyncDay(ctx, s.DB, s.Client, d.Format("2006-01-02")))
		}
		// Days just synced (or just failed, retried next run) are skipped.
		for _, d := range RefreshDates(now, s.Cfg.RefreshDays, dates) {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			refreshed = append(refreshed, SyncDayWith(ctx, s.DB, s.Client, d.Format("2006-01-02"),
				SyncOptions{Mode: ModeUpsert, JobName: RefreshJobName, ReportChanges: true}))
		}
		return nil
	})
	results = append(results, refreshed...)
	skipped = !acquired
	if err != nil {
		return results, skipped, err
	}

	if len(results) > 0 {
		s.report(results)
	}
	return results, skipped, nil
}

func (s *Scheduler) report(results []DaySyncResult) {
	var failed, changes []string
	rows := 0
	for _, r := range results {
		rows += r.Outlets + r.Sales + r.Payments + r.MenuItems
		if !r.OK {
			failed = append(failed, fmt.Sprintf("%s (%s)", r.Date, r.Error))
		}
		for _, c := range r.StatusChanges {
			changes = append(changes, fmt.Sprintf("%s %s %s→%s", r.Date, c.SalesNum, c.From, c.To))
		}
	}
	summary := fmt.Sprintf("sync-esb scheduled: %d day(s), %d rows, %d failed, %d status change(s)",
		len(results), rows, len(failed), len(changes))
	if len(changes) > 0 {
		log.Print("sync-esb refresh status changes: " + strings.Join(changes, "; "))
	}
	log.Print(summary)
	PingHealthchecks(s.Cfg.HealthchecksURL, summary, len(failed) > 0)
	if len(failed) > 0 {
		SendAlert(s.Cfg.AlertWebhookURL, summary+": "+strings.Join(failed, "; "))
	}
}

// Start blocks until ctx is cancelled: catch-up once immediately (covers
// downtime and a restart right after the daily run was missed), then run
// every day at Cfg.Hour WIB.
func (s *Scheduler) Start(ctx context.Context) {
	s.tick(ctx)
	for {
		next := NextRun(s.now(), s.Cfg.Hour)
		log.Printf("sync-esb scheduler: next run at %s", next.Format(time.RFC3339))
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.tick(ctx)
		}
	}
}

// tick never panics or returns an error to the loop: one bad night must not
// kill the scheduler for good.
func (s *Scheduler) tick(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("sync-esb scheduler panic: %v", r)
			log.Print(msg)
			SendAlert(s.Cfg.AlertWebhookURL, msg)
		}
	}()
	if _, skipped, err := s.RunOnce(ctx); err != nil {
		msg := "sync-esb scheduler error: " + err.Error()
		log.Print(msg)
		SendAlert(s.Cfg.AlertWebhookURL, msg)
	} else if skipped {
		log.Print("sync-esb scheduler: another instance holds the lock, skipping")
	}
}

// SendAlert posts text to a Slack- or Discord-compatible incoming webhook
// (the payload carries both "text" and "content"). Best-effort, like the
// healthchecks ping; a no-op when url is empty.
func SendAlert(url, text string) {
	if url == "" {
		return
	}
	body, _ := json.Marshal(map[string]string{"text": text, "content": text})
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("alert webhook failed: %v", err)
		return
	}
	resp.Body.Close()
}

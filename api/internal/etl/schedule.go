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

// SchedulerConfig configures the in-process nightly sync.
type SchedulerConfig struct {
	Hour            int    // WIB hour of the daily run
	CatchupDays     int    // how far back to look for unsynced days
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

// RunOnce syncs every missing day in the catch-up window. It returns nil
// summaries (skipped=true) if another instance holds the advisory lock.
func (s *Scheduler) RunOnce(ctx context.Context) (results []DaySyncResult, skipped bool, err error) {
	acquired, err := WithSyncLock(s.DB, func() error {
		dates, err := MissingDates(s.DB, s.now(), s.Cfg.CatchupDays)
		if err != nil {
			return err
		}
		for _, d := range dates {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			results = append(results, SyncDay(ctx, s.DB, s.Client, d.Format("2006-01-02")))
		}
		return nil
	})
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
	var failed []string
	rows := 0
	for _, r := range results {
		rows += r.Outlets + r.Sales + r.Payments + r.MenuItems
		if !r.OK {
			failed = append(failed, fmt.Sprintf("%s (%s)", r.Date, r.Error))
		}
	}
	summary := fmt.Sprintf("sync-esb scheduled: %d day(s), %d rows, %d failed", len(results), rows, len(failed))
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

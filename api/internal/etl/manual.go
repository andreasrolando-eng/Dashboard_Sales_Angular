package etl

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"
)

// MaxManualDays caps one manual sync. Each day costs at least one ESB
// request, and the UI shows progress per day, so an unbounded range is
// neither useful nor kind to the ESB API.
const MaxManualDays = 62

// ErrBusy is returned by Start while another manual sync is still running.
var ErrBusy = errors.New("sinkron manual lain masih berjalan, tunggu sampai selesai")

// Job and day statuses.
const (
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFailed  = "failed"
	StatusPending = "pending"
)

// ManualMode selects what a manual sync does with bills that are already stored.
type ManualMode string

const (
	// ManualFill only inserts what is missing and never touches stored rows.
	ManualFill ManualMode = "fill"
	// ManualRefresh overwrites stored rows with what ESB returns now, so edits
	// made after the first pull (e.g. a void) show up. It reports what changed.
	ManualRefresh ManualMode = "refresh"
)

// ParseManualMode maps the API value to a mode; empty means ManualFill.
func ParseManualMode(s string) (ManualMode, error) {
	switch ManualMode(s) {
	case "", ManualFill:
		return ManualFill, nil
	case ManualRefresh:
		return ManualRefresh, nil
	}
	return "", fmt.Errorf("mode tidak dikenal %q (pakai \"fill\" atau \"refresh\")", s)
}

type ManualDay struct {
	Date   string         `json:"date"`
	Status string         `json:"status"` // pending | running | done | failed
	Result *DaySyncResult `json:"result,omitempty"`
}

// ManualJob is the state of one manual sync, as reported to the UI.
type ManualJob struct {
	ID         string      `json:"id"`
	Mode       ManualMode  `json:"mode"`
	Status     string      `json:"status"` // running | done | failed
	DateFrom   string      `json:"date_from"`
	DateTo     string      `json:"date_to"`
	Days       []ManualDay `json:"days"`
	StartedAt  time.Time   `json:"started_at"`
	FinishedAt *time.Time  `json:"finished_at,omitempty"`
	// Error is set when the whole job could not run (e.g. the nightly sync
	// held the lock); per-day errors live on Days[i].Result.Error.
	Error string `json:"error,omitempty"`
}

// ManualSyncer runs user-triggered syncs in the background, one at a time.
// Either way nothing is ever duplicated (every table is keyed on its natural
// ESB identifier). ManualFill only inserts what is missing and leaves stored
// rows untouched; ManualRefresh overwrites them with the latest ESB data.
type ManualSyncer struct {
	db     *gorm.DB
	client *Client
	ctx    context.Context
	now    func() time.Time

	mu  sync.Mutex
	job *ManualJob
}

func NewManualSyncer(db *gorm.DB, client *Client) *ManualSyncer {
	return &ManualSyncer{db: db, client: client, ctx: context.Background(), now: time.Now}
}

// ValidateManualRange checks a requested range against today (a WIB date).
func ValidateManualRange(from, to time.Time, today string) error {
	if from.After(to) {
		return errors.New("tanggal awal harus sebelum atau sama dengan tanggal akhir")
	}
	if to.Format("2006-01-02") > today {
		return errors.New("tanggal akhir tidak boleh melewati hari ini")
	}
	if days := int(to.Sub(from).Hours()/24) + 1; days > MaxManualDays {
		return fmt.Errorf("rentang maksimal %d hari per sinkron (diminta %d hari)", MaxManualDays, days)
	}
	return nil
}

// Start validates the range and launches the sync in the background. It
// returns ErrBusy if a manual sync is already running.
func (m *ManualSyncer) Start(from, to time.Time, mode ManualMode) (ManualJob, error) {
	if mode == "" {
		mode = ManualFill
	}
	if err := ValidateManualRange(from, to, m.now().In(WIB).Format("2006-01-02")); err != nil {
		return ManualJob{}, err
	}

	m.mu.Lock()
	if m.job != nil && m.job.Status == StatusRunning {
		m.mu.Unlock()
		return ManualJob{}, ErrBusy
	}
	job := &ManualJob{
		ID:        fmt.Sprintf("%d", m.now().UnixNano()),
		Mode:      mode,
		Status:    StatusRunning,
		DateFrom:  from.Format("2006-01-02"),
		DateTo:    to.Format("2006-01-02"),
		StartedAt: m.now(),
	}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		job.Days = append(job.Days, ManualDay{Date: d.Format("2006-01-02"), Status: StatusPending})
	}
	m.job = job
	snapshot := m.copyLocked()
	m.mu.Unlock()

	go m.run(job)
	return snapshot, nil
}

// Current returns a copy of the latest job (running or finished), or nil if
// none has been started since the process began.
func (m *ManualSyncer) Current() *ManualJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.job == nil {
		return nil
	}
	c := m.copyLocked()
	return &c
}

func (m *ManualSyncer) copyLocked() ManualJob {
	c := *m.job
	c.Days = make([]ManualDay, len(m.job.Days))
	for i, d := range m.job.Days {
		if d.Result != nil {
			r := *d.Result
			d.Result = &r
		}
		c.Days[i] = d
	}
	return c
}

func (m *ManualSyncer) update(fn func(j *ManualJob)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fn(m.job)
}

func (m *ManualSyncer) finish(status, errMsg string) {
	m.update(func(j *ManualJob) {
		now := m.now()
		j.Status, j.Error, j.FinishedAt = status, errMsg, &now
	})
}

func (m *ManualSyncer) run(job *ManualJob) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("manual sync panic: %v", r)
			m.finish(StatusFailed, fmt.Sprintf("kesalahan internal: %v", r))
		}
	}()

	okDays := 0
	acquired, err := WithSyncLock(m.db, func() error {
		for i := range job.Days {
			date := job.Days[i].Date
			m.update(func(j *ManualJob) { j.Days[i].Status = StatusRunning })

			opts := SyncOptions{Mode: ModeFillMissing, JobName: ManualJobName}
			if job.Mode == ManualRefresh {
				opts = SyncOptions{Mode: ModeUpsert, JobName: ManualRefreshJobName, ReportChanges: true}
			}
			res := SyncDayWith(m.ctx, m.db, m.client, date, opts)

			m.update(func(j *ManualJob) {
				j.Days[i].Result = &res
				if res.OK {
					j.Days[i].Status = StatusDone
				} else {
					j.Days[i].Status = StatusFailed
				}
			})
			if res.OK {
				okDays++
			}
		}
		return nil
	})

	switch {
	case err != nil:
		m.finish(StatusFailed, err.Error())
	case !acquired:
		m.finish(StatusFailed, "sinkron harian otomatis sedang berjalan, coba lagi beberapa menit lagi")
	case okDays == 0:
		m.finish(StatusFailed, "semua tanggal gagal disinkronkan")
	default:
		m.finish(StatusDone, "")
	}
}

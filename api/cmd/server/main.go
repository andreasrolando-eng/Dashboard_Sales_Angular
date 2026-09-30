package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/auth"
	"github.com/Operations-ESB/dashboard-sales/api/internal/config"
	"github.com/Operations-ESB/dashboard-sales/api/internal/db"
	"github.com/Operations-ESB/dashboard-sales/api/internal/etl"
	"github.com/Operations-ESB/dashboard-sales/api/internal/handler"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cfg := config.Load()

	switch os.Args[1] {
	case "serve":
		serve(cfg)
	case "migrate":
		if err := runMigrate(cfg, os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "seed":
		if err := runSeed(cfg); err != nil {
			log.Fatal(err)
		}
	case "user":
		if err := runUserCommand(cfg, os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "sync":
		if err := runSyncCommand(cfg, os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: server <serve|migrate|seed|sync|user> [args]")
	fmt.Fprintln(os.Stderr, "  server serve")
	fmt.Fprintln(os.Stderr, "  server migrate <up|down|status|has-pending>")
	fmt.Fprintln(os.Stderr, "  server seed   (dev-only sample data, truncates raw_* tables first)")
	fmt.Fprintln(os.Stderr, "  server user set-password --email=EMAIL [--admin]  (creates the user if missing; asks for the password)")
	fmt.Fprintln(os.Stderr, "  server sync run [--date=YYYY-MM-DD | --from=... --to=...]  (defaults to yesterday WIB)")
}

func serve(cfg config.Config) {
	gormDB, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}

	if cfg.BootstrapAdminEmail != "" || cfg.BootstrapAdminPassword != "" {
		msg, err := bootstrapAdmin(gormDB, cfg.BootstrapAdminEmail, cfg.BootstrapAdminPassword)
		if err != nil {
			// Not fatal: a typo here must not take the whole dashboard down.
			log.Printf("bootstrap admin: %v", err)
		} else {
			log.Printf("bootstrap admin: %s", msg)
		}
	}

	startSyncScheduler(cfg, gormDB)
	manualSyncer := newManualSyncer(cfg, gormDB)

	router := newRouter(cfg, gormDB, manualSyncer)

	addr := ":" + cfg.AppPort
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}

// startSyncScheduler launches the nightly ETL in the background when enabled.
func startSyncScheduler(cfg config.Config, gormDB *gorm.DB) {
	if !cfg.SyncSchedulerEnabled {
		log.Print("sync scheduler disabled (set SYNC_SCHEDULER_ENABLED=true to enable)")
		return
	}
	if cfg.ESBAPIBaseURL == "" || cfg.ESBAPIKey == "" {
		log.Print("sync scheduler NOT started: ESB_API_BASE_URL / ESB_API_KEY not set")
		return
	}
	s := &etl.Scheduler{
		DB:     gormDB,
		Client: etl.NewClient(cfg.ESBAPIBaseURL, cfg.ESBAPIKey),
		Cfg: etl.SchedulerConfig{
			Hour:            cfg.SyncHourWIB,
			CatchupDays:     cfg.SyncCatchupDays,
			AlertWebhookURL: cfg.SyncAlertWebhookURL,
			HealthchecksURL: cfg.HealthchecksPingURL,
		},
	}
	go s.Start(context.Background())
}

// newManualSyncer returns the runner behind POST /api/admin/sync, or nil when
// the ESB credentials are missing (the endpoint then answers 503).
func newManualSyncer(cfg config.Config, gormDB *gorm.DB) *etl.ManualSyncer {
	if cfg.ESBAPIBaseURL == "" || cfg.ESBAPIKey == "" {
		return nil
	}
	return etl.NewManualSyncer(gormDB, etl.NewClient(cfg.ESBAPIBaseURL, cfg.ESBAPIKey))
}

// newRouter builds the whole HTTP API. Only /healthz and POST /api/auth/login
// are public; everything else needs a signed-in user, and /api/admin/* needs
// an admin.
func newRouter(cfg config.Config, gormDB *gorm.DB, manualSyncer *etl.ManualSyncer) http.Handler {
	authSvc := auth.NewService(gormDB, time.Duration(cfg.SessionTTLHours)*time.Hour)
	cookies := auth.CookieConfig{Secure: cfg.CookieSecure, TTL: authSvc.TTL}

	r := chi.NewRouter()
	r.Use(middleware.RealIP) // real client address behind the nginx reverse proxy
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", handler.Healthz)

	r.Route("/api", func(r chi.Router) {
		// Responses carry account and sales data: never let a browser or proxy cache them.
		r.Use(noStore)

		// Public: only the sign-in form itself.
		r.Post("/auth/login", handler.Login(authSvc, cookies))

		// Everything else needs a signed-in user.
		r.Group(func(r chi.Router) {
			r.Use(auth.Require(authSvc))
			r.Get("/me", handler.Me())
			r.Post("/auth/logout", handler.Logout(authSvc, cookies))
			r.Post("/auth/password", handler.ChangeOwnPassword(authSvc))

			r.Route("/meta", func(r chi.Router) {
				r.Get("/outlets", handler.ListOutlets(gormDB))
				r.Get("/categories", handler.ListCategories(gormDB))
				r.Get("/category-details", handler.ListCategoryDetails(gormDB))
				r.Get("/last-sync", handler.GetLastSync(gormDB))
			})
			r.Route("/sales", func(r chi.Router) {
				r.Get("/summary", handler.SalesSummary(gormDB))
				r.Get("/daily", handler.SalesDaily(gormDB))
				r.Get("/hourly", handler.SalesHourly(gormDB))
				r.Get("/revenue-by-outlet", handler.RevenueByOutlet(gormDB))
				r.Get("/revenue-by-category", handler.RevenueByCategory(gormDB))
				r.Get("/top-products", handler.TopProducts(gormDB))
				r.Get("/menu-performance", handler.MenuPerformance(gormDB))
				r.Get("/bills", handler.SalesBills(gormDB))
				r.Get("/bills/export", handler.SalesBillsExport(gormDB))
			})
			r.Route("/ops", func(r chi.Router) {
				r.Get("/summary", handler.OpsSummary(gormDB))
			})
			r.Route("/membership", func(r chi.Router) {
				r.Get("/summary", handler.MembershipSummary(gormDB))
				r.Get("/top-members", handler.TopMembers(gormDB))
				r.Get("/member-options", handler.MemberOptions(gormDB))
				r.Get("/members/{memberCode}/menu-purchases", handler.MemberMenuPurchases(gormDB))
				r.Get("/new-weekly", handler.MembershipNewWeekly(gormDB))
			})
			r.Route("/marketing", func(r chi.Router) {
				r.Get("/promo-performance", handler.PromoPerformance(gormDB))
			})

			// Admin only: user management and manual sync.
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireAdmin)
				r.Route("/admin/users", func(r chi.Router) {
					r.Get("/", handler.ListUsers(gormDB))
					r.Post("/", handler.AddUser(gormDB, authSvc))
					r.Post("/{email}/password", handler.SetUserPassword(authSvc))
					r.Delete("/{email}", handler.RemoveUser(gormDB))
				})
				r.Route("/admin/sync", func(r chi.Router) {
					r.Get("/", handler.GetManualSync(manualSyncer))
					r.Post("/", handler.StartManualSync(manualSyncer))
					r.Get("/logs", handler.ListSyncLogs(gormDB))
				})
			})
		})
	})

	// Single-container deploy: everything that is not /api or /healthz is the
	// Angular app (same origin, so the session cookie just works).
	if cfg.StaticDir != "" {
		r.Handle("/*", handler.SPA(cfg.StaticDir))
	}
	return r
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, req)
	})
}

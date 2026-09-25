package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	msqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/liftedkilt/openchore/internal/api"
	"github.com/liftedkilt/openchore/internal/config"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/internal/tts"
	"github.com/liftedkilt/openchore/internal/webhook"
	"github.com/liftedkilt/openchore/migrations"
)

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "openchore.db"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	db, err := sql.Open("sqlite", dbPath+"?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	if err := runMigrations(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	s := store.New(db)

	// Load and apply config file (only populates an empty database)
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config/config.yaml"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	if cfg != nil {
		if err := config.Apply(context.Background(), s, cfg); err != nil {
			log.Fatalf("failed to apply config: %v", err)
		}
	}

	if err := api.RetireLegacyPasscodeIfUnused(context.Background(), s); err != nil {
		log.Printf("auth: could not check legacy admin passcode: %v", err)
	}

	dispatcher := webhook.NewDispatcher(s)

	// Start background checkers
	expiryChecker := webhook.NewExpiryChecker(s, dispatcher)
	go expiryChecker.Start(context.Background())
	decayChecker := webhook.NewDecayChecker(s, dispatcher)
	go decayChecker.Start(context.Background())
	pointsDecayChecker := webhook.NewPointsDecayChecker(s, dispatcher)
	// POINTS_DECAY_INTERVAL shortens the decay cadence. The e2e suite sets it
	// so a decay is observable within a test run instead of 15 minutes later.
	if v := os.Getenv("POINTS_DECAY_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			log.Printf("ignoring invalid POINTS_DECAY_INTERVAL %q", v)
		} else {
			pointsDecayChecker.SetInterval(d)
			log.Printf("points decay interval overridden to %s", d)
		}
	}
	go pointsDecayChecker.Start(context.Background())

	// Webhook delivery log cleanup (issue #18): bounded retention for webhook_deliveries.
	deliveryCleaner := webhook.NewDeliveryCleaner(
		s,
		cfg.WebhookRetentionDays(),
		cfg.WebhookCleanupIntervalHours(),
	)
	go deliveryCleaner.Start(context.Background())

	authCfg, err := config.ResolveAuth(cfg)
	if err != nil {
		log.Fatalf("invalid auth config: %v", err)
	}
	secret, err := api.LoadSessionSecret(context.Background(), s, os.Getenv("OPENCHORE_SESSION_SECRET"))
	if err != nil {
		log.Fatalf("failed to load session secret: %v", err)
	}
	sessions := api.NewSessionManager(secret)
	oidcSvc := api.NewOIDCService(s, sessions, dispatcher, authCfg)
	for _, p := range authCfg.OIDC {
		log.Printf("auth: OIDC provider %q (%s) enabled", p.ID, p.Issuer)
	}
	// Adds providers and session lengths saved under Manage → Settings.
	if err := oidcSvc.Reload(context.Background()); err != nil {
		log.Fatalf("failed to load sign-in settings: %v", err)
	}

	router, choreHandler, reportsHandler := api.NewRouter(s, dispatcher, api.Auth{Sessions: sessions, OIDC: oidcSvc})

	// Optional AI and read-aloud audio: off unless a base URL is set, either
	// in the environment or under Manage → Settings → AI.
	for _, legacy := range []string{"AI_ENDPOINT", "OLLAMA_ENDPOINT", "TTS_ENDPOINT"} {
		if os.Getenv(legacy) != "" {
			log.Printf("WARNING: %s is no longer used; set AI_BASE_URL / TTS_BASE_URL (with the /v1 suffix) instead — see docs/ai.md", legacy)
		}
	}
	aiServices := choreHandler.AIServices()
	aiServices.Reload(context.Background())
	go func() {
		tts.CleanOrphans(context.Background(), s)
		if audio := aiServices.Audio(); audio != nil {
			audio.Sync(context.Background(), false)
		}
	}()
	go reportsHandler.StartWeeklySummaries(context.Background())

	log.Printf("starting server on :%s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func runMigrations(db *sql.DB) error {
	driver, err := msqlite.WithInstance(db, &msqlite.Config{})
	if err != nil {
		return fmt.Errorf("creating migration driver: %w", err)
	}

	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}

	log.Println("migrations complete")
	return nil
}

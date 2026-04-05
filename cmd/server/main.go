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
	"github.com/liftedkilt/openchore/internal/ai"
	"github.com/liftedkilt/openchore/internal/api"
	"github.com/liftedkilt/openchore/internal/config"
	"github.com/liftedkilt/openchore/internal/aibackend"
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

	dispatcher := webhook.NewDispatcher(s)

	// Start background checkers
	expiryChecker := webhook.NewExpiryChecker(s, dispatcher)
	go expiryChecker.Start(context.Background())
	decayChecker := webhook.NewDecayChecker(s, dispatcher)
	go decayChecker.Start(context.Background())

	router, choreHandler, reportsHandler := api.NewRouter(s, dispatcher)

	// Initialize optional AI services in background (waits for sidecars to become ready)
	go initAIServices(s, choreHandler, reportsHandler)

	log.Printf("starting server on :%s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func initAIServices(s *store.Store, choreHandler *api.ChoreHandler, reportsHandler *api.ReportsHandler) {
	aiEndpoint := os.Getenv("AI_ENDPOINT")
	if aiEndpoint == "" {
		aiEndpoint = os.Getenv("OLLAMA_ENDPOINT") // backward compat
	}
	if aiEndpoint == "" {
		aiEndpoint = "http://litert:8080"
	}

	ttsEndpoint := os.Getenv("TTS_ENDPOINT")
	if ttsEndpoint == "" {
		ttsEndpoint = "http://kokoro:8880"
	}

	aiClient := aibackend.NewClient(aiEndpoint)

	// Wait for the AI endpoint to become available (retry every 5s for up to 2 minutes)
	log.Printf("Waiting for AI endpoint at %s...", aiEndpoint)
	for attempt := 1; attempt <= 24; attempt++ {
		if aiClient.Healthy(context.Background()) {
			break
		}
		if attempt == 24 {
			log.Printf("AI endpoint not available at %s after 2 minutes — AI features disabled", aiEndpoint)
			return
		}
		time.Sleep(5 * time.Second)
	}

	aiModel := os.Getenv("AI_MODEL")
	if aiModel == "" {
		aiModel = "gemma4:e4b"
	}

	// Auto-pull model if not present
	if !aiClient.HasModel(context.Background(), aiModel) {
		log.Printf("Model %s not found — pulling (this may take a few minutes on first run)...", aiModel)
		if err := aiClient.Pull(context.Background(), aiModel); err != nil {
			log.Printf("WARNING: failed to pull model %s: %v — AI features disabled", aiModel, err)
			return
		}
		log.Printf("Model %s pulled successfully", aiModel)
	}

	reviewer := ai.NewReviewer(aiClient, aiModel)

	// Wait for TTS sidecar (retry every 5s for up to 30s)
	var ttsClient *tts.Client
	ttsC := tts.NewClient(ttsEndpoint)
	log.Printf("Checking for TTS service at %s...", ttsEndpoint)
	for attempt := 1; attempt <= 6; attempt++ {
		if ttsC.Healthy(context.Background()) {
			ttsClient = ttsC
			log.Printf("TTS audio service available at %s", ttsEndpoint)
			break
		}
		if attempt == 6 {
			log.Printf("TTS audio service not available at %s — will retry lazily on first use", ttsEndpoint)
			break
		}
		time.Sleep(5 * time.Second)
	}

	ttsVoice, _ := s.GetSetting(context.Background(), "ai_tts_voice")
	if ttsVoice == "" {
		ttsVoice = "af_heart"
	}
	ttsGen := ai.NewTTSGenerator(aiClient, aiModel, ttsClient, ttsEndpoint, ttsVoice)

	descGen := ai.NewDescriptionGenerator(aiClient, aiModel)
	summarizer := ai.NewSummarizer(aiClient, aiModel)

	choreHandler.SetAIServices(reviewer, ttsGen)
	choreHandler.SetAIExtras(descGen, summarizer)
	reportsHandler.SetSummarizer(summarizer)
	log.Printf("AI services initialized (%s at %s, model=%s)", aiClient.ServerType(context.Background()), aiEndpoint, aiModel)

	// Start TTS sync loop — generates audio for all chores, cleans up orphans
	syncer := ai.NewTTSSyncer(s, ttsGen)
	go syncer.Start(context.Background(), 5*time.Minute)
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

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/golang-migrate/migrate/v4"
	msqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/liftedkilt/openchore/internal/ai"
	"github.com/liftedkilt/openchore/internal/api"
	"github.com/liftedkilt/openchore/internal/config"
	"github.com/liftedkilt/openchore/internal/ollama"
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

	// Initialize optional AI services (LiteRT or Ollama for vision/text, Kokoro for TTS audio)
	var reviewer *ai.Reviewer
	var ttsGen *ai.TTSGenerator
	ollamaEndpoint := os.Getenv("OLLAMA_ENDPOINT")
	if ollamaEndpoint == "" {
		if ep, _ := s.GetSetting(context.Background(), "ai_endpoint"); ep != "" {
			ollamaEndpoint = ep
		} else {
			ollamaEndpoint = "http://litert:8080"
		}
	}
	ollamaClient := ollama.NewClient(ollamaEndpoint)
	if ollamaClient.Healthy(context.Background()) {
		aiModel, _ := s.GetSetting(context.Background(), "ai_model")
		if aiModel == "" {
			aiModel = "gemma4:e2b"
		}

		// Auto-pull model if not present (runs in background so server starts immediately)
		if !ollamaClient.HasModel(context.Background(), aiModel) {
			log.Printf("Model %s not found — pulling in background (this may take a few minutes on first run)...", aiModel)
			go func() {
				if err := ollamaClient.Pull(context.Background(), aiModel); err != nil {
					log.Printf("WARNING: failed to pull model %s: %v", aiModel, err)
					log.Printf("AI features will not work until the model is available")
				} else {
					log.Printf("Model %s pulled successfully — AI features are now ready", aiModel)
				}
			}()
		}

		reviewer = ai.NewReviewer(ollamaClient, aiModel)

		// Initialize TTS: LLM for text descriptions, Kokoro for audio synthesis
		var ttsClient *tts.Client
		ttsEndpoint := os.Getenv("TTS_ENDPOINT")
		if ttsEndpoint == "" {
			if ep, _ := s.GetSetting(context.Background(), "ai_tts_endpoint"); ep != "" {
				ttsEndpoint = ep
			} else {
				ttsEndpoint = "http://kokoro:8880"
			}
		}
		ttsC := tts.NewClient(ttsEndpoint)
		if ttsC.Healthy(context.Background()) {
			ttsClient = ttsC
			log.Printf("TTS audio service available at %s", ttsEndpoint)
		} else {
			log.Printf("TTS audio service not available at %s — browser TTS will be used as fallback", ttsEndpoint)
		}

		ttsVoice, _ := s.GetSetting(context.Background(), "ai_tts_voice")
		if ttsVoice == "" {
			ttsVoice = "af_heart"
		}
		ttsGen = ai.NewTTSGenerator(ollamaClient, aiModel, ttsClient, ttsVoice)

		log.Printf("AI services initialized (endpoint=%s, model=%s)", ollamaEndpoint, aiModel)
	} else {
		log.Printf("AI endpoint not available at %s — AI features disabled", ollamaEndpoint)
	}

	router := api.NewRouter(s, dispatcher, reviewer, ttsGen)

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

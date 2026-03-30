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
	"github.com/liftedkilt/openchore/internal/api"
	"github.com/liftedkilt/openchore/internal/config"
	"github.com/liftedkilt/openchore/internal/store"
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

	router := api.NewRouter(s, dispatcher)

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

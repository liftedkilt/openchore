// Command hub runs the friend-families coordination server (internal/hub):
// the one piece that has to be reachable from the internet so that the
// OpenChore instances themselves don't.
//
//	PORT       listen port (default 8090)
//	HUB_DB     SQLite file (default hub.db)
//	HUB_KEYS   comma-separated keys allowed to create links
//	HUB_OPEN   "true" to let anyone create links (no HUB_KEYS needed)
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/liftedkilt/openchore/internal/hub"
	_ "modernc.org/sqlite"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	var keys []string
	for _, k := range strings.Split(os.Getenv("HUB_KEYS"), ",") {
		if k = strings.TrimSpace(k); k != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 && os.Getenv("HUB_OPEN") != "true" {
		log.Fatal("set HUB_KEYS (keys allowed to create links), or HUB_OPEN=true to let anyone create them")
	}

	db, err := sql.Open("sqlite", env("HUB_DB", "hub.db")+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	db.SetMaxOpenConns(1)
	h, err := hub.New(db, keys)
	if err != nil {
		log.Fatalf("preparing database: %v", err)
	}
	go func() {
		for range time.Tick(time.Hour) {
			if err := h.Sweep(context.Background()); err != nil {
				log.Printf("sweep: %v", err)
			}
		}
	}()

	addr := ":" + env("PORT", "8090")
	log.Printf("openchore hub listening on %s (%d key(s))", addr, len(keys))
	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

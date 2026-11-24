// Command hub runs the friend-families coordination server (internal/hub):
// the one piece that has to be reachable from the internet so that the
// OpenChore instances themselves don't.
//
//	PORT             listen port (default 8090)
//	HUB_DB           SQLite file (default hub.db)
//	HUB_KEYS         comma-separated keys allowed to create links
//	HUB_KEY_FILE     file holding one such key; made with a random key on
//	                 first start if it doesn't exist (default hub.key next
//	                 to HUB_DB, used when HUB_KEYS is empty)
//	HUB_OPEN         "true" to let anyone create links (no key needed)
//	HUB_TRUST_PROXY  "true" behind one reverse proxy: rate limits then go by
//	                 the last X-Forwarded-For address
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
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

// keyFromFile reads the hub's key file, creating it with a fresh random key
// the first time. The key is never logged: the operator reads the file.
func keyFromFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err == nil {
		if k := strings.TrimSpace(string(raw)); k != "" {
			return k, nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	k := base64.RawURLEncoding.EncodeToString(b)
	if err := os.WriteFile(path, []byte(k+"\n"), 0o600); err != nil {
		return "", err
	}
	log.Printf("made a new key for creating links; read it from %s", path)
	return k, nil
}

func main() {
	dbPath := env("HUB_DB", "hub.db")
	var keys []string
	for _, k := range strings.Split(os.Getenv("HUB_KEYS"), ",") {
		if k = strings.TrimSpace(k); k != "" {
			keys = append(keys, k)
		}
	}
	open := os.Getenv("HUB_OPEN") == "true"
	if len(keys) == 0 && !open {
		k, err := keyFromFile(env("HUB_KEY_FILE", filepath.Join(filepath.Dir(dbPath), "hub.key")))
		if err != nil {
			log.Fatalf("key file: %v", err)
		}
		keys = append(keys, k)
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	db.SetMaxOpenConns(1)
	h, err := hub.New(db, keys)
	if err != nil {
		log.Fatalf("preparing database: %v", err)
	}
	h.TrustProxy = os.Getenv("HUB_TRUST_PROXY") == "true"
	h.Log = log.Default()
	go func() {
		for range time.Tick(time.Hour) {
			if err := h.Sweep(context.Background()); err != nil {
				log.Printf("sweep: %v", err)
			}
		}
	}()

	addr := ":" + env("PORT", "8090")
	if open {
		log.Printf("openchore hub listening on %s; anyone can create links (HUB_OPEN)", addr)
	} else {
		log.Printf("openchore hub listening on %s (%d key(s))", addr, len(keys))
	}
	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	log.Fatal(srv.ListenAndServe())
}

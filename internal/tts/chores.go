package tts

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

// Dir is where chore audio lives; the router serves it at /tts/.
const Dir = "data/tts"

// ChoreAudio keeps one MP3 per chore that reads its title and description
// aloud. Audio is made when a chore is saved, not by a polling loop.
type ChoreAudio struct {
	client *Client
	store  *store.Store
	mu     sync.Mutex // one synthesis at a time; also guards file writes
}

// NewChoreAudio returns nil when client is nil so callers can treat
// "not configured" and "no audio manager" the same way.
func NewChoreAudio(client *Client, s *store.Store) *ChoreAudio {
	if client == nil {
		return nil
	}
	return &ChoreAudio{client: client, store: s}
}

// Client returns the underlying speech client.
func (a *ChoreAudio) Client() *Client { return a.client }

// SpokenText is what gets read aloud for a chore.
func SpokenText(c *model.Chore) string {
	text := strings.TrimSpace(c.Title)
	if d := strings.TrimSpace(c.Description); d != "" {
		if !strings.HasSuffix(text, ".") && !strings.HasSuffix(text, "!") && !strings.HasSuffix(text, "?") {
			text += "."
		}
		text += " " + d
	}
	return text
}

func (a *ChoreAudio) voice(ctx context.Context) string {
	if v, _ := a.store.GetSetting(ctx, "tts_voice"); v != "" {
		return v
	}
	return DefaultVoice
}

// Generate synthesizes and saves audio for one chore and records its URL.
// The URL carries a version so browsers don't replay stale audio.
func (a *ChoreAudio) Generate(ctx context.Context, c *model.Chore) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	audio, err := a.client.Synthesize(ctx, SpokenText(c), a.voice(ctx))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(Dir, 0750); err != nil {
		return "", fmt.Errorf("creating TTS directory: %w", err)
	}
	name := fmt.Sprintf("chore_%d.mp3", c.ID)
	if err := os.WriteFile(filepath.Join(Dir, name), audio, 0640); err != nil {
		return "", fmt.Errorf("saving TTS file: %w", err)
	}
	url := fmt.Sprintf("/tts/%s?v=%d", name, time.Now().UnixMilli())
	if err := a.store.UpdateChoreTTSAudioURL(ctx, c.ID, url); err != nil {
		return "", err
	}
	c.TTSAudioURL = url
	return url, nil
}

// GenerateAsync is Generate in the background, for request handlers.
func (a *ChoreAudio) GenerateAsync(c model.Chore) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if _, err := a.Generate(ctx, &c); err != nil {
			log.Printf("tts: audio for chore %d failed: %v", c.ID, err)
		}
	}()
}

// Remove deletes a chore's audio file.
func Remove(choreID int64) {
	path := filepath.Join(Dir, fmt.Sprintf("chore_%d.mp3", choreID))
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("tts: removing %s: %v", path, err)
	}
}

// Sync generates audio for chores that have none, or for every chore when
// all is true (after a voice change). It stops early if ctx is cancelled.
func (a *ChoreAudio) Sync(ctx context.Context, all bool) {
	chores, err := a.store.ListChores(ctx)
	if err != nil {
		log.Printf("tts: listing chores: %v", err)
		return
	}
	made := 0
	for i := range chores {
		if ctx.Err() != nil {
			return
		}
		if !all && chores[i].TTSAudioURL != "" {
			continue
		}
		if _, err := a.Generate(ctx, &chores[i]); err != nil {
			log.Printf("tts: audio for chore %d failed: %v", chores[i].ID, err)
			continue
		}
		made++
	}
	if made > 0 {
		log.Printf("tts: generated audio for %d chores", made)
	}
}

// CleanOrphans deletes audio files no chore points at, including leftovers
// from older versions (feedback clips, audio for deleted chores). It runs
// whether or not TTS is configured.
func CleanOrphans(ctx context.Context, s *store.Store) {
	entries, err := os.ReadDir(Dir)
	if err != nil {
		return
	}
	chores, err := s.ListChores(ctx)
	if err != nil {
		log.Printf("tts: listing chores: %v", err)
		return
	}
	keep := make(map[string]bool, len(chores))
	for _, c := range chores {
		if c.TTSAudioURL != "" {
			name, _, _ := strings.Cut(strings.TrimPrefix(c.TTSAudioURL, "/tts/"), "?")
			keep[name] = true
		}
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || keep[e.Name()] {
			continue
		}
		if err := os.Remove(filepath.Join(Dir, e.Name())); err == nil {
			removed++
		}
	}
	if removed > 0 {
		log.Printf("tts: removed %d unused audio files", removed)
	}
}

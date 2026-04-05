package ai

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/liftedkilt/openchore/internal/store"
)

// TTSSyncer periodically ensures all chores have TTS descriptions and audio files,
// and cleans up orphaned audio from deleted chores.
type TTSSyncer struct {
	store   *store.Store
	ttsGen  *TTSGenerator
	trigger chan struct{}
}

// NewTTSSyncer creates a new TTS syncer.
func NewTTSSyncer(s *store.Store, ttsGen *TTSGenerator) *TTSSyncer {
	return &TTSSyncer{store: s, ttsGen: ttsGen, trigger: make(chan struct{}, 1)}
}

// Trigger requests an immediate sync. Non-blocking; if a sync is already
// pending the request is coalesced.
func (s *TTSSyncer) Trigger() {
	select {
	case s.trigger <- struct{}{}:
	default: // already pending
	}
}

// Start runs the sync loop. It checks every interval and generates missing TTS.
// Call this in a goroutine.
func (s *TTSSyncer) Start(ctx context.Context, interval time.Duration) {
	// Run immediately on start, then on interval
	s.sync(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sync(ctx)
		case <-s.trigger:
			s.sync(ctx)
		}
	}
}

func (s *TTSSyncer) sync(ctx context.Context) {
	ttsEnabled, _ := s.store.GetSetting(ctx, "ai_tts_enabled")
	if ttsEnabled != "true" {
		return
	}

	if s.ttsGen == nil || !s.ttsGen.TTSAvailable() {
		return
	}

	chores, err := s.store.ListChores(ctx)
	if err != nil {
		log.Printf("tts-sync: failed to list chores: %v", err)
		return
	}

	dir := ttsDir
	_ = os.MkdirAll(dir, 0750)

	// Track which chore IDs are valid (for cleanup)
	validFiles := make(map[string]bool)

	for _, chore := range chores {
		audioFile := fmt.Sprintf("chore_%d.mp3", chore.ID)
		audioPath := filepath.Join(dir, audioFile)
		audioURL := "/tts/" + audioFile
		validFiles[audioFile] = true

		// Skip if audio file already exists
		if _, err := os.Stat(audioPath); err == nil {
			// File exists — make sure DB has the URL
			if chore.TTSAudioURL != audioURL {
				_ = s.store.UpdateChoreTTSAudioURL(ctx, chore.ID, audioURL)
			}
			continue
		}

		// Generate TTS description if missing
		desc := chore.TTSDescription
		if desc == "" {
			log.Printf("tts-sync: generating description for chore %d (%s)", chore.ID, chore.Title)
			desc, err = s.ttsGen.GenerateDescription(ctx, chore.Title, chore.Description)
			if err != nil {
				log.Printf("tts-sync: failed to generate description for chore %d: %v", chore.ID, err)
				continue
			}
			_ = s.store.UpdateChoreTTSDescription(ctx, chore.ID, desc)
		}

		// Synthesize audio
		log.Printf("tts-sync: synthesizing audio for chore %d (%s)", chore.ID, chore.Title)
		audio, err := s.ttsGen.Synthesize(ctx, desc, s.ttsGen.ttsVoice)
		if err != nil {
			log.Printf("tts-sync: failed to synthesize audio for chore %d: %v", chore.ID, err)
			continue
		}

		if err := os.WriteFile(audioPath, audio, 0640); err != nil {
			log.Printf("tts-sync: failed to write audio for chore %d: %v", chore.ID, err)
			continue
		}

		_ = s.store.UpdateChoreTTSAudioURL(ctx, chore.ID, audioURL)
		log.Printf("tts-sync: chore %d (%s) — %d bytes", chore.ID, chore.Title, len(audio))
	}

	// Clean up orphaned files
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".mp3") {
			continue
		}

		// Keep valid chore audio files
		if validFiles[name] {
			continue
		}

		// Keep feedback files that reference existing completions
		if strings.HasPrefix(name, "feedback_") {
			// Extract completion ID: feedback_{id}_{timestamp}.mp3
			parts := strings.SplitN(strings.TrimPrefix(name, "feedback_"), "_", 2)
			if len(parts) >= 1 {
				if id, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
					if comp, _ := s.store.GetCompletion(ctx, id); comp != nil {
						continue // Completion still exists, keep the file
					}
				}
			}
		}

		// Orphaned file — remove it
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err == nil {
			log.Printf("tts-sync: cleaned up orphaned file %s", name)
		}
	}
}

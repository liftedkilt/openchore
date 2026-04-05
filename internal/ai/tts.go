package ai

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/liftedkilt/openchore/internal/aibackend"
	"github.com/liftedkilt/openchore/internal/tts"
)

const ttsDir = "data/tts"

func saveTTSFile(filename string, audio []byte) (string, error) {
	if err := os.MkdirAll(ttsDir, 0750); err != nil {
		return "", fmt.Errorf("creating TTS directory: %w", err)
	}
	filePath := filepath.Join(ttsDir, filename)
	if err := os.WriteFile(filePath, audio, 0640); err != nil {
		return "", fmt.Errorf("saving TTS file: %w", err)
	}
	return "/tts/" + filename, nil
}

// TTSGenerator uses an LLM to create kid-friendly spoken descriptions for chores,
// and optionally a TTS service to synthesize audio files.
type TTSGenerator struct {
	mu          sync.Mutex
	llmClient   *aibackend.Client
	llmModel    string
	ttsClient   *tts.Client // nil if TTS audio synthesis is not available
	ttsEndpoint string      // stored for lazy reconnection
	ttsVoice    string
}

// NewTTSGenerator creates a new TTS description generator.
// ttsClient and ttsVoice are optional — pass nil/"" to skip audio synthesis.
// ttsEndpoint is stored for lazy reconnection if the TTS service starts later.
func NewTTSGenerator(llmClient *aibackend.Client, llmModel string, ttsClient *tts.Client, ttsEndpoint, ttsVoice string) *TTSGenerator {
	return &TTSGenerator{
		llmClient:   llmClient,
		llmModel:    llmModel,
		ttsClient:   ttsClient,
		ttsEndpoint: ttsEndpoint,
		ttsVoice:    ttsVoice,
	}
}

// getTTSClient returns the TTS client, attempting lazy reconnection if nil.
func (t *TTSGenerator) getTTSClient() *tts.Client {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ttsClient != nil {
		return t.ttsClient
	}
	if t.ttsEndpoint == "" {
		return nil
	}
	c := tts.NewClient(t.ttsEndpoint)
	if c.Healthy(context.Background()) {
		t.ttsClient = c
		log.Printf("ai: TTS service now available at %s (lazy reconnect)", t.ttsEndpoint)
		return c
	}
	return nil
}

// TTSAvailable reports whether TTS audio synthesis is configured.
func (t *TTSGenerator) TTSAvailable() bool {
	return t.getTTSClient() != nil
}

// Synthesize sends text to the TTS service and returns the resulting audio bytes.
// Returns nil, nil if TTS is not available.
func (t *TTSGenerator) Synthesize(ctx context.Context, text, voice string) ([]byte, error) {
	client := t.getTTSClient()
	if client == nil {
		return nil, nil
	}
	return client.Synthesize(ctx, text, voice)
}

// GenerateDescription creates a natural, kid-friendly spoken description for a chore.
// This text is designed to be read aloud by browser text-to-speech or a TTS service.
func (t *TTSGenerator) GenerateDescription(ctx context.Context, choreTitle, choreDescription string) (string, error) {
	prompt := fmt.Sprintf(`Create a short, friendly spoken description of this chore for a child. The description should sound natural when read aloud by a text-to-speech system.

Chore title: %s
Chore description: %s

Rules:
- Keep it to 1-2 sentences
- Use simple words a 6-year-old would understand
- Be encouraging and positive
- Don't use emoji or special characters
- Respond with ONLY the description text, nothing else`, choreTitle, choreDescription)

	resp, err := t.llmClient.Generate(ctx, &aibackend.GenerateRequest{
		Model:  t.llmModel,
		Prompt: prompt,
		Options: &aibackend.ModelOptions{
			Temperature: 0.7,
			NumPredict:  1024, // needs headroom for model thinking tokens
		},
	})
	if err != nil {
		return "", fmt.Errorf("generating TTS description: %w", err)
	}

	desc := strings.TrimSpace(resp.Response)
	desc = strings.Trim(desc, "\"'")
	log.Printf("ai: generated TTS description for %q: %s", choreTitle, desc)
	return desc, nil
}

// SynthesizeAudio sends text to the TTS service and saves the resulting audio file.
// Returns the relative URL to the saved audio file (e.g. "/tts/chore_42.mp3").
// Returns "" if TTS audio is not available.
func (t *TTSGenerator) SynthesizeAudio(ctx context.Context, text string, choreID int64) (string, error) {
	client := t.getTTSClient()
	if client == nil {
		return "", nil
	}

	audio, err := client.Synthesize(ctx, text, t.ttsVoice)
	if err != nil {
		return "", fmt.Errorf("synthesizing audio: %w", err)
	}

	filename := fmt.Sprintf("chore_%d.mp3", choreID)
	url, err := saveTTSFile(filename, audio)
	if err != nil {
		return "", err
	}

	log.Printf("ai: synthesized TTS audio for chore %d: %s (%d bytes)", choreID, url, len(audio))
	return url, nil
}

// GenerateAndSynthesize creates a TTS description and optionally synthesizes audio.
// Returns (description text, audio URL, error).
func (t *TTSGenerator) GenerateAndSynthesize(ctx context.Context, choreTitle, choreDescription string, choreID int64) (string, string, error) {
	desc, err := t.GenerateDescription(ctx, choreTitle, choreDescription)
	if err != nil {
		return "", "", err
	}

	audioURL, err := t.SynthesizeAudio(ctx, desc, choreID)
	if err != nil {
		// Audio synthesis failed — log but don't fail the whole operation
		log.Printf("ai: audio synthesis failed for chore %d (text description still saved): %v", choreID, err)
		return desc, "", nil
	}

	return desc, audioURL, nil
}

// SynthesizeFeedback synthesizes AI review feedback as audio for a child.
// Returns the relative URL to the saved audio file, or "" if unavailable.
func (t *TTSGenerator) SynthesizeFeedback(ctx context.Context, feedback string, completionID int64) (string, error) {
	client := t.getTTSClient()
	if client == nil {
		return "", nil
	}

	audio, err := client.Synthesize(ctx, feedback, t.ttsVoice)
	if err != nil {
		return "", fmt.Errorf("synthesizing feedback audio: %w", err)
	}

	filename := fmt.Sprintf("feedback_%d_%d.mp3", completionID, time.Now().UnixNano())
	url, err := saveTTSFile(filename, audio)
	if err != nil {
		return "", err
	}

	log.Printf("ai: synthesized feedback audio: %s (%d bytes)", url, len(audio))
	return url, nil
}

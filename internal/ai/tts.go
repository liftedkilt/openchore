package ai

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liftedkilt/openchore/internal/ollama"
	"github.com/liftedkilt/openchore/internal/tts"
)

// TTSGenerator uses an LLM to create kid-friendly spoken descriptions for chores,
// and optionally a TTS service to synthesize audio files.
type TTSGenerator struct {
	llmClient *ollama.Client
	llmModel  string
	ttsClient *tts.Client // nil if TTS audio synthesis is not available
	ttsVoice  string
}

// NewTTSGenerator creates a new TTS description generator.
// ttsClient and ttsVoice are optional — pass nil/"" to skip audio synthesis.
func NewTTSGenerator(llmClient *ollama.Client, llmModel string, ttsClient *tts.Client, ttsVoice string) *TTSGenerator {
	return &TTSGenerator{
		llmClient: llmClient,
		llmModel:  llmModel,
		ttsClient: ttsClient,
		ttsVoice:  ttsVoice,
	}
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

	resp, err := t.llmClient.Generate(ctx, &ollama.GenerateRequest{
		Model:  t.llmModel,
		Prompt: prompt,
		Options: &ollama.ModelOptions{
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
// Returns the relative URL to the saved audio file (e.g. "/tts/chore_42.wav").
// Returns "" if TTS audio is not available.
func (t *TTSGenerator) SynthesizeAudio(ctx context.Context, text string, choreID int64) (string, error) {
	if t.ttsClient == nil {
		return "", nil
	}

	audio, err := t.ttsClient.Synthesize(ctx, text, t.ttsVoice)
	if err != nil {
		return "", fmt.Errorf("synthesizing audio: %w", err)
	}

	// Save to data/tts/ directory
	dir := "data/tts"
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", fmt.Errorf("creating TTS directory: %w", err)
	}

	filename := fmt.Sprintf("chore_%d_%d.mp3", choreID, time.Now().UnixNano())
	filePath := filepath.Join(dir, filename)
	if err := os.WriteFile(filePath, audio, 0640); err != nil {
		return "", fmt.Errorf("saving TTS audio: %w", err)
	}

	url := "/tts/" + filename
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
	if t.ttsClient == nil {
		return "", nil
	}

	audio, err := t.ttsClient.Synthesize(ctx, feedback, t.ttsVoice)
	if err != nil {
		return "", fmt.Errorf("synthesizing feedback audio: %w", err)
	}

	dir := "data/tts"
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", fmt.Errorf("creating TTS directory: %w", err)
	}

	filename := fmt.Sprintf("feedback_%d_%d.mp3", completionID, time.Now().UnixNano())
	filePath := filepath.Join(dir, filename)
	if err := os.WriteFile(filePath, audio, 0640); err != nil {
		return "", fmt.Errorf("saving feedback audio: %w", err)
	}

	url := "/tts/" + filename
	log.Printf("ai: synthesized feedback audio: %s (%d bytes)", url, len(audio))
	return url, nil
}

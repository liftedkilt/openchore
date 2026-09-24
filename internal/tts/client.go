// Package tts synthesizes chore read-aloud audio through any OpenAI-compatible
// speech endpoint (Kokoro-FastAPI, OpenAI, ...).
package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultVoice suits Kokoro, the self-hosted server the compose file ships.
const DefaultVoice = "af_heart"

// Client is an HTTP client for an OpenAI-compatible /audio/speech endpoint.
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewClient creates a TTS client. baseURL includes the API version prefix,
// e.g. "http://kokoro:8880/v1". apiKey may be empty; model defaults to
// "kokoro".
func NewClient(baseURL, apiKey, model string) *Client {
	if model == "" {
		model = "kokoro"
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Model returns the configured model name.
func (c *Client) Model() string { return c.model }

// SpeechRequest is the OpenAI-compatible request body for /audio/speech.
type SpeechRequest struct {
	Model          string `json:"model"`
	Input          string `json:"input"`
	Voice          string `json:"voice"`
	ResponseFormat string `json:"response_format,omitempty"`
}

// Synthesize turns text into MP3 audio.
func (c *Client) Synthesize(ctx context.Context, text, voice string) ([]byte, error) {
	if voice == "" {
		voice = DefaultVoice
	}
	body, err := json.Marshal(SpeechRequest{Model: c.model, Input: text, Voice: voice, ResponseFormat: "mp3"})
	if err != nil {
		return nil, fmt.Errorf("marshaling TTS request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating TTS request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending TTS request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("TTS server returned status %d: %s", resp.StatusCode, string(errBody))
	}
	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading TTS audio: %w", err)
	}
	return audio, nil
}

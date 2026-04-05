package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for an OpenAI-compatible TTS server (e.g. Kokoro-FastAPI).
type Client struct {
	endpoint   string
	httpClient *http.Client
}

// NewClient creates a new TTS client.
// endpoint should be the base URL, e.g. "http://kokoro:8880".
func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SpeechRequest is the OpenAI-compatible request body for /v1/audio/speech.
type SpeechRequest struct {
	Model          string `json:"model"`
	Input          string `json:"input"`
	Voice          string `json:"voice"`
	ResponseFormat string `json:"response_format,omitempty"` // mp3, wav, opus, flac
}

// Synthesize sends text to the TTS server and returns audio bytes.
// Uses the OpenAI-compatible /v1/audio/speech endpoint.
func (c *Client) Synthesize(ctx context.Context, text, voice string) ([]byte, error) {
	if voice == "" {
		voice = "af_heart"
	}
	body, err := json.Marshal(SpeechRequest{
		Model:          "kokoro",
		Input:          text,
		Voice:          voice,
		ResponseFormat: "mp3",
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling TTS request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/v1/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating TTS request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending TTS request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TTS server returned status %d: %s", resp.StatusCode, string(errBody))
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading TTS audio response: %w", err)
	}
	return audio, nil
}

// Healthy checks if the TTS server is reachable.
func (c *Client) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"/v1/audio/voices", nil)
	if err != nil {
		return false
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

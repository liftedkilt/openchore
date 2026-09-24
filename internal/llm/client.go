// Package llm talks to any OpenAI-compatible chat completions API: a local
// server (Ollama, llama.cpp's llama-server, LiteRT-LM, LM Studio, vLLM) or a
// hosted one (OpenAI, Gemini's OpenAI endpoint, OpenRouter, ...).
package llm

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

// Client sends chat completion requests to an OpenAI-compatible server.
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// New creates a client. baseURL includes the API version prefix, e.g.
// "http://ollama:11434/v1" or "https://api.openai.com/v1". apiKey may be
// empty for local servers.
func New(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		// Vision requests against a CPU-only local model can take a while.
		httpClient: &http.Client{Timeout: 3 * time.Minute},
	}
}

// Model returns the configured model name.
func (c *Client) Model() string { return c.model }

// Part is one piece of a multimodal message: text or an image.
type Part struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL holds an image reference; a data: URL carries the bytes inline.
type ImageURL struct {
	URL string `json:"url"`
}

// Message is a chat message. Content is either a string or a []Part.
type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// Schema asks the server to constrain its output to a JSON schema.
type Schema struct {
	Name   string
	Schema map[string]any
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *jsonSchema `json:"json_schema,omitempty"`
}

type jsonSchema struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Chat sends messages and returns the text of the first choice. With a
// schema, the reply is JSON matching it (on servers that support structured
// outputs; callers should still parse defensively).
func (c *Client) Chat(ctx context.Context, messages []Message, schema *Schema) (string, error) {
	req := chatRequest{Model: c.model, Messages: messages}
	if schema != nil {
		req.ResponseFormat = &responseFormat{
			Type:       "json_schema",
			JSONSchema: &jsonSchema{Name: schema.Name, Strict: true, Schema: schema.Schema},
		}
	}
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshaling chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating chat request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("sending chat request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("reading chat response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI server returned status %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}

	var out chatResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("decoding chat response: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("AI server returned no choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

// ask sends a single text prompt and returns the reply with stray quotes trimmed.
func (c *Client) ask(ctx context.Context, prompt string) (string, error) {
	text, err := c.Chat(ctx, []Message{{Role: "user", Content: prompt}}, nil)
	if err != nil {
		return "", err
	}
	return strings.Trim(text, "\"' \n"), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

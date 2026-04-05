package ollama

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for the Ollama API.
type Client struct {
	endpoint   string
	httpClient *http.Client
}

// NewClient creates a new Ollama API client.
// endpoint should be the base URL, e.g. "http://ollama:11434".
func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // vision requests can be slow on CPU
		},
	}
}

// --- Chat Completion (with vision support) ---

// ChatMessage represents a message in a chat completion request.
type ChatMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"` // base64-encoded images
}

// ChatRequest is the request body for /api/chat.
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Format   string        `json:"format,omitempty"` // "json" for structured output
	Options  *ModelOptions `json:"options,omitempty"`
}

// ModelOptions are optional model parameters.
type ModelOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// ChatResponse is the response from /api/chat (non-streaming).
type ChatResponse struct {
	Model           string      `json:"model"`
	Message         ChatMessage `json:"message"`
	Done            bool        `json:"done"`
	TotalDuration   int64       `json:"total_duration"`
	EvalCount       int         `json:"eval_count"`
}

// Chat sends a chat completion request, optionally with images for vision models.
func (c *Client) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	req.Stream = false // always non-streaming for our use case
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decoding chat response: %w", err)
	}
	return &chatResp, nil
}

// --- Generate (text-only) ---

// GenerateRequest is the request body for /api/generate.
type GenerateRequest struct {
	Model   string        `json:"model"`
	Prompt  string        `json:"prompt"`
	Stream  bool          `json:"stream"`
	Format  string        `json:"format,omitempty"`
	Options *ModelOptions `json:"options,omitempty"`
}

// GenerateResponse is the response from /api/generate (non-streaming).
type GenerateResponse struct {
	Model    string `json:"model"`
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Generate sends a text generation request.
func (c *Client) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling generate request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending generate request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var genResp GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return nil, fmt.Errorf("decoding generate response: %w", err)
	}
	return &genResp, nil
}

// --- Health ---

// Healthy checks if the Ollama server is reachable.
func (c *Client) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"/api/tags", nil)
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

// --- Model Management ---

// HasModel checks if a model is already downloaded.
func (c *Client) HasModel(ctx context.Context, name string) bool {
	body, _ := json.Marshal(map[string]string{"model": name})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/api/show", bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Pull downloads a model. This can take a long time for large models.
// The request streams progress but we just wait for completion.
func (c *Client) Pull(ctx context.Context, name string) error {
	body, _ := json.Marshal(map[string]any{"model": name, "stream": false})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/api/pull", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating pull request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Use a separate client with no timeout — pulls can take minutes
	pullClient := &http.Client{}
	resp, err := pullClient.Do(req)
	if err != nil {
		return fmt.Errorf("pulling model %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pull failed for %s (status %d): %s", name, resp.StatusCode, string(respBody))
	}

	// Drain the response body (Ollama sends progress JSON lines even with stream=false)
	io.Copy(io.Discard, resp.Body)
	return nil
}

// EnsureModel checks if a model exists and pulls it if not.
// Returns true if the model is ready (either already existed or was pulled successfully).
func (c *Client) EnsureModel(ctx context.Context, name string) error {
	if c.HasModel(ctx, name) {
		return nil
	}
	return c.Pull(ctx, name)
}

// --- Helpers ---

// EncodeImageBase64 reads an image file and returns its base64-encoded content.
func EncodeImageBase64(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("reading image: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

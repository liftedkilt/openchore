package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("expected /api/chat, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var req ChatRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Model != "test-model" {
			t.Errorf("expected model test-model, got %s", req.Model)
		}
		if req.Stream {
			t.Error("expected stream=false")
		}
		if len(req.Messages) != 1 || req.Messages[0].Role != "user" {
			t.Error("expected one user message")
		}

		json.NewEncoder(w).Encode(ChatResponse{
			Model:   "test-model",
			Message: ChatMessage{Role: "assistant", Content: `{"result": "ok"}`},
			Done:    true,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.Chat(context.Background(), &ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: "test"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message.Content != `{"result": "ok"}` {
		t.Errorf("unexpected response content: %s", resp.Message.Content)
	}
}

func TestChatWithImages(t *testing.T) {
	var receivedReq ChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		json.NewEncoder(w).Encode(ChatResponse{
			Model:   "test-model",
			Message: ChatMessage{Role: "assistant", Content: "reviewed"},
			Done:    true,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Chat(context.Background(), &ChatRequest{
		Model: "test-model",
		Messages: []ChatMessage{
			{Role: "user", Content: "review this", Images: []string{"base64data"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(receivedReq.Messages[0].Images) != 1 {
		t.Error("expected image to be sent")
	}
}

func TestChatServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("model not found"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Chat(context.Background(), &ChatRequest{
		Model:    "bad-model",
		Messages: []ChatMessage{{Role: "user", Content: "test"}},
	})
	if err == nil {
		t.Fatal("expected error for server error response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected status 500 in error, got: %v", err)
	}
}

func TestGenerate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("expected /api/generate, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(GenerateResponse{
			Model:    "test-model",
			Response: "generated text",
			Done:     true,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.Generate(context.Background(), &GenerateRequest{
		Model:  "test-model",
		Prompt: "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Response != "generated text" {
		t.Errorf("unexpected response: %s", resp.Response)
	}
}

func TestGenerateServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("service unavailable"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Generate(context.Background(), &GenerateRequest{
		Model:  "test-model",
		Prompt: "hello",
	})
	if err == nil {
		t.Fatal("expected error for server error response")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("expected status 503 in error, got: %v", err)
	}
}

func TestHealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("expected /api/tags, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"models":[]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if !client.Healthy(context.Background()) {
		t.Error("expected healthy=true")
	}
}

func TestHealthyUnreachable(t *testing.T) {
	client := NewClient("http://localhost:1") // unreachable port
	if client.Healthy(context.Background()) {
		t.Error("expected healthy=false for unreachable server")
	}
}

func TestHealthyServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if client.Healthy(context.Background()) {
		t.Error("expected healthy=false for 500 response")
	}
}

func TestEncodeImageBase64(t *testing.T) {
	r := strings.NewReader("test image data")
	encoded, err := EncodeImageBase64(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if encoded == "" {
		t.Error("expected non-empty base64 string")
	}
	// Verify it's valid base64 by checking for expected encoding
	// "test image data" = "dGVzdCBpbWFnZSBkYXRh"
	if encoded != "dGVzdCBpbWFnZSBkYXRh" {
		t.Errorf("unexpected encoding: %s", encoded)
	}
}

func TestEncodeImageBase64_Empty(t *testing.T) {
	r := strings.NewReader("")
	encoded, err := EncodeImageBase64(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if encoded != "" {
		t.Errorf("expected empty string for empty input, got %q", encoded)
	}
}

func TestChatWithFormat(t *testing.T) {
	var receivedReq ChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		json.NewEncoder(w).Encode(ChatResponse{
			Model:   "test-model",
			Message: ChatMessage{Role: "assistant", Content: `{"ok":true}`},
			Done:    true,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Chat(context.Background(), &ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: "test"}},
		Format:   "json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedReq.Format != "json" {
		t.Errorf("expected format=json, got %s", receivedReq.Format)
	}
}

func TestChatWithOptions(t *testing.T) {
	var receivedReq ChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		json.NewEncoder(w).Encode(ChatResponse{
			Model:   "test-model",
			Message: ChatMessage{Role: "assistant", Content: "ok"},
			Done:    true,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Chat(context.Background(), &ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: "test"}},
		Options: &ModelOptions{
			Temperature: 0.3,
			NumPredict:  512,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedReq.Options == nil {
		t.Fatal("expected options to be sent")
	}
	if receivedReq.Options.Temperature != 0.3 {
		t.Errorf("expected temperature=0.3, got %f", receivedReq.Options.Temperature)
	}
	if receivedReq.Options.NumPredict != 512 {
		t.Errorf("expected num_predict=512, got %d", receivedReq.Options.NumPredict)
	}
}

func TestChatStreamAlwaysFalse(t *testing.T) {
	var receivedReq ChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		json.NewEncoder(w).Encode(ChatResponse{
			Model:   "test-model",
			Message: ChatMessage{Role: "assistant", Content: "ok"},
			Done:    true,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	// Even if caller sets Stream=true, client should override to false
	_, err := client.Chat(context.Background(), &ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: "test"}},
		Stream:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedReq.Stream {
		t.Error("expected stream=false even when caller sets true")
	}
}

func TestGenerateStreamAlwaysFalse(t *testing.T) {
	var receivedReq GenerateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		json.NewEncoder(w).Encode(GenerateResponse{
			Model:    "test-model",
			Response: "ok",
			Done:     true,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Generate(context.Background(), &GenerateRequest{
		Model:  "test-model",
		Prompt: "test",
		Stream: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedReq.Stream {
		t.Error("expected stream=false even when caller sets true")
	}
}

func TestHasModel_Found(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/show" {
			t.Errorf("expected /api/show, got %s", r.URL.Path)
		}
		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)
		if req["model"] != "gemma4:e2b" {
			t.Errorf("expected model gemma4:e2b, got %s", req["model"])
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"modelfile":"..."}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if !client.HasModel(context.Background(), "gemma4:e2b") {
		t.Error("expected HasModel=true")
	}
}

func TestHasModel_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if client.HasModel(context.Background(), "nonexistent") {
		t.Error("expected HasModel=false")
	}
}

func TestPull(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pull" {
			t.Errorf("expected /api/pull, got %s", r.URL.Path)
		}
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		if req["model"] != "gemma4:e2b" {
			t.Errorf("expected model gemma4:e2b, got %v", req["model"])
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if err := client.Pull(context.Background(), "gemma4:e2b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPull_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"pull failed"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if err := client.Pull(context.Background(), "bad-model"); err == nil {
		t.Fatal("expected error for failed pull")
	}
}

func TestEnsureModel_AlreadyExists(t *testing.T) {
	pullCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/show" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		} else if r.URL.Path == "/api/pull" {
			pullCalled = true
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if err := client.EnsureModel(context.Background(), "gemma4:e2b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pullCalled {
		t.Error("expected /api/pull NOT to be called when model exists")
	}
}

func TestEnsureModel_NeedsPull(t *testing.T) {
	pullCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/show" {
			w.WriteHeader(http.StatusNotFound)
		} else if r.URL.Path == "/api/pull" {
			pullCalled = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"success"}`))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if err := client.EnsureModel(context.Background(), "gemma4:e2b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pullCalled {
		t.Error("expected /api/pull to be called when model missing")
	}
}

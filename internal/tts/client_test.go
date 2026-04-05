package tts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSynthesize(t *testing.T) {
	var receivedReq SpeechRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/speech" {
			t.Errorf("expected /v1/audio/speech, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		json.NewDecoder(r.Body).Decode(&receivedReq)

		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("fake-mp3-audio-data"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	audio, err := client.Synthesize(context.Background(), "Hello world", "test-voice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(audio) == 0 {
		t.Error("expected non-empty audio data")
	}
	if receivedReq.Input != "Hello world" {
		t.Errorf("expected input 'Hello world', got %q", receivedReq.Input)
	}
	if receivedReq.Voice != "test-voice" {
		t.Errorf("expected voice 'test-voice', got %q", receivedReq.Voice)
	}
	if receivedReq.Model != "kokoro" {
		t.Errorf("expected model 'kokoro', got %q", receivedReq.Model)
	}
	if receivedReq.ResponseFormat != "mp3" {
		t.Errorf("expected response_format 'mp3', got %q", receivedReq.ResponseFormat)
	}
}

func TestSynthesizeDefaultVoice(t *testing.T) {
	var receivedReq SpeechRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Write([]byte("audio"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Synthesize(context.Background(), "Hello", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedReq.Voice != "af_heart" {
		t.Errorf("expected default voice 'af_heart', got %q", receivedReq.Voice)
	}
}

func TestSynthesizeServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"voice not found"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Synthesize(context.Background(), "Hello", "bad-voice")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestHealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/voices" {
			t.Errorf("expected /v1/audio/voices, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"voices":["af_heart"]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if !client.Healthy(context.Background()) {
		t.Error("expected healthy=true")
	}
}

func TestHealthyUnreachable(t *testing.T) {
	client := NewClient("http://localhost:1")
	if client.Healthy(context.Background()) {
		t.Error("expected healthy=false for unreachable server")
	}
}

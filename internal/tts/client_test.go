package tts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/liftedkilt/openchore/internal/model"
)

func TestSynthesize(t *testing.T) {
	var got SpeechRequest
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/speech" {
			t.Errorf("expected /v1/audio/speech, got %s", r.URL.Path)
		}
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("fake-mp3"))
	}))
	defer server.Close()

	client := NewClient(server.URL+"/v1/", "sk-test", "tts-1")
	audio, err := client.Synthesize(context.Background(), "Hello world", "alloy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(audio) != "fake-mp3" {
		t.Errorf("unexpected audio %q", audio)
	}
	if got.Input != "Hello world" || got.Voice != "alloy" || got.Model != "tts-1" || got.ResponseFormat != "mp3" {
		t.Errorf("unexpected request %+v", got)
	}
	if auth != "Bearer sk-test" {
		t.Errorf("expected bearer auth, got %q", auth)
	}
}

func TestSynthesizeDefaults(t *testing.T) {
	var got SpeechRequest
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte("x"))
	}))
	defer server.Close()

	if _, err := NewClient(server.URL, "", "").Synthesize(context.Background(), "Hi", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Voice != DefaultVoice || got.Model != "kokoro" {
		t.Errorf("expected Kokoro defaults, got %+v", got)
	}
	if auth != "" {
		t.Errorf("expected no auth header without a key, got %q", auth)
	}
}

func TestSynthesizeServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not loaded", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "", "").Synthesize(context.Background(), "Hi", "")
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected a 500 error, got %v", err)
	}
}

func TestSpokenText(t *testing.T) {
	cases := []struct {
		title, desc, want string
	}{
		{"Make bed", "", "Make bed"},
		{"Make bed", "Pull the covers up.", "Make bed. Pull the covers up."},
		{"Feed the cat!", "One scoop.", "Feed the cat! One scoop."},
	}
	for _, c := range cases {
		if got := SpokenText(&model.Chore{Title: c.title, Description: c.desc}); got != c.want {
			t.Errorf("SpokenText(%q, %q) = %q, want %q", c.title, c.desc, got, c.want)
		}
	}
}

package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captured is what the fake server saw of the last request.
type captured struct {
	path string
	auth string
	body map[string]any
}

func fakeServer(t *testing.T, reply string, status int) (*httptest.Server, *captured) {
	t.Helper()
	c := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.path = r.URL.Path
		c.auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&c.body)
		if status != http.StatusOK {
			http.Error(w, reply, status)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": reply}}},
		})
	}))
	t.Cleanup(srv.Close)
	return srv, c
}

func TestReviewPhotoSendsImageAndSchema(t *testing.T) {
	srv, got := fakeServer(t, `{"complete": true, "confidence": 0.9, "feedback": "Bed is made."}`, http.StatusOK)
	photo := filepath.Join(t.TempDir(), "bed.jpg")
	if err := os.WriteFile(photo, []byte("\xff\xd8\xff\xe0jpeg"), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := New(srv.URL+"/v1/", "sk-test", "gemma").ReviewPhoto(context.Background(), "Make bed", "Pull up the covers", photo)
	if err != nil {
		t.Fatalf("ReviewPhoto: %v", err)
	}
	if !res.Complete || res.Confidence != 0.9 || res.Feedback != "Bed is made." {
		t.Errorf("unexpected result %+v", res)
	}
	if got.path != "/v1/chat/completions" || got.auth != "Bearer sk-test" || got.body["model"] != "gemma" {
		t.Errorf("unexpected request: path=%s auth=%q model=%v", got.path, got.auth, got.body["model"])
	}

	rf := got.body["response_format"].(map[string]any)
	if rf["type"] != "json_schema" || rf["json_schema"].(map[string]any)["name"] != "chore_review" {
		t.Errorf("expected a JSON schema response format, got %v", rf)
	}
	parts := got.body["messages"].([]any)[0].(map[string]any)["content"].([]any)
	text := parts[0].(map[string]any)["text"].(string)
	image := parts[1].(map[string]any)["image_url"].(map[string]any)["url"].(string)
	if !strings.Contains(text, "Make bed: Pull up the covers") {
		t.Errorf("expected the chore in the prompt, got %q", text)
	}
	if !strings.HasPrefix(image, "data:image/jpeg;base64,") {
		t.Errorf("expected an inline JPEG, got %.40q", image)
	}
}

func TestDraftDescriptionHasNoKeyOrSchema(t *testing.T) {
	srv, got := fakeServer(t, `"Put your dishes in the sink."`, http.StatusOK)
	desc, err := New(srv.URL, "", "m").DraftDescription(context.Background(), "Dishes", "core")
	if err != nil {
		t.Fatalf("DraftDescription: %v", err)
	}
	if desc != "Put your dishes in the sink." {
		t.Errorf("expected quotes trimmed, got %q", desc)
	}
	if got.auth != "" || got.body["response_format"] != nil {
		t.Errorf("expected no auth and no schema, got auth=%q format=%v", got.auth, got.body["response_format"])
	}
}

func TestChatReportsServerErrors(t *testing.T) {
	srv, _ := fakeServer(t, "model not found", http.StatusNotFound)
	_, err := New(srv.URL, "", "m").DraftDescription(context.Background(), "Dishes", "core")
	if err == nil || !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), "model not found") {
		t.Fatalf("expected the server's error, got %v", err)
	}
}

func TestParseReviewResponse(t *testing.T) {
	cases := map[string]struct {
		raw        string
		complete   bool
		confidence float64
		wantErr    bool
	}{
		"plain":        {raw: `{"complete": true, "confidence": 0.8, "feedback": "ok"}`, complete: true, confidence: 0.8},
		"code fence":   {raw: "```json\n{\"complete\": false, \"confidence\": 0.4, \"feedback\": \"no\"}\n```", confidence: 0.4},
		"prose around": {raw: `Sure! {"complete": true, "confidence": 0.7, "feedback": "ok"} Hope that helps.`, complete: true, confidence: 0.7},
		"clamped high": {raw: `{"complete": true, "confidence": 3, "feedback": ""}`, complete: true, confidence: 1},
		"clamped low":  {raw: `{"complete": false, "confidence": -1, "feedback": ""}`, confidence: 0},
		"not json":     {raw: "I can't tell.", wantErr: true},
		"empty":        {raw: "", wantErr: true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := parseReviewResponse(c.raw)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %+v", res)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Complete != c.complete || res.Confidence != c.confidence {
				t.Errorf("got %+v", res)
			}
		})
	}
}

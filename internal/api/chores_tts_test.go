package api_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/liftedkilt/openchore/internal/ai"
	"github.com/liftedkilt/openchore/internal/aibackend"
	"github.com/liftedkilt/openchore/internal/tts"
)

// ttsMocks wraps the two upstream services the TTSGenerator talks to so a
// single test can assert call counts and the bodies sent to each server.
type ttsMocks struct {
	llm     *httptest.Server
	tts     *httptest.Server
	llmResp atomic.Pointer[string] // current LLM response text ("" means default)
	llmCalls atomic.Int32
	ttsCalls atomic.Int32
	// lastTTSText captures the latest /v1/audio/speech "input" field value.
	lastTTSText atomic.Pointer[string]
}

// setLLMResponse swaps out the text returned by the mocked /api/generate endpoint.
func (m *ttsMocks) setLLMResponse(s string) {
	m.llmResp.Store(&s)
}

// newTTSMocks spins up httptest servers for the LLM and Kokoro TTS backends,
// enabling tests to exercise the real TTSGenerator end-to-end.
func newTTSMocks(t *testing.T) *ttsMocks {
	t.Helper()
	m := &ttsMocks{}
	defaultLLM := "Mocked spoken description."
	m.llmResp.Store(&defaultLLM)

	m.llm = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/generate":
			m.llmCalls.Add(1)
			// Drain body so the client gets a clean response.
			io.Copy(io.Discard, r.Body)
			resp := *m.llmResp.Load()
			_ = json.NewEncoder(w).Encode(map[string]any{
				"model":    "mock",
				"response": resp,
				"done":     true,
			})
		case "/api/tags":
			// /api/tags is used by the Healthy check. Return empty list.
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
		default:
			http.NotFound(w, r)
		}
	}))

	m.tts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/audio/speech":
			m.ttsCalls.Add(1)
			var req struct {
				Input string `json:"input"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			text := req.Input
			m.lastTTSText.Store(&text)
			w.Header().Set("Content-Type", "audio/mpeg")
			// Tiny deterministic byte payload — content is irrelevant since we
			// only verify that the file lands on disk.
			_, _ = w.Write([]byte("MOCKAUDIO:" + text))
		case "/v1/audio/voices":
			// Used by tts.Client.Healthy for the lazy reconnect path.
			_ = json.NewEncoder(w).Encode(map[string]any{"voices": []string{"af_heart"}})
		default:
			http.NotFound(w, r)
		}
	}))

	t.Cleanup(func() {
		m.llm.Close()
		m.tts.Close()
	})
	return m
}

// wireTTS attaches a real TTSGenerator (backed by newTTSMocks) to the
// ChoreHandler under test. It also chdirs into a scratch directory so
// chore_{id}.mp3 files don't leak into the source tree.
func wireTTS(t *testing.T, env *testEnv, m *ttsMocks) {
	t.Helper()
	// Isolate the data/tts output path per test.
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join("data", "tts"), 0o750); err != nil {
		t.Fatalf("mkdir data/tts: %v", err)
	}

	llmClient := aibackend.NewClient(m.llm.URL)
	ttsClient := tts.NewClient(m.tts.URL)
	ttsGen := ai.NewTTSGenerator(llmClient, "mock-model", ttsClient, m.tts.URL, "af_heart")
	env.chores.SetAIServices(nil, ttsGen, nil)
}

// createChoreForTTS inserts a chore via the admin API and returns its id.
func createChoreForTTS(t *testing.T, env *testEnv, title string) int64 {
	t.Helper()
	resp := env.request(t, "POST", "/api/chores", map[string]any{
		"title":    title,
		"category": "core",
	}, adminHeaders())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create chore: %d", resp.StatusCode)
	}
	var chore map[string]any
	decodeBody(t, resp, &chore)
	return int64(chore["id"].(float64))
}

// --- RegenerateChoreTTS ---

func TestRegenerateTTS_HappyPath(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	m := newTTSMocks(t)
	wireTTS(t, env, m)

	choreID := createChoreForTTS(t, env, "Sweep Kitchen")

	resp := env.expectStatus(t, "POST",
		fmt.Sprintf("/api/chores/%d/tts/regenerate", choreID),
		map[string]any{"description": "Please sweep the kitchen floor."},
		adminHeaders(), http.StatusOK)

	var body map[string]any
	decodeBody(t, resp, &body)

	if body["tts_description"] != "Please sweep the kitchen floor." {
		t.Errorf("expected saved tts_description, got %v", body["tts_description"])
	}
	expectedURL := fmt.Sprintf("/tts/chore_%d.mp3", choreID)
	if body["tts_audio_url"] != expectedURL {
		t.Errorf("expected tts_audio_url %q, got %v", expectedURL, body["tts_audio_url"])
	}

	// DB row must reflect the new description + URL.
	var dbDesc, dbURL string
	err := env.db.QueryRow(`SELECT tts_description, tts_audio_url FROM chores WHERE id = ?`, choreID).
		Scan(&dbDesc, &dbURL)
	if err != nil {
		t.Fatalf("query chore: %v", err)
	}
	if dbDesc != "Please sweep the kitchen floor." {
		t.Errorf("db tts_description: got %q", dbDesc)
	}
	if dbURL != expectedURL {
		t.Errorf("db tts_audio_url: got %q", dbURL)
	}

	// Audio file must have been written to disk with the mocked payload.
	audioBytes, err := os.ReadFile(filepath.Join("data", "tts", fmt.Sprintf("chore_%d.mp3", choreID)))
	if err != nil {
		t.Fatalf("read audio file: %v", err)
	}
	if !strings.Contains(string(audioBytes), "Please sweep the kitchen floor.") {
		t.Errorf("audio payload missing description text; got %q", string(audioBytes))
	}

	// The Kokoro mock should have received exactly one synthesis call; the
	// LLM mock should NOT have been called because a description was supplied.
	if got := m.ttsCalls.Load(); got != 1 {
		t.Errorf("tts synth calls: want 1, got %d", got)
	}
	if got := m.llmCalls.Load(); got != 0 {
		t.Errorf("llm calls: want 0 (description provided), got %d", got)
	}
}

func TestRegenerateTTS_FallsBackToExistingDescription(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	m := newTTSMocks(t)
	wireTTS(t, env, m)

	choreID := createChoreForTTS(t, env, "Feed Cat")
	// Seed an existing tts_description so the handler can fall back to it.
	if _, err := env.db.Exec(`UPDATE chores SET tts_description = ? WHERE id = ?`,
		"Feed the cat now.", choreID); err != nil {
		t.Fatalf("seed tts_description: %v", err)
	}

	// Empty description body → handler uses the stored description.
	resp := env.expectStatus(t, "POST",
		fmt.Sprintf("/api/chores/%d/tts/regenerate", choreID),
		map[string]any{"description": ""},
		adminHeaders(), http.StatusOK)

	var body map[string]any
	decodeBody(t, resp, &body)
	if body["tts_description"] != "Feed the cat now." {
		t.Errorf("expected fallback tts_description, got %v", body["tts_description"])
	}

	// The Kokoro mock should have been asked to synthesize the existing text.
	if last := m.lastTTSText.Load(); last == nil || *last != "Feed the cat now." {
		t.Errorf("expected TTS input to be existing description, got %v", last)
	}
}

func TestRegenerateTTS_RequiresDescriptionWhenNoneStored(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	m := newTTSMocks(t)
	wireTTS(t, env, m)

	choreID := createChoreForTTS(t, env, "Brand New Chore")

	env.expectStatus(t, "POST",
		fmt.Sprintf("/api/chores/%d/tts/regenerate", choreID),
		map[string]any{"description": "   "}, // whitespace-only
		adminHeaders(), http.StatusBadRequest)

	if got := m.ttsCalls.Load(); got != 0 {
		t.Errorf("tts should not be called; got %d", got)
	}
}

func TestRegenerateTTS_RequiresAIConfiguration(t *testing.T) {
	// No wireTTS here — ttsGen remains nil.
	env := setupTest(t)
	env.createAdmin(t)
	choreID := createChoreForTTS(t, env, "Take Out Trash")

	env.expectStatus(t, "POST",
		fmt.Sprintf("/api/chores/%d/tts/regenerate", choreID),
		map[string]any{"description": "Take out the trash."},
		adminHeaders(), http.StatusServiceUnavailable)
}

func TestRegenerateTTS_ChoreNotFound(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	m := newTTSMocks(t)
	wireTTS(t, env, m)

	env.expectStatus(t, "POST", "/api/chores/99999/tts/regenerate",
		map[string]any{"description": "nothing to say"},
		adminHeaders(), http.StatusNotFound)
}

func TestRegenerateTTS_InvalidChoreID(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	m := newTTSMocks(t)
	wireTTS(t, env, m)

	env.expectStatus(t, "POST", "/api/chores/not-a-number/tts/regenerate",
		map[string]any{"description": "x"},
		adminHeaders(), http.StatusBadRequest)
}

func TestRegenerateTTS_NonAdminForbidden(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")
	m := newTTSMocks(t)
	wireTTS(t, env, m)

	choreID := createChoreForTTS(t, env, "Walk Dog")

	env.expectStatus(t, "POST",
		fmt.Sprintf("/api/chores/%d/tts/regenerate", choreID),
		map[string]any{"description": "Walk the dog."},
		childHeaders(kidID), http.StatusForbidden)

	if got := m.ttsCalls.Load(); got != 0 {
		t.Errorf("non-admin should not trigger TTS; got %d calls", got)
	}
}

// --- GenerateChoreTTSDescription ---

func TestGenerateTTSDescription_HappyPath(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	m := newTTSMocks(t)
	wireTTS(t, env, m)
	m.setLLMResponse("Take out the trash to the curb.")

	choreID := createChoreForTTS(t, env, "Take Out Trash")

	resp := env.expectStatus(t, "POST",
		fmt.Sprintf("/api/chores/%d/tts/generate-description", choreID),
		nil, adminHeaders(), http.StatusOK)

	var body map[string]any
	decodeBody(t, resp, &body)
	if body["description"] != "Take out the trash to the curb." {
		t.Errorf("expected generated description, got %v", body["description"])
	}

	// Generation must not persist — the chore's tts_description should still be empty.
	var stored string
	if err := env.db.QueryRow(`SELECT tts_description FROM chores WHERE id = ?`, choreID).
		Scan(&stored); err != nil {
		t.Fatalf("query tts_description: %v", err)
	}
	if stored != "" {
		t.Errorf("expected tts_description to remain unpersisted, got %q", stored)
	}

	// No audio file should have been written because Kokoro was not called.
	if _, err := os.Stat(filepath.Join("data", "tts", fmt.Sprintf("chore_%d.mp3", choreID))); !os.IsNotExist(err) {
		t.Errorf("expected no audio file, got err=%v", err)
	}

	if got := m.llmCalls.Load(); got != 1 {
		t.Errorf("llm calls: want 1, got %d", got)
	}
	if got := m.ttsCalls.Load(); got != 0 {
		t.Errorf("tts should not be called; got %d", got)
	}
}

func TestGenerateTTSDescription_RequiresAIConfiguration(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	choreID := createChoreForTTS(t, env, "Clean Room")

	env.expectStatus(t, "POST",
		fmt.Sprintf("/api/chores/%d/tts/generate-description", choreID),
		nil, adminHeaders(), http.StatusServiceUnavailable)
}

func TestGenerateTTSDescription_ChoreNotFound(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	m := newTTSMocks(t)
	wireTTS(t, env, m)

	env.expectStatus(t, "POST", "/api/chores/99999/tts/generate-description",
		nil, adminHeaders(), http.StatusNotFound)
}

func TestGenerateTTSDescription_NonAdminForbidden(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")
	m := newTTSMocks(t)
	wireTTS(t, env, m)

	choreID := createChoreForTTS(t, env, "Make Bed")

	env.expectStatus(t, "POST",
		fmt.Sprintf("/api/chores/%d/tts/generate-description", choreID),
		nil, childHeaders(kidID), http.StatusForbidden)
}

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
	"sync"
	"testing"
	"time"

	"github.com/liftedkilt/openchore/internal/llm"
	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/tts"
)

// fakeAI is an OpenAI-compatible server standing in for both the chat model
// and the speech endpoint.
type fakeAI struct {
	server *httptest.Server

	mu         sync.Mutex
	review     model.AIReviewResult // returned for image prompts
	text       string               // returned for text prompts
	chatCalls  int
	speechText []string
}

func newFakeAI(t *testing.T) *fakeAI {
	t.Helper()
	f := &fakeAI{
		review: model.AIReviewResult{Complete: true, Confidence: 0.95, Feedback: "Bed is made."},
		text:   "A friendly description.",
	}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Path {
		case "/v1/chat/completions":
			f.chatCalls++
			content := f.text
			if strings.Contains(string(body), `"image_url"`) {
				b, _ := json.Marshal(f.review)
				content = string(b)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": content}}},
			})
		case "/v1/audio/speech":
			var req tts.SpeechRequest
			_ = json.Unmarshal(body, &req)
			f.speechText = append(f.speechText, req.Input)
			_, _ = w.Write([]byte("MP3:" + req.Input))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeAI) setReview(r model.AIReviewResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.review = r
}

func (f *fakeAI) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.chatCalls
}

func (f *fakeAI) spoken() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.speechText...)
}

// setupAITest wires the fake server in as both AI and TTS, and runs the
// test from a scratch directory so uploads and audio stay out of the tree.
func setupAITest(t *testing.T, withTTS bool) (*testEnv, *fakeAI) {
	t.Helper()
	t.Chdir(t.TempDir())
	env := setupTest(t)
	f := newFakeAI(t)
	aiClient := llm.New(f.server.URL+"/v1", "", "test-model")
	var audio *tts.ChoreAudio
	if withTTS {
		audio = tts.NewChoreAudio(tts.NewClient(f.server.URL+"/v1", "", ""), env.store)
	}
	env.chores.SetAI(aiClient, audio)
	env.reports.SetAI(aiClient)
	return env, f
}

// writeUpload drops a fake photo where the upload handler would.
func writeUpload(t *testing.T, name string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join("data", "uploads"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("data", "uploads", name), []byte("\xff\xd8\xff\xe0fakejpeg"), 0o640); err != nil {
		t.Fatal(err)
	}
	return "/uploads/" + name
}

func (e *testEnv) setSetting(t *testing.T, key, value string) {
	t.Helper()
	e.expectStatus(t, "PUT", "/api/admin/settings/"+key, map[string]any{"value": value}, adminHeaders(), http.StatusOK)
}

func (e *testEnv) completion(t *testing.T, id int64) *model.ChoreCompletion {
	t.Helper()
	cc, err := e.store.GetCompletion(t.Context(), id)
	if err != nil || cc == nil {
		t.Fatalf("GetCompletion(%d): %v", id, err)
	}
	return cc
}

// completePhotoChore creates a photo chore needing approval for a new kid,
// completes it with a photo as the kid, and waits for the review.
func completePhotoChore(t *testing.T, env *testEnv) (kidID int, completionID int64) {
	t.Helper()
	env.createAdmin(t)
	kidID = env.createChild(t, "Kid")
	sched := env.createScheduledChore(t, "Make bed", kidID, map[string]any{
		"requires_photo": true, "requires_approval": true, "points_value": 5,
	})
	resp := env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", sched), map[string]any{
		"photo_url": writeUpload(t, "bed.jpg"),
	}, childHeaders(kidID), http.StatusCreated)
	var cc map[string]any
	decodeBody(t, resp, &cc)
	if cc["status"] != model.StatusPending {
		t.Fatalf("expected the kid's completion to wait for a parent, got %v", cc["status"])
	}
	env.chores.WaitForReviews()
	return kidID, int64(cc["id"].(float64))
}

func TestPhotoReviewLeavesNoteForParent(t *testing.T) {
	env, f := setupAITest(t, false)
	env.createAdmin(t)
	env.setSetting(t, "ai_photo_review", "true")
	f.setReview(model.AIReviewResult{Complete: false, Confidence: 0.99, Feedback: "Pillows are on the floor."})

	kidID, id := completePhotoChore(t, env)

	cc := env.completion(t, id)
	if cc.Status != model.StatusPending {
		t.Fatalf("the AI must never reject; expected pending, got %s", cc.Status)
	}
	if cc.AIFeedback != "Pillows are on the floor." || cc.AIComplete == nil || *cc.AIComplete {
		t.Errorf("expected the reviewer's note to be stored, got %+v", cc)
	}

	// The note shows up in the parent's approval queue.
	resp := env.expectStatus(t, "GET", "/api/completions/pending", nil, adminHeaders(), http.StatusOK)
	var pending []map[string]any
	decodeBody(t, resp, &pending)
	if len(pending) != 1 || pending[0]["ai_feedback"] != "Pillows are on the floor." || pending[0]["ai_complete"] != false {
		t.Fatalf("expected the note in the pending list, got %+v", pending)
	}

	// The parent still decides.
	env.expectStatus(t, "POST", fmt.Sprintf("/api/completions/%d/approve", id), nil, adminHeaders(), http.StatusNoContent)
	if got := env.balance(t, kidID); got != 5 {
		t.Errorf("expected 5 points after approval, got %d", got)
	}
}

func TestPhotoReviewAutoApprovesClearPasses(t *testing.T) {
	env, _ := setupAITest(t, false)
	env.createAdmin(t)
	env.setSetting(t, "ai_photo_review", "true")
	env.setSetting(t, "ai_auto_approve", "true")

	kidID, id := completePhotoChore(t, env)

	cc := env.completion(t, id)
	if cc.Status != model.StatusApproved || cc.ApprovedBy != nil {
		t.Fatalf("expected an automatic approval, got status=%s approved_by=%v", cc.Status, cc.ApprovedBy)
	}
	if got := env.balance(t, kidID); got != 5 {
		t.Errorf("expected auto-approval to credit 5 points, got %d", got)
	}
}

func TestPhotoReviewBelowThresholdWaitsForParent(t *testing.T) {
	env, f := setupAITest(t, false)
	env.createAdmin(t)
	env.setSetting(t, "ai_photo_review", "true")
	env.setSetting(t, "ai_auto_approve", "true")
	env.setSetting(t, "ai_auto_approve_threshold", "0.9")
	f.setReview(model.AIReviewResult{Complete: true, Confidence: 0.6, Feedback: "Probably made; photo is dark."})

	_, id := completePhotoChore(t, env)

	if cc := env.completion(t, id); cc.Status != model.StatusPending {
		t.Fatalf("expected a low-confidence pass to wait for a parent, got %s", cc.Status)
	}
}

func TestPhotoReviewOffByDefault(t *testing.T) {
	env, f := setupAITest(t, false)

	_, id := completePhotoChore(t, env)

	if f.calls() != 0 {
		t.Errorf("expected no AI calls with photo review off, got %d", f.calls())
	}
	if cc := env.completion(t, id); cc.AIFeedback != "" {
		t.Errorf("expected no note, got %q", cc.AIFeedback)
	}
}

func TestKidCanSkipPhotoAndWaitForParent(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")
	sched := env.createScheduledChore(t, "Make bed", kidID, map[string]any{"requires_photo": true})

	// Without a photo the kid is asked for one...
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", sched), map[string]any{},
		childHeaders(kidID), http.StatusBadRequest)

	// ...but can choose to finish anyway.
	resp := env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", sched), map[string]any{"skip_photo": true},
		childHeaders(kidID), http.StatusCreated)
	var cc map[string]any
	decodeBody(t, resp, &cc)
	if cc["status"] != model.StatusPending {
		t.Fatalf("expected a photo-less completion to wait for a parent, got %v", cc["status"])
	}
	if got := env.balance(t, kidID); got != 0 {
		t.Errorf("expected no points until approved, got %d", got)
	}
}

func TestAIStatus(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	resp := env.expectStatus(t, "GET", "/api/admin/ai/status", nil, adminHeaders(), http.StatusOK)
	var status map[string]map[string]any
	decodeBody(t, resp, &status)
	if status["ai"]["configured"] != false || status["tts"]["configured"] != false {
		t.Errorf("expected nothing configured, got %+v", status)
	}

	env, _ = setupAITest(t, true)
	env.createAdmin(t)
	resp = env.expectStatus(t, "GET", "/api/admin/ai/status", nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &status)
	if status["ai"]["model"] != "test-model" || status["tts"]["model"] != "kokoro" {
		t.Errorf("expected both configured, got %+v", status)
	}
}

func TestAIEndpointsUnavailableWithoutConfig(t *testing.T) {
	env := setupTest(t)
	env.createAdmin(t)
	env.expectStatus(t, "POST", "/api/admin/ai/generate-description", map[string]any{"title": "Dishes"}, adminHeaders(), http.StatusServiceUnavailable)
	env.expectStatus(t, "POST", "/api/admin/ai/test", map[string]any{"chore_title": "Dishes", "photo_url": "/uploads/x.jpg"}, adminHeaders(), http.StatusServiceUnavailable)
	env.expectStatus(t, "GET", "/api/admin/reports/ai-summary?user_id=1", nil, adminHeaders(), http.StatusServiceUnavailable)
	env.expectStatus(t, "POST", "/api/admin/tts/regenerate", nil, adminHeaders(), http.StatusServiceUnavailable)
	env.expectStatus(t, "POST", "/api/chores/1/tts/regenerate", nil, adminHeaders(), http.StatusServiceUnavailable)
}

func TestTestAIReview(t *testing.T) {
	env, _ := setupAITest(t, false)
	env.createAdmin(t)
	resp := env.expectStatus(t, "POST", "/api/admin/ai/test", map[string]any{
		"chore_title": "Make bed", "photo_url": writeUpload(t, "test.jpg"),
	}, adminHeaders(), http.StatusOK)
	var out map[string]any
	decodeBody(t, resp, &out)
	if out["complete"] != true || out["feedback"] != "Bed is made." || out["would_approve"] != false {
		t.Errorf("unexpected test review %+v (auto-approve is off, so would_approve must be false)", out)
	}
}

func TestGenerateDescription(t *testing.T) {
	env, _ := setupAITest(t, false)
	env.createAdmin(t)
	resp := env.expectStatus(t, "POST", "/api/admin/ai/generate-description", map[string]any{
		"title": "Dishes", "category": "core",
	}, adminHeaders(), http.StatusOK)
	var out map[string]string
	decodeBody(t, resp, &out)
	if out["description"] != "A friendly description." {
		t.Errorf("unexpected description %q", out["description"])
	}
	env.expectStatus(t, "POST", "/api/admin/ai/generate-description", map[string]any{}, adminHeaders(), http.StatusBadRequest)
	env.expectStatus(t, "POST", "/api/admin/ai/suggest-points", map[string]any{"title": "Dishes"}, adminHeaders(), http.StatusNotFound)
}

// waitForAudio polls until the chore's audio URL changes from prev.
func waitForAudio(t *testing.T, env *testEnv, choreID int64, prev string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c, _ := env.store.GetChore(t.Context(), choreID)
		if c != nil && c.TTSAudioURL != "" && c.TTSAudioURL != prev {
			return c.TTSAudioURL
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("chore %d audio was not generated", choreID)
	return ""
}

func TestChoreAudioFollowsChoreText(t *testing.T) {
	env, f := setupAITest(t, true)
	env.createAdmin(t)

	resp := env.expectStatus(t, "POST", "/api/chores", map[string]any{
		"title": "Feed the cat", "description": "One scoop of food.", "category": "core",
	}, adminHeaders(), http.StatusCreated)
	var chore map[string]any
	decodeBody(t, resp, &chore)
	id := int64(chore["id"].(float64))

	url := waitForAudio(t, env, id, "")
	if !strings.HasPrefix(url, fmt.Sprintf("/tts/chore_%d.mp3?v=", id)) {
		t.Fatalf("unexpected audio URL %q", url)
	}
	if spoken := f.spoken(); len(spoken) != 1 || spoken[0] != "Feed the cat. One scoop of food." {
		t.Fatalf("expected the title and description read aloud, got %q", spoken)
	}
	audio := env.request(t, "GET", url, nil, nil)
	body, _ := io.ReadAll(audio.Body)
	audio.Body.Close()
	if audio.StatusCode != http.StatusOK || string(body) != "MP3:Feed the cat. One scoop of food." {
		t.Fatalf("expected the audio to be served, got %d %q", audio.StatusCode, body)
	}

	// Changing the text regenerates the audio.
	env.expectStatus(t, "PUT", fmt.Sprintf("/api/chores/%d", id), map[string]any{
		"description": "Two scoops of food.",
	}, adminHeaders(), http.StatusOK)
	waitForAudio(t, env, id, url)

	// Deleting the chore removes its file.
	env.expectStatus(t, "DELETE", fmt.Sprintf("/api/chores/%d", id), nil, adminHeaders(), http.StatusNoContent)
	if _, err := os.Stat(filepath.Join(tts.Dir, fmt.Sprintf("chore_%d.mp3", id))); !os.IsNotExist(err) {
		t.Errorf("expected the audio file to be removed, got %v", err)
	}
}

func TestRegenerateChoreTTS(t *testing.T) {
	env, _ := setupAITest(t, true)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")
	resp := env.expectStatus(t, "POST", "/api/chores", map[string]any{"title": "Dishes", "category": "core"}, adminHeaders(), http.StatusCreated)
	var chore map[string]any
	decodeBody(t, resp, &chore)
	id := int64(chore["id"].(float64))
	waitForAudio(t, env, id, "")

	resp = env.expectStatus(t, "POST", fmt.Sprintf("/api/chores/%d/tts/regenerate", id), nil, adminHeaders(), http.StatusOK)
	var out map[string]string
	decodeBody(t, resp, &out)
	if !strings.HasPrefix(out["tts_audio_url"], fmt.Sprintf("/tts/chore_%d.mp3", id)) {
		t.Errorf("unexpected audio URL %q", out["tts_audio_url"])
	}

	env.expectStatus(t, "POST", "/api/chores/999/tts/regenerate", nil, adminHeaders(), http.StatusNotFound)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/chores/%d/tts/regenerate", id), nil, childHeaders(kidID), http.StatusForbidden)
	env.expectStatus(t, "POST", "/api/admin/tts/regenerate", nil, adminHeaders(), http.StatusAccepted)
}

func TestWeeklySummaryIsKeptForFinishedWeeks(t *testing.T) {
	env, f := setupAITest(t, false)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")
	lastWeek := fmt.Sprintf("/api/admin/reports/ai-summary?user_id=%d&period=week&date=%s",
		kidID, time.Now().AddDate(0, 0, -7).Format(model.DateFormat))

	// A finished week is written once and kept.
	var out map[string]string
	resp := env.expectStatus(t, "GET", lastWeek, nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &out)
	if out["summary"] != "A friendly description." || f.calls() != 1 {
		t.Fatalf("expected a generated summary, got %q (%d calls)", out["summary"], f.calls())
	}
	stored, _ := env.store.GetWeeklySummary(t.Context(), int64(kidID), mondayOf(time.Now().AddDate(0, 0, -7)))
	if stored != out["summary"] {
		t.Fatalf("expected the summary to be stored, got %q", stored)
	}
	resp = env.expectStatus(t, "GET", lastWeek, nil, adminHeaders(), http.StatusOK)
	decodeBody(t, resp, &out)
	if f.calls() != 1 {
		t.Fatalf("expected the stored summary to be reused, got %d calls", f.calls())
	}

	// The current week is still in progress, so it is written each time.
	thisWeek := fmt.Sprintf("/api/admin/reports/ai-summary?user_id=%d&period=week", kidID)
	env.expectStatus(t, "GET", thisWeek, nil, adminHeaders(), http.StatusOK).Body.Close()
	env.expectStatus(t, "GET", thisWeek, nil, adminHeaders(), http.StatusOK).Body.Close()
	if f.calls() != 3 {
		t.Fatalf("expected the current week to be generated each time, got %d calls", f.calls())
	}
}

func TestWriteWeeklySummaries(t *testing.T) {
	env, f := setupAITest(t, false)
	env.createAdmin(t)
	kidID := env.createChild(t, "Kid")
	env.createChild(t, "Idle") // no chores: nothing to summarize

	// Some activity last week.
	thisMonday, _ := time.ParseInLocation(model.DateFormat, mondayOf(time.Now()), time.Local)
	lastWed := thisMonday.AddDate(0, 0, -5).Format(model.DateFormat)
	resp := env.expectStatus(t, "POST", "/api/chores", map[string]any{"title": "Dishes", "category": "core"}, adminHeaders(), http.StatusCreated)
	var chore map[string]any
	decodeBody(t, resp, &chore)
	resp = env.expectStatus(t, "POST", fmt.Sprintf("/api/chores/%d/schedules", int(chore["id"].(float64))),
		map[string]any{"assigned_to": kidID, "specific_date": lastWed}, adminHeaders(), http.StatusCreated)
	var sched map[string]any
	decodeBody(t, resp, &sched)
	env.expectStatus(t, "POST", fmt.Sprintf("/api/schedules/%d/complete", int(sched["id"].(float64))),
		map[string]any{"completion_date": lastWed}, childHeaders(kidID), http.StatusCreated).Body.Close()

	mondayAfternoon := thisMonday.Add(13 * time.Hour)

	env.reports.WriteWeeklySummaries(t.Context(), mondayAfternoon)
	if f.calls() != 0 {
		t.Fatalf("expected nothing while ai_weekly_summary is off, got %d calls", f.calls())
	}

	env.setSetting(t, "ai_weekly_summary", "true")
	env.reports.WriteWeeklySummaries(t.Context(), thisMonday.Add(9*time.Hour))
	if f.calls() != 0 {
		t.Fatalf("expected to wait until Monday noon, got %d calls", f.calls())
	}

	env.reports.WriteWeeklySummaries(t.Context(), mondayAfternoon)
	if f.calls() != 1 {
		t.Fatalf("expected one summary (the idle kid is skipped), got %d calls", f.calls())
	}
	lastMonday := thisMonday.AddDate(0, 0, -7).Format(model.DateFormat)
	if got, _ := env.store.GetWeeklySummary(t.Context(), int64(kidID), lastMonday); got == "" {
		t.Fatal("expected last week's summary to be stored")
	}

	env.reports.WriteWeeklySummaries(t.Context(), mondayAfternoon.Add(time.Hour))
	if f.calls() != 1 {
		t.Fatalf("expected no regeneration, got %d calls", f.calls())
	}
}

func mondayOf(d time.Time) string {
	offset := (int(d.Weekday()) + 6) % 7
	return d.AddDate(0, 0, -offset).Format(model.DateFormat)
}

//go:build integration

package ai

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liftedkilt/openchore/internal/aibackend"
	"github.com/liftedkilt/openchore/internal/tts"
)

// Integration tests that start real AI backend (LiteRT or Ollama) and Kokoro containers.
//
// Run with:
//   go test -tags integration -v -timeout 10m ./internal/ai/
//
// First run will be slow (pulls model + container images).
// Set OLLAMA_MODEL to override the default model (gemma4:e4b).

var (
	testAIClient *aibackend.Client
	testTTSClient    *tts.Client
	testModel        string
	projectRoot      string
)

func TestMain(m *testing.M) {
	testModel = os.Getenv("OLLAMA_MODEL")
	if testModel == "" {
		testModel = "gemma4:e4b"
	}

	// Find project root (walk up looking for compose.yaml)
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "compose.yaml")); err == nil {
			projectRoot = dir
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			fmt.Fprintln(os.Stderr, "FATAL: could not find compose.yaml in any parent directory")
			os.Exit(1)
		}
		dir = parent
	}

	// If endpoints are already set externally, use those (skip container management)
	aiEP := os.Getenv("AI_ENDPOINT")
	if aiEP == "" {
		aiEP = os.Getenv("OLLAMA_ENDPOINT") // backward compat
	}
	ttsEP := os.Getenv("TTS_ENDPOINT")
	managedContainers := aiEP == "" && ttsEP == ""

	if managedContainers {
		fmt.Println("=== Starting AI containers (docker compose --profile ai) ===")
		if err := composeUp(); err != nil {
			fmt.Fprintf(os.Stderr, "FATAL: failed to start containers: %v\n", err)
			os.Exit(1)
		}
		aiEP = "http://localhost:28080"  // LiteRT sidecar (Ollama-compatible API)
		ttsEP = "http://localhost:28880"
	}

	testAIClient = aibackend.NewClient(aiEP)
	testTTSClient = tts.NewClient(ttsEP)

	// Wait for AI backend to be healthy
	fmt.Printf("=== Waiting for AI backend at %s ===\n", aiEP)
	if err := waitForHealthy(func(ctx context.Context) bool {
		return testAIClient.Healthy(ctx)
	}, 60*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: AI backend did not become healthy: %v\n", err)
		if managedContainers {
			composeDown()
		}
		os.Exit(1)
	}
	fmt.Println("=== AI backend is healthy ===")

	// Pull model if needed
	if !testAIClient.HasModel(context.Background(), testModel) {
		fmt.Printf("=== Pulling model %s (this may take several minutes) ===\n", testModel)
		if err := testAIClient.Pull(context.Background(), testModel); err != nil {
			fmt.Fprintf(os.Stderr, "FATAL: failed to pull model %s: %v\n", testModel, err)
			if managedContainers {
				composeDown()
			}
			os.Exit(1)
		}
		fmt.Printf("=== Model %s ready ===\n", testModel)
	} else {
		fmt.Printf("=== Model %s already available ===\n", testModel)
	}

	// Check if Kokoro is healthy (optional — TTS tests skip if unavailable)
	fmt.Printf("=== Checking Kokoro TTS at %s ===\n", ttsEP)
	if err := waitForHealthy(func(ctx context.Context) bool {
		return testTTSClient.Healthy(ctx)
	}, 90*time.Second); err != nil {
		fmt.Printf("=== Kokoro not available — TTS audio tests will be skipped ===\n")
		testTTSClient = nil
	} else {
		fmt.Println("=== Kokoro TTS is healthy ===")
	}

	// Run tests
	code := m.Run()

	// Tear down
	if managedContainers {
		fmt.Println("=== Stopping AI containers ===")
		composeDown()
	}

	os.Exit(code)
}

func composeUp() error {
	cmd := exec.Command("docker", "compose",
		"-f", "compose.yaml", "-f", "compose.integration-test.yaml",
		"--project-name", "openchore-test",
		"--profile", "ai",
		"up", "-d", "litert", "kokoro")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func composeDown() {
	// No -v flag: preserve volumes so models don't need to be re-pulled
	cmd := exec.Command("docker", "compose",
		"-f", "compose.yaml", "-f", "compose.integration-test.yaml",
		"--project-name", "openchore-test",
		"--profile", "ai",
		"down")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func waitForHealthy(check func(context.Context) bool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if check(ctx) {
			cancel()
			return nil
		}
		cancel()
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("not healthy after %s", timeout)
}

// --- Test helpers ---

func requireAIBackend(t *testing.T) {
	t.Helper()
	if testAIClient == nil {
		t.Fatal("AI backend client not initialized")
	}
}

func requireTTS(t *testing.T) {
	t.Helper()
	if testTTSClient == nil {
		t.Skip("Kokoro TTS not available — skipping")
	}
}

func testImage(t *testing.T) string {
	t.Helper()
	// Create a valid 100x100 white PNG using Go's image/png encoder
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.White)
		}
	}
	f, err := os.CreateTemp("", "test-chore-*.png")
	if err != nil {
		t.Fatalf("creating temp image: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encoding PNG: %v", err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

// --- Ollama / Photo Review Tests ---

func TestIntegration_ReviewPhoto(t *testing.T) {
	requireAIBackend(t)
	reviewer := NewReviewer(testAIClient, testModel)
	imgPath := testImage(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	result, err := reviewer.ReviewPhoto(ctx, "Make Bed", "Make your bed neatly with pillows arranged", imgPath)
	if err != nil {
		t.Fatalf("ReviewPhoto failed: %v", err)
	}

	t.Logf("Result: complete=%v, confidence=%.2f, feedback=%q", result.Complete, result.Confidence, result.Feedback)

	if result.Confidence < 0 || result.Confidence > 1 {
		t.Errorf("Confidence out of range [0,1]: %f", result.Confidence)
	}
	if result.Feedback == "" {
		t.Error("Expected non-empty feedback")
	}
	// A 1x1 white pixel should not confidently pass as a made bed
	if result.Complete && result.Confidence > 0.9 {
		t.Logf("WARNING: Model said bed is made from a 1x1 white pixel — may need prompt tuning")
	}
}

func TestIntegration_ReviewPhoto_MultipleChores(t *testing.T) {
	requireAIBackend(t)
	reviewer := NewReviewer(testAIClient, testModel)
	imgPath := testImage(t)

	chores := []struct{ title, desc string }{
		{"Clean Kitchen Table", "Wipe down the table and remove all dishes"},
		{"Take Out Trash", "Empty all trash cans and replace bags"},
		{"Sweep Floor", "Sweep the kitchen floor"},
	}

	for _, chore := range chores {
		t.Run(chore.title, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			result, err := reviewer.ReviewPhoto(ctx, chore.title, chore.desc, imgPath)
			if err != nil {
				t.Fatalf("ReviewPhoto failed for %q: %v", chore.title, err)
			}

			if result.Confidence < 0 || result.Confidence > 1 {
				t.Errorf("Confidence out of range: %f", result.Confidence)
			}
			if result.Feedback == "" {
				t.Errorf("Empty feedback for %q", chore.title)
			}
			t.Logf("%s: complete=%v confidence=%.2f feedback=%q",
				chore.title, result.Complete, result.Confidence, result.Feedback)
		})
	}
}

func TestIntegration_ReviewPhoto_FeedbackIsKidFriendly(t *testing.T) {
	requireAIBackend(t)
	reviewer := NewReviewer(testAIClient, testModel)
	imgPath := testImage(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	result, err := reviewer.ReviewPhoto(ctx, "Clean Room", "Pick up all toys and make the bed", imgPath)
	if err != nil {
		t.Fatalf("ReviewPhoto failed: %v", err)
	}

	// Feedback should be short and not contain technical jargon
	if len(result.Feedback) > 500 {
		t.Errorf("Feedback too long (%d chars) — should be 1-2 sentences for a child", len(result.Feedback))
	}
	lowered := strings.ToLower(result.Feedback)
	for _, bad := range []string{"json", "error", "exception", "null", "undefined", "api"} {
		if strings.Contains(lowered, bad) {
			t.Errorf("Feedback contains technical term %q — not kid-friendly: %s", bad, result.Feedback)
		}
	}
	t.Logf("Feedback: %q", result.Feedback)
}

// --- TTS Text Generation Tests ---

func TestIntegration_GenerateTTSDescription(t *testing.T) {
	requireAIBackend(t)
	gen := NewTTSGenerator(testAIClient, testModel, nil, "", "")

	var desc string
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		desc, err = gen.GenerateDescription(ctx, "Make Bed", "Make your bed neatly with pillows arranged")
		cancel()
		if err != nil {
			t.Fatalf("GenerateDescription failed: %v", err)
		}
		if desc != "" {
			break
		}
		t.Logf("Attempt %d: got empty description, retrying...", attempt)
	}

	t.Logf("TTS description: %q", desc)

	if desc == "" {
		t.Error("Expected non-empty description after 3 attempts")
	}
	if len(desc) > 500 {
		t.Errorf("Description too long (%d chars) — should be 1-2 sentences", len(desc))
	}
}

// --- Kokoro TTS Audio Tests ---

func TestIntegration_SynthesizeAudio(t *testing.T) {
	requireTTS(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	audio, err := testTTSClient.Synthesize(ctx, "Time to make your bed! Put the pillows up nice and neat.", "")
	if err != nil {
		t.Fatalf("Synthesize failed: %v", err)
	}

	t.Logf("Received %d bytes of audio", len(audio))

	if len(audio) < 100 {
		t.Errorf("Audio too small (%d bytes) — likely not valid", len(audio))
	}
}

func TestIntegration_SynthesizeAudio_MultipleVoices(t *testing.T) {
	requireTTS(t)

	voices := []string{"af_heart", "af_bella", "af_sarah"}
	for _, voice := range voices {
		t.Run(voice, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			audio, err := testTTSClient.Synthesize(ctx, "Great job cleaning your room!", voice)
			if err != nil {
				t.Fatalf("Synthesize with voice %s failed: %v", voice, err)
			}
			if len(audio) < 100 {
				t.Errorf("Audio too small for voice %s: %d bytes", voice, len(audio))
			}
			t.Logf("Voice %s: %d bytes", voice, len(audio))
		})
	}
}

// --- End-to-End Tests ---

func TestIntegration_EndToEnd_TTSGenAndSynth(t *testing.T) {
	requireAIBackend(t)
	requireTTS(t)

	gen := NewTTSGenerator(testAIClient, testModel, testTTSClient, "", "af_heart")

	// Retry up to 3 times — CPU inference can occasionally produce empty output
	var desc, audioURL string
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		desc, audioURL, err = gen.GenerateAndSynthesize(ctx, "Sweep Kitchen", "Sweep the kitchen floor clean", 999)
		cancel()
		if err != nil {
			t.Fatalf("GenerateAndSynthesize failed: %v", err)
		}
		if desc != "" {
			break
		}
		t.Logf("Attempt %d: got empty description, retrying...", attempt)
	}

	t.Logf("Description: %q", desc)
	t.Logf("Audio URL: %s", audioURL)

	if desc == "" {
		t.Error("Expected non-empty description after 3 attempts")
	}
	if audioURL == "" {
		// Audio URL may be empty if desc was empty (nothing to synthesize)
		if desc != "" {
			t.Error("Expected non-empty audio URL when description is present")
		}
	}
	if audioURL != "" && !strings.HasSuffix(audioURL, ".mp3") {
		t.Errorf("Expected .mp3 audio URL, got %s", audioURL)
	}

	// Clean up generated file
	if audioURL != "" {
		os.Remove("data" + audioURL)
	}
}

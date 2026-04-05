package ai

import (
	"strings"
	"testing"
)

func TestParseReviewResponse_ValidJSON(t *testing.T) {
	input := `{"complete": true, "confidence": 0.95, "feedback": "Great job!"}`
	result, err := parseReviewResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Complete {
		t.Error("expected complete=true")
	}
	if result.Confidence != 0.95 {
		t.Errorf("expected confidence=0.95, got %f", result.Confidence)
	}
	if result.Feedback != "Great job!" {
		t.Errorf("unexpected feedback: %s", result.Feedback)
	}
}

func TestParseReviewResponse_WrappedJSON(t *testing.T) {
	input := "Here is my review:\n{\"complete\": false, \"confidence\": 0.3, \"feedback\": \"Still needs work.\"}\nDone."
	result, err := parseReviewResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Complete {
		t.Error("expected complete=false")
	}
	if result.Feedback != "Still needs work." {
		t.Errorf("unexpected feedback: %s", result.Feedback)
	}
}

func TestParseReviewResponse_InvalidJSON(t *testing.T) {
	_, err := parseReviewResponse("this is not json at all")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseReviewResponse_EmptyString(t *testing.T) {
	_, err := parseReviewResponse("")
	if err == nil {
		t.Error("expected error for empty string")
	}
}

func TestParseReviewResponse_ClampsHighConfidence(t *testing.T) {
	input := `{"complete": true, "confidence": 1.5, "feedback": "Over confident"}`
	result, err := parseReviewResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Confidence != 1.0 {
		t.Errorf("expected confidence clamped to 1.0, got %f", result.Confidence)
	}
}

func TestParseReviewResponse_ClampsNegativeConfidence(t *testing.T) {
	input := `{"complete": false, "confidence": -0.5, "feedback": "Negative"}`
	result, err := parseReviewResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Confidence != 0 {
		t.Errorf("expected confidence clamped to 0, got %f", result.Confidence)
	}
}

func TestParseReviewResponse_ZeroConfidence(t *testing.T) {
	input := `{"complete": false, "confidence": 0.0, "feedback": "Cannot tell"}`
	result, err := parseReviewResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Confidence != 0 {
		t.Errorf("expected confidence=0, got %f", result.Confidence)
	}
}

func TestParseReviewResponse_ExactlyOne(t *testing.T) {
	input := `{"complete": true, "confidence": 1.0, "feedback": "Perfect"}`
	result, err := parseReviewResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Confidence != 1.0 {
		t.Errorf("expected confidence=1.0, got %f", result.Confidence)
	}
}

func TestParseReviewResponse_JSONWithWhitespace(t *testing.T) {
	input := `
	{
		"complete": true,
		"confidence": 0.8,
		"feedback": "Looks good"
	}
	`
	result, err := parseReviewResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Complete {
		t.Error("expected complete=true")
	}
	if result.Feedback != "Looks good" {
		t.Errorf("unexpected feedback: %s", result.Feedback)
	}
}

func TestParseReviewResponse_PartialJSON(t *testing.T) {
	// Incomplete JSON should fail
	_, err := parseReviewResponse(`{"complete": true, "confidence":`)
	if err == nil {
		t.Error("expected error for partial JSON")
	}
}

func TestBuildReviewPrompt(t *testing.T) {
	prompt := buildReviewPrompt("Make Bed", "Make your bed neatly with pillows")
	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
	if !strings.Contains(prompt, "Make Bed") {
		t.Error("prompt should contain chore title")
	}
	if !strings.Contains(prompt, "Make your bed neatly with pillows") {
		t.Error("prompt should contain chore description")
	}
	if !strings.Contains(prompt, "JSON") {
		t.Error("prompt should mention JSON format")
	}
	if !strings.Contains(prompt, "complete") {
		t.Error("prompt should mention complete field")
	}
	if !strings.Contains(prompt, "confidence") {
		t.Error("prompt should mention confidence field")
	}
	if !strings.Contains(prompt, "feedback") {
		t.Error("prompt should mention feedback field")
	}
}

func TestBuildReviewPrompt_NoDescription(t *testing.T) {
	prompt := buildReviewPrompt("Sweep Floor", "")
	if !strings.Contains(prompt, "Sweep Floor") {
		t.Error("prompt should contain chore title")
	}
	// With empty description, chore info should just be the title
	if !strings.Contains(prompt, "CHORE: Sweep Floor") {
		t.Error("prompt should have CHORE: Sweep Floor without description suffix")
	}
}

func TestBuildReviewPrompt_WithDescription(t *testing.T) {
	prompt := buildReviewPrompt("Dishes", "Wash all dishes and dry them")
	// With description, the chore info should include both
	if !strings.Contains(prompt, "Dishes: Wash all dishes and dry them") {
		t.Error("prompt should combine title and description with colon separator")
	}
}

func TestBuildReviewPrompt_ContainsGuidelines(t *testing.T) {
	prompt := buildReviewPrompt("Test", "test")
	if !strings.Contains(prompt, "encouraging") {
		t.Error("prompt should mention being encouraging")
	}
	if !strings.Contains(prompt, "photo") {
		t.Error("prompt should mention photo")
	}
}

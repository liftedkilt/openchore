package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/ollama"
)

// Reviewer uses an Ollama vision model to verify chore completion photos.
type Reviewer struct {
	client *ollama.Client
	model  string
}

// NewReviewer creates a new AI reviewer.
func NewReviewer(client *ollama.Client, model string) *Reviewer {
	return &Reviewer{client: client, model: model}
}

// ReviewPhoto sends a photo to the vision model and returns a structured review result.
// photoPath is the path on disk (e.g. "data/uploads/123_upload.jpg").
func (r *Reviewer) ReviewPhoto(ctx context.Context, choreTitle, choreDescription, photoPath string) (*model.AIReviewResult, error) {
	f, err := os.Open(photoPath)
	if err != nil {
		return nil, fmt.Errorf("opening photo %s: %w", photoPath, err)
	}
	defer f.Close()

	imageB64, err := ollama.EncodeImageBase64(f)
	if err != nil {
		return nil, fmt.Errorf("encoding photo: %w", err)
	}

	prompt := buildReviewPrompt(choreTitle, choreDescription)

	resp, err := r.client.Chat(ctx, &ollama.ChatRequest{
		Model: r.model,
		Messages: []ollama.ChatMessage{
			{
				Role:    "user",
				Content: prompt,
				Images:  []string{imageB64},
			},
		},
		Format: "json",
		Options: &ollama.ModelOptions{
			Temperature: 0.3, // low temperature for consistent evaluation
			NumPredict:  1024, // needs headroom for model thinking tokens
		},
	})
	if err != nil {
		return nil, fmt.Errorf("AI review request failed: %w", err)
	}

	result, err := parseReviewResponse(resp.Message.Content)
	if err != nil {
		return nil, fmt.Errorf("parsing AI response: %w", err)
	}

	log.Printf("ai: reviewed photo for %q — complete=%v confidence=%.2f", choreTitle, result.Complete, result.Confidence)
	return result, nil
}

func buildReviewPrompt(title, description string) string {
	choreInfo := title
	if description != "" {
		choreInfo += ": " + description
	}

	return fmt.Sprintf(`You are reviewing a photo submitted by a child as proof that a household chore has been completed. The child is showing you the RESULT of their work.

CHORE: %s

Evaluate whether the area shown in the photo looks like the chore has been done. You are judging the current STATE of things, not looking for the act of cleaning in progress.

For example:
- "Pick Up Toys" is complete if the room/area looks tidy and organized, even if some toys are visible on shelves or in bins — they just shouldn't be scattered on the floor
- "Make Bed" is complete if the bed looks made with covers pulled up, even if not perfect
- "Clean Kitchen Table" is complete if the table surface is mostly clear and wiped down
- "Sweep Floor" is complete if the floor looks clean and free of visible debris

Respond ONLY with a JSON object in this exact format, no markdown, no code fences:
{"complete": true or false, "confidence": 0.0 to 1.0, "feedback": "brief kind message"}

Guidelines:
- Be encouraging and age-appropriate — these are kids aged 5-12
- Be GENEROUS: if it looks reasonably done, mark it complete. Kids deserve credit for effort
- Judge the state of the area, not whether you can see the cleaning happening
- If the photo is blurry or unclear, set confidence low and ask kindly for a clearer photo
- If complete is true, acknowledge the good work
- If complete is false, explain specifically and kindly what still needs attention
- Keep feedback to 1-2 sentences max`, choreInfo)
}

func parseReviewResponse(raw string) (*model.AIReviewResult, error) {
	raw = strings.TrimSpace(raw)

	var result model.AIReviewResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		// Try to extract JSON from the response if it's wrapped in other text
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start >= 0 && end > start {
			if err2 := json.Unmarshal([]byte(raw[start:end+1]), &result); err2 != nil {
				return nil, fmt.Errorf("could not parse AI response as JSON: %s", raw)
			}
		} else {
			return nil, fmt.Errorf("could not parse AI response as JSON: %s", raw)
		}
	}

	// Clamp confidence to [0, 1]
	if result.Confidence < 0 {
		result.Confidence = 0
	}
	if result.Confidence > 1 {
		result.Confidence = 1
	}

	return &result, nil
}

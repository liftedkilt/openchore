package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/liftedkilt/openchore/internal/aibackend"
)

// DescriptionGenerator uses an LLM to auto-generate chore descriptions and suggest point values.
type DescriptionGenerator struct {
	client *aibackend.Client
	model  string
}

// NewDescriptionGenerator creates a new description generator.
func NewDescriptionGenerator(client *aibackend.Client, model string) *DescriptionGenerator {
	return &DescriptionGenerator{client: client, model: model}
}

// GenerateDescription creates a brief, kid-friendly description for a chore based on its title and category.
func (g *DescriptionGenerator) GenerateDescription(ctx context.Context, title, category string) (string, error) {
	prompt := fmt.Sprintf("Write a brief 1-2 sentence description of this household chore for kids. Title: %s, Category: %s. Respond with ONLY the description.", title, category)

	resp, err := g.client.Generate(ctx, &aibackend.GenerateRequest{
		Model:  g.model,
		Prompt: prompt,
		Options: &aibackend.ModelOptions{
			Temperature: 0.7,
			NumPredict:  256,
		},
	})
	if err != nil {
		return "", fmt.Errorf("generating description: %w", err)
	}

	desc := strings.TrimSpace(resp.Response)
	desc = strings.Trim(desc, "\"'")
	log.Printf("ai: generated description for %q: %s", title, desc)
	return desc, nil
}

// PointSuggestion holds the AI's recommended point value and time estimate for a chore.
type PointSuggestion struct {
	Points           int    `json:"points"`
	EstimatedMinutes int    `json:"estimated_minutes"`
	Reasoning        string `json:"reasoning"`
}

// SuggestPoints asks the AI to recommend a point value and time estimate for a chore.
func (g *DescriptionGenerator) SuggestPoints(ctx context.Context, title, description, category string) (points int, minutes int, reasoning string, err error) {
	prompt := fmt.Sprintf(`Suggest a point value and time estimate for this household chore assigned to children.

Title: %s
Description: %s
Category: %s

Point scale guidelines:
- required chores: 5-10 points (daily essentials like making bed, brushing teeth)
- core chores: 5-15 points (regular household tasks like dishes, vacuuming)
- bonus chores: 10-30 points (extra tasks like cleaning garage, organizing closet)

Respond with ONLY a JSON object in this exact format, no markdown, no code fences:
{"points": N, "estimated_minutes": N, "reasoning": "brief explanation"}`, title, description, category)

	resp, err := g.client.Generate(ctx, &aibackend.GenerateRequest{
		Model:  g.model,
		Prompt: prompt,
		Format: "json",
		Options: &aibackend.ModelOptions{
			Temperature: 0.3,
			NumPredict:  512,
		},
	})
	if err != nil {
		return 0, 0, "", fmt.Errorf("suggesting points: %w", err)
	}

	raw := strings.TrimSpace(resp.Response)
	var suggestion PointSuggestion
	if jsonErr := json.Unmarshal([]byte(raw), &suggestion); jsonErr != nil {
		// Try to extract JSON from the response if it's wrapped in other text
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start >= 0 && end > start {
			if jsonErr2 := json.Unmarshal([]byte(raw[start:end+1]), &suggestion); jsonErr2 != nil {
				return 0, 0, "", fmt.Errorf("parsing AI point suggestion: %s", raw)
			}
		} else {
			return 0, 0, "", fmt.Errorf("parsing AI point suggestion: %s", raw)
		}
	}

	// Clamp to reasonable ranges
	if suggestion.Points < 1 {
		suggestion.Points = 1
	}
	if suggestion.Points > 50 {
		suggestion.Points = 50
	}
	if suggestion.EstimatedMinutes < 1 {
		suggestion.EstimatedMinutes = 1
	}
	if suggestion.EstimatedMinutes > 120 {
		suggestion.EstimatedMinutes = 120
	}

	log.Printf("ai: suggested %d pts / %d min for %q (%s): %s", suggestion.Points, suggestion.EstimatedMinutes, title, category, suggestion.Reasoning)
	return suggestion.Points, suggestion.EstimatedMinutes, suggestion.Reasoning, nil
}

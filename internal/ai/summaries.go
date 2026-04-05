package ai

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/liftedkilt/openchore/internal/aibackend"
)

// Summarizer uses an LLM to generate encouraging weekly performance summaries for parents.
type Summarizer struct {
	client *aibackend.Client
	model  string
}

// NewSummarizer creates a new AI summarizer.
func NewSummarizer(client *aibackend.Client, model string) *Summarizer {
	return &Summarizer{client: client, model: model}
}

// WeeklyStats holds the data needed to generate a weekly summary for one child.
type WeeklyStats struct {
	KidName        string
	CompletedCount int
	MissedCount    int
	TotalAssigned  int
	PointsEarned   int
	CurrentStreak  int
	CompletionRate float64
	TopChores      []string
	MissedChores   []string
}

// WeeklySummary generates an encouraging parent-facing summary of a child's chore performance.
func (s *Summarizer) WeeklySummary(ctx context.Context, stats WeeklyStats) (string, error) {
	topStr := "none"
	if len(stats.TopChores) > 0 {
		topStr = strings.Join(stats.TopChores, ", ")
	}
	missedStr := "none"
	if len(stats.MissedChores) > 0 {
		missedStr = strings.Join(stats.MissedChores, ", ")
	}

	prompt := fmt.Sprintf(`Write a brief, encouraging 2-3 sentence summary of this child's chore performance for their parent. Kid: %s, completed %d/%d (%.0f%%), earned %d points, %d-day streak. Most completed: %s. Most missed: %s. Be specific and actionable. Respond with ONLY the summary.`,
		stats.KidName, stats.CompletedCount, stats.TotalAssigned, stats.CompletionRate,
		stats.PointsEarned, stats.CurrentStreak, topStr, missedStr)

	resp, err := s.client.Generate(ctx, &aibackend.GenerateRequest{
		Model:  s.model,
		Prompt: prompt,
		Options: &aibackend.ModelOptions{
			Temperature: 0.7,
			NumPredict:  512,
		},
	})
	if err != nil {
		return "", fmt.Errorf("generating weekly summary: %w", err)
	}

	summary := strings.TrimSpace(resp.Response)
	summary = strings.Trim(summary, "\"'")
	log.Printf("ai: generated weekly summary for %s: %s", stats.KidName, summary)
	return summary, nil
}

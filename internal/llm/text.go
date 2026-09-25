package llm

import (
	"context"
	"fmt"
	"strings"
)

// DraftDescription writes a short chore description that works both on
// screen and read aloud to a young child.
func (c *Client) DraftDescription(ctx context.Context, title, category string) (string, error) {
	return c.ask(ctx, fmt.Sprintf(`Write a 1-2 sentence description of the household chore %q (category: %s) for a child aged 5-12. Use simple words, say what "done" looks like, and keep it friendly. It will be shown on screen and read aloud. Reply with only the description.`, title, category))
}

// WeeklyStats holds what the weekly summary is written from.
type WeeklyStats struct {
	KidName        string
	CompletedCount int
	MissedCount    int
	TotalAssigned  int
	PointsEarned   int
	CurrentStreak  int
	CompletionRate float64
	MissedChores   []string
}

// WeeklySummary writes an encouraging 2-3 sentence summary for a parent.
func (c *Client) WeeklySummary(ctx context.Context, s WeeklyStats) (string, error) {
	missed := "none"
	if len(s.MissedChores) > 0 {
		missed = strings.Join(s.MissedChores, ", ")
	}
	return c.ask(ctx, fmt.Sprintf(`Write a brief, encouraging 2-3 sentence summary of this child's chores this week for their parent. Kid: %s. Completed %d of %d (%.0f%%), %d missed, earned %d points, current streak %d days. Most missed: %s. Be specific and suggest one small, practical next step. Reply with only the summary.`,
		s.KidName, s.CompletedCount, s.TotalAssigned, s.CompletionRate, s.MissedCount, s.PointsEarned, s.CurrentStreak, missed))
}

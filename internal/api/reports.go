package api

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/liftedkilt/openchore/internal/discord"
	"github.com/liftedkilt/openchore/internal/llm"
	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/internal/webhook"
)

type ReportsHandler struct {
	store      *store.Store
	dispatcher *webhook.Dispatcher
	discord    *discord.Notifier
	ai         *llm.Client // nil when AI_BASE_URL is unset
}

func NewReportsHandler(s *store.Store, d *webhook.Dispatcher, dn *discord.Notifier) *ReportsHandler {
	return &ReportsHandler{store: s, dispatcher: d, discord: dn}
}

// SetAI wires in the optional AI client used for narrative summaries.
func (h *ReportsHandler) SetAI(ai *llm.Client) {
	h.ai = ai
}

// ReportsResponse is the full payload returned by GET /api/admin/reports.
type ReportsResponse struct {
	Period     string          `json:"period"`
	StartDate  string         `json:"start_date"`
	EndDate    string         `json:"end_date"`
	Kids       []KidSummary   `json:"kids"`
	MostMissed []MissedChore  `json:"most_missed"`
	Trend      []TrendDay     `json:"trend"`
	Categories []CategoryStat `json:"categories"`
	Points     []PointsSummary `json:"points"`
	DayOfWeek  []DayOfWeekStat `json:"day_of_week"`
}

type KidSummary struct {
	UserID         int64   `json:"user_id"`
	Name           string  `json:"name"`
	AvatarURL      string  `json:"avatar_url"`
	TotalAssigned  int     `json:"total_assigned"`
	TotalCompleted int     `json:"total_completed"`
	TotalMissed    int     `json:"total_missed"`
	CompletionRate float64 `json:"completion_rate"`
	PointsEarned   int     `json:"points_earned"`
	CurrentStreak  int     `json:"current_streak"`
}

type MissedChore struct {
	ChoreID   int64    `json:"chore_id"`
	ChoreName string   `json:"chore_name"`
	MissCount int      `json:"miss_count"`
	Kids      []string `json:"kids"`
}

type TrendDay struct {
	Date      string `json:"date"`
	Completed int    `json:"completed"`
	Assigned  int    `json:"assigned"`
}

type CategoryStat struct {
	Category       string  `json:"category"`
	TotalAssigned  int     `json:"total_assigned"`
	TotalCompleted int     `json:"total_completed"`
	CompletionRate float64 `json:"completion_rate"`
}

type PointsSummary struct {
	UserID        int64  `json:"user_id"`
	Name          string `json:"name"`
	PointsEarned  int    `json:"points_earned"`
	PointsDecayed int    `json:"points_decayed"`
	PointsSpent   int    `json:"points_spent"`
}

type DayOfWeekStat struct {
	DayOfWeek      int     `json:"day_of_week"`
	DayName        string  `json:"day_name"`
	TotalAssigned  int     `json:"total_assigned"`
	TotalCompleted int     `json:"total_completed"`
	CompletionRate float64 `json:"completion_rate"`
}

var dayNames = [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

func (h *ReportsHandler) GetReports(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "week"
	}
	if period != "week" && period != "month" && period != "year" {
		writeError(w, http.StatusBadRequest, "period must be week, month, or year")
		return
	}

	dateStr := r.URL.Query().Get("date")
	var refDate time.Time
	if dateStr != "" {
		var err error
		refDate, err = time.Parse(model.DateFormat, dateStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid date format, use YYYY-MM-DD")
			return
		}
	} else {
		refDate = time.Now()
	}

	startDate, endDate := periodRange(period, refDate)
	startStr := startDate.Format(model.DateFormat)
	endStr := endDate.Format(model.DateFormat)

	kidRows, err := h.store.ReportKidSummaries(r.Context(), startStr, endStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get kid summaries")
		return
	}
	kids := make([]KidSummary, 0, len(kidRows))
	for _, k := range kidRows {
		missed := k.TotalAssigned - k.TotalCompleted
		if missed < 0 {
			missed = 0
		}
		rate := 0.0
		if k.TotalAssigned > 0 {
			rate = float64(k.TotalCompleted) / float64(k.TotalAssigned) * 100
		}
		kids = append(kids, KidSummary{
			UserID:         k.UserID,
			Name:           k.Name,
			AvatarURL:      k.AvatarURL,
			TotalAssigned:  k.TotalAssigned,
			TotalCompleted: k.TotalCompleted,
			TotalMissed:    missed,
			CompletionRate: rate,
			PointsEarned:   k.PointsEarned,
			CurrentStreak:  k.CurrentStreak,
		})
	}

	missedRows, err := h.store.ReportMostMissed(r.Context(), startStr, endStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get missed chores")
		return
	}
	mostMissed := make([]MissedChore, 0, len(missedRows))
	for _, m := range missedRows {
		kidList := strings.Split(m.Kids, ",")
		mostMissed = append(mostMissed, MissedChore{
			ChoreID:   m.ChoreID,
			ChoreName: m.ChoreName,
			MissCount: m.MissCount,
			Kids:      kidList,
		})
	}

	trendRows, err := h.store.ReportCompletionTrend(r.Context(), startStr, endStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get trend")
		return
	}
	trend := make([]TrendDay, 0, len(trendRows))
	for _, t := range trendRows {
		trend = append(trend, TrendDay{
			Date:      t.Date,
			Completed: t.Completed,
			Assigned:  t.Assigned,
		})
	}

	catRows, err := h.store.ReportCategoryBreakdown(r.Context(), startStr, endStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get category breakdown")
		return
	}
	categories := make([]CategoryStat, 0, len(catRows))
	for _, c := range catRows {
		rate := 0.0
		if c.TotalAssigned > 0 {
			rate = float64(c.TotalCompleted) / float64(c.TotalAssigned) * 100
		}
		categories = append(categories, CategoryStat{
			Category:       c.Category,
			TotalAssigned:  c.TotalAssigned,
			TotalCompleted: c.TotalCompleted,
			CompletionRate: rate,
		})
	}

	ptRows, err := h.store.ReportPointsSummary(r.Context(), startStr, endStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get points summary")
		return
	}
	points := make([]PointsSummary, 0, len(ptRows))
	for _, p := range ptRows {
		points = append(points, PointsSummary{
			UserID:        p.UserID,
			Name:          p.Name,
			PointsEarned:  p.PointsEarned,
			PointsDecayed: p.PointsDecayed,
			PointsSpent:   p.PointsSpent,
		})
	}

	dowRows, err := h.store.ReportDayOfWeek(r.Context(), startStr, endStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get day of week stats")
		return
	}
	dowStats := make([]DayOfWeekStat, 0, len(dowRows))
	for _, d := range dowRows {
		rate := 0.0
		if d.TotalAssigned > 0 {
			rate = float64(d.TotalCompleted) / float64(d.TotalAssigned) * 100
		}
		name := ""
		if d.DayOfWeek >= 0 && d.DayOfWeek < 7 {
			name = dayNames[d.DayOfWeek]
		}
		dowStats = append(dowStats, DayOfWeekStat{
			DayOfWeek:      d.DayOfWeek,
			DayName:        name,
			TotalAssigned:  d.TotalAssigned,
			TotalCompleted: d.TotalCompleted,
			CompletionRate: rate,
		})
	}

	resp := ReportsResponse{
		Period:     period,
		StartDate:  startStr,
		EndDate:    endStr,
		Kids:       kids,
		MostMissed: mostMissed,
		Trend:      trend,
		Categories: categories,
		Points:     points,
		DayOfWeek:  dowStats,
	}
	writeJSON(w, http.StatusOK, resp)
}

// periodRange computes the inclusive [start, end] dates for a given period.
func periodRange(period string, ref time.Time) (time.Time, time.Time) {
	switch period {
	case "week":
		// Monday through Sunday
		weekday := ref.Weekday()
		if weekday == time.Sunday {
			weekday = 7
		}
		start := ref.AddDate(0, 0, -int(weekday)+1)
		end := start.AddDate(0, 0, 6)
		return start, end
	case "month":
		start := time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, ref.Location())
		end := start.AddDate(0, 1, -1)
		return start, end
	case "year":
		start := time.Date(ref.Year(), 1, 1, 0, 0, 0, 0, ref.Location())
		end := time.Date(ref.Year(), 12, 31, 0, 0, 0, 0, ref.Location())
		return start, end
	default:
		return ref, ref
	}
}

// GetAISummary returns a narrative summary of one person's period. Summaries
// of finished weeks are generated once and kept; anything else is written
// on demand.
func (h *ReportsHandler) GetAISummary(w http.ResponseWriter, r *http.Request) {
	if h.ai == nil {
		writeError(w, http.StatusServiceUnavailable, "AI is not configured")
		return
	}

	userID, err := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "week"
	}
	if period != "week" && period != "month" && period != "year" {
		writeError(w, http.StatusBadRequest, "period must be week, month, or year")
		return
	}

	refDate := time.Now()
	if dateStr := r.URL.Query().Get("date"); dateStr != "" {
		refDate, err = time.ParseInLocation(model.DateFormat, dateStr, time.Local)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid date format, use YYYY-MM-DD")
			return
		}
	}
	start, end := periodRange(period, refDate)
	startStr, endStr := start.Format(model.DateFormat), end.Format(model.DateFormat)

	// A finished week's summary never changes, so reuse it.
	finishedWeek := period == "week" && endStr < time.Now().Format(model.DateFormat)
	if finishedWeek {
		if cached, err := h.store.GetWeeklySummary(r.Context(), userID, startStr); err == nil && cached != "" {
			writeJSON(w, http.StatusOK, map[string]string{"summary": cached})
			return
		}
	}

	stats, err := h.summaryStats(r.Context(), userID, startStr, endStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get report data")
		return
	}
	if stats == nil {
		writeError(w, http.StatusNotFound, "no data for this user in the selected period")
		return
	}

	summary, err := h.ai.WeeklySummary(r.Context(), *stats)
	if err != nil {
		writeError(w, http.StatusBadGateway, "AI summary generation failed: "+err.Error())
		return
	}
	if finishedWeek {
		if err := h.store.SaveWeeklySummary(r.Context(), userID, startStr, summary); err != nil {
			log.Printf("ai: saving weekly summary for user %d: %v", userID, err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"summary": summary})
}

// summaryStats gathers what a summary is written from, or nil if the user
// has no report row for the range.
func (h *ReportsHandler) summaryStats(ctx context.Context, userID int64, startStr, endStr string) (*llm.WeeklyStats, error) {
	kidRows, err := h.store.ReportKidSummaries(ctx, startStr, endStr)
	if err != nil {
		return nil, err
	}
	var kid *store.KidSummaryRow
	for i := range kidRows {
		if kidRows[i].UserID == userID {
			kid = &kidRows[i]
			break
		}
	}
	if kid == nil {
		return nil, nil
	}
	rate := 0.0
	if kid.TotalAssigned > 0 {
		rate = float64(kid.TotalCompleted) / float64(kid.TotalAssigned) * 100
	}

	var missedChores []string
	missedRows, _ := h.store.ReportMostMissed(ctx, startStr, endStr)
	for _, m := range missedRows {
		for _, k := range strings.Split(m.Kids, ",") {
			if strings.TrimSpace(k) == kid.Name {
				missedChores = append(missedChores, m.ChoreName)
				break
			}
		}
		if len(missedChores) >= 3 {
			break
		}
	}

	return &llm.WeeklyStats{
		KidName:        kid.Name,
		CompletedCount: kid.TotalCompleted,
		MissedCount:    max(kid.TotalAssigned-kid.TotalCompleted, 0),
		TotalAssigned:  kid.TotalAssigned,
		PointsEarned:   kid.PointsEarned,
		CurrentStreak:  kid.CurrentStreak,
		CompletionRate: rate,
		MissedChores:   missedChores,
	}, nil
}

// StartWeeklySummaries writes each person's summary of last week once it is
// over and shares it via webhooks and Discord. It only acts while AI is
// configured and the ai_weekly_summary setting is on. Blocks until ctx ends.
func (h *ReportsHandler) StartWeeklySummaries(ctx context.Context) {
	if h.ai == nil {
		return
	}
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		h.WriteWeeklySummaries(ctx, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// WriteWeeklySummaries generates any missing summaries for the week before
// now. It waits until Monday noon so late approvals for Sunday count.
// StartWeeklySummaries calls it hourly.
func (h *ReportsHandler) WriteWeeklySummaries(ctx context.Context, now time.Time) {
	if on, _ := h.store.GetSetting(ctx, "ai_weekly_summary"); on != "true" {
		return
	}
	thisWeek, _ := periodRange("week", now)
	thisWeek = time.Date(thisWeek.Year(), thisWeek.Month(), thisWeek.Day(), 0, 0, 0, 0, now.Location())
	if now.Before(thisWeek.Add(12 * time.Hour)) {
		return
	}
	start, end := periodRange("week", thisWeek.AddDate(0, 0, -7))
	startStr, endStr := start.Format(model.DateFormat), end.Format(model.DateFormat)

	kids, err := h.store.ReportKidSummaries(ctx, startStr, endStr)
	if err != nil {
		log.Printf("ai: weekly summaries: %v", err)
		return
	}
	for _, kid := range kids {
		if ctx.Err() != nil {
			return
		}
		if kid.TotalAssigned == 0 {
			continue // nothing happened; nothing to summarize
		}
		if cached, err := h.store.GetWeeklySummary(ctx, kid.UserID, startStr); err != nil || cached != "" {
			continue
		}
		stats, err := h.summaryStats(ctx, kid.UserID, startStr, endStr)
		if err != nil || stats == nil {
			continue
		}
		summary, err := h.ai.WeeklySummary(ctx, *stats)
		if err != nil {
			log.Printf("ai: weekly summary for %s failed: %v", kid.Name, err)
			continue
		}
		if err := h.store.SaveWeeklySummary(ctx, kid.UserID, startStr, summary); err != nil {
			log.Printf("ai: saving weekly summary for %s: %v", kid.Name, err)
			continue
		}
		log.Printf("ai: wrote weekly summary for %s (week of %s)", kid.Name, startStr)
		h.dispatcher.Fire(webhook.EventWeeklySummary, map[string]any{
			"user_id":    kid.UserID,
			"user_name":  kid.Name,
			"week_start": startStr,
			"week_end":   endStr,
			"summary":    summary,
		})
		h.discord.NotifyWeeklySummary(kid.Name, startStr, summary)
	}
}

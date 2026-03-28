package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

type ReportsHandler struct {
	store *store.Store
}

func NewReportsHandler(s *store.Store) *ReportsHandler {
	return &ReportsHandler{store: s}
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

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

type Store struct {
	db *sql.DB
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// --- Users ---

func (s *Store) CreateUser(ctx context.Context, u *model.User) error {
	paused := boolToInt(u.Paused)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (name, avatar_url, role, age, theme, line_color, paused, pin_hash) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		u.Name, u.AvatarURL, u.Role, u.Age, u.Theme, u.LineColor, paused, u.PinHash)
	if err != nil {
		return err
	}
	u.ID, _ = res.LastInsertId()
	u.HasPin = u.PinHash != ""
	return nil
}

func (s *Store) GetUser(ctx context.Context, id int64) (*model.User, error) {
	u := &model.User{}
	var paused int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, avatar_url, role, age, theme, line_color, paused, pin_hash, created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Name, &u.AvatarURL, &u.Role, &u.Age, &u.Theme, &u.LineColor, &paused, &u.PinHash, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	u.Paused = paused == 1
	u.HasPin = u.PinHash != ""
	return u, err
}

func (s *Store) ListUsers(ctx context.Context) ([]model.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, avatar_url, role, age, theme, line_color, paused, pin_hash, created_at FROM users ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []model.User
	for rows.Next() {
		var u model.User
		var paused int
		if err := rows.Scan(&u.ID, &u.Name, &u.AvatarURL, &u.Role, &u.Age, &u.Theme, &u.LineColor, &paused, &u.PinHash, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.Paused = paused == 1
		u.HasPin = u.PinHash != ""
		users = append(users, u)
	}
	return users, rows.Err()
}

// --- Chores ---

func (s *Store) CreateChore(ctx context.Context, c *model.Chore) error {
	requiresApproval := boolToInt(c.RequiresApproval)
	requiresPhoto := boolToInt(c.RequiresPhoto)
	photoSource := c.PhotoSource
	if photoSource == "" {
		photoSource = model.PhotoSourceChild
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO chores (title, description, category, icon, points_value, missed_penalty_value, estimated_minutes, requires_approval, requires_photo, photo_source, source, external_id, tts_description, tts_audio_url, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Title, c.Description, c.Category, c.Icon, c.PointsValue, c.MissedPenaltyValue, c.EstimatedMinutes, requiresApproval, requiresPhoto, photoSource, c.Source, c.ExternalID, c.TTSDescription, c.TTSAudioURL, c.CreatedBy)
	if err != nil {
		return err
	}
	c.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) GetChore(ctx context.Context, id int64) (*model.Chore, error) {
	c := &model.Chore{}
	var reqApp, reqPho int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, title, description, category, icon, points_value, missed_penalty_value, estimated_minutes, requires_approval, requires_photo, photo_source, source, external_id, tts_description, tts_audio_url, created_by, created_at
		 FROM chores WHERE id = ?`, id).
		Scan(&c.ID, &c.Title, &c.Description, &c.Category, &c.Icon, &c.PointsValue, &c.MissedPenaltyValue, &c.EstimatedMinutes, &reqApp, &reqPho, &c.PhotoSource, &c.Source, &c.ExternalID, &c.TTSDescription, &c.TTSAudioURL, &c.CreatedBy, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	c.RequiresApproval = reqApp == 1
	c.RequiresPhoto = reqPho == 1
	return c, err
}

func (s *Store) ListChores(ctx context.Context) ([]model.Chore, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, description, category, icon, points_value, missed_penalty_value, estimated_minutes, requires_approval, requires_photo, photo_source, source, external_id, tts_description, tts_audio_url, created_by, created_at
		 FROM chores ORDER BY title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chores []model.Chore
	for rows.Next() {
		var c model.Chore
		var reqApp, reqPho int
		if err := rows.Scan(&c.ID, &c.Title, &c.Description, &c.Category, &c.Icon, &c.PointsValue, &c.MissedPenaltyValue, &c.EstimatedMinutes, &reqApp, &reqPho, &c.PhotoSource, &c.Source, &c.ExternalID, &c.TTSDescription, &c.TTSAudioURL, &c.CreatedBy, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.RequiresApproval = reqApp == 1
		c.RequiresPhoto = reqPho == 1
		chores = append(chores, c)
	}
	return chores, rows.Err()
}

func (s *Store) UpdateChore(ctx context.Context, c *model.Chore) error {
	requiresApproval := boolToInt(c.RequiresApproval)
	requiresPhoto := boolToInt(c.RequiresPhoto)
	photoSource := c.PhotoSource
	if photoSource == "" {
		photoSource = model.PhotoSourceChild
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE chores SET title=?, description=?, category=?, icon=?, points_value=?, missed_penalty_value=?, estimated_minutes=?, requires_approval=?, requires_photo=?, photo_source=?, source=?, external_id=?, tts_description=?, tts_audio_url=?
		 WHERE id=?`,
		c.Title, c.Description, c.Category, c.Icon, c.PointsValue, c.MissedPenaltyValue, c.EstimatedMinutes, requiresApproval, requiresPhoto, photoSource, c.Source, c.ExternalID, c.TTSDescription, c.TTSAudioURL, c.ID)
	return err
}

func (s *Store) DeleteChore(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM chores WHERE id = ?`, id)
	return err
}

// UpdateChoreTTSDescription updates only the TTS description for a chore.
func (s *Store) UpdateChoreTTSDescription(ctx context.Context, choreID int64, desc string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE chores SET tts_description = ? WHERE id = ?`, desc, choreID)
	return err
}

// UpdateChoreTTSAudioURL updates only the TTS audio URL for a chore.
func (s *Store) UpdateChoreTTSAudioURL(ctx context.Context, choreID int64, url string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE chores SET tts_audio_url = ? WHERE id = ?`, url, choreID)
	return err
}

// --- Schedules ---

func (s *Store) CreateSchedule(ctx context.Context, cs *model.ChoreSchedule) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO chore_schedules (chore_id, assigned_to, assignment_type, fcfs_group_id, day_of_week, specific_date, available_at, points_multiplier, start_date, end_date, recurrence_interval, recurrence_start, due_by, expiry_penalty, expiry_penalty_value)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cs.ChoreID, cs.AssignedTo, cs.AssignmentType, cs.FcfsGroupID, cs.DayOfWeek, cs.SpecificDate, cs.AvailableAt, cs.PointsMultiplier, cs.StartDate, cs.EndDate, cs.RecurrenceInterval, cs.RecurrenceStart, cs.DueBy, cs.ExpiryPenalty, cs.ExpiryPenaltyValue)
	if err != nil {
		return err
	}
	cs.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) DeleteSchedule(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM chore_schedules WHERE id = ?`, id)
	return err
}

// ScheduleExistsForDate checks if a one-off schedule already exists for a chore + user + date.
func (s *Store) ScheduleExistsForDate(ctx context.Context, choreID, userID int64, date string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM chore_schedules WHERE chore_id = ? AND assigned_to = ? AND specific_date = ?)`,
		choreID, userID, date).Scan(&exists)
	return exists, err
}

func (s *Store) ListSchedulesForChore(ctx context.Context, choreID int64) ([]model.ChoreSchedule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, chore_id, assigned_to, assignment_type, fcfs_group_id, day_of_week, specific_date, available_at, points_multiplier, start_date, end_date, recurrence_interval, recurrence_start, due_by, expiry_penalty, expiry_penalty_value, created_at
		 FROM chore_schedules WHERE chore_id = ? ORDER BY id`, choreID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var schedules []model.ChoreSchedule
	for rows.Next() {
		var cs model.ChoreSchedule
		if err := rows.Scan(&cs.ID, &cs.ChoreID, &cs.AssignedTo, &cs.AssignmentType, &cs.FcfsGroupID, &cs.DayOfWeek, &cs.SpecificDate, &cs.AvailableAt, &cs.PointsMultiplier, &cs.StartDate, &cs.EndDate, &cs.RecurrenceInterval, &cs.RecurrenceStart, &cs.DueBy, &cs.ExpiryPenalty, &cs.ExpiryPenaltyValue, &cs.CreatedAt); err != nil {
			return nil, err
		}
		schedules = append(schedules, cs)
	}
	return schedules, rows.Err()
}

// GetScheduledChoresForUser returns all chores for a user on the given dates, with completion status.
func (s *Store) GetScheduledChoresForUser(ctx context.Context, userID int64, dates []string, now time.Time) ([]model.ScheduledChore, error) {
	if len(dates) == 0 {
		return nil, nil
	}

	query := `
		SELECT
			cs.id, c.id, c.title, c.description, c.category, c.icon, c.points_value, c.missed_penalty_value, c.estimated_minutes, c.requires_approval, c.requires_photo, c.photo_source,
			cs.assignment_type, cs.available_at, cs.due_by, cs.expiry_penalty, cs.expiry_penalty_value,
			cs.day_of_week, cs.specific_date,
			cc.id, cc.completed_at, cc.photo_url, cc.status, cc.ai_feedback,
			c.tts_description, c.tts_audio_url,
			(SELECT u2.name FROM chore_completions cc2
			 JOIN chore_schedules cs2 ON cs2.id = cc2.chore_schedule_id
			 JOIN users u2 ON u2.id = cc2.completed_by
			 WHERE cs2.fcfs_group_id = cs.fcfs_group_id
			   AND cs2.fcfs_group_id IS NOT NULL
			   AND cs2.id != cs.id
			   AND cc2.completion_date = ?
			   AND cc2.status != 'ai_rejected'
			 LIMIT 1) as completed_by_sibling_name
		FROM chore_schedules cs
		JOIN chores c ON c.id = cs.chore_id
		LEFT JOIN chore_completions cc ON cc.id = (
				SELECT cc3.id FROM chore_completions cc3
				WHERE cc3.chore_schedule_id = cs.id AND cc3.completion_date = ?
				ORDER BY CASE cc3.status
					WHEN 'approved' THEN 1
					WHEN 'pending'  THEN 2
					WHEN 'rejected' THEN 3
					WHEN 'ai_rejected' THEN 4
				END
				LIMIT 1
			)
		WHERE cs.assigned_to = ?
		  AND (
			(cs.day_of_week = ? AND cs.specific_date IS NULL AND cs.recurrence_interval IS NULL)
			OR (cs.specific_date = ? AND cs.recurrence_interval IS NULL)
			OR (cs.recurrence_interval IS NOT NULL AND cs.recurrence_start IS NOT NULL
				AND CAST((julianday(?) - julianday(cs.recurrence_start)) AS INTEGER) >= 0
				AND CAST((julianday(?) - julianday(cs.recurrence_start)) AS INTEGER) % cs.recurrence_interval = 0)
		  )
		  AND (cs.start_date IS NULL OR cs.start_date <= ?)
		  AND (cs.end_date IS NULL OR cs.end_date >= ?)
		ORDER BY cs.available_at NULLS FIRST, c.category, c.title`

	var results []model.ScheduledChore
	currentTime := now.Format("15:04")

	for _, dateStr := range dates {
		t, err := time.Parse(model.DateFormat, dateStr)
		if err != nil {
			return nil, fmt.Errorf("invalid date %s: %w", dateStr, err)
		}
		dow := int(t.Weekday())
		rows, err := s.db.QueryContext(ctx, query, dateStr, dateStr, userID, dow, dateStr, dateStr, dateStr, dateStr, dateStr)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var sc model.ScheduledChore
			var compID sql.NullInt64
			var dayOfWeek sql.NullInt64
			var specificDate sql.NullString
			var completedAt sql.NullTime
			var photoURL sql.NullString
			var compStatus sql.NullString
			var aiFeedback sql.NullString
			var siblingName sql.NullString
			var reqApp, reqPho int
			if err := rows.Scan(&sc.ScheduleID, &sc.ChoreID, &sc.Title, &sc.Description, &sc.Category, &sc.Icon,
				&sc.PointsValue, &sc.MissedPenaltyValue, &sc.EstimatedMinutes, &reqApp, &reqPho, &sc.PhotoSource, &sc.AssignmentType, &sc.AvailableAt, &sc.DueBy,
				&sc.ExpiryPenalty, &sc.ExpiryPenaltyValue,
				&dayOfWeek, &specificDate,
				&compID, &completedAt, &photoURL, &compStatus, &aiFeedback,
				&sc.TTSDescription, &sc.TTSAudioURL, &siblingName); err != nil {
				rows.Close()
				return nil, err
			}
			sc.RequiresApproval = reqApp == 1
			sc.RequiresPhoto = reqPho == 1
			sc.Date = dateStr
			// ai_rejected completions are not "completed" from the kid's perspective
			sc.Completed = compID.Valid && compStatus.String != model.StatusAIRejected
			if compID.Valid {
				id := compID.Int64
				sc.CompletionID = &id
			}
			if completedAt.Valid {
				t := completedAt.Time
				sc.CompletedAt = &t
			}
			if photoURL.Valid && photoURL.String != "" {
				s := photoURL.String
				sc.PhotoURL = &s
			}
			if compStatus.Valid {
				s := compStatus.String
				sc.CompletionStatus = &s
			}
			if aiFeedback.Valid && aiFeedback.String != "" {
				s := aiFeedback.String
				sc.AIFeedback = &s
			}
			if siblingName.Valid && siblingName.String != "" {
				sc.CompletedByName = siblingName.String
				sc.CompletedBySibling = true
			}
			if sc.AvailableAt != nil && *sc.AvailableAt != "" {
				sc.Available = currentTime >= *sc.AvailableAt
			} else {
				sc.Available = true
			}
			// Check if chore has expired (past due_by time and not completed)
			if sc.DueBy != nil && *sc.DueBy != "" && !sc.Completed {
				// Only check expiry for today's chores
				if dateStr == now.Format(model.DateFormat) && currentTime > *sc.DueBy {
					sc.Expired = true
				}
			}
			results = append(results, sc)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	return results, nil
}

// --- Completions ---

func (s *Store) CompleteChore(ctx context.Context, cc *model.ChoreCompletion) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO chore_completions (chore_schedule_id, completed_by, status, photo_url, completion_date, ai_feedback, ai_confidence)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		cc.ChoreScheduleID, cc.CompletedBy, cc.Status, cc.PhotoURL, cc.CompletionDate, cc.AIFeedback, cc.AIConfidence)
	if err != nil {
		return err
	}
	cc.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) UncompleteChore(ctx context.Context, scheduleID int64, completionDate string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM chore_completions WHERE chore_schedule_id = ? AND completion_date = ?`,
		scheduleID, completionDate)
	return err
}

func (s *Store) GetSchedule(ctx context.Context, id int64) (*model.ChoreSchedule, error) {
	cs := &model.ChoreSchedule{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, chore_id, assigned_to, assignment_type, fcfs_group_id, day_of_week, specific_date, available_at, points_multiplier, start_date, end_date, recurrence_interval, recurrence_start, due_by, expiry_penalty, expiry_penalty_value, created_at
		 FROM chore_schedules WHERE id = ?`, id).
		Scan(&cs.ID, &cs.ChoreID, &cs.AssignedTo, &cs.AssignmentType, &cs.FcfsGroupID, &cs.DayOfWeek, &cs.SpecificDate, &cs.AvailableAt, &cs.PointsMultiplier, &cs.StartDate, &cs.EndDate, &cs.RecurrenceInterval, &cs.RecurrenceStart, &cs.DueBy, &cs.ExpiryPenalty, &cs.ExpiryPenaltyValue, &cs.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return cs, err
}

func (s *Store) GetCompletionForScheduleDate(ctx context.Context, scheduleID int64, completionDate string) (*model.ChoreCompletion, error) {
	cc := &model.ChoreCompletion{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, chore_schedule_id, completed_by, status, photo_url, approved_by, approved_at, completed_at, completion_date, ai_feedback, ai_confidence
		 FROM chore_completions WHERE chore_schedule_id = ? AND completion_date = ?`,
		scheduleID, completionDate).
		Scan(&cc.ID, &cc.ChoreScheduleID, &cc.CompletedBy, &cc.Status, &cc.PhotoURL, &cc.ApprovedBy, &cc.ApprovedAt, &cc.CompletedAt, &cc.CompletionDate, &cc.AIFeedback, &cc.AIConfidence)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return cc, err
}

func (s *Store) GetCompletion(ctx context.Context, id int64) (*model.ChoreCompletion, error) {
	cc := &model.ChoreCompletion{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, chore_schedule_id, completed_by, status, photo_url, approved_by, approved_at, completed_at, completion_date, ai_feedback, ai_confidence
		 FROM chore_completions WHERE id = ?`, id).
		Scan(&cc.ID, &cc.ChoreScheduleID, &cc.CompletedBy, &cc.Status, &cc.PhotoURL, &cc.ApprovedBy, &cc.ApprovedAt, &cc.CompletedAt, &cc.CompletionDate, &cc.AIFeedback, &cc.AIConfidence)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return cc, err
}

type PendingCompletionRow struct {
	ID             int64     `json:"id"`
	ChoreTitle     string    `json:"chore_title"`
	ChildName      string    `json:"child_name"`
	PhotoURL       string    `json:"photo_url"`
	CompletionDate string    `json:"completion_date"`
	CompletedAt    time.Time `json:"completed_at"`
}

func (s *Store) ListPendingCompletions(ctx context.Context) ([]PendingCompletionRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cc.id, c.title, u.name, cc.photo_url, cc.completion_date, cc.completed_at
		FROM chore_completions cc
		JOIN chore_schedules cs ON cs.id = cc.chore_schedule_id
		JOIN chores c ON c.id = cs.chore_id
		JOIN users u ON u.id = cc.completed_by
		WHERE cc.status = ?
		ORDER BY cc.completed_at DESC
	`, model.StatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pending []PendingCompletionRow
	for rows.Next() {
		var p PendingCompletionRow
		if err := rows.Scan(&p.ID, &p.ChoreTitle, &p.ChildName, &p.PhotoURL, &p.CompletionDate, &p.CompletedAt); err != nil {
			return nil, err
		}
		pending = append(pending, p)
	}
	return pending, rows.Err()
}

func (s *Store) UpdateCompletionStatus(ctx context.Context, id int64, status string, adminID int64) error {
	var err error
	if status == model.StatusApproved {
		_, err = s.db.ExecContext(ctx,
			`UPDATE chore_completions SET status = ?, approved_by = ?, approved_at = CURRENT_TIMESTAMP WHERE id = ?`,
			status, adminID, id)
	} else {
		_, err = s.db.ExecContext(ctx,
			`UPDATE chore_completions SET status = ? WHERE id = ?`,
			status, id)
	}
	return err
}

func (s *Store) UpdateCompletionPhoto(ctx context.Context, id int64, photoURL string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE chore_completions SET photo_url = ? WHERE id = ?`,
		photoURL, id)
	return err
}

// --- Settings ---

func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO app_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?`,
		key, value, value)
	return err
}

func (s *Store) ListSettings(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM app_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		settings[k] = v
	}
	return settings, rows.Err()
}

// --- Users (update) ---

func (s *Store) UpdateUser(ctx context.Context, u *model.User) error {
	paused := boolToInt(u.Paused)
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET name=?, avatar_url=?, role=?, age=?, theme=?, line_color=?, paused=? WHERE id=?`,
		u.Name, u.AvatarURL, u.Role, u.Age, u.Theme, u.LineColor, paused, u.ID)
	return err
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	return err
}

// --- Points ---

func (s *Store) GetChorePointsForSchedule(ctx context.Context, scheduleID int64) (int, error) {
	var pts int
	err := s.db.QueryRowContext(ctx,
		`SELECT CAST(c.points_value * cs.points_multiplier AS INTEGER)
		 FROM chore_schedules cs JOIN chores c ON c.id = cs.chore_id
		 WHERE cs.id = ?`, scheduleID).Scan(&pts)
	return pts, err
}

func (s *Store) CreditChorePoints(ctx context.Context, userID, completionID int64, amount int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO point_transactions (user_id, amount, reason, reference_id, note)
		 VALUES (?, ?, ?, ?, '')`,
		userID, amount, model.ReasonChoreComplete, completionID)
	return err
}

func (s *Store) DebitChorePoints(ctx context.Context, userID, completionID int64, amount int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO point_transactions (user_id, amount, reason, reference_id, note)
		 VALUES (?, ?, ?, ?, '')`,
		userID, -amount, model.ReasonChoreUncomplete, completionID)
	return err
}

// GetNetPointsForCompletion returns the net points credited/debited for a specific completion.
// Positive means points were earned, negative means a penalty was applied.
func (s *Store) GetNetPointsForCompletion(ctx context.Context, completionID int64) (int, error) {
	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM point_transactions
		 WHERE reference_id = ? AND reason IN (?, ?)`,
		completionID, model.ReasonChoreComplete, model.ReasonExpiryPenalty).Scan(&total)
	return total, err
}

func (s *Store) GetPointBalance(ctx context.Context, userID int64) (int, error) {
	var balance int
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM point_transactions WHERE user_id = ?`, userID).Scan(&balance)
	return balance, err
}

type PointBalanceRow struct {
	UserID  int64 `json:"user_id"`
	Balance int   `json:"balance"`
}

func (s *Store) GetAllPointBalances(ctx context.Context) ([]PointBalanceRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT u.id, COALESCE(SUM(pt.amount), 0)
		 FROM users u LEFT JOIN point_transactions pt ON pt.user_id = u.id
		 GROUP BY u.id ORDER BY u.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PointBalanceRow
	for rows.Next() {
		var r PointBalanceRow
		if err := rows.Scan(&r.UserID, &r.Balance); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) ListPointTransactions(ctx context.Context, userID int64, limit int) ([]model.PointTransaction, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, amount, reason, reference_id, note, idempotency_key, created_at
		 FROM point_transactions WHERE user_id = ? ORDER BY id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var txs []model.PointTransaction
	for rows.Next() {
		var t model.PointTransaction
		var idempotencyKey sql.NullString
		if err := rows.Scan(&t.ID, &t.UserID, &t.Amount, &t.Reason, &t.ReferenceID, &t.Note, &idempotencyKey, &t.CreatedAt); err != nil {
			return nil, err
		}
		if idempotencyKey.Valid {
			k := idempotencyKey.String
			t.IdempotencyKey = &k
		}
		txs = append(txs, t)
	}
	return txs, rows.Err()
}

func (s *Store) AdminAdjustPoints(ctx context.Context, userID int64, amount int, note string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO point_transactions (user_id, amount, reason, note)
		 VALUES (?, ?, ?, ?)`, userID, amount, model.ReasonAdminAdjust, note)
	return err
}

// --- Rewards ---

func (s *Store) CreateReward(ctx context.Context, r *model.Reward) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO rewards (name, description, icon, cost, stock, active, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.Name, r.Description, r.Icon, r.Cost, r.Stock, r.Active, r.CreatedBy)
	if err != nil {
		return err
	}
	r.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) GetReward(ctx context.Context, id int64) (*model.Reward, error) {
	r := &model.Reward{}
	var active int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, icon, cost, stock, active, created_by, created_at
		 FROM rewards WHERE id = ?`, id).
		Scan(&r.ID, &r.Name, &r.Description, &r.Icon, &r.Cost, &r.Stock, &active, &r.CreatedBy, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	r.Active = active == 1
	return r, err
}

func (s *Store) ListRewards(ctx context.Context, activeOnly bool) ([]model.Reward, error) {
	q := `SELECT id, name, description, icon, cost, stock, active, created_by, created_at FROM rewards`
	if activeOnly {
		q += ` WHERE active = 1`
	}
	q += ` ORDER BY cost`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rewards []model.Reward
	for rows.Next() {
		var r model.Reward
		var active int
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.Icon, &r.Cost, &r.Stock, &active, &r.CreatedBy, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Active = active == 1
		r.EffectiveCost = r.Cost
		rewards = append(rewards, r)
	}
	return rewards, rows.Err()
}

// ListRewardsForUser returns active rewards available to a specific user.
// If a reward has no assignments, it's available to everyone.
// If it has assignments, only assigned users see it, with per-user cost.
func (s *Store) ListRewardsForUser(ctx context.Context, userID int64) ([]model.Reward, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.name, r.description, r.icon, r.cost, r.stock, r.created_by, r.created_at,
			ra.custom_cost
		FROM rewards r
		LEFT JOIN reward_assignments ra ON ra.reward_id = r.id AND ra.user_id = ?
		WHERE r.active = 1
		  AND (
			-- No assignments at all: available to everyone
			NOT EXISTS (SELECT 1 FROM reward_assignments WHERE reward_id = r.id)
			-- Or this user is assigned
			OR ra.id IS NOT NULL
		  )
		ORDER BY r.cost`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rewards []model.Reward
	for rows.Next() {
		var r model.Reward
		var customCost sql.NullInt64
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.Icon, &r.Cost, &r.Stock, &r.CreatedBy, &r.CreatedAt, &customCost); err != nil {
			return nil, err
		}
		r.Active = true
		if customCost.Valid {
			r.EffectiveCost = int(customCost.Int64)
		} else {
			r.EffectiveCost = r.Cost
		}
		rewards = append(rewards, r)
	}
	return rewards, rows.Err()
}

// ListRewardsWithAssignments returns all rewards with their assignments (admin view).
func (s *Store) ListRewardsWithAssignments(ctx context.Context) ([]model.Reward, error) {
	rewards, err := s.ListRewards(ctx, false)
	if err != nil {
		return nil, err
	}
	for i := range rewards {
		assignments, err := s.GetRewardAssignments(ctx, rewards[i].ID)
		if err != nil {
			return nil, err
		}
		rewards[i].Assignments = assignments
	}
	return rewards, nil
}

func (s *Store) GetRewardAssignments(ctx context.Context, rewardID int64) ([]model.RewardAssignment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, reward_id, user_id, custom_cost FROM reward_assignments WHERE reward_id = ? ORDER BY user_id`, rewardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var assignments []model.RewardAssignment
	for rows.Next() {
		var a model.RewardAssignment
		if err := rows.Scan(&a.ID, &a.RewardID, &a.UserID, &a.CustomCost); err != nil {
			return nil, err
		}
		assignments = append(assignments, a)
	}
	return assignments, rows.Err()
}

func (s *Store) SetRewardAssignments(ctx context.Context, rewardID int64, assignments []model.RewardAssignment) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Remove existing
	_, err = tx.ExecContext(ctx, `DELETE FROM reward_assignments WHERE reward_id = ?`, rewardID)
	if err != nil {
		return err
	}

	// Insert new
	for _, a := range assignments {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO reward_assignments (reward_id, user_id, custom_cost) VALUES (?, ?, ?)`,
			rewardID, a.UserID, a.CustomCost)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) UpdateReward(ctx context.Context, r *model.Reward) error {
	active := boolToInt(r.Active)
	_, err := s.db.ExecContext(ctx,
		`UPDATE rewards SET name=?, description=?, icon=?, cost=?, stock=?, active=? WHERE id=?`,
		r.Name, r.Description, r.Icon, r.Cost, r.Stock, active, r.ID)
	return err
}

func (s *Store) DeleteReward(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM rewards WHERE id = ?`, id)
	return err
}

func (s *Store) RedeemReward(ctx context.Context, userID, rewardID int64) (*model.RewardRedemption, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Get reward
	var baseCost int
	var stock sql.NullInt64
	var active int
	err = tx.QueryRowContext(ctx,
		`SELECT cost, stock, active FROM rewards WHERE id = ?`, rewardID).
		Scan(&baseCost, &stock, &active)
	if err != nil {
		return nil, fmt.Errorf("reward not found")
	}
	if active != 1 {
		return nil, fmt.Errorf("reward is not active")
	}
	if stock.Valid && stock.Int64 <= 0 {
		return nil, fmt.Errorf("reward is out of stock")
	}

	// Check if reward has assignments and if user is assigned
	var hasAssignments bool
	err = tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM reward_assignments WHERE reward_id = ?)`, rewardID).
		Scan(&hasAssignments)
	if err != nil {
		return nil, err
	}
	if hasAssignments {
		var assigned bool
		err = tx.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM reward_assignments WHERE reward_id = ? AND user_id = ?)`,
			rewardID, userID).Scan(&assigned)
		if err != nil {
			return nil, err
		}
		if !assigned {
			return nil, fmt.Errorf("reward is not available to you")
		}
	}

	// Determine effective cost (per-user override or base)
	cost := baseCost
	var customCost sql.NullInt64
	tx.QueryRowContext(ctx,
		`SELECT custom_cost FROM reward_assignments WHERE reward_id = ? AND user_id = ?`,
		rewardID, userID).Scan(&customCost)
	if customCost.Valid {
		cost = int(customCost.Int64)
	}

	// Check balance
	var balance int
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM point_transactions WHERE user_id = ?`, userID).
		Scan(&balance)
	if err != nil {
		return nil, err
	}
	if balance < cost {
		return nil, fmt.Errorf("insufficient points (have %d, need %d)", balance, cost)
	}

	// Insert redemption
	res, err := tx.ExecContext(ctx,
		`INSERT INTO reward_redemptions (reward_id, user_id, points_spent) VALUES (?, ?, ?)`,
		rewardID, userID, cost)
	if err != nil {
		return nil, err
	}
	redemptionID, _ := res.LastInsertId()

	// Debit points
	_, err = tx.ExecContext(ctx,
		`INSERT INTO point_transactions (user_id, amount, reason, reference_id, note)
		 VALUES (?, ?, ?, ?, '')`,
		userID, -cost, model.ReasonRewardRedeem, redemptionID)
	if err != nil {
		return nil, err
	}

	// Decrement stock if limited
	if stock.Valid {
		_, err = tx.ExecContext(ctx, `UPDATE rewards SET stock = stock - 1 WHERE id = ?`, rewardID)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &model.RewardRedemption{
		ID:          redemptionID,
		RewardID:    rewardID,
		UserID:      userID,
		PointsSpent: cost,
	}, nil
}

// --- Streaks ---

func (s *Store) GetUserStreak(ctx context.Context, userID int64) (*model.UserStreak, error) {
	st := &model.UserStreak{UserID: userID}
	err := s.db.QueryRowContext(ctx,
		`SELECT current_streak, longest_streak, streak_start_date, last_completed_date
		 FROM user_streaks WHERE user_id = ?`, userID).
		Scan(&st.CurrentStreak, &st.LongestStreak, &st.StreakStartDate, &st.LastCompletedDate)
	if err == sql.ErrNoRows {
		return st, nil // zero values
	}
	return st, err
}

func (s *Store) RecalculateStreak(ctx context.Context, userID int64, today string) error {
	now, err := time.Parse(model.DateFormat, today)
	if err != nil {
		return err
	}

	streak := 0
	// Walk backwards from yesterday (today may be incomplete)
	for i := 1; i <= 365; i++ {
		d := now.AddDate(0, 0, -i)
		dateStr := d.Format(model.DateFormat)
		chores, err := s.GetScheduledChoresForUser(ctx, userID, []string{dateStr}, d)
		if err != nil {
			return err
		}
		// Filter to required + core only
		var nonBonus []model.ScheduledChore
		for _, c := range chores {
			if c.Category != model.CategoryBonus {
				nonBonus = append(nonBonus, c)
			}
		}
		if len(nonBonus) == 0 {
			continue // free day, don't break or count
		}
		allDone := true
		for _, c := range nonBonus {
			if !c.Completed {
				allDone = false
				break
			}
		}
		if !allDone {
			break
		}
		streak++
	}

	// Check if today is also fully complete (adds to streak)
	todayChores, err := s.GetScheduledChoresForUser(ctx, userID, []string{today}, now)
	if err != nil {
		return err
	}
	var todayNonBonus []model.ScheduledChore
	for _, c := range todayChores {
		if c.Category != model.CategoryBonus {
			todayNonBonus = append(todayNonBonus, c)
		}
	}
	todayComplete := len(todayNonBonus) > 0
	for _, c := range todayNonBonus {
		if !c.Completed {
			todayComplete = false
			break
		}
	}
	if todayComplete {
		streak++
	}

	var startDate *string
	if streak > 0 {
		d := now.AddDate(0, 0, -(streak - 1))
		s := d.Format(model.DateFormat)
		startDate = &s
	}

	var lastCompleted *string
	if todayComplete {
		lastCompleted = &today
	} else if streak > 0 {
		yesterday := now.AddDate(0, 0, -1).Format(model.DateFormat)
		lastCompleted = &yesterday
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO user_streaks (user_id, current_streak, longest_streak, streak_start_date, last_completed_date, updated_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(user_id) DO UPDATE SET
			current_streak = ?,
			longest_streak = MAX(user_streaks.longest_streak, ?),
			streak_start_date = ?,
			last_completed_date = ?,
			updated_at = CURRENT_TIMESTAMP`,
		userID, streak, streak, startDate, lastCompleted,
		streak, streak, startDate, lastCompleted)
	return err
}

func (s *Store) ListStreakRewards(ctx context.Context) ([]model.StreakReward, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, streak_days, bonus_points, label FROM streak_rewards ORDER BY streak_days`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rewards []model.StreakReward
	for rows.Next() {
		var r model.StreakReward
		if err := rows.Scan(&r.ID, &r.StreakDays, &r.BonusPoints, &r.Label); err != nil {
			return nil, err
		}
		rewards = append(rewards, r)
	}
	return rewards, rows.Err()
}

func (s *Store) CreateStreakReward(ctx context.Context, r *model.StreakReward) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO streak_rewards (streak_days, bonus_points, label) VALUES (?, ?, ?)`,
		r.StreakDays, r.BonusPoints, r.Label)
	if err != nil {
		return err
	}
	r.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) DeleteStreakReward(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM streak_rewards WHERE id = ?`, id)
	return err
}

// --- Reward Redemption History ---

type RedemptionHistoryRow struct {
	ID          int64     `json:"id"`
	RewardName  string    `json:"reward_name"`
	RewardIcon  string    `json:"reward_icon"`
	PointsSpent int       `json:"points_spent"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *Store) ListRedemptionsForUser(ctx context.Context, userID int64, limit int) ([]RedemptionHistoryRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT rr.id, r.name, r.icon, rr.points_spent, rr.created_at
		FROM reward_redemptions rr
		JOIN rewards r ON r.id = rr.reward_id
		WHERE rr.user_id = ?
		ORDER BY rr.created_at DESC
		LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []RedemptionHistoryRow
	for rows.Next() {
		var r RedemptionHistoryRow
		if err := rows.Scan(&r.ID, &r.RewardName, &r.RewardIcon, &r.PointsSpent, &r.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) UndoRedemption(ctx context.Context, redemptionID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the redemption details
	var userID, rewardID int64
	var pointsSpent int
	err = tx.QueryRowContext(ctx,
		`SELECT user_id, reward_id, points_spent FROM reward_redemptions WHERE id = ?`,
		redemptionID).Scan(&userID, &rewardID, &pointsSpent)
	if err != nil {
		return fmt.Errorf("redemption not found")
	}

	// Delete the point transaction that debited points
	_, err = tx.ExecContext(ctx,
		`DELETE FROM point_transactions WHERE reason = ? AND reference_id = ?`,
		model.ReasonRewardRedeem,
		redemptionID)
	if err != nil {
		return err
	}

	// Restore stock if the reward has limited stock
	var stock sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT stock FROM rewards WHERE id = ?`, rewardID).Scan(&stock)
	if err == nil && stock.Valid {
		_, err = tx.ExecContext(ctx, `UPDATE rewards SET stock = stock + 1 WHERE id = ?`, rewardID)
		if err != nil {
			return err
		}
	}

	// Delete the redemption record
	_, err = tx.ExecContext(ctx, `DELETE FROM reward_redemptions WHERE id = ?`, redemptionID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// --- Decay Config ---

func (s *Store) GetUserDecayConfig(ctx context.Context, userID int64) (*model.UserDecayConfig, error) {
	cfg := &model.UserDecayConfig{UserID: userID}
	var enabled int
	var lastDecay sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT enabled, decay_rate, decay_interval_hours, last_decay_at FROM user_decay_config WHERE user_id = ?`,
		userID).Scan(&enabled, &cfg.DecayRate, &cfg.DecayIntervalHours, &lastDecay)
	if err == sql.ErrNoRows {
		// Return defaults
		cfg.DecayRate = 5
		cfg.DecayIntervalHours = 24
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	cfg.Enabled = enabled == 1
	if lastDecay.Valid {
		cfg.LastDecayAt = &lastDecay.Time
	}
	return cfg, nil
}

func (s *Store) SetUserDecayConfig(ctx context.Context, cfg *model.UserDecayConfig) error {
	enabled := boolToInt(cfg.Enabled)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO user_decay_config (user_id, enabled, decay_rate, decay_interval_hours)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET enabled = ?, decay_rate = ?, decay_interval_hours = ?`,
		cfg.UserID, enabled, cfg.DecayRate, cfg.DecayIntervalHours,
		enabled, cfg.DecayRate, cfg.DecayIntervalHours)
	return err
}

func (s *Store) ListDecayConfigsEnabled(ctx context.Context) ([]model.UserDecayConfig, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id, decay_rate, decay_interval_hours, last_decay_at FROM user_decay_config WHERE enabled = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var configs []model.UserDecayConfig
	for rows.Next() {
		cfg := model.UserDecayConfig{Enabled: true}
		var lastDecay sql.NullTime
		if err := rows.Scan(&cfg.UserID, &cfg.DecayRate, &cfg.DecayIntervalHours, &lastDecay); err != nil {
			return nil, err
		}
		if lastDecay.Valid {
			cfg.LastDecayAt = &lastDecay.Time
		}
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

func (s *Store) UpdateLastDecayAt(ctx context.Context, userID int64, t time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE user_decay_config SET last_decay_at = ? WHERE user_id = ?`, t, userID)
	return err
}

func (s *Store) DebitExpiryPenalty(ctx context.Context, userID, completionID int64, amount int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO point_transactions (user_id, amount, reason, reference_id, note)
		 VALUES (?, ?, ?, ?, 'Late completion penalty')`,
		userID, -amount, model.ReasonExpiryPenalty, completionID)
	return err
}

func (s *Store) DebitDecay(ctx context.Context, userID int64, amount int, date string) error {
	key := fmt.Sprintf("points_decay:%d:%s", userID, date)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO point_transactions (user_id, amount, reason, note, idempotency_key)
		 VALUES (?, ?, ?, 'Daily points decay', ?)`,
		userID, -amount, model.ReasonPointsDecay, key)
	return err
}

// missedChorePenaltyKey returns the structured idempotency key used to
// guarantee at-most-once application of a missed-chore penalty for a
// given (user, schedule, date) tuple.
func missedChorePenaltyKey(userID, scheduleID int64, date string) string {
	return fmt.Sprintf("missed_chore_penalty:%d:%d:%s", userID, scheduleID, date)
}

func (s *Store) DebitMissedChore(ctx context.Context, userID int64, scheduleID int64, amount int, date string) error {
	key := missedChorePenaltyKey(userID, scheduleID, date)
	// The partial UNIQUE index on idempotency_key ensures that concurrent
	// or retried invocations for the same (user, schedule, date) cannot
	// insert duplicate penalty rows. Treat a UNIQUE violation as a
	// successful no-op so callers don't have to distinguish the race.
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO point_transactions (user_id, amount, reason, reference_id, note, idempotency_key)
		 VALUES (?, ?, 'missed_chore', ?, ?, ?)`,
		userID, -amount, scheduleID, "Penalty for missed chore on "+date, key)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return nil
		}
		return err
	}
	return nil
}

func (s *Store) HasMissedChorePenalty(ctx context.Context, scheduleID int64, date string) (bool, error) {
	// Look up the penalty by exact idempotency-key match. The schedule's
	// assigned user identifies the user portion of the key; resolving it
	// here keeps the public API stable while allowing the key format to
	// carry richer information.
	var userID int64
	err := s.db.QueryRowContext(ctx,
		`SELECT assigned_to FROM chore_schedules WHERE id = ?`, scheduleID).Scan(&userID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	key := missedChorePenaltyKey(userID, scheduleID, date)
	var exists bool
	err = s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM point_transactions WHERE idempotency_key = ?)`,
		key).Scan(&exists)
	return exists, err
}

// isUniqueConstraintErr reports whether err is a SQLite UNIQUE-constraint
// violation. modernc.org/sqlite surfaces these through error messages that
// contain "UNIQUE constraint failed".
func isUniqueConstraintErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// --- FCFS Helpers ---

// ListNonPausedChildren returns all child users that are not paused.
func (s *Store) ListNonPausedChildren(ctx context.Context) ([]model.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, avatar_url, role, age, theme, line_color, paused, created_at FROM users WHERE role = 'child' AND paused = 0 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []model.User
	for rows.Next() {
		var u model.User
		var paused int
		if err := rows.Scan(&u.ID, &u.Name, &u.AvatarURL, &u.Role, &u.Age, &u.Theme, &u.LineColor, &paused, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.Paused = paused == 1
		users = append(users, u)
	}
	return users, rows.Err()
}

// FcfsGroupCompletedForDate checks whether any schedule in an FCFS group has been completed for a given date.
func (s *Store) FcfsGroupCompletedForDate(ctx context.Context, groupID, date string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM chore_completions cc
		   JOIN chore_schedules cs ON cs.id = cc.chore_schedule_id
		   WHERE cs.fcfs_group_id = ? AND cc.completion_date = ?
		   AND cc.status NOT IN ('ai_rejected'))`,
		groupID, date).Scan(&exists)
	return exists, err
}

// CompleteFCFSSiblings creates zero-point shadow completions for all sibling schedules in an FCFS group.
func (s *Store) CompleteFCFSSiblings(ctx context.Context, groupID string, completedByUserID, excludeScheduleID int64, date string) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM chore_schedules WHERE fcfs_group_id = ? AND id != ?`,
		groupID, excludeScheduleID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var siblingIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		siblingIDs = append(siblingIDs, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, sid := range siblingIDs {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO chore_completions (chore_schedule_id, completed_by, status, completion_date) VALUES (?, ?, 'approved', ?)`,
			sid, completedByUserID, date)
		if err != nil {
			return err
		}
	}
	return nil
}

// UncompleteByFCFSGroup deletes all completions for an FCFS group on a given date.
func (s *Store) UncompleteByFCFSGroup(ctx context.Context, groupID, date string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM chore_completions WHERE chore_schedule_id IN (
		   SELECT id FROM chore_schedules WHERE fcfs_group_id = ?
		 ) AND completion_date = ?`,
		groupID, date)
	return err
}

// --- Chore Triggers ---

func (s *Store) CreateChoreTrigger(ctx context.Context, t *model.ChoreTrigger) error {
	enabled := boolToInt(t.Enabled)
	if t.AssignmentType == "" {
		t.AssignmentType = model.AssignmentIndividual
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO chore_triggers (uuid, chore_id, default_assigned_to, default_due_by, default_available_at, enabled, cooldown_minutes, assignment_type)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.UUID, t.ChoreID, t.DefaultAssignedTo, t.DefaultDueBy, t.DefaultAvailableAt, enabled, t.CooldownMinutes, t.AssignmentType)
	if err != nil {
		return err
	}
	t.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) GetChoreTriggerByUUID(ctx context.Context, uuid string) (*model.ChoreTrigger, error) {
	t := &model.ChoreTrigger{}
	var enabled int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, uuid, chore_id, default_assigned_to, default_due_by, default_available_at, enabled, cooldown_minutes, assignment_type, last_triggered_at, created_at
		 FROM chore_triggers WHERE uuid = ?`, uuid).
		Scan(&t.ID, &t.UUID, &t.ChoreID, &t.DefaultAssignedTo, &t.DefaultDueBy, &t.DefaultAvailableAt, &enabled, &t.CooldownMinutes, &t.AssignmentType, &t.LastTriggeredAt, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	t.Enabled = enabled == 1
	return t, err
}

func (s *Store) ListChoreTriggersForChore(ctx context.Context, choreID int64) ([]model.ChoreTrigger, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, uuid, chore_id, default_assigned_to, default_due_by, default_available_at, enabled, cooldown_minutes, assignment_type, last_triggered_at, created_at
		 FROM chore_triggers WHERE chore_id = ? ORDER BY id`, choreID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var triggers []model.ChoreTrigger
	for rows.Next() {
		var t model.ChoreTrigger
		var enabled int
		if err := rows.Scan(&t.ID, &t.UUID, &t.ChoreID, &t.DefaultAssignedTo, &t.DefaultDueBy, &t.DefaultAvailableAt, &enabled, &t.CooldownMinutes, &t.AssignmentType, &t.LastTriggeredAt, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.Enabled = enabled == 1
		triggers = append(triggers, t)
	}
	return triggers, rows.Err()
}

func (s *Store) UpdateChoreTrigger(ctx context.Context, t *model.ChoreTrigger) error {
	enabled := boolToInt(t.Enabled)
	if t.AssignmentType == "" {
		t.AssignmentType = model.AssignmentIndividual
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE chore_triggers SET default_assigned_to = ?, default_due_by = ?, default_available_at = ?, enabled = ?, cooldown_minutes = ?, assignment_type = ? WHERE id = ?`,
		t.DefaultAssignedTo, t.DefaultDueBy, t.DefaultAvailableAt, enabled, t.CooldownMinutes, t.AssignmentType, t.ID)
	return err
}

func (s *Store) DeleteChoreTrigger(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM chore_triggers WHERE id = ?`, id)
	return err
}

func (s *Store) UpdateTriggerLastFired(ctx context.Context, id int64, t time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE chore_triggers SET last_triggered_at = ? WHERE id = ?`, t.UTC().Format("2006-01-02 15:04:05"), id)
	return err
}

func (s *Store) GetUserByName(ctx context.Context, name string) (*model.User, error) {
	u := &model.User{}
	var paused int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, avatar_url, role, age, theme, line_color, paused, pin_hash, created_at FROM users WHERE LOWER(name) = LOWER(?)`, name).
		Scan(&u.ID, &u.Name, &u.AvatarURL, &u.Role, &u.Age, &u.Theme, &u.LineColor, &paused, &u.PinHash, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	u.Paused = paused == 1
	u.HasPin = u.PinHash != ""
	return u, err
}

// SetUserPin stores a bcrypt-hashed PIN for the user. Pass an empty string to clear the PIN.
func (s *Store) SetUserPin(ctx context.Context, userID int64, pinHash string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET pin_hash = ? WHERE id = ?`, pinHash, userID)
	return err
}

// GetUserPinHash returns the stored bcrypt hash for a user's PIN, or empty string if none is set.
func (s *Store) GetUserPinHash(ctx context.Context, userID int64) (string, error) {
	var hash string
	err := s.db.QueryRowContext(ctx,
		`SELECT pin_hash FROM users WHERE id = ?`, userID).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return hash, err
}

// --- API Tokens ---

func (s *Store) CreateAPIToken(ctx context.Context, t *model.APIToken) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO api_tokens (name, token_hash) VALUES (?, ?)`,
		t.Name, t.TokenHash)
	if err != nil {
		return err
	}
	t.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) ValidateAPIToken(ctx context.Context, tokenHash string) (*model.APIToken, error) {
	t := &model.APIToken{}
	var revoked int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, token_hash, last_used_at, revoked, created_at
		 FROM api_tokens WHERE token_hash = ? AND revoked = 0`, tokenHash).
		Scan(&t.ID, &t.Name, &t.TokenHash, &t.LastUsedAt, &revoked, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	t.Revoked = revoked == 1
	return t, err
}

func (s *Store) UpdateTokenLastUsed(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE api_tokens SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func (s *Store) ListAPITokens(ctx context.Context) ([]model.APIToken, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, last_used_at, revoked, created_at FROM api_tokens ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tokens []model.APIToken
	for rows.Next() {
		var t model.APIToken
		var revoked int
		if err := rows.Scan(&t.ID, &t.Name, &t.LastUsedAt, &revoked, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.Revoked = revoked == 1
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (s *Store) RevokeAPIToken(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE api_tokens SET revoked = 1 WHERE id = ?`, id)
	return err
}

// ListTriggersWithChores returns all chores that have at least one enabled trigger.
func (s *Store) ListTriggersWithChores(ctx context.Context) ([]model.TriggerableChore, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.title, c.description, c.category, c.icon, c.points_value,
		       ct.id, ct.uuid, ct.default_assigned_to, ct.default_due_by, ct.default_available_at,
		       ct.enabled, ct.cooldown_minutes,
		       COALESCE(u.name, '') AS default_assigned_name
		FROM chore_triggers ct
		JOIN chores c ON c.id = ct.chore_id
		LEFT JOIN users u ON u.id = ct.default_assigned_to
		WHERE ct.enabled = 1
		ORDER BY c.title, ct.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	choreMap := make(map[int64]*model.TriggerableChore)
	var order []int64
	for rows.Next() {
		var choreID int64
		var tc model.TriggerableChoreInfo
		var tr model.TriggerInfo
		if err := rows.Scan(
			&choreID, &tc.Title, &tc.Description, &tc.Category, &tc.Icon, &tc.PointsValue,
			&tr.ID, &tr.UUID, &tr.DefaultAssignedTo, &tr.DefaultDueBy, &tr.DefaultAvailableAt,
			&tr.Enabled, &tr.CooldownMinutes,
			&tr.DefaultAssignedName,
		); err != nil {
			return nil, err
		}
		if existing, ok := choreMap[choreID]; ok {
			existing.Triggers = append(existing.Triggers, tr)
		} else {
			tc.ID = choreID
			entry := &model.TriggerableChore{
				TriggerableChoreInfo: tc,
				Triggers:             []model.TriggerInfo{tr},
			}
			choreMap[choreID] = entry
			order = append(order, choreID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]model.TriggerableChore, 0, len(order))
	for _, id := range order {
		result = append(result, *choreMap[id])
	}
	return result, nil
}

// --- Webhooks ---

func (s *Store) CreateWebhook(ctx context.Context, w *model.Webhook) error {
	active := boolToInt(w.Active)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO webhooks (url, secret, events, active) VALUES (?, ?, ?, ?)`,
		w.URL, w.Secret, w.Events, active)
	if err != nil {
		return err
	}
	w.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) ListWebhooks(ctx context.Context) ([]model.Webhook, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, url, secret, events, active, created_at FROM webhooks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var webhooks []model.Webhook
	for rows.Next() {
		var w model.Webhook
		var active int
		if err := rows.Scan(&w.ID, &w.URL, &w.Secret, &w.Events, &active, &w.CreatedAt); err != nil {
			return nil, err
		}
		w.Active = active == 1
		webhooks = append(webhooks, w)
	}
	return webhooks, rows.Err()
}

func (s *Store) GetActiveWebhooksForEvent(ctx context.Context, event string) ([]model.Webhook, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, url, secret, events, active, created_at FROM webhooks WHERE active = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var webhooks []model.Webhook
	for rows.Next() {
		var w model.Webhook
		var active int
		if err := rows.Scan(&w.ID, &w.URL, &w.Secret, &w.Events, &active, &w.CreatedAt); err != nil {
			return nil, err
		}
		w.Active = active == 1
		// Filter by event: "*" matches all, otherwise comma-separated list
		if w.Events == "*" || containsEvent(w.Events, event) {
			webhooks = append(webhooks, w)
		}
	}
	return webhooks, rows.Err()
}

func containsEvent(events, event string) bool {
	for _, e := range splitEvents(events) {
		if e == event {
			return true
		}
	}
	return false
}

func splitEvents(events string) []string {
	var result []string
	for _, s := range strings.Split(events, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

func (s *Store) UpdateWebhook(ctx context.Context, w *model.Webhook) error {
	active := boolToInt(w.Active)
	_, err := s.db.ExecContext(ctx,
		`UPDATE webhooks SET url=?, secret=?, events=?, active=? WHERE id=?`,
		w.URL, w.Secret, w.Events, active, w.ID)
	return err
}

func (s *Store) DeleteWebhook(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id = ?`, id)
	return err
}

func (s *Store) LogWebhookDelivery(ctx context.Context, d *model.WebhookDelivery) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO webhook_deliveries (webhook_id, event, payload, status_code, response_body, error)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		d.WebhookID, d.Event, d.Payload, d.StatusCode, d.ResponseBody, d.Error)
	return err
}

func (s *Store) ListWebhookDeliveries(ctx context.Context, webhookID int64, limit int) ([]model.WebhookDelivery, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, webhook_id, event, payload, status_code, response_body, error, created_at
		 FROM webhook_deliveries WHERE webhook_id = ? ORDER BY id DESC LIMIT ?`, webhookID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deliveries []model.WebhookDelivery
	for rows.Next() {
		var d model.WebhookDelivery
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.Event, &d.Payload, &d.StatusCode, &d.ResponseBody, &d.Error, &d.CreatedAt); err != nil {
			return nil, err
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}

// DeleteOldWebhookDeliveries removes webhook_deliveries rows with created_at strictly
// older than the given cutoff. Returns the number of rows deleted. The table has an
// index on created_at (see migration 001), so this is cheap.
func (s *Store) DeleteOldWebhookDeliveries(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM webhook_deliveries WHERE created_at < ?`,
		before.UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// GetExpiredChores returns chores that are past their due_by time and not completed for today.
func (s *Store) GetExpiredChores(ctx context.Context, date string, currentTime string) ([]struct {
	ScheduleID int64
	ChoreTitle string
	UserID     int64
	UserName   string
	DueBy      string
}, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cs.id, c.title, cs.assigned_to, u.name, cs.due_by
		FROM chore_schedules cs
		JOIN chores c ON c.id = cs.chore_id
		JOIN users u ON u.id = cs.assigned_to
		LEFT JOIN chore_completions cc ON cc.id = (
				SELECT cc3.id FROM chore_completions cc3
				WHERE cc3.chore_schedule_id = cs.id AND cc3.completion_date = ?
				  AND cc3.status != 'ai_rejected'
				LIMIT 1
			)
		WHERE u.paused = 0
		  AND cs.due_by IS NOT NULL
		  AND cs.due_by != ''
		  AND cs.due_by <= ?
		  AND cc.id IS NULL
		  AND (
			(cs.day_of_week = ? AND cs.specific_date IS NULL AND cs.recurrence_interval IS NULL)
			OR cs.specific_date = ?
			OR (cs.recurrence_interval IS NOT NULL AND cs.recurrence_start IS NOT NULL
				AND CAST((julianday(?) - julianday(cs.recurrence_start)) AS INTEGER) >= 0
				AND CAST((julianday(?) - julianday(cs.recurrence_start)) AS INTEGER) % cs.recurrence_interval = 0)
		  )
		  AND (cs.start_date IS NULL OR cs.start_date <= ?)
		  AND (cs.end_date IS NULL OR cs.end_date >= ?)`,
		date, currentTime,
		func() int { t, _ := time.Parse(model.DateFormat, date); return int(t.Weekday()) }(),
		date, date, date, date, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []struct {
		ScheduleID int64
		ChoreTitle string
		UserID     int64
		UserName   string
		DueBy      string
	}
	for rows.Next() {
		var r struct {
			ScheduleID int64
			ChoreTitle string
			UserID     int64
			UserName   string
			DueBy      string
		}
		if err := rows.Scan(&r.ScheduleID, &r.ChoreTitle, &r.UserID, &r.UserName, &r.DueBy); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// --- Reports ---

type KidSummaryRow struct {
	UserID         int64
	Name           string
	AvatarURL      string
	TotalAssigned  int
	TotalCompleted int
	PointsEarned   int
	CurrentStreak  int
}

// ReportKidSummaries returns per-kid completion and points stats for the date range.
func (s *Store) ReportKidSummaries(ctx context.Context, startDate, endDate string) ([]KidSummaryRow, error) {
	query := `
		SELECT
			u.id,
			u.name,
			u.avatar_url,
			COALESCE(completed.cnt, 0) AS total_completed,
			COALESCE(missed.cnt, 0) AS total_missed,
			COALESCE(earned.total, 0) AS points_earned,
			COALESCE(us.current_streak, 0) AS current_streak
		FROM users u
		LEFT JOIN (
			SELECT cc.completed_by, COUNT(*) AS cnt
			FROM chore_completions cc
			WHERE cc.completion_date >= ? AND cc.completion_date <= ?
			AND cc.status = 'approved'
			GROUP BY cc.completed_by
		) completed ON completed.completed_by = u.id
		LEFT JOIN (
			SELECT cc.completed_by, COUNT(*) AS cnt
			FROM chore_completions cc
			WHERE cc.completion_date >= ? AND cc.completion_date <= ?
			AND cc.status = 'rejected'
			GROUP BY cc.completed_by
		) missed ON missed.completed_by = u.id
		LEFT JOIN (
			SELECT pt.user_id, SUM(pt.amount) AS total
			FROM point_transactions pt
			WHERE pt.amount > 0
			AND pt.reason IN ('chore_complete', 'streak_bonus')
			AND DATE(pt.created_at) >= ? AND DATE(pt.created_at) <= ?
			GROUP BY pt.user_id
		) earned ON earned.user_id = u.id
		LEFT JOIN user_streaks us ON us.user_id = u.id
		WHERE u.role = 'child'
		ORDER BY u.name`

	rows, err := s.db.QueryContext(ctx, query, startDate, endDate, startDate, endDate, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []KidSummaryRow
	for rows.Next() {
		var r KidSummaryRow
		var totalMissed int
		if err := rows.Scan(&r.UserID, &r.Name, &r.AvatarURL, &r.TotalCompleted, &totalMissed, &r.PointsEarned, &r.CurrentStreak); err != nil {
			return nil, err
		}
		r.TotalAssigned = r.TotalCompleted + totalMissed
		results = append(results, r)
	}
	return results, rows.Err()
}

type MissedChoreRow struct {
	ChoreID   int64
	ChoreName string
	MissCount int
	Kids      string // comma-separated
}

// ReportMostMissed returns chores with the most misses in the date range.
func (s *Store) ReportMostMissed(ctx context.Context, startDate, endDate string) ([]MissedChoreRow, error) {
	query := `
		SELECT
			c.id,
			c.title,
			COUNT(*) AS miss_count,
			GROUP_CONCAT(DISTINCT u.name) AS kids
		FROM chore_completions cc
		JOIN chore_schedules cs ON cs.id = cc.chore_schedule_id
		JOIN chores c ON c.id = cs.chore_id
		JOIN users u ON u.id = cc.completed_by
		WHERE cc.status = 'rejected'
		AND cc.completion_date >= ? AND cc.completion_date <= ?
		GROUP BY c.id, c.title
		ORDER BY miss_count DESC
		LIMIT 10`

	rows, err := s.db.QueryContext(ctx, query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []MissedChoreRow
	for rows.Next() {
		var r MissedChoreRow
		if err := rows.Scan(&r.ChoreID, &r.ChoreName, &r.MissCount, &r.Kids); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

type TrendDayRow struct {
	Date      string
	Completed int
	Assigned  int
}

// ReportCompletionTrend returns daily completion counts for the date range.
func (s *Store) ReportCompletionTrend(ctx context.Context, startDate, endDate string) ([]TrendDayRow, error) {
	query := `
		SELECT
			cc.completion_date,
			SUM(CASE WHEN cc.status = 'approved' THEN 1 ELSE 0 END) AS completed,
			COUNT(*) AS assigned
		FROM chore_completions cc
		WHERE cc.completion_date >= ? AND cc.completion_date <= ?
		GROUP BY cc.completion_date
		ORDER BY cc.completion_date`

	rows, err := s.db.QueryContext(ctx, query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TrendDayRow
	for rows.Next() {
		var r TrendDayRow
		if err := rows.Scan(&r.Date, &r.Completed, &r.Assigned); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

type CategoryStatRow struct {
	Category       string
	TotalAssigned  int
	TotalCompleted int
}

// ReportCategoryBreakdown returns completion stats grouped by chore category.
func (s *Store) ReportCategoryBreakdown(ctx context.Context, startDate, endDate string) ([]CategoryStatRow, error) {
	query := `
		SELECT
			c.category,
			COUNT(*) AS total_assigned,
			SUM(CASE WHEN cc.status = 'approved' THEN 1 ELSE 0 END) AS total_completed
		FROM chore_completions cc
		JOIN chore_schedules cs ON cs.id = cc.chore_schedule_id
		JOIN chores c ON c.id = cs.chore_id
		WHERE cc.completion_date >= ? AND cc.completion_date <= ?
		GROUP BY c.category
		ORDER BY c.category`

	rows, err := s.db.QueryContext(ctx, query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CategoryStatRow
	for rows.Next() {
		var r CategoryStatRow
		if err := rows.Scan(&r.Category, &r.TotalAssigned, &r.TotalCompleted); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

type PointsSummaryRow struct {
	UserID        int64
	Name          string
	PointsEarned  int
	PointsDecayed int
	PointsSpent   int
}

// ReportPointsSummary returns earned/decayed/spent points per kid.
func (s *Store) ReportPointsSummary(ctx context.Context, startDate, endDate string) ([]PointsSummaryRow, error) {
	query := `
		SELECT
			u.id,
			u.name,
			COALESCE(SUM(CASE WHEN pt.amount > 0 THEN pt.amount ELSE 0 END), 0) AS earned,
			COALESCE(SUM(CASE WHEN pt.reason = 'points_decay' THEN ABS(pt.amount) ELSE 0 END), 0) AS decayed,
			COALESCE(SUM(CASE WHEN pt.reason = 'reward_redeem' THEN ABS(pt.amount) ELSE 0 END), 0) AS spent
		FROM users u
		LEFT JOIN point_transactions pt
			ON pt.user_id = u.id
			AND DATE(pt.created_at) >= ? AND DATE(pt.created_at) <= ?
		WHERE u.role = 'child'
		GROUP BY u.id, u.name
		ORDER BY u.name`

	rows, err := s.db.QueryContext(ctx, query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []PointsSummaryRow
	for rows.Next() {
		var r PointsSummaryRow
		if err := rows.Scan(&r.UserID, &r.Name, &r.PointsEarned, &r.PointsDecayed, &r.PointsSpent); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

type DayOfWeekStatRow struct {
	DayOfWeek      int
	TotalAssigned  int
	TotalCompleted int
}

// ReportDayOfWeek returns completion stats grouped by day of the week.
func (s *Store) ReportDayOfWeek(ctx context.Context, startDate, endDate string) ([]DayOfWeekStatRow, error) {
	// SQLite: strftime('%w', date) returns 0=Sunday, 1=Monday, ... 6=Saturday
	query := `
		SELECT
			CAST(strftime('%w', cc.completion_date) AS INTEGER) AS dow,
			COUNT(*) AS total_assigned,
			SUM(CASE WHEN cc.status = 'approved' THEN 1 ELSE 0 END) AS total_completed
		FROM chore_completions cc
		WHERE cc.completion_date >= ? AND cc.completion_date <= ?
		GROUP BY dow
		ORDER BY dow`

	rows, err := s.db.QueryContext(ctx, query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DayOfWeekStatRow
	for rows.Next() {
		var r DayOfWeekStatRow
		if err := rows.Scan(&r.DayOfWeek, &r.TotalAssigned, &r.TotalCompleted); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

package store

import (
	"context"
	"database/sql"

	"github.com/liftedkilt/openchore/internal/model"
)

// Chores a person added for themselves (kid self-service). See
// internal/api/ownchores.go for the rules.

const scheduleInsertSQL = `INSERT INTO chore_schedules (chore_id, assigned_to, assignment_type, fcfs_group_id, day_of_week, specific_date, available_at, points_multiplier, start_date, end_date, recurrence_interval, recurrence_start, due_by, expiry_penalty, expiry_penalty_value)
	 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func insertSchedulesTx(ctx context.Context, tx *sql.Tx, choreID int64, schedules []model.ChoreSchedule) error {
	for i := range schedules {
		cs := &schedules[i]
		cs.ChoreID = choreID
		res, err := tx.ExecContext(ctx, scheduleInsertSQL,
			cs.ChoreID, cs.AssignedTo, cs.AssignmentType, cs.FcfsGroupID, cs.DayOfWeek, cs.SpecificDate, cs.AvailableAt, cs.PointsMultiplier, cs.StartDate, cs.EndDate, cs.RecurrenceInterval, cs.RecurrenceStart, cs.DueBy, cs.ExpiryPenalty, cs.ExpiryPenaltyValue)
		if err != nil {
			return err
		}
		cs.ID, _ = res.LastInsertId()
	}
	return nil
}

// ListOwnChores returns the chores userID added for themselves, by title.
func (s *Store) ListOwnChores(ctx context.Context, userID int64) ([]model.Chore, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, description, category, icon, points_value, missed_penalty_value, estimated_minutes, requires_approval, requires_photo, photo_source, source, external_id, tts_audio_url, created_by, created_at
		 FROM chores WHERE source = ? AND created_by = ? ORDER BY title, id`, model.ChoreSourceSelf, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chores []model.Chore
	for rows.Next() {
		var c model.Chore
		var reqApp, reqPho int
		if err := rows.Scan(&c.ID, &c.Title, &c.Description, &c.Category, &c.Icon, &c.PointsValue, &c.MissedPenaltyValue, &c.EstimatedMinutes, &reqApp, &reqPho, &c.PhotoSource, &c.Source, &c.ExternalID, &c.TTSAudioURL, &c.CreatedBy, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.RequiresApproval = reqApp == 1
		c.RequiresPhoto = reqPho == 1
		chores = append(chores, c)
	}
	return chores, rows.Err()
}

// CreateChoreWithSchedules inserts a chore and its schedules in one
// transaction, so a bad schedule never leaves a chore nobody sees.
func (s *Store) CreateChoreWithSchedules(ctx context.Context, c *model.Chore, schedules []model.ChoreSchedule) error {
	photoSource := c.PhotoSource
	if photoSource == "" {
		photoSource = model.PhotoSourceChild
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO chores (title, description, category, icon, points_value, missed_penalty_value, estimated_minutes, requires_approval, requires_photo, photo_source, source, external_id, tts_audio_url, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Title, c.Description, c.Category, c.Icon, c.PointsValue, c.MissedPenaltyValue, c.EstimatedMinutes, boolToInt(c.RequiresApproval), boolToInt(c.RequiresPhoto), photoSource, c.Source, c.ExternalID, c.TTSAudioURL, c.CreatedBy)
	if err != nil {
		return err
	}
	c.ID, _ = res.LastInsertId()
	if err := insertSchedulesTx(ctx, tx, c.ID, schedules); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceChoreSchedules swaps a chore's schedules for new ones. A schedule
// that already has completions is ended on endOld (yesterday) rather than
// deleted, so its history survives; the rest are deleted.
func (s *Store) ReplaceChoreSchedules(ctx context.Context, choreID int64, endOld string, schedules []model.ChoreSchedule) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM chore_schedules WHERE chore_id = ?
		   AND NOT EXISTS (SELECT 1 FROM chore_completions cc WHERE cc.chore_schedule_id = chore_schedules.id)`, choreID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE chore_schedules SET end_date = ?
		  WHERE chore_id = ? AND (end_date IS NULL OR end_date > ?)`, endOld, choreID, endOld); err != nil {
		return err
	}
	if err := insertSchedulesTx(ctx, tx, choreID, schedules); err != nil {
		return err
	}
	return tx.Commit()
}

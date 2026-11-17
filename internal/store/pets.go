package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/liftedkilt/openchore/internal/model"
)

// ErrPetExists is returned when a person who already has a pet adopts another.
var ErrPetExists = fmt.Errorf("you already have a pet")

const petColumns = `id, user_id, name, species, color, health, fullness, joy, accessory, last_missed_date, last_day_checked, created_at`

func scanPet(row interface{ Scan(...any) error }) (*model.Pet, error) {
	p := &model.Pet{}
	var missed sql.NullString
	if err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.Species, &p.Color, &p.Health, &p.Fullness, &p.Joy, &p.Accessory,
		&missed, &p.LastDayChecked, &p.CreatedAt); err != nil {
		return nil, err
	}
	if missed.Valid {
		m := missed.String
		p.LastMissedDate = &m
	}
	return p, nil
}

// GetPetForUser returns the person's pet (with its accessories), or nil if
// they haven't adopted one.
func (s *Store) GetPetForUser(ctx context.Context, userID int64) (*model.Pet, error) {
	p, err := scanPet(s.db.QueryRowContext(ctx,
		`SELECT `+petColumns+` FROM pets WHERE user_id = ?`, userID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	items, err := s.petItems(ctx, `WHERE pet_id = ?`, p.ID)
	if err != nil {
		return nil, err
	}
	p.Items = items[p.ID]
	if p.Items == nil {
		p.Items = []string{}
	}
	return p, nil
}

func (s *Store) petItems(ctx context.Context, where string, args ...any) (map[int64][]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT pet_id, item FROM pet_items `+where+` ORDER BY acquired_at, item`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]string{}
	for rows.Next() {
		var id int64
		var it string
		if err := rows.Scan(&id, &it); err != nil {
			return nil, err
		}
		out[id] = append(out[id], it)
	}
	return out, rows.Err()
}

// AddPetItem gives a pet an accessory and puts it on. Owning it already is
// fine (it's just worn again).
func (s *Store) AddPetItem(ctx context.Context, userID int64, item string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO pet_items (pet_id, item) SELECT id, ? FROM pets WHERE user_id = ?`, item, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE pets SET accessory = ? WHERE user_id = ?`, item, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// PetOwnsItem reports whether the person's pet already has the accessory.
func (s *Store) PetOwnsItem(ctx context.Context, userID int64, item string) (bool, error) {
	var ok bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM pet_items pi JOIN pets p ON p.id = pi.pet_id WHERE p.user_id = ? AND pi.item = ?)`,
		userID, item).Scan(&ok)
	return ok, err
}

// SetPetAccessory puts on an owned accessory ("" takes it off).
func (s *Store) SetPetAccessory(ctx context.Context, userID int64, item string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE pets SET accessory = ? WHERE user_id = ?`, item, userID)
	return err
}

// ListPets returns every pet in the household.
func (s *Store) ListPets(ctx context.Context) ([]model.Pet, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+petColumns+` FROM pets ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pets []model.Pet
	for rows.Next() {
		p, err := scanPet(rows)
		if err != nil {
			return nil, err
		}
		p.Items = []string{}
		pets = append(pets, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	items, err := s.petItems(ctx, ``)
	if err != nil {
		return nil, err
	}
	for i := range pets {
		if it := items[pets[i].ID]; it != nil {
			pets[i].Items = it
		}
	}
	return pets, nil
}

// CreatePet adopts a pet. Callers set LastDayChecked to adoption day, so the
// first day judged is the next one.
func (s *Store) CreatePet(ctx context.Context, p *model.Pet) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO pets (user_id, name, species, color, health, fullness, joy, last_day_checked)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.UserID, p.Name, p.Species, p.Color, p.Health, p.Fullness, p.Joy, p.LastDayChecked)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return ErrPetExists
		}
		return err
	}
	p.ID, _ = res.LastInsertId()
	return nil
}

// RenamePet renames a person's pet.
func (s *Store) RenamePet(ctx context.Context, userID int64, name string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE pets SET name = ? WHERE user_id = ?`, name, userID)
	return err
}

// SetPetLook changes the animal and/or coat; health, food, joy, accessories
// and history stay as they are.
func (s *Store) SetPetLook(ctx context.Context, userID int64, species, color string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE pets SET species = ?, color = ? WHERE user_id = ?`, species, color, userID)
	return err
}

// SavePetDay stores the outcome of one or more daily checks. The
// last_day_checked guard makes a concurrent or repeated run a no-op.
func (s *Store) SavePetDay(ctx context.Context, p *model.Pet, prevDayChecked string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE pets SET health = ?, fullness = ?, joy = ?, last_missed_date = ?, last_day_checked = ?
		 WHERE id = ? AND last_day_checked = ?`,
		p.Health, p.Fullness, p.Joy, p.LastMissedDate, p.LastDayChecked, p.ID, prevDayChecked)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// AdjustPetNeeds adds the deltas to fullness and joy (clamped to 0..100) in
// a single statement, so it never races the daily check.
func (s *Store) AdjustPetNeeds(ctx context.Context, userID int64, fullness, joy int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE pets SET
			fullness = MAX(0, MIN(100, fullness + ?)),
			joy      = MAX(0, MIN(100, joy + ?))
		 WHERE user_id = ?`, fullness, joy, userID)
	return err
}

// DeletePetForUser removes a person's pet (a grown-up's choice).
func (s *Store) DeletePetForUser(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM pets WHERE user_id = ?`, userID)
	return err
}

// CountPetRewards returns how many rewards carry a pet action.
func (s *Store) CountPetRewards(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM rewards WHERE pet_action != ''`).Scan(&n)
	return n, err
}

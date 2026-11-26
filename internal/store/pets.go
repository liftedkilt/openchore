package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/liftedkilt/openchore/internal/model"
)

// ErrPetExists is returned when a person who already has a pet adopts another.
var ErrPetExists = fmt.Errorf("you already have a pet")

const petColumns = `id, user_id, name, species, color, health, fullness, joy, accessory, sprite, sprite_frames, sprite_approved, sprite_character, last_missed_date, last_day_checked, sick_days, created_at`

func scanPet(row interface{ Scan(...any) error }) (*model.Pet, error) {
	p := &model.Pet{}
	var missed sql.NullString
	if err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.Species, &p.Color, &p.Health, &p.Fullness, &p.Joy, &p.Accessory, &p.Sprite, &p.SpriteFrames, &p.SpriteApproved, &p.Character,
		&missed, &p.LastDayChecked, &p.SickDays, &p.CreatedAt); err != nil {
		return nil, err
	}
	if p.Sprite == "" {
		p.SpriteFrames, p.SpriteApproved, p.Character = 0, false, ""
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

// ListPets returns every pet in the household. The result is never nil, so
// an empty farm serialises as [] rather than null.
func (s *Store) ListPets(ctx context.Context) ([]model.Pet, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+petColumns+` FROM pets ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pets := []model.Pet{}
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

// SetPetSprite sets (or with "" clears) the person's own animal picture and
// returns the id it replaced, so the caller can delete that file. character
// names the ready-made character the picture is (model.PetCharacters), or
// "" for an upload. A character is the app's own art, so it may go to friend
// families straight away; an upload waits for a parent's approval.
func (s *Store) SetPetSprite(ctx context.Context, userID int64, id string, frames int, character string) (old string, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, `SELECT sprite FROM pets WHERE user_id = ?`, userID).Scan(&old); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE pets SET sprite = ?, sprite_frames = ?, sprite_approved = ?, sprite_character = ? WHERE user_id = ?`, id, max(frames, 1), character != "", character, userID); err != nil {
		return "", err
	}
	return old, tx.Commit()
}

// RedemptionCharacter says who a redemption was for and which character it
// bought, or "" if it wasn't a "character" reward.
func (s *Store) RedemptionCharacter(ctx context.Context, redemptionID int64) (userID int64, character string, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT rr.user_id, r.pet_item FROM reward_redemptions rr
		JOIN rewards r ON r.id = rr.reward_id
		WHERE rr.id = ? AND r.pet_action = ?`, redemptionID, model.PetActionCharacter).Scan(&userID, &character)
	if err == sql.ErrNoRows {
		return 0, "", nil
	}
	return userID, character, err
}

// UndoPetCharacter puts the person's animal back to the drawn one if it is
// still the character an undone redemption bought, and returns the picture
// id to delete ("" if nothing changed). A later picture or character
// purchase means the animal isn't that redemption's any more, so it is left
// alone.
func (s *Store) UndoPetCharacter(ctx context.Context, userID int64, character string, redemptionID int64) (old string, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var later int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM reward_redemptions rr
		JOIN rewards r ON r.id = rr.reward_id
		WHERE rr.user_id = ? AND rr.id > ? AND r.pet_action IN (?, ?)`,
		userID, redemptionID, model.PetActionCharacter, model.PetActionPicture).Scan(&later); err != nil {
		return "", err
	}
	if later > 0 {
		return "", nil
	}
	err = tx.QueryRowContext(ctx, `SELECT sprite FROM pets WHERE user_id = ? AND sprite_character = ? AND sprite != ''`, userID, character).Scan(&old)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE pets SET sprite = '', sprite_frames = 1, sprite_approved = 0, sprite_character = '' WHERE user_id = ?`, userID); err != nil {
		return "", err
	}
	return old, tx.Commit()
}

// SetPetSpriteApproved records whether a parent has said the person's
// picture (the one with this id) may be sent to friend families. It reports
// whether that picture is still the person's.
func (s *Store) SetPetSpriteApproved(ctx context.Context, userID int64, sprite string, approved bool) (bool, error) {
	// For the picture the parent was looking at: if the kid has swapped it
	// since, this matches nothing rather than approving one nobody has seen.
	res, err := s.db.ExecContext(ctx, `UPDATE pets SET sprite_approved = ? WHERE user_id = ? AND sprite != '' AND sprite = ?`, approved, userID, sprite)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// SavePetDay stores the outcome of one or more daily checks. The
// last_day_checked guard makes a concurrent or repeated run a no-op.
//
// prevSickDays guards the sick-day count the same way: a parent turning pet
// deaths on, or shortening them, caps every count (SetPetSettings), and a
// check that loaded the old count before that must not write it back.
func (s *Store) SavePetDay(ctx context.Context, p *model.Pet, prevDayChecked string, prevSickDays int) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE pets SET health = ?, fullness = ?, joy = ?, last_missed_date = ?, last_day_checked = ?, sick_days = ?
		 WHERE id = ? AND last_day_checked = ? AND sick_days = ?`,
		p.Health, p.Fullness, p.Joy, p.LastMissedDate, p.LastDayChecked, p.SickDays, p.ID, prevDayChecked, prevSickDays)
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

// BuryPet records that a pet died on diedOn and removes it (with its
// accessories), leaving a grave on the farm. Like SavePetDay, the
// last_day_checked guard makes a concurrent or repeated run a no-op; it
// reports whether this call did it.
//
// It only goes ahead if nothing changed since the check loaded the pet (the
// same last_day_checked and sick_days) and the settings stored right now
// still say it dies: farm pets on, and pet deaths on with at most
// p.SickDays days. Otherwise it reports false and changes nothing.
func (s *Store) BuryPet(ctx context.Context, p *model.Pet, prevDayChecked string, prevSickDays int, diedOn string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx,
		`DELETE FROM pets WHERE id = ? AND last_day_checked = ? AND sick_days = ?
		   AND EXISTS (SELECT 1 FROM app_settings WHERE key = ? AND value = 'true')
		   AND EXISTS (SELECT 1 FROM app_settings WHERE key = ?
		               AND CAST(value AS INTEGER) BETWEEN 1 AND ?)`,
		p.ID, prevDayChecked, prevSickDays, SettingPetsEnabled, SettingPetDeathDays, p.SickDays)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return false, nil
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO pet_graves (user_id, name, species, color, adopted_on, died_on) VALUES (?, ?, ?, ?, ?, ?)`,
		p.UserID, p.Name, p.Species, p.Color, p.CreatedAt.Local().Format(model.DateFormat), diedOn); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// ListPetGraves returns the farm's graves, oldest first. Never nil.
func (s *Store) ListPetGraves(ctx context.Context) ([]model.PetGrave, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name, species, color, adopted_on, died_on, created_at FROM pet_graves ORDER BY died_on, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.PetGrave{}
	for rows.Next() {
		var g model.PetGrave
		if err := rows.Scan(&g.ID, &g.UserID, &g.Name, &g.Species, &g.Color, &g.AdoptedOn, &g.DiedOn, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// DeletePetGrave removes a grave from the farm (a grown-up's choice). It
// reports whether there was one.
func (s *Store) DeletePetGrave(ctx context.Context, id int64) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM pet_graves WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Farm pet settings, shared by the API and the nightly check.
const (
	SettingPetsEnabled  = "pets_enabled"
	SettingPetDeathDays = "pets_death_days" // nights in a row sick after missed chores; "" or "0" = never
)

// ParsePetDeathDays reads a stored pets_death_days value (0 = off).
func ParsePetDeathDays(v string) int {
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// SetPetSettings saves farm pets on/off and, when deathDays isn't nil, the
// pet deaths setting, in one transaction with the sick-day caps they need,
// so an animal never dies without a night's warning:
//   - turning farm pets on (no deaths happen while they're off, but the
//     nightly check still counts) starts every count afresh;
//   - turning deaths on starts every count afresh, and a shorter time leaves
//     every animal at least one night.
//
// Callers validate first. It returns the death setting now in force.
func (s *Store) SetPetSettings(ctx context.Context, enabled bool, deathDays *int) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	get := func(key string) (string, error) {
		var v string
		err := tx.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&v)
		if err == sql.ErrNoRows {
			return "", nil
		}
		return v, err
	}
	set := func(key, v string) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO app_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, v)
		return err
	}
	capTo := func(n int) error {
		_, err := tx.ExecContext(ctx, `UPDATE pets SET sick_days = MIN(sick_days, ?)`, max(n, 0))
		return err
	}
	on, err := get(SettingPetsEnabled)
	if err != nil {
		return 0, err
	}
	v, err := get(SettingPetDeathDays)
	if err != nil {
		return 0, err
	}
	cur := ParsePetDeathDays(v)
	if enabled && on != "true" {
		if err := capTo(0); err != nil {
			return 0, err
		}
	}
	if deathDays != nil {
		n := *deathDays
		switch {
		case n > 0 && cur == 0:
			err = capTo(0)
		case n > 0 && n < cur:
			err = capTo(n - 1)
		}
		if err != nil {
			return 0, err
		}
		if err := set(SettingPetDeathDays, strconv.Itoa(n)); err != nil {
			return 0, err
		}
		cur = n
	}
	val := "false"
	if enabled {
		val = "true"
	}
	if err := set(SettingPetsEnabled, val); err != nil {
		return 0, err
	}
	return cur, tx.Commit()
}

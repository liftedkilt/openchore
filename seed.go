package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/mattn/go-sqlite3"

	"github.com/liftedkilt/openchore/migrations"
)

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "openchore.db"
	}

	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Run migrations to ensure schema exists
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		log.Fatalf("failed to create migration driver: %v", err)
	}
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		log.Fatalf("failed to create migration source: %v", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "sqlite3", driver)
	if err != nil {
		log.Fatalf("failed to create migrator: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("failed to run migrations: %v", err)
	}
	fmt.Println("Migrations complete")

	// Clear existing to avoid conflicts
	for _, table := range []string{
		"webhook_deliveries", "webhooks", "user_decay_config",
		"point_transactions", "reward_redemptions", "reward_assignments",
		"user_streaks", "chore_completions", "chore_schedules",
		"chores", "rewards", "streak_rewards", "users",
	} {
		db.Exec("DELETE FROM " + table)
	}
	db.Exec("DELETE FROM sqlite_sequence")

	// =========================================================================
	// USERS: ID 1=Dad, 2=Mom, 3=Natalie(12), 4=Zoe(10), 5=David(6)
	// =========================================================================
	users := []struct {
		name string
		role string
		age  int
		url  string
	}{
		{"Dad", "admin", 0, "https://api.dicebear.com/9.x/avataaars-neutral/svg?seed=Dad"},
		{"Mom", "admin", 0, "https://api.dicebear.com/9.x/avataaars-neutral/svg?seed=Mom"},
		{"Natalie", "child", 12, "https://api.dicebear.com/9.x/avataaars-neutral/svg?seed=Natalie"},
		{"Zoe", "child", 10, "https://api.dicebear.com/9.x/avataaars-neutral/svg?seed=Zoe"},
		{"David", "child", 6, "https://api.dicebear.com/9.x/avataaars-neutral/svg?seed=David"},
	}

	for _, u := range users {
		var err error
		if u.age > 0 {
			_, err = db.Exec(`INSERT INTO users (name, role, avatar_url, age) VALUES (?, ?, ?, ?)`, u.name, u.role, u.url, u.age)
		} else {
			_, err = db.Exec(`INSERT INTO users (name, role, avatar_url) VALUES (?, ?, ?)`, u.name, u.role, u.url)
		}
		if err != nil {
			fmt.Printf("Error inserting user %s: %v\n", u.name, err)
		} else {
			fmt.Printf("Inserted user: %s\n", u.name)
		}
	}

	// =========================================================================
	// CHORES
	// =========================================================================
	type chore struct {
		title       string
		description string
		category    string
		points      int
		minutes     int
	}
	chores := []chore{
		// ID 1
		{"Feed Cats (Morning)", "Wet food + fresh water for both cats", "required", 5, 5},
		// ID 2
		{"Feed Cats (Evening)", "Wet food + check water bowls", "required", 5, 5},
		// ID 3
		{"Make Bed", "Straighten sheets, blanket, and pillows", "required", 5, 3},
		// ID 4
		{"Brush Teeth (Morning)", "Brush for 2 full minutes", "required", 5, 3},
		// ID 5
		{"Brush Teeth (Evening)", "Brush and floss before bed", "required", 5, 3},
		// ID 6
		{"Empty Dishwasher", "Put away all clean dishes", "core", 10, 10},
		// ID 7
		{"Load Dishwasher", "Rinse and load dirty dishes after dinner", "core", 10, 10},
		// ID 8
		{"Sweep Kitchen", "Sweep the kitchen floor", "core", 10, 10},
		// ID 9
		{"Clean Room", "Pick up floor, organize desk, put away clothes", "core", 15, 15},
		// ID 10
		{"Take Out Trash", "Empty kitchen trash and recycling to bins outside", "core", 10, 5},
		// ID 11
		{"Fold Laundry", "Fold your clean laundry and put it away", "core", 10, 15},
		// ID 12
		{"Set Table", "Plates, silverware, napkins, and cups for dinner", "core", 5, 5},
		// ID 13
		{"Pick Up Toys", "Put all toys back where they belong", "core", 5, 10},
		// ID 14
		{"Read 20 Minutes", "Read a book or chapter for at least 20 minutes", "bonus", 10, 20},
		// ID 15
		{"Practice Piano", "Practice piano for 15 minutes", "bonus", 10, 15},
		// ID 16
		{"Help a Sibling", "Help a brother or sister with one of their chores", "bonus", 15, 10},
		// ID 17
		{"Write in Journal", "Write at least a half page in your journal", "bonus", 10, 10},
		// ID 18
		{"Wipe Bathroom Counter", "Spray and wipe down the bathroom counter and sink", "core", 10, 5},
	}

	for _, c := range chores {
		_, err := db.Exec(
			`INSERT INTO chores (title, description, category, points_value, estimated_minutes, created_by) VALUES (?, ?, ?, ?, ?, 1)`,
			c.title, c.description, c.category, c.points, c.minutes)
		if err != nil {
			fmt.Printf("Error inserting chore %s: %v\n", c.title, err)
		} else {
			fmt.Printf("Inserted chore: %s\n", c.title)
		}
	}

	// =========================================================================
	// SCHEDULES
	// =========================================================================
	// Helper to insert a schedule with all columns
	insertSchedule := func(choreID, userID, dow int, availableAt, dueBy, expiryPenalty string, expiryPenaltyValue int) {
		_, err := db.Exec(
			`INSERT INTO chore_schedules (chore_id, assigned_to, day_of_week, available_at, due_by, expiry_penalty, expiry_penalty_value)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			choreID, userID, dow,
			nilIfEmpty(availableAt), nilIfEmpty(dueBy),
			expiryPenalty, expiryPenaltyValue)
		if err != nil {
			fmt.Printf("Error inserting schedule chore=%d user=%d dow=%d: %v\n", choreID, userID, dow, err)
		}
	}

	fmt.Println("\nSeeding schedules...")

	// --- Daily required chores for all kids ---
	for day := 0; day < 7; day++ {
		// Natalie (3) — feeds cats, has deadlines on morning/evening routines
		insertSchedule(1, 3, day, "07:00", "09:00", "no_points", 0)  // Feed Cats Morning — due by 9am, 0 pts if late
		insertSchedule(2, 3, day, "19:00", "21:00", "no_points", 0)  // Feed Cats Evening — due by 9pm, 0 pts if late
		insertSchedule(3, 3, day, "", "", "block", 0)                 // Make Bed
		insertSchedule(4, 3, day, "07:00", "09:00", "no_points", 0)  // Brush Teeth Morning
		insertSchedule(5, 3, day, "20:00", "21:30", "no_points", 0)  // Brush Teeth Evening

		// Zoe (4)
		insertSchedule(3, 4, day, "", "", "block", 0)                 // Make Bed
		insertSchedule(4, 4, day, "07:00", "09:00", "no_points", 0)  // Brush Teeth Morning
		insertSchedule(5, 4, day, "20:00", "21:30", "no_points", 0)  // Brush Teeth Evening

		// David (5)
		insertSchedule(3, 5, day, "", "", "block", 0)                 // Make Bed
		insertSchedule(4, 5, day, "07:00", "09:00", "no_points", 0)  // Brush Teeth Morning
		insertSchedule(5, 5, day, "20:00", "21:30", "no_points", 0)  // Brush Teeth Evening
	}

	// --- Core chores with rotating/specific days ---
	// Natalie: Empty dishwasher (Mon/Wed/Fri), Clean room (Sat), Fold laundry (Sun), Wipe bathroom (Thu)
	for _, day := range []int{1, 3, 5} {
		insertSchedule(6, 3, day, "", "", "block", 0) // Empty Dishwasher
	}
	insertSchedule(9, 3, 6, "", "", "block", 0)  // Clean Room - Sat
	insertSchedule(11, 3, 0, "", "", "block", 0)  // Fold Laundry - Sun
	insertSchedule(18, 3, 4, "", "", "block", 0)  // Wipe Bathroom - Thu

	// Zoe: Load dishwasher (Tue/Thu/Sat), Sweep kitchen (Mon/Fri), Clean room (Sat), Set table daily
	for _, day := range []int{2, 4, 6} {
		insertSchedule(7, 4, day, "18:00", "", "block", 0) // Load Dishwasher
	}
	for _, day := range []int{1, 5} {
		insertSchedule(8, 4, day, "", "", "block", 0) // Sweep Kitchen
	}
	insertSchedule(9, 4, 6, "", "", "block", 0) // Clean Room - Sat
	for day := 0; day < 7; day++ {
		insertSchedule(12, 4, day, "17:00", "19:00", "penalty", 3) // Set Table — 3 pt penalty if late
	}

	// David: Pick up toys daily, Take out trash (Wed/Sat), Set table (Tue/Thu)
	for day := 0; day < 7; day++ {
		insertSchedule(13, 5, day, "", "", "block", 0) // Pick Up Toys
	}
	for _, day := range []int{3, 6} {
		insertSchedule(10, 5, day, "", "", "block", 0) // Take Out Trash
	}
	for _, day := range []int{2, 4} {
		insertSchedule(12, 5, day, "17:00", "19:00", "penalty", 3) // Set Table — 3 pt penalty if late
	}

	// --- Bonus chores — available to all kids, all week ---
	bonusChores := []int{14, 15, 16, 17}
	for _, choreID := range bonusChores {
		for day := 0; day < 7; day++ {
			insertSchedule(choreID, 3, day, "", "", "block", 0) // Natalie
			insertSchedule(choreID, 4, day, "", "", "block", 0) // Zoe
		}
		if choreID == 14 || choreID == 16 { // Read + Help a Sibling for David too
			for day := 0; day < 7; day++ {
				insertSchedule(choreID, 5, day, "", "", "block", 0)
			}
		}
	}

	fmt.Println("Schedules seeded!")

	// =========================================================================
	// COMPLETIONS — seed today + past 3 days
	// =========================================================================
	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	twoDaysAgo := now.AddDate(0, 0, -2).Format("2006-01-02")
	threeDaysAgo := now.AddDate(0, 0, -3).Format("2006-01-02")

	todayDow := int(now.Weekday())
	yesterdayDow := int(now.AddDate(0, 0, -1).Weekday())
	twoDaysAgoDow := int(now.AddDate(0, 0, -2).Weekday())
	threeDaysAgoDow := int(now.AddDate(0, 0, -3).Weekday())

	completeChore := func(choreTitle string, userID int, dow int, completionDate string) {
		var scheduleID int64
		err := db.QueryRow(`
			SELECT cs.id FROM chore_schedules cs
			JOIN chores c ON c.id = cs.chore_id
			WHERE cs.assigned_to = ? AND cs.day_of_week = ? AND c.title = ?
			LIMIT 1`, userID, dow, choreTitle).Scan(&scheduleID)
		if err != nil {
			fmt.Printf("  WARN: schedule not found for %s (user %d, dow %d): %v\n", choreTitle, userID, dow, err)
			return
		}

		res, err := db.Exec(`
			INSERT INTO chore_completions (chore_schedule_id, completed_by, completion_date)
			VALUES (?, ?, ?)`, scheduleID, userID, completionDate)
		if err != nil {
			fmt.Printf("  WARN: completion insert failed for %s: %v\n", choreTitle, err)
			return
		}
		completionID, _ := res.LastInsertId()

		var points int
		db.QueryRow(`SELECT points_value FROM chores WHERE title = ?`, choreTitle).Scan(&points)
		if points > 0 {
			db.Exec(`INSERT INTO point_transactions (user_id, amount, reason, reference_id, note)
				VALUES (?, ?, 'chore_complete', ?, ?)`, userID, points, completionID, choreTitle)
		}
	}

	fmt.Println("\nSeeding completions...")

	// --- TODAY ---
	fmt.Printf("Seeding today (%s, dow=%d)...\n", today, todayDow)
	// Natalie (3): morning routine done, some bonus
	completeChore("Brush Teeth (Morning)", 3, todayDow, today)
	completeChore("Feed Cats (Morning)", 3, todayDow, today)
	completeChore("Make Bed", 3, todayDow, today)
	completeChore("Read 20 Minutes", 3, todayDow, today)
	completeChore("Practice Piano", 3, todayDow, today)
	completeChore("Feed Cats (Evening)", 3, todayDow, today)
	completeChore("Write in Journal", 3, todayDow, today)
	// Zoe (4): moderate progress
	completeChore("Make Bed", 4, todayDow, today)
	completeChore("Brush Teeth (Morning)", 4, todayDow, today)
	completeChore("Read 20 Minutes", 4, todayDow, today)
	// David (5): a few done
	completeChore("Make Bed", 5, todayDow, today)
	completeChore("Brush Teeth (Morning)", 5, todayDow, today)
	completeChore("Pick Up Toys", 5, todayDow, today)

	// --- YESTERDAY ---
	fmt.Printf("Seeding yesterday (%s, dow=%d)...\n", yesterday, yesterdayDow)
	// All kids completed all required chores (for streak)
	completeChore("Feed Cats (Morning)", 3, yesterdayDow, yesterday)
	completeChore("Feed Cats (Evening)", 3, yesterdayDow, yesterday)
	completeChore("Make Bed", 3, yesterdayDow, yesterday)
	completeChore("Brush Teeth (Morning)", 3, yesterdayDow, yesterday)
	completeChore("Brush Teeth (Evening)", 3, yesterdayDow, yesterday)
	completeChore("Read 20 Minutes", 3, yesterdayDow, yesterday)
	completeChore("Practice Piano", 3, yesterdayDow, yesterday)
	completeChore("Help a Sibling", 3, yesterdayDow, yesterday)
	completeChore("Write in Journal", 3, yesterdayDow, yesterday)
	completeChore("Make Bed", 4, yesterdayDow, yesterday)
	completeChore("Brush Teeth (Morning)", 4, yesterdayDow, yesterday)
	completeChore("Brush Teeth (Evening)", 4, yesterdayDow, yesterday)
	completeChore("Read 20 Minutes", 4, yesterdayDow, yesterday)
	completeChore("Make Bed", 5, yesterdayDow, yesterday)
	completeChore("Brush Teeth (Morning)", 5, yesterdayDow, yesterday)
	completeChore("Brush Teeth (Evening)", 5, yesterdayDow, yesterday)
	completeChore("Pick Up Toys", 5, yesterdayDow, yesterday)
	completeChore("Read 20 Minutes", 5, yesterdayDow, yesterday)

	// --- 2 DAYS AGO ---
	fmt.Printf("Seeding 2 days ago (%s, dow=%d)...\n", twoDaysAgo, twoDaysAgoDow)
	completeChore("Feed Cats (Morning)", 3, twoDaysAgoDow, twoDaysAgo)
	completeChore("Feed Cats (Evening)", 3, twoDaysAgoDow, twoDaysAgo)
	completeChore("Make Bed", 3, twoDaysAgoDow, twoDaysAgo)
	completeChore("Brush Teeth (Morning)", 3, twoDaysAgoDow, twoDaysAgo)
	completeChore("Brush Teeth (Evening)", 3, twoDaysAgoDow, twoDaysAgo)
	completeChore("Make Bed", 4, twoDaysAgoDow, twoDaysAgo)
	completeChore("Brush Teeth (Morning)", 4, twoDaysAgoDow, twoDaysAgo)
	completeChore("Brush Teeth (Evening)", 4, twoDaysAgoDow, twoDaysAgo)
	completeChore("Make Bed", 5, twoDaysAgoDow, twoDaysAgo)
	completeChore("Brush Teeth (Morning)", 5, twoDaysAgoDow, twoDaysAgo)
	completeChore("Brush Teeth (Evening)", 5, twoDaysAgoDow, twoDaysAgo)
	completeChore("Pick Up Toys", 5, twoDaysAgoDow, twoDaysAgo)

	// --- 3 DAYS AGO ---
	fmt.Printf("Seeding 3 days ago (%s, dow=%d)...\n", threeDaysAgo, threeDaysAgoDow)
	completeChore("Feed Cats (Morning)", 3, threeDaysAgoDow, threeDaysAgo)
	completeChore("Feed Cats (Evening)", 3, threeDaysAgoDow, threeDaysAgo)
	completeChore("Make Bed", 3, threeDaysAgoDow, threeDaysAgo)
	completeChore("Brush Teeth (Morning)", 3, threeDaysAgoDow, threeDaysAgo)
	completeChore("Brush Teeth (Evening)", 3, threeDaysAgoDow, threeDaysAgo)
	completeChore("Make Bed", 4, threeDaysAgoDow, threeDaysAgo)
	completeChore("Brush Teeth (Morning)", 4, threeDaysAgoDow, threeDaysAgo)
	completeChore("Brush Teeth (Evening)", 4, threeDaysAgoDow, threeDaysAgo)
	completeChore("Make Bed", 5, threeDaysAgoDow, threeDaysAgo)
	completeChore("Brush Teeth (Morning)", 5, threeDaysAgoDow, threeDaysAgo)
	completeChore("Brush Teeth (Evening)", 5, threeDaysAgoDow, threeDaysAgo)
	completeChore("Pick Up Toys", 5, threeDaysAgoDow, threeDaysAgo)

	// =========================================================================
	// STREAKS
	// =========================================================================
	fmt.Println("Seeding streaks...")
	// Natalie: 4-day streak
	db.Exec(`INSERT OR REPLACE INTO user_streaks (user_id, current_streak, longest_streak, streak_start_date, last_completed_date)
		VALUES (3, 4, 4, ?, ?)`, threeDaysAgo, today)
	// Zoe: 3-day streak
	db.Exec(`INSERT OR REPLACE INTO user_streaks (user_id, current_streak, longest_streak, streak_start_date, last_completed_date)
		VALUES (4, 3, 3, ?, ?)`, twoDaysAgo, today)
	// David: 2-day streak
	db.Exec(`INSERT OR REPLACE INTO user_streaks (user_id, current_streak, longest_streak, streak_start_date, last_completed_date)
		VALUES (5, 2, 2, ?, ?)`, yesterday, today)

	// =========================================================================
	// REWARDS
	// =========================================================================
	fmt.Println("Seeding rewards...")
	db.Exec(`INSERT INTO rewards (name, description, icon, cost, stock, created_by) VALUES ('Extra Screen Time', '30 minutes of extra screen time', '📺', 50, NULL, 1)`)
	db.Exec(`INSERT INTO rewards (name, description, icon, cost, stock, created_by) VALUES ('Ice Cream Trip', 'A trip to the ice cream shop', '🍦', 100, NULL, 1)`)
	db.Exec(`INSERT INTO rewards (name, description, icon, cost, stock, created_by) VALUES ('Movie Night Pick', 'Pick the family movie on Friday', '🎬', 75, NULL, 1)`)
	db.Exec(`INSERT INTO rewards (name, description, icon, cost, stock, created_by) VALUES ('New Book', 'Pick a new book from the bookstore', '📚', 150, NULL, 1)`)
	db.Exec(`INSERT INTO rewards (name, description, icon, cost, stock, created_by) VALUES ('Skip a Chore', 'Skip one non-required chore', '🎉', 30, 3, 1)`)

	// =========================================================================
	// STREAK REWARDS
	// =========================================================================
	db.Exec(`INSERT INTO streak_rewards (streak_days, bonus_points, label) VALUES (3, 10, '3-Day Streak!')`)
	db.Exec(`INSERT INTO streak_rewards (streak_days, bonus_points, label) VALUES (7, 25, 'Week Warrior!')`)
	db.Exec(`INSERT INTO streak_rewards (streak_days, bonus_points, label) VALUES (14, 50, 'Two Week Champion!')`)
	db.Exec(`INSERT INTO streak_rewards (streak_days, bonus_points, label) VALUES (30, 100, 'Monthly Master!')`)

	// =========================================================================
	// DECAY CONFIG — enable for Natalie as example
	// =========================================================================
	db.Exec(`INSERT INTO user_decay_config (user_id, enabled, decay_rate, decay_interval_hours) VALUES (3, 1, 5, 24)`)

	fmt.Println("\nSeeding complete!")

	// Print summary
	var compCount int
	db.QueryRow("SELECT COUNT(*) FROM chore_completions").Scan(&compCount)
	var ptCount int
	db.QueryRow("SELECT COUNT(*) FROM point_transactions").Scan(&ptCount)
	var schedCount int
	db.QueryRow("SELECT COUNT(*) FROM chore_schedules").Scan(&schedCount)
	fmt.Printf("  %d schedules, %d completions, %d point transactions\n", schedCount, compCount, ptCount)

	for _, name := range []string{"Natalie", "Zoe", "David"} {
		var total, done int
		var balance int
		db.QueryRow(`SELECT COUNT(*) FROM chore_schedules cs
			JOIN chores c ON c.id = cs.chore_id
			JOIN users u ON u.id = cs.assigned_to
			WHERE u.name = ? AND cs.day_of_week = ?`, name, todayDow).Scan(&total)
		db.QueryRow(`SELECT COUNT(*) FROM chore_completions cc
			JOIN chore_schedules cs ON cs.id = cc.chore_schedule_id
			JOIN users u ON u.id = cc.completed_by
			WHERE u.name = ? AND cc.completion_date = ?`, name, today).Scan(&done)
		db.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM point_transactions pt
			JOIN users u ON u.id = pt.user_id
			WHERE u.name = ?`, name).Scan(&balance)
		fmt.Printf("  %s: %d/%d today, %d total pts\n", name, done, total, balance)
	}
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

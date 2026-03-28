package api

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/liftedkilt/openchore/internal/discord"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/internal/webhook"
)

func NewRouter(s *store.Store, dispatcher *webhook.Dispatcher) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.SetHeader("Content-Type", "application/json"))

	discordNotifier := discord.NewNotifier(s)

	users := NewUserHandler(s)
	chores := NewChoreHandler(s, dispatcher, discordNotifier)
	admin := NewAdminHandler(s)
	points := NewPointsHandler(s)
	rewards := NewRewardHandler(s, dispatcher)
	streaks := NewStreakHandler(s)
	webhooks := NewWebhookHandler(s)
	setup := NewSetupHandler(s)
	reports := NewReportsHandler(s)

	// Serve uploaded photos
	_ = os.MkdirAll("data/uploads", 0750)
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir("data/uploads"))))

	r.Route("/api", func(r chi.Router) {
		// Public: list users (for profile selection screen)
		r.Get("/users", users.List)
		r.Get("/users/{id}", users.Get)

		// Initial setup (only works when no users exist)
		r.Post("/setup", setup.Setup)

		// Admin passcode verification (no auth required)
		r.Post("/admin/verify", admin.VerifyPasscode)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(RequireUser(s))

			// Any user can view their chores, points, streak
			r.Get("/users/{id}/chores", users.GetChores)
			r.Get("/users/{id}/points", points.GetUserPoints)
			r.Get("/users/{id}/streak", streaks.GetUserStreak)
			r.Get("/users/{id}/redemptions", rewards.ListRedemptions)

			// Any user can update their own profile preferences
			r.Put("/users/{id}/theme", users.UpdateTheme)
			r.Put("/users/{id}/avatar", users.UpdateAvatar)

			// Any user can complete/uncomplete chores
			r.Post("/schedules/{scheduleID}/complete", chores.Complete)
			r.Delete("/schedules/{scheduleID}/complete", chores.Uncomplete)
			r.Post("/upload", chores.UploadPhoto)

			// Any user can view and redeem rewards
			r.Get("/rewards", rewards.List)
			r.Post("/rewards/{id}/redeem", rewards.Redeem)

			// Admin-only routes
			r.Group(func(r chi.Router) {
				r.Use(RequireAdmin)

				r.Post("/users", users.Create)
				r.Put("/users/{id}", users.Update)
				r.Delete("/users/{id}", users.DeleteUser)
				r.Put("/users/{id}/pause", users.Pause)
				r.Put("/users/{id}/unpause", users.Unpause)

				r.Get("/chores", chores.List)
				r.Post("/chores", chores.Create)
				r.Get("/chores/{id}", chores.Get)
				r.Put("/chores/{id}", chores.Update)
				r.Delete("/chores/{id}", chores.Delete)

				r.Get("/chores/{id}/schedules", chores.ListSchedules)
				r.Post("/chores/{id}/schedules", chores.CreateSchedule)
				r.Delete("/chores/{id}/schedules/{scheduleID}", chores.DeleteSchedule)

				// Settings
				r.Get("/admin/settings/{key}", admin.GetSetting)
				r.Put("/admin/settings/{key}", admin.SetSetting)

				// Approvals
				r.Get("/completions/pending", chores.ListPending)
				r.Post("/completions/{id}/approve", chores.Approve)
				r.Post("/completions/{id}/reject", chores.Reject)

				r.Put("/admin/passcode", admin.UpdatePasscode)

				// Points management
				r.Get("/points/balances", points.GetAllBalances)
				r.Post("/points/adjust", points.Adjust)
				r.Get("/admin/users/{id}/decay", points.GetDecayConfig)
				r.Put("/admin/users/{id}/decay", points.SetDecayConfig)

				// Rewards management
				r.Get("/rewards/all", rewards.ListAll)
				r.Post("/rewards", rewards.Create)
				r.Put("/rewards/{id}", rewards.Update)
				r.Put("/rewards/{id}/assignments", rewards.SetAssignments)
				r.Delete("/rewards/{id}", rewards.Delete)
				r.Delete("/redemptions/{redemptionID}", rewards.UndoRedemption)

				// Streak rewards management
				r.Get("/admin/streak-rewards", streaks.ListRewards)
				r.Post("/admin/streak-rewards", streaks.CreateReward)
				r.Delete("/admin/streak-rewards/{id}", streaks.DeleteReward)

				// Config export
				r.Get("/admin/export-config", admin.ExportConfig)

				// Reports
				r.Get("/admin/reports", reports.GetReports)

				// Webhooks management
				r.Get("/admin/webhooks", webhooks.List)
				r.Post("/admin/webhooks", webhooks.Create)
				r.Put("/admin/webhooks/{id}", webhooks.Update)
				r.Delete("/admin/webhooks/{id}", webhooks.Delete)
				r.Get("/admin/webhooks/{id}/deliveries", webhooks.ListDeliveries)
			})
		})
	})

	return r
}

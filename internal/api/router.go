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

// Auth bundles the authentication services the router needs.
type Auth struct {
	Sessions *SessionManager
	OIDC     *OIDCService
}

func NewRouter(s *store.Store, dispatcher *webhook.Dispatcher, auth Auth) (*chi.Mux, *ChoreHandler, *ReportsHandler) {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	discordNotifier := discord.NewNotifier(s)

	users := NewUserHandler(s, dispatcher)
	aiSvc := NewAIServices(s)
	chores := NewChoreHandler(s, dispatcher, discordNotifier, aiSvc)
	admin := NewAdminHandler(s, dispatcher)
	points := NewPointsHandler(s)
	rewards := NewRewardHandler(s, dispatcher)
	streaks := NewStreakHandler(s)
	webhooks := NewWebhookHandler(s)
	triggers := NewTriggerHandler(s)
	tokens := NewTokenHandler(s)
	setup := NewSetupHandler(s, auth.Sessions)
	reports := NewReportsHandler(s, dispatcher, discordNotifier, aiSvc)
	authH := NewAuthHandler(s, auth.Sessions, dispatcher, auth.OIDC)
	oidcH := auth.OIDC

	// Serve uploaded photos
	_ = os.MkdirAll("data/uploads", 0750)
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir("data/uploads"))))

	// Serve read-aloud chore audio
	r.Handle("/tts/*", ttsFileServer())

	r.Route("/api", func(r chi.Router) {
		r.Use(CheckOrigin(auth.OIDC.publicURL))

		// Public: list users (for profile selection screen)
		r.Get("/users", users.List)
		r.Get("/users/{id}", users.Get)

		// Public, read-only: per-user chores, points and streak. The wall
		// display (/ambient) shows these without anyone signing in.
		r.Get("/users/{id}/chores", users.GetChores)
		r.Get("/users/{id}/points", points.GetUserPoints)
		r.Get("/users/{id}/streak", streaks.GetUserStreak)

		// Sign-in: tap a profile (PIN when set) or continue with an OIDC provider
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/logout", authH.Logout)
		r.Get("/auth/providers", oidcH.Providers)
		r.Get("/auth/oidc/{provider}/start", oidcH.Start)
		r.Get("/auth/oidc/{provider}/callback", oidcH.Callback)

		// Public: chore trigger webhook (UUID is the auth)
		r.Post("/hooks/trigger/{uuid}", triggers.FireTrigger)

		// Initial setup (only works when no users exist)
		r.Post("/setup", setup.Setup)

		// Authenticated routes (session cookie, session bearer, or API token)
		r.Group(func(r chi.Router) {
			r.Use(RequireUserOrToken(s, auth.Sessions))

			r.Get("/auth/me", authH.Me)
			r.Post("/auth/logout-everywhere", authH.LogoutEverywhere)
			r.Post("/auth/upload-link", authH.UploadLink)
			r.Get("/users/{id}/identities", oidcH.ListIdentities)
			r.Delete("/users/{id}/identities/{identityID}", oidcH.UnlinkIdentity)

			r.Get("/users/{id}/redemptions", rewards.ListRedemptions)

			// Any user can update their own profile preferences
			r.Put("/users/{id}/theme", users.UpdateTheme)
			r.Put("/users/{id}/avatar", users.UpdateAvatar)
			r.Put("/users/{id}/line-color", users.UpdateLineColor)
			r.Put("/users/{id}/color", users.UpdateColor)
			r.Put("/users/{id}/pin", users.SetPin)
			r.Delete("/users/{id}/pin", users.ClearPin)

			// Any user can complete/uncomplete chores
			r.Post("/schedules/{scheduleID}/complete", chores.Complete)
			r.Delete("/schedules/{scheduleID}/complete", chores.Uncomplete)
			r.Post("/upload", chores.UploadPhoto)
			r.Put("/completions/{id}/photo", chores.AttachPhoto)

			// Any user can view and redeem rewards
			r.Get("/rewards", rewards.List)
			r.Post("/rewards/{id}/redeem", rewards.Redeem)

			// Reward commitments — kids commit points toward a chosen reward
			// and watch progress instead of being tempted to drain spendable.
			r.Get("/users/{id}/commitments", rewards.ListCommitments)
			r.Post("/rewards/{id}/commit", rewards.Commit)
			r.Post("/commitments/{id}/contribute", rewards.Contribute)
			r.Put("/commitments/{id}/auto-contribute", rewards.SetAutoContribute)
			r.Delete("/commitments/{id}", rewards.BreakCommitment)
			r.Get("/pools/{id}", rewards.GetSharedPool)

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
				r.Post("/schedules/{scheduleID}/excuse", chores.Excuse)

				// Chore triggers
				r.Get("/chores/{id}/triggers", triggers.ListForChore)
				r.Post("/chores/{id}/triggers", triggers.Create)
				r.Put("/triggers/{id}", triggers.Update)
				r.Delete("/triggers/{id}", triggers.Delete)

				// Settings
				r.Get("/admin/settings/{key}", admin.GetSetting)
				r.Put("/admin/settings/{key}", admin.SetSetting)

				// Approvals
				r.Get("/completions/pending", chores.ListPending)
				r.Post("/completions/{id}/approve", chores.Approve)
				r.Post("/completions/{id}/reject", chores.Reject)

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

				// API token management
				r.Get("/admin/tokens", tokens.List)
				r.Post("/admin/tokens", tokens.Create)
				r.Delete("/admin/tokens/{id}", tokens.Revoke)

				// Sign-in settings
				r.Get("/admin/auth/config", oidcH.AdminConfig)
				r.Put("/admin/auth/sessions", oidcH.UpdateSessionTTLs)
				r.Post("/admin/auth/providers", oidcH.CreateProvider)
				r.Put("/admin/auth/providers/{id}", oidcH.UpdateProvider)
				r.Delete("/admin/auth/providers/{id}", oidcH.DeleteProvider)
				r.Post("/admin/auth/providers/{id}/test", oidcH.TestProvider)

				// Optional AI and read-aloud audio
				r.Get("/admin/ai/status", chores.AIStatus)
				r.Get("/admin/ai/config", aiSvc.GetConfig)
				r.Put("/admin/ai/config", aiSvc.UpdateConfig)
				r.Post("/admin/ai/test", chores.TestAIReview)
				r.Post("/admin/ai/generate-description", chores.GenerateDescription)
				r.Get("/admin/reports/ai-summary", reports.GetAISummary)
				r.Post("/admin/tts/regenerate", chores.RegenerateAllTTS)
				r.Post("/chores/{id}/tts/regenerate", chores.RegenerateChoreTTS)

				// Integration discovery: chores with triggers
				r.Get("/chores/triggerable", triggers.ListTriggerable)
			})
		})
	})

	return r, chores, reports
}

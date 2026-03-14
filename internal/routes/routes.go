package routes

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"truyenm/backend/internal/handlers"
	"truyenm/backend/internal/middleware"
	"truyenm/backend/internal/models"
)

func Register(app *fiber.App, h *handlers.Handler) {
	app.Use(recover.New())
	app.Use(middleware.SecurityHeaders())
	app.Use(middleware.RequestLogger(h.Logger))

	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	api := app.Group("/api")
	api.Post("/auth/register", h.Register)
	api.Post("/auth/login", h.Login)
	api.Post("/auth/refresh", h.Refresh)
	api.Post("/auth/logout", h.Logout)

	auth := middleware.RequireAuth(h.Cfg.JWTSecret)
	optionalAuth := middleware.OptionalAuth(h.Cfg.JWTSecret)
	rateLimit := middleware.RateLimit(h.Redis, 100, time.Minute)

	// Public endpoints
	api.Get("/comics", h.ListComics)
	api.Get("/comics/:id", optionalAuth, h.GetComic)
	api.Get("/comics/slug/:slug/chapter/:number", h.GetChapterBySlug)
	api.Get("/chapter/:id", h.GetChapter)
	api.Get("/genres", h.ListGenres)
	api.Get("/leaderboard/users", h.ListTopReaders)
	api.Post("/comics/:id/view", optionalAuth, h.TrackView)

	// User endpoints
	api.Get("/me", auth, h.GetMyProfile)
	api.Get("/me/history", auth, h.GetReadingHistory)
	api.Get("/me/follows", auth, h.ListFollows)
	api.Get("/me/follows/updates", auth, h.ListFollowUpdates)
	api.Post("/comics/:id/follow", auth, h.FollowComic)
	api.Delete("/comics/:id/follow", auth, h.UnfollowComic)
	api.Post("/ads/check", auth, h.AdsCheck)
	api.Post("/reward/read", auth, rateLimit, h.RewardRead)
	api.Post("/auth/logout-all", auth, h.LogoutAll)

	// Support tickets
	api.Post("/support/ticket", auth, h.CreateTicket)
	api.Get("/support/tickets", auth, h.ListTickets)
	api.Get("/support/ticket/:id", auth, h.GetTicket)
	api.Post("/support/ticket/:id/reply", auth, h.ReplyTicket)
	api.Post("/support/ticket/:id/close", auth, h.CloseTicket)

	// Admin endpoints
	admin := api.Group("/admin", auth, middleware.RequireRole(models.RoleOwner, models.RoleAdmin, models.RoleStaff, models.RoleModerator))

	admin.Get("/stats", middleware.RequirePermission(h.DB, h.Redis, "admin.stats.read"), h.AdminStats)
	admin.Get("/analytics", middleware.RequirePermission(h.DB, h.Redis, "admin.analytics.read"), h.AdminAnalytics)
	admin.Get("/traffic", middleware.RequirePermission(h.DB, h.Redis, "admin.traffic.read"), h.GetTrafficStats)

	admin.Get("/users", middleware.RequirePermission(h.DB, h.Redis, "admin.users.read"), h.AdminListUsers)
	admin.Get("/users/:id", middleware.RequirePermission(h.DB, h.Redis, "admin.users.read"), h.AdminGetUser)
	admin.Post("/user/ban", middleware.RequirePermission(h.DB, h.Redis, "admin.users.ban"), h.AdminBanUser)
	admin.Post("/user/unban", middleware.RequirePermission(h.DB, h.Redis, "admin.users.ban"), h.AdminUnbanUser)
	admin.Post("/user/reset-points", middleware.RequirePermission(h.DB, h.Redis, "admin.users.reset_points"), h.AdminResetPoints)
	admin.Post("/user/role", middleware.RequirePermission(h.DB, h.Redis, "admin.users.role"), h.AdminChangeUserRole)
	admin.Post("/user/points", middleware.RequirePermission(h.DB, h.Redis, "admin.users.reset_points"), h.AdminManualPoints)

	admin.Get("/comics", middleware.RequirePermission(h.DB, h.Redis, "admin.comics.write"), h.AdminListComics)
	admin.Post("/comic", middleware.RequirePermission(h.DB, h.Redis, "admin.comics.write"), h.AdminCreateComic)
	admin.Put("/comic/:id", middleware.RequirePermission(h.DB, h.Redis, "admin.comics.write"), h.AdminUpdateComic)
	admin.Delete("/comic/:id", middleware.RequirePermission(h.DB, h.Redis, "admin.comics.write"), h.AdminDeleteComic)
	admin.Post("/comic/:id/crawler/run", middleware.RequirePermission(h.DB, h.Redis, "admin.comics.write"), h.AdminRunCrawlerNow)
	admin.Get("/comic/:id/crawler/logs", middleware.RequirePermission(h.DB, h.Redis, "admin.comics.write"), h.AdminListCrawlerLogs)

	admin.Post("/comic/:id/chapter", middleware.RequirePermission(h.DB, h.Redis, "admin.chapters.write"), h.AdminCreateChapter)
	admin.Put("/chapter/:id", middleware.RequirePermission(h.DB, h.Redis, "admin.chapters.write"), h.AdminUpdateChapter)
	admin.Delete("/chapter/:id", middleware.RequirePermission(h.DB, h.Redis, "admin.chapters.write"), h.AdminDeleteChapter)
	admin.Post("/chapter/:id/upload", middleware.RequirePermission(h.DB, h.Redis, "admin.chapters.write"), h.UploadChapterPages)

	admin.Get("/genres", middleware.RequirePermission(h.DB, h.Redis, "admin.genres.read"), h.AdminListGenres)
	admin.Post("/genre", middleware.RequirePermission(h.DB, h.Redis, "admin.genres.write"), h.AdminCreateGenre)
	admin.Put("/genre/:id", middleware.RequirePermission(h.DB, h.Redis, "admin.genres.write"), h.AdminUpdateGenre)

	admin.Get("/ads/stats", middleware.RequirePermission(h.DB, h.Redis, "admin.ads.read"), h.AdminGetAdsStats)
	admin.Put("/ads/stat/:date", middleware.RequirePermission(h.DB, h.Redis, "admin.ads.write"), h.AdminUpdateAdsStat)
	admin.Get("/ads/settings", middleware.RequirePermission(h.DB, h.Redis, "admin.ads.read"), h.AdminGetAdsSettings)
	admin.Put("/ads/settings", middleware.RequirePermission(h.DB, h.Redis, "admin.ads.write"), h.AdminUpdateAdsSettings)
	admin.Get("/ads/direct", middleware.RequirePermission(h.DB, h.Redis, "admin.ads.read"), h.AdminListDirectAds)
	admin.Post("/ads/direct", middleware.RequirePermission(h.DB, h.Redis, "admin.ads.write"), h.AdminCreateDirectAd)
	admin.Put("/ads/direct/:id", middleware.RequirePermission(h.DB, h.Redis, "admin.ads.write"), h.AdminUpdateDirectAd)
	admin.Delete("/ads/direct/:id", middleware.RequirePermission(h.DB, h.Redis, "admin.ads.write"), h.AdminDeleteDirectAd)

	admin.Post("/reward/distribute", middleware.RequirePermission(h.DB, h.Redis, "admin.rewards.distribute"), h.AdminDistributeRewards)

	admin.Get("/permissions", h.AdminListPermissions)
	admin.Put("/permission", middleware.RequireOwner(), h.AdminUpdatePermission)

	admin.Get("/logs", middleware.RequirePermission(h.DB, h.Redis, "admin.stats.read"), h.AdminListLogs)
}

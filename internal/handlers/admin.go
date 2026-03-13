package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"truyenm/backend/internal/crawler"
	"truyenm/backend/internal/models"
	"truyenm/backend/internal/storage"
	"truyenm/backend/internal/utils"
)

// ─── Request types ────────────────────────────────────────────────────────────

type AdminUserActionRequest struct {
	UserID uint `json:"user_id"`
}

type AdminBanRequest struct {
	UserID    uint   `json:"user_id"`
	Reason    string `json:"reason"`
	ExpiresAt string `json:"expires_at"` // optional RFC3339
}

type AdminChangeRoleRequest struct {
	UserID uint   `json:"user_id"`
	Role   string `json:"role"`
}

type AdminManualPointsRequest struct {
	UserID uint   `json:"user_id"`
	Points int    `json:"points"`
	Note   string `json:"note"`
}

type AdminComicRequest struct {
	Title                  string  `json:"title"`
	Author                 string  `json:"author"`
	Description            string  `json:"description"`
	Cover                  string  `json:"cover"`
	ContentType            string  `json:"content_type"`
	Status                 string  `json:"status"`
	GenreIDs               []uint  `json:"genre_ids"`
	CrawlerEnabled         *bool   `json:"crawler_enabled"`
	CrawlerMode            *int    `json:"crawler_mode"`
	CrawlerSourceURL       *string `json:"crawler_source_url"`
	CrawlerIntervalMinutes *int    `json:"crawler_interval_minutes"`
	CrawlerWeekdays        []int   `json:"crawler_weekdays"`
}

type AdminChapterRequest struct {
	ChapterNumber int      `json:"chapter_number"`
	Title         string   `json:"title"`
	ContentURL    string   `json:"content_url"`
	PageCount     int      `json:"page_count"`
	PageURLs      []string `json:"page_urls"`
}

// ─── Stats ────────────────────────────────────────────────────────────────────

func (h *Handler) AdminStats(c *fiber.Ctx) error {
	start, end := utils.DayBoundsUTC(time.Now())

	var totalUsers int64
	if err := h.DB.Model(&models.User{}).Count(&totalUsers).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	var dailyUsers int64
	if err := h.DB.Model(&models.ReadingHistory{}).
		Distinct("user_id").
		Where("read_at >= ? AND read_at < ?", start, end).
		Count(&dailyUsers).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	var chaptersRead int64
	if err := h.DB.Model(&models.ReadingHistory{}).
		Where("read_at >= ? AND read_at < ?", start, end).
		Count(&chaptersRead).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	var pointsDistributed int64
	if err := h.DB.Model(&models.PointsLog{}).
		Select("COALESCE(SUM(points),0)").
		Where("created_at >= ? AND created_at < ?", start, end).
		Scan(&pointsDistributed).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	var impressions int64
	if err := h.DB.Model(&models.AdsStat{}).
		Select("COALESCE(SUM(impressions),0)").
		Where("date >= ? AND date < ?", start, end).
		Scan(&impressions).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	var totalComics int64
	h.DB.Model(&models.Comic{}).Count(&totalComics)

	var openTickets int64
	h.DB.Model(&models.SupportTicket{}).Where("status IN ?", []string{"open", "pending"}).Count(&openTickets)

	return c.JSON(fiber.Map{
		"users":              totalUsers,
		"daily_users":        dailyUsers,
		"chapters_read":      chaptersRead,
		"ads_impressions":    impressions,
		"points_distributed": pointsDistributed,
		"total_comics":       totalComics,
		"open_tickets":       openTickets,
	})
}

// ─── Analytics (range) ───────────────────────────────────────────────────────

type DailyPoint struct {
	Date  string `json:"date"`
	Value int64  `json:"value"`
}

func (h *Handler) AdminAnalytics(c *fiber.Ctx) error {
	rangeStr := c.Query("range", "7d")
	startStr := c.Query("start")
	endStr := c.Query("end")

	var since time.Time
	var until time.Time
	if startStr != "" && endStr != "" {
		start, err := time.Parse("2006-01-02", startStr)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid start date")
		}
		end, err := time.Parse("2006-01-02", endStr)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid end date")
		}
		since = start.UTC()
		until = end.AddDate(0, 0, 1).UTC()
	} else {
		var days int
		switch rangeStr {
		case "30d":
			days = 30
		case "90d":
			days = 90
		default:
			days = 7
		}
		since = time.Now().UTC().AddDate(0, 0, -days)
		until = time.Now().UTC().AddDate(0, 0, 1)
	}

	// Daily active users
	type DailyCount struct {
		Day   string `json:"day"`
		Count int64  `json:"count"`
	}
	var dau []DailyCount
	h.DB.Raw(`
		SELECT TO_CHAR(read_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') as day,
		       COUNT(DISTINCT user_id) as count
		FROM reading_history
		WHERE read_at >= ? AND read_at < ?
		GROUP BY day ORDER BY day
	`, since, until).Scan(&dau)

	// Daily chapters read
	var dailyChapters []DailyCount
	h.DB.Raw(`
		SELECT TO_CHAR(read_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') as day,
		       COUNT(*) as count
		FROM reading_history
		WHERE read_at >= ? AND read_at < ?
		GROUP BY day ORDER BY day
	`, since, until).Scan(&dailyChapters)

	// Top comics by views
	type TopComic struct {
		ID    uint   `json:"id"`
		Title string `json:"title"`
		Views int    `json:"views"`
	}
	var topComics []TopComic
	h.DB.Model(&models.Comic{}).
		Select("id, title, views").
		Order("views desc").
		Limit(10).
		Scan(&topComics)

	// Daily points distributed
	var dailyPoints []DailyCount
	h.DB.Raw(`
		SELECT TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') as day,
		       COALESCE(SUM(points),0) as count
		FROM points_log
		WHERE created_at >= ? AND created_at < ?
		GROUP BY day ORDER BY day
	`, since, until).Scan(&dailyPoints)

	return c.JSON(fiber.Map{
		"range":          rangeStr,
		"start":          since.Format("2006-01-02"),
		"end":            until.AddDate(0, 0, -1).Format("2006-01-02"),
		"daily_users":    dau,
		"daily_chapters": dailyChapters,
		"top_comics":     topComics,
		"daily_points":   dailyPoints,
	})
}

// ─── Users ────────────────────────────────────────────────────────────────────

func (h *Handler) AdminListUsers(c *fiber.Ctx) error {
	page, limit := utils.ParsePagination(c, 1, 20)
	query := c.Query("q")
	role := c.Query("role")
	banFilter := c.Query("banned")

	var users []models.User
	var total int64
	offset := (page - 1) * limit

	q := h.DB.Model(&models.User{})
	if query != "" {
		like := "%" + query + "%"
		q = q.Where("username ILIKE ? OR email ILIKE ?", like, like)
	}
	if role != "" {
		q = q.Where("role = ?", role)
	}
	if banFilter == "true" {
		q = q.Where("is_banned = true")
	} else if banFilter == "false" {
		q = q.Where("is_banned = false")
	}

	q.Count(&total)
	if err := q.Limit(limit).Offset(offset).Order("id desc").Find(&users).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{
		"page":  page,
		"limit": limit,
		"total": total,
		"data":  users,
	})
}

func (h *Handler) AdminGetUser(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.ErrBadRequest
	}

	var user models.User
	if err := h.DB.First(&user, id).Error; err != nil {
		return fiber.ErrNotFound
	}

	var pointsTotal int64
	h.DB.Model(&models.PointsLog{}).
		Select("COALESCE(SUM(points),0)").
		Where("user_id = ?", id).Scan(&pointsTotal)

	var chaptersRead int64
	h.DB.Model(&models.ReadingHistory{}).Where("user_id = ?", id).Count(&chaptersRead)

	return c.JSON(fiber.Map{
		"user":          user,
		"total_points":  pointsTotal,
		"chapters_read": chaptersRead,
	})
}

func (h *Handler) AdminBanUser(c *fiber.Ctx) error {
	var req AdminBanRequest
	if err := c.BodyParser(&req); err != nil || req.UserID == 0 {
		return fiber.ErrBadRequest
	}

	now := time.Now().UTC()
	updates := map[string]any{
		"is_banned":  true,
		"banned_at":  &now,
		"ban_reason": req.Reason,
	}
	if req.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, req.ExpiresAt); err == nil {
			updates["ban_expires_at"] = &t
		}
	}

	if err := h.DB.Model(&models.User{}).Where("id = ?", req.UserID).
		Updates(updates).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	h.logActivity(c, fmt.Sprintf("ban_user:%d", req.UserID), req.Reason)
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) AdminUnbanUser(c *fiber.Ctx) error {
	var req AdminUserActionRequest
	if err := c.BodyParser(&req); err != nil || req.UserID == 0 {
		return fiber.ErrBadRequest
	}

	if err := h.DB.Model(&models.User{}).Where("id = ?", req.UserID).
		Updates(map[string]any{
			"is_banned":      false,
			"banned_at":      nil,
			"ban_reason":     "",
			"ban_expires_at": nil,
		}).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	h.logActivity(c, fmt.Sprintf("unban_user:%d", req.UserID), "")
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) AdminChangeUserRole(c *fiber.Ctx) error {
	var req AdminChangeRoleRequest
	if err := c.BodyParser(&req); err != nil || req.UserID == 0 {
		return fiber.ErrBadRequest
	}

	allowed := map[string]bool{
		models.RoleUser:      true,
		models.RoleAdmin:     true,
		models.RoleStaff:     true,
		models.RoleModerator: true,
	}
	if !allowed[req.Role] {
		return fiber.NewError(fiber.StatusBadRequest, "invalid role")
	}

	// Only owner can set admin role
	callerRole, _ := c.Locals("role").(string)
	if req.Role == models.RoleAdmin && callerRole != models.RoleOwner {
		return fiber.ErrForbidden
	}

	if err := h.DB.Model(&models.User{}).Where("id = ? AND role != ?", req.UserID, models.RoleOwner).
		Update("role", req.Role).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	h.logActivity(c, fmt.Sprintf("change_role:%d", req.UserID), req.Role)
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) AdminResetPoints(c *fiber.Ctx) error {
	var req AdminUserActionRequest
	if err := c.BodyParser(&req); err != nil || req.UserID == 0 {
		return fiber.ErrBadRequest
	}

	if err := h.DB.Model(&models.User{}).Where("id = ?", req.UserID).
		Update("points", 0).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	h.logActivity(c, fmt.Sprintf("reset_points:%d", req.UserID), "")
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) AdminManualPoints(c *fiber.Ctx) error {
	var req AdminManualPointsRequest
	if err := c.BodyParser(&req); err != nil || req.UserID == 0 || req.Points == 0 {
		return fiber.ErrBadRequest
	}

	if err := h.DB.Model(&models.User{}).Where("id = ?", req.UserID).
		UpdateColumn("points", gorm.Expr("points + ?", req.Points)).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	pl := models.PointsLog{
		UserID:    req.UserID,
		Points:    req.Points,
		Reason:    models.PointReasonManual,
		CreatedAt: time.Now().UTC(),
	}
	h.DB.Create(&pl)

	h.logActivity(c, fmt.Sprintf("manual_points:%d", req.UserID), fmt.Sprintf("%d pts – %s", req.Points, req.Note))
	return c.JSON(fiber.Map{"ok": true})
}

// ─── Comics ───────────────────────────────────────────────────────────────────

func (h *Handler) AdminListComics(c *fiber.Ctx) error {
	page, limit := utils.ParsePagination(c, 1, 20)
	q := c.Query("q")
	status := c.Query("status")
	offset := (page - 1) * limit

	db := h.DB.Model(&models.Comic{}).Preload("Genres")
	var total int64

	if q != "" {
		like := "%" + q + "%"
		db = db.Where("title ILIKE ? OR author ILIKE ?", like, like)
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}

	db.Count(&total)
	var comics []models.Comic
	if err := db.Limit(limit).Offset(offset).Order("id desc").Find(&comics).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{"page": page, "limit": limit, "total": total, "data": comics})
}

func (h *Handler) AdminCreateComic(c *fiber.Ctx) error {
	var req AdminComicRequest
	if err := c.BodyParser(&req); err != nil || req.Title == "" {
		return fiber.ErrBadRequest
	}

	slug, err := ensureUniqueComicSlug(h.DB, utils.Slugify(req.Title), 0)
	if err != nil {
		return fiber.ErrInternalServerError
	}

	comic := models.Comic{
		Title:       req.Title,
		Slug:        slug,
		Author:      req.Author,
		Description: req.Description,
		Cover:       req.Cover,
		ContentType: req.ContentType,
		Status:      req.Status,
	}
	if comic.ContentType == "" {
		comic.ContentType = "comic"
	}
	if comic.Status == "" {
		comic.Status = "ongoing"
	}
	if req.CrawlerEnabled != nil {
		comic.CrawlerEnabled = *req.CrawlerEnabled
	}
	if req.CrawlerMode != nil {
		comic.CrawlerMode = normalizeCrawlerMode(*req.CrawlerMode)
	}
	if req.CrawlerSourceURL != nil {
		baseURL, targetChapter := utils.NormalizeCrawlerSourceURL(*req.CrawlerSourceURL)
		comic.CrawlerSourceURL = baseURL
		comic.CrawlerTargetChapter = maxInt(targetChapter, 0)
	}
	if req.CrawlerIntervalMinutes != nil {
		comic.CrawlerIntervalMinutes = maxInt(*req.CrawlerIntervalMinutes, 0)
	}
	if req.CrawlerWeekdays != nil {
		comic.CrawlerWeekdays = utils.WeekdaysToMask(req.CrawlerWeekdays)
	}

	if err := h.DB.Create(&comic).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	if len(req.GenreIDs) > 0 {
		var genres []models.Genre
		h.DB.Where("id IN ?", req.GenreIDs).Find(&genres)
		h.DB.Model(&comic).Association("Genres").Replace(genres)
	}

	h.logActivity(c, fmt.Sprintf("create_comic:%d", comic.ID), comic.Title)

	if comic.CrawlerEnabled && comic.CrawlerSourceURL != "" && comic.CrawlerMode == models.CrawlerModeFullSync {
		if service, err := crawler.NewService(h.Cfg, h.DB, h.Logger); err == nil {
			var triggeredBy *uint
			if v := c.Locals("user_id"); v != nil {
				if uid, ok := v.(uint); ok {
					triggeredBy = &uid
				}
			}
			go func() {
				_ = service.RunComic(context.Background(), comic, "create", triggeredBy)
			}()
		}
	}

	return c.Status(fiber.StatusCreated).JSON(comic)
}

func (h *Handler) AdminUpdateComic(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.ErrBadRequest
	}

	var req AdminComicRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}

	updates := map[string]any{}
	if req.Title != "" {
		updates["title"] = req.Title
		if slug, err := ensureUniqueComicSlug(h.DB, utils.Slugify(req.Title), uint(id)); err == nil {
			updates["slug"] = slug
		} else {
			return fiber.ErrInternalServerError
		}
	}
	if req.Author != "" {
		updates["author"] = req.Author
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Cover != "" {
		updates["cover"] = req.Cover
	}
	if req.ContentType != "" {
		updates["content_type"] = req.ContentType
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.CrawlerEnabled != nil {
		updates["crawler_enabled"] = *req.CrawlerEnabled
	}
	if req.CrawlerMode != nil {
		updates["crawler_mode"] = normalizeCrawlerMode(*req.CrawlerMode)
	}
	if req.CrawlerSourceURL != nil {
		baseURL, targetChapter := utils.NormalizeCrawlerSourceURL(*req.CrawlerSourceURL)
		updates["crawler_source_url"] = baseURL
		updates["crawler_target_chapter"] = maxInt(targetChapter, 0)
	}
	if req.CrawlerIntervalMinutes != nil {
		updates["crawler_interval_minutes"] = maxInt(*req.CrawlerIntervalMinutes, 0)
	}
	if req.CrawlerWeekdays != nil {
		updates["crawler_weekdays"] = utils.WeekdaysToMask(req.CrawlerWeekdays)
	}

	if len(updates) == 0 && len(req.GenreIDs) == 0 {
		return fiber.ErrBadRequest
	}

	if len(updates) > 0 {
		if err := h.DB.Model(&models.Comic{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return fiber.ErrInternalServerError
		}
	}

	if len(req.GenreIDs) > 0 {
		var genres []models.Genre
		h.DB.Where("id IN ?", req.GenreIDs).Find(&genres)
		comic := models.Comic{}
		comic.ID = uint(id)
		h.DB.Model(&comic).Association("Genres").Replace(genres)
	}

	h.logActivity(c, fmt.Sprintf("update_comic:%d", id), "")
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) AdminDeleteComic(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.ErrBadRequest
	}

	if err := h.DB.Delete(&models.Comic{}, id).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	h.logActivity(c, fmt.Sprintf("delete_comic:%d", id), "")
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) AdminRunCrawlerNow(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.ErrBadRequest
	}

	var comic models.Comic
	if err := h.DB.Select("id", "crawler_source_url").First(&comic, id).Error; err != nil {
		return fiber.ErrNotFound
	}
	if comic.CrawlerSourceURL == "" {
		return fiber.NewError(fiber.StatusBadRequest, "crawler_source_url is required")
	}

	service, err := crawler.NewService(h.Cfg, h.DB, h.Logger)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "crawler not configured")
	}

	var triggeredBy *uint
	if v := c.Locals("user_id"); v != nil {
		if uid, ok := v.(uint); ok {
			triggeredBy = &uid
		}
	}

	go func() {
		_ = service.RunComicByID(context.Background(), uint(id), "manual", triggeredBy)
	}()

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"ok": true, "status": "queued"})
}

func (h *Handler) AdminListCrawlerLogs(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.ErrBadRequest
	}

	page, limit := utils.ParsePagination(c, 1, 10)
	offset := (page - 1) * limit

	var total int64
	if err := h.DB.Model(&models.CrawlerLog{}).Where("comic_id = ?", id).Count(&total).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	var logs []models.CrawlerLog
	if err := h.DB.Where("comic_id = ?", id).
		Order("started_at desc").
		Limit(limit).Offset(offset).
		Find(&logs).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{
		"page":  page,
		"limit": limit,
		"total": total,
		"data":  logs,
	})
}

func normalizeCrawlerMode(value int) int {
	if value < models.CrawlerModeFullSync || value > models.CrawlerModeSchedule {
		return models.CrawlerModeFullSync
	}
	return value
}

func maxInt(v int, fallback int) int {
	if v < fallback {
		return fallback
	}
	return v
}

func ensureUniqueComicSlug(db *gorm.DB, base string, excludeID uint) (string, error) {
	slug := strings.Trim(base, "-")
	if slug == "" {
		slug = fmt.Sprintf("truyen-%d", time.Now().Unix())
	}

	for i := 0; i < 50; i++ {
		candidate := slug
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", slug, i)
		}
		var count int64
		query := db.Model(&models.Comic{}).Where("slug = ?", candidate)
		if excludeID > 0 {
			query = query.Where("id <> ?", excludeID)
		}
		if err := query.Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique slug")
}

func isCloudinaryURL(raw string, cloudName string) bool {
	if raw == "" || cloudName == "" {
		return false
	}
	needle := fmt.Sprintf("res.cloudinary.com/%s/", cloudName)
	return strings.Contains(raw, needle)
}

// ─── Chapters ─────────────────────────────────────────────────────────────────

func (h *Handler) AdminCreateChapter(c *fiber.Ctx) error {
	comicID, err := c.ParamsInt("id")
	if err != nil || comicID <= 0 {
		return fiber.ErrBadRequest
	}

	var req AdminChapterRequest
	if err := c.BodyParser(&req); err != nil || req.ChapterNumber <= 0 {
		return fiber.ErrBadRequest
	}

	chapter := models.Chapter{
		ComicID:       uint(comicID),
		ChapterNumber: req.ChapterNumber,
		Title:         req.Title,
	}

	if len(req.PageURLs) > 0 {
		return fiber.NewError(fiber.StatusBadRequest, "page_urls is not supported, upload images instead")
	}
	if req.ContentURL != "" && !isCloudinaryURL(req.ContentURL, h.Cfg.CloudinaryCloudName) {
		return fiber.NewError(fiber.StatusBadRequest, "content_url must be a cloudinary url")
	}
	chapter.ContentURL = req.ContentURL
	chapter.PageCount = req.PageCount

	if err := h.DB.Create(&chapter).Error; err != nil {
		return fiber.NewError(fiber.StatusConflict, "chapter number already exists")
	}

	h.logActivity(c, fmt.Sprintf("create_chapter:%d", chapter.ID), fmt.Sprintf("comic %d ch%d", comicID, req.ChapterNumber))
	return c.Status(fiber.StatusCreated).JSON(chapter)
}

func (h *Handler) AdminUpdateChapter(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.ErrBadRequest
	}

	var req AdminChapterRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}

	updates := map[string]any{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.ContentURL != "" {
		if !isCloudinaryURL(req.ContentURL, h.Cfg.CloudinaryCloudName) {
			return fiber.NewError(fiber.StatusBadRequest, "content_url must be a cloudinary url")
		}
		updates["content_url"] = req.ContentURL
	}
	if req.PageCount > 0 {
		updates["page_count"] = req.PageCount
	}

	if len(req.PageURLs) > 0 {
		return fiber.NewError(fiber.StatusBadRequest, "page_urls is not supported, upload images instead")
	}

	if len(updates) == 0 {
		return fiber.ErrBadRequest
	}

	if err := h.DB.Model(&models.Chapter{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) AdminDeleteChapter(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.ErrBadRequest
	}

	var chapter models.Chapter
	if err := h.DB.First(&chapter, id).Error; err != nil {
		return fiber.ErrNotFound
	}

	if err := h.DB.Delete(&models.Chapter{}, id).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	if chapter.PageCount > 0 {
		if client, err := storage.NewCloudinary(h.Cfg); err == nil {
			for i := 1; i <= chapter.PageCount; i++ {
				publicID := fmt.Sprintf("truyenm/comics/%d/chapter_%d/page_%d", chapter.ComicID, chapter.ChapterNumber, i)
				_ = client.Destroy(context.Background(), publicID)
			}
		}
	}

	return c.JSON(fiber.Map{"ok": true})
}

// ─── Ads ──────────────────────────────────────────────────────────────────────

func (h *Handler) AdminGetAdsStats(c *fiber.Ctx) error {
	rangeStr := c.Query("range", "7d")
	startStr := c.Query("start")
	endStr := c.Query("end")

	var since time.Time
	var until time.Time
	if startStr != "" && endStr != "" {
		start, err := time.Parse("2006-01-02", startStr)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid start date")
		}
		end, err := time.Parse("2006-01-02", endStr)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid end date")
		}
		since = start.UTC()
		until = end.AddDate(0, 0, 1).UTC()
	} else {
		var days int
		switch rangeStr {
		case "30d":
			days = 30
		case "90d":
			days = 90
		default:
			days = 7
		}
		since = time.Now().UTC().AddDate(0, 0, -days)
		until = time.Now().UTC().AddDate(0, 0, 1)
	}

	var stats []models.AdsStat
	if err := h.DB.Where("date >= ? AND date < ?", since, until).Order("date asc").Find(&stats).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{
		"range": rangeStr,
		"start": since.Format("2006-01-02"),
		"end":   until.AddDate(0, 0, -1).Format("2006-01-02"),
		"data":  stats,
	})
}

type AdminUpdateAdsStatRequest struct {
	Impressions int     `json:"impressions"`
	Revenue     float64 `json:"revenue"`
}

func (h *Handler) AdminUpdateAdsStat(c *fiber.Ctx) error {
	dateStr := c.Params("date")
	if dateStr == "" {
		return fiber.ErrBadRequest
	}
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid date")
	}

	var req AdminUpdateAdsStatRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}

	stat := models.AdsStat{
		Date:        date.UTC(),
		Impressions: req.Impressions,
		Revenue:     req.Revenue,
	}

	if err := h.DB.Where("date = ?", stat.Date).
		Assign(models.AdsStat{Impressions: req.Impressions, Revenue: req.Revenue}).
		FirstOrCreate(&stat).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	h.logActivity(c, "update_ads_stat", dateStr)
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) AdminDistributeRewards(c *fiber.Ctx) error {
	// Distribute 5 bonus points to all users who read at least 1 chapter today
	start, end := utils.DayBoundsUTC(time.Now())

	type UserReward struct {
		UserID uint
	}
	var recipients []UserReward
	h.DB.Raw(`
		SELECT DISTINCT user_id FROM reading_history
		WHERE read_at >= ? AND read_at < ?
	`, start, end).Scan(&recipients)

	now := time.Now().UTC()
	for _, r := range recipients {
		h.DB.Model(&models.User{}).Where("id = ?", r.UserID).
			UpdateColumn("points", gorm.Expr("points + 5"))
		h.DB.Create(&models.PointsLog{
			UserID:    r.UserID,
			Points:    5,
			Reason:    models.PointReasonDailyLogin,
			CreatedAt: now,
		})
	}

	h.logActivity(c, "distribute_rewards", fmt.Sprintf("%d users rewarded", len(recipients)))
	return c.JSON(fiber.Map{"rewarded": len(recipients)})
}

// ─── Permissions ─────────────────────────────────────────────────────────────

func (h *Handler) AdminListPermissions(c *fiber.Ctx) error {
	role := c.Query("role")
	callerRole, _ := c.Locals("role").(string)
	if role == "" {
		if callerRole != models.RoleOwner {
			role = callerRole
		}
	}
	q := h.DB.Model(&models.RolePermission{})
	if role != "" {
		q = q.Where("role = ?", role)
	}
	var perms []models.RolePermission
	if err := q.Find(&perms).Error; err != nil {
		return fiber.ErrInternalServerError
	}
	return c.JSON(fiber.Map{"data": perms})
}

type AdminUpdatePermissionRequest struct {
	Role       string `json:"role"`
	Permission string `json:"permission"`
	Granted    bool   `json:"granted"`
}

func (h *Handler) AdminUpdatePermission(c *fiber.Ctx) error {
	var req AdminUpdatePermissionRequest
	if err := c.BodyParser(&req); err != nil || req.Role == "" || req.Permission == "" {
		return fiber.ErrBadRequest
	}
	// Prevent anyone from modifying owner wildcard
	if req.Role == models.RoleOwner {
		return fiber.ErrForbidden
	}

	if req.Granted {
		perm := models.RolePermission{Role: req.Role, Permission: req.Permission}
		if err := h.DB.FirstOrCreate(&perm, perm).Error; err != nil {
			return fiber.ErrInternalServerError
		}
	} else {
		if err := h.DB.Where("role = ? AND permission = ?", req.Role, req.Permission).
			Delete(&models.RolePermission{}).Error; err != nil {
			return fiber.ErrInternalServerError
		}
	}

	// Bust Redis permission cache for this role
	if h.Redis != nil {
		h.Redis.Del(c.Context(), "perms:"+req.Role)
	}

	h.logActivity(c, "update_permission", fmt.Sprintf("%s:%s=%v", req.Role, req.Permission, req.Granted))
	return c.JSON(fiber.Map{"ok": true})
}

// ─── Activity Logs ────────────────────────────────────────────────────────────

func (h *Handler) AdminListLogs(c *fiber.Ctx) error {
	page, limit := utils.ParsePagination(c, 1, 50)
	offset := (page - 1) * limit

	var logs []models.ActivityLog
	var total int64
	h.DB.Model(&models.ActivityLog{}).Count(&total)
	if err := h.DB.Order("id desc").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{"page": page, "limit": limit, "total": total, "data": logs})
}

// ─── Internal helper ─────────────────────────────────────────────────────────

func (h *Handler) logActivity(c *fiber.Ctx, action, detail string) {
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return
	}
	h.DB.Create(&models.ActivityLog{
		UserID:    userID,
		Action:    action,
		Detail:    detail,
		IP:        c.IP(),
		CreatedAt: time.Now().UTC(),
	})
}

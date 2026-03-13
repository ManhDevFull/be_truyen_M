package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"truyenm/backend/internal/models"
	"truyenm/backend/internal/utils"
)

func (h *Handler) ListComics(c *fiber.Ctx) error {
	page, limit := utils.ParsePagination(c, 1, 20)
	q := c.Query("q")
	status := c.Query("status")
	genre := c.Query("genre")
	sortBy := c.Query("sort", "id")

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
	if genre != "" {
		db = db.Joins("JOIN comic_genres ON comic_genres.comic_id = comics.id").
			Joins("JOIN genres ON genres.id = comic_genres.genre_id AND genres.slug = ? AND genres.is_active = true", genre)
	}

	db.Count(&total)

	orderMap := map[string]string{
		"id":      "id desc",
		"title":   "title asc",
		"views":   "views desc",
		"updated": "updated_at desc",
	}
	order := orderMap[sortBy]
	if order == "" {
		order = "id desc"
	}

	var comics []models.Comic
	if err := db.Limit(limit).Offset(offset).Order(order).Find(&comics).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{
		"page":  page,
		"limit": limit,
		"total": total,
		"data":  comics,
	})
}

func (h *Handler) GetComic(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.ErrBadRequest
	}

	var comic models.Comic
	if err := h.DB.Preload("Genres").First(&comic, id).Error; err != nil {
		return fiber.ErrNotFound
	}

	var chapters []models.Chapter
	if err := h.DB.Where("comic_id = ?", comic.ID).Order("chapter_number asc").Find(&chapters).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	isFollowed := false
	if v := c.Locals("user_id"); v != nil {
		if userID, ok := v.(uint); ok {
			var count int64
			if err := h.DB.Model(&models.ComicFollow{}).
				Where("user_id = ? AND comic_id = ?", userID, comic.ID).
				Count(&count).Error; err == nil && count > 0 {
				isFollowed = true
			}
		}
	}

	// Increment view counter via Redis atomic counter (flush daily to DB)
	if h.Redis != nil {
		h.Redis.Incr(c.Context(), "view:comic:"+c.Params("id"))
	}

	return c.JSON(fiber.Map{
		"comic":       comic,
		"chapters":    chapters,
		"is_followed": isFollowed,
	})
}

func (h *Handler) GetChapter(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.ErrBadRequest
	}

	var chapter models.Chapter
	if err := h.DB.First(&chapter, id).Error; err != nil {
		return fiber.ErrNotFound
	}

	// Increment view counter for this chapter
	if h.Redis != nil {
		h.Redis.Incr(c.Context(), "view:chapter:"+c.Params("id"))
	}

	return c.JSON(chapter)
}

func (h *Handler) GetChapterBySlug(c *fiber.Ctx) error {
	slug := c.Params("slug")
	number, err := c.ParamsInt("number")
	if slug == "" || err != nil || number <= 0 {
		return fiber.ErrBadRequest
	}

	var comic models.Comic
	if err := h.DB.Select("id").Where("slug = ?", slug).First(&comic).Error; err != nil {
		return fiber.ErrNotFound
	}

	var chapter models.Chapter
	if err := h.DB.Where("comic_id = ? AND chapter_number = ?", comic.ID, number).First(&chapter).Error; err != nil {
		return fiber.ErrNotFound
	}

	return c.JSON(chapter)
}

func (h *Handler) ListGenres(c *fiber.Ctx) error {
	var genres []models.Genre
	if err := h.DB.Where("is_active = true").Order("name asc").Find(&genres).Error; err != nil {
		return fiber.ErrInternalServerError
	}
	return c.JSON(fiber.Map{"data": genres})
}

func (h *Handler) ListTopReaders(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 5)
	if limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}

	rangeParam := strings.ToLower(c.Query("range", "all"))
	now := time.Now().UTC()
	var start *time.Time
	switch rangeParam {
	case "week":
		startTime := startOfWeekUTC(now)
		start = &startTime
	case "month":
		startTime := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		start = &startTime
	default:
		rangeParam = "all"
	}

	type Leader struct {
		UserID       uint   `json:"user_id"`
		Username     string `json:"username"`
		ChaptersRead int64  `json:"chapters_read"`
	}

	var rows []Leader
	sql := `
		SELECT u.id as user_id, u.username, COUNT(DISTINCT rh.chapter_id) as chapters_read
		FROM reading_history rh
		JOIN users u ON u.id = rh.user_id
	`
	args := []any{}
	if start != nil {
		sql += " WHERE rh.read_at >= ?"
		args = append(args, *start)
	}
	sql += `
		GROUP BY u.id, u.username
		ORDER BY chapters_read DESC, u.id ASC
		LIMIT ?
	`
	args = append(args, limit)

	if err := h.DB.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{"data": rows})
}

func startOfWeekUTC(now time.Time) time.Time {
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := now.AddDate(0, 0, -(weekday - 1))
	return time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
}

func (h *Handler) GetMyProfile(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)
	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		return fiber.ErrNotFound
	}
	return c.JSON(user)
}

func (h *Handler) GetReadingHistory(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)
	page, limit := utils.ParsePagination(c, 1, 20)
	offset := (page - 1) * limit

	type HistoryItem struct {
		models.ReadingHistory
		ComicID       uint   `json:"comic_id"`
		ComicTitle    string `json:"comic_title"`
		ChapterNumber int    `json:"chapter_number"`
		ChapterTitle  string `json:"chapter_title"`
	}

	var items []HistoryItem
	h.DB.Raw(`
		SELECT rh.*, ch.comic_id, c.title as comic_title,
		       ch.chapter_number, ch.title as chapter_title
		FROM reading_history rh
		JOIN chapters ch ON ch.id = rh.chapter_id
		JOIN comics c ON c.id = ch.comic_id
		WHERE rh.user_id = ?
		ORDER BY rh.read_at DESC
		LIMIT ? OFFSET ?
	`, userID, limit, offset).Scan(&items)

	return c.JSON(fiber.Map{"page": page, "limit": limit, "data": items})
}

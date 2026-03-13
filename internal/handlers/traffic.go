package handlers

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"truyenm/backend/internal/models"
	"truyenm/backend/internal/utils"
)

func (h *Handler) TrackView(c *fiber.Ctx) error {
	comicID, err := c.ParamsInt("id")
	if err != nil || comicID <= 0 {
		return fiber.ErrBadRequest
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)

	var chapterIDPtr *uint
	if chID := c.QueryInt("chapter_id", 0); chID > 0 {
		id := uint(chID)
		chapterIDPtr = &id
	} else if chID, err := c.ParamsInt("chapterId"); err == nil && chID > 0 {
		id := uint(chID)
		chapterIDPtr = &id
	}

	// Idempotent view tracking via Redis (per day per device)
	if h.Redis != nil {
		fingerprint := utils.DeviceFingerprint(c.Get("User-Agent"), c.IP())
		chapterKey := 0
		if chapterIDPtr != nil {
			chapterKey = int(*chapterIDPtr)
		}
		key := fmt.Sprintf("view:%d:%d:%s:%s", comicID, chapterKey, fingerprint, today.Format("2006-01-02"))
		set, err := h.Redis.SetNX(c.Context(), key, "1", 24*time.Hour).Result()
		if err == nil && !set {
			return c.JSON(fiber.Map{"ok": true, "deduped": true})
		}
	}

	result := h.DB.Exec(`
		INSERT INTO traffic_logs (comic_id, chapter_id, date, view_count)
		VALUES (?, ?, ?, 1)
		ON CONFLICT (comic_id, chapter_id, date)
		DO UPDATE SET view_count = traffic_logs.view_count + 1
	`, uint(comicID), chapterIDPtr, today)

	if result.Error != nil {
		return fiber.ErrInternalServerError
	}

	// Also update comic.views column
	h.DB.Model(&models.Comic{}).Where("id = ?", comicID).
		UpdateColumn("views", gorm.Expr("views + 1"))

	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) GetTrafficStats(c *fiber.Ctx) error {
	rangeStr := c.Query("range", "7d")
	comicID := c.QueryInt("comic_id", 0)

	var days int
	switch rangeStr {
	case "30d":
		days = 30
	case "all":
		days = 3650
	default:
		days = 7
	}

	since := time.Now().UTC().AddDate(0, 0, -days)

	type DailyStat struct {
		Date      string `json:"date"`
		ViewCount int64  `json:"view_count"`
	}

	q := h.DB.Model(&models.TrafficLog{}).
		Select("TO_CHAR(date, 'YYYY-MM-DD') as date, SUM(view_count) as view_count").
		Where("date >= ?", since).
		Where("chapter_id IS NULL"). // comic-level only
		Group("date").
		Order("date asc")

	if comicID > 0 {
		q = q.Where("comic_id = ?", comicID)
	}

	var dailyStats []DailyStat
	q.Scan(&dailyStats)

	// Top comics by total views in range
	type TopComic struct {
		ID        uint   `json:"id"`
		Title     string `json:"title"`
		ViewCount int64  `json:"view_count"`
	}
	var topComics []TopComic
	h.DB.Raw(`
		SELECT tl.comic_id as id, c.title, SUM(tl.view_count) as view_count
		FROM traffic_logs tl
		JOIN comics c ON c.id = tl.comic_id
		WHERE tl.date >= ? AND tl.chapter_id IS NULL
		GROUP BY tl.comic_id, c.title
		ORDER BY view_count DESC
		LIMIT 10
	`, since).Scan(&topComics)

	return c.JSON(fiber.Map{
		"range":      rangeStr,
		"daily":      dailyStats,
		"top_comics": topComics,
	})
}

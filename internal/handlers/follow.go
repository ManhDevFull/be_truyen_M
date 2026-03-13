package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm/clause"

	"truyenm/backend/internal/models"
)

type FollowUpdate struct {
	ComicID       uint      `json:"comic_id"`
	ComicTitle    string    `json:"comic_title"`
	ComicSlug     string    `json:"comic_slug,omitempty"`
	ComicCover    string    `json:"comic_cover,omitempty"`
	ChapterID     uint      `json:"chapter_id"`
	ChapterNumber int       `json:"chapter_number"`
	ChapterTitle  string    `json:"chapter_title,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

func (h *Handler) FollowComic(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.ErrBadRequest
	}

	now := time.Now().UTC()
	follow := models.ComicFollow{UserID: userID, ComicID: uint(id), LastNotifiedAt: &now}
	if err := h.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&follow).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{"followed": true})
}

func (h *Handler) UnfollowComic(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.ErrBadRequest
	}

	if err := h.DB.Where("user_id = ? AND comic_id = ?", userID, uint(id)).Delete(&models.ComicFollow{}).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{"followed": false})
}

func (h *Handler) ListFollows(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)
	var follows []models.ComicFollow
	if err := h.DB.Preload("Comic").Where("user_id = ?", userID).Order("created_at desc").Find(&follows).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{"data": follows})
}

func (h *Handler) ListFollowUpdates(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)
	sinceParam := c.Query("since")
	var since time.Time
	if sinceParam == "" {
		since = time.Now().Add(-24 * time.Hour)
	} else {
		parsed, err := time.Parse(time.RFC3339, sinceParam)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid since")
		}
		since = parsed
	}

	var updates []FollowUpdate
	if err := h.DB.Raw(`
		SELECT ch.id AS chapter_id,
		       ch.chapter_number,
		       ch.title AS chapter_title,
		       ch.created_at,
		       c.id AS comic_id,
		       c.title AS comic_title,
		       c.slug AS comic_slug,
		       c.cover AS comic_cover
		FROM chapters ch
		JOIN comics c ON c.id = ch.comic_id
		JOIN comic_follows f ON f.comic_id = ch.comic_id
		WHERE f.user_id = ? AND ch.created_at > ?
		ORDER BY ch.created_at DESC
		LIMIT 50
	`, userID, since).Scan(&updates).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{
		"data":  updates,
		"since": since.Format(time.RFC3339),
	})
}

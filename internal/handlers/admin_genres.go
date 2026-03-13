package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"truyenm/backend/internal/models"
	"truyenm/backend/internal/utils"
)

type AdminGenreRequest struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	IsActive *bool  `json:"is_active"`
}

type AdminGenreItem struct {
	ID         uint      `json:"id"`
	Name       string    `json:"name"`
	Slug       string    `json:"slug"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ComicCount int64     `json:"comic_count"`
}

func (h *Handler) AdminListGenres(c *fiber.Ctx) error {
	page, limit := utils.ParsePagination(c, 1, 20)
	offset := (page - 1) * limit
	q := strings.TrimSpace(c.Query("q"))
	status := strings.ToLower(strings.TrimSpace(c.Query("status", "all")))

	countDB := h.DB.Model(&models.Genre{})
	if q != "" {
		like := "%" + q + "%"
		countDB = countDB.Where("name ILIKE ? OR slug ILIKE ?", like, like)
	}
	if status == "active" {
		countDB = countDB.Where("is_active = true")
	} else if status == "inactive" {
		countDB = countDB.Where("is_active = false")
	}

	var total int64
	if err := countDB.Count(&total).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	query := h.DB.Table("genres").
		Select("genres.id, genres.name, genres.slug, genres.is_active, genres.created_at, genres.updated_at, COUNT(comic_genres.comic_id) as comic_count").
		Joins("LEFT JOIN comic_genres ON comic_genres.genre_id = genres.id")
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("genres.name ILIKE ? OR genres.slug ILIKE ?", like, like)
	}
	if status == "active" {
		query = query.Where("genres.is_active = true")
	} else if status == "inactive" {
		query = query.Where("genres.is_active = false")
	}

	var rows []AdminGenreItem
	if err := query.
		Group("genres.id, genres.name, genres.slug, genres.is_active, genres.created_at, genres.updated_at").
		Order("genres.name asc").
		Limit(limit).Offset(offset).
		Scan(&rows).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{
		"page":  page,
		"limit": limit,
		"total": total,
		"data":  rows,
	})
}

func (h *Handler) AdminCreateGenre(c *fiber.Ctx) error {
	var req AdminGenreRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return fiber.ErrBadRequest
	}

	baseSlug := utils.Slugify(strings.TrimSpace(req.Slug))
	if baseSlug == "" {
		baseSlug = utils.Slugify(name)
	}
	if baseSlug == "" {
		baseSlug = fmt.Sprintf("the-loai-%d", time.Now().Unix())
	}
	uniqueSlug, err := ensureUniqueGenreSlug(h.DB, baseSlug, 0)
	if err != nil {
		return fiber.ErrInternalServerError
	}

	genre := models.Genre{
		Name:     name,
		Slug:     uniqueSlug,
		IsActive: true,
	}
	if req.IsActive != nil {
		genre.IsActive = *req.IsActive
	}

	if err := h.DB.Create(&genre).Error; err != nil {
		return fiber.NewError(fiber.StatusConflict, "genre already exists")
	}

	h.logActivity(c, "create_genre", fmt.Sprintf("genre %d", genre.ID))
	return c.Status(fiber.StatusCreated).JSON(genre)
}

func (h *Handler) AdminUpdateGenre(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.ErrBadRequest
	}

	var genre models.Genre
	if err := h.DB.First(&genre, id).Error; err != nil {
		return fiber.ErrNotFound
	}

	var req AdminGenreRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}

	updates := map[string]any{}
	if name := strings.TrimSpace(req.Name); name != "" {
		updates["name"] = name
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if slugValue := strings.TrimSpace(req.Slug); slugValue != "" {
		base := utils.Slugify(slugValue)
		if base == "" {
			base = fmt.Sprintf("the-loai-%d", time.Now().Unix())
		}
		uniqueSlug, err := ensureUniqueGenreSlug(h.DB, base, genre.ID)
		if err != nil {
			return fiber.ErrInternalServerError
		}
		updates["slug"] = uniqueSlug
	}

	if len(updates) == 0 {
		return c.JSON(genre)
	}

	if err := h.DB.Model(&genre).Updates(updates).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	if err := h.DB.First(&genre, id).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	h.logActivity(c, "update_genre", fmt.Sprintf("genre %d", genre.ID))
	return c.JSON(genre)
}

func ensureUniqueGenreSlug(db *gorm.DB, base string, excludeID uint) (string, error) {
	slug := strings.Trim(base, "-")
	if slug == "" {
		slug = fmt.Sprintf("the-loai-%d", time.Now().Unix())
	}

	for i := 0; i < 50; i++ {
		candidate := slug
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", slug, i)
		}
		var count int64
		query := db.Model(&models.Genre{}).Where("slug = ?", candidate)
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

	return "", fmt.Errorf("failed to generate unique genre slug")
}

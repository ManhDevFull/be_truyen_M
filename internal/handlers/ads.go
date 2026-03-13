package handlers

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"truyenm/backend/internal/models"
)

type AdsCheckRequest struct {
	Adblock bool `json:"adblock"`
}

func (h *Handler) AdsCheck(c *fiber.Ctx) error {
	var req AdsCheckRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}

	userID := c.Locals("user_id").(uint)

	var status models.UserAdblock
	err := h.DB.Where("user_id = ?", userID).First(&status).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.ErrInternalServerError
		}
		status = models.UserAdblock{UserID: userID, Adblock: req.Adblock, UpdatedAt: time.Now().UTC()}
		if err := h.DB.Create(&status).Error; err != nil {
			return fiber.ErrInternalServerError
		}
	} else {
		status.Adblock = req.Adblock
		status.UpdatedAt = time.Now().UTC()
		if err := h.DB.Save(&status).Error; err != nil {
			return fiber.ErrInternalServerError
		}
	}

	return c.JSON(fiber.Map{"adblock": status.Adblock})
}

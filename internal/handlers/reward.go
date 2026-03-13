package handlers

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"truyenm/backend/internal/models"
	"truyenm/backend/internal/utils"
)

type RewardReadRequest struct {
	ChapterID   uint `json:"chapter_id"`
	ReadingTime int  `json:"reading_time"`
	Adblock     bool `json:"adblock"`
}

func (h *Handler) RewardRead(c *fiber.Ctx) error {
	var req RewardReadRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	if req.ChapterID == 0 || req.ReadingTime < 10 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid reading_time or chapter_id")
	}

	userID := c.Locals("user_id").(uint)

	settings, err := h.getOrCreateAdsSettings()
	if err != nil {
		return fiber.ErrInternalServerError
	}
	if settings.AdsenseEnabled {
		if req.Adblock {
			return fiber.NewError(fiber.StatusForbidden, "adblock detected")
		}
		var status models.UserAdblock
		if err := h.DB.Where("user_id = ?", userID).First(&status).Error; err == nil {
			if status.Adblock {
				return fiber.NewError(fiber.StatusForbidden, "adblock detected")
			}
		}
	}

	start, end := utils.DayBoundsUTC(time.Now())
	var count int64
	if err := h.DB.Model(&models.PointsLog{}).
		Where("user_id = ? AND reason = ? AND created_at >= ? AND created_at < ?", userID, models.PointReasonRead, start, end).
		Count(&count).Error; err != nil {
		return fiber.ErrInternalServerError
	}
	if count >= 30 {
		return fiber.NewError(fiber.StatusTooManyRequests, "daily limit reached")
	}

	err = h.DB.Transaction(func(tx *gorm.DB) error {
		var rh models.ReadingHistory
		err := tx.Where("user_id = ? AND chapter_id = ?", userID, req.ChapterID).First(&rh).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				rh = models.ReadingHistory{
					UserID:      userID,
					ChapterID:   req.ChapterID,
					ReadAt:      time.Now().UTC(),
					ReadingTime: req.ReadingTime,
				}
				if err := tx.Create(&rh).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		} else {
			rh.ReadAt = time.Now().UTC()
			if req.ReadingTime > rh.ReadingTime {
				rh.ReadingTime = req.ReadingTime
			}
			if err := tx.Save(&rh).Error; err != nil {
				return err
			}
		}

		if rh.RewardedAt != nil {
			return fiber.NewError(fiber.StatusConflict, "chapter already rewarded")
		}

		now := time.Now().UTC()
		if err := tx.Model(&rh).Update("rewarded_at", &now).Error; err != nil {
			return err
		}

		if err := tx.Model(&models.User{}).Where("id = ?", userID).
			Update("points", gorm.Expr("points + ?", 1)).Error; err != nil {
			return err
		}

		chapterID := req.ChapterID
		pl := models.PointsLog{
			UserID:    userID,
			ChapterID: &chapterID,
			Points:    1,
			Reason:    models.PointReasonRead,
			CreatedAt: now,
		}
		if err := tx.Create(&pl).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		var fe *fiber.Error
		if errors.As(err, &fe) {
			return fe
		}
		h.Logger.Error("reward failed", zap.Error(err))
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{"points_added": 1})
}

package handlers

import (
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"truyenm/backend/internal/models"
	"truyenm/backend/internal/utils"
)

type AdminAdsSettingsRequest struct {
	AdsenseEnabled *bool `json:"adsense_enabled"`
}

func (h *Handler) AdminGetAdsSettings(c *fiber.Ctx) error {
	settings, err := h.getOrCreateAdsSettings()
	if err != nil {
		return fiber.ErrInternalServerError
	}
	return c.JSON(settings)
}

func (h *Handler) AdminUpdateAdsSettings(c *fiber.Ctx) error {
	var req AdminAdsSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	if req.AdsenseEnabled == nil {
		return fiber.ErrBadRequest
	}

	settings, err := h.getOrCreateAdsSettings()
	if err != nil {
		return fiber.ErrInternalServerError
	}

	updates := map[string]any{
		"adsense_enabled": *req.AdsenseEnabled,
		"updated_at":      time.Now().UTC(),
	}
	if err := h.DB.Model(&models.AdsSettings{}).Where("id = ?", settings.ID).Updates(updates).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	h.logActivity(c, "update_ads_settings", fmt.Sprintf("adsense_enabled=%v", *req.AdsenseEnabled))
	settings.AdsenseEnabled = *req.AdsenseEnabled
	settings.UpdatedAt = updates["updated_at"].(time.Time)
	return c.JSON(settings)
}

type DirectAdRequest struct {
	Title       string `json:"title"`
	ImageURL    string `json:"image_url"`
	LinkURL     string `json:"link_url"`
	TrackingKey string `json:"tracking_key"`
	IsActive    *bool  `json:"is_active"`
}

func (h *Handler) AdminListDirectAds(c *fiber.Ctx) error {
	page, limit := utils.ParsePagination(c, 1, 20)
	offset := (page - 1) * limit

	var total int64
	if err := h.DB.Model(&models.DirectAd{}).Count(&total).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	var ads []models.DirectAd
	if err := h.DB.Order("id desc").Limit(limit).Offset(offset).Find(&ads).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{
		"page":  page,
		"limit": limit,
		"total": total,
		"data":  ads,
	})
}

func (h *Handler) AdminCreateDirectAd(c *fiber.Ctx) error {
	var req DirectAdRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	if req.Title == "" || req.ImageURL == "" || req.LinkURL == "" {
		return fiber.NewError(fiber.StatusBadRequest, "title, image_url, link_url are required")
	}

	ad := models.DirectAd{
		Title:       req.Title,
		ImageURL:    req.ImageURL,
		LinkURL:     req.LinkURL,
		TrackingKey: req.TrackingKey,
		IsActive:    true,
	}
	if req.IsActive != nil {
		ad.IsActive = *req.IsActive
	}

	if err := h.DB.Create(&ad).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	h.logActivity(c, fmt.Sprintf("create_direct_ad:%d", ad.ID), ad.Title)
	return c.Status(fiber.StatusCreated).JSON(ad)
}

func (h *Handler) AdminUpdateDirectAd(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.ErrBadRequest
	}
	var req DirectAdRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}

	updates := map[string]any{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.ImageURL != "" {
		updates["image_url"] = req.ImageURL
	}
	if req.LinkURL != "" {
		updates["link_url"] = req.LinkURL
	}
	if req.TrackingKey != "" {
		updates["tracking_key"] = req.TrackingKey
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if len(updates) == 0 {
		return fiber.ErrBadRequest
	}
	updates["updated_at"] = time.Now().UTC()

	if err := h.DB.Model(&models.DirectAd{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fiber.ErrInternalServerError
	}

	h.logActivity(c, fmt.Sprintf("update_direct_ad:%d", id), "")
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) AdminDeleteDirectAd(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return fiber.ErrBadRequest
	}

	if err := h.DB.Delete(&models.DirectAd{}, id).Error; err != nil {
		return fiber.ErrInternalServerError
	}
	h.logActivity(c, fmt.Sprintf("delete_direct_ad:%d", id), "")
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) getOrCreateAdsSettings() (models.AdsSettings, error) {
	var settings models.AdsSettings
	err := h.DB.First(&settings).Error
	if err == nil {
		return settings, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return settings, err
	}
	settings = models.AdsSettings{AdsenseEnabled: true, UpdatedAt: time.Now().UTC()}
	if err := h.DB.Create(&settings).Error; err != nil {
		return settings, err
	}
	return settings, nil
}

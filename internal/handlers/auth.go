package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"truyenm/backend/internal/auth"
	"truyenm/backend/internal/models"
	"truyenm/backend/internal/utils"
)

const (
	accessTokenTTL  = 5 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour
	loginFailLimit  = 10
	loginFailWindow = time.Minute
)

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

func (h *Handler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}

	if len(req.Username) < 3 || len(req.Password) < 6 || !strings.Contains(req.Email, "@") {
		return fiber.NewError(fiber.StatusBadRequest, "invalid input")
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return fiber.ErrInternalServerError
	}

	user := models.User{
		Username:     req.Username,
		Email:        strings.ToLower(req.Email),
		PasswordHash: hash,
		Role:         models.RoleUser,
	}

	if err := h.DB.Create(&user).Error; err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "user already exists")
	}

	token, err := h.issueTokens(c, user)
	if err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"points":   user.Points,
			"role":     user.Role,
		},
	})
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	if req.Identifier == "" || req.Password == "" {
		return fiber.ErrBadRequest
	}

	var user models.User
	identifier := strings.TrimSpace(req.Identifier)
	identifierLower := strings.ToLower(identifier)

	if !strings.Contains(identifierLower, "@") {
		return fiber.NewError(fiber.StatusBadRequest, "email required")
	}

	err := h.DB.
		Where("lower(email) = ?", identifierLower).
		First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			if h.markLoginFailure(c.IP()) >= loginFailLimit {
				return fiber.ErrTooManyRequests
			}
			return fiber.ErrUnauthorized
		}
		return fiber.ErrInternalServerError
	}

	if err := h.ensureNotBanned(&user); err != nil {
		return err
	}

	if !utils.CheckPassword(user.PasswordHash, req.Password) {
		if h.markLoginFailure(c.IP()) >= loginFailLimit {
			return fiber.ErrTooManyRequests
		}
		return fiber.ErrUnauthorized
	}

	token, err := h.issueTokens(c, user)
	if err != nil {
		return fiber.ErrInternalServerError
	}
	h.clearLoginFailures(c.IP())

	return c.JSON(fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"points":   user.Points,
			"role":     user.Role,
		},
	})
}

func (h *Handler) markLoginFailure(ip string) int64 {
	if h.Redis == nil || ip == "" {
		return 0
	}
	key := "rl:login:" + ip
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	count, err := h.Redis.Incr(ctx, key).Result()
	if err != nil {
		return 0
	}
	if count == 1 {
		h.Redis.Expire(ctx, key, loginFailWindow)
	}
	return count
}

func (h *Handler) clearLoginFailures(ip string) {
	if h.Redis == nil || ip == "" {
		return
	}
	key := "rl:login:" + ip
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	h.Redis.Del(ctx, key) //nolint:errcheck
}

func (h *Handler) Refresh(c *fiber.Ctx) error {
	raw := c.Cookies("refresh_token")
	if raw == "" {
		return fiber.ErrUnauthorized
	}

	hash := auth.HashToken(raw)
	now := time.Now().UTC()

	var rt models.RefreshToken
	if err := h.DB.Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", hash, now).
		First(&rt).Error; err != nil {
		return fiber.ErrUnauthorized
	}

	var user models.User
	if err := h.DB.First(&user, rt.UserID).Error; err != nil {
		return fiber.ErrUnauthorized
	}
	if err := h.ensureNotBanned(&user); err != nil {
		return err
	}

	var newToken string
	var newHash string

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND revoked_at IS NULL", rt.ID).
			First(&rt).Error; err != nil {
			return err
		}

		var err error
		newToken, newHash, err = auth.GenerateRefreshToken()
		if err != nil {
			return err
		}

		newRT := models.RefreshToken{
			UserID:    rt.UserID,
			TokenHash: newHash,
			ExpiresAt: now.Add(refreshTokenTTL),
			UserAgent: c.Get("User-Agent"),
			IP:        c.IP(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := tx.Create(&newRT).Error; err != nil {
			return err
		}

		if err := tx.Model(&rt).Updates(map[string]any{
			"revoked_at":     &now,
			"replaced_by_id": newRT.ID,
			"last_used_at":   &now,
			"updated_at":     now,
		}).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		return fiber.ErrUnauthorized
	}

	accessToken, err := auth.GenerateToken(user.ID, user.Role, h.Cfg.JWTSecret, accessTokenTTL)
	if err != nil {
		return fiber.ErrInternalServerError
	}

	h.setAccessCookie(c, accessToken, now.Add(accessTokenTTL))
	h.setRefreshCookie(c, newToken, now.Add(refreshTokenTTL))

	return c.JSON(fiber.Map{
		"token": accessToken,
		"user": fiber.Map{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"points":   user.Points,
			"role":     user.Role,
		},
	})
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	raw := c.Cookies("refresh_token")
	if raw != "" {
		hash := auth.HashToken(raw)
		now := time.Now().UTC()
		h.DB.Model(&models.RefreshToken{}).
			Where("token_hash = ? AND revoked_at IS NULL", hash).
			Updates(map[string]any{"revoked_at": &now, "updated_at": now})
	}

	h.clearAccessCookie(c)
	h.clearRefreshCookie(c)
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) LogoutAll(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)
	now := time.Now().UTC()
	h.DB.Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Updates(map[string]any{"revoked_at": &now, "updated_at": now})

	h.clearAccessCookie(c)
	h.clearRefreshCookie(c)
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) issueTokens(c *fiber.Ctx, user models.User) (string, error) {
	accessToken, err := auth.GenerateToken(user.ID, user.Role, h.Cfg.JWTSecret, accessTokenTTL)
	if err != nil {
		return "", err
	}

	raw, hash, err := auth.GenerateRefreshToken()
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	rt := models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: now.Add(refreshTokenTTL),
		UserAgent: c.Get("User-Agent"),
		IP:        c.IP(),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.DB.Create(&rt).Error; err != nil {
		return "", err
	}

	h.setAccessCookie(c, accessToken, now.Add(accessTokenTTL))
	h.setRefreshCookie(c, raw, rt.ExpiresAt)
	return accessToken, nil
}

func (h *Handler) setAccessCookie(c *fiber.Ctx, token string, expires time.Time) {
	cookie := &fiber.Cookie{
		Name:     "access_token",
		Value:    token,
		Expires:  expires,
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   h.Cfg.Env == "prod",
		Path:     "/",
	}
	if h.Cfg.CookieDomain != "" && h.Cfg.CookieDomain != "localhost" {
		cookie.Domain = h.Cfg.CookieDomain
	}
	c.Cookie(cookie)
}

func (h *Handler) setRefreshCookie(c *fiber.Ctx, token string, expires time.Time) {
	cookie := &fiber.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Expires:  expires,
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   h.Cfg.Env == "prod",
		Path:     "/",
	}
	if h.Cfg.CookieDomain != "" && h.Cfg.CookieDomain != "localhost" {
		cookie.Domain = h.Cfg.CookieDomain
	}
	c.Cookie(cookie)
}

func (h *Handler) clearRefreshCookie(c *fiber.Ctx) {
	cookie := &fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   h.Cfg.Env == "prod",
		Path:     "/",
	}
	if h.Cfg.CookieDomain != "" && h.Cfg.CookieDomain != "localhost" {
		cookie.Domain = h.Cfg.CookieDomain
	}
	c.Cookie(cookie)
}

func (h *Handler) clearAccessCookie(c *fiber.Ctx) {
	cookie := &fiber.Cookie{
		Name:     "access_token",
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   h.Cfg.Env == "prod",
		Path:     "/",
	}
	if h.Cfg.CookieDomain != "" && h.Cfg.CookieDomain != "localhost" {
		cookie.Domain = h.Cfg.CookieDomain
	}
	c.Cookie(cookie)
}

func (h *Handler) ensureNotBanned(user *models.User) error {
	if !user.IsBanned {
		return nil
	}
	if user.BanExpiresAt != nil && time.Now().UTC().After(*user.BanExpiresAt) {
		now := time.Now().UTC()
		h.DB.Model(&models.User{}).Where("id = ?", user.ID).
			Updates(map[string]any{
				"is_banned":      false,
				"banned_at":      nil,
				"ban_reason":     "",
				"ban_expires_at": nil,
				"updated_at":     now,
			})
		user.IsBanned = false
		user.BanExpiresAt = nil
		user.BanReason = ""
		return nil
	}
	return fiber.NewError(fiber.StatusForbidden, "user banned")
}

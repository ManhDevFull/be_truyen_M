package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"truyenm/backend/internal/models"
)

// RequireRole allows only specified roles.
func RequireRole(allowed ...string) fiber.Handler {
	allowedSet := map[string]bool{}
	for _, role := range allowed {
		allowedSet[role] = true
	}
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		if allowedSet[role] {
			return c.Next()
		}
		return fiber.ErrForbidden
	}
}

// RequirePermission checks a specific permission using DB-backed role_permissions table.
// Owner always passes. Results are cached in Redis for 5 minutes.
func RequirePermission(db *gorm.DB, rdb *redis.Client, permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		if role == "" {
			return fiber.ErrForbidden
		}
		if role == models.RoleOwner {
			return c.Next()
		}

		// Try Redis cache first
		if rdb != nil {
			cacheKey := "perms:" + role
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			cached, err := rdb.HGet(ctx, cacheKey, permission).Result()
			if err == nil {
				if cached == "1" {
					return c.Next()
				}
				return fiber.ErrForbidden
			}
			// Cache miss – load from DB and populate
			if err == redis.Nil {
				var perms []models.RolePermission
				if dbErr := db.Where("role = ?", role).Find(&perms).Error; dbErr == nil {
					pipe := rdb.Pipeline()
					for _, p := range perms {
						pipe.HSet(ctx, cacheKey, p.Permission, "1")
					}
					pipe.Expire(ctx, cacheKey, 5*time.Minute)
					pipe.Exec(ctx) //nolint:errcheck

					for _, p := range perms {
						if p.Permission == "*" || p.Permission == permission {
							return c.Next()
						}
					}
					return fiber.ErrForbidden
				}
			}
		}

		// Fallback: direct DB check
		var count int64
		db.Model(&models.RolePermission{}).
			Where("role = ? AND (permission = ? OR permission = '*')", role, permission).
			Count(&count)
		if count > 0 {
			return c.Next()
		}
		return fiber.ErrForbidden
	}
}

// RequireOwner allows only the owner role.
func RequireOwner() fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		if role == models.RoleOwner {
			return c.Next()
		}
		return fiber.ErrForbidden
	}
}

// AdminRoles is the set of roles that can access admin panel.
var AdminRoles = []string{
	models.RoleOwner,
	models.RoleAdmin,
	models.RoleStaff,
	models.RoleModerator,
}

// IsAdminRole returns true if the given role is an admin-level role.
func IsAdminRole(role string) bool {
	for _, r := range AdminRoles {
		if r == role {
			return true
		}
	}
	return false
}

// OptionalRole extracts role without failing if absent.
func OptionalRole(c *fiber.Ctx) string {
	role, _ := c.Locals("role").(string)
	return role
}

// CORSHeaders adds allowed origins from a comma-separated list.
func CORSHeaders(origins string) fiber.Handler {
	originSet := map[string]bool{}
	for _, o := range strings.Split(origins, ",") {
		originSet[strings.TrimSpace(o)] = true
	}
	return func(c *fiber.Ctx) error {
		origin := c.Get("Origin")
		if originSet[origin] {
			c.Set("Access-Control-Allow-Origin", origin)
			c.Set("Access-Control-Allow-Credentials", "true")
			c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Request-ID")
		}
		if c.Method() == "OPTIONS" {
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Next()
	}
}

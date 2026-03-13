package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"truyenm/backend/internal/auth"
)

func RequireAuth(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr := extractBearer(c.Get("Authorization"))
		if tokenStr == "" {
			tokenStr = c.Cookies("access_token")
		}
		if tokenStr == "" {
			return fiber.ErrUnauthorized
		}

		claims, err := auth.ParseToken(tokenStr, jwtSecret)
		if err != nil {
			return fiber.ErrUnauthorized
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("role", claims.Role)
		return c.Next()
	}
}

// OptionalAuth parses JWT if present, otherwise continues without auth.
func OptionalAuth(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr := extractBearer(c.Get("Authorization"))
		if tokenStr == "" {
			tokenStr = c.Cookies("access_token")
		}
		if tokenStr == "" {
			return c.Next()
		}
		claims, err := auth.ParseToken(tokenStr, jwtSecret)
		if err != nil {
			return c.Next()
		}
		c.Locals("user_id", claims.UserID)
		c.Locals("role", claims.Role)
		return c.Next()
	}
}

// LoginRateLimit limits login attempts per IP (5/min).
func LoginRateLimit(rdb *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if rdb == nil {
			return c.Next()
		}
		key := "rl:login:" + c.IP()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			return c.Next()
		}
		if count == 1 {
			rdb.Expire(ctx, key, time.Minute)
		}
		if count > 5 {
			return fiber.ErrTooManyRequests
		}
		return c.Next()
	}
}

func extractBearer(authHeader string) string {
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(authHeader, "Bearer ")
}

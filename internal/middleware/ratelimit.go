package middleware

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"truyenm/backend/internal/utils"
)

func RateLimit(client *redis.Client, limit int, window time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if client == nil {
			return c.Next()
		}

		fingerprint := utils.DeviceFingerprint(c.Get("User-Agent"), c.IP())
		key := "rl:" + fingerprint

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		count, err := client.Incr(ctx, key).Result()
		if err != nil {
			return c.Next()
		}

		if count == 1 {
			client.Expire(ctx, key, window)
		}

		if int(count) > limit {
			return fiber.ErrTooManyRequests
		}

		return c.Next()
	}
}

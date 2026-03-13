package utils

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func ParsePagination(c *fiber.Ctx, defaultPage, defaultLimit int) (page int, limit int) {
	page = defaultPage
	limit = defaultLimit

	if v := c.Query("page"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			page = i
		}
	}
	if v := c.Query("limit"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			limit = i
		}
	}
	return page, limit
}

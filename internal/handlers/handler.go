package handlers

import (
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"truyenm/backend/internal/config"
)

type Handler struct {
	DB     *gorm.DB
	Redis  *redis.Client
	Logger *zap.Logger
	Cfg    *config.Config
}

func NewHandler(db *gorm.DB, redis *redis.Client, logger *zap.Logger, cfg *config.Config) *Handler {
	return &Handler{DB: db, Redis: redis, Logger: logger, Cfg: cfg}
}

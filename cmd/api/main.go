package main

import (
	"context"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"truyenm/backend/internal/cache"
	"truyenm/backend/internal/config"
	"truyenm/backend/internal/crawler"
	"truyenm/backend/internal/db"
	"truyenm/backend/internal/handlers"
	"truyenm/backend/internal/logger"
	"truyenm/backend/internal/models"
	"truyenm/backend/internal/routes"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	zapLogger, err := logger.New(cfg.Env)
	if err != nil {
		log.Fatal(err)
	}
	defer zapLogger.Sync()

	database, err := db.Connect(cfg.DatabaseDSN())
	if err != nil {
		zapLogger.Fatal("db connect failed", zap.Error(err))
	}

	if cfg.DBAutoMigrate {
		if err := database.AutoMigrate(
			&models.User{},
			&models.Comic{},
			&models.Chapter{},
			&models.ReadingHistory{},
			&models.PointsLog{},
			&models.UserAdblock{},
			&models.AdsStat{},
			&models.AdsSettings{},
			&models.DirectAd{},
			&models.RefreshToken{},
			&models.SupportTicket{},
			&models.TicketMessage{},
			&models.TrafficLog{},
			&models.Genre{},
			&models.ComicGenre{},
			&models.ComicFollow{},
			&models.RolePermission{},
			&models.ActivityLog{},
			&models.CrawlerLog{},
		); err != nil {
			zapLogger.Fatal("db migrate failed", zap.Error(err))
		}
	} else {
		zapLogger.Info("db automigrate disabled")
	}

	if err := db.EnsureComicSlugs(database); err != nil {
		zapLogger.Warn("ensure comic slugs failed", zap.Error(err))
	}

	redisClient, err := cache.Connect(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		zapLogger.Warn("redis connect failed", zap.Error(err))
	}

	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Join(strings.FieldsFunc(cfg.CORSOrigins, func(r rune) bool { return r == ',' || r == ' ' }), ","),
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowCredentials: true,
	}))

	h := handlers.NewHandler(database, redisClient, zapLogger, cfg)
	routes.Register(app, h)
	crawler.Start(context.Background(), cfg, database, zapLogger)

	zapLogger.Info("listening", zap.String("port", cfg.Port))
	if err := app.Listen(cfg.Port); err != nil {
		zapLogger.Fatal("server error", zap.Error(err))
	}
}

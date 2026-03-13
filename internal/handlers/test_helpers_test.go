package handlers_test

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"truyenm/backend/internal/config"
	"truyenm/backend/internal/handlers"
	"truyenm/backend/internal/models"
	"truyenm/backend/internal/routes"
)

func setupTestApp(t *testing.T) (*fiber.App, *handlers.Handler, *gorm.DB, *redis.Client, *config.Config) {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.New(log.New(os.Stdout, "", log.LstdFlags), gormlogger.Config{
			LogLevel: gormlogger.Silent,
		}),
	})
	if err != nil {
		t.Fatalf("db connect failed: %v", err)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Comic{},
		&models.Chapter{},
		&models.ReadingHistory{},
		&models.PointsLog{},
		&models.UserAdblock{},
		&models.AdsStat{},
		&models.RefreshToken{},
		&models.SupportTicket{},
		&models.TicketMessage{},
		&models.TrafficLog{},
		&models.Genre{},
		&models.ComicGenre{},
		&models.RolePermission{},
		&models.ActivityLog{},
	); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	cfg := &config.Config{
		Env:         "test",
		Port:        ":0",
		JWTSecret:   "test-secret",
		CORSOrigins: "http://localhost:3000",
	}

	var rdb *redis.Client
	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	if redisAddr != "" {
		rdb = redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: os.Getenv("TEST_REDIS_PASSWORD"),
			DB:       0,
		})
	}

	h := handlers.NewHandler(db, rdb, zap.NewNop(), cfg)
	app := fiber.New()
	routes.Register(app, h)

	// Reduce bcrypt cost for tests if needed
	time.Sleep(10 * time.Millisecond)

	return app, h, db, rdb, cfg
}

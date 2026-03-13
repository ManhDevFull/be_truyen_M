package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Env         string
	Port        string
	JWTSecret   string
	CORSOrigins string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
	DBDSN      string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	MinIOEndpoint string
	MinIOKey      string
	MinIOSecret   string
	MinIOBucket   string

	CloudinaryCloudName string
	CloudinaryAPIKey    string
	CloudinaryAPISecret string
	CloudinaryTransform string

	CookieDomain string

	CrawlerEnabled                 bool
	CrawlerTimezone                string
	CrawlerTickSeconds             int
	CrawlerIntervalDailyMinutes    int
	CrawlerIntervalScheduleMinutes int
	CrawlerCmd                     string
	CrawlerArgs                    string
	CrawlerWebhookURL              string
	CrawlerTimeoutSeconds          int
}

func Load() *Config {
	cfg := &Config{}

	cfg.Env = getEnv("APP_ENV", "dev")
	cfg.Port = getEnv("APP_PORT", ":8080")
	cfg.JWTSecret = getEnv("JWT_SECRET", "")
	cfg.CORSOrigins = getEnv("CORS_ORIGINS", "http://localhost:3000,http://localhost:3001")

	cfg.DBHost = getEnv("DB_HOST", "localhost")
	cfg.DBPort = getEnv("DB_PORT", "5433")
	cfg.DBUser = getEnv("DB_USER", "")
	cfg.DBPassword = getEnv("DB_PASSWORD", "")
	cfg.DBName = getEnv("DB_NAME", "")
	cfg.DBSSLMode = getEnv("DB_SSLMODE", "disable")
	cfg.DBDSN = getEnv("DB_DSN", "")

	cfg.RedisAddr = getEnv("REDIS_ADDR", "localhost:6379")
	cfg.RedisPassword = getEnv("REDIS_PASSWORD", "")
	cfg.RedisDB = getEnvInt("REDIS_DB", 0)

	cfg.MinIOEndpoint = getEnv("MINIO_ENDPOINT", "")
	cfg.MinIOKey = getEnv("MINIO_KEY", "")
	cfg.MinIOSecret = getEnv("MINIO_SECRET", "")
	cfg.MinIOBucket = getEnv("MINIO_BUCKET", "")

	cfg.CloudinaryCloudName = getEnv("CLOUDINARY_CLOUD_NAME", "")
	cfg.CloudinaryAPIKey = getEnv("CLOUDINARY_API_KEY", "")
	cfg.CloudinaryAPISecret = getEnv("CLOUDINARY_API_SECRET", "")
	cfg.CloudinaryTransform = getEnv("CLOUDINARY_TRANSFORM", "f_auto,q_auto:good,w_1600")
	cfg.CookieDomain = getEnv("COOKIE_DOMAIN", "")

	cfg.CrawlerEnabled = getEnvBool("CRAWLER_ENABLED", false)
	cfg.CrawlerTimezone = getEnv("CRAWLER_TIMEZONE", "Asia/Ho_Chi_Minh")
	cfg.CrawlerTickSeconds = getEnvInt("CRAWLER_TICK_SECONDS", 300)
	cfg.CrawlerIntervalDailyMinutes = getEnvInt("CRAWLER_INTERVAL_DAILY_MINUTES", 30)
	cfg.CrawlerIntervalScheduleMinutes = getEnvInt("CRAWLER_INTERVAL_SCHEDULE_MINUTES", 10)
	cfg.CrawlerCmd = getEnv("CRAWLER_CMD", "")
	cfg.CrawlerArgs = getEnv("CRAWLER_ARGS", "")
	cfg.CrawlerWebhookURL = getEnv("CRAWLER_WEBHOOK_URL", "")
	cfg.CrawlerTimeoutSeconds = getEnvInt("CRAWLER_TIMEOUT_SECONDS", 300)

	return cfg
}

func (c *Config) DatabaseDSN() string {
	if c.DBDSN != "" {
		return c.DBDSN
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		val := strings.ToLower(v)
		if val == "true" || val == "1" || val == "yes" {
			return true
		}
		if val == "false" || val == "0" || val == "no" {
			return false
		}
	}
	return fallback
}

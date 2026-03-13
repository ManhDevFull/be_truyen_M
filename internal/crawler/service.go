package crawler

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"truyenm/backend/internal/config"
	"truyenm/backend/internal/models"
	"truyenm/backend/internal/utils"
)

type Service struct {
	db                      *gorm.DB
	logger                  *zap.Logger
	runner                  JobRunner
	loc                     *time.Location
	timeout                 time.Duration
	intervalDailyMinutes    int
	intervalScheduleMinutes int
}

func NewService(cfg *config.Config, db *gorm.DB, logger *zap.Logger) (*Service, error) {
	loc, err := time.LoadLocation(cfg.CrawlerTimezone)
	if err != nil {
		logger.Warn("invalid crawler timezone, fallback to local", zap.String("timezone", cfg.CrawlerTimezone))
		loc = time.Local
	}

	runner := NewJobRunner(cfg, logger)
	if runner == nil {
		return nil, fmt.Errorf("crawler runner is not configured")
	}

	intervalDaily := cfg.CrawlerIntervalDailyMinutes
	if intervalDaily <= 0 {
		intervalDaily = 30
	}
	intervalSchedule := cfg.CrawlerIntervalScheduleMinutes
	if intervalSchedule <= 0 {
		intervalSchedule = 10
	}
	timeout := time.Duration(cfg.CrawlerTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	return &Service{
		db:                      db,
		logger:                  logger,
		runner:                  runner,
		loc:                     loc,
		timeout:                 timeout,
		intervalDailyMinutes:    intervalDaily,
		intervalScheduleMinutes: intervalSchedule,
	}, nil
}

func (s *Service) ShouldRun(comic models.Comic, now time.Time) bool {
	if !comic.CrawlerEnabled || comic.CrawlerSourceURL == "" {
		return false
	}

	switch comic.CrawlerMode {
	case models.CrawlerModeFullSync:
		return comic.CrawlerLastCheckedAt == nil
	case models.CrawlerModeDaily:
		interval := pickInterval(comic.CrawlerIntervalMinutes, s.intervalDailyMinutes)
		return due(comic.CrawlerLastCheckedAt, now, interval)
	case models.CrawlerModeSchedule:
		if comic.CrawlerWeekdays == 0 {
			return false
		}
		if !utils.MaskHasWeekday(comic.CrawlerWeekdays, now) {
			return false
		}
		interval := pickInterval(comic.CrawlerIntervalMinutes, s.intervalScheduleMinutes)
		return due(comic.CrawlerLastCheckedAt, now, interval)
	default:
		return false
	}
}

func (s *Service) RunComicByID(ctx context.Context, comicID uint, trigger string, userID *uint) error {
	var comic models.Comic
	if err := s.db.First(&comic, comicID).Error; err != nil {
		return err
	}
	return s.RunComic(ctx, comic, trigger, userID)
}

func (s *Service) RunComic(ctx context.Context, comic models.Comic, trigger string, userID *uint) error {
	if comic.CrawlerSourceURL == "" {
		return fmt.Errorf("crawler source url is empty")
	}
	now := time.Now().In(s.loc)
	mode := "incremental"
	if comic.CrawlerMode == models.CrawlerModeFullSync {
		mode = "full_sync"
	}

	job := CrawlJob{
		ComicID:       comic.ID,
		Title:         comic.Title,
		SourceURL:     comic.CrawlerSourceURL,
		Mode:          mode,
		LastChapter:   comic.CrawlerLastChapter,
		TargetChapter: comic.CrawlerTargetChapter,
		ContentType:   comic.ContentType,
	}

	startedAt := time.Now()
	logID, _ := s.createLog(comic, trigger, userID, startedAt)

	runCtx, cancel := context.WithTimeout(ctx, s.timeout)
	err := s.runner.Run(runCtx, job)
	cancel()

	finishedAt := time.Now()
	durationMs := finishedAt.Sub(startedAt).Milliseconds()

	updates := map[string]any{}
	if err == nil {
		updates["crawler_last_checked_at"] = now
		maxChapter, maxErr := s.maxChapterNumber(comic.ID)
		if maxErr == nil {
			updates["crawler_last_chapter"] = maxChapter
		}
		if comic.CrawlerMode == models.CrawlerModeFullSync {
			updates["crawler_enabled"] = false
		}
		s.updateLog(logID, models.CrawlerStatusSuccess, "", finishedAt, durationMs, maxInt(maxChapter, comic.CrawlerLastChapter))
	} else {
		if comic.CrawlerMode != models.CrawlerModeFullSync {
			updates["crawler_last_checked_at"] = now
		}
		s.updateLog(logID, models.CrawlerStatusFailed, err.Error(), finishedAt, durationMs, comic.CrawlerLastChapter)
	}

	if len(updates) > 0 {
		if dbErr := s.db.Model(&models.Comic{}).Where("id = ?", comic.ID).Updates(updates).Error; dbErr != nil {
			s.logger.Warn("crawler update state failed", zap.Uint("comic_id", comic.ID), zap.Error(dbErr))
		}
	}

	return err
}

func (s *Service) createLog(comic models.Comic, trigger string, userID *uint, startedAt time.Time) (uint, error) {
	log := models.CrawlerLog{
		ComicID:           comic.ID,
		Mode:              comic.CrawlerMode,
		Trigger:           trigger,
		Status:            models.CrawlerStatusRunning,
		Message:           "",
		SourceURL:         comic.CrawlerSourceURL,
		LastChapterBefore: comic.CrawlerLastChapter,
		LastChapterAfter:  comic.CrawlerLastChapter,
		TargetChapter:     comic.CrawlerTargetChapter,
		StartedAt:         startedAt,
		TriggeredBy:       userID,
	}
	if err := s.db.Create(&log).Error; err != nil {
		s.logger.Warn("crawler log create failed", zap.Uint("comic_id", comic.ID), zap.Error(err))
		return 0, err
	}
	return log.ID, nil
}

func (s *Service) updateLog(id uint, status string, message string, finishedAt time.Time, durationMs int64, lastChapterAfter int) {
	if id == 0 {
		return
	}
	updates := map[string]any{
		"status":             status,
		"message":            message,
		"finished_at":        finishedAt,
		"duration_ms":        durationMs,
		"last_chapter_after": lastChapterAfter,
	}
	if err := s.db.Model(&models.CrawlerLog{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		s.logger.Warn("crawler log update failed", zap.Uint("log_id", id), zap.Error(err))
	}
}

func (s *Service) maxChapterNumber(comicID uint) (int, error) {
	var max int
	if err := s.db.Model(&models.Chapter{}).Where("comic_id = ?", comicID).
		Select("COALESCE(MAX(chapter_number),0)").Scan(&max).Error; err != nil {
		return 0, err
	}
	return max, nil
}

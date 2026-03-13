package crawler

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"truyenm/backend/internal/config"
	"truyenm/backend/internal/models"
)

type Scheduler struct {
	service *Service
	tick    time.Duration
	running bool
}

func Start(ctx context.Context, cfg *config.Config, db *gorm.DB, logger *zap.Logger) {
	if !cfg.CrawlerEnabled {
		logger.Info("crawler disabled")
		return
	}

	service, err := NewService(cfg, db, logger)
	if err != nil {
		logger.Warn("crawler enabled but service not ready", zap.Error(err))
		return
	}

	tickSeconds := cfg.CrawlerTickSeconds
	if tickSeconds <= 0 {
		tickSeconds = 300
	}

	s := &Scheduler{
		service: service,
		tick:    time.Duration(tickSeconds) * time.Second,
	}

	logger.Info("crawler scheduler started",
		zap.Duration("tick", s.tick),
		zap.String("timezone", service.loc.String()),
		zap.Int("daily_interval_minutes", service.intervalDailyMinutes),
		zap.Int("schedule_interval_minutes", service.intervalScheduleMinutes),
	)

	go s.loop(ctx)
}

func (s *Scheduler) loop(ctx context.Context) {
	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.running {
				s.service.logger.Warn("crawler tick skipped, previous run still active")
				continue
			}
			s.running = true
			s.runOnce(ctx)
			s.running = false
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context) {
	var comics []models.Comic
	if err := s.service.db.Where("crawler_enabled = ? AND crawler_source_url <> ''", true).Find(&comics).Error; err != nil {
		s.service.logger.Error("crawler load comics failed", zap.Error(err))
		return
	}

	now := time.Now().In(s.service.loc)
	for _, comic := range comics {
		if !s.service.ShouldRun(comic, now) {
			continue
		}
		if err := s.service.RunComic(ctx, comic, "scheduler", nil); err != nil {
			s.service.logger.Error("crawler run failed", zap.Uint("comic_id", comic.ID), zap.Error(err))
		}
	}
}

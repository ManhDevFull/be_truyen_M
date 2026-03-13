package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"

	"truyenm/backend/internal/config"
)

type CrawlJob struct {
	ComicID       uint   `json:"comic_id"`
	Title         string `json:"title"`
	SourceURL     string `json:"source_url"`
	Mode          string `json:"mode"`
	LastChapter   int    `json:"last_chapter"`
	TargetChapter int    `json:"target_chapter"`
	ContentType   string `json:"content_type"`
}

type JobRunner interface {
	Run(ctx context.Context, job CrawlJob) error
}

type ExecJobRunner struct {
	cmd    string
	args   []string
	logger *zap.Logger
}

func (r *ExecJobRunner) Run(ctx context.Context, job CrawlJob) error {
	if r.cmd == "" {
		return fmt.Errorf("crawler cmd is empty")
	}
	c := exec.CommandContext(ctx, r.cmd, r.args...)
	c.Env = append(os.Environ(),
		fmt.Sprintf("CRAWL_COMIC_ID=%d", job.ComicID),
		fmt.Sprintf("CRAWL_TITLE=%s", job.Title),
		fmt.Sprintf("CRAWL_SOURCE_URL=%s", job.SourceURL),
		fmt.Sprintf("CRAWL_MODE=%s", job.Mode),
		fmt.Sprintf("CRAWL_LAST_CHAPTER=%d", job.LastChapter),
		fmt.Sprintf("CRAWL_TARGET_CHAPTER=%d", job.TargetChapter),
		fmt.Sprintf("CRAWL_CONTENT_TYPE=%s", job.ContentType),
	)
	output, err := c.CombinedOutput()
	if len(output) > 0 {
		r.logger.Info("crawler output", zap.String("output", truncate(string(output), 2000)))
	}
	if err != nil {
		return fmt.Errorf("crawler command failed: %w", err)
	}
	return nil
}

type WebhookJobRunner struct {
	url    string
	logger *zap.Logger
}

func (r *WebhookJobRunner) Run(ctx context.Context, job CrawlJob) error {
	if r.url == "" {
		return fmt.Errorf("crawler webhook url is empty")
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("crawler webhook returned %d", resp.StatusCode)
	}
	r.logger.Info("crawler webhook ok", zap.Int("status", resp.StatusCode), zap.Uint("comic_id", job.ComicID))
	return nil
}

func NewJobRunner(cfg *config.Config, logger *zap.Logger) JobRunner {
	if cfg.CrawlerWebhookURL != "" {
		return &WebhookJobRunner{url: cfg.CrawlerWebhookURL, logger: logger}
	}
	if cfg.CrawlerCmd != "" {
		args := strings.Fields(cfg.CrawlerArgs)
		return &ExecJobRunner{cmd: cfg.CrawlerCmd, args: args, logger: logger}
	}
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

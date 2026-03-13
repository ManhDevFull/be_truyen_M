package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"truyenm/backend/internal/config"
	"truyenm/backend/internal/db"
	"truyenm/backend/internal/logger"
	"truyenm/backend/internal/models"
	"truyenm/backend/internal/storage"
	"truyenm/backend/internal/utils"
)

type crawlContext struct {
	cfg      *config.Config
	db       *gorm.DB
	cloud    *storage.CloudinaryClient
	logger   *zap.Logger
	http     *http.Client
	comicID  uint
	mode     string
	source   string
	lastChap int
	target   int
}

const defaultUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"

func main() {
	cfg := config.Load()
	zapLogger, err := logger.New(cfg.Env)
	if err != nil {
		panic(err)
	}
	defer zapLogger.Sync()

	comicID := readIntEnv("CRAWL_COMIC_ID")
	if comicID <= 0 {
		zapLogger.Fatal("CRAWL_COMIC_ID is required")
	}

	mode := strings.TrimSpace(os.Getenv("CRAWL_MODE"))
	if mode == "" {
		mode = "incremental"
	}

	sourceURL := strings.TrimSpace(os.Getenv("CRAWL_SOURCE_URL"))
	lastChapter := readIntEnv("CRAWL_LAST_CHAPTER")
	targetChapter := readIntEnv("CRAWL_TARGET_CHAPTER")

	database, err := db.Connect(cfg.DatabaseDSN())
	if err != nil {
		zapLogger.Fatal("db connect failed", zap.Error(err))
	}

	ctx := &crawlContext{
		cfg:      cfg,
		db:       database,
		logger:   zapLogger,
		http:     &http.Client{Timeout: 30 * time.Second},
		comicID:  uint(comicID),
		mode:     mode,
		source:   sourceURL,
		lastChap: lastChapter,
		target:   targetChapter,
	}

	if err := ctx.ensureSourceFromDB(); err != nil {
		zapLogger.Fatal("load source url failed", zap.Error(err))
	}

	cloudClient, err := storage.NewCloudinary(cfg)
	if err != nil {
		zapLogger.Fatal("cloudinary init failed", zap.Error(err))
	}
	ctx.cloud = cloudClient

	if err := ctx.run(); err != nil {
		zapLogger.Fatal("crawler failed", zap.Error(err))
	}
}

func (c *crawlContext) run() error {
	baseURL, target := utils.NormalizeCrawlerSourceURL(c.source)
	if baseURL == "" {
		return fmt.Errorf("source url is empty")
	}
	if c.target <= 0 && target > 0 {
		c.target = target
	}
	c.source = baseURL

	c.logger.Info("crawler start", zap.String("mode", c.mode), zap.String("source", c.source), zap.Int("target", c.target))

	chapters, err := fetchChapterLinks(c.http, c.source)
	if err != nil {
		return err
	}
	if len(chapters) == 0 {
		return fmt.Errorf("no chapter links found")
	}

	existing := c.loadExistingChapters()
	toCrawl := filterChapters(chapters, existing, c.mode, c.lastChap, c.target)
	if len(toCrawl) == 0 {
		c.logger.Info("no chapters to crawl")
		return nil
	}

	for _, ch := range toCrawl {
		c.logger.Info("crawl chapter", zap.Int("chapter", ch.Number), zap.String("url", ch.URL))
		if err := c.crawlChapter(ch); err != nil {
			c.logger.Error("crawl chapter failed", zap.Int("chapter", ch.Number), zap.Error(err))
		}
	}
	return nil
}

func (c *crawlContext) ensureSourceFromDB() error {
	if c.source != "" && c.target > 0 {
		return nil
	}
	var comic models.Comic
	if err := c.db.First(&comic, c.comicID).Error; err != nil {
		return err
	}
	if c.source == "" {
		c.source = comic.CrawlerSourceURL
	}
	if c.target == 0 && comic.CrawlerTargetChapter > 0 {
		c.target = comic.CrawlerTargetChapter
	}
	if c.lastChap == 0 && comic.CrawlerLastChapter > 0 {
		c.lastChap = comic.CrawlerLastChapter
	}
	return nil
}

func (c *crawlContext) loadExistingChapters() map[int]bool {
	var numbers []int
	c.db.Model(&models.Chapter{}).Where("comic_id = ?", c.comicID).Pluck("chapter_number", &numbers)
	existing := make(map[int]bool, len(numbers))
	for _, n := range numbers {
		existing[n] = true
	}
	return existing
}

func (c *crawlContext) crawlChapter(ch chapterLink) error {
	doc, err := fetchDocument(c.http, ch.URL)
	if err != nil {
		return err
	}

	title := strings.TrimSpace(doc.Find("h1").First().Text())
	if title == "" {
		title = fmt.Sprintf("Chương %d", ch.Number)
	}

	imageURLs := extractImageURLs(doc, ch.URL)
	if len(imageURLs) == 0 {
		return fmt.Errorf("no images found")
	}

	for idx, imgURL := range imageURLs {
		publicID := fmt.Sprintf("truyenm/comics/%d/chapter_%d/page_%d", c.comicID, ch.Number, idx+1)
		err := c.downloadAndUpload(imgURL, publicID, ch.URL)
		if err != nil {
			return err
		}
	}

	contentURL := c.cloud.ChapterURLTemplate(c.comicID, ch.Number)

	chapter := models.Chapter{
		ComicID:       c.comicID,
		ChapterNumber: ch.Number,
		Title:         title,
		ContentURL:    contentURL,
		PageCount:     len(imageURLs),
	}
	if err := c.db.Create(&chapter).Error; err != nil {
		return err
	}
	return nil
}

func (c *crawlContext) downloadAndUpload(imgURL string, publicID string, referer string) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, imgURL, nil)
	if err != nil {
		return err
	}
	setImageHeaders(req, referer)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	filename := path.Base(imgURL)
	if filename == "" || filename == "/" {
		filename = "page.jpg"
	}
	if _, err := c.cloud.UploadImage(context.Background(), bytes.NewReader(data), filename, publicID); err != nil {
		return err
	}
	return nil
}

type chapterLink struct {
	Number int
	URL    string
}

func fetchChapterLinks(client *http.Client, base string) ([]chapterLink, error) {
	doc, err := fetchDocument(client, base)
	if err != nil {
		return nil, err
	}

	baseURL, _ := url.Parse(base)
	re := regexp.MustCompile(`(?i)chapter[-_/]?(\d+)`)
	seen := map[int]bool{}
	var chapters []chapterLink

	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok || !strings.Contains(strings.ToLower(href), "chapter") {
			return
		}
		matches := re.FindStringSubmatch(href)
		if len(matches) < 2 {
			return
		}
		num, err := strconv.Atoi(matches[1])
		if err != nil || num <= 0 {
			return
		}
		if seen[num] {
			return
		}
		seen[num] = true

		link, err := resolveURL(baseURL, href)
		if err != nil {
			return
		}
		chapters = append(chapters, chapterLink{Number: num, URL: link})
	})

	sort.Slice(chapters, func(i, j int) bool { return chapters[i].Number < chapters[j].Number })
	return chapters, nil
}

func fetchDocument(client *http.Client, target string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	setDocumentHeaders(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch failed: %s", resp.Status)
	}
	return goquery.NewDocumentFromReader(resp.Body)
}

func extractImageURLs(doc *goquery.Document, baseURL string) []string {
	selectors := []string{".reading-content img", ".chapter-content img", "img"}
	for _, selector := range selectors {
		if urls := extractImagesFromSelection(doc.Find(selector), baseURL, true); len(urls) > 0 {
			return urls
		}
	}
	for _, selector := range selectors {
		if urls := extractImagesFromSelection(doc.Find(selector), baseURL, false); len(urls) > 0 {
			return urls
		}
	}
	return nil
}

func extractImagesFromSelection(selection *goquery.Selection, baseURL string, dataOnly bool) []string {
	urls := []string{}
	seen := map[string]bool{}

	selection.Each(func(_ int, s *goquery.Selection) {
		candidates := []string{
			attrOrEmpty(s, "data-original"),
			attrOrEmpty(s, "data-src"),
			attrOrEmpty(s, "data-lazy-src"),
		}
		if !dataOnly {
			candidates = append(candidates, attrOrEmpty(s, "src"))
		}
		for _, raw := range candidates {
			u := normalizeImageURL(raw, baseURL)
			if u == "" || strings.HasPrefix(u, "data:") || isPlaceholderURL(u) {
				continue
			}
			if seen[u] {
				continue
			}
			seen[u] = true
			urls = append(urls, u)
			break
		}
	})

	return urls
}

func attrOrEmpty(s *goquery.Selection, name string) string {
	if v, ok := s.Attr(name); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func resolveURL(base *url.URL, href string) (string, error) {
	ref, err := url.Parse(href)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(ref).String(), nil
}

func normalizeImageURL(raw string, base string) string {
	u := strings.TrimSpace(raw)
	if u == "" {
		return ""
	}
	if strings.HasPrefix(u, "//") {
		return "https:" + u
	}
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	if base == "" {
		return u
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return u
	}
	resolved, err := resolveURL(baseURL, u)
	if err != nil {
		return u
	}
	return resolved
}

func isPlaceholderURL(u string) bool {
	lower := strings.ToLower(u)
	return strings.Contains(lower, "/images/loading") ||
		strings.Contains(lower, "/images/placeholder") ||
		strings.Contains(lower, "loading.svg") ||
		strings.Contains(lower, "placeholder.jpg")
}

func setDocumentHeaders(req *http.Request) {
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")
}

func setImageHeaders(req *http.Request, referer string) {
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")
	if referer == "" {
		return
	}
	req.Header.Set("Referer", referer)
	if origin := originFromURL(referer); origin != "" {
		req.Header.Set("Origin", origin)
	}
}

func originFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func filterChapters(chapters []chapterLink, existing map[int]bool, mode string, lastChapter int, target int) []chapterLink {
	filtered := []chapterLink{}
	maxTarget := target
	if mode == "full_sync" {
		if maxTarget <= 0 && len(chapters) > 0 {
			maxTarget = chapters[len(chapters)-1].Number
		}
		for _, ch := range chapters {
			if ch.Number > maxTarget {
				continue
			}
			if existing[ch.Number] {
				continue
			}
			filtered = append(filtered, ch)
		}
		return filtered
	}

	for _, ch := range chapters {
		if ch.Number <= lastChapter {
			continue
		}
		if existing[ch.Number] {
			continue
		}
		filtered = append(filtered, ch)
	}
	return filtered
}

func readIntEnv(key string) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return 0
	}
	n, _ := strconv.Atoi(v)
	return n
}

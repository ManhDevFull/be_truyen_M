package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"truyenm/backend/internal/config"
)

func (h *Handler) uploadPagesJSON(ctx context.Context, comicID uint, chapterNumber int, pages []string) (string, error) {
	pages = normalizePageURLs(pages)
	if len(pages) == 0 {
		return "", fmt.Errorf("page_urls is empty")
	}
	if h.Cfg.MinIOEndpoint == "" || h.Cfg.MinIOKey == "" || h.Cfg.MinIOSecret == "" || h.Cfg.MinIOBucket == "" {
		return "", fmt.Errorf("minio not configured")
	}

	client, baseURL, err := initMinioClient(h.Cfg)
	if err != nil {
		return "", err
	}

	payload := map[string][]string{"pages": pages}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	objectName := fmt.Sprintf("comics/%d/chapter_%d/pages.json", comicID, chapterNumber)
	_, err = client.PutObject(ctx, h.Cfg.MinIOBucket, objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/json",
	})
	if err != nil {
		return "", fmt.Errorf("upload pages.json failed")
	}

	url := fmt.Sprintf("%s/%s/%s", strings.TrimRight(baseURL, "/"), h.Cfg.MinIOBucket, objectName)
	return url, nil
}

func normalizePageURLs(urls []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(urls))
	for _, raw := range urls {
		u := strings.TrimSpace(raw)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		result = append(result, u)
	}
	return result
}

func initMinioClient(cfg *config.Config) (*minio.Client, string, error) {
	endpoint := strings.TrimPrefix(cfg.MinIOEndpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	secure := strings.HasPrefix(cfg.MinIOEndpoint, "https://")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOKey, cfg.MinIOSecret, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, "", err
	}

	exists, err := client.BucketExists(context.Background(), cfg.MinIOBucket)
	if err != nil {
		return nil, "", err
	}
	if !exists {
		if err := client.MakeBucket(context.Background(), cfg.MinIOBucket, minio.MakeBucketOptions{}); err != nil {
			return nil, "", err
		}
	}

	baseURL := cfg.MinIOEndpoint
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}

	return client, baseURL, nil
}

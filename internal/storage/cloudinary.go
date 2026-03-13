package storage

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"truyenm/backend/internal/config"
)

type CloudinaryClient struct {
	cloudName  string
	apiKey     string
	apiSecret  string
	transform  string
	httpClient *http.Client
}

type CloudinaryUploadResult struct {
	PublicID  string `json:"public_id"`
	SecureURL string `json:"secure_url"`
}

type cloudinaryError struct {
	Message string `json:"message"`
}

type cloudinaryResponse struct {
	PublicID  string           `json:"public_id"`
	SecureURL string           `json:"secure_url"`
	Error     *cloudinaryError `json:"error,omitempty"`
}

func NewCloudinary(cfg *config.Config) (*CloudinaryClient, error) {
	if cfg.CloudinaryCloudName == "" || cfg.CloudinaryAPIKey == "" || cfg.CloudinaryAPISecret == "" {
		return nil, fmt.Errorf("cloudinary not configured")
	}
	return &CloudinaryClient{
		cloudName:  cfg.CloudinaryCloudName,
		apiKey:     cfg.CloudinaryAPIKey,
		apiSecret:  cfg.CloudinaryAPISecret,
		transform:  strings.Trim(cfg.CloudinaryTransform, "/"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (c *CloudinaryClient) UploadImage(ctx context.Context, file io.Reader, filename string, publicID string) (CloudinaryUploadResult, error) {
	if publicID == "" {
		return CloudinaryUploadResult{}, fmt.Errorf("public_id is required")
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signParams := map[string]string{
		"public_id": publicID,
		"timestamp": timestamp,
		"overwrite": "true",
	}
	signature := buildSignature(signParams, c.apiSecret)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("api_key", c.apiKey)
	_ = writer.WriteField("timestamp", timestamp)
	_ = writer.WriteField("public_id", publicID)
	_ = writer.WriteField("overwrite", "true")
	_ = writer.WriteField("signature", signature)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return CloudinaryUploadResult{}, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return CloudinaryUploadResult{}, err
	}
	if err := writer.Close(); err != nil {
		return CloudinaryUploadResult{}, err
	}

	uploadURL := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/upload", c.cloudName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, body)
	if err != nil {
		return CloudinaryUploadResult{}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return CloudinaryUploadResult{}, err
	}
	defer resp.Body.Close()

	var payload cloudinaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return CloudinaryUploadResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if payload.Error != nil && payload.Error.Message != "" {
			return CloudinaryUploadResult{}, fmt.Errorf(payload.Error.Message)
		}
		return CloudinaryUploadResult{}, fmt.Errorf("cloudinary upload failed")
	}
	return CloudinaryUploadResult{PublicID: payload.PublicID, SecureURL: payload.SecureURL}, nil
}

func (c *CloudinaryClient) Destroy(ctx context.Context, publicID string) error {
	if publicID == "" {
		return nil
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signParams := map[string]string{
		"public_id": publicID,
		"timestamp": timestamp,
	}
	signature := buildSignature(signParams, c.apiSecret)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("api_key", c.apiKey)
	_ = writer.WriteField("timestamp", timestamp)
	_ = writer.WriteField("public_id", publicID)
	_ = writer.WriteField("signature", signature)
	if err := writer.Close(); err != nil {
		return err
	}

	destroyURL := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/destroy", c.cloudName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, destroyURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cloudinary destroy failed")
	}
	return nil
}

func (c *CloudinaryClient) ChapterURLTemplate(comicID uint, chapterNumber int) string {
	base := fmt.Sprintf("https://res.cloudinary.com/%s/image/upload", c.cloudName)
	if c.transform != "" {
		base = base + "/" + c.transform
	}
	return fmt.Sprintf("%s/truyenm/comics/%d/chapter_%d/page_{page}", base, comicID, chapterNumber)
}

func buildSignature(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params[k]))
	}
	raw := strings.Join(parts, "&") + secret
	sum := sha1.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

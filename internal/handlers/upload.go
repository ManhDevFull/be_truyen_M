package handlers

import (
	"context"
	"fmt"
	"mime/multipart"
	"sort"

	"github.com/gofiber/fiber/v2"

	"truyenm/backend/internal/models"
	"truyenm/backend/internal/storage"
)

type UploadResult struct {
	Object string `json:"object"`
	URL    string `json:"url"`
	Size   int64  `json:"size"`
}

func (h *Handler) UploadChapterPages(c *fiber.Ctx) error {
	chapterID, err := c.ParamsInt("id")
	if err != nil || chapterID <= 0 {
		return fiber.ErrBadRequest
	}

	var chapter models.Chapter
	if err := h.DB.First(&chapter, chapterID).Error; err != nil {
		return fiber.ErrNotFound
	}

	client, err := storage.NewCloudinary(h.Cfg)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "cloudinary not configured")
	}

	form, err := c.MultipartForm()
	if err != nil {
		return fiber.ErrBadRequest
	}

	files := form.File["files"]
	if len(files) == 0 {
		files = form.File["file"]
	}
	if len(files) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "no files uploaded")
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Filename < files[j].Filename })

	results := make([]UploadResult, 0, len(files))
	for idx, fh := range files {
		res, err := h.uploadObject(c.Context(), client, chapter, fh, idx+1)
		if err != nil {
			return err
		}
		results = append(results, res)
	}

	contentURL := client.ChapterURLTemplate(chapter.ComicID, chapter.ChapterNumber)
	h.DB.Model(&models.Chapter{}).Where("id = ?", chapter.ID).Updates(map[string]any{
		"content_url": contentURL,
		"page_count":  len(results),
	})

	return c.JSON(fiber.Map{
		"uploaded":    results,
		"content_url": contentURL,
		"page_count":  len(results),
	})
}

func (h *Handler) uploadObject(ctx context.Context, client *storage.CloudinaryClient, chapter models.Chapter, fh *multipart.FileHeader, index int) (UploadResult, error) {
	file, err := fh.Open()
	if err != nil {
		return UploadResult{}, fiber.ErrInternalServerError
	}
	defer file.Close()

	publicID := fmt.Sprintf("truyenm/comics/%d/chapter_%d/page_%d", chapter.ComicID, chapter.ChapterNumber, index)
	uploaded, err := client.UploadImage(ctx, file, fh.Filename, publicID)
	if err != nil {
		return UploadResult{}, fiber.NewError(fiber.StatusInternalServerError, "upload failed")
	}

	return UploadResult{Object: publicID, URL: uploaded.SecureURL, Size: fh.Size}, nil
}

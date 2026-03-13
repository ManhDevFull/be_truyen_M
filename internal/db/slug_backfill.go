package db

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"truyenm/backend/internal/models"
	"truyenm/backend/internal/utils"
)

func EnsureComicSlugs(database *gorm.DB) error {
	var comics []models.Comic
	if err := database.Where("slug IS NULL OR slug = ''").Find(&comics).Error; err != nil {
		return err
	}

	for _, comic := range comics {
		base := utils.Slugify(comic.Title)
		slug, err := ensureUniqueSlug(database, base, comic.ID)
		if err != nil {
			return err
		}
		if err := database.Model(&models.Comic{}).Where("id = ?", comic.ID).Update("slug", slug).Error; err != nil {
			return err
		}
	}

	return nil
}

func ensureUniqueSlug(database *gorm.DB, base string, excludeID uint) (string, error) {
	slug := base
	if slug == "" {
		slug = fmt.Sprintf("truyen-%d", time.Now().Unix())
	}
	for i := 0; i < 50; i++ {
		candidate := slug
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", slug, i)
		}
		var count int64
		query := database.Model(&models.Comic{}).Where("slug = ?", candidate)
		if excludeID > 0 {
			query = query.Where("id <> ?", excludeID)
		}
		if err := query.Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique slug")
}

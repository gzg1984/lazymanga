package handlers

import (
	"lazymanga/models"

	"gorm.io/gorm"
)

func updateRepoISOMissingFlag(repoDB *gorm.DB, row *models.RepoISO, missing bool) error {
	if row == nil {
		return nil
	}
	if row.IsMissing == missing {
		return nil
	}
	if err := repoDB.Model(&models.RepoISO{}).Where("id = ?", row.ID).Update("is_missing", missing).Error; err != nil {
		return err
	}
	row.IsMissing = missing
	return nil
}

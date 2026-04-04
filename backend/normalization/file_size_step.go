package normalization

import (
	"fmt"
	"lazyiso/models"
	"os"

	"gorm.io/gorm"
)

// FileSizeBackfillStep fills file size in bytes for indexed records.
type FileSizeBackfillStep struct{}

func NewFileSizeBackfillStep() RecordStep {
	return FileSizeBackfillStep{}
}

func (s FileSizeBackfillStep) Name() string {
	return "file-size-backfill"
}

func (s FileSizeBackfillStep) Process(_ uint, repoDB *gorm.DB, rootAbs string, record *models.RepoISO) error {
	if record.SizeBytes > 0 {
		return nil
	}

	absPath, err := resolveRecordAbsPath(rootAbs, record.Path)
	if err != nil {
		return err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path is directory")
	}

	size := info.Size()
	if err := repoDB.Model(&models.RepoISO{}).Where("id = ?", record.ID).Update("size_bytes", size).Error; err != nil {
		return err
	}

	record.SizeBytes = size
	return nil
}

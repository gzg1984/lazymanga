package normalization

import (
	"lazymanga/models"
	"os"
	"path/filepath"

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

	size, err := CalculatePathSizeBytes(absPath)
	if err != nil {
		return err
	}
	if err := repoDB.Model(&models.RepoISO{}).Where("id = ?", record.ID).Update("size_bytes", size).Error; err != nil {
		return err
	}

	record.SizeBytes = size
	return nil
}

// CalculatePathSizeBytes returns a file size directly and a directory size as the recursive sum of child files.
func CalculatePathSizeBytes(absPath string) (int64, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return info.Size(), nil
	}

	var total int64
	err = filepath.Walk(absPath, func(current string, currentInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if currentInfo == nil || currentInfo.IsDir() {
			return nil
		}
		total += currentInfo.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}

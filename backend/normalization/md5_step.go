package normalization

import (
	"crypto/md5"
	"fmt"
	"io"
	"lazyiso/models"
	"os"
	"strings"

	"gorm.io/gorm"
)

// MD5BackfillStep fills md5 values for indexed records.
type MD5BackfillStep struct{}

func NewMD5BackfillStep() RecordStep {
	return MD5BackfillStep{}
}

func (s MD5BackfillStep) Name() string {
	return "md5-backfill"
}

func (s MD5BackfillStep) Process(_ uint, repoDB *gorm.DB, rootAbs string, record *models.RepoISO) error {
	if strings.TrimSpace(record.MD5) != "" {
		return nil
	}

	absPath, err := resolveRecordAbsPath(rootAbs, record.Path)
	if err != nil {
		return err
	}

	sum, err := calculateFileMD5(absPath)
	if err != nil {
		return err
	}

	if err := repoDB.Model(&models.RepoISO{}).Where("id = ?", record.ID).Update("md5", sum).Error; err != nil {
		return err
	}

	record.MD5 = sum
	return nil
}

func calculateFileMD5(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

package handlers

import (
	"errors"
	"lazymanga/models"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetRepoStorageSummary returns repo-level capacity and indexed ISO size summary.
func GetRepoStorageSummary(c *gin.Context) {
	repoID := strings.TrimSpace(c.Param("id"))
	if repoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	var repo models.Repository
	if err := db.First(&repo, repoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db query failed: " + err.Error()})
		return
	}

	repoDB, rootAbs, dbPath, err := openRepoScopedDB(repo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prepare repo db failed: " + err.Error()})
		return
	}

	var isoCount int64
	if err := repoDB.Model(&models.RepoISO{}).Count(&isoCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "count repoisos failed: " + err.Error()})
		return
	}

	var sizeKnownCount int64
	if err := repoDB.Model(&models.RepoISO{}).Where("size_bytes >= 0").Count(&sizeKnownCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "count known size failed: " + err.Error()})
		return
	}

	totalSizeBytes := int64(0)
	if err := repoDB.Model(&models.RepoISO{}).Select("COALESCE(SUM(CASE WHEN size_bytes >= 0 THEN size_bytes ELSE 0 END), 0)").Scan(&totalSizeBytes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sum size failed: " + err.Error()})
		return
	}

	sizeMissingCount := isoCount - sizeKnownCount
	if sizeMissingCount < 0 {
		sizeMissingCount = 0
	}

	availableBytes, totalDiskBytes, err := getStorageStats(rootAbs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage stats failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"repo_id":             repo.ID,
		"repo_name":           repo.Name,
		"root_path":           rootAbs,
		"db_path":             dbPath,
		"iso_count":           isoCount,
		"total_size_bytes":    totalSizeBytes,
		"size_known_count":    sizeKnownCount,
		"size_missing_count":  sizeMissingCount,
		"has_incomplete_size": sizeMissingCount > 0,
		"available_bytes":     availableBytes,
		"disk_total_bytes":    totalDiskBytes,
	})
}

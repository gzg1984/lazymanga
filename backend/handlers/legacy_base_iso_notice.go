package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"lazymanga/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetLegacyBaseISOMigrationNotice returns upgrade migration summary and marks notice as consumed.
// It is designed for frontend first-open popup logic.
func GetLegacyBaseISOMigrationNotice(c *gin.Context) {
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database is not initialized"})
		return
	}

	var basicRepo models.Repository
	err := db.Where("basic = ?", true).Order("id asc").First(&basicRepo).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = db.Where("name = ?", basicRepoName).Order("id asc").First(&basicRepo).Error
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query basic repository failed: " + err.Error()})
		return
	}

	repoDB, _, _, err := openRepoScopedDB(basicRepo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "open basic repo db failed: " + err.Error()})
		return
	}

	resp := gin.H{
		"show":               false,
		"migrated":           false,
		"migrated_count":     0,
		"skipped_count":      0,
		"total_legacy_count": 0,
		"migrated_at":        "",
		"notice_shown":       false,
		"notice_shown_at":    "",
	}

	err = repoDB.Transaction(func(tx *gorm.DB) error {
		info, ensureErr := EnsureRepoInfoFromRepository(tx, basicRepo)
		if ensureErr != nil {
			return fmt.Errorf("ensure repo_info failed: %w", ensureErr)
		}

		flags := parseRepoInfoFlags(info.FlagsJSON)
		migrated, _ := flags[legacyBaseISOMigratedFlagKey].(bool)
		noticeShown, _ := flags[legacyBaseISOMigrationNoticeShownKey].(bool)
		migratedAt := flagString(flags, legacyBaseISOMigratedAtKey)
		noticeShownAt := flagString(flags, legacyBaseISOMigrationNoticeShownAtKey)
		migratedCount := flagInt(flags, legacyBaseISOMigratedCountKey)
		skippedCount := flagInt(flags, legacyBaseISOSkippedCountKey)

		resp["migrated"] = migrated
		resp["migrated_count"] = migratedCount
		resp["skipped_count"] = skippedCount
		resp["total_legacy_count"] = migratedCount + skippedCount
		resp["migrated_at"] = migratedAt
		resp["notice_shown"] = noticeShown
		resp["notice_shown_at"] = noticeShownAt

		if !migrated || noticeShown {
			resp["show"] = false
			return nil
		}

		now := time.Now().UTC().Format(time.RFC3339)
		flags[legacyBaseISOMigrationNoticeShownKey] = true
		flags[legacyBaseISOMigrationNoticeShownAtKey] = now
		info.FlagsJSON = mustMarshalFlagsJSON(flags)
		if saveErr := tx.Save(&info).Error; saveErr != nil {
			return fmt.Errorf("save notice consumed flag failed: %w", saveErr)
		}

		resp["show"] = true
		resp["notice_shown"] = true
		resp["notice_shown_at"] = now
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read migration notice failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func flagString(flags map[string]interface{}, key string) string {
	v, _ := flags[key].(string)
	return v
}

func flagInt(flags map[string]interface{}, key string) int {
	v, ok := flags[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func mustMarshalFlagsJSON(flags map[string]interface{}) string {
	encoded, err := json.Marshal(flags)
	if err != nil {
		return defaultRepoInfoFlagsJSON
	}
	return string(encoded)
}

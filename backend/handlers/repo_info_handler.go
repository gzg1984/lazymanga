package handlers

import (
	"errors"
	"lazyiso/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetRepoInfo returns the singleton repo_info row from a specific repository-scoped repo.db.
func GetRepoInfo(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	var repo models.Repository
	if err := db.First(&repo, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db query failed: " + err.Error()})
		return
	}

	repoDB, _, _, err := openRepoScopedDB(repo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prepare repo db failed: " + err.Error()})
		return
	}

	info, err := EnsureRepoInfoFromRepository(repoDB, repo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "load repo_info failed: " + err.Error()})
		return
	}

	if err := DetectRepositoryBindingConflict(repo, info); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	if err := SyncRepositoryCacheFromRepoInfo(repo, info); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sync repository cache failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, info)
}

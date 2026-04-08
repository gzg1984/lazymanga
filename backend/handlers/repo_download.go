package handlers

import (
	"errors"
	"lazymanga/models"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DownloadRepoISO downloads a repo-mode ISO file by repo id and repoiso id.
func DownloadRepoISO(c *gin.Context) {
	log.Printf("DownloadRepoISO: start method=%s path=%s remote=%s", c.Request.Method, c.Request.URL.Path, c.ClientIP())

	repoID := strings.TrimSpace(c.Param("id"))
	isoID := strings.TrimSpace(c.Param("isoId"))
	if repoID == "" || isoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id or isoId"})
		return
	}

	var repo models.Repository
	if err := db.First(&repo, repoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query repo failed: " + err.Error()})
		return
	}

	repoDB, rootAbs, _, err := openRepoScopedDB(repo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prepare repo db failed: " + err.Error()})
		return
	}

	var row models.RepoISO
	if err := repoDB.First(&row, isoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "repository entry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query repository entry failed: " + err.Error()})
		return
	}

	absPath, err := resolveRepoISOAbsPath(rootAbs, row.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid source path: " + err.Error()})
		return
	}

	fi, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			if updateErr := updateRepoISOMissingFlag(repoDB, &row, true); updateErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "file not found and update missing flag failed: " + updateErr.Error()})
				return
			}
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "stat file failed: " + err.Error()})
		return
	}
	if fi.IsDir() {
		if row.IsDirectory {
			c.JSON(http.StatusBadRequest, gin.H{"error": "directory entries are not downloadable as single files"})
			return
		}
		if updateErr := updateRepoISOMissingFlag(repoDB, &row, true); updateErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "path is a directory and update missing flag failed: " + updateErr.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is a directory"})
		return
	}
	if err := updateRepoISOMissingFlag(repoDB, &row, false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "clear missing flag failed: " + err.Error()})
		return
	}

	filename := strings.TrimSpace(row.FileName)
	if filename == "" {
		filename = filepath.Base(absPath)
	}
	c.FileAttachment(absPath, filename)
}

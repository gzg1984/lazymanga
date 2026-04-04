package handlers

import (
	"errors"
	"lazymanga/models"
	"lazymanga/sys"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CheckISOFileStatus checks whether the file behind a base ISO record still exists.
func CheckISOFileStatus(c *gin.Context) {
	log.Printf("CheckISOFileStatus: start method=%s path=%s remote=%s", c.Request.Method, c.Request.URL.Path, c.ClientIP())

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	var iso models.ISOs
	if err := db.First(&iso, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "iso record not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query iso failed: " + err.Error()})
		return
	}

	exists, isDir, resolvedPath, err := checkISOPathStatus(strings.TrimSpace(iso.Path))
	if err != nil {
		log.Printf("CheckISOFileStatus: stat failed id=%d path=%q resolved=%q err=%v", iso.ID, iso.Path, resolvedPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "check file status failed: " + err.Error()})
		return
	}

	message := "record file exists"
	if !exists {
		message = "record file missing; this record can be deleted"
	}

	log.Printf("CheckISOFileStatus: done id=%d exists=%t isDir=%t path=%q resolved=%q", iso.ID, exists, isDir, iso.Path, resolvedPath)
	c.JSON(http.StatusOK, gin.H{
		"id":            iso.ID,
		"path":          iso.Path,
		"exists":        exists,
		"is_directory":  isDir,
		"resolved_path": resolvedPath,
		"can_delete":    !exists,
		"message":       message,
	})
}

func checkISOPathStatus(rawPath string) (exists bool, isDir bool, resolvedPath string, err error) {
	candidates := buildISOPathCandidates(rawPath)
	if len(candidates) == 0 {
		return false, false, "", nil
	}

	var lastErr error
	for _, candidate := range candidates {
		fi, statErr := os.Stat(candidate)
		if statErr == nil {
			if fi.IsDir() {
				return false, true, candidate, nil
			}
			return true, false, candidate, nil
		}
		if os.IsNotExist(statErr) {
			continue
		}
		lastErr = statErr
	}

	if lastErr != nil {
		return false, false, "", lastErr
	}
	return false, false, "", nil
}

func buildISOPathCandidates(rawPath string) []string {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return nil
	}

	seen := make(map[string]struct{})
	result := make([]string, 0, 2)
	add := func(p string) {
		p = filepath.Clean(strings.TrimSpace(p))
		if p == "" || p == "." {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		result = append(result, p)
	}

	if filepath.IsAbs(trimmed) {
		add(trimmed)
	}
	add(sys.GetFullPathFromDBSubPath(trimmed))
	if !filepath.IsAbs(trimmed) {
		add(trimmed)
	}

	return result
}

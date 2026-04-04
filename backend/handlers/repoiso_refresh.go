package handlers

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"lazyiso/models"
	"lazyiso/normalization"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RefreshRepoISORecord checks file existence and backfills missing md5/size metadata.
func RefreshRepoISORecord(c *gin.Context) {
	log.Printf("RefreshRepoISORecord: start method=%s path=%s remote=%s", c.Request.Method, c.Request.URL.Path, c.ClientIP())

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

	repoInfo, err := EnsureRepoInfoFromRepository(repoDB, repo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "load repo info failed: " + err.Error()})
		return
	}

	var row models.RepoISO
	if err := repoDB.First(&row, isoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "repo iso record not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query repo iso failed: " + err.Error()})
		return
	}

	absPath, err := resolveRepoISOAbsPath(rootAbs, row.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid source path: " + err.Error()})
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := updateRepoISOMissingFlag(repoDB, &row, true); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "update missing flag failed: " + err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"message":      "file missing; record can be deleted",
				"exists":       false,
				"can_delete":   true,
				"md5_updated":  false,
				"size_updated": false,
				"record":       row,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "stat file failed: " + err.Error()})
		return
	}
	if info.IsDir() {
		if err := updateRepoISOMissingFlag(repoDB, &row, true); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "update missing flag failed: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message":      "path is directory; record can be deleted",
			"exists":       false,
			"can_delete":   true,
			"md5_updated":  false,
			"size_updated": false,
			"record":       row,
		})
		return
	}

	pathMoved, movedType, movedKeyword, err := maybeRelocateRepoISOPathByFlags(repoDB, rootAbs, &row, absPath, repoInfo.AutoNormalize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "relocate by flags failed: " + err.Error()})
		return
	}
	if pathMoved {
		absPath, err = resolveRepoISOAbsPath(rootAbs, row.Path)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid relocated path: " + err.Error()})
			return
		}

		info, err = os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				if err := updateRepoISOMissingFlag(repoDB, &row, true); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "update missing flag failed: " + err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{
					"message":              "relocated path missing; record can be deleted",
					"exists":               false,
					"can_delete":           true,
					"path_moved":           true,
					"move_matched_type":    movedType,
					"move_matched_keyword": movedKeyword,
					"md5_updated":          false,
					"size_updated":         false,
					"record":               row,
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "stat relocated file failed: " + err.Error()})
			return
		}
		if info.IsDir() {
			if err := updateRepoISOMissingFlag(repoDB, &row, true); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "update missing flag failed: " + err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"message":              "relocated path is directory; record can be deleted",
				"exists":               false,
				"can_delete":           true,
				"path_moved":           true,
				"move_matched_type":    movedType,
				"move_matched_keyword": movedKeyword,
				"md5_updated":          false,
				"size_updated":         false,
				"record":               row,
			})
			return
		}
	}

	updates := make(map[string]interface{})
	md5Updated := false
	sizeUpdated := false
	if row.IsMissing {
		updates["is_missing"] = false
	}

	if row.SizeBytes <= 0 {
		updates["size_bytes"] = info.Size()
		sizeUpdated = true
	}

	if strings.TrimSpace(row.MD5) == "" {
		sum, err := calculateRepoISOFileMD5(absPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "calculate md5 failed: " + err.Error()})
			return
		}
		updates["md5"] = sum
		md5Updated = true
	}

	if len(updates) > 0 {
		if err := repoDB.Model(&models.RepoISO{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "update metadata failed: " + err.Error()})
			return
		}
		if err := repoDB.First(&row, row.ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "reload record failed: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":              "record metadata refreshed",
		"auto_normalize":       repoInfo.AutoNormalize,
		"exists":               true,
		"can_delete":           false,
		"path_moved":           pathMoved,
		"move_matched_type":    movedType,
		"move_matched_keyword": movedKeyword,
		"md5_updated":          md5Updated,
		"size_updated":         sizeUpdated,
		"record":               row,
	})
}

func maybeRelocateRepoISOPathByFlags(repoDB *gorm.DB, rootAbs string, row *models.RepoISO, sourceAbs string, autoNormalize bool) (bool, string, string, error) {
	if !autoNormalize {
		return false, "", "", nil
	}

	fileName := strings.TrimSpace(row.FileName)
	if fileName == "" {
		fileName = filepath.Base(filepath.FromSlash(strings.TrimSpace(row.Path)))
	}
	if fileName == "" {
		return false, "", "", nil
	}

	targetDir := ""
	moveType := ""
	moveKeyword := ""

	if row.IsEntertament {
		targetDir = "Entertainment"
		moveType = "Entertainment"
	} else if row.IsOS {
		if matched, ok := normalization.GuessOSRuleByFileName(fileName); ok {
			targetDir = matched.TargetDir
			moveType = matched.TypeName
			moveKeyword = matched.Keyword
		} else {
			targetDir = "OS"
			moveType = "OS"
		}
	} else {
		return false, "", "", nil
	}

	targetAbs := filepath.Join(rootAbs, filepath.FromSlash(targetDir), fileName)
	if !isPathWithinRoot(rootAbs, targetAbs) {
		return false, moveType, moveKeyword, fmt.Errorf("target path out of repo root")
	}

	_, finalRelPath, moved, err := relocateRepoISOFile(sourceAbs, targetAbs, rootAbs)
	if err != nil {
		return false, moveType, moveKeyword, err
	}

	newFileName := filepath.Base(finalRelPath)
	if !moved && row.Path == finalRelPath && row.FileName == newFileName {
		return false, moveType, moveKeyword, nil
	}

	updates := map[string]interface{}{
		"path":      finalRelPath,
		"file_name": newFileName,
		"tags":      models.ExtractTagsFromFileName(newFileName),
	}
	if err := repoDB.Model(&models.RepoISO{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
		return false, moveType, moveKeyword, err
	}

	row.Path = finalRelPath
	row.FileName = newFileName
	row.Tags = models.ExtractTagsFromFileName(newFileName)

	return true, moveType, moveKeyword, nil
}

func calculateRepoISOFileMD5(absPath string) (string, error) {
	f, err := os.Open(absPath)
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

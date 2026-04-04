package handlers

import (
	"errors"
	"fmt"
	"io"
	"lazymanga/models"
	"lazymanga/normalization"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type createRepoISORequest struct {
	Path     string `json:"path"`
	PathKind string `json:"path_kind"`
}

func normalizeCreateRepoISOPathKind(v string) string {
	kind := strings.TrimSpace(strings.ToLower(v))
	if kind == "directory" || kind == "dir" || kind == "folder" {
		return "directory"
	}
	return "file"
}

// CreateRepoISO manually adds a single file or directory entry into the target repository index.
// If the selected path is outside current repo root, it is copied into
// <repo-root>/manual_added or <repo-root>/manual_added_dirs before indexing.
func CreateRepoISO(c *gin.Context) {
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

	repoDB, rootAbs, _, err := openRepoScopedDB(repo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prepare repo db failed: " + err.Error()})
		return
	}

	var req createRepoISORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	pathKind := normalizeCreateRepoISOPathKind(req.PathKind)

	sourceRel, sourceAbs, err := resolveInternalSourcePath(req.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}

	srcInfo, err := os.Stat(sourceAbs)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "source path not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "stat source path failed: " + err.Error()})
		return
	}
	if srcInfo.IsDir() {
		if pathKind != "directory" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "source path is a directory; please use add directory"})
			return
		}

		row, copied, err := createRepoDirectoryEntry(repoDB, rootAbs, sourceAbs)
		if err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			msg := err.Error()
			status := http.StatusInternalServerError
			if strings.Contains(msg, "already exists") {
				status = http.StatusConflict
			}
			c.JSON(status, gin.H{"error": msg})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":   "repo directory added",
			"repo_id":   repo.ID,
			"source":    sourceRel,
			"copied":    copied,
			"path_kind": "directory",
			"repo_iso":  row,
		})
		return
	}
	if pathKind == "directory" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source path is not a directory"})
		return
	}
	if !strings.EqualFold(filepath.Ext(sourceAbs), ".iso") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only .iso files are supported in file add mode"})
		return
	}

	targetAbs := sourceAbs
	if !isPathWithinRoot(rootAbs, sourceAbs) {
		targetAbs = filepath.Join(rootAbs, "manual_added", filepath.Base(sourceAbs))
		if !isPathWithinRoot(rootAbs, targetAbs) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "target path out of repo root"})
			return
		}

		if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "prepare target folder failed: " + err.Error()})
			return
		}

		finalTargetAbs, err := findRepoISOAvailableTargetPath(targetAbs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "allocate target path failed: " + err.Error()})
			return
		}

		if err := copyFile(sourceAbs, finalTargetAbs, srcInfo.Mode()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "copy source file failed: " + err.Error()})
			return
		}
		targetAbs = finalTargetAbs
	}

	relPath, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "resolve repo relative path failed: " + err.Error()})
		return
	}
	relPath = filepath.ToSlash(relPath)

	var exists models.RepoISO
	if err := repoDB.Where("path = ?", relPath).First(&exists).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "repo iso already exists", "id": exists.ID})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query existing repo iso failed: " + err.Error()})
		return
	}

	targetInfo, err := os.Stat(targetAbs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "stat target file failed: " + err.Error()})
		return
	}

	row := models.RepoISO{
		FileName:  filepath.Base(targetAbs),
		Path:      relPath,
		SizeBytes: targetInfo.Size(),
		Tags:      models.ExtractTagsFromFileName(filepath.Base(targetAbs)),
	}
	if err := repoDB.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create repo iso failed: " + err.Error()})
		return
	}

	normalization.StartAsyncPostIndexNormalization(repo.ID, repoDB, rootAbs, []models.RepoISO{row})
	c.JSON(http.StatusCreated, gin.H{
		"message":  "repo iso added",
		"repo_id":  repo.ID,
		"source":   sourceRel,
		"copied":   !sameRepoISOPath(sourceAbs, targetAbs),
		"repo_iso": row,
	})
}

func createRepoDirectoryEntry(repoDB *gorm.DB, rootAbs string, sourceAbs string) (models.RepoISO, bool, error) {
	targetAbs := sourceAbs
	if !isPathWithinRoot(rootAbs, sourceAbs) {
		targetAbs = filepath.Join(rootAbs, "manual_added_dirs", filepath.Base(sourceAbs))
		if !isPathWithinRoot(rootAbs, targetAbs) {
			return models.RepoISO{}, false, fmt.Errorf("target path out of repo root")
		}

		if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
			return models.RepoISO{}, false, fmt.Errorf("prepare target folder failed: %w", err)
		}

		finalTargetAbs, err := findRepoISOAvailableTargetPath(targetAbs)
		if err != nil {
			return models.RepoISO{}, false, fmt.Errorf("allocate target path failed: %w", err)
		}
		if err := copyDirectoryRecursive(sourceAbs, finalTargetAbs); err != nil {
			return models.RepoISO{}, false, fmt.Errorf("copy source directory failed: %w", err)
		}
		targetAbs = finalTargetAbs
	}

	relPath, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return models.RepoISO{}, false, fmt.Errorf("resolve repo relative path failed: %w", err)
	}
	relPath = filepath.ToSlash(relPath)

	var exists models.RepoISO
	if err := repoDB.Where("path = ?", relPath).First(&exists).Error; err == nil {
		return models.RepoISO{}, false, fmt.Errorf("repo path already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.RepoISO{}, false, fmt.Errorf("query existing repo path failed: %w", err)
	}

	row := models.RepoISO{
		FileName:  filepath.Base(targetAbs),
		Path:      relPath,
		SizeBytes: models.UnknownRepoISOSizeBytes,
		Tags:      models.ExtractTagsFromFileName(filepath.Base(targetAbs)),
	}
	if err := repoDB.Create(&row).Error; err != nil {
		return models.RepoISO{}, false, fmt.Errorf("create repo directory failed: %w", err)
	}

	return row, !sameRepoISOPath(sourceAbs, targetAbs), nil
}

func copyDirectoryRecursive(source string, target string) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source path is not a directory")
	}

	return filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		targetPath := target
		if rel != "." {
			targetPath = filepath.Join(target, rel)
		}

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		return copyFile(path, targetPath, info.Mode())
	})
}

func copyFile(source string, target string, mode os.FileMode) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	if err := dst.Sync(); err != nil {
		return fmt.Errorf("flush target file failed: %w", err)
	}
	return nil
}

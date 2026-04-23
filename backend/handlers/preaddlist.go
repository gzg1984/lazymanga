package handlers

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"lazymanga/models"
	"lazymanga/normalization"
	"lazymanga/normalization/rulebook"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func shouldIncludePreAddEntry(entry os.DirEntry, scanSpec *rulebook.ScanSpec) bool {
	if entry == nil {
		return false
	}
	if entry.IsDir() {
		return true
	}
	if scanSpec == nil {
		return strings.EqualFold(filepath.Ext(entry.Name()), ".iso")
	}
	return scanSpec.ShouldScanFile(entry.Name())
}

func loadPreAddScanSpec(c *gin.Context) (*rulebook.ScanSpec, error) {
	repoID := strings.TrimSpace(c.Query("repo_id"))
	if repoID == "" || db == nil {
		return nil, nil
	}

	var repo models.Repository
	if err := db.First(&repo, repoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	repoDB, _, _, err := openRepoScopedDB(repo)
	if err != nil {
		return nil, err
	}
	scanSpec := normalization.GetRuleBookScanSpecForRepo(repo.ID, repoDB)
	return &scanSpec, nil
}

func FilesHandler(c *gin.Context) {
	root := "/lzcapp/run/mnt/home/" // 根目录
	dirParam := c.Query("dir")
	dir := root
	if dirParam != "" {
		dir = filepath.Join(root, dirParam)
	}
	scanSpec, err := loadPreAddScanSpec(c)
	if err != nil {
		c.JSON(500, gin.H{"error": "无法读取仓库扫描配置"})
		return
	}
	type FileInfo struct {
		Name  string `json:"name"`
		Size  int64  `json:"size"`
		IsDir bool   `json:"isDir"`
	}
	var files []FileInfo
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(500, gin.H{"error": "无法读取目录"})
		return
	}
	for _, entry := range entries {
		if !shouldIncludePreAddEntry(entry, scanSpec) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if entry.IsDir() {
			files = append(files, FileInfo{
				Name:  entry.Name(),
				Size:  0,
				IsDir: true,
			})
		} else {
			files = append(files, FileInfo{
				Name:  entry.Name(),
				Size:  info.Size(),
				IsDir: false,
			})
		}
	}
	c.JSON(200, files)
}

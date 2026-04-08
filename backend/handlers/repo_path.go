package handlers

import (
	"errors"
	"fmt"
	"lazymanga/models"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const internalRepoRoot = "/lzcapp/run/mnt/home"
const externalRepoRoot = "/lzcapp/media"
const externalExcludedDir = "RemoteFS"

type folderEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

type externalDeviceEntry struct {
	Name string `json:"name"`
}

// ListExternalRepoDevices 列出可用外部存储目录（排除 RemoteFS）
func ListExternalRepoDevices(c *gin.Context) {
	log.Printf("ListExternalRepoDevices: start method=%s path=%s remote=%s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	var repo models.Repository
	if err := db.First(&repo, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("ListExternalRepoDevices: repo not found id=%s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
			return
		}
		log.Printf("ListExternalRepoDevices: query failed id=%s error=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db query failed: " + err.Error()})
		return
	}

	entries, err := os.ReadDir(externalRepoRoot)
	if err != nil {
		log.Printf("ListExternalRepoDevices: read root failed root=%s error=%v", externalRepoRoot, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to read external root"})
		return
	}

	devices := make([]externalDeviceEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == externalExcludedDir {
			continue
		}
		devices = append(devices, externalDeviceEntry{Name: name})
	}

	sort.Slice(devices, func(i, j int) bool { return devices[i].Name < devices[j].Name })
	log.Printf("ListExternalRepoDevices: success id=%s count=%d", id, len(devices))
	c.JSON(http.StatusOK, gin.H{"devices": devices})
}

// ListRepoPathOptions 列出仓库路径可选目录（仅目录）
func ListRepoPathOptions(c *gin.Context) {
	log.Printf("ListRepoPathOptions: start method=%s path=%s remote=%s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	var repo models.Repository
	if err := db.First(&repo, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("ListRepoPathOptions: repo not found id=%s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
			return
		}
		log.Printf("ListRepoPathOptions: query failed id=%s error=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db query failed: " + err.Error()})
		return
	}

	internal := repo.IsInternal
	internalParam := strings.TrimSpace(c.Query("internal"))
	if internalParam != "" {
		internal = internalParam == "true" || internalParam == "1"
	}

	externalDeviceName := strings.TrimSpace(c.Query("external_device_name"))
	if externalDeviceName == "" {
		externalDeviceName = strings.TrimSpace(repo.ExternalDeviceName)
	}

	relDir, err := normalizeBrowseRelativePath(c.Query("dir"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dir: " + err.Error()})
		return
	}

	root := internalRepoRoot
	if !internal {
		deviceName, err := normalizeExternalDeviceName(externalDeviceName)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid external_device_name: " + err.Error()})
			return
		}
		externalDeviceName = deviceName
		root = filepath.Join(externalRepoRoot, filepath.FromSlash(deviceName))
		if !isPathWithinRoot(externalRepoRoot, root) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "external device out of root"})
			return
		}
		info, statErr := os.Stat(root)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				c.JSON(http.StatusNotFound, gin.H{"error": "external device not found"})
				return
			}
			log.Printf("ListRepoPathOptions: stat external root failed id=%s root=%s error=%v", id, root, statErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to read external device"})
			return
		}
		if !info.IsDir() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "external device path is not a directory"})
			return
		}
	}

	target := root
	if relDir != "" {
		target = filepath.Join(root, filepath.FromSlash(relDir))
	}
	if !isPathWithinRoot(root, target) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dir out of root"})
		return
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		log.Printf("ListRepoPathOptions: read dir failed id=%s target=%s error=%v", id, target, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to read directory"})
		return
	}

	folders := make([]folderEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		folders = append(folders, folderEntry{
			Name:  entry.Name(),
			IsDir: true,
			Size:  0,
		})
	}
	sort.Slice(folders, func(i, j int) bool { return folders[i].Name < folders[j].Name })

	log.Printf("ListRepoPathOptions: success id=%s internal=%t dir=%q count=%d", id, internal, relDir, len(folders))
	c.JSON(http.StatusOK, gin.H{
		"dir":                  relDir,
		"internal":             internal,
		"external_device_name": externalDeviceName,
		"entries":              folders,
	})
}

// UpdateRepoPath 更新仓库路径和内部/外部模式
func UpdateRepoPath(c *gin.Context) {
	log.Printf("UpdateRepoPath: start method=%s path=%s remote=%s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	var repo models.Repository
	if err := db.First(&repo, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("UpdateRepoPath: repo not found id=%s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
			return
		}
		log.Printf("UpdateRepoPath: query failed id=%s error=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db query failed: " + err.Error()})
		return
	}

	var req struct {
		RootPath           string `json:"root_path"`
		IsInternal         *bool  `json:"is_internal"`
		ExternalDeviceName string `json:"external_device_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("UpdateRepoPath: invalid request id=%s error=%v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	normalizedPath, err := normalizeRepoRootPath(req.RootPath, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid root_path: " + err.Error()})
		return
	}

	newInternal := repo.IsInternal
	if req.IsInternal != nil {
		newInternal = *req.IsInternal
	}
	newExternalDeviceName := strings.TrimSpace(req.ExternalDeviceName)
	if !newInternal {
		deviceName, deviceErr := normalizeExternalDeviceName(newExternalDeviceName)
		if deviceErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid external_device_name: " + deviceErr.Error()})
			return
		}
		newExternalDeviceName = deviceName
	} else {
		newExternalDeviceName = ""
	}

	oldPath := repo.RootPath
	oldInternal := repo.IsInternal
	oldExternalDevice := repo.ExternalDeviceName

	repo.RootPath = normalizedPath
	repo.IsInternal = newInternal
	repo.ExternalDeviceName = newExternalDeviceName

	if err := db.Save(&repo).Error; err != nil {
		log.Printf("UpdateRepoPath: save failed id=%s error=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db update failed: " + err.Error()})
		return
	}

	if err := BootstrapSingleRepository(repo); err != nil {
		log.Printf("UpdateRepoPath: bootstrap repo metadata failed id=%s error=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sync repo metadata failed: " + err.Error()})
		return
	}

	if err := db.First(&repo, id).Error; err != nil {
		log.Printf("UpdateRepoPath: reload repository failed id=%s error=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "reload repository failed: " + err.Error()})
		return
	}

	if oldPath != repo.RootPath || oldInternal != repo.IsInternal || oldExternalDevice != repo.ExternalDeviceName {
		triggerRepoIncrementalNormalize(repo, "update-path")
	}

	log.Printf("UpdateRepoPath: success id=%s path %q->%q internal %t->%t extDevice %q->%q", id, oldPath, repo.RootPath, oldInternal, repo.IsInternal, oldExternalDevice, repo.ExternalDeviceName)
	c.JSON(http.StatusOK, repo)
}

func normalizeBrowseRelativePath(value string) (string, error) {
	v := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if v == "" || v == "." || v == "/" {
		return "", nil
	}

	v = strings.TrimPrefix(v, "/")
	cleaned := path.Clean(v)
	if cleaned == "." {
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path traversal is not allowed")
	}
	return cleaned, nil
}

func normalizeRepoRootPath(value string, allowEmpty bool) (string, error) {
	v := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if v == "" {
		if allowEmpty {
			return "", nil
		}
		return "", fmt.Errorf("root_path required, use '/' for storage root")
	}

	if v == "/" {
		return "/", nil
	}

	v = strings.TrimPrefix(v, "/")
	cleaned := path.Clean(v)
	if cleaned == "." {
		if allowEmpty {
			return "", nil
		}
		return "", fmt.Errorf("root_path required, use '/' for storage root")
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path traversal is not allowed")
	}
	return cleaned, nil
}

func normalizeExternalDeviceName(value string) (string, error) {
	v := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if v == "" {
		return "", fmt.Errorf("external_device_name required")
	}
	if strings.Contains(v, "/") {
		return "", fmt.Errorf("path separator is not allowed")
	}
	cleaned := path.Clean(v)
	if cleaned == "" || cleaned == "." || cleaned == ".." || strings.Contains(cleaned, "/") {
		return "", fmt.Errorf("invalid device name")
	}
	if cleaned == externalExcludedDir {
		return "", fmt.Errorf("device is not selectable")
	}
	return cleaned, nil
}

func isPathWithinRoot(root string, target string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "")
}

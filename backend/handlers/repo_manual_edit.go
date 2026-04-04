package handlers

import (
	"errors"
	"fmt"
	"io"
	"lazymanga/models"
	"lazymanga/normalization"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type manualEditRepoISORequest struct {
	TargetType string `json:"target_type"`
	NameMode   string `json:"name_mode"`
	ManualName string `json:"manual_name"`
}

func ManualEditRepoISO(c *gin.Context) {
	log.Printf("ManualEditRepoISO: start method=%s path=%s remote=%s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
	repoID := c.Param("id")
	isoID := c.Param("isoId")
	if repoID == "" || isoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id or isoId"})
		return
	}

	var repo models.Repository
	if err := db.First(&repo, repoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("ManualEditRepoISO: repo not found id=%s", repoID)
			c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
			return
		}
		log.Printf("ManualEditRepoISO: query repo failed id=%s error=%v", repoID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db query failed: " + err.Error()})
		return
	}

	repoDB, rootAbs, dbPath, err := openRepoScopedDB(repo)
	if err != nil {
		log.Printf("ManualEditRepoISO: open scoped db failed repo=%s error=%v", repoID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "prepare repo db failed: " + err.Error()})
		return
	}

	info, err := EnsureRepoInfoFromRepository(repoDB, repo)
	if err != nil {
		log.Printf("ManualEditRepoISO: load repo info failed repo=%s error=%v", repoID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "load repo info failed: " + err.Error()})
		return
	}

	var row models.RepoISO
	if err := repoDB.First(&row, isoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("ManualEditRepoISO: iso record not found repo=%s iso=%s", repoID, isoID)
			c.JSON(http.StatusNotFound, gin.H{"error": "repo iso record not found"})
			return
		}
		log.Printf("ManualEditRepoISO: query iso failed repo=%s iso=%s error=%v", repoID, isoID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query repo iso failed: " + err.Error()})
		return
	}

	var req manualEditRepoISORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("ManualEditRepoISO: invalid request repo=%s iso=%s error=%v", repoID, isoID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := normalizeManualEditRepoISORequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sourceAbs, err := resolveRepoISOAbsPath(rootAbs, row.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid source path: " + err.Error()})
		return
	}
	if _, err := os.Stat(sourceAbs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source file not found"})
		return
	}

	currentFileName := strings.TrimSpace(row.FileName)
	if currentFileName == "" {
		currentFileName = filepath.Base(filepath.FromSlash(strings.TrimSpace(row.Path)))
	}
	if currentFileName == "" {
		currentFileName = "unnamed.iso"
	}

	targetFileName, err := decideManualEditTargetFileName(req, row.Path, currentFileName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	isOS, isEntertament := repoISOFlagsForTargetType(req.TargetType)

	if !info.AutoNormalize {
		updates := map[string]interface{}{
			"is_os":          isOS,
			"is_entertament": isEntertament,
		}

		moved := false
		if req.NameMode == "manual" {
			targetAbs, err := buildManualEditTargetAbs(rootAbs, row.Path, "", targetFileName)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rename target path: " + err.Error()})
				return
			}

			_, finalRelPath, renameMoved, err := relocateRepoISOFile(sourceAbs, targetAbs, rootAbs)
			if err != nil {
				log.Printf("ManualEditRepoISO: rename failed repo=%s iso=%s source=%q target=%q error=%v", repoID, isoID, sourceAbs, targetAbs, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "rename file failed: " + err.Error()})
				return
			}

			newFileName := filepath.Base(finalRelPath)
			updates["path"] = finalRelPath
			updates["file_name"] = newFileName
			updates["tags"] = models.ExtractTagsFromFileName(newFileName)
			moved = renameMoved
		}

		if err := repoDB.Model(&models.RepoISO{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			log.Printf("ManualEditRepoISO: update flags failed repo=%s iso=%s error=%v", repoID, isoID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "update repo iso record failed: " + err.Error()})
			return
		}
		if err := repoDB.First(&row, row.ID).Error; err != nil {
			log.Printf("ManualEditRepoISO: reload row failed repo=%s iso=%s error=%v", repoID, isoID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "reload repo iso failed: " + err.Error()})
			return
		}

		log.Printf("ManualEditRepoISO: skipped auto relocation repo=%d iso=%d auto_normalize=%t", repo.ID, row.ID, info.AutoNormalize)
		c.JSON(http.StatusOK, gin.H{
			"message":          "manual edit applied",
			"repo_id":          repo.ID,
			"iso_id":           row.ID,
			"auto_normalize":   info.AutoNormalize,
			"auto_relocate":    false,
			"moved":            moved,
			"relocate_skipped": true,
			"record":           row,
		})
		return
	}

	targetDir, matchedType, matchedKeyword, err := decideManualEditTargetDir(req.TargetType, targetFileName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	targetAbs, err := buildManualEditTargetAbs(rootAbs, row.Path, targetDir, targetFileName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target path: " + err.Error()})
		return
	}

	finalAbs, finalRelPath, moved, err := relocateRepoISOFile(sourceAbs, targetAbs, rootAbs)
	if err != nil {
		log.Printf("ManualEditRepoISO: move failed repo=%s iso=%s source=%q target=%q error=%v", repoID, isoID, sourceAbs, targetAbs, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "move file failed: " + err.Error()})
		return
	}

	newFileName := filepath.Base(finalRelPath)
	updates := map[string]interface{}{
		"path":           finalRelPath,
		"file_name":      newFileName,
		"tags":           models.ExtractTagsFromFileName(newFileName),
		"is_os":          isOS,
		"is_entertament": isEntertament,
	}
	if err := repoDB.Model(&models.RepoISO{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
		log.Printf("ManualEditRepoISO: update row failed repo=%s iso=%s error=%v", repoID, isoID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update repo iso failed: " + err.Error()})
		return
	}

	if err := repoDB.First(&row, row.ID).Error; err != nil {
		log.Printf("ManualEditRepoISO: reload row failed repo=%s iso=%s error=%v", repoID, isoID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "reload repo iso failed: " + err.Error()})
		return
	}

	log.Printf("ManualEditRepoISO: done repo=%d iso=%d type=%s mode=%s moved=%t source=%q final=%q db=%q abs=%q matchType=%q keyword=%q", repo.ID, row.ID, req.TargetType, req.NameMode, moved, sourceAbs, finalRelPath, dbPath, finalAbs, matchedType, matchedKeyword)
	c.JSON(http.StatusOK, gin.H{
		"message":            "manual edit applied",
		"repo_id":            repo.ID,
		"iso_id":             row.ID,
		"auto_normalize":     info.AutoNormalize,
		"target_type":        req.TargetType,
		"name_mode":          req.NameMode,
		"target_dir":         targetDir,
		"os_matched_type":    matchedType,
		"os_matched_keyword": matchedKeyword,
		"moved":              moved,
		"record":             row,
	})
}

func normalizeManualEditRepoISORequest(req *manualEditRepoISORequest) error {
	req.TargetType = strings.ToLower(strings.TrimSpace(req.TargetType))
	req.NameMode = strings.ToLower(strings.TrimSpace(req.NameMode))
	req.ManualName = strings.TrimSpace(req.ManualName)

	if req.TargetType != "os" && req.TargetType != "entertainment" && req.TargetType != "others" {
		return fmt.Errorf("target_type must be os, entertainment or others")
	}
	if req.NameMode == "" {
		req.NameMode = "auto"
	}
	if req.NameMode != "auto" && req.NameMode != "manual" {
		return fmt.Errorf("name_mode must be auto or manual")
	}
	if req.NameMode == "manual" && req.ManualName == "" {
		return fmt.Errorf("manual_name required when name_mode is manual")
	}
	if req.NameMode == "manual" {
		manualName := sanitizeInputFileName(req.ManualName)
		if !strings.EqualFold(filepath.Ext(manualName), ".iso") {
			return fmt.Errorf("manual_name must end with .iso")
		}
	}
	return nil
}

func repoISOFlagsForTargetType(targetType string) (bool, bool) {
	if targetType == "entertainment" {
		return false, true
	}
	if targetType == "others" {
		return false, false
	}
	return true, false
}

func decideManualEditTargetFileName(req manualEditRepoISORequest, currentRelPath string, currentFileName string) (string, error) {
	currentFileName = sanitizeInputFileName(currentFileName)
	if currentFileName == "" {
		return "", fmt.Errorf("current file name is empty")
	}

	if req.NameMode == "manual" {
		manualName := sanitizeInputFileName(req.ManualName)
		if manualName == "" {
			return "", fmt.Errorf("manual_name is invalid")
		}
		if !strings.EqualFold(filepath.Ext(manualName), ".iso") {
			return "", fmt.Errorf("manual_name must end with .iso")
		}
		return manualName, nil
	}

	targetFileName := currentFileName
	if req.TargetType == "entertainment" {
		if prefix := entertainmentPathPrefixForAuto(currentRelPath); prefix != "" {
			targetFileName = prefix + "_" + targetFileName
		}
	}
	return targetFileName, nil
}

func decideManualEditTargetDir(targetType string, targetFileName string) (string, string, string, error) {
	if targetType == "entertainment" {
		return "Entertainment", "", "", nil
	}

	if targetType == "os" {
		if matched, ok := normalization.GuessOSRuleByFileName(targetFileName); ok {
			return matched.TargetDir, matched.TypeName, matched.Keyword, nil
		}
		return "OS", "", "", nil
	}

	if targetType == "others" {
		return "", "", "", nil
	}

	return "", "", "", fmt.Errorf("unsupported target_type")
}

func buildManualEditTargetAbs(rootAbs string, currentRelPath string, targetDir string, targetFileName string) (string, error) {
	cleanName := sanitizeInputFileName(targetFileName)
	if cleanName == "" {
		return "", fmt.Errorf("target file name is invalid")
	}

	if strings.TrimSpace(targetDir) != "" {
		targetAbs := filepath.Join(rootAbs, filepath.FromSlash(targetDir), cleanName)
		if !isPathWithinRoot(rootAbs, targetAbs) {
			return "", fmt.Errorf("target path out of repo root")
		}
		return targetAbs, nil
	}

	normalized := strings.Trim(strings.ReplaceAll(currentRelPath, "\\", "/"), "/")
	currentDir := filepath.Dir(filepath.FromSlash(normalized))
	if currentDir == "." {
		currentDir = ""
	}

	targetAbs := filepath.Join(rootAbs, currentDir, cleanName)
	if !isPathWithinRoot(rootAbs, targetAbs) {
		return "", fmt.Errorf("target path out of repo root")
	}
	return targetAbs, nil
}

func entertainmentPathPrefixForAuto(currentRelPath string) string {
	normalized := strings.Trim(strings.ReplaceAll(currentRelPath, "\\", "/"), "/")
	if normalized == "" {
		return ""
	}
	parts := strings.Split(normalized, "/")
	if len(parts) <= 1 {
		return ""
	}
	dirs := parts[:len(parts)-1]
	if len(dirs) > 0 && strings.EqualFold(dirs[0], "entertainment") {
		dirs = dirs[1:]
	}
	if len(dirs) == 0 {
		return ""
	}
	prefix := strings.Join(dirs, "_")
	prefix = strings.ReplaceAll(prefix, " ", "_")
	return strings.Trim(prefix, "_")
}

func sanitizeInputFileName(name string) string {
	v := strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if v == "" {
		return ""
	}
	base := filepath.Base(v)
	if base == "." || base == ".." {
		return ""
	}
	return base
}

func resolveRepoISOAbsPath(rootAbs string, relPath string) (string, error) {
	normalized := strings.Trim(strings.ReplaceAll(relPath, "\\", "/"), "/")
	if normalized == "" {
		return "", fmt.Errorf("empty repo iso path")
	}
	absPath := filepath.Join(rootAbs, filepath.FromSlash(normalized))
	if !isPathWithinRoot(rootAbs, absPath) {
		return "", fmt.Errorf("path out of root")
	}
	return absPath, nil
}

func relocateRepoISOFile(sourceAbs string, targetAbs string, rootAbs string) (string, string, bool, error) {
	if sameRepoISOPath(sourceAbs, targetAbs) {
		rel, err := filepath.Rel(rootAbs, targetAbs)
		if err != nil {
			return "", "", false, err
		}
		return targetAbs, filepath.ToSlash(rel), false, nil
	}

	if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
		return "", "", false, err
	}

	finalTargetAbs, err := findRepoISOAvailableTargetPath(targetAbs)
	if err != nil {
		return "", "", false, err
	}

	if err := moveRepoISOFileWithFallback(sourceAbs, finalTargetAbs); err != nil {
		return "", "", false, err
	}

	rel, err := filepath.Rel(rootAbs, finalTargetAbs)
	if err != nil {
		return "", "", true, err
	}
	return finalTargetAbs, filepath.ToSlash(rel), true, nil
}

func findRepoISOAvailableTargetPath(targetAbs string) (string, error) {
	if _, err := os.Stat(targetAbs); err != nil {
		if os.IsNotExist(err) {
			return targetAbs, nil
		}
		return "", err
	}

	dir := filepath.Dir(targetAbs)
	fileName := filepath.Base(targetAbs)
	ext := filepath.Ext(fileName)
	base := strings.TrimSuffix(fileName, ext)

	for i := 1; i < 10000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, i, ext))
		if _, err := os.Stat(candidate); err != nil {
			if os.IsNotExist(err) {
				return candidate, nil
			}
			return "", err
		}
	}

	return "", fmt.Errorf("unable to allocate unique target for %q", targetAbs)
}

func moveRepoISOFileWithFallback(sourceAbs string, targetAbs string) error {
	if err := os.Rename(sourceAbs, targetAbs); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return err
	}

	if err := copyRepoISOFileWithMode(sourceAbs, targetAbs); err != nil {
		return err
	}
	return os.Remove(sourceAbs)
}

func copyRepoISOFileWithMode(sourceAbs string, targetAbs string) error {
	srcInfo, err := os.Stat(sourceAbs)
	if err != nil {
		return err
	}

	src, err := os.Open(sourceAbs)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(targetAbs, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return dst.Sync()
}

func sameRepoISOPath(a string, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

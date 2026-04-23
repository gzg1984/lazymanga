package handlers

import (
	"encoding/json"
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

const (
	repoManualAddedFilesSubdir = "manual_added"
	repoManualAddedDirsSubdir  = "manual_added_dirs"
	repoISOItemKindArchive     = "archive"
)

type repoImportPlan struct {
	TargetAbs    string
	Copied       bool
	ItemKind     string
	TargetSubdir string
}

func normalizeCreateRepoISOPathKind(v string) string {
	kind := strings.TrimSpace(strings.ToLower(v))
	if kind == "directory" || kind == "dir" || kind == "folder" {
		return "directory"
	}
	return "file"
}

func parseRepoISOMetadataJSONMap(raw string) map[string]any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "{}" {
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil
	}
	if len(decoded) == 0 {
		return nil
	}
	return decoded
}

func detectRepoISOItemKind(row models.RepoISO, settings repoTypeSettings) string {
	if metadata := parseRepoISOMetadataJSONMap(row.MetadataJSON); metadata != nil {
		if kind, ok := metadata["item_kind"].(string); ok && strings.TrimSpace(strings.ToLower(kind)) == repoISOItemKindArchive {
			return repoISOItemKindArchive
		}
	}
	if !row.IsDirectory && isArchiveFileForSettings(row.FileName, settings) {
		return repoISOItemKindArchive
	}
	return ""
}

func isArchiveFileForSettings(fileName string, settings repoTypeSettings) bool {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(fileName)))
	if ext == "" {
		return false
	}
	for _, part := range strings.Split(canonicalizeArchiveExtensionsCSV(settings.ArchiveExtensions), ",") {
		if strings.TrimSpace(part) == ext {
			return true
		}
	}
	return false
}

func resolveRepoManualImportSubdir(pathKind string, fileName string, settings repoTypeSettings) (string, string, error) {
	if pathKind == "directory" {
		if settings.MaterializedSubdir != defaultMaterializedSubdir {
			return filepath.ToSlash(filepath.Join(settings.MaterializedSubdir, repoManualAddedDirsSubdir)), "", nil
		}
		return repoManualAddedDirsSubdir, "", nil
	}
	if isArchiveFileForSettings(fileName, settings) {
		archiveSubdir, _, err := validateArchiveSettings(settings.ArchiveSubdir, settings.MaterializedSubdir)
		if err != nil {
			return "", "", err
		}
		return archiveSubdir, repoISOItemKindArchive, nil
	}
	if settings.MaterializedSubdir != defaultMaterializedSubdir {
		return filepath.ToSlash(filepath.Join(settings.MaterializedSubdir, repoManualAddedFilesSubdir)), "", nil
	}
	return repoManualAddedFilesSubdir, "", nil
}

func planRepoImport(rootAbs string, sourceAbs string, pathKind string, settings repoTypeSettings) (repoImportPlan, error) {
	plan := repoImportPlan{TargetAbs: sourceAbs}
	if isPathWithinRoot(rootAbs, sourceAbs) {
		if pathKind != "directory" && isArchiveFileForSettings(filepath.Base(sourceAbs), settings) {
			plan.ItemKind = repoISOItemKindArchive
		}
		return plan, nil
	}
	subdir, itemKind, err := resolveRepoManualImportSubdir(pathKind, filepath.Base(sourceAbs), settings)
	if err != nil {
		return repoImportPlan{}, err
	}
	targetAbs := filepath.Join(rootAbs, filepath.FromSlash(subdir), filepath.Base(sourceAbs))
	if !isPathWithinRoot(rootAbs, targetAbs) {
		return repoImportPlan{}, fmt.Errorf("target path out of repo root")
	}
	plan.TargetAbs = targetAbs
	plan.Copied = true
	plan.ItemKind = itemKind
	plan.TargetSubdir = subdir
	return plan, nil
}

func planRepoTransfer(rootAbs string, sourceAbs string, row models.RepoISO, settings repoTypeSettings) (repoImportPlan, error) {
	pathKind := "file"
	if row.IsDirectory {
		pathKind = "directory"
	}
	if isPathWithinRoot(rootAbs, sourceAbs) {
		plan := repoImportPlan{TargetAbs: sourceAbs, ItemKind: detectRepoISOItemKind(row, settings)}
		return plan, nil
	}
	subdir, itemKind, err := resolveRepoManualImportSubdir(pathKind, row.FileName, settings)
	if err != nil {
		return repoImportPlan{}, err
	}
	if itemKind == "" {
		itemKind = detectRepoISOItemKind(row, settings)
	}
	targetAbs := filepath.Join(rootAbs, filepath.FromSlash(subdir), row.FileName)
	if !isPathWithinRoot(rootAbs, targetAbs) {
		return repoImportPlan{}, fmt.Errorf("target path out of repo root")
	}
	return repoImportPlan{
		TargetAbs:    targetAbs,
		Copied:       true,
		ItemKind:     itemKind,
		TargetSubdir: subdir,
	}, nil
}

func importRepoFile(rootAbs string, sourceAbs string, mode os.FileMode, settings repoTypeSettings) (repoImportPlan, error) {
	plan, err := planRepoImport(rootAbs, sourceAbs, "file", settings)
	if err != nil {
		return repoImportPlan{}, err
	}
	if !plan.Copied {
		return plan, nil
	}
	if err := os.MkdirAll(filepath.Dir(plan.TargetAbs), 0o755); err != nil {
		return repoImportPlan{}, fmt.Errorf("prepare target folder failed: %w", err)
	}
	finalTargetAbs, err := findRepoISOAvailableTargetPath(plan.TargetAbs)
	if err != nil {
		return repoImportPlan{}, fmt.Errorf("allocate target path failed: %w", err)
	}
	if err := copyFile(sourceAbs, finalTargetAbs, mode); err != nil {
		return repoImportPlan{}, fmt.Errorf("copy source file failed: %w", err)
	}
	plan.TargetAbs = finalTargetAbs
	return plan, nil
}

func buildImportedFileMetadataJSON(itemKind string, relPath string, fileName string, sourcePath string) (string, error) {
	return buildImportedFileMetadataJSONFromExisting("", itemKind, relPath, fileName, sourcePath)
}

func buildImportedFileMetadataJSONFromExisting(existingRaw string, itemKind string, relPath string, fileName string, sourcePath string) (string, error) {
	if itemKind != repoISOItemKindArchive {
		return strings.TrimSpace(existingRaw), nil
	}
	payload := parseRepoISOMetadataJSONMap(existingRaw)
	if payload == nil {
		payload = map[string]any{}
	}
	payload["item_kind"] = repoISOItemKindArchive
	payload["archive_format"] = strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
	if payload["lifecycle"] == nil || strings.TrimSpace(fmt.Sprint(payload["lifecycle"])) == "" {
		payload["lifecycle"] = "ingested"
	}
	payload["archive_storage_path"] = relPath
	if payload["source_path"] == nil || strings.TrimSpace(fmt.Sprint(payload["source_path"])) == "" {
		if sanitizedSourcePath := sanitizeStoredSourceRelativePath(sourcePath); sanitizedSourcePath != "" {
			payload["source_path"] = sanitizedSourcePath
		}
	}
	if payload["original_name"] == nil || strings.TrimSpace(fmt.Sprint(payload["original_name"])) == "" {
		originalName := sanitizeStoredSourcePathSegment(fileName)
		if sanitizedSourcePath := sanitizeStoredSourceRelativePath(sourcePath); sanitizedSourcePath != "" {
			if sourceBase := sanitizeStoredSourcePathSegment(filepath.Base(strings.TrimRight(sanitizedSourcePath, "/"))); sourceBase != "" {
				originalName = sourceBase
			}
		}
		if originalName != "" {
			payload["original_name"] = originalName
		}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
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
	repoInfo, err := EnsureRepoInfoFromRepository(repoDB, repo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "load repo info failed: " + err.Error()})
		return
	}
	_, _, _, effectiveSettings, _, err := resolveEffectiveRepoTypeSettings(repoInfo, repo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "resolve repo type settings failed: " + err.Error()})
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

		targetAbs, copied, err := importRepoDirectory(rootAbs, sourceAbs, effectiveSettings)
		if err != nil {
			msg := err.Error()
			status := http.StatusInternalServerError
			if strings.Contains(msg, "already exists") {
				status = http.StatusConflict
			}
			c.JSON(status, gin.H{"error": msg})
			return
		}

		targetRel, err := filepath.Rel(rootAbs, targetAbs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "resolve imported directory path failed: " + err.Error()})
			return
		}
		targetRel = filepath.ToSlash(targetRel)

		normalizeResult, err := runRepoIncrementalNormalize(repo, targetRel)
		if err != nil {
			msg := err.Error()
			status := http.StatusInternalServerError
			if strings.HasPrefix(msg, "prepare repo db failed: ") {
				status = http.StatusBadRequest
			}
			c.JSON(status, gin.H{"error": msg})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":                      "repo directory imported and scoped scan completed",
			"repo_id":                      repo.ID,
			"source":                       sourceRel,
			"copied":                       copied,
			"path_kind":                    "directory",
			"target_path":                  targetRel,
			"scan_scope":                   normalizeResult.ScanScope,
			"scan_triggered":               true,
			"scanned_iso_count":            normalizeResult.ScannedISOCount,
			"existing_iso_count":           normalizeResult.ExistingISOCount,
			"inserted_new_iso_count":       normalizeResult.InsertedNewISOCount,
			"existing_missing_total_count": normalizeResult.ExistingMissingTotalCount,
		})
		return
	}
	if pathKind == "directory" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source path is not a directory"})
		return
	}

	scanSpec := normalization.GetRuleBookScanSpecForRepo(repo.ID, repoDB)
	if !scanSpec.ShouldScanFile(filepath.Base(sourceAbs)) {
		allowed := strings.Join(scanSpec.Extensions, ", ")
		if allowed == "" {
			allowed = ".iso"
		}
		if allowed == "*" {
			allowed = "all files"
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "current rulebook scan config does not include this file type, allowed: " + allowed})
		return
	}

	importPlan, err := importRepoFile(rootAbs, sourceAbs, srcInfo.Mode(), effectiveSettings)
	if err != nil {
		msg := err.Error()
		status := http.StatusInternalServerError
		if strings.Contains(msg, "target path out of repo root") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": msg})
		return
	}
	targetAbs := importPlan.TargetAbs

	relPath, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "resolve repo relative path failed: " + err.Error()})
		return
	}
	relPath = filepath.ToSlash(relPath)

	var exists models.RepoISO
	if err := repoDB.Where("path = ?", relPath).First(&exists).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "repository entry already exists", "id": exists.ID})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query existing repository entry failed: " + err.Error()})
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
	metadataJSON, err := buildImportedFileMetadataJSON(importPlan.ItemKind, relPath, row.FileName, sourceRel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "build metadata failed: " + err.Error()})
		return
	}
	row.MetadataJSON = metadataJSON
	if err := repoDB.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create repository entry failed: " + err.Error()})
		return
	}

	normalization.StartAsyncPostIndexNormalization(repo.ID, repoDB, rootAbs, []models.RepoISO{row})
	c.JSON(http.StatusCreated, gin.H{
		"message":      "repository entry added",
		"repo_id":      repo.ID,
		"source":       sourceRel,
		"copied":       importPlan.Copied,
		"import_kind":  importPlan.ItemKind,
		"target_subdir": importPlan.TargetSubdir,
		"repo_iso":     row,
	})
}

func importRepoDirectory(rootAbs string, sourceAbs string, settings repoTypeSettings) (string, bool, error) {
	plan, err := planRepoImport(rootAbs, sourceAbs, "directory", settings)
	if err != nil {
		return "", false, err
	}
	if !plan.Copied {
		return plan.TargetAbs, false, nil
	}

	if err := os.MkdirAll(filepath.Dir(plan.TargetAbs), 0o755); err != nil {
		return "", false, fmt.Errorf("prepare target folder failed: %w", err)
	}

	finalTargetAbs, err := findRepoISOAvailableTargetPath(plan.TargetAbs)
	if err != nil {
		return "", false, fmt.Errorf("allocate target path failed: %w", err)
	}
	if err := copyDirectoryRecursive(sourceAbs, finalTargetAbs); err != nil {
		return "", false, fmt.Errorf("copy source directory failed: %w", err)
	}

	return finalTargetAbs, true, nil
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

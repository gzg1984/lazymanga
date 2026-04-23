package handlers

import (
	"errors"
	"fmt"
	"lazymanga/models"
	"lazymanga/normalization"
	"lazymanga/normalization/rulebook"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type repoIncrementalNormalizeResult struct {
	RootAbs                       string
	DBPath                        string
	ScanScope                     string
	ScannedISOCount               int
	ExistingISOCount              int
	ExistingMissingMarkedCount    int
	ExistingMissingRecoveredCount int
	ExistingMissingTotalCount     int
	ExistingMissingMetaCount      int
	InsertedNewISOCount           int
	NewRecordsAsyncSteps          []string
	ExistingAsyncSteps            []string
}

func runRepoIncrementalNormalize(repo models.Repository, scanScope string) (repoIncrementalNormalizeResult, error) {
	result := repoIncrementalNormalizeResult{
		NewRecordsAsyncSteps: normalization.DefaultStepNames(),
		ExistingAsyncSteps:   []string{"directory-transform", "file-size-backfill", "md5-backfill"},
	}

	normalizedScope, err := normalizeRepoScanScope(scanScope)
	if err != nil {
		return result, fmt.Errorf("invalid scan scope: %w", err)
	}
	if shouldBlockFullRepoScan(repo, normalizedScope) {
		return result, fmt.Errorf("full repo scan is disabled for basic repositories")
	}

	repoDB, rootAbs, dbPath, err := openRepoScopedDB(repo)
	if err != nil {
		return result, fmt.Errorf("prepare repo db failed: %w", err)
	}
	result.RootAbs = rootAbs
	result.DBPath = dbPath
	repoInfo, err := EnsureRepoInfoFromRepository(repoDB, repo)
	if err != nil {
		return result, fmt.Errorf("ensure repo info failed: %w", err)
	}
	_, _, _, effectiveSettings, _, err := resolveEffectiveRepoTypeSettings(repoInfo, repo)
	if err != nil {
		return result, fmt.Errorf("resolve repo type settings failed: %w", err)
	}
	archivePaths, err := resolveRepoArchivePaths(rootAbs, effectiveSettings)
	if err != nil {
		return result, fmt.Errorf("resolve archive paths failed: %w", err)
	}
	effectiveScope := normalizedScope
	if effectiveScope == "" && archivePaths.MaterializedSubdir != defaultMaterializedSubdir {
		effectiveScope = archivePaths.MaterializedSubdir
	}
	result.ScanScope = effectiveScope

	scanSpec := normalization.GetRuleBookScanSpecForRepo(repo.ID, repoDB)
	log.Printf("runRepoIncrementalNormalize: scan spec repo=%d scope=%q extensions=%v include_no_ext=%t dir_rules=%d", repo.ID, effectiveScope, scanSpec.Extensions, scanSpec.IncludeFilesWithoutExt, len(scanSpec.DirectoryRules))

	records, err := collectRepoISORecordsByScanSpec(rootAbs, scanSpec, effectiveScope, archivePaths.ExcludeRootAbsPaths...)
	if err != nil {
		return result, fmt.Errorf("scan repo path failed: %w", err)
	}
	result.ScannedISOCount = len(records)

	var existingRows []models.RepoISO
	existingQuery := repoDB
	if effectiveScope != "" {
		existingQuery = existingQuery.Where("path = ? OR path LIKE ?", effectiveScope, effectiveScope+"/%")
	}
	if err := existingQuery.Find(&existingRows).Error; err != nil {
		return result, fmt.Errorf("query repoisos failed: %w", err)
	}
	result.ExistingISOCount = len(existingRows)

	scannedByPath := make(map[string]models.RepoISO, len(records))
	for _, scanned := range records {
		scannedByPath[scanned.Path] = scanned
	}
	coveredScannedPaths := make(map[string]struct{}, len(records))

	for i := range existingRows {
		row := &existingRows[i]
		prevMissing := row.IsMissing
		originalPath := row.Path

		if scanned, ok := scannedByPath[row.Path]; ok && row.IsDirectory != scanned.IsDirectory {
			if err := repoDB.Model(&models.RepoISO{}).Where("id = ?", row.ID).Update("is_directory", scanned.IsDirectory).Error; err != nil {
				return result, fmt.Errorf("update is_directory failed: %w", err)
			}
			row.IsDirectory = scanned.IsDirectory
		}

		missing := false
		absPath, pathErr := resolveRepoISOAbsPath(rootAbs, row.Path)
		if pathErr != nil {
			missing = true
			log.Printf("runRepoIncrementalNormalize: invalid row path treated as missing repo=%d row_id=%d path=%q error=%v", repo.ID, row.ID, row.Path, pathErr)
		} else {
			info, statErr := os.Stat(absPath)
			if statErr != nil {
				if os.IsNotExist(statErr) {
					missing = true
				} else {
					return result, fmt.Errorf("stat existing repo iso failed: %w", statErr)
				}
			} else if row.IsDirectory {
				missing = !info.IsDir()
			} else if info.IsDir() {
				missing = true
			}
		}

		if err := updateRepoISOMissingFlag(repoDB, row, missing); err != nil {
			return result, fmt.Errorf("update missing flag failed: %w", err)
		}

		if _, ok := scannedByPath[originalPath]; ok {
			coveredScannedPaths[originalPath] = struct{}{}
		}
		if !missing && row.IsDirectory {
			coveredPaths, err := refreshExistingDirectoryRepoISO(repo.ID, repoDB, rootAbs, row)
			if err != nil {
				return result, fmt.Errorf("refresh existing directory repo iso failed: %w", err)
			}
			for _, coveredPath := range coveredPaths {
				coveredScannedPaths[coveredPath] = struct{}{}
			}
		}

		if !prevMissing && missing {
			result.ExistingMissingMarkedCount++
		}
		if prevMissing && !missing {
			result.ExistingMissingRecoveredCount++
		}
	}

	existingByPath := make(map[string]models.RepoISO, len(existingRows))
	for _, row := range existingRows {
		existingByPath[row.Path] = row
	}

	newRecords := make([]models.RepoISO, 0)
	for _, scanned := range records {
		if _, ok := coveredScannedPaths[scanned.Path]; ok {
			continue
		}
		if _, ok := existingByPath[scanned.Path]; ok {
			continue
		}
		newRecords = append(newRecords, scanned)
	}

	if len(newRecords) > 0 {
		if err := repoDB.Create(&newRecords).Error; err != nil {
			return result, fmt.Errorf("insert new repoisos failed: %w", err)
		}
		result.InsertedNewISOCount = len(newRecords)
	}

	missingMetaRows := make([]models.RepoISO, 0)
	for _, row := range existingRows {
		if row.IsMissing {
			result.ExistingMissingTotalCount++
			continue
		}
		needsSize := row.SizeBytes <= 0
		needsMD5 := !row.IsDirectory && strings.TrimSpace(row.MD5) == ""
		needsDirectoryMetadata := false
		if needsSize || needsMD5 || needsDirectoryMetadata {
			missingMetaRows = append(missingMetaRows, row)
		}
	}
	result.ExistingMissingMetaCount = len(missingMetaRows)

	if len(newRecords) > 0 {
		normalization.StartAsyncPostIndexNormalization(repo.ID, repoDB, rootAbs, newRecords)
	}
	if len(missingMetaRows) > 0 {
		normalization.StartAsyncMetadataBackfill(repo.ID, repoDB, rootAbs, missingMetaRows)
	}

	return result, nil
}

func refreshExistingDirectoryRepoISO(repoID uint, repoDB *gorm.DB, rootAbs string, row *models.RepoISO) ([]string, error) {
	if row == nil || row.IsMissing || !row.IsDirectory {
		return nil, nil
	}
	originalPath := row.Path
	pathMoved, _, err := refreshDirectoryRecordMetadata(repoID, repoDB, rootAbs, row)
	if err != nil {
		return nil, err
	}
	return coveredScannedPathsForExistingRow(originalPath, row, pathMoved), nil
}

func coveredScannedPathsForExistingRow(originalPath string, row *models.RepoISO, pathMoved bool) []string {
	paths := make([]string, 0, 2)
	seen := make(map[string]struct{}, 2)
	for _, candidate := range []string{originalPath} {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		paths = append(paths, trimmed)
	}
	if pathMoved && row != nil {
		trimmed := strings.TrimSpace(row.Path)
		if trimmed != "" {
			if _, ok := seen[trimmed]; !ok {
				paths = append(paths, trimmed)
			}
		}
	}
	return paths
}

func triggerRepoIncrementalNormalize(repo models.Repository, reason string, scanScopes ...string) {
	if strings.TrimSpace(repo.RootPath) == "" {
		return
	}

	scanScope := ""
	if len(scanScopes) > 0 {
		scanScope = scanScopes[0]
	}

	if shouldBlockFullRepoScan(repo, scanScope) {
		log.Printf("triggerRepoIncrementalNormalize: skipped reason=%s repo=%d basic=%t scope=%q detail=full scan disabled for basic repository", reason, repo.ID, repo.Basic, scanScope)
		return
	}

	go func(r models.Repository, scope string) {
		startedAt := time.Now()
		result, err := runRepoIncrementalNormalize(r, scope)
		if err != nil {
			log.Printf("triggerRepoIncrementalNormalize: failed reason=%s repo=%d scope=%q error=%v", reason, r.ID, scope, err)
			return
		}
		log.Printf("triggerRepoIncrementalNormalize: done reason=%s repo=%d scope=%q root=%q db=%q scanned=%d inserted=%d existing=%d missing_marked=%d missing_recovered=%d missing_total=%d missing_meta=%d elapsed=%s", reason, r.ID, result.ScanScope, result.RootAbs, result.DBPath, result.ScannedISOCount, result.InsertedNewISOCount, result.ExistingISOCount, result.ExistingMissingMarkedCount, result.ExistingMissingRecoveredCount, result.ExistingMissingTotalCount, result.ExistingMissingMetaCount, time.Since(startedAt).Truncate(time.Millisecond))
	}(repo, scanScope)
}

// ForceNormalizeRepoIncremental scans current repo root and inserts only new ISO rows.
// New rows run full async normalization (relocation + size + md5).
// Existing rows only backfill missing metadata (size/md5) asynchronously.
func ForceNormalizeRepoIncremental(c *gin.Context) {
	startedAt := time.Now()
	log.Printf("ForceNormalizeRepoIncremental: start method=%s path=%s remote=%s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	var repo models.Repository
	if err := db.First(&repo, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("ForceNormalizeRepoIncremental: repo not found id=%s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
			return
		}
		log.Printf("ForceNormalizeRepoIncremental: query failed id=%s error=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db query failed: " + err.Error()})
		return
	}

	result, err := runRepoIncrementalNormalize(repo, "")
	if err != nil {
		status := http.StatusInternalServerError
		if strings.HasPrefix(err.Error(), "prepare repo db failed: ") {
			status = http.StatusBadRequest
		} else if strings.HasPrefix(err.Error(), "full repo scan is disabled for basic repositories") {
			status = http.StatusForbidden
		}
		log.Printf("ForceNormalizeRepoIncremental: failed repo=%d error=%v", repo.ID, err)
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	log.Printf("ForceNormalizeRepoIncremental: done repo=%d root=%q db=%q scanned=%d inserted=%d existing=%d missing_marked=%d missing_recovered=%d missing_total=%d missing_meta=%d elapsed=%s", repo.ID, result.RootAbs, result.DBPath, result.ScannedISOCount, result.InsertedNewISOCount, result.ExistingISOCount, result.ExistingMissingMarkedCount, result.ExistingMissingRecoveredCount, result.ExistingMissingTotalCount, result.ExistingMissingMetaCount, time.Since(startedAt).Truncate(time.Millisecond))
	c.JSON(http.StatusOK, gin.H{
		"message":                          "incremental normalize triggered",
		"repo_id":                          repo.ID,
		"root_path":                        result.RootAbs,
		"db_path":                          result.DBPath,
		"scanned_iso_count":                result.ScannedISOCount,
		"existing_iso_count":               result.ExistingISOCount,
		"existing_missing_marked_count":    result.ExistingMissingMarkedCount,
		"existing_missing_recovered_count": result.ExistingMissingRecoveredCount,
		"existing_missing_total_count":     result.ExistingMissingTotalCount,
		"inserted_new_iso_count":           result.InsertedNewISOCount,
		"existing_missing_meta_count":      result.ExistingMissingMetaCount,
		"new_records_async_steps":          result.NewRecordsAsyncSteps,
		"existing_async_steps":             result.ExistingAsyncSteps,
		"normalize_async":                  true,
	})
}

// ForceNormalizeRepo scans all files matched by the current rule book and rebuilds repoisos table in repo-local DB.
func ForceNormalizeRepo(c *gin.Context) {
	startedAt := time.Now()
	log.Printf("ForceNormalizeRepo: start method=%s path=%s remote=%s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	var repo models.Repository
	if err := db.First(&repo, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("ForceNormalizeRepo: repo not found id=%s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
			return
		}
		log.Printf("ForceNormalizeRepo: query failed id=%s error=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db query failed: " + err.Error()})
		return
	}

	log.Printf("ForceNormalizeRepo: repo loaded id=%d name=%q internal=%t root=%q db_file=%q", repo.ID, repo.Name, repo.IsInternal, repo.RootPath, repo.DBFile)

	if shouldBlockFullRepoScan(repo, "") {
		c.JSON(http.StatusForbidden, gin.H{"error": "full repo scan is disabled for basic repositories"})
		return
	}

	repoDB, rootAbs, dbPath, err := openRepoScopedDB(repo)
	if err != nil {
		log.Printf("ForceNormalizeRepo: open scoped db failed id=%s error=%v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "prepare repo db failed: " + err.Error()})
		return
	}
	log.Printf("ForceNormalizeRepo: scoped db ready id=%d root_abs=%q db_path=%q", repo.ID, rootAbs, dbPath)

	collectStartedAt := time.Now()
	log.Printf("ForceNormalizeRepo: collect stage start id=%d root=%q", repo.ID, rootAbs)
	scanSpec := normalization.GetRuleBookScanSpecForRepo(repo.ID, repoDB)
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
	archivePaths, err := resolveRepoArchivePaths(rootAbs, effectiveSettings)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resolve archive paths failed: " + err.Error()})
		return
	}
	effectiveScope := ""
	if archivePaths.MaterializedSubdir != defaultMaterializedSubdir {
		effectiveScope = archivePaths.MaterializedSubdir
	}
	log.Printf("ForceNormalizeRepo: scan spec repo=%d scope=%q extensions=%v include_no_ext=%t dir_rules=%d", repo.ID, effectiveScope, scanSpec.Extensions, scanSpec.IncludeFilesWithoutExt, len(scanSpec.DirectoryRules))

	records, err := collectRepoISORecordsByScanSpec(rootAbs, scanSpec, effectiveScope, archivePaths.ExcludeRootAbsPaths...)
	if err != nil {
		log.Printf("ForceNormalizeRepo: scan failed id=%s root=%s error=%v", id, rootAbs, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "scan repo path failed: " + err.Error()})
		return
	}
	log.Printf("ForceNormalizeRepo: collect stage done id=%d records=%d elapsed=%s", repo.ID, len(records), time.Since(collectStartedAt).Truncate(time.Millisecond))

	rebuildStartedAt := time.Now()
	log.Printf("ForceNormalizeRepo: rebuild stage start id=%d db=%q records=%d", repo.ID, dbPath, len(records))
	if err := rebuildRepoISOIndex(repoDB, records); err != nil {
		log.Printf("ForceNormalizeRepo: rebuild index failed id=%s db=%s error=%v", id, dbPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "rebuild repoisos failed: " + err.Error()})
		return
	}
	log.Printf("ForceNormalizeRepo: rebuild stage done id=%d elapsed=%s", repo.ID, time.Since(rebuildStartedAt).Truncate(time.Millisecond))

	normalizeSteps := normalization.DefaultStepNames()
	normalization.StartAsyncPostIndexNormalization(repo.ID, repoDB, rootAbs, records)

	log.Printf("ForceNormalizeRepo: done id=%d name=%q root=%q db=%q indexed=%d elapsed=%s", repo.ID, repo.Name, rootAbs, dbPath, len(records), time.Since(startedAt).Truncate(time.Millisecond))
	c.JSON(http.StatusOK, gin.H{
		"message":           "normalize completed",
		"repo_id":           repo.ID,
		"root_path":         rootAbs,
		"db_path":           dbPath,
		"iso_count":         len(records),
		"normalize_async":   true,
		"normalize_pending": len(records),
		"normalize_steps":   normalizeSteps,
		"table_name":        "repoisos",
	})
}

func collectRepoISORecordsByScanSpec(rootAbs string, scanSpec rulebook.ScanSpec, scanScope string, excludeRootAbsPaths ...string) ([]models.RepoISO, error) {
	startedAt := time.Now()
	normalizedScope, err := normalizeRepoScanScope(scanScope)
	if err != nil {
		return nil, err
	}

	scanRootAbs := rootAbs
	if normalizedScope != "" {
		scanRootAbs = filepath.Join(rootAbs, filepath.FromSlash(normalizedScope))
		if !isPathWithinRoot(rootAbs, scanRootAbs) {
			return nil, fmt.Errorf("scan scope out of root")
		}
		info, err := os.Stat(scanRootAbs)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("scan scope is not a directory")
		}
	}

	log.Printf("collectRepoISORecordsByScanSpec: start root=%q scope=%q", rootAbs, normalizedScope)

	records := make([]models.RepoISO, 0)
	visitedFiles := 0
	matchedFiles := 0
	matchedDirs := 0
	normalizedExcludedRoots := make([]string, 0, len(excludeRootAbsPaths))
	for _, excluded := range excludeRootAbsPaths {
		trimmed := strings.TrimSpace(excluded)
		if trimmed == "" {
			continue
		}
		normalizedExcludedRoots = append(normalizedExcludedRoots, filepath.Clean(trimmed))
	}

	err = filepath.WalkDir(scanRootAbs, func(absPath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		cleanAbsPath := filepath.Clean(absPath)
		for _, excludedRoot := range normalizedExcludedRoots {
			if cleanAbsPath == excludedRoot || strings.HasPrefix(cleanAbsPath+string(os.PathSeparator), excludedRoot+string(os.PathSeparator)) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if d.IsDir() {
			if normalizedScope == "" && filepath.Clean(absPath) == filepath.Clean(rootAbs) {
				return nil
			}
			entries, err := os.ReadDir(absPath)
			if err != nil {
				return err
			}
			fileNames := make([]string, 0, len(entries))
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				fileNames = append(fileNames, entry.Name())
			}
			if matchedRule, count, ok := scanSpec.MatchDirectoryFiles(fileNames); ok {
				relPath, err := filepath.Rel(rootAbs, absPath)
				if err != nil {
					return err
				}
				relPath = filepath.ToSlash(relPath)
				records = append(records, models.RepoISO{
					FileName:    d.Name(),
					Path:        relPath,
					MD5:         "",
					SizeBytes:   models.UnknownRepoISOSizeBytes,
					Tags:        models.ExtractTagsFromFileName(d.Name()),
					IsDirectory: true,
				})
				matchedDirs++
				log.Printf("collectRepoISORecordsByScanSpec: matched directory root=%q dir=%q rule=%q file_count=%d", rootAbs, relPath, matchedRule.Name, count)
				return filepath.SkipDir
			}
			return nil
		}

		visitedFiles++
		if !scanSpec.ShouldScanFile(d.Name()) {
			return nil
		}
		matchedFiles++

		relPath, err := filepath.Rel(rootAbs, absPath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		records = append(records, models.RepoISO{
			FileName:    d.Name(),
			Path:        relPath,
			MD5:         "",
			SizeBytes:   models.UnknownRepoISOSizeBytes,
			Tags:        models.ExtractTagsFromFileName(d.Name()),
			IsDirectory: false,
		})

		if matchedFiles%100 == 0 {
			log.Printf("collectRepoISORecordsByScanSpec: progress root=%q visited_files=%d matched_files=%d matched_dirs=%d last_match=%q elapsed=%s", rootAbs, visitedFiles, matchedFiles, matchedDirs, relPath, time.Since(startedAt).Truncate(time.Millisecond))
		}
		return nil
	})
	if err != nil {
		log.Printf("collectRepoISORecordsByScanSpec: failed root=%q visited_files=%d matched_files=%d matched_dirs=%d elapsed=%s error=%v", rootAbs, visitedFiles, matchedFiles, matchedDirs, time.Since(startedAt).Truncate(time.Millisecond), err)
		return nil, err
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Path < records[j].Path
	})
	log.Printf("collectRepoISORecordsByScanSpec: done root=%q scope=%q visited_files=%d matched_files=%d matched_dirs=%d total_records=%d elapsed=%s", rootAbs, normalizedScope, visitedFiles, matchedFiles, matchedDirs, len(records), time.Since(startedAt).Truncate(time.Millisecond))
	return records, nil
}

func shouldBlockFullRepoScan(repo models.Repository, scanScope string) bool {
	normalizedScope, err := normalizeRepoScanScope(scanScope)
	if err != nil {
		return false
	}
	return repo.Basic && normalizedScope == ""
}

func normalizeRepoScanScope(value string) (string, error) {
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

func collectRepoISORecords(rootAbs string, shouldScanFile func(string) bool) ([]models.RepoISO, error) {
	startedAt := time.Now()
	log.Printf("collectRepoISORecords: start root=%q", rootAbs)

	if shouldScanFile == nil {
		shouldScanFile = func(name string) bool {
			return strings.EqualFold(filepath.Ext(name), ".iso")
		}
	}

	records := make([]models.RepoISO, 0)
	visitedFiles := 0
	matchedFiles := 0

	err := filepath.WalkDir(rootAbs, func(absPath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		visitedFiles++
		if !shouldScanFile(d.Name()) {
			return nil
		}
		matchedFiles++

		relPath, err := filepath.Rel(rootAbs, absPath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		records = append(records, models.RepoISO{
			FileName:    d.Name(),
			Path:        relPath,
			MD5:         "",
			SizeBytes:   models.UnknownRepoISOSizeBytes,
			Tags:        models.ExtractTagsFromFileName(d.Name()),
			IsDirectory: false,
		})

		if matchedFiles%100 == 0 {
			log.Printf("collectRepoISORecords: progress root=%q visited_files=%d matched_files=%d last_match=%q elapsed=%s", rootAbs, visitedFiles, matchedFiles, relPath, time.Since(startedAt).Truncate(time.Millisecond))
		}
		return nil
	})
	if err != nil {
		log.Printf("collectRepoISORecords: failed root=%q visited_files=%d matched_files=%d elapsed=%s error=%v", rootAbs, visitedFiles, matchedFiles, time.Since(startedAt).Truncate(time.Millisecond), err)
		return nil, err
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Path < records[j].Path
	})
	log.Printf("collectRepoISORecords: done root=%q visited_files=%d matched_files=%d elapsed=%s", rootAbs, visitedFiles, matchedFiles, time.Since(startedAt).Truncate(time.Millisecond))
	return records, nil
}

func rebuildRepoISOIndex(repoDB *gorm.DB, records []models.RepoISO) error {
	startedAt := time.Now()
	log.Printf("rebuildRepoISOIndex: begin records=%d", len(records))

	tx := repoDB.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	deleteResult := tx.Where("1 = 1").Delete(&models.RepoISO{})
	if deleteResult.Error != nil {
		tx.Rollback()
		return deleteResult.Error
	}
	log.Printf("rebuildRepoISOIndex: cleared old rows=%d", deleteResult.RowsAffected)

	if len(records) > 0 {
		if err := tx.Create(&records).Error; err != nil {
			tx.Rollback()
			return err
		}
		log.Printf("rebuildRepoISOIndex: inserted rows=%d", len(records))
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}
	log.Printf("rebuildRepoISOIndex: committed records=%d elapsed=%s", len(records), time.Since(startedAt).Truncate(time.Millisecond))
	return nil
}

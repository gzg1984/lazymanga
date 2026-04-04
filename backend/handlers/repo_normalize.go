package handlers

import (
	"errors"
	"lazyiso/models"
	"lazyiso/normalization"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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

	repoDB, rootAbs, dbPath, err := openRepoScopedDB(repo)
	if err != nil {
		log.Printf("ForceNormalizeRepoIncremental: open scoped db failed id=%s error=%v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "prepare repo db failed: " + err.Error()})
		return
	}

	collectStartedAt := time.Now()
	records, err := collectRepoISORecords(rootAbs)
	if err != nil {
		log.Printf("ForceNormalizeRepoIncremental: scan failed id=%s root=%s error=%v", id, rootAbs, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "scan repo path failed: " + err.Error()})
		return
	}
	log.Printf("ForceNormalizeRepoIncremental: collect done repo=%d scanned=%d elapsed=%s", repo.ID, len(records), time.Since(collectStartedAt).Truncate(time.Millisecond))

	queryStartedAt := time.Now()
	var existingRows []models.RepoISO
	if err := repoDB.Find(&existingRows).Error; err != nil {
		log.Printf("ForceNormalizeRepoIncremental: query existing rows failed repo=%d error=%v", repo.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query repoisos failed: " + err.Error()})
		return
	}
	log.Printf("ForceNormalizeRepoIncremental: loaded existing rows repo=%d total=%d elapsed=%s", repo.ID, len(existingRows), time.Since(queryStartedAt).Truncate(time.Millisecond))

	missingMarkedCount := 0
	missingRecoveredCount := 0
	for i := range existingRows {
		row := &existingRows[i]
		prevMissing := row.IsMissing

		missing := false
		absPath, pathErr := resolveRepoISOAbsPath(rootAbs, row.Path)
		if pathErr != nil {
			missing = true
			log.Printf("ForceNormalizeRepoIncremental: invalid row path treated as missing repo=%d row_id=%d path=%q error=%v", repo.ID, row.ID, row.Path, pathErr)
		} else {
			info, statErr := os.Stat(absPath)
			if statErr != nil {
				if os.IsNotExist(statErr) {
					missing = true
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "stat existing repo iso failed: " + statErr.Error()})
					return
				}
			} else if info.IsDir() {
				missing = true
			}
		}

		if err := updateRepoISOMissingFlag(repoDB, row, missing); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "update missing flag failed: " + err.Error()})
			return
		}

		if !prevMissing && missing {
			missingMarkedCount++
		}
		if prevMissing && !missing {
			missingRecoveredCount++
		}
	}

	existingByPath := make(map[string]models.RepoISO, len(existingRows))
	for _, row := range existingRows {
		existingByPath[row.Path] = row
	}

	newRecords := make([]models.RepoISO, 0)
	for _, scanned := range records {
		if _, ok := existingByPath[scanned.Path]; ok {
			continue
		}
		newRecords = append(newRecords, scanned)
	}

	insertedCount := 0
	if len(newRecords) > 0 {
		insertStartedAt := time.Now()
		if err := repoDB.Create(&newRecords).Error; err != nil {
			log.Printf("ForceNormalizeRepoIncremental: insert new rows failed repo=%d total_new=%d error=%v", repo.ID, len(newRecords), err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "insert new repoisos failed: " + err.Error()})
			return
		}
		insertedCount = len(newRecords)
		log.Printf("ForceNormalizeRepoIncremental: inserted new rows repo=%d total_new=%d elapsed=%s", repo.ID, insertedCount, time.Since(insertStartedAt).Truncate(time.Millisecond))
	}

	missingMetaRows := make([]models.RepoISO, 0)
	missingTotalCount := 0
	for _, row := range existingRows {
		if row.IsMissing {
			missingTotalCount++
			continue
		}
		if strings.TrimSpace(row.MD5) == "" || row.SizeBytes <= 0 {
			missingMetaRows = append(missingMetaRows, row)
		}
	}

	if len(newRecords) > 0 {
		normalization.StartAsyncPostIndexNormalization(repo.ID, repoDB, rootAbs, newRecords)
	}
	if len(missingMetaRows) > 0 {
		normalization.StartAsyncMetadataBackfill(repo.ID, repoDB, rootAbs, missingMetaRows)
	}

	log.Printf("ForceNormalizeRepoIncremental: done repo=%d root=%q db=%q scanned=%d inserted=%d existing=%d missing_marked=%d missing_recovered=%d missing_total=%d missing_meta=%d elapsed=%s", repo.ID, rootAbs, dbPath, len(records), insertedCount, len(existingRows), missingMarkedCount, missingRecoveredCount, missingTotalCount, len(missingMetaRows), time.Since(startedAt).Truncate(time.Millisecond))
	c.JSON(http.StatusOK, gin.H{
		"message":                          "incremental normalize triggered",
		"repo_id":                          repo.ID,
		"root_path":                        rootAbs,
		"db_path":                          dbPath,
		"scanned_iso_count":                len(records),
		"existing_iso_count":               len(existingRows),
		"existing_missing_marked_count":    missingMarkedCount,
		"existing_missing_recovered_count": missingRecoveredCount,
		"existing_missing_total_count":     missingTotalCount,
		"inserted_new_iso_count":           insertedCount,
		"existing_missing_meta_count":      len(missingMetaRows),
		"new_records_async_steps":          normalization.DefaultStepNames(),
		"existing_async_steps":             []string{"file-size-backfill", "md5-backfill"},
		"normalize_async":                  true,
	})
}

// ForceNormalizeRepo scans all ISO files under current repo path and rebuilds repoisos table in repo-local DB.
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

	repoDB, rootAbs, dbPath, err := openRepoScopedDB(repo)
	if err != nil {
		log.Printf("ForceNormalizeRepo: open scoped db failed id=%s error=%v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "prepare repo db failed: " + err.Error()})
		return
	}
	log.Printf("ForceNormalizeRepo: scoped db ready id=%d root_abs=%q db_path=%q", repo.ID, rootAbs, dbPath)

	collectStartedAt := time.Now()
	log.Printf("ForceNormalizeRepo: collect stage start id=%d root=%q", repo.ID, rootAbs)
	records, err := collectRepoISORecords(rootAbs)
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

func collectRepoISORecords(rootAbs string) ([]models.RepoISO, error) {
	startedAt := time.Now()
	log.Printf("collectRepoISORecords: start root=%q", rootAbs)

	records := make([]models.RepoISO, 0)
	visitedFiles := 0
	isoFiles := 0

	err := filepath.WalkDir(rootAbs, func(absPath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		visitedFiles++
		if !strings.EqualFold(filepath.Ext(d.Name()), ".iso") {
			return nil
		}
		isoFiles++

		relPath, err := filepath.Rel(rootAbs, absPath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		records = append(records, models.RepoISO{
			FileName:  d.Name(),
			Path:      relPath,
			MD5:       "",
			SizeBytes: models.UnknownRepoISOSizeBytes,
			Tags:      models.ExtractTagsFromFileName(d.Name()),
		})

		if isoFiles%100 == 0 {
			log.Printf("collectRepoISORecords: progress root=%q visited_files=%d iso_found=%d last_iso=%q elapsed=%s", rootAbs, visitedFiles, isoFiles, relPath, time.Since(startedAt).Truncate(time.Millisecond))
		}
		return nil
	})
	if err != nil {
		log.Printf("collectRepoISORecords: failed root=%q visited_files=%d iso_found=%d elapsed=%s error=%v", rootAbs, visitedFiles, isoFiles, time.Since(startedAt).Truncate(time.Millisecond), err)
		return nil, err
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Path < records[j].Path
	})
	log.Printf("collectRepoISORecords: done root=%q visited_files=%d iso_found=%d elapsed=%s", rootAbs, visitedFiles, isoFiles, time.Since(startedAt).Truncate(time.Millisecond))
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

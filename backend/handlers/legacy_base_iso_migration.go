package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"lazyiso/models"
	"lazyiso/normalization"
	"log"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	legacyBaseISOMigratedFlagKey           = "legacy_base_iso_to_repoisos_migrated"
	legacyBaseISOMigratedAtKey             = "legacy_base_iso_to_repoisos_migrated_at"
	legacyBaseISOMigratedCountKey          = "legacy_base_iso_to_repoisos_migrated_count"
	legacyBaseISOSkippedCountKey           = "legacy_base_iso_to_repoisos_skipped_count"
	legacyBaseISOMigrationSource           = "legacy_base_isos_table"
	legacyBaseISOMigrationSourceKey        = "legacy_base_iso_to_repoisos_source"
	legacyBaseISOMigrationNoticeShownKey   = "legacy_base_iso_to_repoisos_notice_shown"
	legacyBaseISOMigrationNoticeShownAtKey = "legacy_base_iso_to_repoisos_notice_shown_at"
	legacyBaseISOMetaBackfillDoneKey       = "legacy_base_iso_repoisos_metadata_backfill_done"
	legacyBaseISOMetaBackfillAtKey         = "legacy_base_iso_repoisos_metadata_backfill_at"
	legacyBaseISOMetaBackfillCountKey      = "legacy_base_iso_repoisos_metadata_backfill_count"
)

// MigrateLegacyBaseISOsOnce migrates legacy global ISOs rows into basic repo repoisos.
// It persists a migration flag in basic repo's repo_info.flags_json to guarantee one-time execution.
func MigrateLegacyBaseISOsOnce() error {
	if db == nil {
		return errors.New("database is not initialized")
	}

	var basicRepo models.Repository
	err := db.Where("basic = ?", true).Order("id asc").First(&basicRepo).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = db.Where("name = ?", basicRepoName).Order("id asc").First(&basicRepo).Error
	}
	if err != nil {
		return fmt.Errorf("query basic repository failed: %w", err)
	}

	repoDB, rootAbs, dbPath, err := openRepoScopedDB(basicRepo)
	if err != nil {
		return fmt.Errorf("open basic repo db failed: %w", err)
	}

	var legacyRows []models.ISOs
	if err := db.Order("id asc").Find(&legacyRows).Error; err != nil {
		return fmt.Errorf("query legacy isos failed: %w", err)
	}

	migratedCount := 0
	skippedCount := 0
	alreadyMigrated := false

	err = repoDB.Transaction(func(tx *gorm.DB) error {
		info, ensureErr := EnsureRepoInfoFromRepository(tx, basicRepo)
		if ensureErr != nil {
			return fmt.Errorf("ensure repo_info failed: %w", ensureErr)
		}

		flags := parseRepoInfoFlags(info.FlagsJSON)
		if done, _ := flags[legacyBaseISOMigratedFlagKey].(bool); done {
			alreadyMigrated = true
			return nil
		}

		var existing []models.RepoISO
		if queryErr := tx.Select("path").Find(&existing).Error; queryErr != nil {
			return fmt.Errorf("query existing repoisos failed: %w", queryErr)
		}

		existingPaths := make(map[string]struct{}, len(existing))
		for _, row := range existing {
			p := strings.TrimSpace(row.Path)
			if p != "" {
				existingPaths[p] = struct{}{}
			}
		}

		pending := make([]models.RepoISO, 0)
		pendingPaths := make(map[string]struct{})

		for _, old := range legacyRows {
			relPath, convertErr := normalizeLegacyISOPathForRepo(rootAbs, old.Path)
			if convertErr != nil {
				skippedCount++
				continue
			}

			if _, exists := existingPaths[relPath]; exists {
				skippedCount++
				continue
			}
			if _, exists := pendingPaths[relPath]; exists {
				skippedCount++
				continue
			}

			fileName := strings.TrimSpace(old.FileName)
			if fileName == "" {
				fileName = filepath.Base(filepath.FromSlash(relPath))
			}

			tags := strings.TrimSpace(old.Tags)
			if tags == "" {
				tags = models.ExtractTagsFromFileName(fileName)
			}

			pending = append(pending, models.RepoISO{
				UUID:      old.UUID,
				FileName:  fileName,
				Path:      relPath,
				MountPath: old.MountPath,
				MD5:       old.MD5,
				SizeBytes: models.UnknownRepoISOSizeBytes,
				Tags:      tags,
				IsMounted: old.IsMounted,
			})
			pendingPaths[relPath] = struct{}{}
		}

		if len(pending) > 0 {
			if createErr := tx.Create(&pending).Error; createErr != nil {
				return fmt.Errorf("insert migrated repoisos failed: %w", createErr)
			}
			migratedCount = len(pending)
		}

		flags[legacyBaseISOMigratedFlagKey] = true
		flags[legacyBaseISOMigratedAtKey] = time.Now().UTC().Format(time.RFC3339)
		flags[legacyBaseISOMigratedCountKey] = migratedCount
		flags[legacyBaseISOSkippedCountKey] = skippedCount
		flags[legacyBaseISOMigrationSourceKey] = legacyBaseISOMigrationSource

		encoded, marshalErr := json.Marshal(flags)
		if marshalErr != nil {
			return fmt.Errorf("marshal migration flags failed: %w", marshalErr)
		}
		info.FlagsJSON = string(encoded)

		if saveErr := tx.Save(&info).Error; saveErr != nil {
			return fmt.Errorf("save migration flags failed: %w", saveErr)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("migrate legacy base isos failed: %w", err)
	}

	if alreadyMigrated {
		log.Printf("MigrateLegacyBaseISOsOnce: skipped, migration already applied basic_repo_id=%d db=%s", basicRepo.ID, dbPath)
	} else {
		log.Printf("MigrateLegacyBaseISOsOnce: done basic_repo_id=%d db=%s migrated=%d skipped=%d total_legacy=%d", basicRepo.ID, dbPath, migratedCount, skippedCount, len(legacyRows))
	}

	metaBackfillRows, metaBackfillSkipped, err := scheduleLegacyBaseRepoMetadataBackfillOnce(repoDB, basicRepo)
	if err != nil {
		return fmt.Errorf("schedule legacy base metadata backfill failed: %w", err)
	}
	if metaBackfillSkipped {
		log.Printf("MigrateLegacyBaseISOsOnce: metadata backfill skipped basic_repo_id=%d db=%s", basicRepo.ID, dbPath)
	} else {
		log.Printf("MigrateLegacyBaseISOsOnce: metadata backfill scheduled basic_repo_id=%d db=%s rows=%d", basicRepo.ID, dbPath, metaBackfillRows)
	}

	return nil
}

func scheduleLegacyBaseRepoMetadataBackfillOnce(repoDB *gorm.DB, repo models.Repository) (int, bool, error) {
	candidates := make([]models.RepoISO, 0)
	alreadyDone := false

	err := repoDB.Transaction(func(tx *gorm.DB) error {
		info, ensureErr := EnsureRepoInfoFromRepository(tx, repo)
		if ensureErr != nil {
			return fmt.Errorf("ensure repo_info failed: %w", ensureErr)
		}

		flags := parseRepoInfoFlags(info.FlagsJSON)
		if done, _ := flags[legacyBaseISOMetaBackfillDoneKey].(bool); done {
			alreadyDone = true
			return nil
		}

		var rows []models.RepoISO
		if queryErr := tx.Select("id", "path", "file_name", "md5", "size_bytes").Find(&rows).Error; queryErr != nil {
			return fmt.Errorf("query repoisos for metadata backfill failed: %w", queryErr)
		}

		for _, row := range rows {
			if strings.TrimSpace(row.MD5) == "" || row.SizeBytes <= 0 {
				candidates = append(candidates, row)
			}
		}

		flags[legacyBaseISOMetaBackfillDoneKey] = true
		flags[legacyBaseISOMetaBackfillAtKey] = time.Now().UTC().Format(time.RFC3339)
		flags[legacyBaseISOMetaBackfillCountKey] = len(candidates)

		encoded, marshalErr := json.Marshal(flags)
		if marshalErr != nil {
			return fmt.Errorf("marshal metadata backfill flags failed: %w", marshalErr)
		}
		info.FlagsJSON = string(encoded)

		if saveErr := tx.Save(&info).Error; saveErr != nil {
			return fmt.Errorf("save metadata backfill flags failed: %w", saveErr)
		}
		return nil
	})
	if err != nil {
		return 0, false, err
	}

	if alreadyDone {
		return 0, true, nil
	}

	if len(candidates) > 0 {
		repoDBForAsync, rootAbs, _, openErr := openRepoScopedDB(repo)
		if openErr != nil {
			return 0, false, fmt.Errorf("open repo db for async metadata backfill failed: %w", openErr)
		}
		normalization.StartAsyncMetadataBackfill(repo.ID, repoDBForAsync, rootAbs, candidates)
	}

	return len(candidates), false, nil
}

func parseRepoInfoFlags(flagsJSON string) map[string]interface{} {
	flags := map[string]interface{}{}
	v := strings.TrimSpace(flagsJSON)
	if v == "" {
		return flags
	}
	if err := json.Unmarshal([]byte(v), &flags); err != nil {
		return map[string]interface{}{}
	}
	return flags
}

func normalizeLegacyISOPathForRepo(rootAbs string, legacyPath string) (string, error) {
	trimmed := strings.TrimSpace(legacyPath)
	if trimmed == "" {
		return "", errors.New("empty legacy path")
	}

	normalizedInput := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(trimmed, "\\", "/")))
	if filepath.IsAbs(normalizedInput) {
		rel, err := filepath.Rel(rootAbs, normalizedInput)
		if err != nil {
			return "", err
		}
		rel = filepath.ToSlash(filepath.Clean(rel))
		rel = strings.TrimPrefix(rel, "./")
		if rel == "" || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
			return "", errors.New("legacy absolute path out of basic repo root")
		}
		return rel, nil
	}

	rel := filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.ReplaceAll(trimmed, "\\", "/"))))
	rel = strings.TrimPrefix(rel, "./")
	rel = strings.Trim(rel, "/")
	if rel == "" || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", errors.New("invalid legacy relative path")
	}
	return rel, nil
}

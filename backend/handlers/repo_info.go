package handlers

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"lazymanga/models"
	"log"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

const (
	repoInfoSingletonID         uint          = 1
	repoInfoSchemaVersion       int           = 5
	defaultRepoInfoFlagsJSON    string        = "{}"
	repoTypeNone                string        = "none"
	repoTypeOS                  string        = "os"
	repoMetadataRefreshInterval time.Duration = 15 * time.Second
)

var (
	repoMetadataRefreshMu     sync.Mutex
	repoMetadataLastRefreshAt time.Time
)

func normalizeCreateRepoType(repoType string) (string, error) {
	key, _, err := resolveVisibleRepoTypeForCreate(repoType)
	if err != nil {
		return "", err
	}
	return key, nil
}

func applyRepoInfoPresetByType(repo models.Repository, repoType string) error {
	repoDB, _, dbPath, err := openRepoScopedDB(repo)
	if err != nil {
		return fmt.Errorf("open repo db failed: %w", err)
	}

	info, err := EnsureRepoInfoFromRepository(repoDB, repo)
	if err != nil {
		return fmt.Errorf("ensure repo_info failed for db=%s: %w", dbPath, err)
	}

	key, def, err := resolveRepoTypeForCreate(repoType)
	if err != nil {
		return err
	}

	effective := applyRepoSettingsOverride(repoTypeDefToSettings(def), repoTypeSettingsOverride{})
	changed, err := applyEffectiveSettingsToRepoInfo(&info, key, repoTypeSettingsOverride{}, effective)
	if err != nil {
		return fmt.Errorf("apply repo type settings failed for db=%s: %w", dbPath, err)
	}
	if !changed {
		return nil
	}

	if err := repoDB.Save(&info).Error; err != nil {
		return fmt.Errorf("save repo_info preset failed for db=%s: %w", dbPath, err)
	}
	if err := updateRepositoryRepoTypeKey(repo.ID, key); err != nil {
		return fmt.Errorf("sync repository repo_type_key failed: %w", err)
	}
	return nil
}

func setRepoRulebookBindingOnInfo(info *models.RepoInfo, name string, version string) bool {
	if info == nil {
		return false
	}

	flags := map[string]interface{}{}
	if strings.TrimSpace(info.FlagsJSON) != "" {
		if err := json.Unmarshal([]byte(info.FlagsJSON), &flags); err != nil {
			flags = map[string]interface{}{}
		}
	}

	curName, _ := flags["rulebook_name"].(string)
	curVersion, _ := flags["rulebook_version"].(string)
	if strings.TrimSpace(curName) == name && strings.TrimSpace(curVersion) == version {
		return false
	}

	flags["rulebook_name"] = name
	flags["rulebook_version"] = version
	encoded, err := json.Marshal(flags)
	if err != nil {
		return false
	}

	info.FlagsJSON = string(encoded)
	return true
}

func BootstrapRepositories() error {
	if db == nil {
		return errors.New("database is not initialized")
	}

	var repos []models.Repository
	if err := db.Order("id asc").Find(&repos).Error; err != nil {
		return fmt.Errorf("query repositories failed: %w", err)
	}

	var errs []error
	for _, repo := range repos {
		if err := BootstrapSingleRepository(repo); err != nil {
			log.Printf("BootstrapRepositories: repo id=%d name=%q failed: %v", repo.ID, repo.Name, err)
			errs = append(errs, fmt.Errorf("repo id=%d name=%q: %w", repo.ID, repo.Name, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	log.Printf("BootstrapRepositories: completed total=%d", len(repos))
	return nil
}

func shouldRefreshRepositoryMetadataAt(lastRefresh time.Time, now time.Time, ttl time.Duration, force bool) bool {
	if force || ttl <= 0 {
		return true
	}
	if lastRefresh.IsZero() {
		return true
	}
	return now.Sub(lastRefresh) >= ttl
}

// RefreshRepositoryMetadataCachesIfStale refreshes repo metadata caches only when the last refresh is older than the throttle interval.
func RefreshRepositoryMetadataCachesIfStale(repos []models.Repository, force bool) (bool, int, []error) {
	now := time.Now()
	repoMetadataRefreshMu.Lock()
	if !shouldRefreshRepositoryMetadataAt(repoMetadataLastRefreshAt, now, repoMetadataRefreshInterval, force) {
		repoMetadataRefreshMu.Unlock()
		return false, 0, nil
	}
	repoMetadataLastRefreshAt = now
	repoMetadataRefreshMu.Unlock()

	updated, errs := RefreshRepositoryMetadataCaches(repos)
	return true, updated, errs
}

// RefreshRepositoryMetadataCaches refreshes global name/basic/repo_uuid caches
// from repo.db metadata for repositories with a configured root path.
// It is designed for read-path usage: failures are returned for logging,
// while callers can still serve stale-but-available global data.
func RefreshRepositoryMetadataCaches(repos []models.Repository) (int, []error) {
	updated := 0
	errs := make([]error, 0)

	for _, repo := range repos {
		if strings.TrimSpace(repo.RootPath) == "" {
			continue
		}

		beforeUUID := repo.RepoUUID
		beforeName := repo.Name
		beforeBasic := repo.Basic

		if err := BootstrapSingleRepository(repo); err != nil {
			errs = append(errs, fmt.Errorf("repo id=%d name=%q: %w", repo.ID, repo.Name, err))
			continue
		}

		var reloaded models.Repository
		if err := db.First(&reloaded, repo.ID).Error; err != nil {
			errs = append(errs, fmt.Errorf("repo id=%d reload failed: %w", repo.ID, err))
			continue
		}

		if reloaded.RepoUUID != beforeUUID || reloaded.Name != beforeName || reloaded.Basic != beforeBasic {
			updated++
		}
	}

	return updated, errs
}

func BootstrapSingleRepository(repo models.Repository) error {
	repoDB, _, dbPath, err := openRepoScopedDB(repo)
	if err != nil {
		return fmt.Errorf("open repo db failed: %w", err)
	}

	info, err := EnsureRepoInfoFromRepository(repoDB, repo)
	if err != nil {
		return fmt.Errorf("ensure repo_info failed for db=%s: %w", dbPath, err)
	}

	if err := DetectRepositoryBindingConflict(repo, info); err != nil {
		log.Printf("BootstrapSingleRepository: binding conflict repo id=%d global_uuid=%q repo_info_uuid=%q db=%s error=%v", repo.ID, repo.RepoUUID, info.RepoUUID, dbPath, err)
		return err
	}

	if err := SyncRepositoryCacheFromRepoInfo(repo, info); err != nil {
		return fmt.Errorf("sync repository cache failed: %w", err)
	}

	log.Printf("BootstrapSingleRepository: synced repo id=%d name=%q repoUUID=%q db=%s", repo.ID, repo.Name, info.RepoUUID, dbPath)
	return nil
}

func EnsureRepoInfoFromRepository(repoDB *gorm.DB, repo models.Repository) (models.RepoInfo, error) {
	info, exists, err := loadRepoInfoSingleton(repoDB)
	if err != nil {
		return models.RepoInfo{}, err
	}

	if !exists {
		created, err := buildRepoInfoFromRepository(repo)
		if err != nil {
			return models.RepoInfo{}, err
		}
		if err := repoDB.Create(&created).Error; err != nil {
			return models.RepoInfo{}, err
		}
		return created, nil
	}

	changed := false
	if strings.TrimSpace(info.RepoUUID) == "" {
		info.RepoUUID, err = repositoryOrNewUUID(repo.RepoUUID)
		if err != nil {
			return models.RepoInfo{}, err
		}
		changed = true
	}
	if strings.TrimSpace(info.Name) == "" {
		info.Name = fallbackRepoDisplayName(repo)
		changed = true
	}
	if info.SchemaVersion <= 0 {
		info.SchemaVersion = 1
		changed = true
	}

	// Schema v2 migration: backfill show_md5/show_size for existing basic repos.
	if info.SchemaVersion < 2 {
		if info.Basic || repo.Basic {
			if !info.ShowMD5 {
				info.ShowMD5 = true
				changed = true
			}
			if !info.ShowSize {
				info.ShowSize = true
				changed = true
			}
		}
		info.SchemaVersion = 2
		changed = true
	}

	// Schema v3 migration: backfill single_move for existing basic repos.
	if info.SchemaVersion < 3 {
		if (info.Basic || repo.Basic) && !info.SingleMove {
			info.SingleMove = true
			changed = true
		}
		info.SchemaVersion = 3
		changed = true
	}

	if info.SchemaVersion < 4 {
		if strings.TrimSpace(info.RepoTypeKey) == "" {
			info.RepoTypeKey = inferRepoTypeKeyFromInfo(info, repo)
			changed = true
		}
		if strings.TrimSpace(info.SettingsOverrideJSON) == "" {
			info.SettingsOverrideJSON = defaultRepoSettingsOverrideJSON
			changed = true
		}
		info.SchemaVersion = 4
		changed = true
	}

	if info.SchemaVersion < 5 {
		info.SchemaVersion = repoInfoSchemaVersion
		changed = true
	}

	if strings.TrimSpace(info.FlagsJSON) == "" {
		info.FlagsJSON = defaultRepoInfoFlagsJSON
		changed = true
	}
	if strings.TrimSpace(info.RepoTypeKey) == "" {
		info.RepoTypeKey = inferRepoTypeKeyFromInfo(info, repo)
		changed = true
	}
	if strings.TrimSpace(info.SettingsOverrideJSON) == "" {
		info.SettingsOverrideJSON = defaultRepoSettingsOverrideJSON
		changed = true
	}

	if repoTypeKey, _, override, effective, _, resolveErr := resolveEffectiveRepoTypeSettings(info, repo); resolveErr == nil {
		applied, applyErr := applyEffectiveSettingsToRepoInfo(&info, repoTypeKey, override, effective)
		if applyErr != nil {
			return models.RepoInfo{}, applyErr
		}
		if applied {
			changed = true
		}
	} else {
		log.Printf("EnsureRepoInfoFromRepository: resolve effective repo type settings failed repo_id=%d name=%q err=%v", repo.ID, repo.Name, resolveErr)
	}

	if changed {
		if err := repoDB.Save(&info).Error; err != nil {
			return models.RepoInfo{}, err
		}
	}

	return info, nil
}

func SyncRepositoryCacheFromRepoInfo(repo models.Repository, info models.RepoInfo) error {
	changed := false
	if repo.RepoUUID != info.RepoUUID {
		repo.RepoUUID = info.RepoUUID
		changed = true
	}
	if repo.Name != info.Name {
		repo.Name = info.Name
		changed = true
	}
	if repo.RepoTypeKey != info.RepoTypeKey {
		repo.RepoTypeKey = info.RepoTypeKey
		changed = true
	}
	if repo.Basic != info.Basic {
		repo.Basic = info.Basic
		changed = true
	}

	if !changed {
		return nil
	}

	if err := db.Save(&repo).Error; err != nil {
		return err
	}

	log.Printf("SyncRepositoryCacheFromRepoInfo: updated repo id=%d repoUUID=%q name=%q basic=%t", repo.ID, repo.RepoUUID, repo.Name, repo.Basic)
	return nil
}

func writeRepoInfoMetadata(repo models.Repository, nextName string, nextBasic bool) error {
	repoDB, _, dbPath, err := openRepoScopedDB(repo)
	if err != nil {
		return fmt.Errorf("open repo db failed: %w", err)
	}

	info, err := EnsureRepoInfoFromRepository(repoDB, repo)
	if err != nil {
		return fmt.Errorf("ensure repo_info failed for db=%s: %w", dbPath, err)
	}

	if err := DetectRepositoryBindingConflict(repo, info); err != nil {
		return err
	}

	changed := false
	if info.Name != nextName {
		info.Name = nextName
		changed = true
	}
	if info.Basic != nextBasic {
		info.Basic = nextBasic
		changed = true
	}
	if nextBasic && !info.AddButton {
		info.AddButton = true
		changed = true
	}
	if nextBasic && !info.DeleteButton {
		info.DeleteButton = true
		changed = true
	}

	if !changed {
		return nil
	}

	if err := repoDB.Save(&info).Error; err != nil {
		return fmt.Errorf("save repo_info failed for db=%s: %w", dbPath, err)
	}

	return nil
}

func DetectRepositoryBindingConflict(repo models.Repository, info models.RepoInfo) error {
	globalUUID := strings.TrimSpace(repo.RepoUUID)
	repoUUID := strings.TrimSpace(info.RepoUUID)
	if globalUUID != "" && repoUUID != "" && globalUUID != repoUUID {
		return fmt.Errorf("repo_uuid conflict global=%q repo_db=%q", globalUUID, repoUUID)
	}

	if repoUUID == "" {
		return fmt.Errorf("repo_info repo_uuid is empty")
	}

	var count int64
	if err := db.Model(&models.Repository{}).Where("repo_uuid = ? AND id <> ?", repoUUID, repo.ID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("repo_uuid %q already bound by another repository", repoUUID)
	}

	return nil
}

func loadRepoInfoSingleton(repoDB *gorm.DB) (models.RepoInfo, bool, error) {
	var infos []models.RepoInfo
	if err := repoDB.Order("id asc").Find(&infos).Error; err != nil {
		return models.RepoInfo{}, false, err
	}

	if len(infos) == 0 {
		return models.RepoInfo{}, false, nil
	}
	if len(infos) > 1 {
		return models.RepoInfo{}, false, fmt.Errorf("repo_info singleton violated: found %d rows", len(infos))
	}
	if infos[0].ID != repoInfoSingletonID {
		return models.RepoInfo{}, false, fmt.Errorf("repo_info singleton id mismatch: got %d want %d", infos[0].ID, repoInfoSingletonID)
	}

	return infos[0], true, nil
}

func buildRepoInfoFromRepository(repo models.Repository) (models.RepoInfo, error) {
	repoUUID, err := repositoryOrNewUUID(repo.RepoUUID)
	if err != nil {
		return models.RepoInfo{}, err
	}

	initialRepoTypeKey := strings.TrimSpace(strings.ToLower(repo.RepoTypeKey))
	if repo.Basic {
		initialRepoTypeKey = manualMangaRepoTypeKey
	} else if initialRepoTypeKey == "" {
		initialRepoTypeKey = defaultRepoTypeKey
	}

	return models.RepoInfo{
		ID:                   repoInfoSingletonID,
		RepoUUID:             repoUUID,
		Name:                 fallbackRepoDisplayName(repo),
		RepoTypeKey:          initialRepoTypeKey,
		Basic:                repo.Basic,
		AddButton:            repo.Basic,
		AddDirectoryButton:   repo.Basic,
		DeleteButton:         repo.Basic,
		AutoNormalize:        false,
		ShowMD5:              repo.Basic,
		ShowSize:             repo.Basic,
		SingleMove:           repo.Basic,
		SchemaVersion:        repoInfoSchemaVersion,
		FlagsJSON:            defaultRepoInfoFlagsJSON,
		SettingsOverrideJSON: defaultRepoSettingsOverrideJSON,
	}, nil
}

func fallbackRepoDisplayName(repo models.Repository) string {
	name := strings.TrimSpace(repo.Name)
	if name != "" {
		return name
	}
	if repo.Basic {
		return basicRepoName
	}
	return fmt.Sprintf("repo-%d", repo.ID)
}

func repositoryOrNewUUID(current string) (string, error) {
	v := strings.TrimSpace(current)
	if v != "" {
		return v, nil
	}
	return newRepoUUID()
}

func newRepoUUID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate repo uuid failed: %w", err)
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		buf[0], buf[1], buf[2], buf[3],
		buf[4], buf[5],
		buf[6], buf[7],
		buf[8], buf[9],
		buf[10], buf[11], buf[12], buf[13], buf[14], buf[15],
	), nil
}

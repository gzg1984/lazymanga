package handlers

import (
	"errors"
	"fmt"
	"lazymanga/models"
	"log"
	"os"
	"sort"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type repoBindingAudit struct {
	Repo          models.Repository
	LocationKey   string
	RootAbs       string
	DBPath        string
	DBExists      bool
	RepoInfoFound bool
	RepoInfo      models.RepoInfo
	InspectErr    error
	ConflictErr   error
}

func AuditRepositoryBindingsFromEnv() error {
	autoDelete := parseBoolEnv("LAZYMANGA_AUTO_DELETE_DUPLICATE_REPOS")
	return AuditRepositoryBindings(autoDelete)
}

func AuditRepositoryBindings(autoDelete bool) error {
	if db == nil {
		return errors.New("database is not initialized")
	}

	var repos []models.Repository
	if err := db.Order("id asc").Find(&repos).Error; err != nil {
		return fmt.Errorf("query repositories failed: %w", err)
	}

	log.Printf("RepositoryBindingAudit: start total=%d auto_delete_duplicate_repos=%t", len(repos), autoDelete)

	audits := make([]repoBindingAudit, 0, len(repos))
	for _, repo := range repos {
		audit := inspectRepositoryBinding(repo)
		audits = append(audits, audit)
		logRepositoryBindingAudit(audit)
	}

	logDuplicateRepositoryUUIDs(audits)
	logDuplicateRepositoryLocations(audits)

	if autoDelete {
		deletedIDs, cleanupErrs := deleteDuplicateRepositoryLocationRows(audits)
		for _, cleanupErr := range cleanupErrs {
			log.Printf("RepositoryBindingAudit: auto-delete warning: %v", cleanupErr)
		}
		if len(deletedIDs) > 0 {
			log.Printf("RepositoryBindingAudit: auto-delete removed repository ids=%v", deletedIDs)
		}
	}

	log.Printf("RepositoryBindingAudit: completed total=%d", len(repos))
	return nil
}

func inspectRepositoryBinding(repo models.Repository) repoBindingAudit {
	audit := repoBindingAudit{
		Repo:        repo,
		LocationKey: repositoryLocationKey(repo),
	}

	rootAbs, dbPath, err := resolveRepoDBPath(repo)
	if err != nil {
		audit.InspectErr = err
		return audit
	}
	audit.RootAbs = rootAbs
	audit.DBPath = dbPath

	if _, err := os.Stat(dbPath); err != nil {
		if !os.IsNotExist(err) {
			audit.InspectErr = fmt.Errorf("stat repo db failed: %w", err)
		}
		return audit
	}
	audit.DBExists = true

	repoDB, err := openExistingRepoDB(dbPath)
	if err != nil {
		audit.InspectErr = err
		return audit
	}

	info, exists, err := loadRepoInfoSingleton(repoDB)
	if err != nil {
		audit.InspectErr = fmt.Errorf("load repo_info failed: %w", err)
		return audit
	}
	if !exists {
		return audit
	}

	audit.RepoInfoFound = true
	audit.RepoInfo = info
	audit.ConflictErr = DetectRepositoryBindingConflict(repo, info)
	return audit
}

func openExistingRepoDB(dbPath string) (*gorm.DB, error) {
	repoDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open existing repo db failed: %w", err)
	}
	return repoDB, nil
}

func logRepositoryBindingAudit(audit repoBindingAudit) {
	repo := audit.Repo
	baseMsg := fmt.Sprintf(
		"RepositoryBindingAudit: repo id=%d name=%q global_uuid=%q basic=%t internal=%t external_device=%q root_path=%q dbfile=%q location=%q",
		repo.ID,
		repo.Name,
		repo.RepoUUID,
		repo.Basic,
		repo.IsInternal,
		repo.ExternalDeviceName,
		repo.RootPath,
		repo.DBFile,
		audit.LocationKey,
	)

	if audit.RootAbs != "" || audit.DBPath != "" {
		baseMsg += fmt.Sprintf(" root_abs=%q db_path=%q db_exists=%t", audit.RootAbs, audit.DBPath, audit.DBExists)
	}

	if audit.InspectErr != nil {
		log.Printf("%s inspect_error=%v", baseMsg, audit.InspectErr)
		return
	}

	if !audit.RepoInfoFound {
		log.Printf("%s repo_info=missing", baseMsg)
		return
	}

	msg := fmt.Sprintf(
		"%s repo_info_uuid=%q repo_info_name=%q repo_info_basic=%t",
		baseMsg,
		audit.RepoInfo.RepoUUID,
		audit.RepoInfo.Name,
		audit.RepoInfo.Basic,
	)
	if audit.ConflictErr != nil {
		log.Printf("%s binding_conflict=%v", msg, audit.ConflictErr)
		return
	}
	log.Printf("%s binding_status=ok", msg)
}

func logDuplicateRepositoryUUIDs(audits []repoBindingAudit) {
	byUUID := make(map[string][]models.Repository)
	for _, audit := range audits {
		uuid := strings.TrimSpace(audit.Repo.RepoUUID)
		if uuid == "" {
			continue
		}
		byUUID[uuid] = append(byUUID[uuid], audit.Repo)
	}

	for uuid, repos := range byUUID {
		if len(repos) <= 1 {
			continue
		}
		sort.Slice(repos, func(i, j int) bool { return repos[i].ID < repos[j].ID })
		parts := make([]string, 0, len(repos))
		for _, repo := range repos {
			parts = append(parts, fmt.Sprintf("id=%d name=%q root=%q dbfile=%q", repo.ID, repo.Name, repo.RootPath, repo.DBFile))
		}
		log.Printf("RepositoryBindingAudit: duplicate global repo_uuid=%q repos=[%s]", uuid, strings.Join(parts, "; "))
	}
}

func logDuplicateRepositoryLocations(audits []repoBindingAudit) {
	byLocation := make(map[string][]repoBindingAudit)
	for _, audit := range audits {
		byLocation[audit.LocationKey] = append(byLocation[audit.LocationKey], audit)
	}

	for location, group := range byLocation {
		if len(group) <= 1 {
			continue
		}
		sort.Slice(group, func(i, j int) bool { return group[i].Repo.ID < group[j].Repo.ID })
		parts := make([]string, 0, len(group))
		for _, audit := range group {
			repoInfoUUID := ""
			if audit.RepoInfoFound {
				repoInfoUUID = audit.RepoInfo.RepoUUID
			}
			parts = append(parts, fmt.Sprintf("id=%d global_uuid=%q repo_info_uuid=%q inspect_err=%q", audit.Repo.ID, audit.Repo.RepoUUID, repoInfoUUID, errString(audit.InspectErr)))
		}
		log.Printf("RepositoryBindingAudit: duplicate location=%q repos=[%s]", location, strings.Join(parts, "; "))
	}
}

func deleteDuplicateRepositoryLocationRows(audits []repoBindingAudit) ([]uint, []error) {
	byLocation := make(map[string][]repoBindingAudit)
	for _, audit := range audits {
		byLocation[audit.LocationKey] = append(byLocation[audit.LocationKey], audit)
	}

	deletedIDs := make([]uint, 0)
	errs := make([]error, 0)
	for location, group := range byLocation {
		if len(group) <= 1 {
			continue
		}

		keeperIndex, ok := selectDuplicateLocationKeeper(group)
		if !ok {
			log.Printf("RepositoryBindingAudit: duplicate location=%q detected but no unambiguous keeper found, skip auto-delete", location)
			continue
		}

		keeper := group[keeperIndex]
		for idx, audit := range group {
			if idx == keeperIndex {
				continue
			}
			if err := db.Delete(&models.Repository{}, audit.Repo.ID).Error; err != nil {
				errs = append(errs, fmt.Errorf("delete duplicate repo id=%d location=%q keep=%d failed: %w", audit.Repo.ID, location, keeper.Repo.ID, err))
				continue
			}
			deletedIDs = append(deletedIDs, audit.Repo.ID)
			log.Printf("RepositoryBindingAudit: auto-deleted duplicate repo id=%d location=%q keep_id=%d keep_uuid=%q", audit.Repo.ID, location, keeper.Repo.ID, keeper.Repo.RepoUUID)
		}
	}

	sort.Slice(deletedIDs, func(i, j int) bool { return deletedIDs[i] < deletedIDs[j] })
	return deletedIDs, errs
}

func selectDuplicateLocationKeeper(group []repoBindingAudit) (int, bool) {
	matchedIndexes := make([]int, 0)
	for idx, audit := range group {
		if !audit.RepoInfoFound || audit.InspectErr != nil {
			continue
		}
		if strings.TrimSpace(audit.Repo.RepoUUID) == strings.TrimSpace(audit.RepoInfo.RepoUUID) {
			matchedIndexes = append(matchedIndexes, idx)
		}
	}
	if len(matchedIndexes) != 1 {
		return 0, false
	}
	return matchedIndexes[0], true
}

func repositoryLocationKey(repo models.Repository) string {
	dbFile := strings.TrimSpace(repo.DBFile)
	if dbFile == "" {
		dbFile = "repo.db"
	}
	return fmt.Sprintf(
		"basic=%t|internal=%t|device=%s|root=%s|dbfile=%s",
		repo.Basic,
		repo.IsInternal,
		strings.TrimSpace(repo.ExternalDeviceName),
		strings.TrimSpace(repo.RootPath),
		dbFile,
	)
}

func parseBoolEnv(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

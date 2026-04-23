package handlers

import (
	"errors"
	"fmt"
	"lazymanga/models"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const basicRepoName = "基础漫画仓库"

// basicRepoDBDir points to the directory used to persist basic repo.db.
// It follows the directory of global lazymanga.db (configured by --db).
var basicRepoDBDir = "/lzcapp/var"

// EnsureBasicRepository makes sure a named baseline repository exists.
func EnsureBasicRepository(lazymangaDBPath string) error {
	if db == nil {
		return errors.New("database is not initialized")
	}

	basicDBDir, err := deriveBasicRepoDBDir(lazymangaDBPath)
	if err != nil {
		return err
	}
	basicRepoDBDir = basicDBDir

	baseRootPath, err := deriveBasicRepoRootPath()
	if err != nil {
		return err
	}

	var repo models.Repository
	err = db.Where("basic = ?", true).Order("id asc").First(&repo).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = db.Where("name = ?", basicRepoName).Order("id asc").First(&repo).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		repo = models.Repository{
			Name:        basicRepoName,
			RepoTypeKey: manualMangaRepoTypeKey,
			Basic:       true,
			RootPath:    baseRootPath,
			DBFile:      "repo.db",
			IsInternal:  true,
		}
		if createErr := db.Create(&repo).Error; createErr != nil {
			return createErr
		}
		if err := ensureBasicRepositoryTypeManualManga(repo); err != nil {
			return err
		}
		log.Printf("EnsureBasicRepository: created id=%d name=%q basic=%t root=%q dbfile=%q", repo.ID, repo.Name, repo.Basic, repo.RootPath, repo.DBFile)
		return nil
	}
	if err != nil {
		return err
	}

	changed := false
	if !repo.Basic {
		repo.Basic = true
		changed = true
	}
	if repo.Name != basicRepoName {
		repo.Name = basicRepoName
		changed = true
	}
	if repo.RootPath != baseRootPath {
		repo.RootPath = baseRootPath
		changed = true
	}
	if strings.TrimSpace(repo.DBFile) != "repo.db" {
		repo.DBFile = "repo.db"
		changed = true
	}
	if !repo.IsInternal {
		repo.IsInternal = true
		changed = true
	}
	if repo.ExternalDeviceName != "" {
		repo.ExternalDeviceName = ""
		changed = true
	}
	if strings.TrimSpace(strings.ToLower(repo.RepoTypeKey)) != manualMangaRepoTypeKey {
		repo.RepoTypeKey = manualMangaRepoTypeKey
		changed = true
	}

	if !changed {
		if err := ensureBasicRepositoryTypeManualManga(repo); err != nil {
			return err
		}
		if err := writeRepoInfoMetadata(repo, basicRepoName, true); err != nil {
			return err
		}
		log.Printf("EnsureBasicRepository: already synced id=%d name=%q root=%q dbfile=%q", repo.ID, repo.Name, repo.RootPath, repo.DBFile)
		return nil
	}

	if err := db.Save(&repo).Error; err != nil {
		return err
	}
	if err := ensureBasicRepositoryTypeManualManga(repo); err != nil {
		return err
	}
	if err := writeRepoInfoMetadata(repo, basicRepoName, true); err != nil {
		return err
	}

	log.Printf("EnsureBasicRepository: updated id=%d name=%q basic=%t root=%q dbfile=%q", repo.ID, repo.Name, repo.Basic, repo.RootPath, repo.DBFile)
	return nil
}

func ensureBasicRepositoryTypeManualManga(repo models.Repository) error {
	if !repo.Basic {
		return nil
	}
	if err := applyRepoInfoPresetByType(repo, manualMangaRepoTypeKey); err != nil {
		return fmt.Errorf("ensure basic repository repo type manual manga failed: %w", err)
	}
	return nil
}

func deriveBasicRepoRootPath() (string, error) {
	root := filepath.Clean(internalRepoRoot)
	if !filepath.IsAbs(root) {
		return "", errors.New("basic repo root must be absolute")
	}
	return filepath.ToSlash(root), nil
}

func deriveBasicRepoDBDir(lazymangaDBPath string) (string, error) {
	v := strings.TrimSpace(lazymangaDBPath)
	if v == "" {
		return "", errors.New("lazymanga db path is required")
	}
	absDBPath, err := filepath.Abs(v)
	if err != nil {
		return "", err
	}
	dir := filepath.Clean(filepath.Dir(absDBPath))
	if !filepath.IsAbs(dir) {
		return "", errors.New("failed to derive absolute db directory")
	}
	return filepath.ToSlash(dir), nil
}

// GetRepos 列出所有仓库（global DB 中的 repositories 表）
func GetRepos(c *gin.Context) {
	log.Printf("GetRepos: start method=%s path=%s remote=%s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
	var repos []models.Repository
	if err := applyRepositoryListOrder(db).Find(&repos).Error; err != nil {
		log.Printf("GetRepos: db query failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db query failed: " + err.Error()})
		return
	}

	forceRefresh := strings.EqualFold(strings.TrimSpace(c.Query("refresh_metadata")), "true") || c.Query("refresh_metadata") == "1"
	refreshed, updated, refreshErrs := RefreshRepositoryMetadataCachesIfStale(repos, forceRefresh)
	if len(refreshErrs) > 0 {
		for _, refreshErr := range refreshErrs {
			log.Printf("GetRepos: metadata refresh warning: %v", refreshErr)
		}
	}
	if refreshed && updated > 0 {
		if err := applyRepositoryListOrder(db).Find(&repos).Error; err != nil {
			log.Printf("GetRepos: requery after metadata refresh failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db requery failed: " + err.Error()})
			return
		}
	}

	if len(repos) == 0 {
		log.Printf("GetRepos: success total=0")
	} else {
		parts := make([]string, 0, len(repos))
		for _, r := range repos {
			parts = append(parts, "{id="+toStringUint(r.ID)+",name="+r.Name+",root="+r.RootPath+",dbfile="+r.DBFile+",internal="+strconv.FormatBool(r.IsInternal)+",extDevice="+r.ExternalDeviceName+"}")
		}
		log.Printf("GetRepos: success total=%d repos=%s", len(repos), strings.Join(parts, ","))
	}
	c.JSON(http.StatusOK, repos)
}

// CreateRepo 新增仓库记录
func CreateRepo(c *gin.Context) {
	log.Printf("CreateRepo: start method=%s path=%s remote=%s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
	var req struct {
		Name               string `json:"name"`
		RootPath           string `json:"root_path"`
		DBFile             string `json:"db_filename"`
		IsInternal         *bool  `json:"is_internal"`
		ExternalDeviceName string `json:"external_device_name"`
		RepoType           string `json:"repo_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("CreateRepo: invalid request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	log.Printf("CreateRepo: request parsed name=%q root=%q dbfile=%q internal=%v extDevice=%q repo_type=%q", req.Name, req.RootPath, req.DBFile, req.IsInternal, req.ExternalDeviceName, req.RepoType)
	if req.Name == "" {
		log.Printf("CreateRepo: validation failed, empty name")
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}

	normalizedRootPath, err := normalizeRepoRootPath(req.RootPath, false)
	if err != nil {
		log.Printf("CreateRepo: invalid root_path=%q error=%v", req.RootPath, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid root_path: " + err.Error()})
		return
	}

	isInternal := true
	if req.IsInternal != nil {
		isInternal = *req.IsInternal
	}

	externalDeviceName := ""
	if !isInternal {
		deviceName, deviceErr := normalizeExternalDeviceName(req.ExternalDeviceName)
		if deviceErr != nil {
			log.Printf("CreateRepo: invalid external_device_name=%q error=%v", req.ExternalDeviceName, deviceErr)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid external_device_name: " + deviceErr.Error()})
			return
		}
		externalDeviceName = deviceName
	}

	repoType, err := normalizeCreateRepoType(req.RepoType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid repo_type: " + err.Error()})
		return
	}

	repo := models.Repository{
		Name:               req.Name,
		RepoTypeKey:        repoType,
		Basic:              false,
		RootPath:           normalizedRootPath,
		DBFile:             req.DBFile,
		IsInternal:         isInternal,
		ExternalDeviceName: externalDeviceName,
	}
	if repo.DBFile == "" {
		repo.DBFile = "repo.db"
		log.Printf("CreateRepo: db_filename empty, fallback to default=%q", repo.DBFile)
	}

	if err := ensureRepoRootExistsForCreate(repo); err != nil {
		log.Printf("CreateRepo: ensure repo root failed error=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "prepare repo root failed: " + err.Error()})
		return
	}

	importedFromExistingRepoDB := false
	importedRepoUUID, importMatched, err := detectExistingRepoUUIDForCreate(repo)
	if err != nil {
		log.Printf("CreateRepo: inspect existing repo db failed root=%q dbfile=%q error=%v", repo.RootPath, repo.DBFile, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "inspect existing repo db failed: " + err.Error()})
		return
	}
	if importMatched {
		repo.RepoUUID = importedRepoUUID
		importedFromExistingRepoDB = true
		log.Printf("CreateRepo: importing existing repo db root=%q dbfile=%q repo_uuid=%q", repo.RootPath, repo.DBFile, repo.RepoUUID)
	} else {
		repoUUID, err := repositoryOrNewUUID("")
		if err != nil {
			log.Printf("CreateRepo: generate repo_uuid failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "generate repo_uuid failed: " + err.Error()})
			return
		}
		repo.RepoUUID = repoUUID
	}

	if err := db.Select("RepoUUID", "Name", "RepoTypeKey", "Basic", "RootPath", "DBFile", "IsInternal", "ExternalDeviceName").Create(&repo).Error; err != nil {
		log.Printf("CreateRepo: insert failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db insert failed: " + err.Error()})
		return
	}

	requestedInternal := isInternal
	requestedExternalDeviceName := externalDeviceName
	requestedRootPath := normalizedRootPath
	requestedRepoTypeKey := repoType
	if err := db.First(&repo, repo.ID).Error; err != nil {
		log.Printf("CreateRepo: reload after insert failed id=%d: %v", repo.ID, err)
		_ = db.Delete(&models.Repository{}, repo.ID).Error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "reload repository after insert failed: " + err.Error()})
		return
	}
	if repo.IsInternal != requestedInternal ||
		repo.ExternalDeviceName != requestedExternalDeviceName ||
		repo.RootPath != requestedRootPath ||
		strings.TrimSpace(repo.RepoTypeKey) != requestedRepoTypeKey {
		log.Printf(
			"CreateRepo: persisted repository mismatch id=%d requested_internal=%t actual_internal=%t requested_external_device=%q actual_external_device=%q requested_root=%q actual_root=%q requested_repo_type=%q actual_repo_type=%q",
			repo.ID,
			requestedInternal,
			repo.IsInternal,
			requestedExternalDeviceName,
			repo.ExternalDeviceName,
			requestedRootPath,
			repo.RootPath,
			requestedRepoTypeKey,
			repo.RepoTypeKey,
		)
		_ = db.Delete(&models.Repository{}, repo.ID).Error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "persisted repository binding mismatch after insert"})
		return
	}

	if err := BootstrapSingleRepository(repo); err != nil {
		log.Printf("CreateRepo: bootstrap repo metadata failed id=%d: %v", repo.ID, err)
		_ = db.Delete(&models.Repository{}, repo.ID).Error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bootstrap repo metadata failed: " + err.Error()})
		return
	}

	if !importedFromExistingRepoDB {
		if err := applyRepoInfoPresetByType(repo, repoType); err != nil {
			log.Printf("CreateRepo: apply repo type preset failed id=%d type=%q: %v", repo.ID, repoType, err)
			_ = db.Delete(&models.Repository{}, repo.ID).Error
			c.JSON(http.StatusInternalServerError, gin.H{"error": "apply repo_type preset failed: " + err.Error()})
			return
		}
	}

	if err := db.First(&repo, repo.ID).Error; err != nil {
		log.Printf("CreateRepo: reload repository after bootstrap failed id=%d: %v", repo.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "reload repository failed: " + err.Error()})
		return
	}

	if !importedFromExistingRepoDB {
		triggerAutoNormalizeForNewRepo(repo)
	}

	log.Printf("CreateRepo: insert success id=%d name=%q root=%q dbfile=%q internal=%t extDevice=%q repo_type=%q imported_repo_db=%t", repo.ID, repo.Name, repo.RootPath, repo.DBFile, repo.IsInternal, repo.ExternalDeviceName, repoType, importedFromExistingRepoDB)
	c.JSON(http.StatusOK, repo)
}

// DeleteRepo 删除仓库全局记录
func DeleteRepo(c *gin.Context) {
	log.Printf("DeleteRepo: start method=%s path=%s remote=%s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}
	if err := db.Delete(&models.Repository{}, id).Error; err != nil {
		log.Printf("DeleteRepo: delete failed id=%s error=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db delete failed: " + err.Error()})
		return
	}
	log.Printf("DeleteRepo: success id=%s", id)
	c.Status(http.StatusNoContent)
}

// UpdateRepo 修改仓库名称
func UpdateRepo(c *gin.Context) {
	log.Printf("UpdateRepo: start method=%s path=%s remote=%s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	var req struct {
		Name  *string `json:"name"`
		Basic *bool   `json:"basic"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("UpdateRepo: invalid request body id=%s error=%v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Name == nil && req.Basic == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one of name or basic is required"})
		return
	}

	var requestedName *string
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			log.Printf("UpdateRepo: validation failed id=%s empty name", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
			return
		}
		requestedName = &trimmed
	}

	var repo models.Repository
	if err := db.First(&repo, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("UpdateRepo: repo not found id=%s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
			return
		}
		log.Printf("UpdateRepo: query failed id=%s error=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db query failed: " + err.Error()})
		return
	}

	oldName := repo.Name
	oldBasic := repo.Basic

	nextName := repo.Name
	if requestedName != nil {
		nextName = *requestedName
	}
	nextBasic := repo.Basic
	if req.Basic != nil {
		nextBasic = *req.Basic
	}

	if nextName == repo.Name && nextBasic == repo.Basic {
		c.JSON(http.StatusOK, repo)
		return
	}

	if strings.TrimSpace(repo.RootPath) == "" {
		repo.Name = nextName
		repo.Basic = nextBasic
		if err := db.Save(&repo).Error; err != nil {
			log.Printf("UpdateRepo: save failed id=%s error=%v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db update failed: " + err.Error()})
			return
		}
		log.Printf("UpdateRepo: root_path unset, updated global cache only id=%s oldName=%q newName=%q oldBasic=%t newBasic=%t", id, oldName, repo.Name, oldBasic, repo.Basic)
		c.JSON(http.StatusOK, repo)
		return
	}

	if err := writeRepoInfoMetadata(repo, nextName, nextBasic); err != nil {
		log.Printf("UpdateRepo: write repo_info failed id=%s error=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update repo metadata failed: " + err.Error()})
		return
	}

	if err := BootstrapSingleRepository(repo); err != nil {
		log.Printf("UpdateRepo: bootstrap repo metadata failed id=%s error=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sync repository cache failed: " + err.Error()})
		return
	}

	if err := db.First(&repo, id).Error; err != nil {
		log.Printf("UpdateRepo: reload failed id=%s error=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "reload updated repository failed: " + err.Error()})
		return
	}

	if repo.Name != nextName || repo.Basic != nextBasic {
		log.Printf("UpdateRepo: metadata write mismatch id=%s expectedName=%q actualName=%q expectedBasic=%t actualBasic=%t", id, nextName, repo.Name, nextBasic, repo.Basic)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("metadata sync mismatch: expected (name=%q,basic=%t) got (name=%q,basic=%t)", nextName, nextBasic, repo.Name, repo.Basic)})
		return
	}

	log.Printf("UpdateRepo: success id=%s oldName=%q newName=%q oldBasic=%t newBasic=%t", id, oldName, repo.Name, oldBasic, repo.Basic)
	c.JSON(http.StatusOK, repo)
}

func toStringUint(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}

func applyRepositoryListOrder(tx *gorm.DB) *gorm.DB {
	return tx.Order("basic DESC").Order("id ASC")
}

func detectExistingRepoUUIDForCreate(repo models.Repository) (string, bool, error) {
	_, dbPath, err := resolveRepoDBPath(repo)
	if err != nil {
		return "", false, err
	}

	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("stat repo db failed: %w", err)
	}

	repoDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return "", false, fmt.Errorf("open existing repo db failed: %w", err)
	}

	if !repoDB.Migrator().HasTable(&models.RepoInfo{}) {
		return "", false, nil
	}

	var info models.RepoInfo
	if err := repoDB.Where("id = ?", repoInfoSingletonID).First(&info).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("query repo_info failed: %w", err)
	}

	repoUUID := strings.TrimSpace(info.RepoUUID)
	if repoUUID == "" {
		return "", false, nil
	}
	return repoUUID, true, nil
}

func resolveRepoDBPath(repo models.Repository) (string, string, error) {
	rootAbs, err := resolveRepoRootAbs(repo)
	if err != nil {
		return "", "", err
	}

	dbFile := strings.TrimSpace(repo.DBFile)
	if dbFile == "" {
		dbFile = "repo.db"
	}
	dbFile = filepath.Base(dbFile)

	dbBaseDir := rootAbs
	boundaryRoot := rootAbs
	if repo.Basic {
		dbBaseDir = filepath.Clean(filepath.FromSlash(strings.TrimSpace(basicRepoDBDir)))
		boundaryRoot = dbBaseDir
		if dbBaseDir == "" || !filepath.IsAbs(dbBaseDir) {
			return "", "", fmt.Errorf("invalid basic repo db dir")
		}
	}

	dbPath := filepath.Join(dbBaseDir, dbFile)
	if !isPathWithinRoot(boundaryRoot, dbPath) {
		return "", "", fmt.Errorf("db file out of boundary")
	}

	return rootAbs, dbPath, nil
}

func triggerAutoNormalizeForNewRepo(repo models.Repository) {
	triggerRepoIncrementalNormalize(repo, "create-repo")
}

func ensureRepoRootExistsForCreate(repo models.Repository) error {
	if repo.Basic {
		return nil
	}

	rootPath := strings.TrimSpace(repo.RootPath)
	if rootPath == "" {
		return fmt.Errorf("repo root_path is empty")
	}

	baseRoot := internalRepoRoot
	boundaryRoot := internalRepoRoot
	if !repo.IsInternal {
		deviceName, err := normalizeExternalDeviceName(repo.ExternalDeviceName)
		if err != nil {
			return fmt.Errorf("invalid external device: %w", err)
		}
		baseRoot = filepath.Join(externalRepoRoot, filepath.FromSlash(deviceName))
		boundaryRoot = externalRepoRoot

		info, err := os.Stat(baseRoot)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("external device not found")
			}
			return fmt.Errorf("stat external device failed: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("external device path is not a directory")
		}
	}

	rootAbs := baseRoot
	if rootPath != "/" {
		rootAbs = filepath.Join(baseRoot, filepath.FromSlash(rootPath))
	}
	if !isPathWithinRoot(boundaryRoot, rootAbs) {
		return fmt.Errorf("repo root out of boundary")
	}

	if err := os.MkdirAll(rootAbs, 0o755); err != nil {
		return fmt.Errorf("create repo root failed: %w", err)
	}

	info, err := os.Stat(rootAbs)
	if err != nil {
		return fmt.Errorf("stat repo root failed: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("repo root is not a directory")
	}

	return nil
}

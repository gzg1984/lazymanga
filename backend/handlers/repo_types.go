package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"lazymanga/models"
	"lazymanga/normalization"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	defaultRepoTypeKey              string = "manga"
	defaultRepoSettingsOverrideJSON string = "{}"
)

var repoTypeKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type repoTypeSettings struct {
	AddButton          bool   `json:"add_button"`
	AddDirectoryButton bool   `json:"add_directory_button"`
	DeleteButton       bool   `json:"delete_button"`
	AutoNormalize      bool   `json:"auto_normalize"`
	ShowMD5            bool   `json:"show_md5"`
	ShowSize           bool   `json:"show_size"`
	SingleMove         bool   `json:"single_move"`
	RuleBookName       string `json:"rulebook_name"`
	RuleBookVersion    string `json:"rulebook_version"`
}

type repoTypeSettingsOverride struct {
	AddButton          *bool   `json:"add_button,omitempty"`
	AddDirectoryButton *bool   `json:"add_directory_button,omitempty"`
	DeleteButton       *bool   `json:"delete_button,omitempty"`
	AutoNormalize      *bool   `json:"auto_normalize,omitempty"`
	ShowMD5            *bool   `json:"show_md5,omitempty"`
	ShowSize           *bool   `json:"show_size,omitempty"`
	SingleMove         *bool   `json:"single_move,omitempty"`
	RuleBookName       *string `json:"rulebook_name,omitempty"`
	RuleBookVersion    *string `json:"rulebook_version,omitempty"`
}

type repoTypeUpsertPayload struct {
	Key                string `json:"key"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	Enabled            *bool  `json:"enabled"`
	SortOrder          *int   `json:"sort_order"`
	AddButton          *bool  `json:"add_button"`
	AddDirectoryButton *bool  `json:"add_directory_button"`
	DeleteButton       *bool  `json:"delete_button"`
	AutoNormalize      *bool  `json:"auto_normalize"`
	ShowMD5            *bool  `json:"show_md5"`
	ShowSize           *bool  `json:"show_size"`
	SingleMove         *bool  `json:"single_move"`
	RuleBookName       string `json:"rulebook_name"`
	RuleBookVersion    string `json:"rulebook_version"`
}

type repoTypeSettingsUpdatePayload struct {
	RepoTypeKey      string          `json:"repo_type_key"`
	SettingsOverride json.RawMessage `json:"settings_override"`
}

func EnsureDefaultRepoTypes() error {
	if db == nil {
		return errors.New("database is not initialized")
	}

	defaults := []models.RepoTypeDef{
		{
			Key:                repoTypeNone,
			Name:               "无类型",
			Description:        "适合手工管理的通用仓库模板",
			Enabled:            true,
			SortOrder:          10,
			AddButton:          true,
			AddDirectoryButton: false,
			DeleteButton:       true,
			AutoNormalize:      false,
			ShowMD5:            true,
			ShowSize:           true,
			SingleMove:         true,
			RuleBookName:       "noop",
			RuleBookVersion:    "v1",
		},
		{
			Key:                defaultRepoTypeKey,
			Name:               "漫画仓库",
			Description:        "面向漫画资料管理的默认仓库模板",
			Enabled:            true,
			SortOrder:          20,
			AddButton:          false,
			AddDirectoryButton: true,
			DeleteButton:       true,
			AutoNormalize:      false,
			ShowMD5:            false,
			ShowSize:           true,
			SingleMove:         true,
			RuleBookName:       "noop",
			RuleBookVersion:    "v1",
		},
		{
			Key:                repoTypeOS,
			Name:               "操作系统镜像库",
			Description:        "兼容旧版系统镜像整理流程的仓库模板",
			Enabled:            true,
			SortOrder:          90,
			AddButton:          false,
			AddDirectoryButton: false,
			DeleteButton:       false,
			AutoNormalize:      true,
			ShowMD5:            false,
			ShowSize:           false,
			SingleMove:         false,
			RuleBookName:       "default-os-relocation",
			RuleBookVersion:    "v1",
		},
	}

	for _, item := range defaults {
		var existing models.RepoTypeDef
		err := db.Where("key = ?", item.Key).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if createErr := db.Create(&item).Error; createErr != nil {
				return fmt.Errorf("create default repo type %q failed: %w", item.Key, createErr)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("query default repo type %q failed: %w", item.Key, err)
		}

		existing.Name = item.Name
		existing.Description = item.Description
		existing.Enabled = item.Enabled
		existing.SortOrder = item.SortOrder
		existing.AddButton = item.AddButton
		existing.AddDirectoryButton = item.AddDirectoryButton
		existing.DeleteButton = item.DeleteButton
		existing.AutoNormalize = item.AutoNormalize
		existing.ShowMD5 = item.ShowMD5
		existing.ShowSize = item.ShowSize
		existing.SingleMove = item.SingleMove
		existing.RuleBookName = item.RuleBookName
		existing.RuleBookVersion = item.RuleBookVersion
		if saveErr := db.Save(&existing).Error; saveErr != nil {
			return fmt.Errorf("update default repo type %q failed: %w", item.Key, saveErr)
		}
	}

	return nil
}

func normalizeRepoTypeKey(key string) (string, error) {
	v := strings.TrimSpace(strings.ToLower(key))
	if v == "" {
		return "", errors.New("repo type key required")
	}
	if !repoTypeKeyPattern.MatchString(v) {
		return "", errors.New("repo type key must match ^[a-z0-9][a-z0-9-]*$")
	}
	return v, nil
}

func inferRepoTypeKeyFromInfo(info models.RepoInfo, repo models.Repository) string {
	if v := strings.TrimSpace(strings.ToLower(info.RepoTypeKey)); v != "" {
		return v
	}
	if v := strings.TrimSpace(strings.ToLower(repo.RepoTypeKey)); v != "" {
		return v
	}
	if info.AutoNormalize && !info.AddButton && !info.DeleteButton && !info.ShowMD5 && !info.ShowSize && !info.SingleMove {
		return repoTypeOS
	}
	if info.Basic || repo.Basic {
		return repoTypeNone
	}
	return defaultRepoTypeKey
}

func repoTypeDefToSettings(def models.RepoTypeDef) repoTypeSettings {
	return repoTypeSettings{
		AddButton:          def.AddButton,
		AddDirectoryButton: def.AddDirectoryButton,
		DeleteButton:       def.DeleteButton,
		AutoNormalize:      def.AutoNormalize,
		ShowMD5:            def.ShowMD5,
		ShowSize:           def.ShowSize,
		SingleMove:         def.SingleMove,
		RuleBookName:       strings.TrimSpace(def.RuleBookName),
		RuleBookVersion:    strings.TrimSpace(def.RuleBookVersion),
	}
}

func repoInfoFallbackSettings(info models.RepoInfo) repoTypeSettings {
	binding := normalization.ResolveEffectiveRuleBookBinding(info)
	return repoTypeSettings{
		AddButton:          info.AddButton,
		AddDirectoryButton: info.AddDirectoryButton,
		DeleteButton:       info.DeleteButton,
		AutoNormalize:      info.AutoNormalize,
		ShowMD5:            info.ShowMD5,
		ShowSize:           info.ShowSize,
		SingleMove:         info.SingleMove,
		RuleBookName:       binding.Name,
		RuleBookVersion:    binding.Version,
	}
}

func applyRepoSettingsOverride(base repoTypeSettings, override repoTypeSettingsOverride) repoTypeSettings {
	result := base
	if override.AddButton != nil {
		result.AddButton = *override.AddButton
	}
	if override.AddDirectoryButton != nil {
		result.AddDirectoryButton = *override.AddDirectoryButton
	}
	if override.DeleteButton != nil {
		result.DeleteButton = *override.DeleteButton
	}
	if override.AutoNormalize != nil {
		result.AutoNormalize = *override.AutoNormalize
	}
	if override.ShowMD5 != nil {
		result.ShowMD5 = *override.ShowMD5
	}
	if override.ShowSize != nil {
		result.ShowSize = *override.ShowSize
	}
	if override.SingleMove != nil {
		result.SingleMove = *override.SingleMove
	}
	if override.RuleBookName != nil {
		result.RuleBookName = strings.TrimSpace(*override.RuleBookName)
	}
	if override.RuleBookVersion != nil {
		result.RuleBookVersion = strings.TrimSpace(*override.RuleBookVersion)
	}
	return result
}

func parseRepoSettingsOverrideJSON(raw string) (repoTypeSettingsOverride, error) {
	var override repoTypeSettingsOverride
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return override, nil
	}
	if err := json.Unmarshal([]byte(trimmed), &override); err != nil {
		return repoTypeSettingsOverride{}, err
	}
	return override, nil
}

func parseRepoSettingsOverrideRaw(raw json.RawMessage) (repoTypeSettingsOverride, error) {
	if len(raw) == 0 {
		return repoTypeSettingsOverride{}, nil
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return repoTypeSettingsOverride{}, nil
	}
	var override repoTypeSettingsOverride
	if err := json.Unmarshal(raw, &override); err != nil {
		return repoTypeSettingsOverride{}, err
	}
	return override, nil
}

func canonicalizeRepoSettingsOverride(override repoTypeSettingsOverride) (string, error) {
	encoded, err := json.Marshal(override)
	if err != nil {
		return "", err
	}
	if string(encoded) == "null" || string(encoded) == "" {
		return defaultRepoSettingsOverrideJSON, nil
	}
	return string(encoded), nil
}

func findRepoTypeDefByKey(key string) (models.RepoTypeDef, error) {
	if db == nil {
		return models.RepoTypeDef{}, errors.New("database is not initialized")
	}
	var def models.RepoTypeDef
	if err := db.Where("key = ?", key).First(&def).Error; err != nil {
		return models.RepoTypeDef{}, err
	}
	return def, nil
}

func resolveRepoTypeForCreate(repoType string) (string, models.RepoTypeDef, error) {
	if err := EnsureDefaultRepoTypes(); err != nil {
		return "", models.RepoTypeDef{}, err
	}

	key := strings.TrimSpace(strings.ToLower(repoType))
	if key == "" {
		key = defaultRepoTypeKey
	}
	key, err := normalizeRepoTypeKey(key)
	if err != nil {
		return "", models.RepoTypeDef{}, err
	}

	def, err := findRepoTypeDefByKey(key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", models.RepoTypeDef{}, fmt.Errorf("unknown repo type %q", key)
		}
		return "", models.RepoTypeDef{}, err
	}
	if !def.Enabled {
		return "", models.RepoTypeDef{}, fmt.Errorf("repo type %q is disabled", key)
	}
	return key, def, nil
}

func applyEffectiveSettingsToRepoInfo(info *models.RepoInfo, repoTypeKey string, override repoTypeSettingsOverride, effective repoTypeSettings) (bool, error) {
	if info == nil {
		return false, errors.New("repo info is nil")
	}

	nextOverrideJSON, err := canonicalizeRepoSettingsOverride(override)
	if err != nil {
		return false, err
	}

	changed := false
	if info.RepoTypeKey != repoTypeKey {
		info.RepoTypeKey = repoTypeKey
		changed = true
	}
	if strings.TrimSpace(info.SettingsOverrideJSON) != nextOverrideJSON {
		info.SettingsOverrideJSON = nextOverrideJSON
		changed = true
	}
	if info.AddButton != effective.AddButton {
		info.AddButton = effective.AddButton
		changed = true
	}
	if info.AddDirectoryButton != effective.AddDirectoryButton {
		info.AddDirectoryButton = effective.AddDirectoryButton
		changed = true
	}
	if info.DeleteButton != effective.DeleteButton {
		info.DeleteButton = effective.DeleteButton
		changed = true
	}
	if info.AutoNormalize != effective.AutoNormalize {
		info.AutoNormalize = effective.AutoNormalize
		changed = true
	}
	if info.ShowMD5 != effective.ShowMD5 {
		info.ShowMD5 = effective.ShowMD5
		changed = true
	}
	if info.ShowSize != effective.ShowSize {
		info.ShowSize = effective.ShowSize
		changed = true
	}
	if info.SingleMove != effective.SingleMove {
		info.SingleMove = effective.SingleMove
		changed = true
	}
	if setRepoRulebookBindingOnInfo(info, effective.RuleBookName, effective.RuleBookVersion) {
		changed = true
	}

	return changed, nil
}

func updateRepositoryRepoTypeKey(repoID uint, repoTypeKey string) error {
	if db == nil || repoID == 0 {
		return nil
	}
	return db.Model(&models.Repository{}).Where("id = ?", repoID).Update("repo_type_key", repoTypeKey).Error
}

func resolveEffectiveRepoTypeSettings(info models.RepoInfo, repo models.Repository) (string, *models.RepoTypeDef, repoTypeSettingsOverride, repoTypeSettings, string, error) {
	fallback := repoInfoFallbackSettings(info)
	repoTypeKey := inferRepoTypeKeyFromInfo(info, repo)
	override, err := parseRepoSettingsOverrideJSON(info.SettingsOverrideJSON)
	if err != nil {
		return repoTypeKey, nil, repoTypeSettingsOverride{}, fallback, "invalid settings_override_json, fallback to repo_info", nil
	}

	def, err := findRepoTypeDefByKey(repoTypeKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			effective := applyRepoSettingsOverride(fallback, override)
			return repoTypeKey, nil, override, effective, "repo type template not found, fallback to repo_info", nil
		}
		return repoTypeKey, nil, repoTypeSettingsOverride{}, fallback, "", err
	}

	effective := applyRepoSettingsOverride(repoTypeDefToSettings(def), override)
	return repoTypeKey, &def, override, effective, "template + overlay", nil
}

func ListRepoTypes(c *gin.Context) {
	if err := EnsureDefaultRepoTypes(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "prepare repo types failed: " + err.Error()})
		return
	}

	includeDisabled := strings.EqualFold(strings.TrimSpace(c.Query("include_disabled")), "true") || c.Query("all") == "1"
	query := db.Model(&models.RepoTypeDef{}).Order("sort_order asc").Order("id asc")
	if !includeDisabled {
		query = query.Where("enabled = ?", true)
	}

	var items []models.RepoTypeDef
	if err := query.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query repo types failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total": len(items),
		"items": items,
	})
}

func CreateRepoType(c *gin.Context) {
	if err := EnsureDefaultRepoTypes(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "prepare repo types failed: " + err.Error()})
		return
	}

	var req repoTypeUpsertPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	key, err := normalizeRepoTypeKey(req.Key)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}

	rulebookName, rulebookVersion, err := normalizeBindingInput(req.RuleBookName, req.RuleBookVersion)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, _, err := normalization.ValidateRuleBookSpec(rulebookName, rulebookVersion); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rulebook spec invalid: " + err.Error()})
		return
	}

	var count int64
	if err := db.Model(&models.RepoTypeDef{}).Where("key = ?", key).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query repo type failed: " + err.Error()})
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "repo type already exists"})
		return
	}

	item := models.RepoTypeDef{
		Key:                key,
		Name:               name,
		Description:        strings.TrimSpace(req.Description),
		Enabled:            req.Enabled == nil || *req.Enabled,
		SortOrder:          0,
		AddButton:          req.AddButton != nil && *req.AddButton,
		AddDirectoryButton: req.AddDirectoryButton != nil && *req.AddDirectoryButton,
		DeleteButton:       req.DeleteButton != nil && *req.DeleteButton,
		AutoNormalize:      req.AutoNormalize != nil && *req.AutoNormalize,
		ShowMD5:            req.ShowMD5 != nil && *req.ShowMD5,
		ShowSize:           req.ShowSize != nil && *req.ShowSize,
		SingleMove:         req.SingleMove != nil && *req.SingleMove,
		RuleBookName:       rulebookName,
		RuleBookVersion:    rulebookVersion,
	}
	if req.SortOrder != nil {
		item.SortOrder = *req.SortOrder
	}

	if err := db.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create repo type failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, item)
}

func UpdateRepoType(c *gin.Context) {
	key, err := normalizeRepoTypeKey(c.Param("key"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var item models.RepoTypeDef
	if err := db.Where("key = ?", key).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "repo type not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query repo type failed: " + err.Error()})
		return
	}

	var req repoTypeUpsertPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if name := strings.TrimSpace(req.Name); name != "" {
		item.Name = name
	}
	item.Description = strings.TrimSpace(req.Description)
	if req.Enabled != nil {
		item.Enabled = *req.Enabled
	}
	if req.SortOrder != nil {
		item.SortOrder = *req.SortOrder
	}
	if req.AddButton != nil {
		item.AddButton = *req.AddButton
	}
	if req.AddDirectoryButton != nil {
		item.AddDirectoryButton = *req.AddDirectoryButton
	}
	if req.DeleteButton != nil {
		item.DeleteButton = *req.DeleteButton
	}
	if req.AutoNormalize != nil {
		item.AutoNormalize = *req.AutoNormalize
	}
	if req.ShowMD5 != nil {
		item.ShowMD5 = *req.ShowMD5
	}
	if req.ShowSize != nil {
		item.ShowSize = *req.ShowSize
	}
	if req.SingleMove != nil {
		item.SingleMove = *req.SingleMove
	}
	if req.RuleBookName != "" || req.RuleBookVersion != "" {
		rulebookName, rulebookVersion, err := normalizeBindingInput(req.RuleBookName, req.RuleBookVersion)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if _, _, err := normalization.ValidateRuleBookSpec(rulebookName, rulebookVersion); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "rulebook spec invalid: " + err.Error()})
			return
		}
		item.RuleBookName = rulebookName
		item.RuleBookVersion = rulebookVersion
	}

	if strings.TrimSpace(item.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}

	if err := db.Save(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update repo type failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, item)
}

func DeleteRepoType(c *gin.Context) {
	key, err := normalizeRepoTypeKey(c.Param("key"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if key == repoTypeNone || key == defaultRepoTypeKey || key == repoTypeOS {
		c.JSON(http.StatusBadRequest, gin.H{"error": "built-in repo type cannot be deleted"})
		return
	}

	var count int64
	if err := db.Model(&models.Repository{}).Where("repo_type_key = ?", key).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query repo usage failed: " + err.Error()})
		return
	}

	if count > 0 {
		if err := db.Model(&models.RepoTypeDef{}).Where("key = ?", key).Update("enabled", false).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "disable repo type failed: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message":      "repo type is still in use and has been disabled instead of deleted",
			"repo_type_key": key,
			"in_use_count": count,
		})
		return
	}

	res := db.Where("key = ?", key).Delete(&models.RepoTypeDef{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete repo type failed: " + res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo type not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

func GetRepoTypeSettings(c *gin.Context) {
	repo, _, info, ok := readRepoInfoByID(c)
	if !ok {
		return
	}

	repoTypeKey, def, override, effective, note, err := resolveEffectiveRepoTypeSettings(info, repo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "resolve repo type settings failed: " + err.Error()})
		return
	}

	resp := gin.H{
		"repo_id":          repo.ID,
		"repo_type_key":    repoTypeKey,
		"settings_override": override,
		"effective":        effective,
		"resolution_note":  note,
	}
	if def != nil {
		resp["template"] = def
	}

	c.JSON(http.StatusOK, resp)
}

func UpdateRepoTypeSettings(c *gin.Context) {
	repo, repoDB, info, ok := readRepoInfoByID(c)
	if !ok {
		return
	}

	var req repoTypeSettingsUpdatePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	currentKey, _, _, _, _, err := resolveEffectiveRepoTypeSettings(info, repo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "resolve current repo type settings failed: " + err.Error()})
		return
	}

	nextKey := strings.TrimSpace(strings.ToLower(req.RepoTypeKey))
	if nextKey == "" {
		nextKey = currentKey
	}
	key, def, err := resolveRepoTypeForCreate(nextKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	override, err := parseRepoSettingsOverrideRaw(req.SettingsOverride)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid settings_override: " + err.Error()})
		return
	}

	effective := applyRepoSettingsOverride(repoTypeDefToSettings(def), override)
	changed, err := applyEffectiveSettingsToRepoInfo(&info, key, override, effective)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "apply repo type settings failed: " + err.Error()})
		return
	}

	if changed {
		if err := repoDB.Save(&info).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "save repo type settings failed: " + err.Error()})
			return
		}
	}
	if err := updateRepositoryRepoTypeKey(repo.ID, key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update repository repo_type_key failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":           "repo type settings updated",
		"repo_id":           repo.ID,
		"repo_type_key":     key,
		"template":          def,
		"settings_override": override,
		"effective":         effective,
		"changed":           changed,
	})
}

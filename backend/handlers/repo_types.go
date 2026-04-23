package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"lazymanga/models"
	"lazymanga/normalization"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	defaultRepoTypeKey              string = "manga"
	manualMangaRepoTypeKey          string = "manga-manual"
	manualMangaRuleBookName         string = "manga-manual"
	karitaRepoTypeKey               string = "karita-manga"
	defaultRepoSettingsOverrideJSON string = "{}"
	defaultArchiveSubdir            string = "archives"
	defaultMaterializedSubdir       string = "/"
	defaultArchiveExtensionsCSV     string = ".zip,.rar,.7z,.cbz,.cbr"
	manualEditorModeLegacy          string = "legacy-type-editor"
	manualEditorModeMetadata        string = "metadata-editor"
	metadataDisplayModeHidden       string = "hidden"
	metadataDisplayModeAuto         string = "auto"
	metadataDisplayModeSelected     string = "selected"
)

var repoTypeKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

var defaultMetadataDisplayFields = []string{
	"title",
	"series_name",
	"scanlator_group",
	"author_name",
	"author_alias",
	"original_work",
	"event_code",
	"comic_market",
	"year",
	"karita_id",
}

func isRepoTypeHiddenFromPublicViews(key string) bool {
	switch strings.TrimSpace(strings.ToLower(key)) {
	case defaultRepoTypeKey, repoTypeNone:
		return true
	default:
		return false
	}
}

func filterPublicRepoTypes(items []models.RepoTypeDef) []models.RepoTypeDef {
	if len(items) == 0 {
		return nil
	}
	filtered := make([]models.RepoTypeDef, 0, len(items))
	for _, item := range items {
		if isRepoTypeHiddenFromPublicViews(item.Key) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func defaultManualEditorModeForRepo(repoTypeKey string, ruleBookName string) string {
	key := strings.TrimSpace(strings.ToLower(repoTypeKey))
	rulebook := strings.TrimSpace(strings.ToLower(ruleBookName))
	if key == defaultRepoTypeKey || key == manualMangaRepoTypeKey || key == karitaRepoTypeKey || rulebook == manualMangaRuleBookName || rulebook == "karita-manga" || rulebook == "manga-files" {
		return manualEditorModeMetadata
	}
	return manualEditorModeLegacy
}

func normalizeManualEditorMode(mode string, repoTypeKey string, ruleBookName string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "metadata", manualEditorModeMetadata:
		return manualEditorModeMetadata
	case "legacy", manualEditorModeLegacy:
		return manualEditorModeLegacy
	default:
		return defaultManualEditorModeForRepo(repoTypeKey, ruleBookName)
	}
}

func validateManualEditorMode(mode string, repoTypeKey string, ruleBookName string) (string, error) {
	trimmed := strings.TrimSpace(strings.ToLower(mode))
	if trimmed == "" {
		return normalizeManualEditorMode("", repoTypeKey, ruleBookName), nil
	}
	if trimmed == "metadata" || trimmed == manualEditorModeMetadata {
		return manualEditorModeMetadata, nil
	}
	if trimmed == "legacy" || trimmed == manualEditorModeLegacy {
		return manualEditorModeLegacy, nil
	}
	return "", fmt.Errorf("manual_editor_mode must be %s or %s", manualEditorModeLegacy, manualEditorModeMetadata)
}

func defaultMetadataDisplayModeForRepo(manualEditorMode string, repoTypeKey string, ruleBookName string) string {
	mode := normalizeManualEditorMode(manualEditorMode, repoTypeKey, ruleBookName)
	if mode == manualEditorModeMetadata {
		return metadataDisplayModeSelected
	}
	return metadataDisplayModeHidden
}

func normalizeMetadataDisplayMode(mode string, manualEditorMode string, repoTypeKey string, ruleBookName string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case metadataDisplayModeHidden, "none", "off":
		return metadataDisplayModeHidden
	case metadataDisplayModeAuto:
		return metadataDisplayModeAuto
	case metadataDisplayModeSelected, "fields", "custom":
		return metadataDisplayModeSelected
	default:
		return defaultMetadataDisplayModeForRepo(manualEditorMode, repoTypeKey, ruleBookName)
	}
}

func validateMetadataDisplayMode(mode string, manualEditorMode string, repoTypeKey string, ruleBookName string) (string, error) {
	trimmed := strings.TrimSpace(strings.ToLower(mode))
	if trimmed == "" {
		return normalizeMetadataDisplayMode("", manualEditorMode, repoTypeKey, ruleBookName), nil
	}
	if trimmed == metadataDisplayModeHidden || trimmed == "none" || trimmed == "off" {
		return metadataDisplayModeHidden, nil
	}
	if trimmed == metadataDisplayModeAuto {
		return metadataDisplayModeAuto, nil
	}
	if trimmed == metadataDisplayModeSelected || trimmed == "fields" || trimmed == "custom" {
		return metadataDisplayModeSelected, nil
	}
	return "", fmt.Errorf("metadata_display_mode must be %s, %s or %s", metadataDisplayModeHidden, metadataDisplayModeAuto, metadataDisplayModeSelected)
}

func defaultMetadataDisplayFieldsCSV(manualEditorMode string, repoTypeKey string, ruleBookName string) string {
	if defaultMetadataDisplayModeForRepo(manualEditorMode, repoTypeKey, ruleBookName) != metadataDisplayModeSelected {
		return ""
	}
	return strings.Join(defaultMetadataDisplayFields, ",")
}

func canonicalizeMetadataDisplayFieldsCSV(raw string) string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '，', ';', '；', '\n', '\r', '\t':
			return true
		default:
			return false
		}
	})
	if len(parts) == 0 {
		return ""
	}
	seen := make(map[string]struct{}, len(parts))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key == "" || strings.HasPrefix(key, "_") {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return strings.Join(result, ",")
}

func resolveMetadataDisplayFields(raw string, metadataDisplayMode string, manualEditorMode string, repoTypeKey string, ruleBookName string) string {
	canonical := canonicalizeMetadataDisplayFieldsCSV(raw)
	switch normalizeMetadataDisplayMode(metadataDisplayMode, manualEditorMode, repoTypeKey, ruleBookName) {
	case metadataDisplayModeSelected:
		if canonical != "" {
			return canonical
		}
		return defaultMetadataDisplayFieldsCSV(manualEditorMode, repoTypeKey, ruleBookName)
	default:
		return canonical
	}
}

func normalizeArchiveSubdir(raw string) (string, error) {
	v := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if v == "" {
		v = defaultArchiveSubdir
	}
	v = strings.Trim(v, "/")
	if v == "" {
		return "", errors.New("archive_subdir must not be root")
	}
	if strings.Contains(v, "..") {
		return "", errors.New("archive_subdir must be a repository-relative path")
	}
	return v, nil
}

func normalizeMaterializedSubdir(raw string) (string, error) {
	v := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if v == "" || v == "/" {
		return defaultMaterializedSubdir, nil
	}
	v = strings.Trim(v, "/")
	if v == "" {
		return defaultMaterializedSubdir, nil
	}
	if strings.Contains(v, "..") {
		return "", errors.New("materialized_subdir must be root or a repository-relative path")
	}
	return v, nil
}

func canonicalizeArchiveExtensionsCSV(raw string) string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '，', ';', '；', '\n', '\r', '\t':
			return true
		default:
			return false
		}
	})
	if len(parts) == 0 {
		return defaultArchiveExtensionsCSV
	}
	seen := make(map[string]struct{}, len(parts))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.ToLower(strings.TrimSpace(part))
		if value == "" {
			continue
		}
		if !strings.HasPrefix(value, ".") {
			value = "." + value
		}
		if value == "." {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	if len(result) == 0 {
		return defaultArchiveExtensionsCSV
	}
	return strings.Join(result, ",")
}

func validateArchiveSettings(archiveSubdir string, materializedSubdir string) (string, string, error) {
	normalizedArchive, err := normalizeArchiveSubdir(archiveSubdir)
	if err != nil {
		return "", "", err
	}
	normalizedMaterialized, err := normalizeMaterializedSubdir(materializedSubdir)
	if err != nil {
		return "", "", err
	}
	if normalizedMaterialized != defaultMaterializedSubdir {
		if normalizedArchive == normalizedMaterialized || strings.HasPrefix(normalizedArchive+"/", normalizedMaterialized+"/") || strings.HasPrefix(normalizedMaterialized+"/", normalizedArchive+"/") {
			return "", "", errors.New("archive_subdir and materialized_subdir must not overlap")
		}
	}
	return normalizedArchive, normalizedMaterialized, nil
}

func validateArchiveOverride(base repoTypeSettings, override repoTypeSettingsOverride) error {
	archiveSubdir := base.ArchiveSubdir
	if override.ArchiveSubdir != nil {
		archiveSubdir = *override.ArchiveSubdir
	}
	materializedSubdir := base.MaterializedSubdir
	if override.MaterializedSubdir != nil {
		materializedSubdir = *override.MaterializedSubdir
	}
	_, _, err := validateArchiveSettings(archiveSubdir, materializedSubdir)
	return err
}

type repoArchivePaths struct {
	ArchiveSubdir      string
	MaterializedSubdir string
	ArchiveRootAbs     string
	MaterializedRootAbs string
	ExcludeRootAbsPaths []string
}

func resolveRepoArchivePaths(rootAbs string, settings repoTypeSettings) (repoArchivePaths, error) {
	archiveSubdir, materializedSubdir, err := validateArchiveSettings(settings.ArchiveSubdir, settings.MaterializedSubdir)
	if err != nil {
		return repoArchivePaths{}, err
	}
	paths := repoArchivePaths{
		ArchiveSubdir:      archiveSubdir,
		MaterializedSubdir: materializedSubdir,
		ArchiveRootAbs:     filepath.Join(rootAbs, filepath.FromSlash(archiveSubdir)),
	}
	if materializedSubdir == defaultMaterializedSubdir {
		paths.MaterializedRootAbs = rootAbs
		paths.ExcludeRootAbsPaths = []string{paths.ArchiveRootAbs}
		return paths, nil
	}
	paths.MaterializedRootAbs = filepath.Join(rootAbs, filepath.FromSlash(materializedSubdir))
	return paths, nil
}

type repoTypeSettings struct {
	AddButton          bool   `json:"add_button"`
	AddDirectoryButton bool   `json:"add_directory_button"`
	DeleteButton       bool   `json:"delete_button"`
	AutoNormalize      bool   `json:"auto_normalize"`
	ShowMD5            bool   `json:"show_md5"`
	ShowSize           bool   `json:"show_size"`
	SingleMove         bool   `json:"single_move"`
	ManualEditorMode   string `json:"manual_editor_mode"`
	MetadataDisplayMode   string `json:"metadata_display_mode"`
	MetadataDisplayFields string `json:"metadata_display_fields"`
	ArchiveSubdir      string `json:"archive_subdir"`
	MaterializedSubdir string `json:"materialized_subdir"`
	ArchiveExtensions  string `json:"archive_extensions"`
	ArchiveReadInnerLayout bool `json:"archive_read_inner_layout"`
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
	ManualEditorMode   *string `json:"manual_editor_mode,omitempty"`
	MetadataDisplayMode   *string `json:"metadata_display_mode,omitempty"`
	MetadataDisplayFields *string `json:"metadata_display_fields,omitempty"`
	ArchiveSubdir      *string `json:"archive_subdir,omitempty"`
	MaterializedSubdir *string `json:"materialized_subdir,omitempty"`
	ArchiveExtensions  *string `json:"archive_extensions,omitempty"`
	ArchiveReadInnerLayout *bool `json:"archive_read_inner_layout,omitempty"`
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
	ManualEditorMode   string `json:"manual_editor_mode"`
	MetadataDisplayMode   string `json:"metadata_display_mode"`
	MetadataDisplayFields string `json:"metadata_display_fields"`
	ArchiveSubdir      string `json:"archive_subdir"`
	MaterializedSubdir string `json:"materialized_subdir"`
	ArchiveExtensions  string `json:"archive_extensions"`
	ArchiveReadInnerLayout *bool `json:"archive_read_inner_layout"`
	RuleBookName       string `json:"rulebook_name"`
	RuleBookVersion    string `json:"rulebook_version"`
}

type repoTypeSettingsUpdatePayload struct {
	RepoTypeKey      string          `json:"repo_type_key"`
	SettingsOverride json.RawMessage `json:"settings_override"`
}

func defaultRepoTypeDefinitions() []models.RepoTypeDef {
	return []models.RepoTypeDef{
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
			ManualEditorMode:   manualEditorModeLegacy,
			MetadataDisplayMode:   metadataDisplayModeHidden,
			MetadataDisplayFields: "",
			ArchiveSubdir:      defaultArchiveSubdir,
			MaterializedSubdir: defaultMaterializedSubdir,
			ArchiveExtensions:  defaultArchiveExtensionsCSV,
			ArchiveReadInnerLayout: true,
			RuleBookName:       "noop",
			RuleBookVersion:    "v1",
		},
		{
			Key:                manualMangaRepoTypeKey,
			Name:               "手工漫画管理",
			Description:        "不做自动整理，但按漫画文件与图片目录规则进行筛选和索引的仓库模板",
			Enabled:            true,
			SortOrder:          15,
			AddButton:          true,
			AddDirectoryButton: true,
			DeleteButton:       true,
			AutoNormalize:      false,
			ShowMD5:            false,
			ShowSize:           true,
			SingleMove:         true,
			ManualEditorMode:   manualEditorModeMetadata,
			MetadataDisplayMode:   metadataDisplayModeSelected,
			MetadataDisplayFields: strings.Join(defaultMetadataDisplayFields, ","),
			ArchiveSubdir:      defaultArchiveSubdir,
			MaterializedSubdir: defaultMaterializedSubdir,
			ArchiveExtensions:  defaultArchiveExtensionsCSV,
			ArchiveReadInnerLayout: true,
			RuleBookName:       manualMangaRuleBookName,
			RuleBookVersion:    "v1",
		},
		{
			Key:                defaultRepoTypeKey,
			Name:               "漫画仓库",
			Description:        "面向漫画资料管理的默认仓库模板",
			Enabled:            true,
			SortOrder:          20,
			AddButton:          false,
			AddDirectoryButton: false,
			DeleteButton:       true,
			AutoNormalize:      false,
			ShowMD5:            false,
			ShowSize:           true,
			SingleMove:         true,
			ManualEditorMode:   manualEditorModeMetadata,
			MetadataDisplayMode:   metadataDisplayModeSelected,
			MetadataDisplayFields: strings.Join(defaultMetadataDisplayFields, ","),
			ArchiveSubdir:      defaultArchiveSubdir,
			MaterializedSubdir: defaultMaterializedSubdir,
			ArchiveExtensions:  defaultArchiveExtensionsCSV,
			ArchiveReadInnerLayout: true,
			RuleBookName:       "manga-files",
			RuleBookVersion:    "v1",
		},
		{
			Key:                karitaRepoTypeKey,
			Name:               "Karita漫画仓库",
			Description:        "使用 karita 目录改名与 sidecar 元数据规则书的漫画仓库模板",
			Enabled:            true,
			SortOrder:          30,
			AddButton:          false,
			AddDirectoryButton: false,
			DeleteButton:       true,
			AutoNormalize:      true,
			ShowMD5:            false,
			ShowSize:           true,
			SingleMove:         true,
			ManualEditorMode:   manualEditorModeMetadata,
			MetadataDisplayMode:   metadataDisplayModeSelected,
			MetadataDisplayFields: strings.Join(defaultMetadataDisplayFields, ","),
			ArchiveSubdir:      defaultArchiveSubdir,
			MaterializedSubdir: defaultMaterializedSubdir,
			ArchiveExtensions:  defaultArchiveExtensionsCSV,
			ArchiveReadInnerLayout: true,
			RuleBookName:       "karita-manga",
			RuleBookVersion:    "v1",
		},/*
		{
			Key:                repoTypeOS,
			Name:               "操作系统元素库",
			Description:        "兼容旧版系统元素整理流程的仓库模板（默认停用）",
			Enabled:            false,
			SortOrder:          90,
			AddButton:          false,
			AddDirectoryButton: false,
			DeleteButton:       false,
			AutoNormalize:      true,
			ShowMD5:            false,
			ShowSize:           false,
			SingleMove:         false,
			ManualEditorMode:   manualEditorModeLegacy,
			RuleBookName:       "default-os-relocation",
			RuleBookVersion:    "v1",
		},*/
	}
}

func EnsureDefaultRepoTypes() error {
	if db == nil {
		return errors.New("database is not initialized")
	}

	defaults := defaultRepoTypeDefinitions()

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
		existing.ManualEditorMode = item.ManualEditorMode
		existing.MetadataDisplayMode = item.MetadataDisplayMode
		existing.MetadataDisplayFields = item.MetadataDisplayFields
		existing.ArchiveSubdir = item.ArchiveSubdir
		existing.MaterializedSubdir = item.MaterializedSubdir
		existing.ArchiveExtensions = item.ArchiveExtensions
		existing.ArchiveReadInnerLayout = item.ArchiveReadInnerLayout
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
	isBasic := info.Basic || repo.Basic
	if isBasic {
		if v := strings.TrimSpace(strings.ToLower(info.RepoTypeKey)); v != "" && v != repoTypeNone {
			return v
		}
		if v := strings.TrimSpace(strings.ToLower(repo.RepoTypeKey)); v != "" && v != repoTypeNone {
			return v
		}
		return manualMangaRepoTypeKey
	}
	if v := strings.TrimSpace(strings.ToLower(info.RepoTypeKey)); v != "" {
		return v
	}
	if v := strings.TrimSpace(strings.ToLower(repo.RepoTypeKey)); v != "" {
		return v
	}
	if info.AutoNormalize && !info.AddButton && !info.DeleteButton && !info.ShowMD5 && !info.ShowSize && !info.SingleMove {
		return repoTypeOS
	}
	return defaultRepoTypeKey
}

func repoTypeDefToSettings(def models.RepoTypeDef) repoTypeSettings {
	archiveSubdir, materializedSubdir, err := validateArchiveSettings(def.ArchiveSubdir, def.MaterializedSubdir)
	if err != nil {
		archiveSubdir = defaultArchiveSubdir
		materializedSubdir = defaultMaterializedSubdir
	}
	return repoTypeSettings{
		AddButton:          def.AddButton,
		AddDirectoryButton: def.AddDirectoryButton,
		DeleteButton:       def.DeleteButton,
		AutoNormalize:      def.AutoNormalize,
		ShowMD5:            def.ShowMD5,
		ShowSize:           def.ShowSize,
		SingleMove:         def.SingleMove,
		ManualEditorMode:   normalizeManualEditorMode(def.ManualEditorMode, def.Key, def.RuleBookName),
		MetadataDisplayMode:   normalizeMetadataDisplayMode(def.MetadataDisplayMode, def.ManualEditorMode, def.Key, def.RuleBookName),
		MetadataDisplayFields: resolveMetadataDisplayFields(def.MetadataDisplayFields, def.MetadataDisplayMode, def.ManualEditorMode, def.Key, def.RuleBookName),
		ArchiveSubdir:      archiveSubdir,
		MaterializedSubdir: materializedSubdir,
		ArchiveExtensions:  canonicalizeArchiveExtensionsCSV(def.ArchiveExtensions),
		ArchiveReadInnerLayout: def.ArchiveReadInnerLayout,
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
		ManualEditorMode:   normalizeManualEditorMode(info.ManualEditorMode, info.RepoTypeKey, binding.Name),
		MetadataDisplayMode:   defaultMetadataDisplayModeForRepo(info.ManualEditorMode, info.RepoTypeKey, binding.Name),
		MetadataDisplayFields: defaultMetadataDisplayFieldsCSV(info.ManualEditorMode, info.RepoTypeKey, binding.Name),
		ArchiveSubdir:      defaultArchiveSubdir,
		MaterializedSubdir: defaultMaterializedSubdir,
		ArchiveExtensions:  defaultArchiveExtensionsCSV,
		ArchiveReadInnerLayout: true,
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
	if override.ManualEditorMode != nil {
		result.ManualEditorMode = normalizeManualEditorMode(*override.ManualEditorMode, "", result.RuleBookName)
	} else {
		result.ManualEditorMode = normalizeManualEditorMode(result.ManualEditorMode, "", result.RuleBookName)
	}
	if override.MetadataDisplayMode != nil {
		result.MetadataDisplayMode = normalizeMetadataDisplayMode(*override.MetadataDisplayMode, result.ManualEditorMode, "", result.RuleBookName)
	} else {
		result.MetadataDisplayMode = normalizeMetadataDisplayMode(result.MetadataDisplayMode, result.ManualEditorMode, "", result.RuleBookName)
	}
	metadataDisplayFields := result.MetadataDisplayFields
	if override.MetadataDisplayFields != nil {
		metadataDisplayFields = *override.MetadataDisplayFields
	}
	result.MetadataDisplayFields = resolveMetadataDisplayFields(metadataDisplayFields, result.MetadataDisplayMode, result.ManualEditorMode, "", result.RuleBookName)
	archiveSubdir := result.ArchiveSubdir
	if override.ArchiveSubdir != nil {
		archiveSubdir = *override.ArchiveSubdir
	}
	materializedSubdir := result.MaterializedSubdir
	if override.MaterializedSubdir != nil {
		materializedSubdir = *override.MaterializedSubdir
	}
	if normalizedArchive, normalizedMaterialized, err := validateArchiveSettings(archiveSubdir, materializedSubdir); err == nil {
		result.ArchiveSubdir = normalizedArchive
		result.MaterializedSubdir = normalizedMaterialized
	}
	archiveExtensions := result.ArchiveExtensions
	if override.ArchiveExtensions != nil {
		archiveExtensions = *override.ArchiveExtensions
	}
	result.ArchiveExtensions = canonicalizeArchiveExtensionsCSV(archiveExtensions)
	if override.ArchiveReadInnerLayout != nil {
		result.ArchiveReadInnerLayout = *override.ArchiveReadInnerLayout
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

func normalizePublicRepoTypeKeyForCreate(repoType string) (string, error) {
	key := strings.TrimSpace(strings.ToLower(repoType))
	if key == "" {
		return manualMangaRepoTypeKey, nil
	}

	normalizedKey, err := normalizeRepoTypeKey(key)
	if err != nil {
		return "", err
	}
	if isRepoTypeHiddenFromPublicViews(normalizedKey) {
		return "", fmt.Errorf("repo type %q is not available for new repositories", normalizedKey)
	}
	return normalizedKey, nil
}

func resolveVisibleRepoTypeForCreate(repoType string) (string, models.RepoTypeDef, error) {
	key, err := normalizePublicRepoTypeKeyForCreate(repoType)
	if err != nil {
		return "", models.RepoTypeDef{}, err
	}

	resolvedKey, def, err := resolveRepoTypeForCreate(key)
	if err != nil {
		return "", models.RepoTypeDef{}, err
	}
	return resolvedKey, def, nil
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
	if strings.TrimSpace(info.ManualEditorMode) != effective.ManualEditorMode || normalizeManualEditorMode(info.ManualEditorMode, repoTypeKey, effective.RuleBookName) != effective.ManualEditorMode {
		info.ManualEditorMode = effective.ManualEditorMode
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
	includeHidden := strings.EqualFold(strings.TrimSpace(c.Query("include_hidden")), "true")
	query := db.Model(&models.RepoTypeDef{}).Order("sort_order asc").Order("id asc")
	if !includeDisabled {
		query = query.Where("enabled = ?", true)
	}

	var items []models.RepoTypeDef
	if err := query.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query repo types failed: " + err.Error()})
		return
	}
	if !includeHidden {
		items = filterPublicRepoTypes(items)
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
	manualEditorMode, err := validateManualEditorMode(req.ManualEditorMode, key, rulebookName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	metadataDisplayMode, err := validateMetadataDisplayMode(req.MetadataDisplayMode, manualEditorMode, key, rulebookName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	metadataDisplayFields := resolveMetadataDisplayFields(req.MetadataDisplayFields, metadataDisplayMode, manualEditorMode, key, rulebookName)
	archiveSubdir, materializedSubdir, err := validateArchiveSettings(req.ArchiveSubdir, req.MaterializedSubdir)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	archiveExtensions := canonicalizeArchiveExtensionsCSV(req.ArchiveExtensions)
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
		ManualEditorMode:   manualEditorMode,
		MetadataDisplayMode:   metadataDisplayMode,
		MetadataDisplayFields: metadataDisplayFields,
		ArchiveSubdir:      archiveSubdir,
		MaterializedSubdir: materializedSubdir,
		ArchiveExtensions:  archiveExtensions,
		ArchiveReadInnerLayout: req.ArchiveReadInnerLayout == nil || *req.ArchiveReadInnerLayout,
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
	if req.ManualEditorMode != "" {
		manualEditorMode, err := validateManualEditorMode(req.ManualEditorMode, item.Key, item.RuleBookName)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		item.ManualEditorMode = manualEditorMode
	}
	if req.MetadataDisplayMode != "" {
		metadataDisplayMode, err := validateMetadataDisplayMode(req.MetadataDisplayMode, item.ManualEditorMode, item.Key, item.RuleBookName)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		item.MetadataDisplayMode = metadataDisplayMode
	}
	if req.MetadataDisplayFields != "" {
		item.MetadataDisplayFields = canonicalizeMetadataDisplayFieldsCSV(req.MetadataDisplayFields)
	}
	archiveSubdir := item.ArchiveSubdir
	if req.ArchiveSubdir != "" {
		archiveSubdir = req.ArchiveSubdir
	}
	materializedSubdir := item.MaterializedSubdir
	if req.MaterializedSubdir != "" {
		materializedSubdir = req.MaterializedSubdir
	}
	archiveSubdir, materializedSubdir, err = validateArchiveSettings(archiveSubdir, materializedSubdir)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item.ArchiveSubdir = archiveSubdir
	item.MaterializedSubdir = materializedSubdir
	if req.ArchiveExtensions != "" {
		item.ArchiveExtensions = canonicalizeArchiveExtensionsCSV(req.ArchiveExtensions)
	} else if strings.TrimSpace(item.ArchiveExtensions) == "" {
		item.ArchiveExtensions = defaultArchiveExtensionsCSV
	}
	if req.ArchiveReadInnerLayout != nil {
		item.ArchiveReadInnerLayout = *req.ArchiveReadInnerLayout
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
		item.ManualEditorMode = normalizeManualEditorMode(item.ManualEditorMode, item.Key, rulebookName)
	}

	item.ManualEditorMode = normalizeManualEditorMode(item.ManualEditorMode, item.Key, item.RuleBookName)
	item.MetadataDisplayMode = normalizeMetadataDisplayMode(item.MetadataDisplayMode, item.ManualEditorMode, item.Key, item.RuleBookName)
	item.MetadataDisplayFields = resolveMetadataDisplayFields(item.MetadataDisplayFields, item.MetadataDisplayMode, item.ManualEditorMode, item.Key, item.RuleBookName)
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

	if key == repoTypeNone || key == defaultRepoTypeKey || key == karitaRepoTypeKey || key == repoTypeOS {
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
			"message":       "repo type is still in use and has been disabled instead of deleted",
			"repo_type_key": key,
			"in_use_count":  count,
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
	repo, repoDB, info, ok := readRepoInfoByID(c)
	if !ok {
		return
	}

	repoTypeKey, def, override, effective, note, err := resolveEffectiveRepoTypeSettings(info, repo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "resolve repo type settings failed: " + err.Error()})
		return
	}

	directoryAddCautionMessage := ""
	if effective.AddDirectoryButton && !repo.Basic && strings.TrimSpace(repo.RootPath) != "" {
		directoryAddCautionMessage = "本仓库已设定根目录，直接添加目录可能导致数据迁移出错，请谨慎使用。"
	}

	resp := gin.H{
		"repo_id":                       repo.ID,
		"repo_name":                     repo.Name,
		"repo_basic":                    repo.Basic,
		"repo_root_path":                repo.RootPath,
		"repo_root_configured":          strings.TrimSpace(repo.RootPath) != "",
		"repo_type_key":                 repoTypeKey,
		"settings_override":             override,
		"effective":                     effective,
		"resolution_note":               note,
		"directory_add_caution":         directoryAddCautionMessage != "",
		"directory_add_caution_message": directoryAddCautionMessage,
	}
	scanSpec := normalization.GetRuleBookScanSpecForRepo(repo.ID, repoDB)
	resp["scan_spec"] = scanSpec
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
	if err := validateArchiveOverride(repoTypeDefToSettings(def), override); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

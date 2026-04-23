package handlers

import (
	"errors"
	"fmt"
	pathpkg "path"
	"sort"
	"strings"
	"time"

	"lazymanga/models"
	"lazymanga/normalization"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type repoISORefreshProposalContext struct {
	RepoID        uint
	EditorMode    string
	ItemKind      string
	ArchiveSubdir string
	Model         *normalization.RepoPathAnalysisModel
}

type repoISORefreshProposalQueueItem struct {
	ISOID            uint                            `json:"iso_id"`
	Path             string                          `json:"path"`
	FileName         string                          `json:"file_name,omitempty"`
	IsDirectory      bool                            `json:"is_directory"`
	ItemKind         string                          `json:"item_kind,omitempty"`
	MetadataProposal *repoISORefreshMetadataProposal `json:"metadata_proposal,omitempty"`
}

type repoISORefreshMetadataFieldChange struct {
	From any `json:"from,omitempty"`
	To   any `json:"to,omitempty"`
}

type repoISORefreshMetadataProposal struct {
	Available            bool                                         `json:"available"`
	RequiresConfirmation bool                                         `json:"requires_confirmation"`
	Source               string                                       `json:"source,omitempty"`
	EditorMode           string                                       `json:"editor_mode,omitempty"`
	AnalysisPath         string                                       `json:"analysis_path,omitempty"`
	Metadata             map[string]any                               `json:"metadata,omitempty"`
	ChangedFields        []string                                     `json:"changed_fields,omitempty"`
	Changes              map[string]repoISORefreshMetadataFieldChange `json:"changes,omitempty"`
}

type repoISORefreshMetadataAnalysis struct {
	Attempted         bool     `json:"attempted"`
	Status            string   `json:"status,omitempty"`
	Reason            string   `json:"reason,omitempty"`
	AnalyzedAt        string   `json:"analyzed_at,omitempty"`
	EditorMode        string   `json:"editor_mode,omitempty"`
	ItemKind          string   `json:"item_kind,omitempty"`
	AnalysisPath      string   `json:"analysis_path,omitempty"`
	ProposalAvailable bool     `json:"proposal_available"`
	ChangedFieldCount int      `json:"changed_field_count,omitempty"`
	DetectedFields    []string `json:"detected_fields,omitempty"`
	BlockedFields     []string `json:"blocked_fields,omitempty"`
}

func buildRepoISORefreshMetadataProposal(repo models.Repository, repoDB *gorm.DB, info models.RepoInfo, row models.RepoISO) (*repoISORefreshMetadataProposal, error) {
	_, proposal, err := buildRepoISORefreshMetadataAnalysis(repo, repoDB, info, row)
	if err != nil {
		return nil, err
	}
	return proposal, nil
}

func buildRepoISORefreshMetadataAnalysis(repo models.Repository, repoDB *gorm.DB, info models.RepoInfo, row models.RepoISO) (*repoISORefreshMetadataAnalysis, *repoISORefreshMetadataProposal, error) {
	editorMode := resolveRepoISOManualEditorMode(info, row)
	analyzedAt := time.Now().UTC().Format(time.RFC3339)
	ctx, err := prepareRepoISORefreshProposalContext(repo, repoDB, info, row)
	if err != nil {
		return nil, nil, err
	}
	if ctx == nil {
		return &repoISORefreshMetadataAnalysis{
			Attempted:         false,
			Status:            "skipped",
			Reason:            "editor-mode-not-metadata",
			AnalyzedAt:        analyzedAt,
			EditorMode:        editorMode,
			ProposalAvailable: false,
		}, nil, nil
	}
	return buildRepoISORefreshMetadataAnalysisWithContext(ctx, row, analyzedAt)
}

func prepareRepoISORefreshProposalContext(repo models.Repository, repoDB *gorm.DB, info models.RepoInfo, row models.RepoISO) (*repoISORefreshProposalContext, error) {
	editorMode := resolveRepoISOManualEditorMode(info, row)
	if editorMode != manualEditorModeMetadata {
		return nil, nil
	}

	_, _, _, effectiveSettings, _, err := resolveEffectiveRepoTypeSettings(info, repo)
	if err != nil {
		return nil, err
	}
	itemKind := detectRepoISOItemKind(row, effectiveSettings)
	archiveSubdir := ""
	if itemKind == repoISOItemKindArchive {
		normalizedArchiveSubdir, normalizeErr := normalizeArchiveSubdir(effectiveSettings.ArchiveSubdir)
		if normalizeErr == nil {
			archiveSubdir = normalizedArchiveSubdir
		}
	}
	model, err := normalization.BuildRepoPathAnalysisModel(repo.ID, repoDB)
	if err != nil {
		return nil, err
	}
	return &repoISORefreshProposalContext{
		RepoID:        repo.ID,
		EditorMode:    editorMode,
		ItemKind:      itemKind,
		ArchiveSubdir: archiveSubdir,
		Model:         model,
	}, nil
}

func buildRepoISORefreshMetadataProposalWithContext(ctx *repoISORefreshProposalContext, row models.RepoISO) (*repoISORefreshMetadataProposal, error) {
	_, proposal, err := buildRepoISORefreshMetadataAnalysisWithContext(ctx, row, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	return proposal, nil
}

func buildRepoISORefreshMetadataAnalysisWithContext(ctx *repoISORefreshProposalContext, row models.RepoISO, analyzedAt string) (*repoISORefreshMetadataAnalysis, *repoISORefreshMetadataProposal, error) {
	if ctx == nil {
		return nil, nil, nil
	}
	analysisPath := buildRepoISORefreshAnalysisPath(row, ctx.ItemKind, ctx.ArchiveSubdir)
	guess := normalization.AnalyzePathMetadata(ctx.Model, analysisPath)
	existingMetadata := parseRepoISOMetadataJSONMap(row.MetadataJSON)
	detectedFields := nonEmptyRepoISORefreshGuessFields(guess.Metadata)
	blockedFields := repoISORefreshBlockedExistingFields(existingMetadata, guess.Metadata)
	if !row.IsDirectory {
		if sourcePath := sanitizeStoredSourceRelativePath(row.Path); sourcePath != "" {
			guess.Metadata["source_path"] = sourcePath
		}
		if originalName := sanitizeStoredSourcePathSegment(row.FileName); originalName != "" {
			guess.Metadata["original_name"] = originalName
		}
	}

	proposedMetadata, changedFields, changes, err := buildRepoISORefreshProposedMetadata(row, ctx.ItemKind, guess.Metadata)
	if err != nil {
		return nil, nil, err
	}
	analysis := &repoISORefreshMetadataAnalysis{
		Attempted:         true,
		Status:            "no-proposal",
		Reason:            "no-inferred-metadata",
		AnalyzedAt:        analyzedAt,
		EditorMode:        ctx.EditorMode,
		ItemKind:          ctx.ItemKind,
		AnalysisPath:      analysisPath,
		ProposalAvailable: false,
		ChangedFieldCount: len(changedFields),
		DetectedFields:    detectedFields,
		BlockedFields:     blockedFields,
	}
	if len(blockedFields) > 0 {
		analysis.Reason = "blocked-by-existing-metadata"
	} else if len(detectedFields) > 0 {
		analysis.Reason = "detected-fields-produced-no-new-changes"
	}
	if len(changedFields) == 0 {
		return analysis, nil, nil
	}

	proposal := &repoISORefreshMetadataProposal{
		Available:            true,
		RequiresConfirmation: true,
		Source:               "path-analysis",
		EditorMode:           ctx.EditorMode,
		AnalysisPath:         analysisPath,
		Metadata:             proposedMetadata,
		ChangedFields:        changedFields,
		Changes:              changes,
	}
	analysis.Status = "proposed"
	analysis.Reason = "metadata-proposal-generated"
	analysis.ProposalAvailable = true
	return analysis, proposal, nil
}

func ListRepoISORefreshProposals(c *gin.Context) {
	repoID := strings.TrimSpace(c.Param("id"))
	if repoID == "" {
		c.JSON(400, gin.H{"error": "missing id"})
		return
	}

	var repo models.Repository
	if err := db.First(&repo, repoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(404, gin.H{"error": "repo not found"})
			return
		}
		c.JSON(500, gin.H{"error": "db query failed: " + err.Error()})
		return
	}

	repoDB, _, _, err := openRepoScopedDB(repo)
	if err != nil {
		c.JSON(400, gin.H{"error": "prepare repo db failed: " + err.Error()})
		return
	}

	info, err := EnsureRepoInfoFromRepository(repoDB, repo)
	if err != nil {
		c.JSON(500, gin.H{"error": "load repo info failed: " + err.Error()})
		return
	}

	var rows []models.RepoISO
	if err := repoDB.Order("id desc").Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"error": "query repoisos failed: " + err.Error()})
		return
	}

	items := make([]repoISORefreshProposalQueueItem, 0)
	proposalCount := 0
	for _, row := range rows {
		if row.IsMissing {
			continue
		}
		ctx, err := prepareRepoISORefreshProposalContext(repo, repoDB, info, row)
		if err != nil {
			c.JSON(500, gin.H{"error": "prepare refresh proposal failed: " + err.Error()})
			return
		}
		if ctx == nil {
			continue
		}
		proposal, err := buildRepoISORefreshMetadataProposalWithContext(ctx, row)
		if err != nil {
			c.JSON(500, gin.H{"error": "build refresh proposal failed: " + err.Error()})
			return
		}
		if proposal == nil {
			continue
		}
		proposalCount++
		items = append(items, repoISORefreshProposalQueueItem{
			ISOID:            row.ID,
			Path:             row.Path,
			FileName:         row.FileName,
			IsDirectory:      row.IsDirectory,
			ItemKind:         ctx.ItemKind,
			MetadataProposal: proposal,
		})
	}

	c.JSON(200, gin.H{
		"repo_id":         repo.ID,
		"proposal_count":  proposalCount,
		"total_row_count": len(rows),
		"items":           items,
	})
}

func buildRepoISORefreshProposedMetadata(row models.RepoISO, itemKind string, guess map[string]string) (map[string]any, []string, map[string]repoISORefreshMetadataFieldChange, error) {
	existing := parseRepoISOMetadataJSONMap(row.MetadataJSON)
	if existing == nil {
		existing = map[string]any{}
	}
	candidate := cloneRepoISOMetadataMap(existing)

	if itemKind == repoISOItemKindArchive {
		archiveRaw, err := buildImportedFileMetadataJSONFromExisting(row.MetadataJSON, itemKind, row.Path, row.FileName, "")
		if err != nil {
			return nil, nil, nil, fmt.Errorf("build archive proposal metadata failed: %w", err)
		}
		archiveMetadata := parseRepoISOMetadataJSONMap(archiveRaw)
		if archiveMetadata != nil {
			candidate = archiveMetadata
		}
	}

	for key, value := range guess {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		if existingValue := repoISOMetadataStringValue(candidate, trimmedKey); !normalization.CanAutoApplyFieldValue(trimmedKey, existingValue) {
			continue
		}
		candidate[trimmedKey] = trimmedValue
	}

	changedFields := make([]string, 0)
	changes := make(map[string]repoISORefreshMetadataFieldChange)
	for _, key := range sortedRepoISOMetadataKeys(candidate, existing) {
		before := repoISOMetadataStringValue(existing, key)
		after := repoISOMetadataStringValue(candidate, key)
		if before == after {
			continue
		}
		if after == "" {
			continue
		}
		if !normalization.ShouldIncludeFieldInProposalChanges(key) {
			continue
		}
		changedFields = append(changedFields, key)
		changes[key] = repoISORefreshMetadataFieldChange{
			From: repoISOMetadataChangeValue(existing, key),
			To:   candidate[key],
		}
	}
	if len(changedFields) == 0 {
		return nil, nil, nil, nil
	}
	sort.Strings(changedFields)
	return candidate, changedFields, changes, nil
}

func buildRepoISORefreshAnalysisPath(row models.RepoISO, itemKind string, archiveSubdir string) string {
	relativePath := strings.TrimSpace(strings.ReplaceAll(row.Path, "\\", "/"))
	if itemKind == repoISOItemKindArchive {
		relativePath = stripArchiveSubdirPrefix(relativePath, archiveSubdir)
	}
	if row.IsDirectory || relativePath == "" {
		return relativePath
	}
	leaf := pathpkg.Base(relativePath)
	stem := strings.TrimSpace(strings.TrimSuffix(leaf, pathpkg.Ext(leaf)))
	if stem == "" {
		return relativePath
	}
	parent := strings.TrimSpace(pathpkg.Dir(relativePath))
	if parent == "" || parent == "." || parent == "/" {
		return stem
	}
	return strings.Trim(parent+"/"+stem, "/")
}

func stripArchiveSubdirPrefix(relativePath string, archiveSubdir string) string {
	trimmedPath := strings.Trim(strings.TrimSpace(strings.ReplaceAll(relativePath, "\\", "/")), "/")
	trimmedArchive := strings.Trim(strings.TrimSpace(strings.ReplaceAll(archiveSubdir, "\\", "/")), "/")
	if trimmedPath == "" || trimmedArchive == "" {
		return trimmedPath
	}
	if trimmedPath == trimmedArchive {
		return ""
	}
	prefix := trimmedArchive + "/"
	if strings.HasPrefix(trimmedPath, prefix) {
		return strings.TrimPrefix(trimmedPath, prefix)
	}
	return trimmedPath
}

func cloneRepoISOMetadataMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(src))
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}

func sortedRepoISOMetadataKeys(maps ...map[string]any) []string {
	set := make(map[string]struct{})
	for _, items := range maps {
		for key := range items {
			trimmed := strings.TrimSpace(key)
			if trimmed == "" {
				continue
			}
			set[trimmed] = struct{}{}
		}
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func repoISOMetadataChangeValue(metadata map[string]any, key string) any {
	if metadata == nil {
		return nil
	}
	if value, ok := metadata[key]; ok {
		return value
	}
	return nil
}

func repoISOMetadataStringValue(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func nonEmptyRepoISORefreshGuessFields(guess map[string]string) []string {
	if len(guess) == 0 {
		return nil
	}
	fields := make([]string, 0, len(guess))
	for key, value := range guess {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" || !normalization.ShouldCountFieldAsSemanticProposalSignal(trimmedKey) {
			continue
		}
		fields = append(fields, trimmedKey)
	}
	if len(fields) == 0 {
		return nil
	}
	sort.Strings(fields)
	return fields
}

func repoISORefreshBlockedExistingFields(existing map[string]any, guess map[string]string) []string {
	if len(existing) == 0 || len(guess) == 0 {
		return nil
	}
	blocked := make([]string, 0, len(guess))
	for key, value := range guess {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" || !normalization.ShouldCountFieldAsSemanticProposalSignal(trimmedKey) {
			continue
		}
		if repoISOMetadataStringValue(existing, trimmedKey) == "" {
			continue
		}
		blocked = append(blocked, trimmedKey)
	}
	if len(blocked) == 0 {
		return nil
	}
	sort.Strings(blocked)
	return blocked
}

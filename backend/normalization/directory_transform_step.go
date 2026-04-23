package normalization

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"lazymanga/models"
	"lazymanga/normalization/rulebook"
	"lazymanga/normalization/textanalyzer"

	"gorm.io/gorm"
)

var directoryTemplateVarPattern = regexp.MustCompile(`\$\{([a-zA-Z0-9_-]+)\}`)

// DirectoryTransformStep applies optional rulebook-driven directory rename + sidecar metadata logic.
type DirectoryTransformStep struct{}

func NewDirectoryTransformStep() RecordStep {
	return DirectoryTransformStep{}
}

func (s DirectoryTransformStep) Name() string {
	return "directory-transform"
}

func (s DirectoryTransformStep) Process(repoID uint, repoDB *gorm.DB, rootAbs string, record *models.RepoISO) error {
	autoNormalizeEnabled, err := repoAutoNormalizeEnabled(repoDB)
	if err != nil {
		return err
	}
	if !autoNormalizeEnabled || !record.IsDirectory {
		return nil
	}

	currentName := strings.TrimSpace(record.FileName)
	if currentName == "" {
		currentName = filepath.Base(record.Path)
	}
	if currentName == "" {
		return nil
	}

	dirAbs, err := resolveRecordAbsPath(rootAbs, record.Path)
	if err != nil {
		return err
	}

	scanSpec := loadRuleBookForRepo(repoID, repoDB).EffectiveScanSpec()
	matchedRule, _, ok, err := matchDirectoryRuleForPath(dirAbs, scanSpec)
	if err != nil {
		return err
	}
	if !ok || matchedRule.Transform == nil {
		return nil
	}

	analysisModel, modelErr := BuildRepoPathAnalysisModel(repoID, repoDB)
	if modelErr != nil {
		log.Printf("NormalizePipeline: repo path analysis model warning repo_id=%d id=%d path=%q error=%v", repoID, record.ID, record.Path, modelErr)
	}
	existingMetadata := parseDirectoryMetadataJSON(record.MetadataJSON)

	targetName, captures, matched, err := applyDirectoryTransformWithAnalysis(matchedRule.Transform, currentName, record.Path, analysisModel, existingMetadata)
	if err != nil {
		return err
	}
	if !matched {
		return nil
	}

	finalAbs := dirAbs
	finalRelPath := record.Path
	moved := false

	if targetName != "" {
		targetRelPath := strings.TrimSpace(captures["target_path"])
		if targetRelPath == "" {
			parentDir := filepath.ToSlash(filepath.Dir(record.Path))
			if parentDir == "." {
				parentDir = ""
			}
			if parentDir == "" {
				targetRelPath = targetName
			} else {
				targetRelPath = parentDir + "/" + targetName
			}
		}
		targetRelPath = sanitizeRelativeDirectoryPath(targetRelPath)
		if targetRelPath == "" {
			return fmt.Errorf("directory transform rendered empty target path for %q", currentName)
		}
		targetAbs := filepath.Join(rootAbs, filepath.FromSlash(targetRelPath))
		finalAbs, finalRelPath, moved, err = relocateDirectoryWithUniqueTarget(dirAbs, targetAbs, rootAbs)
		if err != nil {
			return err
		}
	}

	payload := buildDirectorySidecarPayload(matchedRule.Name, currentName, filepath.Base(finalRelPath), finalRelPath, matchedRule.Transform, captures)
	if err := writeDirectorySidecar(finalAbs, matchedRule.Transform.MetadataFile, payload); err != nil {
		return err
	}
	metadataJSON, err := buildDirectoryMetadataJSON(payload)
	if err != nil {
		return err
	}

	updates := map[string]interface{}{}
	if finalRelPath != record.Path || filepath.Base(finalRelPath) != record.FileName {
		updates["path"] = finalRelPath
		updates["file_name"] = filepath.Base(finalRelPath)
	}
	if metadataJSON != record.MetadataJSON {
		updates["metadata_json"] = metadataJSON
	}
	if len(updates) > 0 {
		if err := repoDB.Model(&models.RepoISO{}).Where("id = ?", record.ID).Updates(updates).Error; err != nil {
			return err
		}
		record.Path = finalRelPath
		record.FileName = filepath.Base(finalRelPath)
		record.MetadataJSON = metadataJSON
	}

	if moved {
		log.Printf("NormalizePipeline: transformed directory repo_id=%d id=%d rule=%q from=%q to=%q", repoID, record.ID, matchedRule.Name, currentName, filepath.Base(finalRelPath))
	} else {
		log.Printf("NormalizePipeline: refreshed directory metadata repo_id=%d id=%d rule=%q path=%q", repoID, record.ID, matchedRule.Name, finalRelPath)
	}
	return nil
}

func matchDirectoryRuleForPath(dirAbs string, spec rulebook.ScanSpec) (rulebook.DirectoryScanRule, int, bool, error) {
	entries, err := os.ReadDir(dirAbs)
	if err != nil {
		return rulebook.DirectoryScanRule{}, 0, false, err
	}
	fileNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fileNames = append(fileNames, entry.Name())
	}
	matchedRule, count, ok := spec.MatchDirectoryFiles(fileNames)
	return matchedRule, count, ok, nil
}

func applyDirectoryTransform(transform *rulebook.DirectoryTransformSpec, currentName string, relativePath string) (string, map[string]string, bool, error) {
	if transform == nil {
		return "", nil, false, nil
	}

	context := buildBaseDirectoryTemplateContext(currentName, relativePath)
	captures := map[string]string{}
	matched := false

	if recognizerName := strings.TrimSpace(transform.RecognizerName); recognizerName != "" {
		recognizerCaptures, ok, err := evaluateDirectoryNameRecognizer(recognizerName, transform.RecognizerVersion, currentName, relativePath)
		if err != nil {
			return "", nil, false, err
		}
		if ok {
			for key, value := range recognizerCaptures {
				captures[key] = value
				context[key] = value
			}
			matched = true
		}
	}

	if !matched && strings.TrimSpace(transform.Pattern) != "" {
		re, err := regexp.Compile(transform.Pattern)
		if err != nil {
			return "", nil, false, err
		}
		matches := re.FindStringSubmatch(currentName)
		if matches != nil {
			regexContext, namedCaptures := buildDirectoryTemplateContext(re, matches, currentName, relativePath)
			for key, value := range regexContext {
				context[key] = value
			}
			for key, value := range namedCaptures {
				captures[key] = value
			}
			matched = true
		}
	}

	if !matched {
		return "", nil, false, nil
	}

	targetName, renderedCaptures, err := renderDirectoryTransformFromMetadata(transform, currentName, relativePath, captures)
	if err != nil {
		return "", nil, false, err
	}
	return targetName, renderedCaptures, true, nil
}

func applyDirectoryTransformWithAnalysis(transform *rulebook.DirectoryTransformSpec, currentName string, relativePath string, model *RepoPathAnalysisModel, existingMetadata map[string]string) (string, map[string]string, bool, error) {
	targetName, captures, matched, err := applyDirectoryTransform(transform, currentName, relativePath)
	if err != nil || transform == nil {
		return targetName, captures, matched, err
	}

	guessMetadata := analyzeDirectoryPathFallback(relativePath, model)
	merged := make(map[string]string, len(existingMetadata)+len(guessMetadata)+len(captures))
	for key, value := range existingMetadata {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		merged[trimmedKey] = trimmedValue
	}
	for key, value := range guessMetadata {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		if strings.TrimSpace(merged[trimmedKey]) == "" {
			merged[trimmedKey] = trimmedValue
		}
	}
	for key, value := range captures {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		merged[trimmedKey] = trimmedValue
	}

	if !matched && strings.TrimSpace(merged["title"]) == "" {
		return targetName, captures, matched, nil
	}

	renderedName, renderedCaptures, renderErr := renderDirectoryTransformFromMetadata(transform, currentName, relativePath, merged)
	if renderErr != nil {
		if matched {
			return targetName, merged, true, nil
		}
		return targetName, captures, matched, nil
	}
	return renderedName, renderedCaptures, true, nil
}

func analyzeDirectoryPathFallback(relativePath string, model *RepoPathAnalysisModel) map[string]string {
	leaf := strings.TrimSpace(filepath.Base(strings.TrimRight(filepath.ToSlash(relativePath), "/")))
	if leaf == "" {
		return map[string]string{}
	}

	registry := buildTextAnalyzerRegistryFromRepoPathAnalysisModel(model)
	analyzer := textanalyzer.NewAnalyzer()
	result, err := analyzer.Analyze(textanalyzer.AnalyzeTextRequest{
		Input:              leaf,
		AutoRepairBrackets: true,
		PreferLongestMatch: true,
	}, registry)
	if err != nil {
		return map[string]string{}
	}

	metadata := map[string]string{}
	for key, values := range result.Fields {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" || len(values) == 0 {
			continue
		}
		trimmedValue := strings.TrimSpace(values[0])
		if trimmedValue == "" {
			continue
		}
		metadata[trimmedKey] = trimmedValue
	}
	if title := strings.TrimSpace(result.TitleCandidate); title != "" {
		if best := bestBareSegment(title); best != "" {
			metadata["title"] = best
		} else {
			metadata["title"] = cleanInferenceSegmentText(title)
		}
	}
	if sourcePath := filepath.ToSlash(strings.TrimSpace(relativePath)); sourcePath != "" {
		metadata["source_path"] = sourcePath
		metadata["original_name"] = strings.TrimSpace(filepath.Base(strings.TrimRight(sourcePath, "/")))
	}
	applyMetadataQualityGuards(metadata)
	applyMetadataAliases(metadata)
	return metadata
}

func buildTextAnalyzerRegistryFromRepoPathAnalysisModel(model *RepoPathAnalysisModel) textanalyzer.AnalysisHintRegistry {
	if model == nil || len(model.FieldValueCounts) == 0 {
		return textanalyzer.AnalysisHintRegistry{}
	}

	fieldKeys := make([]string, 0, len(model.FieldValueCounts))
	for field := range model.FieldValueCounts {
		if !ShouldIncludeFieldInTextAnalyzerHints(field) {
			continue
		}
		fieldKeys = append(fieldKeys, field)
	}
	sort.Strings(fieldKeys)

	registry := textanalyzer.AnalysisHintRegistry{Fields: make([]textanalyzer.AnalysisFieldHint, 0, len(fieldKeys))}
	for _, field := range fieldKeys {
		valuesByKey := model.FieldValueCounts[field]
		valueKeys := make([]string, 0, len(valuesByKey))
		for valueKey := range valuesByKey {
			valueKeys = append(valueKeys, valueKey)
		}
		sort.SliceStable(valueKeys, func(i, j int) bool {
			leftCount := valuesByKey[valueKeys[i]]
			rightCount := valuesByKey[valueKeys[j]]
			if leftCount != rightCount {
				return leftCount > rightCount
			}
			return valueKeys[i] < valueKeys[j]
		})

		values := make([]textanalyzer.AnalysisValueHint, 0, len(valueKeys))
		for _, valueKey := range valueKeys {
			canonicalValue := strings.TrimSpace(model.CanonicalValues[valueKey])
			if canonicalValue == "" {
				canonicalValue = valueKey
			}
			values = append(values, textanalyzer.AnalysisValueHint{
				CanonicalValue: canonicalValue,
				Aliases:        []string{canonicalValue},
				Weight:         valuesByKey[valueKey],
				Source:         "repo_path_analysis_model",
			})
		}

		registry.Fields = append(registry.Fields, textanalyzer.AnalysisFieldHint{
			Key:        field,
			Label:      field,
			MultiValue: field == "tags",
			Priority:   defaultTextAnalyzerFieldPriority(field),
			Values:     values,
		})
	}
	return registry
}
func defaultTextAnalyzerFieldPriority(field string) int {
	switch strings.TrimSpace(field) {
	case "scanlator_group", "author_name", "author_alias", "circle_name", "circle":
		return 10
	case "comic_market", "event_code", "original_work":
		return 9
	default:
		return 5
	}
}

func renderDirectoryTransformFromMetadata(transform *rulebook.DirectoryTransformSpec, currentName string, relativePath string, metadata map[string]string) (string, map[string]string, error) {
	if transform == nil {
		return "", nil, fmt.Errorf("directory transform is nil")
	}

	context := buildBaseDirectoryTemplateContext(currentName, relativePath)
	captures := map[string]string{}
	for key, value := range metadata {
		normalizedKey := strings.TrimSpace(key)
		normalizedValue := strings.TrimSpace(value)
		if normalizedKey == "" || normalizedValue == "" {
			continue
		}
		captures[normalizedKey] = normalizedValue
		context[normalizedKey] = normalizedValue
	}
	if strings.TrimSpace(context["title"]) == "" {
		context["title"] = strings.TrimSpace(currentName)
	}
	captures["title"] = strings.TrimSpace(context["title"])
	applyMetadataQualityGuards(captures)
	if seriesName, ok := captures["series_name"]; ok {
		context["series_name"] = seriesName
	} else {
		delete(context, "series_name")
	}

	renameTemplate := strings.TrimSpace(transform.RenameTemplate)
	if renameTemplate == "" {
		if title := strings.TrimSpace(context["title"]); title != "" {
			renameTemplate = "${title}"
		} else {
			renameTemplate = currentName
		}
	}

	targetName := sanitizePathSegment(renderDirectoryTemplate(renameTemplate, context))
	if targetName == "" {
		return "", nil, fmt.Errorf("directory transform rendered empty name for %q", currentName)
	}
	captures["title"] = strings.TrimSpace(context["title"])
	captures["normalized_name"] = targetName
	captures["path"] = filepath.ToSlash(strings.TrimSpace(relativePath))

	if targetPathTemplate := strings.TrimSpace(transform.TargetPathTemplate); targetPathTemplate != "" {
		context["normalized_name"] = targetName
		renderedPath := sanitizeRelativeDirectoryPath(renderDirectoryTemplate(targetPathTemplate, context))
		if renderedPath == "" {
			return "", nil, fmt.Errorf("directory transform rendered empty target path for %q", currentName)
		}
		captures["target_path"] = renderedPath
		context["target_path"] = renderedPath
	}

	return targetName, captures, nil
}

// ApplyDirectoryMetadataEdit rewrites a directory record using edited metadata, updates the sidecar JSON, and re-renders the target path from the bound rulebook templates.
func ApplyDirectoryMetadataEdit(repoID uint, repoDB *gorm.DB, rootAbs string, record *models.RepoISO, metadata map[string]any, manualName string) (bool, error) {
	if record == nil {
		return false, fmt.Errorf("record is nil")
	}
	if !record.IsDirectory {
		return false, nil
	}

	currentName := strings.TrimSpace(record.FileName)
	if currentName == "" {
		currentName = filepath.Base(strings.TrimSpace(record.Path))
	}
	if currentName == "" {
		currentName = "unnamed"
	}

	dirAbs, err := resolveRecordAbsPath(rootAbs, record.Path)
	if err != nil {
		return false, err
	}

	mergedMetadata := parseDirectoryMetadataJSON(record.MetadataJSON)
	for key, value := range normalizeDirectoryMetadataMap(metadata) {
		if strings.TrimSpace(value) == "" {
			delete(mergedMetadata, key)
			continue
		}
		mergedMetadata[key] = value
	}
	if strings.TrimSpace(mergedMetadata["title"]) == "" {
		mergedMetadata["title"] = currentName
	}

	scanSpec := loadRuleBookForRepo(repoID, repoDB).EffectiveScanSpec()
	matchedRule, _, ok, err := matchDirectoryRuleForPath(dirAbs, scanSpec)
	if err != nil {
		return false, err
	}
	if !ok || matchedRule.Transform == nil {
		metadataJSON, err := buildDirectoryMetadataJSON(map[string]any{"metadata": mergedMetadata})
		if err != nil {
			return false, err
		}
		record.MetadataJSON = metadataJSON
		return false, nil
	}

	targetName, captures, err := renderDirectoryTransformFromMetadata(matchedRule.Transform, currentName, record.Path, mergedMetadata)
	if err != nil {
		return false, err
	}

	if overrideName := sanitizePathSegment(strings.TrimSpace(manualName)); overrideName != "" {
		targetName = overrideName
		captures["normalized_name"] = overrideName
		parentDir := filepath.ToSlash(filepath.Dir(record.Path))
		if parentDir == "." {
			parentDir = ""
		}
		if parentDir == "" {
			captures["target_path"] = overrideName
		} else {
			captures["target_path"] = sanitizeRelativeDirectoryPath(parentDir + "/" + overrideName)
		}
	}

	targetRelPath := strings.TrimSpace(captures["target_path"])
	if targetRelPath == "" {
		parentDir := filepath.ToSlash(filepath.Dir(record.Path))
		if parentDir == "." {
			parentDir = ""
		}
		if parentDir == "" {
			targetRelPath = targetName
		} else {
			targetRelPath = parentDir + "/" + targetName
		}
	}
	targetRelPath = sanitizeRelativeDirectoryPath(targetRelPath)
	if targetRelPath == "" {
		return false, fmt.Errorf("directory metadata edit rendered empty target path for %q", currentName)
	}

	targetAbs := filepath.Join(rootAbs, filepath.FromSlash(targetRelPath))
	finalAbs, finalRelPath, moved, err := relocateDirectoryWithUniqueTarget(dirAbs, targetAbs, rootAbs)
	if err != nil {
		return false, err
	}

	payload := buildDirectorySidecarPayload(matchedRule.Name, currentName, filepath.Base(finalRelPath), finalRelPath, matchedRule.Transform, captures)
	if err := writeDirectorySidecar(finalAbs, matchedRule.Transform.MetadataFile, payload); err != nil {
		return false, err
	}
	metadataJSON, err := buildDirectoryMetadataJSON(payload)
	if err != nil {
		return false, err
	}

	record.Path = finalRelPath
	record.FileName = filepath.Base(finalRelPath)
	record.MetadataJSON = metadataJSON
	return moved, nil
}

func buildBaseDirectoryTemplateContext(currentName string, relativePath string) map[string]string {
	normalizedPath := filepath.ToSlash(strings.TrimSpace(relativePath))
	context := map[string]string{
		"original_name": currentName,
		"current_name":  currentName,
		"path":          normalizedPath,
		"source_path":   normalizedPath,
	}
	parts := splitRelativePathParts(normalizedPath)
	context["path_depth"] = strconv.Itoa(len(parts))
	for idx, part := range parts {
		context[fmt.Sprintf("path_part_%d", idx+1)] = part
	}
	if len(parts) > 0 {
		context["top_path"] = parts[0]
		context["leaf_path"] = parts[len(parts)-1]
	}
	if len(parts) > 1 {
		context["parent_path"] = parts[len(parts)-2]
	}
	return context
}

func buildDirectoryTemplateContext(re *regexp.Regexp, matches []string, currentName string, relativePath string) (map[string]string, map[string]string) {
	context := buildBaseDirectoryTemplateContext(currentName, relativePath)
	namedCaptures := map[string]string{}
	for idx, value := range matches {
		if idx == 0 {
			continue
		}
		trimmed := strings.TrimSpace(value)
		context[strconv.Itoa(idx)] = trimmed
		if idx < len(re.SubexpNames()) {
			name := strings.TrimSpace(re.SubexpNames()[idx])
			if name != "" {
				context[name] = trimmed
				namedCaptures[name] = trimmed
			}
		}
	}
	return context, namedCaptures
}

func renderDirectoryTemplate(template string, context map[string]string) string {
	rendered := directoryTemplateVarPattern.ReplaceAllStringFunc(template, func(token string) string {
		parts := directoryTemplateVarPattern.FindStringSubmatch(token)
		if len(parts) != 2 {
			return ""
		}
		return context[parts[1]]
	})
	return strings.TrimSpace(rendered)
}

func sanitizePathSegment(name string) string {
	replacer := strings.NewReplacer(
		"/", "／",
		"\\", "＼",
		":", "：",
		"*", "＊",
		"?", "？",
		`"`, "＂",
		"<", "＜",
		">", "＞",
		"|", "｜",
	)
	name = replacer.Replace(strings.TrimSpace(name))
	name = strings.Trim(name, ". ")
	return strings.TrimSpace(name)
}

func sanitizeRelativeDirectoryPath(raw string) string {
	cleaned := strings.TrimSpace(filepath.ToSlash(raw))
	if cleaned == "" {
		return ""
	}
	cleaned = strings.TrimPrefix(path.Clean("/"+cleaned), "/")
	if cleaned == "." {
		return ""
	}
	parts := splitRelativePathParts(cleaned)
	sanitized := make([]string, 0, len(parts))
	for _, part := range parts {
		segment := sanitizePathSegment(part)
		if segment == "" || segment == "." || segment == ".." {
			continue
		}
		sanitized = append(sanitized, segment)
	}
	return strings.Join(sanitized, "/")
}

func splitRelativePathParts(relativePath string) []string {
	normalized := filepath.ToSlash(strings.TrimSpace(relativePath))
	if normalized == "" {
		return nil
	}
	parts := strings.Split(normalized, "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" || trimmed == "." {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func buildDirectorySidecarPayload(ruleName string, originalName string, normalizedName string, relativePath string, transform *rulebook.DirectoryTransformSpec, captures map[string]string) map[string]any {
	normalizedPath := filepath.ToSlash(relativePath)
	sourcePath := normalizedPath
	if v := strings.TrimSpace(captures["source_path"]); v != "" {
		sourcePath = filepath.ToSlash(v)
	} else if v := strings.TrimSpace(captures["path"]); v != "" {
		sourcePath = filepath.ToSlash(v)
	}
	preservedOriginalName := strings.TrimSpace(captures["original_name"])
	if preservedOriginalName == "" {
		preservedOriginalName = originalName
	}

	payload := map[string]any{
		"rule_name":       ruleName,
		"original_name":   preservedOriginalName,
		"normalized_name": normalizedName,
		"relative_path":   normalizedPath,
		"source_path":     sourcePath,
		"path_parts":      splitRelativePathParts(sourcePath),
		"updated_at":      time.Now().UTC().Format(time.RFC3339),
	}
	if sourcePath != normalizedPath {
		payload["normalized_path"] = normalizedPath
	}
	if len(captures) > 0 {
		payload["captured"] = captures
	}

	metadata := map[string]string{}
	context := buildBaseDirectoryTemplateContext(originalName, sourcePath)
	context["normalized_name"] = normalizedName
	context["relative_path"] = normalizedPath
	for key, value := range captures {
		trimmedValue := strings.TrimSpace(value)
		context[key] = trimmedValue
		if shouldPersistDirectoryMetadataKey(key) && trimmedValue != "" {
			metadata[key] = trimmedValue
		}
	}
	if transform != nil && len(transform.Metadata) > 0 {
		for key, template := range transform.Metadata {
			normalizedKey := strings.TrimSpace(key)
			switch normalizedKey {
			case "source_path", "original_name":
				if preserved := strings.TrimSpace(context[normalizedKey]); preserved != "" {
					metadata[normalizedKey] = preserved
					continue
				}
			}
			rendered := strings.TrimSpace(renderDirectoryTemplate(template, context))
			if shouldPersistDirectoryMetadataKey(normalizedKey) && rendered != "" {
				metadata[normalizedKey] = rendered
			}
		}
	}
	if len(metadata) > 0 {
		payload["metadata"] = metadata
	}
	return payload
}

func shouldPersistDirectoryMetadataKey(key string) bool {
	normalizedKey := strings.TrimSpace(key)
	if normalizedKey == "" || strings.HasPrefix(normalizedKey, "_") {
		return false
	}
	switch normalizedKey {
	case "path", "target_path", "normalized_name", "relative_path", "recognizer_name", "recognizer_version", "recognizer_rule_id":
		return false
	}
	if _, err := strconv.Atoi(normalizedKey); err == nil {
		return false
	}
	return true
}

func parseDirectoryMetadataJSON(raw string) map[string]string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "{}" {
		return map[string]string{}
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return map[string]string{}
	}
	result := make(map[string]string, len(decoded))
	for key, value := range decoded {
		normalizedKey := strings.TrimSpace(key)
		normalizedValue := strings.TrimSpace(fmt.Sprint(value))
		if shouldPersistDirectoryMetadataKey(normalizedKey) && normalizedValue != "" && normalizedValue != "<nil>" {
			result[normalizedKey] = normalizedValue
		}
	}
	return result
}

func normalizeDirectoryMetadataMap(metadata map[string]any) map[string]string {
	if len(metadata) == 0 {
		return map[string]string{}
	}
	result := make(map[string]string, len(metadata))
	for key, value := range metadata {
		normalizedKey := strings.TrimSpace(key)
		if !shouldPersistDirectoryMetadataKey(normalizedKey) {
			continue
		}

		normalizedValue := ""
		switch v := value.(type) {
		case nil:
			normalizedValue = ""
		case string:
			normalizedValue = strings.TrimSpace(v)
		default:
			normalizedValue = strings.TrimSpace(fmt.Sprint(v))
			if normalizedValue == "<nil>" {
				normalizedValue = ""
			}
		}
		result[normalizedKey] = normalizedValue
	}
	return result
}

func buildDirectoryMetadataJSON(payload map[string]any) (string, error) {
	if len(payload) == 0 {
		return "", nil
	}

	normalized := map[string]string{}
	switch metadata := payload["metadata"].(type) {
	case map[string]string:
		for key, value := range metadata {
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key == "" || value == "" {
				continue
			}
			normalized[key] = value
		}
	case map[string]any:
		for key, value := range metadata {
			key = strings.TrimSpace(key)
			text := strings.TrimSpace(fmt.Sprint(value))
			if key == "" || text == "" || text == "<nil>" {
				continue
			}
			normalized[key] = text
		}
	}
	if len(normalized) == 0 {
		return "", nil
	}

	raw, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func writeDirectorySidecar(dirAbs string, metadataFile string, payload map[string]any) error {
	fileName := strings.TrimSpace(metadataFile)
	if fileName == "" {
		fileName = ".lazymanga.meta.json"
	}
	fileName = filepath.Base(fileName)
	if fileName == "." || fileName == string(filepath.Separator) {
		return fmt.Errorf("invalid sidecar file name %q", metadataFile)
	}

	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(filepath.Join(dirAbs, fileName), raw, 0o644)
}

func relocateDirectoryWithUniqueTarget(sourceAbs string, targetAbs string, rootAbs string) (string, string, bool, error) {
	if sameFilePath(sourceAbs, targetAbs) {
		rel, err := filepath.Rel(rootAbs, targetAbs)
		if err != nil {
			return "", "", false, err
		}
		return targetAbs, filepath.ToSlash(rel), false, nil
	}

	if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
		return "", "", false, err
	}

	finalTargetAbs, err := findAvailableTargetPath(targetAbs)
	if err != nil {
		return "", "", false, err
	}
	if err := moveDirectoryWithFallback(sourceAbs, finalTargetAbs); err != nil {
		return "", "", false, err
	}
	cleanupEmptyParentDirectories(rootAbs, sourceAbs)

	rel, err := filepath.Rel(rootAbs, finalTargetAbs)
	if err != nil {
		return "", "", true, err
	}
	return finalTargetAbs, filepath.ToSlash(rel), true, nil
}

func moveDirectoryWithFallback(sourceAbs string, targetAbs string) error {
	if err := os.Rename(sourceAbs, targetAbs); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return err
	}

	if err := copyDirectoryRecursive(sourceAbs, targetAbs); err != nil {
		return err
	}
	return os.RemoveAll(sourceAbs)
}

func cleanupEmptyParentDirectories(rootAbs string, sourceAbs string) {
	root := filepath.Clean(strings.TrimSpace(rootAbs))
	current := filepath.Clean(filepath.Dir(strings.TrimSpace(sourceAbs)))
	if root == "" || current == "" || current == "." {
		return
	}

	for current != root && current != "." {
		if !isPathWithinRoot(root, current) {
			return
		}
		err := os.Remove(current)
		if err == nil || errors.Is(err, os.ErrNotExist) {
			next := filepath.Dir(current)
			if next == current {
				return
			}
			current = next
			continue
		}
		return
	}
}

func copyDirectoryRecursive(sourceAbs string, targetAbs string) error {
	return filepath.Walk(sourceAbs, func(current string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(sourceAbs, current)
		if err != nil {
			return err
		}
		destination := filepath.Join(targetAbs, rel)
		if info.IsDir() {
			return os.MkdirAll(destination, info.Mode().Perm())
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		return copyFileWithMode(current, destination)
	})
}

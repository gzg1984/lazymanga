package textanalyzer

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"lazymanga/models"
	"lazymanga/normalization/fieldsemantics"
)

const repoMetadataHintSource = "repo_metadata"

type RepoMetadataHintProvider struct{}

func NewRepoMetadataHintProvider() HintProvider {
	return RepoMetadataHintProvider{}
}

func (RepoMetadataHintProvider) BuildHints(ctx HintBuildContext) (AnalysisHintRegistry, error) {
	rows := ctx.Rows
	if len(rows) == 0 && ctx.RepoDB != nil {
		if err := ctx.RepoDB.Where("metadata_json <> ''").Find(&rows).Error; err != nil {
			return AnalysisHintRegistry{}, err
		}
	}
	return buildRepoMetadataHintRegistry(rows), nil
}

func buildRepoMetadataHintRegistry(rows []models.RepoISO) AnalysisHintRegistry {
	fieldValueCounts := map[string]map[string]int{}
	canonicalValues := map[string]map[string]string{}

	for i := range rows {
		metadata := parseMetadataStringMap(rows[i].MetadataJSON)
		if len(metadata) == 0 {
			continue
		}
		applyMetadataAliases(metadata)
		for field, rawValue := range metadata {
			cleanField := strings.TrimSpace(field)
			cleanValue := cleanInferenceSegmentText(rawValue)
			if !shouldUseMetadataFieldForHints(cleanField, cleanValue) {
				continue
			}
			normalizedValue := normalizeInferenceKey(cleanValue)
			if normalizedValue == "" {
				continue
			}
			if _, ok := fieldValueCounts[cleanField]; !ok {
				fieldValueCounts[cleanField] = map[string]int{}
			}
			fieldValueCounts[cleanField][normalizedValue]++
			if _, ok := canonicalValues[cleanField]; !ok {
				canonicalValues[cleanField] = map[string]string{}
			}
			if canonicalValues[cleanField][normalizedValue] == "" {
				canonicalValues[cleanField][normalizedValue] = cleanValue
			}
		}
	}

	fieldKeys := make([]string, 0, len(fieldValueCounts))
	for field := range fieldValueCounts {
		fieldKeys = append(fieldKeys, field)
	}
	sort.Strings(fieldKeys)

	registry := AnalysisHintRegistry{Fields: make([]AnalysisFieldHint, 0, len(fieldKeys))}
	for _, field := range fieldKeys {
		valuesByKey := fieldValueCounts[field]
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

		values := make([]AnalysisValueHint, 0, len(valueKeys))
		for _, valueKey := range valueKeys {
			canonicalValue := canonicalValues[field][valueKey]
			if canonicalValue == "" {
				canonicalValue = valueKey
			}
			values = append(values, AnalysisValueHint{
				CanonicalValue: canonicalValue,
				Aliases:        []string{canonicalValue},
				Weight:         valuesByKey[valueKey],
				Source:         repoMetadataHintSource,
			})
		}
		registry.Fields = append(registry.Fields, AnalysisFieldHint{
			Key:        field,
			Label:      field,
			MultiValue: fieldAllowsMultipleValues(field),
			Priority:   defaultFieldPriority(field),
			Values:     values,
		})
	}
	return registry
}

func parseMetadataStringMap(raw string) map[string]string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "{}" {
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil
	}
	if len(decoded) == 0 {
		return nil
	}
	result := map[string]string{}
	for key, value := range decoded {
		cleanKey := strings.TrimSpace(key)
		cleanValue := cleanMetadataValue(value)
		if cleanKey == "" || cleanValue == "" {
			continue
		}
		result[cleanKey] = cleanValue
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func cleanMetadataValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []string:
		return strings.TrimSpace(strings.Join(typed, " / "))
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			cleaned := cleanMetadataValue(item)
			if cleaned != "" {
				parts = append(parts, cleaned)
			}
		}
		return strings.TrimSpace(strings.Join(parts, " / "))
	default:
		cleaned := strings.TrimSpace(fmt.Sprint(value))
		if cleaned == "" || cleaned == "<nil>" {
			return ""
		}
		return cleaned
	}
}

func shouldUseMetadataFieldForHints(field string, value string) bool {
	field = strings.TrimSpace(field)
	value = strings.TrimSpace(value)
	if field == "" || value == "" {
		return false
	}
	return fieldsemantics.ShouldIncludeInTextAnalyzerHints(field)
}

func applyMetadataAliases(metadata map[string]string) {
	if metadata == nil {
		return
	}
	if metadata["comic_market"] == "" && metadata["event_code"] != "" {
		metadata["comic_market"] = metadata["event_code"]
	}
	if metadata["circle"] == "" && metadata["circle_name"] != "" {
		metadata["circle"] = metadata["circle_name"]
	}
	if metadata["author_alias"] == "" && metadata["circle_name"] != "" {
		metadata["author_alias"] = metadata["circle_name"]
	}
}

func fieldAllowsMultipleValues(field string) bool {
	switch strings.TrimSpace(field) {
	case "tags":
		return true
	default:
		return false
	}
}

func defaultFieldPriority(field string) int {
	switch strings.TrimSpace(field) {
	case "scanlator_group", "author_name", "author_alias", "circle_name", "circle":
		return 10
	case "comic_market", "event_code", "original_work":
		return 9
	default:
		return 5
	}
}

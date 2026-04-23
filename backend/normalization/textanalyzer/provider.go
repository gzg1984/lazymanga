package textanalyzer

import (
	"strings"

	"lazymanga/models"

	"gorm.io/gorm"
)

type HintBuildContext struct {
	RepoID uint
	RepoDB *gorm.DB
	Rows   []models.RepoISO
}

type HintProvider interface {
	BuildHints(ctx HintBuildContext) (AnalysisHintRegistry, error)
}

type RegistryMerger interface {
	Merge(registries ...AnalysisHintRegistry) AnalysisHintRegistry
}

type DefaultRegistryMerger struct{}

func (DefaultRegistryMerger) Merge(registries ...AnalysisHintRegistry) AnalysisHintRegistry {
	fieldIndex := map[string]int{}
	merged := AnalysisHintRegistry{Fields: make([]AnalysisFieldHint, 0)}
	for _, registry := range registries {
		for _, field := range registry.Fields {
			key := cleanInferenceSegmentText(field.Key)
			if key == "" {
				continue
			}
			index, exists := fieldIndex[key]
			if !exists {
				copied := AnalysisFieldHint{
					Key:           field.Key,
					Label:         field.Label,
					MultiValue:    field.MultiValue,
					Priority:      field.Priority,
					ExclusiveWith: append([]string(nil), field.ExclusiveWith...),
					Values:        copyAnalysisValues(field.Values),
				}
				fieldIndex[key] = len(merged.Fields)
				merged.Fields = append(merged.Fields, copied)
				continue
			}
			target := &merged.Fields[index]
			if target.Label == "" {
				target.Label = field.Label
			}
			if field.MultiValue {
				target.MultiValue = true
			}
			if field.Priority > target.Priority {
				target.Priority = field.Priority
			}
			target.ExclusiveWith = mergeStringValues(target.ExclusiveWith, field.ExclusiveWith...)
			target.Values = mergeAnalysisValues(target.Values, field.Values)
		}
	}
	return merged
}

func copyAnalysisValues(values []AnalysisValueHint) []AnalysisValueHint {
	if len(values) == 0 {
		return nil
	}
	result := make([]AnalysisValueHint, 0, len(values))
	for _, item := range values {
		copied := item
		copied.Aliases = append([]string(nil), item.Aliases...)
		result = append(result, copied)
	}
	return result
}

func mergeAnalysisValues(existing []AnalysisValueHint, incoming []AnalysisValueHint) []AnalysisValueHint {
	if len(incoming) == 0 {
		return existing
	}
	index := map[string]int{}
	for idx, item := range existing {
		key := normalizeInferenceKey(item.CanonicalValue)
		if key == "" {
			continue
		}
		index[key] = idx
	}
	for _, item := range incoming {
		key := normalizeInferenceKey(item.CanonicalValue)
		if key == "" {
			continue
		}
		if idx, ok := index[key]; ok {
			target := &existing[idx]
			if item.Weight > target.Weight {
				target.Weight = item.Weight
			}
			if target.Source == "" {
				target.Source = item.Source
			}
			target.Aliases = mergeStringValues(target.Aliases, item.Aliases...)
			continue
		}
		copied := item
		copied.Aliases = append([]string(nil), item.Aliases...)
		index[key] = len(existing)
		existing = append(existing, copied)
	}
	return existing
}

func mergeStringValues(existing []string, incoming ...string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(existing)+len(incoming))
	appendValue := func(item string) {
		cleaned := strings.TrimSpace(item)
		if cleaned == "" {
			return
		}
		key := strings.ToLower(cleaned)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		result = append(result, cleaned)
	}
	for _, item := range existing {
		appendValue(item)
	}
	for _, item := range incoming {
		appendValue(item)
	}
	return result
}

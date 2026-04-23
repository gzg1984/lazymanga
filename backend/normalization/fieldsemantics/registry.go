package fieldsemantics

import (
	"sort"
	"strings"
)

type Role struct {
	Key                    string
	Technical              bool
	IncludeInAnalysisModel bool
	ContextAnchor          bool
	TitleRelated           bool
	ProposalVisible        bool
	FillEmptyOnly          bool
}

var defaultRoles = map[string]Role{
	"title": {
		Key:                    "title",
		IncludeInAnalysisModel: true,
		ContextAnchor:          true,
		TitleRelated:           true,
		ProposalVisible:        true,
		FillEmptyOnly:          true,
	},
	"series_name": {
		Key:                    "series_name",
		IncludeInAnalysisModel: true,
		ContextAnchor:          true,
		TitleRelated:           true,
		ProposalVisible:        true,
		FillEmptyOnly:          true,
	},
	"source_path": {
		Key:             "source_path",
		Technical:       true,
		ProposalVisible: true,
		FillEmptyOnly:   true,
	},
	"original_name": {
		Key:             "original_name",
		Technical:       true,
		ProposalVisible: true,
		FillEmptyOnly:   true,
	},
	"normalized_name": {
		Key:           "normalized_name",
		Technical:     true,
		FillEmptyOnly: true,
	},
	"path": {
		Key:           "path",
		Technical:     true,
		FillEmptyOnly: true,
	},
	"target_path": {
		Key:           "target_path",
		Technical:     true,
		FillEmptyOnly: true,
	},
	"relative_path": {
		Key:             "relative_path",
		Technical:       true,
		ProposalVisible: true,
		FillEmptyOnly:   true,
	},
	"path_parts": {
		Key:             "path_parts",
		Technical:       true,
		ProposalVisible: true,
		FillEmptyOnly:   true,
	},
	"archive_storage_path": {
		Key:             "archive_storage_path",
		Technical:       true,
		ProposalVisible: true,
		FillEmptyOnly:   true,
	},
	"item_kind": {
		Key:             "item_kind",
		Technical:       true,
		ProposalVisible: true,
		FillEmptyOnly:   true,
	},
}

func Resolve(key string) Role {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return Role{}
	}
	if role, ok := defaultRoles[trimmed]; ok {
		return role
	}
	return Role{
		Key:                    trimmed,
		IncludeInAnalysisModel: true,
		ProposalVisible:        true,
		FillEmptyOnly:          true,
	}
}

func ShouldIncludeInAnalysisModel(key string) bool {
	role := Resolve(key)
	return role.Key != "" && role.IncludeInAnalysisModel && !role.Technical
}

func ShouldCountAsSemanticProposalSignal(key string) bool {
	role := Resolve(key)
	return role.Key != "" && !role.Technical
}

func IsContextAnchor(key string) bool {
	role := Resolve(key)
	return role.Key != "" && role.ContextAnchor && !role.Technical
}

func IsTitleRelated(key string) bool {
	role := Resolve(key)
	return role.Key != "" && role.TitleRelated && !role.Technical
}

func ContextAnchorFields() []string {
	fields := make([]string, 0, len(defaultRoles))
	for key := range defaultRoles {
		if IsContextAnchor(key) {
			fields = append(fields, key)
		}
	}
	sort.Strings(fields)
	return fields
}

func ShouldOnlyFillEmpty(key string) bool {
	role := Resolve(key)
	if role.Key == "" {
		return true
	}
	return role.FillEmptyOnly
}

func CanAutoApplyValue(key string, existingValue string) bool {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return false
	}
	if !ShouldOnlyFillEmpty(trimmedKey) {
		return true
	}
	return strings.TrimSpace(existingValue) == ""
}

func ShouldIncludeInTextAnalyzerHints(key string) bool {
	role := Resolve(key)
	return role.Key != "" && !role.Technical && !role.TitleRelated
}

func ShouldIncludeInProposalChanges(key string) bool {
	role := Resolve(key)
	return role.Key != "" && role.ProposalVisible
}

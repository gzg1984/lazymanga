package rulebook

import (
	"path/filepath"
	"regexp"
	"strings"
)

var defaultScanExtensions = []string{".iso"}

// RuleBook defines a set of ordered rules used by the relocation engine.
type RuleBook struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Scan    ScanSpec `json:"scan,omitempty"`
	Rules   []Rule   `json:"rules"`
}

// ScanSpec defines which file types should be indexed during repo scans.
type ScanSpec struct {
	Extensions             []string            `json:"extensions,omitempty"`
	IncludeFilesWithoutExt bool                `json:"include_files_without_extension,omitempty"`
	DirectoryRules         []DirectoryScanRule `json:"directory_rules,omitempty"`
}

// DirectoryScanRule groups a folder into one logical manga record when enough matching files are found inside it.
type DirectoryScanRule struct {
	Name                   string                  `json:"name,omitempty"`
	Extensions             []string                `json:"extensions,omitempty"`
	IncludeFilesWithoutExt bool                    `json:"include_files_without_extension,omitempty"`
	MinFileCount           int                     `json:"min_file_count,omitempty"`
	Transform              *DirectoryTransformSpec `json:"transform,omitempty"`
}

// DirectoryTransformSpec describes optional directory rename + sidecar metadata behavior.
type DirectoryTransformSpec struct {
	Pattern            string            `json:"pattern,omitempty"`
	RecognizerName     string            `json:"recognizer_name,omitempty"`
	RecognizerVersion  string            `json:"recognizer_version,omitempty"`
	RenameTemplate     string            `json:"rename_template,omitempty"`
	TargetPathTemplate string            `json:"target_path_template,omitempty"`
	MetadataFile       string            `json:"metadata_file,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

// EffectiveScanSpec returns the normalized scan settings with backward-compatible defaults.
func (b RuleBook) EffectiveScanSpec() ScanSpec {
	spec := ScanSpec{
		Extensions:             normalizeScanExtensions(b.Scan.Extensions),
		IncludeFilesWithoutExt: b.Scan.IncludeFilesWithoutExt,
		DirectoryRules:         make([]DirectoryScanRule, 0, len(b.Scan.DirectoryRules)),
	}
	if len(spec.Extensions) == 0 {
		spec.Extensions = append([]string(nil), defaultScanExtensions...)
	}
	for _, rawRule := range b.Scan.DirectoryRules {
		normalizedRule := rawRule
		normalizedRule.Extensions = normalizeScanExtensions(rawRule.Extensions)
		if normalizedRule.MinFileCount <= 0 {
			normalizedRule.MinFileCount = 10
		}
		spec.DirectoryRules = append(spec.DirectoryRules, normalizedRule)
	}
	return spec
}

// ShouldScanFile reports whether the given file name should be indexed for this rule book.
func (b RuleBook) ShouldScanFile(fileName string) bool {
	return b.EffectiveScanSpec().ShouldScanFile(fileName)
}

// ShouldScanFile reports whether the given file name matches the normalized scan spec.
func (s ScanSpec) ShouldScanFile(fileName string) bool {
	normalized := s
	normalized.Extensions = normalizeScanExtensions(s.Extensions)
	if len(normalized.Extensions) == 0 {
		normalized.Extensions = append([]string(nil), defaultScanExtensions...)
	}
	return matchFileByExtensions(fileName, normalized.Extensions, normalized.IncludeFilesWithoutExt)
}

// MatchDirectoryFiles returns the first directory rule that matches the provided child file names.
func (s ScanSpec) MatchDirectoryFiles(fileNames []string) (DirectoryScanRule, int, bool) {
	for _, rule := range s.DirectoryRules {
		normalizedRule := rule
		normalizedRule.Extensions = normalizeScanExtensions(rule.Extensions)
		if normalizedRule.MinFileCount <= 0 {
			normalizedRule.MinFileCount = 10
		}
		if len(normalizedRule.Extensions) == 0 && !normalizedRule.IncludeFilesWithoutExt {
			continue
		}

		count := 0
		for _, name := range fileNames {
			if matchFileByExtensions(name, normalizedRule.Extensions, normalizedRule.IncludeFilesWithoutExt) {
				count++
			}
		}
		if count >= normalizedRule.MinFileCount {
			return normalizedRule, count, true
		}
	}
	return DirectoryScanRule{}, 0, false
}

func matchFileByExtensions(fileName string, extensions []string, includeFilesWithoutExt bool) bool {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return false
	}

	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(fileName)))
	if ext == "" {
		for _, item := range extensions {
			if item == "*" {
				return true
			}
		}
		return includeFilesWithoutExt
	}

	for _, item := range extensions {
		if item == "*" || item == ext {
			return true
		}
	}
	return false
}

func normalizeScanExtensions(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		v := strings.TrimSpace(strings.ToLower(raw))
		if v == "" {
			continue
		}
		if v == "*" || v == "all" || v == "all-files" {
			v = "*"
		} else {
			v = strings.TrimPrefix(v, "*")
			if !strings.HasPrefix(v, ".") {
				v = "." + v
			}
			if v == "." {
				continue
			}
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	return result
}

// Rule defines a single match + action pair.
type Rule struct {
	ID       string    `json:"id"`
	Priority int       `json:"priority"`
	Enabled  bool      `json:"enabled"`
	Match    Condition `json:"match"`
	Action   Action    `json:"action"`
}

// Condition defines rule matching constraints.
type Condition struct {
	IsOS             *bool    `json:"is_os,omitempty"`
	IsEntertainment  *bool    `json:"is_entertainment,omitempty"`
	FileNameContains []string `json:"file_name_contains,omitempty"`
}

// Action defines the relocation outcome when a rule matches.
type Action struct {
	TargetDir string `json:"target_dir"`
	RuleType  string `json:"rule_type"`
	InferIsOS bool   `json:"infer_is_os"`
}

// EvalInput is the minimum state required by rule engine.
type EvalInput struct {
	FileName        string
	IsOS            bool
	IsEntertainment bool
}

// EvalResult contains matched rule and action details.
type EvalResult struct {
	Matched     bool   `json:"matched"`
	RuleID      string `json:"rule_id"`
	RuleBook    string `json:"rule_book"`
	RuleVersion string `json:"rule_version"`
	TargetDir   string `json:"target_dir"`
	RuleType    string `json:"rule_type"`
	Keyword     string `json:"keyword"`
	InferIsOS   bool   `json:"infer_is_os"`
}

// Validate checks basic shape constraints for a rule book.
func (b RuleBook) Validate() error {
	if b.Name == "" {
		return ErrInvalidRuleBook("rule book name required")
	}
	if b.Version == "" {
		return ErrInvalidRuleBook("rule book version required")
	}

	for _, raw := range b.Scan.Extensions {
		v := strings.TrimSpace(raw)
		if v == "" {
			return ErrInvalidRuleBook("scan.extensions cannot contain empty values")
		}
		if strings.ContainsAny(v, `/\\`) {
			return ErrInvalidRuleBook("scan.extensions must be file suffixes, not paths")
		}
	}
	for _, dirRule := range b.Scan.DirectoryRules {
		for _, raw := range dirRule.Extensions {
			v := strings.TrimSpace(raw)
			if v == "" {
				return ErrInvalidRuleBook("scan.directory_rules.extensions cannot contain empty values")
			}
			if strings.ContainsAny(v, `/\\`) {
				return ErrInvalidRuleBook("scan.directory_rules.extensions must be file suffixes, not paths")
			}
		}
		if dirRule.MinFileCount < 0 {
			return ErrInvalidRuleBook("scan.directory_rules.min_file_count cannot be negative")
		}
		if dirRule.Transform != nil {
			if strings.TrimSpace(dirRule.Transform.Pattern) == "" && strings.TrimSpace(dirRule.Transform.RecognizerName) == "" {
				return ErrInvalidRuleBook("scan.directory_rules.transform requires pattern or recognizer_name")
			}
			if strings.TrimSpace(dirRule.Transform.Pattern) != "" {
				if _, err := regexp.Compile(dirRule.Transform.Pattern); err != nil {
					return ErrInvalidRuleBook("scan.directory_rules.transform.pattern invalid: " + err.Error())
				}
			}
			metaFile := strings.TrimSpace(dirRule.Transform.MetadataFile)
			if strings.ContainsAny(metaFile, `/\\`) {
				return ErrInvalidRuleBook("scan.directory_rules.transform.metadata_file must be a file name, not a path")
			}
			for key := range dirRule.Transform.Metadata {
				if strings.TrimSpace(key) == "" {
					return ErrInvalidRuleBook("scan.directory_rules.transform.metadata keys cannot be empty")
				}
			}
		}
	}

	for _, rule := range b.Rules {
		if rule.ID == "" {
			return ErrInvalidRuleBook("rule id required")
		}
		if rule.Action.TargetDir == "" {
			return ErrInvalidRuleBook("rule action target_dir required")
		}
		if rule.Action.RuleType == "" {
			return ErrInvalidRuleBook("rule action rule_type required")
		}
	}

	return nil
}

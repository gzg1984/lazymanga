package normalization

import (
	"encoding/json"
	"errors"
	"fmt"
	"lazymanga/models"
	"lazymanga/normalization/rulebook"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

const (
	defaultRuleBookName    = "noop"
	defaultRuleBookVersion = "v1"
	osRuleBookName         = "default-os-relocation"
	osRuleBookVersion      = "v1"
)

type repoRuleBookFlags struct {
	RuleBookName    string `json:"rulebook_name"`
	RuleBookVersion string `json:"rulebook_version"`
}

// ResolvedRuleBookBinding describes the effective rulebook chosen for a repo.
type ResolvedRuleBookBinding struct {
	Name           string `json:"rulebook_name"`
	Version        string `json:"rulebook_version"`
	Source         string `json:"binding_source"`
	Explicit       bool   `json:"explicit_binding"`
	RawFlagsJSON   string `json:"raw_flags_json,omitempty"`
	ResolutionNote string `json:"resolution_note,omitempty"`
}

var (
	ruleBookEngineCacheMu sync.RWMutex
	ruleBookEngineCache   = map[string]*rulebook.Engine{}
	bookNameSafePattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	bookVersionPattern    = regexp.MustCompile(`^v[0-9]+$`)
)

// InvalidateRuleBookEngineCache clears in-memory engine cache.
func InvalidateRuleBookEngineCache() {
	ruleBookEngineCacheMu.Lock()
	defer ruleBookEngineCacheMu.Unlock()
	ruleBookEngineCache = map[string]*rulebook.Engine{}
}

// AvailableRuleBook describes a rulebook JSON found under normalization/rules.
type AvailableRuleBook struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Path      string `json:"path"`
	Valid     bool   `json:"valid"`
	BookName  string `json:"book_name"`
	RuleCount int    `json:"rule_count"`
	Error     string `json:"error,omitempty"`
}

func buildRuleBookFilePath(name string, version string) (string, error) {
	n := strings.TrimSpace(strings.ToLower(name))
	v := strings.TrimSpace(strings.ToLower(version))
	if !bookNameSafePattern.MatchString(n) {
		return "", fmt.Errorf("invalid rulebook_name")
	}
	if !bookVersionPattern.MatchString(v) {
		return "", fmt.Errorf("invalid rulebook_version")
	}

	relPath := filepath.Join("normalization", "rules", fmt.Sprintf("%s.%s.json", n, v))
	candidates := buildRuleBookPathCandidates(relPath)
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return filepath.ToSlash(p), nil
		}
	}

	// Return executable-dir candidate first for clear deployment diagnostics.
	if len(candidates) > 0 {
		return filepath.ToSlash(candidates[0]), nil
	}

	return filepath.ToSlash(relPath), nil
}

func buildRuleBookPathCandidates(relPath string) []string {
	cleanRel := filepath.Clean(relPath)
	candidates := make([]string, 0, 3)

	if envDir := strings.TrimSpace(os.Getenv("LAZYMANGA_RULEBOOK_DIR")); envDir != "" {
		candidates = append(candidates, filepath.Join(envDir, filepath.Base(cleanRel)))
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates, filepath.Join(exeDir, cleanRel))
	}

	candidates = append(candidates, cleanRel)
	return candidates
}

func getRuleBookSearchGlob() string {
	const relDir = "normalization/rules"

	if envDir := strings.TrimSpace(os.Getenv("LAZYMANGA_RULEBOOK_DIR")); envDir != "" {
		return filepath.Join(envDir, "*.json")
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidate := filepath.Join(exeDir, relDir)
		if fi, statErr := os.Stat(candidate); statErr == nil && fi.IsDir() {
			return filepath.Join(candidate, "*.json")
		}
	}

	return filepath.Join(relDir, "*.json")
}

// ValidateRuleBookSpec validates that a named/versioned rulebook file exists and can be loaded.
func ValidateRuleBookSpec(name string, version string) (string, rulebook.RuleBook, error) {
	path, err := buildRuleBookFilePath(name, version)
	if err != nil {
		return "", rulebook.RuleBook{}, err
	}

	book, err := rulebook.LoadRuleBookFromFile(path)
	if err != nil {
		return path, rulebook.RuleBook{}, err
	}

	return path, book, nil
}

// ListAvailableRuleBooks scans normalization/rules and returns parse results.
func ListAvailableRuleBooks() []AvailableRuleBook {
	searchGlob := getRuleBookSearchGlob()
	paths, err := filepath.Glob(searchGlob)
	if err != nil {
		return []AvailableRuleBook{{
			Path:  filepath.ToSlash(searchGlob),
			Valid: false,
			Error: err.Error(),
		}}
	}

	results := make([]AvailableRuleBook, 0, len(paths))
	for _, p := range paths {
		normalizedPath := filepath.ToSlash(p)
		base := filepath.Base(p)
		trimmed := strings.TrimSuffix(base, ".json")
		idx := strings.LastIndex(trimmed, ".")
		if idx <= 0 || idx >= len(trimmed)-1 {
			results = append(results, AvailableRuleBook{Path: normalizedPath, Valid: false, Error: "invalid filename, expected <name>.<version>.json"})
			continue
		}

		name := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
		version := strings.ToLower(strings.TrimSpace(trimmed[idx+1:]))
		_, book, loadErr := ValidateRuleBookSpec(name, version)
		if loadErr != nil {
			results = append(results, AvailableRuleBook{Path: normalizedPath, Name: name, Version: version, Valid: false, Error: loadErr.Error()})
			continue
		}

		results = append(results, AvailableRuleBook{
			Name:      name,
			Version:   version,
			Path:      normalizedPath,
			Valid:     true,
			BookName:  book.Name,
			RuleCount: len(book.Rules),
		})
	}

	return results
}

func getRuleEngineForRepo(repoID uint, repoDB *gorm.DB) *rulebook.Engine {
	name, version := resolveRepoRuleBookBinding(repoID, repoDB)
	cacheKey := fmt.Sprintf("%s@%s", name, version)

	ruleBookEngineCacheMu.RLock()
	if engine, ok := ruleBookEngineCache[cacheKey]; ok {
		ruleBookEngineCacheMu.RUnlock()
		return engine
	}
	ruleBookEngineCacheMu.RUnlock()

	engine := loadRuleEngineByBookSpec(repoID, name, version)

	ruleBookEngineCacheMu.Lock()
	ruleBookEngineCache[cacheKey] = engine
	ruleBookEngineCacheMu.Unlock()

	return engine
}

// ResolveEffectiveRuleBookBinding derives the effective binding from repo_info.
// Legacy repos that predate explicit rulebook bindings are inferred from AutoNormalize.
func ResolveEffectiveRuleBookBinding(info models.RepoInfo) ResolvedRuleBookBinding {
	resolved := ResolvedRuleBookBinding{
		Name:         defaultRuleBookName,
		Version:      defaultRuleBookVersion,
		Source:       "default-noop",
		Explicit:     false,
		RawFlagsJSON: info.FlagsJSON,
	}

	flags := repoRuleBookFlags{}
	if strings.TrimSpace(info.FlagsJSON) != "" {
		if err := json.Unmarshal([]byte(info.FlagsJSON), &flags); err != nil {
			resolved.Source = "invalid-flags-fallback"
			resolved.ResolutionNote = err.Error()
			return resolved
		}
	}

	name := strings.TrimSpace(strings.ToLower(flags.RuleBookName))
	version := strings.TrimSpace(strings.ToLower(flags.RuleBookVersion))
	if name == "" && version == "" {
		if info.AutoNormalize {
			resolved.Name = osRuleBookName
			resolved.Version = osRuleBookVersion
			resolved.Source = "legacy-auto-normalize"
			resolved.ResolutionNote = "missing explicit binding, inferred from auto_normalize=true"
			return resolved
		}
		return resolved
	}

	if name == "" {
		resolved.Source = "invalid-binding-fallback"
		resolved.ResolutionNote = "missing rulebook_name"
		return resolved
	}
	if version == "" {
		version = defaultRuleBookVersion
	}

	if !bookNameSafePattern.MatchString(name) || !bookVersionPattern.MatchString(version) {
		resolved.Source = "invalid-binding-fallback"
		resolved.ResolutionNote = fmt.Sprintf("invalid binding name=%q version=%q", name, version)
		return resolved
	}

	resolved.Name = name
	resolved.Version = version
	resolved.Source = "explicit"
	resolved.Explicit = true
	return resolved
}

func resolveRepoRuleBookBinding(repoID uint, repoDB *gorm.DB) (string, string) {
	var info models.RepoInfo
	err := repoDB.First(&info, 1).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("RuleBook: query repo_info failed repo_id=%d error=%v fallback=%s@%s", repoID, err, defaultRuleBookName, defaultRuleBookVersion)
		}
		return defaultRuleBookName, defaultRuleBookVersion
	}

	resolved := ResolveEffectiveRuleBookBinding(info)
	if resolved.Source != "explicit" {
		log.Printf("RuleBook: derived binding repo_id=%d source=%s name=%s version=%s note=%q", repoID, resolved.Source, resolved.Name, resolved.Version, resolved.ResolutionNote)
	}
	return resolved.Name, resolved.Version
}

func loadRuleEngineByBookSpec(repoID uint, name string, version string) *rulebook.Engine {
	path, book, err := ValidateRuleBookSpec(name, version)
	if err != nil {
		embedded := rulebook.DefaultNoopRuleBook()
		setDefaultRuleBookLoadStatus(RuleBookLoadStatus{
			Source:        "embedded",
			FilePath:      path,
			UsingFallback: true,
			LastError:     err.Error(),
			BookName:      embedded.Name,
			BookVersion:   embedded.Version,
			RuleCount:     len(embedded.Rules),
			UpdatedAt:     time.Now(),
		})
		log.Printf("RuleBook: load file failed repo_id=%d path=%q error=%v fallback=embedded", repoID, path, err)
		return rulebook.MustNewEngine(embedded)
	}

	engine, err := rulebook.NewEngine(book)
	if err != nil {
		embedded := rulebook.DefaultNoopRuleBook()
		setDefaultRuleBookLoadStatus(RuleBookLoadStatus{
			Source:        "embedded",
			FilePath:      path,
			UsingFallback: true,
			LastError:     err.Error(),
			BookName:      embedded.Name,
			BookVersion:   embedded.Version,
			RuleCount:     len(embedded.Rules),
			UpdatedAt:     time.Now(),
		})
		log.Printf("RuleBook: build engine failed repo_id=%d path=%q error=%v fallback=embedded", repoID, path, err)
		return rulebook.MustNewEngine(embedded)
	}

	setDefaultRuleBookLoadStatus(RuleBookLoadStatus{
		Source:        "file",
		FilePath:      path,
		UsingFallback: false,
		LastError:     "",
		BookName:      book.Name,
		BookVersion:   book.Version,
		RuleCount:     len(book.Rules),
		UpdatedAt:     time.Now(),
	})

	log.Printf("RuleBook: loaded file repo_id=%d path=%q name=%s version=%s rules=%d", repoID, path, book.Name, book.Version, len(book.Rules))
	return engine
}

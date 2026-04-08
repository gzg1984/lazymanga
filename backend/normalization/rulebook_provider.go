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
	"sort"
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
	ruleBookUserDir       string
)

// InvalidateRuleBookEngineCache clears in-memory engine cache.
func InvalidateRuleBookEngineCache() {
	ruleBookEngineCacheMu.Lock()
	defer ruleBookEngineCacheMu.Unlock()
	ruleBookEngineCache = map[string]*rulebook.Engine{}
}

// AvailableRuleBook describes a discoverable rulebook JSON from either the packaged rules dir or the writable user dir.
type AvailableRuleBook struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Path      string `json:"path"`
	Source    string `json:"source"`
	Editable  bool   `json:"editable"`
	Valid     bool   `json:"valid"`
	BookName  string `json:"book_name"`
	RuleCount int    `json:"rule_count"`
	Error     string `json:"error,omitempty"`
}

type RuleBookCatalogInfo struct {
	WritableDir string   `json:"writable_dir"`
	BuiltinDir  string   `json:"builtin_dir,omitempty"`
	SearchDirs  []string `json:"search_dirs"`
}

type ruleBookSearchDir struct {
	Dir      string
	Source   string
	Editable bool
}

// SetRuleBookUserDir configures the writable directory used for user-created rulebooks.
func SetRuleBookUserDir(dir string) {
	ruleBookUserDir = strings.TrimSpace(dir)
}

// GetRuleBookCatalogInfo returns the current search roots and writable target directory.
func GetRuleBookCatalogInfo() RuleBookCatalogInfo {
	dirs := getRuleBookSearchDirs()
	searchDirs := make([]string, 0, len(dirs))
	writableDir := ""
	builtinDir := ""
	for _, item := range dirs {
		normalized := filepath.ToSlash(item.Dir)
		searchDirs = append(searchDirs, normalized)
		if writableDir == "" && item.Editable {
			writableDir = normalized
		}
		if builtinDir == "" && item.Source == "builtin" {
			builtinDir = normalized
		}
	}
	return RuleBookCatalogInfo{
		WritableDir: writableDir,
		BuiltinDir:  builtinDir,
		SearchDirs:  searchDirs,
	}
}

func appendRuleBookSearchDir(dirs []ruleBookSearchDir, seen map[string]struct{}, dir string, source string, editable bool) []ruleBookSearchDir {
	clean := strings.TrimSpace(dir)
	if clean == "" {
		return dirs
	}
	clean = filepath.Clean(filepath.FromSlash(clean))
	key := filepath.ToSlash(clean)
	if _, ok := seen[key]; ok {
		return dirs
	}
	seen[key] = struct{}{}
	return append(dirs, ruleBookSearchDir{Dir: clean, Source: source, Editable: editable})
}

func getRuleBookSearchDirs() []ruleBookSearchDir {
	dirs := make([]ruleBookSearchDir, 0, 4)
	seen := map[string]struct{}{}

	if envDir := strings.TrimSpace(os.Getenv("LAZYMANGA_RULEBOOK_DIR")); envDir != "" {
		dirs = appendRuleBookSearchDir(dirs, seen, envDir, "user", true)
	}
	if strings.TrimSpace(ruleBookUserDir) != "" {
		dirs = appendRuleBookSearchDir(dirs, seen, ruleBookUserDir, "user", true)
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		dirs = appendRuleBookSearchDir(dirs, seen, filepath.Join(exeDir, "normalization", "rules"), "builtin", false)
	}
	dirs = appendRuleBookSearchDir(dirs, seen, filepath.Join("normalization", "rules"), "builtin", false)

	return dirs
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
	fileName := filepath.Base(filepath.Clean(relPath))
	dirs := getRuleBookSearchDirs()
	candidates := make([]string, 0, len(dirs))
	for _, item := range dirs {
		candidates = append(candidates, filepath.Join(item.Dir, fileName))
	}
	return candidates
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

// ListAvailableRuleBooks scans both the writable user dir and the packaged built-in dir and returns parse results.
func ListAvailableRuleBooks() []AvailableRuleBook {
	dirs := getRuleBookSearchDirs()
	results := make([]AvailableRuleBook, 0, 16)
	seenSpecs := map[string]struct{}{}

	for _, root := range dirs {
		info, err := os.Stat(root.Dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			results = append(results, AvailableRuleBook{
				Path:     filepath.ToSlash(root.Dir),
				Source:   root.Source,
				Editable: root.Editable,
				Valid:    false,
				Error:    err.Error(),
			})
			continue
		}
		if !info.IsDir() {
			results = append(results, AvailableRuleBook{
				Path:     filepath.ToSlash(root.Dir),
				Source:   root.Source,
				Editable: root.Editable,
				Valid:    false,
				Error:    "search path is not a directory",
			})
			continue
		}

		paths, globErr := filepath.Glob(filepath.Join(root.Dir, "*.json"))
		if globErr != nil {
			results = append(results, AvailableRuleBook{
				Path:     filepath.ToSlash(filepath.Join(root.Dir, "*.json")),
				Source:   root.Source,
				Editable: root.Editable,
				Valid:    false,
				Error:    globErr.Error(),
			})
			continue
		}
		sort.Strings(paths)

		for _, p := range paths {
			normalizedPath := filepath.ToSlash(p)
			base := filepath.Base(p)
			trimmed := strings.TrimSuffix(base, ".json")
			idx := strings.LastIndex(trimmed, ".")
			if idx <= 0 || idx >= len(trimmed)-1 {
				results = append(results, AvailableRuleBook{
					Path:     normalizedPath,
					Source:   root.Source,
					Editable: root.Editable,
					Valid:    false,
					Error:    "invalid filename, expected <name>.<version>.json",
				})
				continue
			}

			name := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
			version := strings.ToLower(strings.TrimSpace(trimmed[idx+1:]))
			specKey := name + "@" + version
			if _, ok := seenSpecs[specKey]; ok {
				continue
			}
			seenSpecs[specKey] = struct{}{}

			book, loadErr := rulebook.LoadRuleBookFromFile(p)
			if loadErr != nil {
				results = append(results, AvailableRuleBook{
					Path:     normalizedPath,
					Name:     name,
					Version:  version,
					Source:   root.Source,
					Editable: root.Editable,
					Valid:    false,
					Error:    loadErr.Error(),
				})
				continue
			}

			results = append(results, AvailableRuleBook{
				Name:      name,
				Version:   version,
				Path:      normalizedPath,
				Source:    root.Source,
				Editable:  root.Editable,
				Valid:     true,
				BookName:  book.Name,
				RuleCount: len(book.Rules),
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		left := results[i]
		right := results[j]
		if left.Valid != right.Valid {
			return left.Valid && !right.Valid
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		if left.Version != right.Version {
			return left.Version < right.Version
		}
		return left.Path < right.Path
	})

	return results
}

// SaveUserRuleBook validates and writes a rulebook JSON into the writable user data directory.
func SaveUserRuleBook(book rulebook.RuleBook, overwrite bool) (AvailableRuleBook, error) {
	name := strings.TrimSpace(strings.ToLower(book.Name))
	version := strings.TrimSpace(strings.ToLower(book.Version))
	if !bookNameSafePattern.MatchString(name) {
		return AvailableRuleBook{}, fmt.Errorf("invalid rulebook_name")
	}
	if !bookVersionPattern.MatchString(version) {
		return AvailableRuleBook{}, fmt.Errorf("invalid rulebook_version")
	}

	book.Name = name
	book.Version = version
	if err := book.Validate(); err != nil {
		return AvailableRuleBook{}, err
	}

	catalogInfo := GetRuleBookCatalogInfo()
	writableDir := strings.TrimSpace(catalogInfo.WritableDir)
	if writableDir == "" {
		return AvailableRuleBook{}, errors.New("rulebook writable dir is not configured")
	}
	writableDir = filepath.Clean(filepath.FromSlash(writableDir))
	if err := os.MkdirAll(writableDir, 0o755); err != nil {
		return AvailableRuleBook{}, fmt.Errorf("prepare rulebook directory failed: %w", err)
	}

	targetPath := filepath.Join(writableDir, fmt.Sprintf("%s.%s.json", name, version))
	for _, existing := range ListAvailableRuleBooks() {
		if existing.Name != name || existing.Version != version {
			continue
		}
		if filepath.Clean(filepath.FromSlash(existing.Path)) != targetPath {
			return AvailableRuleBook{}, fmt.Errorf("rulebook %s@%s already exists at %s, please use a new version", name, version, existing.Path)
		}
	}

	if _, err := os.Stat(targetPath); err == nil && !overwrite {
		return AvailableRuleBook{}, fmt.Errorf("rulebook %s@%s already exists", name, version)
	} else if err != nil && !os.IsNotExist(err) {
		return AvailableRuleBook{}, fmt.Errorf("stat target rulebook failed: %w", err)
	}

	encoded, err := json.MarshalIndent(book, "", "  ")
	if err != nil {
		return AvailableRuleBook{}, fmt.Errorf("encode rulebook failed: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(targetPath, encoded, 0o644); err != nil {
		return AvailableRuleBook{}, fmt.Errorf("write rulebook failed: %w", err)
	}

	InvalidateRuleBookEngineCache()
	return AvailableRuleBook{
		Name:      name,
		Version:   version,
		Path:      filepath.ToSlash(targetPath),
		Source:    "user",
		Editable:  true,
		Valid:     true,
		BookName:  book.Name,
		RuleCount: len(book.Rules),
	}, nil
}

// FindAvailableRuleBook resolves metadata for the requested rulebook spec.
func FindAvailableRuleBook(name string, version string) (AvailableRuleBook, error) {
	n := strings.TrimSpace(strings.ToLower(name))
	v := strings.TrimSpace(strings.ToLower(version))
	if !bookNameSafePattern.MatchString(n) {
		return AvailableRuleBook{}, fmt.Errorf("invalid rulebook_name")
	}
	if !bookVersionPattern.MatchString(v) {
		return AvailableRuleBook{}, fmt.Errorf("invalid rulebook_version")
	}

	for _, item := range ListAvailableRuleBooks() {
		if item.Name != n || item.Version != v {
			continue
		}
		if !item.Valid {
			if strings.TrimSpace(item.Error) != "" {
				return item, errors.New(item.Error)
			}
			return item, fmt.Errorf("rulebook %s@%s is invalid", n, v)
		}
		return item, nil
	}

	return AvailableRuleBook{}, fmt.Errorf("rulebook %s@%s not found", n, v)
}

// GetRuleBookFileContent reads the raw JSON and parsed content for an available rulebook.
func GetRuleBookFileContent(name string, version string) (AvailableRuleBook, []byte, rulebook.RuleBook, error) {
	item, err := FindAvailableRuleBook(name, version)
	if err != nil {
		return AvailableRuleBook{}, nil, rulebook.RuleBook{}, err
	}

	cleanPath := filepath.Clean(filepath.FromSlash(item.Path))
	raw, err := os.ReadFile(cleanPath)
	if err != nil {
		return item, nil, rulebook.RuleBook{}, fmt.Errorf("read rulebook file %q: %w", cleanPath, err)
	}
	book, err := rulebook.LoadRuleBookFromFile(cleanPath)
	if err != nil {
		return item, raw, rulebook.RuleBook{}, err
	}

	return item, raw, book, nil
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

func fallbackRuleBookByName(name string) rulebook.RuleBook {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case osRuleBookName:
		return rulebook.DefaultOSRelocationRuleBook()
	case "manga-files":
		return rulebook.DefaultMangaFilesRuleBook()
	case "karita-manga":
		return rulebook.DefaultKaritaMangaRuleBook()
	default:
		return rulebook.DefaultNoopRuleBook()
	}
}

func loadRuleBookForRepo(repoID uint, repoDB *gorm.DB) rulebook.RuleBook {
	name, version := resolveRepoRuleBookBinding(repoID, repoDB)
	path, book, err := ValidateRuleBookSpec(name, version)
	if err == nil {
		return book
	}

	fallback := fallbackRuleBookByName(name)
	log.Printf("RuleBook: load scan config failed repo_id=%d path=%q error=%v fallback=%s@%s", repoID, path, err, fallback.Name, fallback.Version)
	return fallback
}

// GetRuleBookScanSpecForRepo returns the effective scan settings for the repository's bound rulebook.
func GetRuleBookScanSpecForRepo(repoID uint, repoDB *gorm.DB) rulebook.ScanSpec {
	book := loadRuleBookForRepo(repoID, repoDB)
	return book.EffectiveScanSpec()
}

func loadRuleEngineByBookSpec(repoID uint, name string, version string) *rulebook.Engine {
	path, book, err := ValidateRuleBookSpec(name, version)
	if err != nil {
		embedded := fallbackRuleBookByName(name)
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
		log.Printf("RuleBook: load file failed repo_id=%d path=%q error=%v fallback=%s@%s", repoID, path, err, embedded.Name, embedded.Version)
		return rulebook.MustNewEngine(embedded)
	}

	engine, err := rulebook.NewEngine(book)
	if err != nil {
		embedded := fallbackRuleBookByName(name)
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
		log.Printf("RuleBook: build engine failed repo_id=%d path=%q error=%v fallback=%s@%s", repoID, path, err, embedded.Name, embedded.Version)
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

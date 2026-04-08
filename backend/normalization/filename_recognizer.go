package normalization

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const defaultDirectoryRecognizerVersion = "v1"

type directoryNameRecognizerBook struct {
	Name    string                        `json:"name"`
	Version string                        `json:"version"`
	Rules   []directoryNameRecognizerRule `json:"rules"`
}

type directoryNameRecognizerRule struct {
	ID       string            `json:"id"`
	MatchOn  string            `json:"match_on,omitempty"`
	Pattern  string            `json:"pattern"`
	Defaults map[string]string `json:"defaults,omitempty"`
}

var (
	directoryRecognizerCacheMu sync.RWMutex
	directoryRecognizerCache   = map[string]directoryNameRecognizerBook{}
)

func getDirectoryNameRecognizerSearchDirs() []string {
	dirs := make([]string, 0, 3)
	seen := map[string]struct{}{}
	appendDir := func(dir string) {
		clean := strings.TrimSpace(dir)
		if clean == "" {
			return
		}
		clean = filepath.Clean(filepath.FromSlash(clean))
		key := filepath.ToSlash(clean)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		dirs = append(dirs, clean)
	}

	if envDir := strings.TrimSpace(os.Getenv("LAZYMANGA_FILENAME_RULE_DIR")); envDir != "" {
		appendDir(envDir)
	}
	if exePath, err := os.Executable(); err == nil {
		appendDir(filepath.Join(filepath.Dir(exePath), "normalization", "filename_rules"))
	}
	appendDir(filepath.Join("normalization", "filename_rules"))

	return dirs
}

func loadDirectoryNameRecognizerBySpec(name string, version string) (directoryNameRecognizerBook, error) {
	n := strings.TrimSpace(strings.ToLower(name))
	if !bookNameSafePattern.MatchString(n) {
		return directoryNameRecognizerBook{}, fmt.Errorf("invalid recognizer_name")
	}
	v := strings.TrimSpace(strings.ToLower(version))
	if v == "" {
		v = defaultDirectoryRecognizerVersion
	}
	if !bookVersionPattern.MatchString(v) {
		return directoryNameRecognizerBook{}, fmt.Errorf("invalid recognizer_version")
	}

	cacheKey := n + "@" + v
	directoryRecognizerCacheMu.RLock()
	if cached, ok := directoryRecognizerCache[cacheKey]; ok {
		directoryRecognizerCacheMu.RUnlock()
		return cached, nil
	}
	directoryRecognizerCacheMu.RUnlock()

	fileName := fmt.Sprintf("%s.%s.json", n, v)
	for _, dir := range getDirectoryNameRecognizerSearchDirs() {
		candidate := filepath.Join(dir, fileName)
		if _, err := os.Stat(candidate); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return directoryNameRecognizerBook{}, fmt.Errorf("stat directory recognizer %q: %w", candidate, err)
		}
		book, err := loadDirectoryNameRecognizerFromFile(candidate)
		if err != nil {
			return directoryNameRecognizerBook{}, err
		}
		directoryRecognizerCacheMu.Lock()
		directoryRecognizerCache[cacheKey] = book
		directoryRecognizerCacheMu.Unlock()
		return book, nil
	}

	if fallback, ok := fallbackDirectoryNameRecognizer(n, v); ok {
		directoryRecognizerCacheMu.Lock()
		directoryRecognizerCache[cacheKey] = fallback
		directoryRecognizerCacheMu.Unlock()
		return fallback, nil
	}

	return directoryNameRecognizerBook{}, fmt.Errorf("directory recognizer %s@%s not found", n, v)
}

func loadDirectoryNameRecognizerFromFile(path string) (directoryNameRecognizerBook, error) {
	cleanPath := filepath.Clean(path)
	raw, err := os.ReadFile(cleanPath)
	if err != nil {
		return directoryNameRecognizerBook{}, fmt.Errorf("read directory recognizer file %q: %w", cleanPath, err)
	}

	var book directoryNameRecognizerBook
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&book); err != nil {
		return directoryNameRecognizerBook{}, fmt.Errorf("decode directory recognizer file %q: %w", cleanPath, err)
	}
	if err := book.Validate(); err != nil {
		return directoryNameRecognizerBook{}, fmt.Errorf("validate directory recognizer file %q: %w", cleanPath, err)
	}
	return book, nil
}

func (b directoryNameRecognizerBook) Validate() error {
	if !bookNameSafePattern.MatchString(strings.TrimSpace(strings.ToLower(b.Name))) {
		return fmt.Errorf("recognizer name required")
	}
	if !bookVersionPattern.MatchString(strings.TrimSpace(strings.ToLower(b.Version))) {
		return fmt.Errorf("recognizer version required")
	}
	if len(b.Rules) == 0 {
		return fmt.Errorf("at least one recognizer rule is required")
	}
	for _, rule := range b.Rules {
		if strings.TrimSpace(rule.ID) == "" {
			return fmt.Errorf("recognizer rule id required")
		}
		if strings.TrimSpace(rule.Pattern) == "" {
			return fmt.Errorf("recognizer rule pattern required")
		}
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return fmt.Errorf("recognizer rule %s pattern invalid: %w", rule.ID, err)
		}
		switch strings.TrimSpace(strings.ToLower(rule.MatchOn)) {
		case "", "current_name", "name", "path", "relative_path":
		default:
			return fmt.Errorf("recognizer rule %s has unsupported match_on %q", rule.ID, rule.MatchOn)
		}
	}
	return nil
}

func evaluateDirectoryNameRecognizer(name string, version string, currentName string, relativePath string) (map[string]string, bool, error) {
	book, err := loadDirectoryNameRecognizerBySpec(name, version)
	if err != nil {
		return nil, false, err
	}

	normalizedPath := filepath.ToSlash(strings.TrimSpace(relativePath))
	repairedName := autoBalanceBracketText(currentName)
	repairedPath := autoBalanceBracketText(normalizedPath)

	var bestCaptures map[string]string
	bestScore := -1
	for _, rule := range book.Rules {
		subjects := []string{currentName}
		matchOn := strings.TrimSpace(strings.ToLower(rule.MatchOn))
		switch matchOn {
		case "path", "relative_path":
			subjects = []string{normalizedPath}
			if repairedPath != "" && repairedPath != normalizedPath {
				subjects = append(subjects, repairedPath)
			}
		default:
			if repairedName != "" && repairedName != currentName {
				subjects = append(subjects, repairedName)
			}
		}

		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return nil, false, err
		}
		for _, subject := range subjects {
			matches := re.FindStringSubmatch(subject)
			if matches == nil {
				continue
			}

			context, captures := buildDirectoryTemplateContext(re, matches, currentName, relativePath)
			captures["recognizer_name"] = book.Name
			captures["recognizer_version"] = book.Version
			captures["recognizer_rule_id"] = rule.ID
			context["recognizer_name"] = book.Name
			context["recognizer_version"] = book.Version
			context["recognizer_rule_id"] = rule.ID

			for key, template := range rule.Defaults {
				rendered := renderDirectoryTemplate(template, context)
				captures[key] = rendered
				context[key] = rendered
			}
			applyMetadataQualityGuards(captures)
			if seriesName, ok := captures["series_name"]; ok {
				context["series_name"] = seriesName
			} else {
				delete(context, "series_name")
			}

			score := len(captures)
			if _, ok := captures["series_name"]; ok && strings.TrimSpace(captures["series_name"]) != "" {
				score += 4
			}
			if matchOn == "path" || matchOn == "relative_path" {
				score += 2
			}
			if score > bestScore {
				bestScore = score
				bestCaptures = captures
			}
		}
	}

	if bestScore >= 0 {
		return bestCaptures, true, nil
	}
	return nil, false, nil
}

func fallbackDirectoryNameRecognizer(name string, version string) (directoryNameRecognizerBook, bool) {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "karita-manga-filename":
		if version == "" || version == "v1" {
			return defaultKaritaMangaFilenameRecognizer(), true
		}
	}
	return directoryNameRecognizerBook{}, false
}

func defaultKaritaMangaFilenameRecognizer() directoryNameRecognizerBook {
	return directoryNameRecognizerBook{
		Name:    "karita-manga-filename",
		Version: "v1",
		Rules: []directoryNameRecognizerRule{
			{
				ID:      "scanlator-circle-author-title",
				MatchOn: "current_name",
				Pattern: `^【(?P<scanlator_group>[^】]+)】[（(](?P<event_code>[^）)]+)[）)]\s*\[(?P<circle_name>[^\[(（\]]+?)(?:\s*[（(](?P<author_name>[^）)]+)[）)])?\]\s*(?P<title>.+?)(?:\s*[（(](?P<original_work>[^）)]+)[）)])?$`,
				Defaults: map[string]string{
					"comic_market": `${event_code}`,
					"circle":       `${circle_name}`,
					"author_alias": `${circle_name}`,
				},
			},
			{
				ID:       "scanlator-author-alias-title-noevent",
				MatchOn:  "current_name",
				Pattern:  `^【(?P<scanlator_group>[^】]+)】\s*\[(?P<author_alias>[^\[(（\]]+?)(?:\s*[（(](?P<author_name>[^）)]+)[）)])?\]\s*(?P<title>.+?)$`,
				Defaults: map[string]string{},
			},
			{
				ID:      "parent-series-structured-child-title",
				MatchOn: "relative_path",
				Pattern: `(?:^|.*/)【[^】]+】[（(][^）)]+[）)]\s*\[[^\]]+\]\s*(?P<series_name>[^/]+?)\s*[（(][^）)]+[）)]/【(?P<scanlator_group>[^】]+)】[（(](?P<event_code>[^）)]+)[）)]\s*\[(?P<circle_name>[^\[(（]+?)(?:\s*[（(](?P<author_name>[^）)]+)[）)])?\]\s*(?P<title>.+?)\s*[（(](?P<original_work>[^）)]+)[）)]$`,
				Defaults: map[string]string{
					"comic_market": `${event_code}`,
					"circle":       `${circle_name}`,
					"author_alias": `${circle_name}`,
				},
			},
			{
				ID:      "parent-series-child-title",
				MatchOn: "relative_path",
				Pattern: `(?:^|.*/)【(?P<scanlator_group>[^】]+)】[（(](?P<event_code>[^）)]+)[）)]\s*\[(?P<circle_name>[^\[(（]+?)(?:\s*[（(](?P<author_name>[^）)]+)[）)])?\]\s*(?P<series_name>.+?)\s*[（(](?P<original_work>[^）)]+)[）)]/(?:【[^】]+】)?(?P<title>[^/]+)$`,
				Defaults: map[string]string{
					"comic_market": `${event_code}`,
					"circle":       `${circle_name}`,
					"author_alias": `${circle_name}`,
				},
			},
			{
				ID:      "legacy-circle-title-year-id",
				MatchOn: "current_name",
				Pattern: `^\[(?P<circle>[^\]]+)\]\s*(?P<title>.+?)(?:\s+[（(](?P<year>\d{4})[）)])?(?:\s+\[(?P<karita_id>\d+)\])?$`,
				Defaults: map[string]string{
					"circle_name": `${circle}`,
				},
			},
		},
	}
}

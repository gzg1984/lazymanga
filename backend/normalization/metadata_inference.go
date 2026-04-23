package normalization

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"lazymanga/models"

	"gorm.io/gorm"
)

const repoPathAnalysisModelTTL = 15 * time.Minute

type RepoPathAnalysisModel struct {
	RepoID              uint      `json:"repo_id"`
	SampleCount         int       `json:"sample_count"`
	BuiltAt             time.Time `json:"built_at"`
	ExpiresAt           time.Time `json:"expires_at"`
	FieldValueCounts    map[string]map[string]int
	ValueFieldCounts    map[string]map[string]int
	ContextValueCounts  map[string]map[string]map[string]int
	CanonicalValues     map[string]string
	IgnoredPrefixCounts map[string]int
	IgnoredPrefixRaw    map[string]string
}

type PathMetadataGuess struct {
	Metadata     map[string]string `json:"metadata"`
	RepairedPath string            `json:"repaired_path,omitempty"`
	SampleCount  int               `json:"sample_count,omitempty"`
}

type pathInferenceSegment struct {
	Text      string
	Bracketed bool
}

var (
	repoPathAnalysisModelCacheMu sync.RWMutex
	repoPathAnalysisModelCache   = map[uint]RepoPathAnalysisModel{}
)

// BuildRepoPathAnalysisModel loads repository-local metadata associations into an in-memory model and caches it for a short TTL.
func BuildRepoPathAnalysisModel(repoID uint, repoDB *gorm.DB) (*RepoPathAnalysisModel, error) {
	if repoDB == nil {
		return &RepoPathAnalysisModel{
			RepoID:              repoID,
			FieldValueCounts:    map[string]map[string]int{},
			ValueFieldCounts:    map[string]map[string]int{},
			ContextValueCounts:  map[string]map[string]map[string]int{},
			CanonicalValues:     map[string]string{},
			IgnoredPrefixCounts: map[string]int{},
			IgnoredPrefixRaw:    map[string]string{},
		}, nil
	}

	now := time.Now()
	repoPathAnalysisModelCacheMu.RLock()
	if cached, ok := repoPathAnalysisModelCache[repoID]; ok && !shouldRefreshRepoPathAnalysisModel(cached, now) {
		copyModel := cached
		repoPathAnalysisModelCacheMu.RUnlock()
		return &copyModel, nil
	}
	repoPathAnalysisModelCacheMu.RUnlock()

	model, err := buildRepoPathAnalysisModelFromDB(repoID, repoDB, now)
	if err != nil {
		return nil, err
	}

	repoPathAnalysisModelCacheMu.Lock()
	repoPathAnalysisModelCache[repoID] = *model
	repoPathAnalysisModelCacheMu.Unlock()
	return model, nil
}

func shouldRefreshRepoPathAnalysisModel(model RepoPathAnalysisModel, now time.Time) bool {
	if model.ExpiresAt.IsZero() {
		return true
	}
	return !now.Before(model.ExpiresAt)
}

func buildRepoPathAnalysisModelFromDB(repoID uint, repoDB *gorm.DB, now time.Time) (*RepoPathAnalysisModel, error) {
	var rows []models.RepoISO
	if err := repoDB.Where("metadata_json <> ''").Find(&rows).Error; err != nil {
		return nil, err
	}
	return buildRepoPathAnalysisModelFromRows(repoID, rows, now), nil
}

func buildRepoPathAnalysisModelFromRows(repoID uint, rows []models.RepoISO, now time.Time) *RepoPathAnalysisModel {
	model := &RepoPathAnalysisModel{
		RepoID:              repoID,
		BuiltAt:             now,
		ExpiresAt:           now.Add(repoPathAnalysisModelTTL),
		FieldValueCounts:    map[string]map[string]int{},
		ValueFieldCounts:    map[string]map[string]int{},
		ContextValueCounts:  map[string]map[string]map[string]int{},
		CanonicalValues:     map[string]string{},
		IgnoredPrefixCounts: map[string]int{},
		IgnoredPrefixRaw:    map[string]string{},
	}
	for i := range rows {
		metadata := parseDirectoryMetadataJSON(rows[i].MetadataJSON)
		if len(metadata) == 0 {
			continue
		}
		indexRepoPathAnalysisRow(model, rows[i], metadata)
	}
	return model
}

func indexRepoPathAnalysisRow(model *RepoPathAnalysisModel, row models.RepoISO, metadata map[string]string) {
	if model == nil || len(metadata) == 0 {
		return
	}
	model.SampleCount++

	for field, value := range metadata {
		field = strings.TrimSpace(field)
		cleanValue := cleanInferenceSegmentText(value)
		if !shouldUseMetadataFieldForAnalysis(field, cleanValue) {
			continue
		}
		normalizedValue := normalizeInferenceKey(cleanValue)
		if normalizedValue == "" {
			continue
		}
		if _, ok := model.FieldValueCounts[field]; !ok {
			model.FieldValueCounts[field] = map[string]int{}
		}
		model.FieldValueCounts[field][normalizedValue]++
		if _, ok := model.ValueFieldCounts[normalizedValue]; !ok {
			model.ValueFieldCounts[normalizedValue] = map[string]int{}
		}
		model.ValueFieldCounts[normalizedValue][field]++
		if _, ok := model.CanonicalValues[normalizedValue]; !ok {
			model.CanonicalValues[normalizedValue] = cleanValue
		}
	}

	for field, value := range metadata {
		field = strings.TrimSpace(field)
		cleanValue := cleanInferenceSegmentText(value)
		if !shouldUseMetadataFieldForAnalysis(field, cleanValue) {
			continue
		}
		normalizedValue := normalizeInferenceKey(cleanValue)
		if normalizedValue == "" {
			continue
		}
		for _, anchorField := range ContextAnchorFields() {
			anchorValue := cleanInferenceSegmentText(metadata[anchorField])
			anchorKey := normalizeInferenceKey(anchorValue)
			if anchorKey == "" {
				continue
			}
			addContextValueVote(model, anchorField+":"+anchorKey, field, normalizedValue)
		}
	}

	originalName := cleanInferenceSegmentText(metadata["original_name"])
	if originalName == "" {
		originalName = cleanInferenceSegmentText(row.FileName)
	}
	if originalName == "" {
		sourcePath := strings.TrimSpace(metadata["source_path"])
		if sourcePath != "" {
			originalName = cleanInferenceSegmentText(filepath.Base(filepath.FromSlash(sourcePath)))
		}
	}
	recordIgnoredPrefixesFromExample(model, originalName, metadata)
}

func addContextValueVote(model *RepoPathAnalysisModel, contextKey string, field string, value string) {
	if model == nil || contextKey == "" || field == "" || value == "" {
		return
	}
	if _, ok := model.ContextValueCounts[contextKey]; !ok {
		model.ContextValueCounts[contextKey] = map[string]map[string]int{}
	}
	if _, ok := model.ContextValueCounts[contextKey][field]; !ok {
		model.ContextValueCounts[contextKey][field] = map[string]int{}
	}
	model.ContextValueCounts[contextKey][field][value]++
}

func shouldUseMetadataFieldForAnalysis(field string, value string) bool {
	field = strings.TrimSpace(field)
	value = strings.TrimSpace(value)
	if field == "" || value == "" {
		return false
	}
	return ShouldIncludeFieldInAnalysisModel(field)
}

func recordIgnoredPrefixesFromExample(model *RepoPathAnalysisModel, originalName string, metadata map[string]string) {
	if model == nil {
		return
	}
	originalName = strings.TrimSpace(autoBalanceBracketText(originalName))
	if originalName == "" {
		return
	}
	titleKey := normalizeInferenceKey(metadata["title"])
	if titleKey == "" {
		return
	}

	segments := splitPathInferenceSegments(originalName)
	for _, segment := range segments {
		cleaned := cleanInferenceSegmentText(segment.Text)
		if cleaned == "" {
			continue
		}
		segmentKey := normalizeInferenceKey(cleaned)
		if segmentKey == "" {
			continue
		}
		if segmentKey == titleKey || strings.Contains(segmentKey, titleKey) || strings.Contains(titleKey, segmentKey) {
			break
		}
		if segmentMatchesKnownMetadata(segmentKey, metadata) {
			continue
		}
		model.IgnoredPrefixCounts[segmentKey]++
		if _, ok := model.IgnoredPrefixRaw[segmentKey]; !ok {
			model.IgnoredPrefixRaw[segmentKey] = cleaned
		}
	}
}

func segmentMatchesKnownMetadata(segmentKey string, metadata map[string]string) bool {
	for field, value := range metadata {
		if !shouldUseMetadataFieldForAnalysis(field, value) {
			continue
		}
		valueKey := normalizeInferenceKey(value)
		if valueKey != "" && (valueKey == segmentKey || strings.Contains(valueKey, segmentKey) || strings.Contains(segmentKey, valueKey)) {
			return true
		}
	}
	return false
}

// AnalyzePathMetadata applies a cached repo model to a concrete path and returns a best-effort metadata JSON payload.
func AnalyzePathMetadata(model *RepoPathAnalysisModel, relativePath string) PathMetadataGuess {
	repairedPath := autoBalanceBracketText(relativePath)
	parts := splitRelativePathParts(repairedPath)
	metadata := map[string]string{}
	if sourcePath := filepath.ToSlash(strings.TrimSpace(relativePath)); sourcePath != "" {
		metadata["source_path"] = sourcePath
		metadata["original_name"] = cleanInferenceSegmentText(filepath.Base(strings.TrimRight(sourcePath, "/")))
	}

	for idx, part := range parts {
		inferMetadataFromPathPart(model, part, idx == len(parts)-1, metadata)
	}
	if len(parts) > 0 {
		leaf := parts[len(parts)-1]
		if title := inferTitleFromLeaf(model, leaf, metadata); title != "" {
			metadata["title"] = title
		}
	}
	applyContextMetadataHints(model, metadata)
	applyMetadataQualityGuards(metadata)
	applyMetadataAliases(metadata)
	return PathMetadataGuess{
		Metadata:     metadata,
		RepairedPath: repairedPath,
		SampleCount:  analysisModelSampleCount(model),
	}
}

func analysisModelSampleCount(model *RepoPathAnalysisModel) int {
	if model == nil {
		return 0
	}
	return model.SampleCount
}

func inferMetadataFromPathPart(model *RepoPathAnalysisModel, part string, isLeaf bool, metadata map[string]string) {
	cleanPart := strings.TrimSpace(autoBalanceBracketText(part))
	if cleanPart == "" {
		return
	}
	segments := splitPathInferenceSegments(cleanPart)
	for _, segment := range segments {
		cleaned := cleanInferenceSegmentText(segment.Text)
		if cleaned == "" {
			continue
		}
		matches := matchMetadataFieldsForToken(model, cleaned)
		for field, value := range matches {
			if strings.TrimSpace(value) == "" {
				continue
			}
			if metadata[field] == "" {
				metadata[field] = value
			}
		}
	}

	if !isLeaf && metadata["series_name"] == "" {
		if candidate := bestBareSegment(cleanPart); candidate != "" {
			candidate = trimIgnoredPrefixCandidate(model, candidate)
			if candidate != "" {
				metadata["series_name"] = candidate
			}
		}
	}
}

func inferTitleFromLeaf(model *RepoPathAnalysisModel, leaf string, metadata map[string]string) string {
	leaf = strings.TrimSpace(autoBalanceBracketText(leaf))
	if leaf == "" {
		return ""
	}

	segments := splitPathInferenceSegments(leaf)
	candidates := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment.Bracketed {
			continue
		}
		candidate := trimIgnoredPrefixCandidate(model, cleanInferenceSegmentText(segment.Text))
		if candidate == "" || isLikelyNoiseTitle(candidate) {
			continue
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	if len(candidates) > 1 {
		sort.SliceStable(candidates, func(i, j int) bool {
			return scoreTitleCandidate(model, candidates[i], metadata) > scoreTitleCandidate(model, candidates[j], metadata)
		})
		return candidates[0]
	}

	residual := trimIgnoredPrefixCandidate(model, cleanInferenceSegmentText(stripBracketedSegments(leaf)))
	if residual != "" && !isLikelyNoiseTitle(residual) {
		return residual
	}

	bracketedFallback := inferTitleFromBracketedSegments(model, segments, metadata)
	if bracketedFallback != "" {
		return bracketedFallback
	}
	return ""
}

func inferTitleFromBracketedSegments(model *RepoPathAnalysisModel, segments []pathInferenceSegment, metadata map[string]string) string {
	if len(segments) == 0 {
		return ""
	}
	candidates := make([]string, 0, len(segments))
	for _, segment := range segments {
		if !segment.Bracketed {
			continue
		}
		candidate := trimIgnoredPrefixCandidate(model, cleanInferenceSegmentText(segment.Text))
		if candidate == "" || isLikelyNoiseTitle(candidate) || looksLikeContributorTag(candidate) {
			continue
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		return ""
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return scoreTitleCandidate(model, candidates[i], metadata) > scoreTitleCandidate(model, candidates[j], metadata)
	})
	return candidates[0]
}

func scoreTitleCandidate(model *RepoPathAnalysisModel, candidate string, metadata map[string]string) int {
	cleaned := cleanInferenceSegmentText(candidate)
	if cleaned == "" {
		return 0
	}
	score := len([]rune(cleaned)) * 10
	normalized := normalizeInferenceKey(cleaned)
	if normalized == "" {
		return 0
	}
	if hasMeaningfulRunes(cleaned) {
		score += 15
	}
	if seriesName := cleanInferenceSegmentText(metadata["series_name"]); seriesName != "" && isSeriesNameRelevantToTitle(seriesName, cleaned) {
		score += 35
		if normalized != normalizeInferenceKey(seriesName) {
			score += 15
		}
	}
	if model != nil {
		if _, ok := model.ContextValueCounts["title:"+normalized]; ok {
			score += 25
		}
		if _, ok := model.ContextValueCounts["series_name:"+normalized]; ok {
			score += 10
		}
	}
	for field, value := range metadata {
		if IsTitleRelatedField(field) {
			continue
		}
		valueKey := normalizeInferenceKey(value)
		if valueKey != "" && valueKey == normalized {
			score -= 30
		}
	}
	return score
}

func applyContextMetadataHints(model *RepoPathAnalysisModel, metadata map[string]string) {
	if model == nil || len(model.ContextValueCounts) == 0 || len(metadata) == 0 {
		return
	}
	for _, field := range ContextAnchorFields() {
		valueKey := normalizeInferenceKey(metadata[field])
		if valueKey == "" {
			continue
		}
		contextKey := field + ":" + valueKey
		contextFields := model.ContextValueCounts[contextKey]
		for targetField, valueVotes := range contextFields {
			if metadata[targetField] != "" {
				continue
			}
			bestValue := pickBestValueByVotes(model, valueVotes)
			if bestValue != "" {
				metadata[targetField] = bestValue
			}
		}
	}
}

func applyMetadataQualityGuards(metadata map[string]string) {
	if metadata == nil {
		return
	}

	title := stripFixedTitleNoiseTags(strings.TrimSpace(metadata["title"]))
	title = stripRecognizedMetadataTitleSuffix(title, metadata)
	if title == "" {
		delete(metadata, "title")
	} else {
		metadata["title"] = title
	}

	seriesName := stripFixedTitleNoiseTags(strings.TrimSpace(metadata["series_name"]))
	if seriesName == "" {
		delete(metadata, "series_name")
		return
	}
	metadata["series_name"] = seriesName
	if title == "" {
		return
	}
	if !isSeriesNameRelevantToTitle(seriesName, title) {
		delete(metadata, "series_name")
	}
}

func stripFixedTitleNoiseTags(raw string) string {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return ""
	}

	noiseKeywords := []string{
		"中国翻訳",
		"DL版",
		"無修正",
		"重新排列",
		"白碼",
		"薄碼",
		"白碼、薄碼",
		"疏碼",
	}
	noiseWrappers := [][2]string{
		{"[", "]"},
		{"【", "】"},
		{"(", ")"},
		{"（", "）"},
	}
	for _, keyword := range noiseKeywords {
		for _, wrapper := range noiseWrappers {
			cleaned = strings.ReplaceAll(cleaned, wrapper[0]+keyword+wrapper[1], " ")
		}
	}
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	return strings.TrimSpace(cleaned)
}

func stripRecognizedMetadataTitleSuffix(title string, metadata map[string]string) string {
	cleaned := strings.TrimSpace(title)
	if cleaned == "" || len(metadata) == 0 {
		return cleaned
	}
	for {
		next, changed := stripOneRecognizedMetadataTitleSuffix(cleaned, metadata)
		if !changed {
			return cleaned
		}
		cleaned = next
	}
}

func stripOneRecognizedMetadataTitleSuffix(title string, metadata map[string]string) (string, bool) {
	wrappers := [][2]string{{"(", ")"}, {"（", "）"}, {"[", "]"}, {"【", "】"}}
	for _, wrapper := range wrappers {
		prefix, inner, ok := splitTrailingWrappedText(title, wrapper[0], wrapper[1])
		if !ok {
			continue
		}
		if isRecognizedMetadataSuffix(inner, metadata) {
			return prefix, true
		}
	}
	return title, false
}

func splitTrailingWrappedText(raw string, open string, close string) (string, string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !strings.HasSuffix(trimmed, close) {
		return "", "", false
	}
	end := len(trimmed) - len(close)
	start := strings.LastIndex(trimmed[:end], open)
	if start < 0 {
		return "", "", false
	}
	prefix := strings.TrimSpace(trimmed[:start])
	inner := strings.TrimSpace(trimmed[start+len(open) : end])
	if prefix == "" || inner == "" {
		return "", "", false
	}
	return prefix, inner, true
}

func isRecognizedMetadataSuffix(raw string, metadata map[string]string) bool {
	normalizedRaw := normalizeSeriesSimilarityKey(raw)
	if normalizedRaw == "" {
		return false
	}
	for _, candidate := range recognizedMetadataSuffixCandidates(metadata) {
		if normalizeSeriesSimilarityKey(candidate) == normalizedRaw {
			return true
		}
	}
	return false
}

func recognizedMetadataSuffixCandidates(metadata map[string]string) []string {
	values := uniqueRecognizedMetadataSuffixValues(metadata)
	if len(values) == 0 {
		return nil
	}
	candidates := make([]string, 0, len(values)+4)
	candidates = append(candidates, values...)
	if len(values) >= 2 {
		for i := 0; i < len(values); i++ {
			for j := i + 1; j < len(values); j++ {
				candidates = append(candidates, strings.TrimSpace(values[i]+" "+values[j]))
			}
		}
	}
	if len(values) >= 3 {
		candidates = append(candidates, strings.TrimSpace(values[0]+" "+values[1]+" "+values[2]))
	}
	return candidates
}

func uniqueRecognizedMetadataSuffixValues(metadata map[string]string) []string {
	fields := []string{"comic_market", "event_code", "original_work"}
	values := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, field := range fields {
		value := cleanInferenceSegmentText(metadata[field])
		normalized := normalizeSeriesSimilarityKey(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		values = append(values, value)
	}
	return values
}

func isSeriesNameRelevantToTitle(seriesName string, title string) bool {
	seriesKey := normalizeSeriesSimilarityKey(seriesName)
	titleKey := normalizeSeriesSimilarityKey(title)
	if seriesKey == "" || titleKey == "" {
		return false
	}
	if isGenericCategorySeriesName(seriesKey) {
		return false
	}
	if strings.Contains(titleKey, seriesKey) {
		return true
	}
	if len([]rune(seriesKey)) >= 3 && strings.Contains(seriesKey, titleKey) {
		return true
	}

	commonLen := longestCommonSubstringRuneLen(seriesKey, titleKey)
	minCommon := 2
	if len([]rune(seriesKey)) >= 5 || len([]rune(titleKey)) >= 5 {
		minCommon = 3
	}
	return commonLen >= minCommon
}

func normalizeSeriesSimilarityKey(raw string) string {
	cleaned := cleanInferenceSegmentText(raw)
	if cleaned == "" {
		return ""
	}
	cleaned = strings.ToLower(cleaned)
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return r
		}
		return -1
	}, cleaned)
}

func isGenericCategorySeriesName(seriesKey string) bool {
	switch seriesKey {
	case "漫画", "manga", "comic", "comics", "同人", "同人志", "doujin", "books", "book":
		return true
	default:
		return false
	}
}

func longestCommonSubstringRuneLen(left string, right string) int {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	if len(leftRunes) == 0 || len(rightRunes) == 0 {
		return 0
	}

	prev := make([]int, len(rightRunes)+1)
	best := 0
	for i := 1; i <= len(leftRunes); i++ {
		curr := make([]int, len(rightRunes)+1)
		for j := 1; j <= len(rightRunes); j++ {
			if leftRunes[i-1] == rightRunes[j-1] {
				curr[j] = prev[j-1] + 1
				if curr[j] > best {
					best = curr[j]
				}
			}
		}
		prev = curr
	}
	return best
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

func pickBestValueByVotes(model *RepoPathAnalysisModel, votes map[string]int) string {
	bestKey := ""
	bestCount := -1
	keys := make([]string, 0, len(votes))
	for key := range votes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		count := votes[key]
		if count > bestCount || (count == bestCount && len([]rune(key)) > len([]rune(bestKey))) {
			bestKey = key
			bestCount = count
		}
	}
	if bestKey == "" {
		return ""
	}
	if model != nil {
		if canonical := model.CanonicalValues[bestKey]; canonical != "" {
			return canonical
		}
	}
	return bestKey
}

func matchMetadataFieldsForToken(model *RepoPathAnalysisModel, token string) map[string]string {
	results := map[string]string{}
	if model == nil {
		return results
	}
	for _, candidate := range extractInferenceCandidates(token) {
		candidateKey := normalizeInferenceKey(candidate)
		if candidateKey == "" {
			continue
		}
		if fields, ok := model.ValueFieldCounts[candidateKey]; ok {
			for field := range fields {
				results[field] = candidate
			}
			continue
		}

		for field, values := range model.FieldValueCounts {
			if field == "title" || field == "series_name" {
				continue
			}
			bestKey := ""
			bestCount := 0
			for valueKey, count := range values {
				if strings.Contains(valueKey, candidateKey) || strings.Contains(candidateKey, valueKey) {
					if count > bestCount || (count == bestCount && len([]rune(valueKey)) > len([]rune(bestKey))) {
						bestKey = valueKey
						bestCount = count
					}
				}
			}
			if bestKey != "" {
				results[field] = pickBestValueByVotes(model, map[string]int{bestKey: bestCount})
			}
		}
	}
	return results
}

func extractInferenceCandidates(raw string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, 6)
	appendCandidate := func(text string) {
		cleaned := cleanInferenceSegmentText(text)
		if cleaned == "" {
			return
		}
		key := normalizeInferenceKey(cleaned)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		result = append(result, cleaned)
	}

	cleaned := cleanInferenceSegmentText(raw)
	appendCandidate(cleaned)
	inner := stripOuterBrackets(cleaned)
	appendCandidate(inner)
	for _, segment := range splitPathInferenceSegments(inner) {
		appendCandidate(segment.Text)
	}
	for _, part := range strings.FieldsFunc(inner, func(r rune) bool {
		switch r {
		case '/', '／', '|', '｜', ',', '，', '、', ';', '；':
			return true
		default:
			return false
		}
	}) {
		appendCandidate(part)
	}
	return result
}

func splitPathInferenceSegments(raw string) []pathInferenceSegment {
	text := strings.TrimSpace(autoBalanceBracketText(raw))
	if text == "" {
		return nil
	}

	segments := make([]pathInferenceSegment, 0, 6)
	var builder strings.Builder
	depth := 0
	currentBracketed := false
	flush := func(bracketed bool) {
		segmentText := strings.TrimSpace(builder.String())
		builder.Reset()
		if segmentText == "" {
			return
		}
		segments = append(segments, pathInferenceSegment{Text: segmentText, Bracketed: bracketed})
	}

	for _, r := range text {
		switch r {
		case '【', '[', '(', '（':
			if depth == 0 && builder.Len() > 0 {
				flush(false)
			}
			currentBracketed = true
			depth++
			builder.WriteRune(r)
		case '】', ']', ')', '）':
			if depth == 0 {
				builder.WriteRune(r)
				continue
			}
			builder.WriteRune(r)
			depth--
			if depth == 0 {
				flush(true)
				currentBracketed = false
			}
		default:
			builder.WriteRune(r)
		}
	}
	if builder.Len() > 0 {
		flush(currentBracketed)
	}
	return segments
}

func stripBracketedSegments(raw string) string {
	segments := splitPathInferenceSegments(raw)
	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment.Bracketed {
			continue
		}
		cleaned := cleanInferenceSegmentText(segment.Text)
		if cleaned != "" {
			parts = append(parts, cleaned)
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func bestBareSegment(raw string) string {
	segments := splitPathInferenceSegments(raw)
	best := ""
	for _, segment := range segments {
		if segment.Bracketed {
			continue
		}
		candidate := cleanInferenceSegmentText(segment.Text)
		if len([]rune(candidate)) > len([]rune(best)) {
			best = candidate
		}
	}
	return best
}

func trimIgnoredPrefixCandidate(model *RepoPathAnalysisModel, text string) string {
	trimmed := cleanInferenceSegmentText(text)
	if trimmed == "" || model == nil || len(model.IgnoredPrefixRaw) == 0 {
		return trimmed
	}
	type prefixEntry struct {
		key   string
		raw   string
		count int
	}
	entries := make([]prefixEntry, 0, len(model.IgnoredPrefixRaw))
	for key, raw := range model.IgnoredPrefixRaw {
		entries = append(entries, prefixEntry{key: key, raw: raw, count: model.IgnoredPrefixCounts[key]})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if len([]rune(entries[i].raw)) == len([]rune(entries[j].raw)) {
			return entries[i].count > entries[j].count
		}
		return len([]rune(entries[i].raw)) > len([]rune(entries[j].raw))
	})
	for _, entry := range entries {
		prefix := strings.TrimSpace(entry.raw)
		if prefix == "" {
			continue
		}
		if strings.HasPrefix(trimmed, prefix) {
			next := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
			if cleanInferenceSegmentText(next) != "" {
				trimmed = next
			}
		}
	}
	return cleanInferenceSegmentText(trimmed)
}

func cleanInferenceSegmentText(raw string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(raw, "　", " "))
	if trimmed == "" {
		return ""
	}
	for {
		stripped := stripOuterBrackets(trimmed)
		if stripped == trimmed {
			break
		}
		trimmed = strings.TrimSpace(stripped)
	}
	trimmed = strings.Trim(trimmed, " \t\r\n-_~·・—–:：;；,，.。[](){}【】（）")
	trimmed = strings.Join(strings.Fields(trimmed), " ")
	return strings.TrimSpace(trimmed)
}

func stripOuterBrackets(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) < 2 {
		return trimmed
	}
	pairs := map[rune]rune{'【': '】', '[': ']', '(': ')', '（': '）'}
	first := runes[0]
	last := runes[len(runes)-1]
	if expected, ok := pairs[first]; ok && expected == last {
		return strings.TrimSpace(string(runes[1 : len(runes)-1]))
	}
	return trimmed
}

func normalizeInferenceKey(raw string) string {
	cleaned := cleanInferenceSegmentText(raw)
	if cleaned == "" {
		return ""
	}
	cleaned = strings.ToLower(cleaned)
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	return strings.TrimSpace(cleaned)
}

func isLikelyNoiseTitle(text string) bool {
	cleaned := cleanInferenceSegmentText(text)
	if cleaned == "" {
		return true
	}
	if len([]rune(cleaned)) == 1 && !hasMeaningfulRunes(cleaned) {
		return true
	}
	return false
}

func looksLikeContributorTag(text string) bool {
	cleaned := cleanInferenceSegmentText(text)
	if cleaned == "" {
		return false
	}
	if strings.ContainsAny(cleaned, ",，") {
		return true
	}
	if strings.Contains(cleaned, "(") || strings.Contains(cleaned, "（") || strings.Contains(cleaned, ")") || strings.Contains(cleaned, "）") {
		return true
	}
	return false
}

func hasMeaningfulRunes(text string) bool {
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return true
		}
	}
	return false
}

func autoBalanceBracketText(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	pairs := map[rune]rune{'【': '】', '[': ']', '(': ')', '（': '）'}
	closing := map[rune]rune{'】': '【', ']': '[', ')': '(', '）': '（'}
	stack := make([]rune, 0, 8)
	var builder strings.Builder
	for _, r := range text {
		if closeRune, ok := pairs[r]; ok {
			builder.WriteRune(r)
			stack = append(stack, closeRune)
			continue
		}
		if _, ok := closing[r]; ok {
			for len(stack) > 0 && stack[len(stack)-1] != r {
				builder.WriteRune(stack[len(stack)-1])
				stack = stack[:len(stack)-1]
			}
			if len(stack) > 0 && stack[len(stack)-1] == r {
				builder.WriteRune(r)
				stack = stack[:len(stack)-1]
			}
			continue
		}
		builder.WriteRune(r)
	}
	for i := len(stack) - 1; i >= 0; i-- {
		builder.WriteRune(stack[i])
	}
	return strings.TrimSpace(builder.String())
}

package textanalyzer

import (
	"strings"
	"unicode"
)

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
	noiseWrappers := [][2]string{{"[", "]"}, {"【", "】"}, {"(", ")"}, {"（", "）"}}
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

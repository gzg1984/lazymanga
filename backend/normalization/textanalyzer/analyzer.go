package textanalyzer

import (
	"sort"
	"strings"
	"unicode"
)

func NewAnalyzer() Analyzer {
	return defaultAnalyzer{}
}

type defaultAnalyzer struct{}

type candidateMatch struct {
	AnalyzeMatch
	fieldPriority int
	fieldMulti    bool
	valueWeight   int
	spanLength    int
	matchedLength int
	normalized    string
}

func (defaultAnalyzer) Analyze(req AnalyzeTextRequest, registry AnalysisHintRegistry) (AnalyzeTextResult, error) {
	input := req.Input
	preparedInput := strings.TrimSpace(input)
	warnings := make([]string, 0, 1)
	if req.AutoRepairBrackets {
		repairedInput := autoBalanceBracketText(preparedInput)
		if repairedInput != preparedInput {
			warnings = append(warnings, "input_brackets_auto_repaired")
		}
		preparedInput = repairedInput
	}
	normalizedInput := normalizeInput(preparedInput)
	result := AnalyzeTextResult{
		Input:           input,
		NormalizedInput: normalizedInput,
		Fields:          map[string][]string{},
		Warnings:        warnings,
	}
	if normalizedInput == "" {
		return result, nil
	}

	candidates := collectCandidates(normalizedInput, registry)
	accepted, rejected := resolveCandidates(candidates, req.PreferLongestMatch, req.MaxResults)
	result.Rejected = rejected
	result.Matches = make([]AnalyzeMatch, 0, len(accepted))
	for _, item := range accepted {
		result.Matches = append(result.Matches, item.AnalyzeMatch)
		result.Fields[item.Key] = appendUnique(result.Fields[item.Key], item.Value)
	}
	if len(result.Fields) == 0 {
		result.ResidualText = normalizedInput
		result.TitleCandidate = normalizedInput
		return result, nil
	}

	result.ResidualText = buildResidualText(normalizedInput, accepted)
	result.TitleCandidate = result.ResidualText
	return result, nil
}

func normalizeInput(raw string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
}

func collectCandidates(input string, registry AnalysisHintRegistry) []candidateMatch {
	if strings.TrimSpace(input) == "" || len(registry.Fields) == 0 {
		return nil
	}

	lowerInput := strings.ToLower(input)
	results := make([]candidateMatch, 0)
	for _, field := range registry.Fields {
		key := strings.TrimSpace(field.Key)
		if key == "" {
			continue
		}
		for _, value := range field.Values {
			canonicalValue := strings.TrimSpace(value.CanonicalValue)
			if canonicalValue == "" {
				continue
			}
			terms := make([]string, 0, len(value.Aliases)+1)
			terms = append(terms, canonicalValue)
			terms = append(terms, value.Aliases...)
			for _, term := range terms {
				normalizedTerm := normalizeInput(term)
				if normalizedTerm == "" {
					continue
				}
				start := strings.Index(lowerInput, strings.ToLower(normalizedTerm))
				if start < 0 {
					continue
				}
				end := start + len(normalizedTerm)
				if !isBoundary(lowerInput, start, end) {
					continue
				}
				confidence := 0.9
				if strings.EqualFold(normalizedTerm, canonicalValue) {
					confidence = 1.0
				}
				results = append(results, candidateMatch{
					AnalyzeMatch: AnalyzeMatch{
						Key:         key,
						Value:       canonicalValue,
						Alias:       normalizedTerm,
						Source:      strings.TrimSpace(value.Source),
						Confidence:  confidence,
						Start:       start,
						End:         end,
						MatchedText: input[start:end],
					},
					fieldPriority: field.Priority,
					fieldMulti:    field.MultiValue,
					valueWeight:   value.Weight,
					spanLength:    end - start,
					matchedLength: len(normalizedTerm),
					normalized:    strings.ToLower(normalizedTerm),
				})
				break
			}
		}
	}
	return results
}

func resolveCandidates(candidates []candidateMatch, preferLongest bool, maxResults int) ([]candidateMatch, []AnalyzeRejectedMatch) {
	if len(candidates) == 0 {
		return nil, nil
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if left.Start != right.Start {
			return left.Start < right.Start
		}
		if preferLongest && left.spanLength != right.spanLength {
			return left.spanLength > right.spanLength
		}
		if left.fieldPriority != right.fieldPriority {
			return left.fieldPriority > right.fieldPriority
		}
		if left.valueWeight != right.valueWeight {
			return left.valueWeight > right.valueWeight
		}
		if left.Confidence != right.Confidence {
			return left.Confidence > right.Confidence
		}
		return left.Key < right.Key
	})

	accepted := make([]candidateMatch, 0, len(candidates))
	usedSpans := make([]candidateMatch, 0, len(candidates))
	rejected := make([]AnalyzeRejectedMatch, 0)
	selectedValuesByField := map[string]map[string]struct{}{}

	for _, candidate := range candidates {
		if maxResults > 0 && len(accepted) >= maxResults {
			rejected = append(rejected, AnalyzeRejectedMatch{
				Key:         candidate.Key,
				Value:       candidate.Value,
				Reason:      "max_results_exceeded",
				MatchedText: candidate.MatchedText,
			})
			continue
		}
		if overlapsExisting(candidate, usedSpans) {
			rejected = append(rejected, AnalyzeRejectedMatch{
				Key:         candidate.Key,
				Value:       candidate.Value,
				Reason:      "overlapping_match",
				MatchedText: candidate.MatchedText,
			})
			continue
		}
		if !candidate.fieldMulti {
			if _, exists := selectedValuesByField[candidate.Key]; exists {
				rejected = append(rejected, AnalyzeRejectedMatch{
					Key:         candidate.Key,
					Value:       candidate.Value,
					Reason:      "single_value_field_already_selected",
					MatchedText: candidate.MatchedText,
				})
				continue
			}
		}
		selectedSet := selectedValuesByField[candidate.Key]
		if selectedSet == nil {
			selectedSet = map[string]struct{}{}
			selectedValuesByField[candidate.Key] = selectedSet
		}
		if _, exists := selectedSet[candidate.Value]; exists {
			rejected = append(rejected, AnalyzeRejectedMatch{
				Key:         candidate.Key,
				Value:       candidate.Value,
				Reason:      "duplicate_value",
				MatchedText: candidate.MatchedText,
			})
			continue
		}
		selectedSet[candidate.Value] = struct{}{}
		accepted = append(accepted, candidate)
		usedSpans = append(usedSpans, candidate)
	}

	sort.SliceStable(accepted, func(i, j int) bool {
		return accepted[i].Start < accepted[j].Start
	})
	return accepted, rejected
}

func overlapsExisting(candidate candidateMatch, existing []candidateMatch) bool {
	for _, item := range existing {
		if candidate.Start < item.End && item.Start < candidate.End {
			return true
		}
	}
	return false
}

func buildResidualText(input string, accepted []candidateMatch) string {
	if len(accepted) == 0 {
		return input
	}
	var builder strings.Builder
	current := 0
	for _, item := range accepted {
		if item.Start > current {
			builder.WriteString(input[current:item.Start])
		}
		builder.WriteByte(' ')
		current = item.End
	}
	if current < len(input) {
		builder.WriteString(input[current:])
	}
	residual := strings.Join(strings.Fields(builder.String()), " ")
	residual = strings.NewReplacer(
		"( )", " ",
		"[]", " ",
		"[ ]", " ",
		"()", " ",
		"（ ）", " ",
		"（）", " ",
		"【 】", " ",
		"【】", " ",
	).Replace(residual)
	return strings.TrimSpace(strings.Join(strings.Fields(residual), " "))
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func isBoundary(input string, start int, end int) bool {
	if start > 0 {
		previous, _ := utf8DecodeLastRuneInString(input[:start])
		if isWordRune(previous) {
			return false
		}
	}
	if end < len(input) {
		next, _ := utf8DecodeRuneInString(input[end:])
		if isWordRune(next) {
			return false
		}
	}
	return true
}

func isWordRune(value rune) bool {
	if value == utf8RuneErrorSentinel {
		return false
	}
	return unicode.IsLetter(value) || unicode.IsDigit(value)
}

const utf8RuneErrorSentinel rune = -1

func utf8DecodeLastRuneInString(value string) (rune, int) {
	if value == "" {
		return utf8RuneErrorSentinel, 0
	}
	runes := []rune(value)
	if len(runes) == 0 {
		return utf8RuneErrorSentinel, 0
	}
	return runes[len(runes)-1], len(string(runes[len(runes)-1]))
}

func utf8DecodeRuneInString(value string) (rune, int) {
	if value == "" {
		return utf8RuneErrorSentinel, 0
	}
	runes := []rune(value)
	if len(runes) == 0 {
		return utf8RuneErrorSentinel, 0
	}
	return runes[0], len(string(runes[0]))
}

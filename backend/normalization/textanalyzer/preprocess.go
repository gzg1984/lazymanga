package textanalyzer

import (
	"strings"
)

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

package textanalyzer

import "strings"

type inferenceSegment struct {
	Text      string
	Bracketed bool
}

func splitInferenceSegments(raw string) []inferenceSegment {
	text := strings.TrimSpace(autoBalanceBracketText(raw))
	if text == "" {
		return nil
	}

	segments := make([]inferenceSegment, 0, 6)
	var builder strings.Builder
	depth := 0
	currentBracketed := false
	flush := func(bracketed bool) {
		segmentText := strings.TrimSpace(builder.String())
		builder.Reset()
		if segmentText == "" {
			return
		}
		segments = append(segments, inferenceSegment{Text: segmentText, Bracketed: bracketed})
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
	segments := splitInferenceSegments(raw)
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
	segments := splitInferenceSegments(raw)
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

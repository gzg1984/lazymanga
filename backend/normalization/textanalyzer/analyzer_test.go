package textanalyzer

import "testing"

func TestAnalyzerExtractsFieldsAndResidualText(t *testing.T) {
	analyzer := NewAnalyzer()
	registry := AnalysisHintRegistry{
		Fields: []AnalysisFieldHint{
			{
				Key:        "scanlator_group",
				Priority:   10,
				MultiValue: false,
				Values: []AnalysisValueHint{{
					CanonicalValue: "CE家族社",
					Aliases:        []string{"【CE家族社】", "CE家族社"},
					Weight:         10,
					Source:         "test-registry",
				}},
			},
			{
				Key:        "comic_market",
				Priority:   9,
				MultiValue: false,
				Values: []AnalysisValueHint{{
					CanonicalValue: "C86",
					Aliases:        []string{"(C86)", "C86"},
					Weight:         10,
					Source:         "test-registry",
				}},
			},
			{
				Key:        "original_work",
				Priority:   8,
				MultiValue: false,
				Values: []AnalysisValueHint{{
					CanonicalValue: "女子小学生はじめました",
					Aliases:        []string{"(女子小学生はじめました)"},
					Weight:         10,
					Source:         "test-registry",
				}},
			},
		},
	}

	result, err := analyzer.Analyze(AnalyzeTextRequest{
		Input:              "【CE家族社】 (C86) J's 4 (女子小学生はじめました)",
		PreferLongestMatch: true,
	}, registry)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if got := result.Fields["scanlator_group"]; len(got) != 1 || got[0] != "CE家族社" {
		t.Fatalf("unexpected scanlator_group fields: %#v", got)
	}
	if got := result.Fields["comic_market"]; len(got) != 1 || got[0] != "C86" {
		t.Fatalf("unexpected comic_market fields: %#v", got)
	}
	if got := result.Fields["original_work"]; len(got) != 1 || got[0] != "女子小学生はじめました" {
		t.Fatalf("unexpected original_work fields: %#v", got)
	}
	if result.ResidualText != "J's 4" {
		t.Fatalf("expected residual text J's 4, got %q", result.ResidualText)
	}
	if result.TitleCandidate != "J's 4" {
		t.Fatalf("expected title candidate J's 4, got %q", result.TitleCandidate)
	}
	if len(result.Matches) != 3 {
		t.Fatalf("expected 3 accepted matches, got %#v", result.Matches)
	}
}

func TestAnalyzerPrefersLongerOverlappingMatch(t *testing.T) {
	analyzer := NewAnalyzer()
	registry := AnalysisHintRegistry{
		Fields: []AnalysisFieldHint{
			{
				Key:      "author_name",
				Priority: 10,
				Values: []AnalysisValueHint{
					{CanonicalValue: "Comic Market", Aliases: []string{"Comic Market"}, Weight: 1},
					{CanonicalValue: "Comic Market 86", Aliases: []string{"Comic Market 86"}, Weight: 10},
				},
			},
		},
	}

	result, err := analyzer.Analyze(AnalyzeTextRequest{
		Input:              "[Comic Market 86] J's 4",
		PreferLongestMatch: true,
	}, registry)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if got := result.Fields["author_name"]; len(got) != 1 || got[0] != "Comic Market 86" {
		t.Fatalf("unexpected author_name fields: %#v", got)
	}
	if len(result.Rejected) != 1 || result.Rejected[0].Reason != "overlapping_match" {
		t.Fatalf("expected one overlapping rejection, got %#v", result.Rejected)
	}
}

package textanalyzer

import "testing"

func TestAnalyzerAutoRepairsBrokenBrackets(t *testing.T) {
	analyzer := NewAnalyzer()
	result, err := analyzer.Analyze(AnalyzeTextRequest{
		Input:              "【CE家族社】[(C86] J's 4",
		AutoRepairBrackets: true,
	}, AnalysisHintRegistry{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if result.NormalizedInput != "【CE家族社】[(C86)] J's 4" {
		t.Fatalf("unexpected normalized input after repair: %q", result.NormalizedInput)
	}
	if len(result.Warnings) != 1 || result.Warnings[0] != "input_brackets_auto_repaired" {
		t.Fatalf("expected auto repair warning, got %#v", result.Warnings)
	}
}

func TestAutoBalanceBracketTextRepairsBrokenSequence(t *testing.T) {
	got := autoBalanceBracketText("[(C86]")
	if got != "[(C86)]" {
		t.Fatalf("expected repaired bracket text, got %q", got)
	}
}

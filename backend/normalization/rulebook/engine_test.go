package rulebook

import "testing"

func TestEnginePriority(t *testing.T) {
	book := RuleBook{
		Name:    "test",
		Version: "v1",
		Rules: []Rule{
			{
				ID:       "low",
				Priority: 20,
				Enabled:  true,
				Match: Condition{
					FileNameContains: []string{"ubuntu"},
				},
				Action: Action{TargetDir: "A", RuleType: "A"},
			},
			{
				ID:       "high",
				Priority: 10,
				Enabled:  true,
				Match: Condition{
					FileNameContains: []string{"ubuntu"},
				},
				Action: Action{TargetDir: "B", RuleType: "B"},
			},
		},
	}

	engine, err := NewEngine(book)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	result, err := engine.Evaluate(EvalInput{FileName: "ubuntu.iso"})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected matched result")
	}
	if result.RuleID != "high" || result.TargetDir != "B" {
		t.Fatalf("unexpected rule selection: %#v", result)
	}
}

func TestDefaultOSRuleBookExplicitOSFallback(t *testing.T) {
	engine := MustNewEngine(DefaultOSRelocationRuleBook())

	result, err := engine.Evaluate(EvalInput{
		FileName:        "unknown-system-build.iso",
		IsOS:            true,
		IsEntertainment: false,
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected matched result")
	}
	if result.TargetDir != "OS" || result.RuleType != "OS" {
		t.Fatalf("unexpected fallback action: %#v", result)
	}
}

func TestDefaultOSRuleBookInferByName(t *testing.T) {
	engine := MustNewEngine(DefaultOSRelocationRuleBook())

	result, err := engine.Evaluate(EvalInput{
		FileName:        "ubuntu-24.04-live-server.iso",
		IsOS:            false,
		IsEntertainment: false,
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected matched result")
	}
	if result.TargetDir != "OS/Linux/Ubuntu" {
		t.Fatalf("unexpected target dir: %#v", result)
	}
	if !result.InferIsOS {
		t.Fatalf("expected infer is_os=true")
	}
}

func TestDefaultOSRuleBookNoMatchWhenNotOSAndNoKeyword(t *testing.T) {
	engine := MustNewEngine(DefaultOSRelocationRuleBook())

	result, err := engine.Evaluate(EvalInput{
		FileName:        "movie-night.iso",
		IsOS:            false,
		IsEntertainment: false,
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if result.Matched {
		t.Fatalf("expected unmatched result, got: %#v", result)
	}
}

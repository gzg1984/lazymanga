package rulebook

// RuleBook defines a set of ordered rules used by the relocation engine.
type RuleBook struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Rules   []Rule `json:"rules"`
}

// Rule defines a single match + action pair.
type Rule struct {
	ID       string    `json:"id"`
	Priority int       `json:"priority"`
	Enabled  bool      `json:"enabled"`
	Match    Condition `json:"match"`
	Action   Action    `json:"action"`
}

// Condition defines rule matching constraints.
type Condition struct {
	IsOS             *bool    `json:"is_os,omitempty"`
	IsEntertainment  *bool    `json:"is_entertainment,omitempty"`
	FileNameContains []string `json:"file_name_contains,omitempty"`
}

// Action defines the relocation outcome when a rule matches.
type Action struct {
	TargetDir string `json:"target_dir"`
	RuleType  string `json:"rule_type"`
	InferIsOS bool   `json:"infer_is_os"`
}

// EvalInput is the minimum state required by rule engine.
type EvalInput struct {
	FileName        string
	IsOS            bool
	IsEntertainment bool
}

// EvalResult contains matched rule and action details.
type EvalResult struct {
	Matched     bool   `json:"matched"`
	RuleID      string `json:"rule_id"`
	RuleBook    string `json:"rule_book"`
	RuleVersion string `json:"rule_version"`
	TargetDir   string `json:"target_dir"`
	RuleType    string `json:"rule_type"`
	Keyword     string `json:"keyword"`
	InferIsOS   bool   `json:"infer_is_os"`
}

// Validate checks basic shape constraints for a rule book.
func (b RuleBook) Validate() error {
	if b.Name == "" {
		return ErrInvalidRuleBook("rule book name required")
	}
	if b.Version == "" {
		return ErrInvalidRuleBook("rule book version required")
	}

	for _, rule := range b.Rules {
		if rule.ID == "" {
			return ErrInvalidRuleBook("rule id required")
		}
		if rule.Action.TargetDir == "" {
			return ErrInvalidRuleBook("rule action target_dir required")
		}
		if rule.Action.RuleType == "" {
			return ErrInvalidRuleBook("rule action rule_type required")
		}
	}

	return nil
}


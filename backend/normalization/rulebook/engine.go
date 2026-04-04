package rulebook

import (
	"fmt"
	"sort"
	"strings"
)

// Engine evaluates relocation decisions from a fixed rule book snapshot.
type Engine struct {
	book RuleBook
}

// NewEngine builds an immutable engine from a rule book.
func NewEngine(book RuleBook) (*Engine, error) {
	if strings.TrimSpace(book.Name) == "" {
		return nil, fmt.Errorf("rule book name required")
	}

	rules := make([]Rule, 0, len(book.Rules))
	for _, rule := range book.Rules {
		if strings.TrimSpace(rule.ID) == "" {
			return nil, fmt.Errorf("rule id required")
		}
		rules = append(rules, rule)
	}

	sort.SliceStable(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})

	return &Engine{book: RuleBook{Name: book.Name, Version: book.Version, Rules: rules}}, nil
}

// MustNewEngine panics if the rule book is invalid.
func MustNewEngine(book RuleBook) *Engine {
	engine, err := NewEngine(book)
	if err != nil {
		panic(err)
	}
	return engine
}

// Evaluate returns the first matching rule result by priority.
func (e *Engine) Evaluate(in EvalInput) (EvalResult, error) {
	fileName := strings.ToLower(strings.TrimSpace(in.FileName))
	for _, rule := range e.book.Rules {
		if !rule.Enabled {
			continue
		}
		keyword, matched := matchCondition(rule.Match, in, fileName)
		if !matched {
			continue
		}

		return EvalResult{
			Matched:     true,
			RuleID:      rule.ID,
			RuleBook:    e.book.Name,
			RuleVersion: e.book.Version,
			TargetDir:   rule.Action.TargetDir,
			RuleType:    rule.Action.RuleType,
			Keyword:     keyword,
			InferIsOS:   rule.Action.InferIsOS,
		}, nil
	}

	return EvalResult{Matched: false, RuleBook: e.book.Name, RuleVersion: e.book.Version}, nil
}

func matchCondition(cond Condition, in EvalInput, lowerFileName string) (string, bool) {
	if cond.IsOS != nil && in.IsOS != *cond.IsOS {
		return "", false
	}
	if cond.IsEntertainment != nil && in.IsEntertainment != *cond.IsEntertainment {
		return "", false
	}

	if len(cond.FileNameContains) == 0 {
		return "", true
	}

	for _, keyword := range cond.FileNameContains {
		k := strings.ToLower(strings.TrimSpace(keyword))
		if k == "" {
			continue
		}
		if strings.Contains(lowerFileName, k) {
			return keyword, true
		}
	}

	return "", false
}

package normalization

import (
	"sync"
	"time"
)

// RuleBookLoadStatus describes runtime state for default rulebook loading.
type RuleBookLoadStatus struct {
	Source        string    `json:"source"`
	FilePath      string    `json:"file_path"`
	UsingFallback bool      `json:"using_fallback"`
	LastError     string    `json:"last_error"`
	BookName      string    `json:"book_name"`
	BookVersion   string    `json:"book_version"`
	RuleCount     int       `json:"rule_count"`
	UpdatedAt     time.Time `json:"updated_at"`
}

var (
	ruleBookStatusMu          sync.RWMutex
	defaultRuleBookLoadStatus RuleBookLoadStatus
)

func setDefaultRuleBookLoadStatus(status RuleBookLoadStatus) {
	ruleBookStatusMu.Lock()
	defer ruleBookStatusMu.Unlock()
	defaultRuleBookLoadStatus = status
}

// GetDefaultRuleBookLoadStatus returns a snapshot for API/UI inspection.
func GetDefaultRuleBookLoadStatus() RuleBookLoadStatus {
	ruleBookStatusMu.RLock()
	defer ruleBookStatusMu.RUnlock()
	return defaultRuleBookLoadStatus
}

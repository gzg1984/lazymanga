package rulebook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadRuleBookFromFile loads and validates a rule book from a JSON file.
func LoadRuleBookFromFile(path string) (RuleBook, error) {
	cleanPath := filepath.Clean(path)
	raw, err := os.ReadFile(cleanPath)
	if err != nil {
		return RuleBook{}, fmt.Errorf("read rule book file %q: %w", cleanPath, err)
	}

	var book RuleBook
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&book); err != nil {
		return RuleBook{}, fmt.Errorf("decode rule book file %q: %w", cleanPath, err)
	}

	if err := book.Validate(); err != nil {
		return RuleBook{}, fmt.Errorf("validate rule book file %q: %w", cleanPath, err)
	}

	return book, nil
}

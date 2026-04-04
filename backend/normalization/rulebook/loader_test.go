package rulebook

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRuleBookFromFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "book.json")

	content := `{
  "name": "test-book",
  "version": "v1",
  "rules": [
    {
      "id": "r1",
      "priority": 10,
      "enabled": true,
      "match": {"is_os": true},
      "action": {
        "target_dir": "OS",
        "rule_type": "OS",
        "infer_is_os": false
      }
    }
  ]
}`

	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	book, err := LoadRuleBookFromFile(filePath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if book.Name != "test-book" || book.Version != "v1" {
		t.Fatalf("unexpected metadata: %#v", book)
	}
	if len(book.Rules) != 1 || book.Rules[0].ID != "r1" {
		t.Fatalf("unexpected rules: %#v", book.Rules)
	}
}

func TestLoadRuleBookFromFileInvalid(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "invalid.json")

	if err := os.WriteFile(filePath, []byte(`{"name":"","version":"v1","rules":[]}`), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	_, err := LoadRuleBookFromFile(filePath)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadRuleBookFromFileNoopRulesAllowed(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "noop.json")

	if err := os.WriteFile(filePath, []byte(`{"name":"noop","version":"v1","rules":[]}`), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	book, err := LoadRuleBookFromFile(filePath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(book.Rules) != 0 {
		t.Fatalf("expected empty rules, got=%d", len(book.Rules))
	}
}

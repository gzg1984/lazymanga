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
  "scan": {
    "extensions": [".cbz", "zip"]
  },
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
	if !book.ShouldScanFile("chapter01.cbz") || !book.ShouldScanFile("chapter02.zip") {
		t.Fatalf("expected scan config to be loaded from json: %#v", book.Scan)
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

func TestRuleBookShouldScanFileUsesConfiguredExtensions(t *testing.T) {
	book := RuleBook{
		Name:    "manga",
		Version: "v1",
		Scan: ScanSpec{
			Extensions: []string{".cbz", "zip"},
		},
	}

	if !book.ShouldScanFile("chapter01.cbz") {
		t.Fatalf("expected .cbz file to be scanned")
	}
	if !book.ShouldScanFile("chapter02.ZIP") {
		t.Fatalf("expected .ZIP file to be scanned case-insensitively")
	}
	if book.ShouldScanFile("cover.iso") {
		t.Fatalf("did not expect .iso file when scan extensions are overridden")
	}
}

func TestRuleBookShouldScanFileDefaultsToISO(t *testing.T) {
	book := RuleBook{Name: "noop", Version: "v1"}

	if !book.ShouldScanFile("system.iso") {
		t.Fatalf("expected .iso file to be scanned by default")
	}
	if book.ShouldScanFile("comic.cbz") {
		t.Fatalf("did not expect .cbz file without explicit scan config")
	}
}

func TestScanSpecMatchDirectoryFilesUsesThreshold(t *testing.T) {
	spec := ScanSpec{
		DirectoryRules: []DirectoryScanRule{{
			Name:         "image-folder",
			Extensions:   []string{".jpg", ".jpeg", ".png"},
			MinFileCount: 3,
		}},
	}

	rule, count, ok := spec.MatchDirectoryFiles([]string{"001.jpg", "002.JPG", "003.png", "note.txt"})
	if !ok {
		t.Fatalf("expected directory rule to match")
	}
	if rule.Name != "image-folder" {
		t.Fatalf("unexpected matched rule: %#v", rule)
	}
	if count != 3 {
		t.Fatalf("expected 3 matching files, got %d", count)
	}
}

func TestLoadRuleBookFromFileWithDirectoryTransform(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "karita.json")

	content := `{
  "name": "karita-manga",
  "version": "v1",
  "scan": {
    "directory_rules": [
      {
        "name": "karita-folder",
        "extensions": [".jpg", ".png"],
        "min_file_count": 2,
        "transform": {
          "pattern": "^\\[(?P<circle>[^\\]]+)\\]\\s*(?P<title>.+)$",
          "rename_template": "${title}",
          "metadata_file": ".karita.meta.json",
          "metadata": {
            "circle": "${circle}"
          }
        }
      }
    ]
  },
  "rules": []
}`

	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	book, err := LoadRuleBookFromFile(filePath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(book.Scan.DirectoryRules) != 1 || book.Scan.DirectoryRules[0].Transform == nil {
		t.Fatalf("expected transform config to be loaded: %#v", book.Scan.DirectoryRules)
	}
	if book.Scan.DirectoryRules[0].Transform.MetadataFile != ".karita.meta.json" {
		t.Fatalf("unexpected metadata file: %#v", book.Scan.DirectoryRules[0].Transform)
	}
}

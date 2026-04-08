package normalization

import (
	"strings"
	"testing"

	"lazymanga/normalization/rulebook"
)

func TestSaveUserRuleBookAndListAvailableRuleBooks(t *testing.T) {
	tmpDir := t.TempDir()
	SetRuleBookUserDir(tmpDir)
	t.Cleanup(func() {
		SetRuleBookUserDir("")
	})

	book := rulebook.RuleBook{
		Name:    "custom-manga",
		Version: "v1",
		Scan: rulebook.ScanSpec{
			Extensions: []string{".cbz", ".zip"},
		},
		Rules: []rulebook.Rule{},
	}

	saved, err := SaveUserRuleBook(book, false)
	if err != nil {
		t.Fatalf("SaveUserRuleBook failed: %v", err)
	}
	if saved.Source != "user" {
		t.Fatalf("expected user source, got %q", saved.Source)
	}
	if !saved.Editable {
		t.Fatal("expected saved rulebook to be editable")
	}

	items := ListAvailableRuleBooks()
	var found bool
	for _, item := range items {
		if item.Name == "custom-manga" && item.Version == "v1" {
			found = true
			if item.Source != "user" {
				t.Fatalf("expected listed source=user, got %q", item.Source)
			}
			if !item.Valid {
				t.Fatalf("expected listed rulebook to be valid, error=%q", item.Error)
			}
		}
	}
	if !found {
		t.Fatal("expected custom-manga@v1 to be listed")
	}

	if _, err := SaveUserRuleBook(book, false); err == nil {
		t.Fatal("expected duplicate save to fail without overwrite")
	}
}

func TestGetRuleBookFileContentReturnsSavedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	SetRuleBookUserDir(tmpDir)
	t.Cleanup(func() {
		SetRuleBookUserDir("")
	})

	book := rulebook.RuleBook{
		Name:    "editable-book",
		Version: "v2",
		Scan: rulebook.ScanSpec{
			Extensions: []string{".cbz"},
		},
		Rules: []rulebook.Rule{},
	}
	if _, err := SaveUserRuleBook(book, false); err != nil {
		t.Fatalf("SaveUserRuleBook failed: %v", err)
	}

	item, raw, loaded, err := GetRuleBookFileContent("editable-book", "v2")
	if err != nil {
		t.Fatalf("GetRuleBookFileContent failed: %v", err)
	}
	if !item.Editable {
		t.Fatal("expected editable-book to be editable")
	}
	if loaded.Name != "editable-book" || loaded.Version != "v2" {
		t.Fatalf("unexpected loaded rulebook: %+v", loaded)
	}
	if !strings.Contains(string(raw), `"name": "editable-book"`) {
		t.Fatalf("expected raw json to contain rulebook name, got %s", string(raw))
	}
}

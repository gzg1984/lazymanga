package normalization

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGuessOSRuleByFileNameMatched(t *testing.T) {
	match, ok := GuessOSRuleByFileName("Windows11-24H2.iso")
	if !ok {
		t.Fatalf("expected match")
	}
	if match.TargetDir != "OS/Windows/Windows11" {
		t.Fatalf("unexpected target dir: %#v", match)
	}
}

func TestGuessOSRuleByFileNameNotMatched(t *testing.T) {
	_, ok := GuessOSRuleByFileName("my-random.iso")
	if ok {
		t.Fatalf("expected no match")
	}
}

func TestRelocateFileWithUniqueTargetKeepsSamePath(t *testing.T) {
	root := t.TempDir()
	sourceAbs := filepath.Join(root, "OS", "Ubuntu", "ubuntu-12.iso")
	if err := os.MkdirAll(filepath.Dir(sourceAbs), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(sourceAbs, []byte("iso"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	finalAbs, finalRel, moved, err := relocateFileWithUniqueTarget(sourceAbs, sourceAbs, root)
	if err != nil {
		t.Fatalf("relocate failed: %v", err)
	}
	if moved {
		t.Fatalf("expected moved=false")
	}
	if finalAbs != sourceAbs {
		t.Fatalf("unexpected finalAbs: %q", finalAbs)
	}
	if finalRel != "OS/Ubuntu/ubuntu-12.iso" {
		t.Fatalf("unexpected finalRel: %q", finalRel)
	}
	if _, err := os.Stat(filepath.Join(root, "OS", "Ubuntu", "ubuntu-12_1.iso")); !os.IsNotExist(err) {
		t.Fatalf("unexpected suffixed file created")
	}
}

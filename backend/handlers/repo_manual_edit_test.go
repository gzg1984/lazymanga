package handlers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRelocateRepoISOFileKeepsSamePath(t *testing.T) {
	root := t.TempDir()
	sourceAbs := filepath.Join(root, "OS", "ubuntu-12.iso")
	if err := os.MkdirAll(filepath.Dir(sourceAbs), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(sourceAbs, []byte("iso"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	finalAbs, finalRel, moved, err := relocateRepoISOFile(sourceAbs, sourceAbs, root)
	if err != nil {
		t.Fatalf("relocate failed: %v", err)
	}
	if moved {
		t.Fatalf("expected moved=false")
	}
	if finalAbs != sourceAbs {
		t.Fatalf("unexpected finalAbs: %q", finalAbs)
	}
	if finalRel != "OS/ubuntu-12.iso" {
		t.Fatalf("unexpected finalRel: %q", finalRel)
	}
	if _, err := os.Stat(sourceAbs); err != nil {
		t.Fatalf("expected source file to remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "OS", "ubuntu-12_1.iso")); !os.IsNotExist(err) {
		t.Fatalf("unexpected suffixed file created")
	}
}

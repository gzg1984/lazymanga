package handlers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportRepoDirectoryKeepsPathWithinRoot(t *testing.T) {
	root := t.TempDir()
	sourceAbs := filepath.Join(root, "series", "vol1")
	if err := os.MkdirAll(sourceAbs, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	fileAbs := filepath.Join(sourceAbs, "chapter01.iso")
	if err := os.WriteFile(fileAbs, []byte("iso"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	targetAbs, copied, err := importRepoDirectory(root, sourceAbs)
	if err != nil {
		t.Fatalf("importRepoDirectory failed: %v", err)
	}
	if copied {
		t.Fatalf("expected copied=false for path already inside repo root")
	}
	if targetAbs != sourceAbs {
		t.Fatalf("unexpected targetAbs: %q", targetAbs)
	}
	if _, err := os.Stat(filepath.Join(targetAbs, "chapter01.iso")); err != nil {
		t.Fatalf("expected original file to remain accessible: %v", err)
	}
}

func TestImportRepoDirectoryCopiesOutsidePathIntoManualDir(t *testing.T) {
	root := t.TempDir()
	outsideBase := t.TempDir()
	sourceAbs := filepath.Join(outsideBase, "incoming", "seriesA")
	if err := os.MkdirAll(filepath.Join(sourceAbs, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceAbs, "nested", "chapter01.iso"), []byte("iso"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	targetAbs, copied, err := importRepoDirectory(root, sourceAbs)
	if err != nil {
		t.Fatalf("importRepoDirectory failed: %v", err)
	}
	if !copied {
		t.Fatalf("expected copied=true for path outside repo root")
	}
	if !isPathWithinRoot(root, targetAbs) {
		t.Fatalf("expected target to stay within repo root, got %q", targetAbs)
	}
	if filepath.Base(filepath.Dir(targetAbs)) != "manual_added_dirs" {
		t.Fatalf("expected copied directory under manual_added_dirs, got %q", targetAbs)
	}

	content, err := os.ReadFile(filepath.Join(targetAbs, "nested", "chapter01.iso"))
	if err != nil {
		t.Fatalf("expected copied file to exist: %v", err)
	}
	if string(content) != "iso" {
		t.Fatalf("unexpected copied file content: %q", string(content))
	}
}

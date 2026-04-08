package normalization

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCalculatePathSizeBytesRecursivelyForDirectory(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "chapter", "pages")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	files := map[string][]byte{
		filepath.Join(root, "cover.jpg"): []byte("cover"),
		filepath.Join(nested, "001.jpg"): []byte("page-001"),
		filepath.Join(nested, "002.jpg"): []byte("page-002-data"),
	}
	var expected int64
	for absPath, content := range files {
		if err := os.WriteFile(absPath, content, 0o644); err != nil {
			t.Fatalf("write file failed: %v", err)
		}
		expected += int64(len(content))
	}

	size, err := CalculatePathSizeBytes(root)
	if err != nil {
		t.Fatalf("CalculatePathSizeBytes failed: %v", err)
	}
	if size != expected {
		t.Fatalf("expected recursive size %d, got %d", expected, size)
	}
}

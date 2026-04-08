package handlers

import (
	"lazymanga/models"
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

func TestMoveRepoISOPathWithFallbackMovesDirectory(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "incoming", "series")
	targetDir := filepath.Join(root, "library", "series")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source dir failed: %v", err)
	}
	pageAbs := filepath.Join(sourceDir, "page01.jpg")
	if err := os.WriteFile(pageAbs, []byte("img"), 0o644); err != nil {
		t.Fatalf("write source file failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		t.Fatalf("mkdir target parent failed: %v", err)
	}

	if err := moveRepoISOPathWithFallback(sourceDir, targetDir); err != nil {
		t.Fatalf("moveRepoISOPathWithFallback failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "page01.jpg")); err != nil {
		t.Fatalf("expected moved directory contents at target: %v", err)
	}
	if _, err := os.Stat(sourceDir); !os.IsNotExist(err) {
		t.Fatalf("expected source directory to be removed, got err=%v", err)
	}
}

func TestRestoreRepoISODirectoryToStoredSourcePathCreatesParentsAndRestoresName(t *testing.T) {
	root := t.TempDir()
	currentDir := filepath.Join(root, "normalized", "renamed-series")
	if err := os.MkdirAll(currentDir, 0o755); err != nil {
		t.Fatalf("mkdir current dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(currentDir, "page01.jpg"), []byte("img"), 0o644); err != nil {
		t.Fatalf("write current file failed: %v", err)
	}

	row := models.RepoISO{
		Path:        "normalized/renamed-series",
		FileName:    "renamed-series",
		IsDirectory: true,
		MetadataJSON: `{
			"source_path": "incoming/raw/series-original",
			"original_name": "series-original"
		}`,
	}

	finalAbs, finalRel, moved, err := restoreRepoISODirectoryToStoredSourcePath(root, &row)
	if err != nil {
		t.Fatalf("restoreRepoISODirectoryToStoredSourcePath failed: %v", err)
	}
	if !moved {
		t.Fatal("expected moved=true when restoring to stored source path")
	}
	if finalRel != "incoming/raw/series-original" {
		t.Fatalf("expected exact stored path, got %q", finalRel)
	}
	if row.Path != finalRel {
		t.Fatalf("expected row.Path to update to %q, got %q", finalRel, row.Path)
	}
	if row.FileName != "series-original" {
		t.Fatalf("expected row.FileName to restore original directory name, got %q", row.FileName)
	}
	if _, err := os.Stat(filepath.Join(finalAbs, "page01.jpg")); err != nil {
		t.Fatalf("expected restored file at original path: %v", err)
	}
	if _, err := os.Stat(currentDir); !os.IsNotExist(err) {
		t.Fatalf("expected renamed directory to be removed, got err=%v", err)
	}
}

func TestDecideManualEditTargetFileNamePreservesCurrentExtension(t *testing.T) {
	name, err := decideManualEditTargetFileName(manualEditRepoISORequest{
		NameMode:   "manual",
		ManualName: "new-title",
	}, "manga/old-title.cbz", "old-title.cbz")
	if err != nil {
		t.Fatalf("decideManualEditTargetFileName failed: %v", err)
	}
	if name != "new-title.cbz" {
		t.Fatalf("expected current extension to be appended, got %q", name)
	}

	_, err = decideManualEditTargetFileName(manualEditRepoISORequest{
		NameMode:   "manual",
		ManualName: "new-title.zip",
	}, "manga/old-title.cbz", "old-title.cbz")
	if err == nil {
		t.Fatal("expected mismatched extension to be rejected")
	}
}

func TestNormalizeManualEditRepoISORequestAllowsMetadataEditorWithoutTargetType(t *testing.T) {
	req := manualEditRepoISORequest{
		NameMode:   "manual",
		ManualName: "J's 2",
		Metadata: map[string]any{
			"title":           "  J's 2 ",
			"scanlator_group": " CE家族社 ",
			"author_alias":    "",
			"_internal":       "skip",
		},
	}

	if err := normalizeManualEditRepoISORequest(&req, manualEditorModeMetadata); err != nil {
		t.Fatalf("normalizeManualEditRepoISORequest failed: %v", err)
	}
	if req.TargetType != "" {
		t.Fatalf("expected metadata editor to allow empty target type, got %q", req.TargetType)
	}
	if got := req.Metadata["title"]; got != "J's 2" {
		t.Fatalf("expected title to be trimmed, got %#v", got)
	}
	if value, exists := req.Metadata["author_alias"]; !exists {
		t.Fatal("expected explicit empty metadata values to be preserved for field clearing")
	} else if value != "" {
		t.Fatalf("expected author_alias to remain an empty string, got %#v", value)
	}
	if _, exists := req.Metadata["_internal"]; exists {
		t.Fatal("expected internal metadata fields to be removed")
	}
}

func TestBuildManualEditMetadataJSONOmitsEmptyMap(t *testing.T) {
	encoded, err := buildManualEditMetadataJSON(nil)
	if err != nil {
		t.Fatalf("buildManualEditMetadataJSON returned error: %v", err)
	}
	if encoded != "" {
		t.Fatalf("expected empty metadata json for nil map, got %q", encoded)
	}
}

func TestSanitizeManualEditMetadataKeepsExplicitEmptyStringForFieldClear(t *testing.T) {
	metadata := sanitizeManualEditMetadata(map[string]any{
		"series_name": "",
		"title":       "Sample Title",
	})

	value, exists := metadata["series_name"]
	if !exists {
		t.Fatal("expected explicit empty string to be preserved so metadata fields can be cleared")
	}
	if value != "" {
		t.Fatalf("expected series_name to remain an empty string, got %#v", value)
	}
	if metadata["title"] != "Sample Title" {
		t.Fatalf("expected non-empty metadata to remain intact, got %#v", metadata)
	}
}

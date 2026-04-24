package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func defaultTestRepoImportSettings() repoTypeSettings {
	return repoTypeSettings{
		ArchiveSubdir:      defaultArchiveSubdir,
		MaterializedSubdir: defaultMaterializedSubdir,
		ArchiveExtensions:  defaultArchiveExtensionsCSV,
		ArchiveReadInnerLayout: true,
	}
}

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

	targetAbs, copied, err := importRepoDirectory(root, sourceAbs, defaultTestRepoImportSettings())
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

	targetAbs, copied, err := importRepoDirectory(root, sourceAbs, defaultTestRepoImportSettings())
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

func TestImportRepoFileCopiesArchiveIntoArchiveSubdir(t *testing.T) {
	root := t.TempDir()
	outsideBase := t.TempDir()
	sourceAbs := filepath.Join(outsideBase, "incoming", "seriesA.cbz")
	if err := os.MkdirAll(filepath.Dir(sourceAbs), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(sourceAbs, []byte("cbz"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	plan, err := importRepoFile(root, sourceAbs, 0o644, defaultTestRepoImportSettings())
	if err != nil {
		t.Fatalf("importRepoFile failed: %v", err)
	}
	if !plan.Copied {
		t.Fatal("expected archive import to copy external file into repo")
	}
	if plan.ItemKind != repoISOItemKindArchive {
		t.Fatalf("expected archive item kind, got %q", plan.ItemKind)
	}
	if filepath.Base(filepath.Dir(plan.TargetAbs)) != defaultArchiveSubdir {
		t.Fatalf("expected target under archive subdir, got %q", plan.TargetAbs)
	}
	if _, err := os.Stat(plan.TargetAbs); err != nil {
		t.Fatalf("expected copied archive to exist: %v", err)
	}
	content, err := os.ReadFile(plan.TargetAbs)
	if err != nil {
		t.Fatalf("read copied archive failed: %v", err)
	}
	if string(content) != "cbz" {
		t.Fatalf("unexpected copied content: %q", string(content))
	}
}

func TestImportRepoFileCopiesRegularFileIntoMaterializedManualDir(t *testing.T) {
	root := t.TempDir()
	outsideBase := t.TempDir()
	sourceAbs := filepath.Join(outsideBase, "incoming", "note.txt")
	if err := os.MkdirAll(filepath.Dir(sourceAbs), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(sourceAbs, []byte("note"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	settings := defaultTestRepoImportSettings()
	settings.MaterializedSubdir = "library"

	plan, err := importRepoFile(root, sourceAbs, 0o644, settings)
	if err != nil {
		t.Fatalf("importRepoFile failed: %v", err)
	}
	if !plan.Copied {
		t.Fatal("expected regular file import to copy external file into repo")
	}
	if plan.ItemKind != "" {
		t.Fatalf("did not expect archive item kind for regular file, got %q", plan.ItemKind)
	}
	if filepath.Base(filepath.Dir(plan.TargetAbs)) != repoManualAddedFilesSubdir {
		t.Fatalf("expected manual added leaf dir, got %q", plan.TargetAbs)
	}
	if filepath.Base(filepath.Dir(filepath.Dir(plan.TargetAbs))) != "library" {
		t.Fatalf("expected target under materialized subdir, got %q", plan.TargetAbs)
	}
}

func TestBuildImportedFileMetadataJSONForArchive(t *testing.T) {
	raw, err := buildImportedFileMetadataJSON(repoISOItemKindArchive, "archives/series/vol1.cbz", "vol1.cbz", "incoming/batch-01/vol1.cbz")
	if err != nil {
		t.Fatalf("buildImportedFileMetadataJSON failed: %v", err)
	}
	if raw == "" {
		t.Fatal("expected archive metadata json to be populated")
	}
	if !strings.Contains(raw, `"item_kind":"archive"`) {
		t.Fatalf("expected archive item kind in metadata, got %q", raw)
	}
	if !strings.Contains(raw, `"archive_storage_path":"archives/series/vol1.cbz"`) {
		t.Fatalf("expected archive storage path in metadata, got %q", raw)
	}
	if !strings.Contains(raw, `"source_path":"incoming/batch-01/vol1.cbz"`) {
		t.Fatalf("expected source path in metadata, got %q", raw)
	}
	if !strings.Contains(raw, `"original_name":"vol1.cbz"`) {
		t.Fatalf("expected original name in metadata, got %q", raw)
	}
}

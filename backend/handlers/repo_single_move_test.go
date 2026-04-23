package handlers

import (
	"path/filepath"
	"strings"
	"testing"

	"lazymanga/models"
)

func TestPlanRepoTransferRoutesArchiveIntoArchiveSubdir(t *testing.T) {
	root := t.TempDir()
	sourceAbs := filepath.Join(t.TempDir(), "incoming", "volume01.cbz")
	settings := repoTypeSettings{
		ArchiveSubdir:      defaultArchiveSubdir,
		MaterializedSubdir: defaultMaterializedSubdir,
		ArchiveExtensions:  defaultArchiveExtensionsCSV,
	}
	row := models.RepoISO{
		FileName:     "volume01.cbz",
		Path:         "manual_added/volume01.cbz",
		MetadataJSON: `{"item_kind":"archive","archive_storage_path":"manual_added/volume01.cbz"}`,
	}

	plan, err := planRepoTransfer(root, sourceAbs, row, settings)
	if err != nil {
		t.Fatalf("planRepoTransfer failed: %v", err)
	}
	if plan.ItemKind != repoISOItemKindArchive {
		t.Fatalf("expected archive item kind, got %q", plan.ItemKind)
	}
	if filepath.Base(filepath.Dir(plan.TargetAbs)) != defaultArchiveSubdir {
		t.Fatalf("expected target under archive subdir, got %q", plan.TargetAbs)
	}
	if plan.TargetSubdir != defaultArchiveSubdir {
		t.Fatalf("expected target subdir %q, got %q", defaultArchiveSubdir, plan.TargetSubdir)
	}
}

func TestPlanRepoTransferRoutesRegularFileIntoMaterializedManualDir(t *testing.T) {
	root := t.TempDir()
	sourceAbs := filepath.Join(t.TempDir(), "incoming", "cover.txt")
	settings := repoTypeSettings{
		ArchiveSubdir:      defaultArchiveSubdir,
		MaterializedSubdir: "library",
		ArchiveExtensions:  defaultArchiveExtensionsCSV,
	}
	row := models.RepoISO{
		FileName: "cover.txt",
		Path:     "old/cover.txt",
	}

	plan, err := planRepoTransfer(root, sourceAbs, row, settings)
	if err != nil {
		t.Fatalf("planRepoTransfer failed: %v", err)
	}
	if plan.ItemKind != "" {
		t.Fatalf("did not expect archive item kind, got %q", plan.ItemKind)
	}
	if filepath.Base(filepath.Dir(plan.TargetAbs)) != repoManualAddedFilesSubdir {
		t.Fatalf("expected regular file leaf dir %q, got %q", repoManualAddedFilesSubdir, plan.TargetAbs)
	}
	if filepath.Base(filepath.Dir(filepath.Dir(plan.TargetAbs))) != "library" {
		t.Fatalf("expected target under materialized subdir, got %q", plan.TargetAbs)
	}
}

func TestBuildImportedFileMetadataJSONFromExistingUpdatesArchiveStoragePath(t *testing.T) {
	raw, err := buildImportedFileMetadataJSONFromExisting(`{"item_kind":"archive","lifecycle":"managed","title":"Vol 1","source_path":"incoming/raw/volume01.cbz","original_name":"volume01.cbz"}`, repoISOItemKindArchive, "archives/series/volume01.cbz", "volume01.cbz", "")
	if err != nil {
		t.Fatalf("buildImportedFileMetadataJSONFromExisting failed: %v", err)
	}
	if !strings.Contains(raw, `"archive_storage_path":"archives/series/volume01.cbz"`) {
		t.Fatalf("expected updated archive storage path, got %q", raw)
	}
	if !strings.Contains(raw, `"lifecycle":"managed"`) {
		t.Fatalf("expected existing lifecycle to be preserved, got %q", raw)
	}
	if !strings.Contains(raw, `"title":"Vol 1"`) {
		t.Fatalf("expected existing metadata fields to be preserved, got %q", raw)
	}
	if !strings.Contains(raw, `"source_path":"incoming/raw/volume01.cbz"`) {
		t.Fatalf("expected existing source path to be preserved, got %q", raw)
	}
}

func TestBuildImportedFileMetadataJSONFromExistingBackfillsSourcePathFromSourceRowPath(t *testing.T) {
	raw, err := buildImportedFileMetadataJSONFromExisting(`{"item_kind":"archive","lifecycle":"managed","title":"Vol 1"}`, repoISOItemKindArchive, "archives/series/volume01.cbz", "volume01.cbz", "base-manga/系列A/volume01.cbz")
	if err != nil {
		t.Fatalf("buildImportedFileMetadataJSONFromExisting failed: %v", err)
	}
	if !strings.Contains(raw, `"source_path":"base-manga/系列A/volume01.cbz"`) {
		t.Fatalf("expected source path to be backfilled from source row path, got %q", raw)
	}
	if !strings.Contains(raw, `"original_name":"volume01.cbz"`) {
		t.Fatalf("expected original name to be backfilled from source row path, got %q", raw)
	}
}
package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lazymanga/models"
	"lazymanga/normalization/rulebook"
)

func TestCollectRepoISORecordsUsesMatcher(t *testing.T) {
	root := t.TempDir()
	files := []string{
		"series/chapter01.cbz",
		"series/chapter02.zip",
		"series/notes.txt",
	}
	for _, rel := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(abs, []byte(rel), 0o644); err != nil {
			t.Fatalf("write file failed: %v", err)
		}
	}

	records, err := collectRepoISORecords(root, func(name string) bool {
		lower := strings.ToLower(name)
		return strings.HasSuffix(lower, ".cbz") || strings.HasSuffix(lower, ".zip")
	})
	if err != nil {
		t.Fatalf("collectRepoISORecords failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 matched records, got %d: %#v", len(records), records)
	}
	if records[0].Path != "series/chapter01.cbz" {
		t.Fatalf("unexpected first path: %q", records[0].Path)
	}
	if records[1].Path != "series/chapter02.zip" {
		t.Fatalf("unexpected second path: %q", records[1].Path)
	}
}

func TestCollectRepoISORecordsByScanSpecTreatsImageFolderAsDirectoryUnit(t *testing.T) {
	root := t.TempDir()
	imageDir := filepath.Join(root, "manga", "Volume01")
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	for i := 1; i <= 12; i++ {
		fileAbs := filepath.Join(imageDir, "page"+formatPageNumber(i)+".jpg")
		if err := os.WriteFile(fileAbs, []byte("img"), 0o644); err != nil {
			t.Fatalf("write file failed: %v", err)
		}
	}
	zipAbs := filepath.Join(root, "manga", "Volume02.zip")
	if err := os.WriteFile(zipAbs, []byte("zip"), 0o644); err != nil {
		t.Fatalf("write zip failed: %v", err)
	}

	records, err := collectRepoISORecordsByScanSpec(root, rulebook.ScanSpec{
		Extensions: []string{".zip"},
		DirectoryRules: []rulebook.DirectoryScanRule{{
			Name:         "image-folder",
			Extensions:   []string{".jpg", ".jpeg", ".png"},
			MinFileCount: 5,
		}},
	}, "")
	if err != nil {
		t.Fatalf("collectRepoISORecordsByScanSpec failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d: %#v", len(records), records)
	}
	if records[0].Path != "manga/Volume01" || !records[0].IsDirectory {
		t.Fatalf("expected first record to be directory unit, got %#v", records[0])
	}
	if records[1].Path != "manga/Volume02.zip" || records[1].IsDirectory {
		t.Fatalf("expected second record to be zip file, got %#v", records[1])
	}
}

func TestCollectRepoISORecordsByScanSpecScopeOnlyIncludesSubtree(t *testing.T) {
	root := t.TempDir()
	paths := []string{
		"incoming/new/vol01.cbz",
		"incoming/new/vol02.cbz",
		"existing/old/archived.cbz",
	}
	for _, rel := range paths {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(abs, []byte(rel), 0o644); err != nil {
			t.Fatalf("write file failed: %v", err)
		}
	}

	records, err := collectRepoISORecordsByScanSpec(root, rulebook.ScanSpec{Extensions: []string{".cbz"}}, "incoming/new")
	if err != nil {
		t.Fatalf("collectRepoISORecordsByScanSpec scoped failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 scoped records, got %d: %#v", len(records), records)
	}
	for _, record := range records {
		if !strings.HasPrefix(record.Path, "incoming/new/") {
			t.Fatalf("unexpected out-of-scope path returned: %q", record.Path)
		}
	}
}

func TestShouldBlockFullRepoScanForBasicRepo(t *testing.T) {
	basicRepo := models.Repository{Basic: true}
	if !shouldBlockFullRepoScan(basicRepo, "") {
		t.Fatalf("expected full-repo scan to be blocked for basic repo")
	}
	if shouldBlockFullRepoScan(basicRepo, "manual_added_dirs/new-series") {
		t.Fatalf("expected scoped scan to remain allowed for basic repo")
	}

	normalRepo := models.Repository{Basic: false}
	if shouldBlockFullRepoScan(normalRepo, "") {
		t.Fatalf("did not expect full-repo scan to be blocked for normal repo")
	}
}

func TestInferRepoTypeKeyFromInfoDefaultsBasicRepoToManga(t *testing.T) {
	got := inferRepoTypeKeyFromInfo(models.RepoInfo{Basic: true}, models.Repository{Basic: true})
	if got != defaultRepoTypeKey {
		t.Fatalf("expected basic repo default type %q, got %q", defaultRepoTypeKey, got)
	}

	got = inferRepoTypeKeyFromInfo(models.RepoInfo{Basic: true, RepoTypeKey: repoTypeNone}, models.Repository{Basic: true, RepoTypeKey: repoTypeNone})
	if got != defaultRepoTypeKey {
		t.Fatalf("expected basic repo to migrate legacy type %q to %q, got %q", repoTypeNone, defaultRepoTypeKey, got)
	}
}

func TestBuildRepoInfoFromRepositoryDefaultsBasicRepoTypeToManga(t *testing.T) {
	info, err := buildRepoInfoFromRepository(models.Repository{Basic: true, Name: basicRepoName})
	if err != nil {
		t.Fatalf("buildRepoInfoFromRepository failed: %v", err)
	}
	if info.RepoTypeKey != defaultRepoTypeKey {
		t.Fatalf("expected basic repo info type %q, got %q", defaultRepoTypeKey, info.RepoTypeKey)
	}
}

func formatPageNumber(v int) string {
	return fmt.Sprintf("%02d", v)
}

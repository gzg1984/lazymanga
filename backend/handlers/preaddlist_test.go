package handlers

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"lazymanga/normalization/rulebook"
)

func TestShouldIncludePreAddEntryFallsBackToISOWithoutRepoScanSpec(t *testing.T) {
	file := fstest.MapFile{Data: []byte("x")}
	dir := fstest.MapFile{Mode: 0o755 | fs.ModeDir}
	fsys := fstest.MapFS{
		"chapter01.cbz": &file,
		"system.iso":    &file,
		"folder":        &dir,
	}

	entries, err := fsys.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir failed: %v", err)
	}

	var sawISO bool
	var sawCBZ bool
	var sawDir bool
	for _, entry := range entries {
		switch entry.Name() {
		case "system.iso":
			sawISO = shouldIncludePreAddEntry(entry, nil)
		case "chapter01.cbz":
			sawCBZ = shouldIncludePreAddEntry(entry, nil)
		case "folder":
			sawDir = shouldIncludePreAddEntry(entry, nil)
		}
	}
	if !sawISO {
		t.Fatal("expected .iso file to remain visible without repo scan spec")
	}
	if sawCBZ {
		t.Fatal("did not expect .cbz file without repo scan spec")
	}
	if !sawDir {
		t.Fatal("expected directories to remain visible")
	}
}

func TestShouldIncludePreAddEntryUsesRepoScanSpecForMangaFiles(t *testing.T) {
	book := rulebook.DefaultManualMangaRuleBook()
	effective := book.EffectiveScanSpec()
	file := fstest.MapFile{Data: []byte("x")}
	entries, err := fstest.MapFS{
		"chapter01.cbz": &file,
		"chapter02.zip": &file,
		"chapter03.rar": &file,
		"system.iso":    &file,
	}.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir failed: %v", err)
	}

	visible := map[string]bool{}
	for _, entry := range entries {
		visible[entry.Name()] = shouldIncludePreAddEntry(entry, &effective)
	}
	if !visible["chapter01.cbz"] || !visible["chapter02.zip"] || !visible["chapter03.rar"] {
		t.Fatal("expected manga scan spec to expose cbz/zip/rar files")
	}
	if visible["system.iso"] {
		t.Fatal("did not expect manga scan spec to expose .iso files")
	}
}
package handlers

import "testing"

func TestDefaultRepoTypeDefinitionsIncludeKaritaTemplate(t *testing.T) {
	defs := defaultRepoTypeDefinitions()
	for _, item := range defs {
		if item.Key != karitaRepoTypeKey {
			continue
		}
		if item.RuleBookName != "karita-manga" || item.RuleBookVersion != "v1" {
			t.Fatalf("expected karita template to bind karita-manga@v1, got %s@%s", item.RuleBookName, item.RuleBookVersion)
		}
		if item.AddDirectoryButton {
			t.Fatal("expected karita template to disable add-directory by default for non-basic manga repos")
		}
		if !item.AutoNormalize {
			t.Fatal("expected karita template to enable auto-normalize for directory transforms")
		}
		if item.AddButton {
			t.Fatal("did not expect file-only add button to be enabled by default")
		}
		if item.ManualEditorMode != manualEditorModeMetadata {
			t.Fatalf("expected karita template to default to metadata editor, got %q", item.ManualEditorMode)
		}
		if item.MetadataDisplayMode != metadataDisplayModeSelected {
			t.Fatalf("expected karita template metadata display mode to default to %q, got %q", metadataDisplayModeSelected, item.MetadataDisplayMode)
		}
		if item.MetadataDisplayFields == "" {
			t.Fatal("expected karita template metadata display fields to be preconfigured")
		}
		return
	}

	t.Fatalf("expected built-in repo type %q to exist", karitaRepoTypeKey)
}

func TestDefaultRepoTypeDefinitionsIncludeManualMangaTemplate(t *testing.T) {
	defs := defaultRepoTypeDefinitions()
	for _, item := range defs {
		if item.Key != manualMangaRepoTypeKey {
			continue
		}
		if item.RuleBookName != manualMangaRuleBookName || item.RuleBookVersion != "v1" {
			t.Fatalf("expected manual manga template to bind %s@v1, got %s@%s", manualMangaRuleBookName, item.RuleBookName, item.RuleBookVersion)
		}
		if !item.AddButton || !item.AddDirectoryButton {
			t.Fatal("expected manual manga template to allow both file and directory add operations")
		}
		if item.AutoNormalize {
			t.Fatal("did not expect manual manga template to enable auto-normalize")
		}
		if item.ManualEditorMode != manualEditorModeMetadata {
			t.Fatalf("expected manual manga template to default to metadata editor, got %q", item.ManualEditorMode)
		}
		if item.MetadataDisplayMode != metadataDisplayModeSelected {
			t.Fatalf("expected manual manga template metadata display mode to default to %q, got %q", metadataDisplayModeSelected, item.MetadataDisplayMode)
		}
		return
	}

	t.Fatalf("expected built-in repo type %q to exist", manualMangaRepoTypeKey)
}

func TestDefaultRepoTypeDefinitionsHideMetadataForNoneTemplate(t *testing.T) {
	defs := defaultRepoTypeDefinitions()
	for _, item := range defs {
		if item.Key != repoTypeNone {
			continue
		}
		if item.MetadataDisplayMode != metadataDisplayModeHidden {
			t.Fatalf("expected none template metadata display mode to default to %q, got %q", metadataDisplayModeHidden, item.MetadataDisplayMode)
		}
		if item.MetadataDisplayFields != "" {
			t.Fatalf("expected none template metadata display fields to stay empty, got %q", item.MetadataDisplayFields)
		}
		return
	}

	t.Fatalf("expected built-in repo type %q to exist", repoTypeNone)
}

func TestApplyRepoSettingsOverrideNormalizesMetadataDisplayConfig(t *testing.T) {
	base := repoTypeSettings{
		ManualEditorMode:     manualEditorModeMetadata,
		MetadataDisplayMode:  metadataDisplayModeSelected,
		MetadataDisplayFields: "title, series_name",
		RuleBookName:         manualMangaRuleBookName,
	}
	hidden := metadataDisplayModeHidden
	fields := " title ， title ; author_name\noriginal_work "
	override := repoTypeSettingsOverride{
		MetadataDisplayMode:   &hidden,
		MetadataDisplayFields: &fields,
	}
	result := applyRepoSettingsOverride(base, override)
	if result.MetadataDisplayMode != metadataDisplayModeHidden {
		t.Fatalf("expected metadata display mode %q, got %q", metadataDisplayModeHidden, result.MetadataDisplayMode)
	}
	if result.MetadataDisplayFields != "title,author_name,original_work" {
		t.Fatalf("unexpected canonical metadata fields: %q", result.MetadataDisplayFields)
	}
}

func TestDefaultRepoTypeDefinitionsIncludeArchiveDefaults(t *testing.T) {
	defs := defaultRepoTypeDefinitions()
	for _, item := range defs {
		if item.Key != defaultRepoTypeKey {
			continue
		}
		if item.ArchiveSubdir != defaultArchiveSubdir {
			t.Fatalf("expected archive_subdir %q, got %q", defaultArchiveSubdir, item.ArchiveSubdir)
		}
		if item.MaterializedSubdir != defaultMaterializedSubdir {
			t.Fatalf("expected materialized_subdir %q, got %q", defaultMaterializedSubdir, item.MaterializedSubdir)
		}
		if item.ArchiveExtensions != defaultArchiveExtensionsCSV {
			t.Fatalf("expected archive_extensions %q, got %q", defaultArchiveExtensionsCSV, item.ArchiveExtensions)
		}
		if !item.ArchiveReadInnerLayout {
			t.Fatal("expected archive_read_inner_layout to default to true")
		}
		return
	}

	t.Fatalf("expected built-in repo type %q to exist", defaultRepoTypeKey)
}

func TestApplyRepoSettingsOverrideNormalizesArchiveConfig(t *testing.T) {
	base := repoTypeSettings{
		ArchiveSubdir:      defaultArchiveSubdir,
		MaterializedSubdir: defaultMaterializedSubdir,
		ArchiveExtensions:  defaultArchiveExtensionsCSV,
		ArchiveReadInnerLayout: true,
	}
	archiveSubdir := "raw\\archives/"
	materializedSubdir := "library/"
	archiveExtensions := "ZIP, cbz ; rar\n.7z"
	innerLayout := false
	override := repoTypeSettingsOverride{
		ArchiveSubdir:      &archiveSubdir,
		MaterializedSubdir: &materializedSubdir,
		ArchiveExtensions:  &archiveExtensions,
		ArchiveReadInnerLayout: &innerLayout,
	}
	result := applyRepoSettingsOverride(base, override)
	if result.ArchiveSubdir != "raw/archives" {
		t.Fatalf("expected normalized archive_subdir, got %q", result.ArchiveSubdir)
	}
	if result.MaterializedSubdir != "library" {
		t.Fatalf("expected normalized materialized_subdir, got %q", result.MaterializedSubdir)
	}
	if result.ArchiveExtensions != ".zip,.cbz,.rar,.7z" {
		t.Fatalf("expected canonical archive_extensions, got %q", result.ArchiveExtensions)
	}
	if result.ArchiveReadInnerLayout {
		t.Fatal("expected archive_read_inner_layout override to be applied")
	}
}

func TestValidateArchiveOverrideRejectsOverlap(t *testing.T) {
	base := repoTypeSettings{
		ArchiveSubdir:      defaultArchiveSubdir,
		MaterializedSubdir: defaultMaterializedSubdir,
	}
	archiveSubdir := "library/archives"
	materializedSubdir := "library"
	override := repoTypeSettingsOverride{
		ArchiveSubdir:      &archiveSubdir,
		MaterializedSubdir: &materializedSubdir,
	}
	if err := validateArchiveOverride(base, override); err == nil {
		t.Fatal("expected overlapping archive/materialized paths to be rejected")
	}
}

func TestDefaultRepoTypeDefinitionsDisableAddDirectoryForMangaReposByDefault(t *testing.T) {
	defs := defaultRepoTypeDefinitions()
	for _, item := range defs {
		if item.Key != defaultRepoTypeKey {
			continue
		}
		if item.AddDirectoryButton {
			t.Fatal("expected manga template to disable add-directory by default for repos with their own root path")
		}
		return
	}

	t.Fatalf("expected built-in repo type %q to exist", defaultRepoTypeKey)
}

func TestDefaultRepoTypeDefinitionsHideOSRepoTypeByDefault(t *testing.T) {
	defs := defaultRepoTypeDefinitions()
	for _, item := range defs {
		if item.Key != repoTypeOS {
			continue
		}
		if item.Enabled {
			t.Fatal("expected OS repo type to be hidden by default until explicitly enabled")
		}
		return
	}

	t.Fatalf("expected built-in repo type %q to exist", repoTypeOS)
}

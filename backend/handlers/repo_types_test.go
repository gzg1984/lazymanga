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
		return
	}

	t.Fatalf("expected built-in repo type %q to exist", karitaRepoTypeKey)
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

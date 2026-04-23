package handlers

import (
	"testing"
)


func TestFilterPublicRepoTypesHidesMangaAndNoneFromPublicView(t *testing.T) {
	items := filterPublicRepoTypes(defaultRepoTypeDefinitions())
	if len(items) == 0 {
		t.Fatal("expected visible repo types to remain after filtering")
	}
	for _, item := range items {
		if item.Key == defaultRepoTypeKey || item.Key == repoTypeNone {
			t.Fatalf("expected hidden repo type %q to be omitted from public list", item.Key)
		}
	}
}

func TestNormalizePublicRepoTypeKeyForCreateDefaultsToVisibleType(t *testing.T) {
	got, err := normalizePublicRepoTypeKeyForCreate("")
	if err != nil {
		t.Fatalf("normalizePublicRepoTypeKeyForCreate returned error: %v", err)
	}
	if got != manualMangaRepoTypeKey {
		t.Fatalf("expected empty repo type to default to %q, got %q", manualMangaRepoTypeKey, got)
	}
}

func TestNormalizePublicRepoTypeKeyForCreateRejectsHiddenTypes(t *testing.T) {
	if _, err := normalizePublicRepoTypeKeyForCreate(defaultRepoTypeKey); err == nil {
		t.Fatalf("expected hidden repo type %q to be rejected for repository creation", defaultRepoTypeKey)
	}
	if _, err := normalizePublicRepoTypeKeyForCreate(repoTypeNone); err == nil {
		t.Fatalf("expected hidden repo type %q to be rejected for repository creation", repoTypeNone)
	}
	if _, err := normalizePublicRepoTypeKeyForCreate(manualMangaRepoTypeKey); err != nil {
		t.Fatalf("expected visible repo type %q to remain valid: %v", manualMangaRepoTypeKey, err)
	}
}
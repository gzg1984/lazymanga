package normalization

import (
	"strings"
	"testing"
)

func TestBuildDirectoryMetadataJSONUsesPayloadMetadata(t *testing.T) {
	payload := map[string]any{
		"metadata": map[string]string{
			"title":           "J's 2",
			"scanlator_group": "CE家族社",
			"author_name":     "牛乳のみお",
			"original_work":   "女子小学生はじめました",
			"empty_field":     "",
		},
	}

	raw, err := buildDirectoryMetadataJSON(payload)
	if err != nil {
		t.Fatalf("buildDirectoryMetadataJSON failed: %v", err)
	}
	if !strings.Contains(raw, `"scanlator_group":"CE家族社"`) {
		t.Fatalf("expected metadata json to contain scanlator_group, got %s", raw)
	}
	if strings.Contains(raw, `"empty_field"`) {
		t.Fatalf("did not expect empty metadata values to be persisted, got %s", raw)
	}
}

func TestNormalizeDirectoryMetadataMapKeepsExplicitEmptyStringForFieldClear(t *testing.T) {
	metadata := normalizeDirectoryMetadataMap(map[string]any{
		"series_name": "",
		"title":       "Sample Title",
	})

	value, exists := metadata["series_name"]
	if !exists {
		t.Fatal("expected explicit empty string to be preserved for metadata clearing")
	}
	if value != "" {
		t.Fatalf("expected series_name to remain an empty string, got %#v", value)
	}
	if metadata["title"] != "Sample Title" {
		t.Fatalf("expected non-empty metadata to remain intact, got %#v", metadata)
	}
}

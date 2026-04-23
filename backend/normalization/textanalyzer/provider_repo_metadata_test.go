package textanalyzer

import (
	"testing"

	"lazymanga/models"
)

func TestRepoMetadataHintProviderBuildsHintsFromRows(t *testing.T) {
	provider := NewRepoMetadataHintProvider()
	registry, err := provider.BuildHints(HintBuildContext{
		RepoID: 42,
		Rows: []models.RepoISO{
			{
				FileName:     "J's 2",
				IsDirectory:  true,
				MetadataJSON: `{"title":"J's 2","scanlator_group":"CE家族社","event_code":"C86","circle_name":"牛乳屋さん","original_work":"女子小学生はじめました","source_path":"漫画/【CE家族社】(C86) [牛乳屋さん (牛乳のみお)] J's 2 (女子小学生はじめました)","original_name":"【CE家族社】(C86) [牛乳屋さん (牛乳のみお)] J's 2 (女子小学生はじめました)"}`,
			},
			{
				FileName:     "J's 3",
				IsDirectory:  true,
				MetadataJSON: `{"title":"J's 3","scanlator_group":"CE家族社","event_code":"C87","circle_name":"牛乳屋さん","original_work":"女子小学生はじめました"}`,
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildHints returned error: %v", err)
	}
	assertFieldHasCanonicalValue(t, registry, "scanlator_group", "CE家族社")
	assertFieldHasCanonicalValue(t, registry, "event_code", "C86")
	assertFieldHasCanonicalValue(t, registry, "event_code", "C87")
	assertFieldHasCanonicalValue(t, registry, "comic_market", "C86")
	assertFieldHasCanonicalValue(t, registry, "circle_name", "牛乳屋さん")
	assertFieldHasCanonicalValue(t, registry, "author_alias", "牛乳屋さん")
	assertFieldHasCanonicalValue(t, registry, "original_work", "女子小学生はじめました")
	if _, ok := findFieldHint(registry, "title"); ok {
		t.Fatalf("did not expect title field to be exposed in repo metadata hints: %#v", registry)
	}
	if _, ok := findFieldHint(registry, "series_name"); ok {
		t.Fatalf("did not expect series_name field to be exposed in repo metadata hints: %#v", registry)
	}
}

func TestDefaultRegistryMergerMergesFieldValuesWithoutDuplicates(t *testing.T) {
	merger := DefaultRegistryMerger{}
	merged := merger.Merge(
		AnalysisHintRegistry{Fields: []AnalysisFieldHint{{
			Key:        "scanlator_group",
			Priority:   5,
			MultiValue: false,
			Values:     []AnalysisValueHint{{CanonicalValue: "CE家族社", Aliases: []string{"CE家族社"}, Weight: 1, Source: "left"}},
		}}},
		AnalysisHintRegistry{Fields: []AnalysisFieldHint{{
			Key:        "scanlator_group",
			Priority:   10,
			MultiValue: false,
			Values:     []AnalysisValueHint{{CanonicalValue: "CE家族社", Aliases: []string{"【CE家族社】"}, Weight: 3, Source: "right"}},
		}}},
	)
	field, ok := findFieldHint(merged, "scanlator_group")
	if !ok {
		t.Fatalf("expected merged scanlator_group field, got %#v", merged)
	}
	if field.Priority != 10 {
		t.Fatalf("expected merged priority 10, got %#v", field)
	}
	if len(field.Values) != 1 {
		t.Fatalf("expected deduplicated values, got %#v", field.Values)
	}
	if field.Values[0].Weight != 3 {
		t.Fatalf("expected highest weight to win, got %#v", field.Values[0])
	}
	if len(field.Values[0].Aliases) != 2 {
		t.Fatalf("expected aliases to merge, got %#v", field.Values[0])
	}
}

func findFieldHint(registry AnalysisHintRegistry, key string) (AnalysisFieldHint, bool) {
	for _, field := range registry.Fields {
		if field.Key == key {
			return field, true
		}
	}
	return AnalysisFieldHint{}, false
}

func assertFieldHasCanonicalValue(t *testing.T, registry AnalysisHintRegistry, fieldKey string, expectedValue string) {
	t.Helper()
	field, ok := findFieldHint(registry, fieldKey)
	if !ok {
		t.Fatalf("expected field %q in registry, got %#v", fieldKey, registry)
	}
	for _, value := range field.Values {
		if value.CanonicalValue == expectedValue {
			return
		}
	}
	t.Fatalf("expected field %q to contain value %q, got %#v", fieldKey, expectedValue, field.Values)
}

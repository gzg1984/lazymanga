package normalization

import (
	"testing"
	"time"

	"lazymanga/models"
)

func TestAnalyzePathMetadataUsesRepoModelForBrokenBrackets(t *testing.T) {
	samples := []models.RepoISO{
		{
			FileName:     "J's 2",
			Path:         "J's 2",
			IsDirectory:  true,
			MetadataJSON: `{"title":"J's 2","scanlator_group":"CE家族社","event_code":"C86","circle_name":"牛乳屋さん","author_alias":"牛乳屋さん","original_work":"女子小学生はじめました","source_path":"漫画/漫画zip/H/LittleStory/C71-C87/【CE家族社】(C86) [牛乳屋さん (牛乳のみお)] J's 2 (女子小学生はじめました)","original_name":"【CE家族社】(C86) [牛乳屋さん (牛乳のみお)] J's 2 (女子小学生はじめました)"}`,
		},
		{
			FileName:     "J's 3",
			Path:         "J's 3",
			IsDirectory:  true,
			MetadataJSON: `{"title":"J's 3","scanlator_group":"CE家族社","event_code":"C87","circle_name":"牛乳屋さん","author_alias":"牛乳屋さん","original_work":"女子小学生はじめました","source_path":"漫画/漫画zip/H/LittleStory/C71-C87/【CE家族社】(C87) [牛乳屋さん (牛乳のみお)] J's 3 (女子小学生はじめました)","original_name":"【CE家族社】(C87) [牛乳屋さん (牛乳のみお)] J's 3 (女子小学生はじめました)"}`,
		},
	}
	model := buildRepoPathAnalysisModelFromRows(42, samples, time.Now())
	guess := AnalyzePathMetadata(model, "漫画/待整理/【CE家族社】[(C86] [牛乳屋さん (牛乳のみお)] J's 4 (女子小学生はじめました)")
	if guess.Metadata["title"] != "J's 4" {
		t.Fatalf("expected guessed title from bare middle segment, got %#v", guess)
	}
	if guess.Metadata["scanlator_group"] != "CE家族社" || guess.Metadata["original_work"] != "女子小学生はじめました" {
		t.Fatalf("expected guessed group/work from repo model, got %#v", guess)
	}
	if guess.Metadata["event_code"] != "C86" {
		t.Fatalf("expected broken bracket event_code to be repaired and inferred, got %#v", guess)
	}
}

func TestAnalyzePathMetadataUsesSingleBareSegmentAsTitle(t *testing.T) {
	samples := []models.RepoISO{
		{
			FileName:     "きょうこの日々",
			Path:         "きょうこの日々",
			IsDirectory:  true,
			MetadataJSON: `{"title":"きょうこの日々","series_name":"きょうこの日々","scanlator_group":"CE家族社","circle_name":"とんずら道中","original_work":"東方Project","source_path":"漫画/【CE家族社】(C82) [とんずら道中] きょうこの日々 (東方Project)","original_name":"【CE家族社】(C82) [とんずら道中] きょうこの日々 (東方Project)"}`,
		},
	}
	model := buildRepoPathAnalysisModelFromRows(7, samples, time.Now())
	guess := AnalyzePathMetadata(model, "待整理/【CE家族社】 [とんずら道中] きょうこの日々 7日目! [(東方Project]")
	if guess.Metadata["title"] != "きょうこの日々 7日目!" {
		t.Fatalf("expected the only bare middle segment to be used as title, got %#v", guess)
	}
	if guess.Metadata["scanlator_group"] != "CE家族社" || guess.Metadata["circle_name"] != "とんずら道中" || guess.Metadata["original_work"] != "東方Project" {
		t.Fatalf("expected bracket tokens to map back to known metadata, got %#v", guess)
	}
}

func TestApplyMetadataQualityGuardsDropsUnrelatedSeriesName(t *testing.T) {
	metadata := map[string]string{
		"title":       "J's 2",
		"series_name": "漫画",
	}

	applyMetadataQualityGuards(metadata)
	if _, exists := metadata["series_name"]; exists {
		t.Fatalf("expected unrelated series_name to be dropped, got %#v", metadata)
	}
}

func TestApplyMetadataQualityGuardsKeepsRelatedSeriesName(t *testing.T) {
	metadata := map[string]string{
		"title":       "きょうこの日々 5日目!",
		"series_name": "きょうこの日々",
	}

	applyMetadataQualityGuards(metadata)
	if metadata["series_name"] != "きょうこの日々" {
		t.Fatalf("expected related series_name to remain, got %#v", metadata)
	}
}

func TestApplyMetadataQualityGuardsStripsFixedTitleNoiseTags(t *testing.T) {
	metadata := map[string]string{
		"title": "制服触手8 [中国翻訳] [DL版] [無修正]",
	}

	applyMetadataQualityGuards(metadata)
	if metadata["title"] != "制服触手8" {
		t.Fatalf("expected fixed noise tags to be stripped from title, got %#v", metadata)
	}
}

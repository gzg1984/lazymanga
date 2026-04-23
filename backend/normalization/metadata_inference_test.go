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
		"title": "制服触手8 [中国翻訳] [DL版] [無修正] [重新排列] [白碼、薄碼] [疏碼]",
	}

	applyMetadataQualityGuards(metadata)
	if metadata["title"] != "制服触手8" {
		t.Fatalf("expected fixed noise tags to be stripped from title, got %#v", metadata)
	}
}

func TestApplyMetadataQualityGuardsStripsRecognizedTrailingMetadataFromTitle(t *testing.T) {
	metadata := map[string]string{
		"title":        "まゆゆうの法則 (COMIC 阿吽 2016年9月号)",
		"comic_market": "COMIC 阿吽",
		"event_code":   "2016年9月号",
	}

	applyMetadataQualityGuards(metadata)
	if metadata["title"] != "まゆゆうの法則" {
		t.Fatalf("expected recognized trailing metadata to be stripped from title, got %#v", metadata)
	}
}

func TestApplyMetadataQualityGuardsStripsRecognizedOriginalWorkSuffixFromTitle(t *testing.T) {
	metadata := map[string]string{
		"title":         "大井さんのお茶 (艦隊これくしょん -艦これ-)",
		"original_work": "艦隊これくしょん -艦これ-",
	}

	applyMetadataQualityGuards(metadata)
	if metadata["title"] != "大井さんのお茶" {
		t.Fatalf("expected recognized original_work suffix to be stripped from title, got %#v", metadata)
	}
}

func TestAnalyzePathMetadataKeepsTitleWhenIgnoredPrefixLeavesOnlyPunctuation(t *testing.T) {
	model := &RepoPathAnalysisModel{
		IgnoredPrefixCounts: map[string]int{
			"えんこーせい": 3,
		},
		IgnoredPrefixRaw: map[string]string{
			"えんこーせい": "えんこーせい",
		},
	}

	guess := AnalyzePathMetadata(model, "えんこーせい! [中国翻訳]")
	if guess.Metadata["title"] != "えんこーせい!" {
		t.Fatalf("expected archive-like title to survive ignored prefix trimming, got %#v", guess)
	}
}

func TestAnalyzePathMetadataUsesSimpleBracketedSegmentAsTitleFallback(t *testing.T) {
	guess := AnalyzePathMetadata(nil, "[口袋妖怪][Royal Moon (白嶺白月, ねいちー)]")
	if guess.Metadata["title"] != "口袋妖怪" {
		t.Fatalf("expected first simple bracketed segment to be used as title fallback, got %#v", guess)
	}
}

func TestScoreTitleCandidatePrefersSeriesRelatedSuffixTitle(t *testing.T) {
	metadata := map[string]string{
		"series_name": "口袋妖怪",
	}

	related := scoreTitleCandidate(nil, "口袋妖怪 特别篇", metadata)
	unrelated := scoreTitleCandidate(nil, "特别篇", metadata)
	if related <= unrelated {
		t.Fatalf("expected series-related title candidate to score higher, got related=%d unrelated=%d", related, unrelated)
	}
}

func TestShouldIncludeFieldInAnalysisModel(t *testing.T) {
	if !ShouldIncludeFieldInAnalysisModel("title") {
		t.Fatal("expected title to be included in analysis model")
	}
	if ShouldIncludeFieldInAnalysisModel("source_path") {
		t.Fatal("expected source_path to be excluded from analysis model")
	}
	if ShouldIncludeFieldInAnalysisModel("archive_storage_path") {
		t.Fatal("expected archive_storage_path to be excluded from analysis model")
	}
}

func TestFieldSemanticContextAndTitleRoles(t *testing.T) {
	if !IsContextAnchorField("title") || !IsContextAnchorField("series_name") {
		t.Fatal("expected title and series_name to be context anchors")
	}
	if !IsTitleRelatedField("title") || !IsTitleRelatedField("series_name") {
		t.Fatal("expected title and series_name to be title-related fields")
	}
	if IsContextAnchorField("source_path") || IsTitleRelatedField("source_path") {
		t.Fatal("expected technical fields to be excluded from context/title roles")
	}
	anchors := ContextAnchorFields()
	if len(anchors) != 2 || anchors[0] != "series_name" || anchors[1] != "title" {
		t.Fatalf("unexpected context anchor fields: %#v", anchors)
	}
}

func TestApplyContextMetadataHintsUsesConfiguredContextAnchors(t *testing.T) {
	model := &RepoPathAnalysisModel{
		ContextValueCounts: map[string]map[string]map[string]int{
			"series_name:口袋妖怪": {
				"circle_name": {
					"皇家汉化组": 3,
				},
			},
		},
		CanonicalValues: map[string]string{
			"皇家汉化组": "皇家汉化组",
		},
	}
	metadata := map[string]string{
		"series_name": "口袋妖怪",
	}

	applyContextMetadataHints(model, metadata)
	if metadata["circle_name"] != "皇家汉化组" {
		t.Fatalf("expected context anchor to backfill circle_name, got %#v", metadata)
	}
}

func TestCanAutoApplyFieldValueDefaultsToFillEmptyOnly(t *testing.T) {
	if !CanAutoApplyFieldValue("title", "") {
		t.Fatal("expected empty title field to accept auto-applied value")
	}
	if CanAutoApplyFieldValue("title", "Manual Title") {
		t.Fatal("expected non-empty title field to reject auto-applied overwrite")
	}
	if CanAutoApplyFieldValue("item_kind", "archive") {
		t.Fatal("expected non-empty item_kind field to reject auto-applied overwrite")
	}
}

func TestShouldIncludeFieldInTextAnalyzerHints(t *testing.T) {
	if ShouldIncludeFieldInTextAnalyzerHints("title") {
		t.Fatal("expected title to be excluded from text analyzer hints")
	}
	if ShouldIncludeFieldInTextAnalyzerHints("source_path") {
		t.Fatal("expected source_path to be excluded from text analyzer hints")
	}
	if !ShouldIncludeFieldInTextAnalyzerHints("scanlator_group") {
		t.Fatal("expected scanlator_group to be included in text analyzer hints")
	}
}

func TestShouldIncludeFieldInProposalChanges(t *testing.T) {
	if !ShouldIncludeFieldInProposalChanges("title") {
		t.Fatal("expected title to be included in proposal changes")
	}
	if !ShouldIncludeFieldInProposalChanges("item_kind") {
		t.Fatal("expected item_kind to be included in proposal changes")
	}
	if ShouldIncludeFieldInProposalChanges("normalized_name") {
		t.Fatal("expected normalized_name to be excluded from proposal changes")
	}
}

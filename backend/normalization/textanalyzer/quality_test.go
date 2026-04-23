package textanalyzer

import "testing"

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

package textanalyzer

import "testing"

func TestSplitInferenceSegmentsKeepsBracketedSections(t *testing.T) {
	segments := splitInferenceSegments("【CE家族社】(C86) [牛乳屋さん (牛乳のみお)] J's 4 (女子小学生はじめました)")
	if len(segments) != 5 {
		t.Fatalf("expected 4 segments, got %#v", segments)
	}
	if !segments[0].Bracketed || segments[0].Text != "【CE家族社】" {
		t.Fatalf("unexpected first segment: %#v", segments[0])
	}
	if !segments[1].Bracketed || segments[1].Text != "(C86)" {
		t.Fatalf("unexpected second segment: %#v", segments[1])
	}
	if !segments[2].Bracketed || segments[2].Text != "[牛乳屋さん (牛乳のみお)]" {
		t.Fatalf("unexpected third segment: %#v", segments[2])
	}
	if segments[3].Bracketed || segments[3].Text != "J's 4" {
		t.Fatalf("unexpected fourth segment: %#v", segments[3])
	}
	if !segments[4].Bracketed || segments[4].Text != "(女子小学生はじめました)" {
		t.Fatalf("unexpected fifth segment: %#v", segments[4])
	}
}

func TestStripBracketedSegmentsReturnsOnlyBareText(t *testing.T) {
	got := stripBracketedSegments("【CE家族社】(C86) [牛乳屋さん (牛乳のみお)] J's 4")
	if got != "J's 4" {
		t.Fatalf("expected bare residual J's 4, got %q", got)
	}
}

func TestBestBareSegmentReturnsLongestNonBracketedPart(t *testing.T) {
	got := bestBareSegment("【CE家族社】 [とんずら道中] きょうこの日々 7日目! [(東方Project]")
	if got != "きょうこの日々 7日目!" {
		t.Fatalf("expected longest bare segment, got %q", got)
	}
}

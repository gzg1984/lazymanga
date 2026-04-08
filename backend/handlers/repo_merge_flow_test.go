package handlers

import (
	"testing"

	"lazymanga/models"
)

func TestCollectTransferNormalizationRowsReturnsMovedRowsWhenTargetAutoNormalize(t *testing.T) {
	rows := []models.RepoISO{
		{ID: 1, Path: "incoming/ubuntu.iso", FileName: "ubuntu.iso", IsOS: true},
		{ID: 2, Path: "incoming/series", FileName: "series", IsDirectory: true},
		{ID: 3, Path: "", FileName: "ignored-empty-path"},
	}

	got := collectTransferNormalizationRows(true, rows)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows queued for post-transfer normalization, got %d: %#v", len(got), got)
	}
	if got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("unexpected queued row ids: %#v", got)
	}
}

func TestCollectTransferNormalizationRowsSkipsWhenTargetAutoNormalizeDisabled(t *testing.T) {
	rows := []models.RepoISO{{ID: 1, Path: "incoming/ubuntu.iso", FileName: "ubuntu.iso", IsOS: true}}
	got := collectTransferNormalizationRows(false, rows)
	if len(got) != 0 {
		t.Fatalf("expected no rows queued when target auto-normalize is disabled, got %#v", got)
	}
}

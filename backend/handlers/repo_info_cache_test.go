package handlers

import (
	"testing"
	"time"
)

func TestShouldRefreshRepositoryMetadataAt(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	if !shouldRefreshRepositoryMetadataAt(time.Time{}, now, 15*time.Second, false) {
		t.Fatal("expected zero last-refresh time to force a refresh")
	}
	if shouldRefreshRepositoryMetadataAt(now.Add(-5*time.Second), now, 15*time.Second, false) {
		t.Fatal("did not expect refresh while still within throttle interval")
	}
	if !shouldRefreshRepositoryMetadataAt(now.Add(-16*time.Second), now, 15*time.Second, false) {
		t.Fatal("expected refresh after throttle interval elapsed")
	}
	if !shouldRefreshRepositoryMetadataAt(now.Add(-1*time.Second), now, 15*time.Second, true) {
		t.Fatal("expected forced refresh to bypass throttle interval")
	}
}

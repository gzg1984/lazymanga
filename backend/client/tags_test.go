package client

import (
	"testing"
)

func TestCallQueryAllTags(t *testing.T) {
	if err := CallQueryAllTags(); err != nil {
		t.Errorf("CallQueryAllTags failed: %v", err)
	}
}

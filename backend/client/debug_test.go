package client_test

import (
	"lazymanga/client"
	"testing"
)

func TestCallUpdateEmptyID(t *testing.T) {
	err := client.CallUpdateEmptyID()
	if err != nil {
		t.Errorf("CallUpdateEmptyID failed: %v", err)
	}
}

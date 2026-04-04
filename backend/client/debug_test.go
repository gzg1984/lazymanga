package client_test

import (
	"lazyiso/client"
	"testing"
)

func TestCallUpdateEmptyID(t *testing.T) {
	err := client.CallUpdateEmptyID()
	if err != nil {
		t.Errorf("CallUpdateEmptyID failed: %v", err)
	}
}

package client_test

import (
	"lazyiso/client"
	"testing"
)

func TestCallGetISOs(t *testing.T) {
	err := client.CallGetISOs()
	if err != nil {
		t.Errorf("CallGetISOs failed: %v", err)
	}
}

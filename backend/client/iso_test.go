package client_test

import (
	"lazymanga/client"
	"testing"
)

func TestCallGetISOs(t *testing.T) {
	err := client.CallGetISOs()
	if err != nil {
		t.Errorf("CallGetISOs failed: %v", err)
	}
}

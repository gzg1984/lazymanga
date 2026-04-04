package client_test

import (
	"lazyiso/client"
	"testing"
)

func TestCallGetUserInfo(t *testing.T) {
	err := client.CallGetUserInfo()
	if err != nil {
		t.Errorf("CallGetUserInfo failed: %v", err)
	}
}

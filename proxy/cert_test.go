package proxy

import (
	"fmt"
	"testing"
)

func TestGetStorePath(t *testing.T) {
	path, err := getStorePath("")
	if err != nil {
		t.Error(err)
	}
	if path == "" {
		t.Errorf("should have path")
	}
}

func TestNewCA(t *testing.T) {
	ca, err := NewCA("")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(ca)
}

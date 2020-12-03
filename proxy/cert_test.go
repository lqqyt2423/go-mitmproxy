package proxy

import (
	"os"
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

	err = ca.saveTo(os.Stdout)
	if err != nil {
		t.Error(err)
	}
}

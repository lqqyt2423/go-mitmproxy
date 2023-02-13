package main

import (
	"testing"
)

func TestMatchHost(t *testing.T) {
	address := "www.baidu.com:443"
	hosts := []string{
		"www.baidu.com:443",
		"www.baidu.com",
		"www.google.com",
	}
	expected := true
	result := matchHost(address, hosts)
	if result != expected {
		t.Errorf("Expected %t but got %t", expected, result)
	}

	address = "www.google.com:80"
	hosts = []string{
		"www.baidu.com:443",
		"www.baidu.com",
		"www.google.com",
	}
	expected = true
	result = matchHost(address, hosts)
	if result != expected {
		t.Errorf("Expected %t but got %t", expected, result)
	}

	address = "www.test.com:80"
	hosts = []string{
		"www.baidu.com:443",
		"www.baidu.com",
		"www.google.com",
	}
	expected = false
	result = matchHost(address, hosts)
	if result != expected {
		t.Errorf("Expected %t but got %t", expected, result)
	}
}

package helper

import "testing"

func TestMatchHost(t *testing.T) {
	// Test case 1: Exact match
	address := "www.baidu.com:443"
	hosts := []string{
		"www.baidu.com:443",
		"www.baidu.com",
		"www.google.com",
	}
	expected := true
	result := MatchHost(address, hosts)
	if result != expected {
		t.Errorf("Expected %t but got %t", expected, result)
	}

	// Test case 2: Exact match with port
	address = "www.google.com:80"
	hosts = []string{
		"www.baidu.com:443",
		"www.baidu.com",
		"www.google.com",
	}
	expected = true
	result = MatchHost(address, hosts)
	if result != expected {
		t.Errorf("Expected %t but got %t", expected, result)
	}

	// Test case 3: No match
	address = "www.test.com:80"
	hosts = []string{
		"www.baidu.com:443",
		"www.baidu.com",
		"www.google.com",
	}
	expected = false
	result = MatchHost(address, hosts)
	if result != expected {
		t.Errorf("Expected %t but got %t", expected, result)
	}

	// Test case 4: Wildcard match
	address = "test.baidu.com:443"
	hosts = []string{
		"*.baidu.com",
		"www.baidu.com:443",
		"www.baidu.com",
		"www.google.com",
	}
	expected = true
	result = MatchHost(address, hosts)
	if result != expected {
		t.Errorf("Expected %t but got %t", expected, result)
	}

	// Test case 5: Wildcard match with port
	address = "test.baidu.com:443"
	hosts = []string{
		"*.baidu.com:443",
		"www.baidu.com:443",
		"www.baidu.com",
		"www.google.com",
	}
	expected = true
	result = MatchHost(address, hosts)
	if result != expected {
		t.Errorf("Expected %t but got %t", expected, result)
	}

	// Test case 6: Wildcard mismatch
	address = "test.baidu.com:80"
	hosts = []string{
		"*.baidu.com:443",
		"www.baidu.com:443",
		"www.baidu.com",
		"www.google.com",
	}
	expected = false
	result = MatchHost(address, hosts)
	if result != expected {
		t.Errorf("Expected %t but got %t", expected, result)
	}

	// Test case 7: Wildcard mismatch
	address = "test.google.com:80"
	hosts = []string{
		"*.baidu.com",
		"www.baidu.com:443",
		"www.baidu.com",
		"www.google.com",
	}
	expected = false
	result = MatchHost(address, hosts)
	if result != expected {
		t.Errorf("Expected %t but got %t", expected, result)
	}
}

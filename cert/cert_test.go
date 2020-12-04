package cert

import (
	"bytes"
	"io/ioutil"
	"reflect"
	"testing"
)

func TestGetStorePath(t *testing.T) {
	path, err := getStorePath("")
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("should have path")
	}
}

func TestNewCA(t *testing.T) {
	ca, err := NewCA("")
	if err != nil {
		t.Fatal(err)
	}

	data := make([]byte, 0)
	buf := bytes.NewBuffer(data)

	err = ca.saveTo(buf)
	if err != nil {
		t.Fatal(err)
	}

	fileContent, err := ioutil.ReadFile(ca.caFile())
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(fileContent, buf.Bytes()) {
		t.Fatal("pem content should equal")
	}
}

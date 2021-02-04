package flow

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"strings"

	"github.com/andybalholm/brotli"
)

// handle http header: content-encoding

var EncodingNotSupport = errors.New("content-encoding not support")

var textContentTypes = []string{
	"text",
	"javascript",
	"json",
}

func (r *Response) IsTextContentType() bool {
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		return false
	}
	for _, substr := range textContentTypes {
		if strings.Contains(contentType, substr) {
			return true
		}
	}
	return false
}

func (r *Response) DecodedBody() ([]byte, error) {
	if r.decodedBody != nil {
		return r.decodedBody, nil
	}

	if r.decodedErr != nil {
		return nil, r.decodedErr
	}

	if r.Body == nil {
		return nil, nil
	}

	if len(r.Body) == 0 {
		r.decodedBody = r.Body
		return r.decodedBody, nil
	}

	enc := r.Header.Get("Content-Encoding")
	if enc == "" {
		r.decodedBody = r.Body
		return r.decodedBody, nil
	}

	decodedBody, decodedErr := Decode(enc, r.Body)
	if decodedErr != nil {
		r.decodedErr = decodedErr
		log.Error(r.decodedErr)
		return nil, decodedErr
	}

	r.decodedBody = decodedBody
	r.decoded = true
	return r.decodedBody, nil
}

// 当 Response.Body 替换为解压的内容时调用
func (r *Response) RemoveEncodingHeader() {
	r.Header.Del("Content-Encoding")
	r.Header.Del("Content-Length")
}

func Decode(enc string, body []byte) ([]byte, error) {
	if enc == "gzip" {
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		buf := bytes.NewBuffer(make([]byte, 0))
		_, err = io.Copy(buf, zr)
		if err != nil {
			return nil, err
		}
		err = zr.Close()
		if err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	} else if enc == "br" {
		brr := brotli.NewReader(bytes.NewReader(body))
		buf := bytes.NewBuffer(make([]byte, 0))
		_, err := io.Copy(buf, brr)
		if err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	return nil, EncodingNotSupport
}

package flow

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"io"
	"strconv"
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
	if enc == "" || enc == "identity" {
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

func (r *Response) ReplaceToDecodedBody() {
	body, err := r.DecodedBody()
	if err != nil || body == nil {
		return
	}

	r.Body = body
	r.Header.Del("Content-Encoding")
	r.Header.Set("Content-Length", strconv.Itoa(len(body)))
	r.Header.Del("Transfer-Encoding")
}

func Decode(enc string, body []byte) ([]byte, error) {
	if enc == "gzip" {
		dreader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		buf := bytes.NewBuffer(make([]byte, 0))
		_, err = io.Copy(buf, dreader)
		if err != nil {
			return nil, err
		}
		err = dreader.Close()
		if err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	} else if enc == "br" {
		dreader := brotli.NewReader(bytes.NewReader(body))
		buf := bytes.NewBuffer(make([]byte, 0))
		_, err := io.Copy(buf, dreader)
		if err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	} else if enc == "deflate" {
		dreader := flate.NewReader(bytes.NewReader(body))
		buf := bytes.NewBuffer(make([]byte, 0))
		_, err := io.Copy(buf, dreader)
		if err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	return nil, EncodingNotSupport
}

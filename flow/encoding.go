package flow

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
)

// handle http header: content-encoding

var EncodingNotSupport = errors.New("content-encoding not support")

func (r *Response) DecodedBody() ([]byte, bool) {
	if r.decodedBody != nil {
		return r.decodedBody, r.decoded
	}

	if r.decodedErr != nil {
		return nil, r.decoded
	}

	if r.Body == nil {
		return nil, r.decoded
	}

	if len(r.Body) == 0 {
		r.decodedBody = r.Body
		return r.decodedBody, r.decoded
	}

	enc := r.Header.Get("Content-Encoding")
	if enc == "" {
		r.decodedBody = r.Body
		return r.decodedBody, r.decoded
	}

	r.decodedBody, r.decodedErr = Decode(enc, r.Body)
	if r.decodedErr != nil {
		log.Error(r.decodedErr)
	} else {
		r.decoded = true
	}

	return r.decodedBody, r.decoded
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
	}

	return nil, EncodingNotSupport
}

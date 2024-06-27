package proxy

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	log "github.com/sirupsen/logrus"
)

var errEncodingNotSupport = errors.New("content-encoding not support")

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
	if len(r.Body) == 0 {
		return r.Body, nil
	}

	enc := r.Header.Get("Content-Encoding")
	if enc == "" || enc == "identity" {
		return r.Body, nil
	}

	decodedBody, decodedErr := decode(enc, r.Body)
	if decodedErr != nil {
		log.Error(decodedErr)
		return nil, decodedErr
	}

	return decodedBody, nil
}

func (r *Response) ReplaceToDecodedBody() {
	body, err := r.DecodedBody()
	if err != nil {
		return
	}

	r.Body = body
	r.Header.Del("Content-Encoding")
	r.Header.Set("Content-Length", strconv.Itoa(len(body)))
	r.Header.Del("Transfer-Encoding")
}

func decode(enc string, body []byte) ([]byte, error) {
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
	} else if enc == "zstd" {
		dreader, err := zstd.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		buf := bytes.NewBuffer(make([]byte, 0))
		_, err = io.Copy(buf, dreader)
		if err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	return nil, errEncodingNotSupport
}

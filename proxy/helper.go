package proxy

import (
	"bytes"
	"io"
)

// 尝试将 Reader 读取至 buffer 中
func ReaderToBuffer(r io.Reader, limit int64) ([]byte, io.Reader, error) {
	buf := bytes.NewBuffer(make([]byte, 0))
	lr := io.LimitReader(r, limit)

	_, err := io.Copy(buf, lr)
	if err != nil {
		return nil, nil, err
	}

	// 达到上限
	if int64(buf.Len()) == limit {
		// 返回新的 Reader
		return nil, io.MultiReader(bytes.NewBuffer(buf.Bytes()), r), nil
	}

	// 返回 buffer
	return buf.Bytes(), nil, nil
}

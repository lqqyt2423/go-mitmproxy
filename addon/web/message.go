package web

import (
	"bytes"
	"encoding/json"

	"github.com/lqqyt2423/go-mitmproxy/flow"
	uuid "github.com/satori/go.uuid"
)

const messageVersion = 1

const (
	messageTypeRequest      = 1
	messageTypeResponse     = 2
	messageTypeResponseBody = 3
)

type message struct {
	messageType int
	id          uuid.UUID
	content     []byte
}

func newMessage(messageType int, id uuid.UUID, content []byte) *message {
	return &message{
		messageType: messageType,
		id:          id,
		content:     content,
	}
}

func newMessageRequest(f *flow.Flow) *message {
	content, err := json.Marshal(f.Request)
	if err != nil {
		panic(err)
	}
	return newMessage(messageTypeRequest, f.Id, content)
}

func newMessageResponse(f *flow.Flow) *message {
	content, err := json.Marshal(f.Response)
	if err != nil {
		panic(err)
	}
	return newMessage(messageTypeResponse, f.Id, content)
}

func newMessageResponseBody(f *flow.Flow) *message {
	return newMessage(messageTypeResponseBody, f.Id, f.Response.Body)
}

func (m *message) bytes() []byte {
	buf := bytes.NewBuffer(make([]byte, 0))
	buf.WriteByte(byte(messageVersion))
	buf.WriteByte(byte(m.messageType))
	buf.WriteString(m.id.String()) // len: 36
	buf.Write(m.content)
	return buf.Bytes()
}

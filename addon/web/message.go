package web

import (
	"bytes"
	"encoding/json"

	"github.com/lqqyt2423/go-mitmproxy/flow"
	uuid "github.com/satori/go.uuid"
)

const messageVersion = 1

type messageType int

const (
	messageTypeRequest      messageType = 1
	messageTypeResponse     messageType = 2
	messageTypeResponseBody messageType = 3

	messageTypeChangeRequest      messageType = 11
	messageTypeChangeInterceptUri messageType = 21
)

func validMessageType(t byte) bool {
	if t == byte(messageTypeRequest) || t == byte(messageTypeResponse) || t == byte(messageTypeResponseBody) || t == byte(messageTypeChangeRequest) || t == byte(messageTypeChangeInterceptUri) {
		return true
	}
	return false
}

type message struct {
	mType         messageType
	id            uuid.UUID
	waitIntercept byte
	content       []byte
}

func newMessage(mType messageType, id uuid.UUID, content []byte) *message {
	return &message{
		mType:   mType,
		id:      id,
		content: content,
	}
}

func parseMessage(data []byte) *message {
	if len(data) < 39 {
		return nil
	}
	if data[0] != messageVersion {
		return nil
	}
	if !validMessageType(data[1]) {
		return nil
	}

	id, err := uuid.FromString(string(data[3:39]))
	if err != nil {
		return nil
	}

	msg := newMessage(messageType(data[1]), id, data[39:])
	msg.waitIntercept = data[2]
	return msg
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
	body, _ := f.Response.DecodedBody()
	return newMessage(messageTypeResponseBody, f.Id, body)
}

func (m *message) bytes() []byte {
	buf := bytes.NewBuffer(make([]byte, 0))
	buf.WriteByte(byte(messageVersion))
	buf.WriteByte(byte(m.mType))
	buf.WriteByte(m.waitIntercept)
	buf.WriteString(m.id.String()) // len: 36
	buf.Write(m.content)
	return buf.Bytes()
}

package web

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

// message:

// type: 0/1/2/3/4/5
// messageFlow
// version 1 byte + type 1 byte + id 36 byte + waitIntercept 1 byte + content left bytes

// type: 11/12/13/14
// messageEdit
// version 1 byte + type 1 byte + id 36 byte + header len 4 byte + header content bytes + body len 4 byte + [body content bytes]

// type: 21
// messageMeta
// version 1 byte + type 1 byte + content left bytes

const messageVersion = 2

type messageType byte

const (
	messageTypeConn         messageType = 0
	messageTypeConnClose    messageType = 5
	messageTypeRequest      messageType = 1
	messageTypeRequestBody  messageType = 2
	messageTypeResponse     messageType = 3
	messageTypeResponseBody messageType = 4

	messageTypeChangeRequest  messageType = 11
	messageTypeChangeResponse messageType = 12
	messageTypeDropRequest    messageType = 13
	messageTypeDropResponse   messageType = 14

	messageTypeChangeBreakPointRules messageType = 21
)

var allMessageTypes = []messageType{
	messageTypeConn,
	messageTypeConnClose,
	messageTypeRequest,
	messageTypeRequestBody,
	messageTypeResponse,
	messageTypeResponseBody,
	messageTypeChangeRequest,
	messageTypeChangeResponse,
	messageTypeDropRequest,
	messageTypeDropResponse,
	messageTypeChangeBreakPointRules,
}

func validMessageType(t byte) bool {
	for _, v := range allMessageTypes {
		if t == byte(v) {
			return true
		}
	}
	return false
}

type message interface {
	bytes() []byte
}

type messageFlow struct {
	mType         messageType
	id            uuid.UUID
	waitIntercept byte
	content       []byte
}

func newMessageFlow(mType messageType, f *proxy.Flow) (*messageFlow, error) {
	var content []byte
	var err error
	id := f.Id

	switch mType {
	case messageTypeConn:
		id = f.ConnContext.Id()
		content, err = json.Marshal(f.ConnContext)
	case messageTypeRequest:
		m := make(map[string]interface{})
		m["request"] = f.Request
		m["connId"] = f.ConnContext.Id().String()
		content, err = json.Marshal(m)
	case messageTypeRequestBody:
		content = f.Request.Body
	case messageTypeResponse:
		if f.Response == nil {
			err = errors.New("no response")
			break
		}
		content, err = json.Marshal(f.Response)
	case messageTypeResponseBody:
		if f.Response == nil {
			err = errors.New("no response")
			break
		}
		content, err = f.Response.DecodedBody()
	default:
		err = errors.New("invalid message type")
	}

	if err != nil {
		return nil, err
	}

	return &messageFlow{
		mType:   mType,
		id:      id,
		content: content,
	}, nil
}

func newMessageConnClose(connCtx *proxy.ConnContext) *messageFlow {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, connCtx.FlowCount.Load())
	return &messageFlow{
		mType:   messageTypeConnClose,
		id:      connCtx.Id(),
		content: buf.Bytes(),
	}
}

func (m *messageFlow) bytes() []byte {
	buf := bytes.NewBuffer(make([]byte, 0))
	buf.WriteByte(byte(messageVersion))
	buf.WriteByte(byte(m.mType))
	buf.WriteString(m.id.String()) // len: 36
	buf.WriteByte(m.waitIntercept)
	buf.Write(m.content)
	return buf.Bytes()
}

type messageEdit struct {
	mType    messageType
	id       uuid.UUID
	request  *proxy.Request
	response *proxy.Response
}

func parseMessageEdit(data []byte) *messageEdit {
	// 2 + 36
	if len(data) < 38 {
		return nil
	}

	mType := (messageType)(data[1])

	id, err := uuid.FromString(string(data[2:38]))
	if err != nil {
		return nil
	}

	msg := &messageEdit{
		mType: mType,
		id:    id,
	}

	if mType == messageTypeDropRequest || mType == messageTypeDropResponse {
		return msg
	}

	// 2 + 36 + 4 + 4
	if len(data) < 46 {
		return nil
	}

	hl := (int)(binary.BigEndian.Uint32(data[38:42]))
	if 42+hl+4 > len(data) {
		return nil
	}
	headerContent := data[42 : 42+hl]

	bl := (int)(binary.BigEndian.Uint32(data[42+hl : 42+hl+4]))
	if 42+hl+4+bl != len(data) {
		return nil
	}
	bodyContent := data[42+hl+4:]

	if mType == messageTypeChangeRequest {
		req := new(proxy.Request)
		err := json.Unmarshal(headerContent, req)
		if err != nil {
			return nil
		}
		req.Body = bodyContent
		msg.request = req
	} else if mType == messageTypeChangeResponse {
		res := new(proxy.Response)
		err := json.Unmarshal(headerContent, res)
		if err != nil {
			return nil
		}
		res.Body = bodyContent
		msg.response = res
	} else {
		return nil
	}

	return msg
}

func (m *messageEdit) bytes() []byte {
	buf := bytes.NewBuffer(make([]byte, 0))
	buf.WriteByte(byte(messageVersion))
	buf.WriteByte(byte(m.mType))
	buf.WriteString(m.id.String()) // len: 36

	if m.mType == messageTypeChangeRequest {
		headerContent, err := json.Marshal(m.request)
		if err != nil {
			panic(err)
		}
		hl := make([]byte, 4)
		binary.BigEndian.PutUint32(hl, (uint32)(len(headerContent)))
		buf.Write(hl)

		bodyContent := m.request.Body
		bl := make([]byte, 4)
		binary.BigEndian.PutUint32(bl, (uint32)(len(bodyContent)))
		buf.Write(bl)
		buf.Write(bodyContent)
	} else if m.mType == messageTypeChangeResponse {
		headerContent, err := json.Marshal(m.response)
		if err != nil {
			panic(err)
		}
		hl := make([]byte, 4)
		binary.BigEndian.PutUint32(hl, (uint32)(len(headerContent)))
		buf.Write(hl)

		bodyContent := m.response.Body
		bl := make([]byte, 4)
		binary.BigEndian.PutUint32(bl, (uint32)(len(bodyContent)))
		buf.Write(bl)
		buf.Write(bodyContent)
	}

	return buf.Bytes()
}

type messageMeta struct {
	mType           messageType
	breakPointRules []*breakPointRule
}

func parseMessageMeta(data []byte) *messageMeta {
	content := data[2:]
	rules := make([]*breakPointRule, 0)
	err := json.Unmarshal(content, &rules)
	if err != nil {
		return nil
	}

	return &messageMeta{
		mType:           messageType(data[1]),
		breakPointRules: rules,
	}
}

func (m *messageMeta) bytes() []byte {
	buf := bytes.NewBuffer(make([]byte, 0))
	buf.WriteByte(byte(messageVersion))
	buf.WriteByte(byte(m.mType))

	content, err := json.Marshal(m.breakPointRules)
	if err != nil {
		panic(err)
	}
	buf.Write(content)

	return buf.Bytes()
}

func parseMessage(data []byte) message {
	if len(data) < 2 {
		return nil
	}

	if data[0] != messageVersion {
		return nil
	}

	if !validMessageType(data[1]) {
		return nil
	}

	mType := (messageType)(data[1])

	if mType == messageTypeChangeRequest || mType == messageTypeChangeResponse || mType == messageTypeDropRequest || mType == messageTypeDropResponse {
		return parseMessageEdit(data)
	} else if mType == messageTypeChangeBreakPointRules {
		return parseMessageMeta(data)
	} else {
		log.Warnf("invalid message type %v", mType)
		return nil
	}
}

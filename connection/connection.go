package connection

import (
	"net"

	uuid "github.com/satori/go.uuid"
)

type Client struct {
	Id   uuid.UUID
	Conn net.Conn
}

func NewClient(c net.Conn) *Client {
	return &Client{
		Id:   uuid.NewV4(),
		Conn: c,
	}
}

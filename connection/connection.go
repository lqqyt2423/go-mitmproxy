package connection

import (
	"net"
	"net/http"

	uuid "github.com/satori/go.uuid"
)

type Client struct {
	Id   uuid.UUID
	Conn net.Conn
	Tls  bool
}

func NewClient(c net.Conn) *Client {
	return &Client{
		Id:   uuid.NewV4(),
		Conn: c,
		Tls:  false,
	}
}

type Server struct {
	Id     uuid.UUID
	Client *http.Client
}

func NewServer() *Server {
	return &Server{
		Id: uuid.NewV4(),
	}
}

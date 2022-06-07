package flow

import "github.com/lqqyt2423/go-mitmproxy/connection"

type ConnContext struct {
	Client *connection.Client
}

var ConnContextKey = new(struct{})

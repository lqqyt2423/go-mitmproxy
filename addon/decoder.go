package addon

import "github.com/lqqyt2423/go-mitmproxy/flow"

// decode content-encoding then respond to client

type Decoder struct {
	Base
}

func (d *Decoder) Response(f *flow.Flow) {
	f.Response.ReplaceToDecodedBody()
}

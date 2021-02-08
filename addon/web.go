package addon

import (
	"net/http"
	"text/template"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

func (web *WebAddon) echo(w http.ResponseWriter, r *http.Request) {
	c, err := web.upgrader.Upgrade(w, r, nil)
	if err != nil {
		web.log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			web.log.Println("read:", err)
			break
		}
		web.log.Printf("recv: %s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			web.log.Println("write:", err)
			break
		}
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	homeTemplate.Execute(w, "ws://"+r.Host+"/echo")
}

type WebAddon struct {
	Base
	addr      string
	upgrader  *websocket.Upgrader
	serverMux *http.ServeMux
	server    *http.Server
	log       *logrus.Entry
}

func NewWebAddon() *WebAddon {
	web := new(WebAddon)
	web.addr = ":9081"
	web.upgrader = &websocket.Upgrader{}

	web.serverMux = new(http.ServeMux)
	web.serverMux.HandleFunc("/echo", web.echo)
	web.serverMux.HandleFunc("/", home)

	web.server = &http.Server{Addr: web.addr, Handler: web.serverMux}

	web.log = log.WithField("in", "WebAddon")

	go func() {
		web.log.Infof("server start listen at %v\n", web.addr)
		err := web.server.ListenAndServe()
		web.log.Error(err)
	}()

	return web
}

var homeTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<script>  
window.addEventListener("load", function(evt) {

    var output = document.getElementById("output");
    var input = document.getElementById("input");
    var ws;

    var print = function(message) {
        var d = document.createElement("div");
        d.textContent = message;
        output.appendChild(d);
    };

    document.getElementById("open").onclick = function(evt) {
        if (ws) {
            return false;
        }
        ws = new WebSocket("{{.}}");
        ws.onopen = function(evt) {
            print("OPEN");
        }
        ws.onclose = function(evt) {
            print("CLOSE");
            ws = null;
        }
        ws.onmessage = function(evt) {
            print("RESPONSE: " + evt.data);
        }
        ws.onerror = function(evt) {
            print("ERROR: " + evt.data);
        }
        return false;
    };

    document.getElementById("send").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        print("SEND: " + input.value);
        ws.send(input.value);
        return false;
    };

    document.getElementById("close").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        ws.close();
        return false;
    };

});
</script>
</head>
<body>
<table>
<tr><td valign="top" width="50%">
<p>Click "Open" to create a connection to the server, 
"Send" to send a message to the server and "Close" to close the connection. 
You can change the message and send multiple times.
<p>
<form>
<button id="open">Open</button>
<button id="close">Close</button>
<p><input id="input" type="text" value="Hello world!">
<button id="send">Send</button>
</form>
</td><td valign="top" width="50%">
<div id="output"></div>
</td></tr></table>
</body>
</html>
`))

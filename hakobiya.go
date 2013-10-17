package main

import (
	"code.google.com/p/go.net/websocket"
	"io/ioutil"
	"log"
	"net/http"
)

var serverConf serverConfig

func main() {
	conf := parseConfig("config.toml")
	serverConf = conf.Server

	// start http services
	log.Printf("Starting Hakobiya %s @ %s%s \n", serverConf.Name, serverConf.Bind, serverConf.Path)
	http.HandleFunc("/", index)
	http.Handle(serverConf.Path, websocket.Handler(serveWS))
	http.ListenAndServe(serverConf.Bind, nil)
}

func index(w http.ResponseWriter, r *http.Request) {
	f, err := ioutil.ReadFile("test.html")
	if err == nil {
		w.Write(f)
	}
}

func serveWS(ws *websocket.Conn) {
	c := &client{
		socket:    ws,
		listening: make(map[string]*channel),

		sendq: make(chan interface{}),
	}
	go c.writer()
	c.run()
}

func Error(wrt, msg string) *errorMessage {
	return &errorMessage{
		Cmd:     "!",
		ReplyTo: wrt,
		Message: msg,
	}
}

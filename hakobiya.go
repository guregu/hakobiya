package main

import (
	"code.google.com/p/go.net/websocket"
	"io/ioutil"
	"log"
	"net/http"
)

var serverConf serverConfig

func main() {
	// load config
	conf := parseConfig("config.toml")
	serverConf = conf.Server
	for _, ccfg := range conf.Channels {
		log.Printf("Channel: %s", ccfg.Prefix)
		channelConfigs[ccfg.Prefix[0]] = ccfg
	}

	// start http services
	log.Printf("Starting Hakobiya %s @ %s%s \n", serverConf.Name, serverConf.Bind, serverConf.Path)
	http.HandleFunc("/", index)
	http.Handle(serverConf.Path, websocket.Handler(serveWS))
	// for testing purposes
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("js"))))
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

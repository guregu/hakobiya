package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"

	"code.google.com/p/go.net/websocket"
	"github.com/drone/routes"
)

var configFile = flag.String("config", "config.toml", "config file path")
var serverConf serverConfig

func main() {
	flag.Parse()

	// load config
	conf, ok := parseConfig(*configFile)
	if !ok {
		log.Println("Giving up.")
		return
	}

	serverConf = conf.Server
	for _, ccfg := range conf.Channels {
		log.Printf("Channel: %s", ccfg.Prefix)
		channelConfigs[ccfg.Prefix[0]] = ccfg
	}
	// start http services
	log.Printf("Starting Hakobiya %s @ %s%s \n", serverConf.Name, serverConf.Bind, serverConf.Path)
	http.Handle(serverConf.Path, websocket.Handler(serveWS))
	http.HandleFunc("/", index)
	// api
	if conf.API.Enabled {
		apiKey = conf.API.Key
		apiPath := conf.API.Path
		if apiPath == "" {
			// the default api path is /api/
			apiPath = "/api"
		}
		mux := routes.New()
		mux.Post(apiPath+"/:channel/broadcast", apiBroadcast)
		mux.Get(apiPath+"/:channel/debug", apiDebug)
		mux.Filter(apiKeyFilter)
		http.Handle(apiPath+"/", mux)
		log.Printf("API open at %s/", apiPath)
	}
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

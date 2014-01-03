package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"unicode/utf8"

	"code.google.com/p/go.net/websocket"
)

var configFile = flag.String("config", "config.toml", "config file path")
var currentConfig config
var templates = make(map[rune]channelTemplate)

func main() {
	flag.Parse()

	// load config
	cfg, ok := parseConfig(*configFile)
	if !ok {
		log.Println("Bad config file, giving up.")
		return
	}
	currentConfig = cfg
	channelBanner := ""
	for _, tmpl := range cfg.Channels {
		channelBanner += tmpl.Prefix
		prefix, _ := utf8.DecodeRuneInString(tmpl.Prefix)
		templates[prefix] = tmpl
	}

	// start http services
	log.Printf("Hakobiya: Starting %s @ %s%s", cfg.Server.Name, cfg.Server.Bind, cfg.Server.Path)
	log.Printf("Channels (%d): %s", len(templates), channelBanner)
	http.Handle(cfg.Server.Path, websocket.Handler(serveWS))
	// api
	if cfg.API.Enabled {
		apiPath := cfg.API.Path
		http.Handle(apiPath+"/", apiHandler(cfg.API))
		log.Printf("API: enabled at %s/", apiPath)
	} else {
		log.Printf("API: off")
	}
	// static server
	if cfg.Static.Enabled {
		log.Printf("Static content server: enabled")
		if cfg.Static.Index == "" {
			http.HandleFunc("/", http.NotFound)
		} else {
			http.HandleFunc("/", index)
			log.Printf("Serving: / → %s", cfg.Static.Index)
		}

		for _, dir := range cfg.Static.Dirs {
			path := "/" + filepath.Base(dir) + "/"
			http.Handle(path, http.StripPrefix(
				path, http.FileServer(http.Dir(dir))))
			log.Printf("Serving: %s → %s", path, dir)
		}
	}
	http.ListenAndServe(cfg.Server.Bind, nil)
}

func index(w http.ResponseWriter, r *http.Request) {
	f, err := ioutil.ReadFile(currentConfig.Static.Index)
	if err == nil {
		w.Write(f)
	}
}

func serveWS(ws *websocket.Conn) {
	c := newClient(ws)
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

package main

import (
	"log"
	"net/http"

	"github.com/drone/routes"
)

type apiRequest struct {
	Var       identifier             `json:"var"`
	Value     interface{}            `json:"value,omitempty"`
	For       string                 `json:"for,omitempty"`
	Key       string                 `json:"key,omitempty"`
	Overwrite map[string]interface{} `json:"overwrite,omitempty"`
}

type apiResponse struct {
	Code  responseCode `json:"code"`
	Msg   string       `json:"msg,omitempty"`
	Value interface{}  `json:"value,omitempty"`
}

type responseCode int

const (
	API_OK              responseCode = 1
	API_NothingHappened responseCode = 0
	API_Error           responseCode = -1
)

func apiHandler(cfg apiConfig) http.Handler {
	mux := routes.New()
	mux.Post(cfg.Path+"/:channel/set", apiSet)
	mux.Post(cfg.Path+"/:channel/fetch", apiFetch)
	return mux
}

func apiSet(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	name := params.Get(":channel")
	if !channelExists(name) {
		log.Printf("API: set to empty channel: %s", name)
		routes.ServeJson(w, apiResponse{API_NothingHappened, "no one's listening", name})
		return
	}
	ch := getChannel(name)
	req := apiRequest{}
	err := routes.ReadJson(r, &req)
	if err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if !checkKey(req) {
		http.Error(w, "bad key", http.StatusUnauthorized)
		return
	}

	msg := setter{
		Var:       req.Var,
		Value:     req.Value,
		Overwrite: req.Overwrite,
		//TODO: From
	}
	ch.set <- msg
	routes.ServeJson(w, apiResponse{API_OK, "", req.Var.String()})
}

func apiFetch(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	name := params.Get(":channel")
	if !channelExists(name) {
		log.Printf("API: get to empty channel: %s", name)
		routes.ServeJson(w, apiResponse{API_NothingHappened, "no one's there", name})
		return
	}
	ch := getChannel(name)
	req := apiRequest{}
	err := routes.ReadJson(r, &req)
	if err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if !checkKey(req) {
		http.Error(w, "bad key", http.StatusUnauthorized)
	} else {
		mailbox := make(chan goods)
		fetch := order{
			get: getter{
				Var: req.Var,
				//TODO: From
			},
			to: mailbox,
		}
		ch.deliver <- fetch
		g := <-mailbox
		if g.err == nil {
			routes.ServeJson(w, apiResponse{API_OK, "", g.value})
		} else {
			routes.ServeJson(w, apiResponse{API_Error, "couldn't get", g.err})
		}
	}
}

func checkKey(req apiRequest) bool {
	if currentConfig.API.Key == "" {
		return true
	}
	return currentConfig.API.Key == req.Key
}

package main

import (
	"log"
	"net/http"

	"github.com/drone/routes"
)

// TODO: make this more better/secure
var apiKey string

type broadcastRequest struct {
	To    string      `json:"to"`
	Value interface{} `json:"value"`
}

type apiResponse struct {
	Code responseCode `json:"code"`
	Msg  string       `json:"msg"`
}

type responseCode int

const (
	API_OK              responseCode = 1
	API_NothingHappened responseCode = 0
	API_Error           responseCode = -1
)

func apiBroadcast(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	name := params.Get(":channel")
	// TODO: check if channel is even valid in the first place?
	if !channelExists(name) {
		log.Printf("Broadcast to nowhere: %s", name)
		routes.ServeJson(w, apiResponse{API_NothingHappened, "no one's listening"})
		return
	}
	ch := getChannel(name)
	breq := broadcastRequest{}
	routes.ReadJson(r, &breq)
	msg := setter{
		Var:   breq.To,
		Value: breq.Value,
	}
	ch.set <- msg
	routes.ServeJson(w, apiResponse{API_OK, "sent"})
}

func apiDebug(w http.ResponseWriter, r *http.Request) {

}

func apiKeyFilter(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	if params.Get("key") != apiKey {
		http.Error(w, "", http.StatusUnauthorized)
	}
}

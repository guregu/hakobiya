package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"sync"

	"code.google.com/p/go.net/websocket"
)

var clientsTable = make(map[clientID]*client)
var clientsTableLock = &sync.RWMutex{}

type clientID string

type client struct {
	id        clientID
	socket    *websocket.Conn
	listening map[string]*channel

	sendq chan interface{}
}

func newClient(socket *websocket.Conn) *client {
	c := &client{
		id:        generateID(),
		socket:    socket,
		listening: make(map[string]*channel),

		sendq: make(chan interface{}),
	}
	addClient(c)
	return c
}

func (c *client) send(msg interface{}) {
	c.sendq <- msg
}

func (c *client) setID(id clientID) {
	renameClient(c.id, id)
	c.id = id
}

func (c *client) writer() {
	for {
		select {
		case msg, ok := <-c.sendq:
			if !ok {
				// our work here is done
				return
			}
			err := websocket.JSON.Send(c.socket, msg)
			if err != nil {
				panic(err)
				return
			}
		}
	}
}

func (c *client) run() {
	for {
		var data []byte
		err := websocket.Message.Receive(c.socket, &data)
		if err != nil {
			break
		}

		var req request
		err = json.Unmarshal(data, &req)
		if err != nil {
			c.send(Error("?", "invalid cmd"))
			continue
		}

		log.Printf("Got: %s\n-> %s\n", req.Cmd, string(data))

		switch req.Cmd {
		case "j": //join
			fallthrough
		case "p": //part
			var jpr joinPartRequest
			json.Unmarshal(data, &jpr)
			ch := getChannel(jpr.Channel)
			if ch != nil {
				if jpr.Cmd == "j" {
					//join
					c.listening[jpr.Channel] = ch
					ch.join <- c
				} else {
					//part
					delete(c.listening, jpr.Channel)
					ch.part <- c
				}
			} else {
				c.send(Error(jpr.Cmd, "invalid channel"))
			}
		case "g": //get
			var gr getRequest
			json.Unmarshal(data, &gr)
			ch := getChannel(gr.Channel)
			if ch != nil {
				get := getter{
					From: c,
					Var:  gr.Var,
				}
				ch.get <- get
			} else {
				c.send(Error(gr.Cmd, "invalid channel"))
			}
		case "G": //multi-get
			var gr multigetRequest
			json.Unmarshal(data, &gr)
			ch := getChannel(gr.Channel)
			if ch != nil {
				for _, v := range gr.Vars {
					get := getter{
						From: c,
						Var:  v,
					}
					ch.get <- get
				}
			} else {
				c.send(Error(gr.Cmd, "invalid channel"))
			}
		case "s": //set
			var sr setRequest
			json.Unmarshal(data, &sr)
			ch := getChannel(sr.Channel)
			if ch != nil {
				set := setter{
					From:  c,
					Var:   sr.Var,
					Value: sr.Value,
				}
				ch.set <- set
			} else {
				c.send(Error(sr.Cmd, "invalid channel"))
			}
		case "S": //multi-set
			var sr multisetRequest
			json.Unmarshal(data, &sr)
			ch := getChannel(sr.Channel)
			if ch != nil {
				for n, v := range sr.Values {
					set := setter{
						From:  c,
						Var:   n,
						Value: v,
					}
					ch.set <- set
				}
			} else {
				c.send(Error(sr.Cmd, "invalid channel"))
			}
		default:
			log.Printf("Unknown req %s\n", req.Cmd)
		}
	}
	// post-disconnect cleanup
	c.socket.Close()
	c.partAll()
	close(c.sendq)
	removeClient(c.id)
}

func (c *client) partAll() {
	for _, g := range c.listening {
		g.part <- c
	}
	c.listening = make(map[string]*channel)
}

func addClient(c *client) {
	clientsTableLock.Lock()
	defer clientsTableLock.Unlock()

	clientsTable[c.id] = c
}

func getClient(id clientID) *client {
	clientsTableLock.RLock()
	defer clientsTableLock.RUnlock()

	return clientsTable[id]
}

func removeClient(id clientID) {
	clientsTableLock.Lock()
	defer clientsTableLock.Unlock()

	delete(clientsTable, id)
}

func renameClient(old clientID, newVal clientID) {
	clientsTableLock.Lock()
	defer clientsTableLock.Unlock()

	c := clientsTable[old]
	delete(clientsTable, old)
	clientsTable[newVal] = c
}

func generateID() clientID {
	const size = 24
	var b = make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		log.Printf("Error in generateID(): %v", err)
	}
	security := base64.StdEncoding.EncodeToString(b)
	id := "_" + security
	return clientID(id)
}

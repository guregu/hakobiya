package main

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"log"
)

type client struct {
	userid    string
	socket    *websocket.Conn
	listening map[string]*channel

	sendq chan interface{}
}

func (c client) isUser(id string) bool {
	return c.userid == id
}

func (c *client) send(msg interface{}) {
	c.sendq <- msg
}

func (c *client) writer() {
	for {
		select {
		case msg, ok := <-c.sendq:
			if !ok {
				log.Println("Stopping SendQ")
				return
			}
			err := websocket.JSON.Send(c.socket, msg)
			if err != nil {
				panic(err)
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
		case "s": //set
		default:
			log.Printf("Unknown req %s\n", req.Cmd)
		}
	}
	c.socket.Close()
	c.partAll()
	close(c.sendq)
}

func (c *client) partAll() {
	for _, g := range c.listening {
		g.part <- c
	}
	c.listening = make(map[string]*channel)
}

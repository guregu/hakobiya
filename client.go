package main

import (
	"encoding/json"
	"log"

	"code.google.com/p/go.net/websocket"
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
				// our work here is done
				return
			}
			err := websocket.JSON.Send(c.socket, msg)
			if err != nil {
				// abandon ship
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

package main

type request struct {
	Cmd string `json:"x"`
}

type joinPartRequest struct {
	Cmd     string `json:"x"` // j or p
	Channel string `json:"c"`
}

type loginRequest struct {
	Cmd string `json:"x"` // l
	Key string `json:"k"`
}

type getRequest struct {
	Cmd     string   `json:"x"` // g
	Channel string   `json:"c"`
	Vars    []string `json:"n"`
}

type setRequest struct {
	Cmd     string      `json:"x"` // s
	Channel string      `json:"c"`
	Var     string      `json:"n"`
	Value   interface{} `json:"v"`
}

type errorMessage struct {
	Cmd     string `json:"x"` // !
	ReplyTo string `json:"w"`
	Channel string `json:"c,omitempty"`
	Var     string `json:"n,omitempty"`
	Message string `json:"m,omitempty"`
}

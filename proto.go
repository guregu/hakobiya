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
	Cmd     string `json:"x"` // g
	Channel string `json:"c"`
	Var     string `json:"n"`
}

type multigetRequest struct {
	Cmd     string   `json:"x"` // G
	Channel string   `json:"c"`
	Vars    []string `json:"n"`
}

type setRequest struct {
	Cmd     string      `json:"x"` // s
	Channel string      `json:"c"`
	Var     string      `json:"n"`
	Value   interface{} `json:"v"`
}

type multisetRequest struct {
	Cmd     string                 `json:"x"` // S
	Channel string                 `json:"c"`
	Values  map[string]interface{} `json:"v"`
}

type errorMessage struct {
	Cmd     string `json:"x"` // !
	ReplyTo string `json:"w"`
	Channel string `json:"c,omitempty"`
	Var     string `json:"n,omitempty"`
	Message string `json:"m,omitempty"`
}

func (e errorMessage) Error() string {
	return e.Message
}

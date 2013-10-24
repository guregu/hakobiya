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

type jsType string

const (
	jsBool   jsType = "bool"
	jsInt    jsType = "int"
	jsFloat  jsType = "float"
	jsString jsType = "string"
)

func (me jsType) is(v interface{}) bool {
	switch me {
	case jsBool:
		switch v.(type) {
		case bool:
			return true
		}
	case jsInt:
		switch v.(type) {
		case int:
			return true
		}
	case jsFloat:
		switch v.(type) {
		case float32, float64:
			return true
		}
	case jsString:
		switch v.(type) {
		case string:
			return true
		}
	default:
		panic("unknown jsType comparison! " + me)
	}
	return false
}

func (me jsType) zero() interface{} {
	switch me {
	case jsBool:
		return false
	case jsInt:
		return 0
	case jsFloat:
		return 0.0
	case jsString:
		return ""
	default:
		panic("unknown jsType " + me)
	}
}

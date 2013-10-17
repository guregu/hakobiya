package main

type channel struct {
	prefix     string
	name       string
	broadcasts map[string]*broadcast
	vars       map[string]interface{}
	computed   map[string]func() interface{}
	listeners  map[*client]bool
	// userVars   map[string]map[*client]interface{}

	send chan message
	get  chan getter
	set  chan setter
	join chan *client
	part chan *client
}

func newChannel(cfg channelConfig) *channel {
	ch := &channel{
		prefix:     cfg.Prefix,
		broadcasts: make(map[string]*broadcast),
		vars:       make(map[string]interface{}),
		listeners:  make(map[*client]bool),

		send: make(chan message),
		get:  make(chan getter),
		set:  make(chan setter),
		join: make(chan *client),
		part: make(chan *client),
	}

	return ch
}

func getChannel(name string) *channel {
	//TODO
	return nil
}

func (ch *channel) run() {
	for {
		select {}
	}
}

type message struct {
	From *client
	To   string
	Body interface{}
}

type getter struct {
	From *client
	Var  string
}

type setter struct {
	From *client
	Var  string
	Body interface{}
}

type broadcast struct {
	Type     string
	ReadOnly bool
}

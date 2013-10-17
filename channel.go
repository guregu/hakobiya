package main

import "sync"

var channelTable = make(map[string]*channel)
var channelTableMutex = &sync.RWMutex{}

var channelConfigs = make(map[uint8]channelConfig)

type varType int

const (
	ChannelVar varType = iota
	UserVar
	MagicVar
	SystemVar
)

type channel struct {
	prefix     string
	name       string
	restrict   []string
	listeners  map[*client]bool
	index      map[string]varType
	broadcasts map[string]broadcast
	vars       map[string]interface{}
	uservars   map[string]map[string]interface{}
	magic      map[string]func() interface{}
	// cache      map[string]interface{}
	// dirty      map[string]bool

	send chan message
	get  chan getter
	set  chan setter
	join chan *client
	part chan *client
}

func newChannel(name string) *channel {
	prefix := name[0]
	cfg, exists := channelConfigs[prefix]
	if !exists {
		//TODO: error: no such prefix
		return nil
	}
	ch := &channel{
		name:       cfg.Prefix + name,
		listeners:  make(map[*client]bool),
		broadcasts: make(map[string]broadcast),
		vars:       make(map[string]interface{}),
		uservars:   make(map[string]map[string]interface{}),
		magic:      make(map[string]func() interface{}),

		send: make(chan message),
		get:  make(chan getter),
		set:  make(chan setter),
		join: make(chan *client),
		part: make(chan *client),
	}
	cfg.apply(ch)
	return ch
}

func registerChannel(ch *channel) {
	channelTableMutex.Lock()
	if _, exists := channelTable[ch.name]; exists {
		panic("Remaking channel: " + ch.name)
	}
	channelTable[ch.name] = ch
	channelTableMutex.Unlock()

	go ch.run()
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

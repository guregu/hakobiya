package main

import "sync"
import "log"

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

var sigilTable = map[uint8]varType{
	'%': UserVar,
	'&': MagicVar,
	'$': SystemVar,
}

type channel struct {
	prefix     string
	name       string
	restrict   []string
	listeners  map[*client]bool
	index      map[string]varType
	broadcasts map[string]broadcast
	vars       map[string]interface{}
	uservars   map[string]map[*client]interface{}
	magic      map[string]func() interface{}
	cache      map[string]interface{}
	deps       map[string][]string
	// dirty      map[string]bool

	send chan message
	get  chan getter
	set  chan setter
	join chan *client
	part chan *client
}

// write all
func (ch *channel) wall(msg interface{}) {
	for c, _ := range ch.listeners {
		c.send(msg)
	}
}

// notify when vars change (vname needs sigil)
func (ch *channel) notify(vname string, value interface{}) {
	ch.wall(&setRequest{
		Cmd:     "s",
		Channel: ch.name,
		Var:     vname,
		Value:   value,
	})
}

func (ch *channel) invalidate(vname string) {
	if _, exists := ch.deps[vname]; exists {
		for _, dep := range ch.deps[vname] {
			oldval := ch.cache[dep]
			newval := ch.magic[dep]()
			if oldval != newval {
				ch.cache[dep] = newval
				ch.notify("&"+dep, newval)
			}
		}
	}
}

func (ch *channel) run() {
	log.Printf("Running channel: %s", ch.name)
	for {
		select {
		// case msg := <-ch.send:
		// 	bc, ok := ch.broadcasts[msg.To]
		// 	if !ok {

		// 	}
		// }
		case gtr := <-ch.get:
			vtype, exists := ch.index[gtr.Var]
			prefix, vname := gtr.Var[0], gtr.Var[1:]
			if !checkPrefix(prefix, vtype) {
				log.Printf("Mismatched sigil: %s for %v", gtr.Var, vtype)
			}
			if !exists {
				gtr.From.send(Error("g", "no such var"))
				continue
			}
			var val interface{}
			switch vtype {
			case UserVar:
				val = ch.uservars[vname]
			case MagicVar:
				val = ch.cache[vname]
			case SystemVar:
				val = ch.vars["$"+vname]
			}
			msg := &setRequest{
				Cmd:     "s",
				Channel: ch.name,
				Var:     gtr.Var,
				Value:   val,
			}
			gtr.From.send(msg)
		case sttr := <-ch.set:
			vtype, exists := ch.index[sttr.Var]
			prefix, vname := sttr.Var[0], sttr.Var[1:]
			if !checkPrefix(prefix, vtype) {
				log.Printf("Mismatched sigil: %s for %v", sttr.Var, vtype)
			}
			if !exists {
				sttr.From.send(Error("g", "no such var"))
				continue
			}
			//new stuff
			switch vtype {
			case UserVar:
				ch.uservars[vname][sttr.From] = sttr.Value
				ch.invalidate(vname)
			case ChannelVar:
				// TODO
			}
		}
	}
}

type message struct {
	From  *client
	To    string
	Value interface{}
}

type getter struct {
	From *client
	Var  string
}

type setter struct {
	From  *client
	Var   string
	Value interface{}
}

type broadcast struct {
	Type     string
	ReadOnly bool
}

func newChannel(name string) *channel {
	prefix := name[0]
	cfg, exists := channelConfigs[prefix]
	if !exists {
		return nil
	}
	ch := &channel{
		name:       name,
		listeners:  make(map[*client]bool),
		index:      make(map[string]varType),
		broadcasts: make(map[string]broadcast),
		vars:       make(map[string]interface{}),
		uservars:   make(map[string]map[*client]interface{}),
		magic:      make(map[string]func() interface{}),
		cache:      make(map[string]interface{}),
		deps:       make(map[string][]string),

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
	channelTableMutex.RLock()
	ch, exists := channelTable[name]
	channelTableMutex.RUnlock()
	if !exists {
		ch = newChannel(name)
		if ch != nil {
			registerChannel(ch)
		}
	}
	return ch
}

func checkPrefix(p uint8, vt varType) bool {
	return sigilTable[p] == vt
}

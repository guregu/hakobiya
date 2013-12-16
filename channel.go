package main

import (
	"errors"
	"log"
	"sync"
)

var channelTable = make(map[string]*channel)
var channelTableMutex = &sync.RWMutex{}

var channelConfigs = make(map[uint8]channelConfig)

type varType int

const (
	ChannelVar varType = iota
	UserVar
	MagicVar
	SystemVar
	BroadcastVar
	WireVar
)

var sigilTable = map[uint8]varType{
	'%': UserVar,
	'&': MagicVar,
	'$': SystemVar,
	'#': BroadcastVar,
	'=': WireVar,
}

type broadcast struct {
	Type     jsType
	ReadOnly bool
}

type wire struct {
	inputType  jsType
	outputType jsType
	rewrite    bool
	transform  func(*channel, wire, *client, interface{}) interface{}
}

type channel struct {
	prefix     string
	name       string
	restrict   []string
	listeners  map[*client]bool
	index      map[string]varType
	types      map[string]jsType
	broadcasts map[string]broadcast
	wires      map[string]wire
	vars       map[string]interface{}
	uservars   map[string]map[*client]interface{}
	magic      map[string]func() interface{}
	cache      map[string]interface{}
	deps       map[string][]string

	get  chan getter
	set  chan setter
	join chan *client
	part chan *client
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
		types:      make(map[string]jsType),
		broadcasts: make(map[string]broadcast),
		wires:      make(map[string]wire),
		vars:       make(map[string]interface{}),
		uservars:   make(map[string]map[*client]interface{}),
		magic:      make(map[string]func() interface{}),
		cache:      make(map[string]interface{}),
		deps:       make(map[string][]string),

		get:  make(chan getter),
		set:  make(chan setter),
		join: make(chan *client),
		part: make(chan *client),
	}
	cfg.apply(ch)
	return ch
}

// write all
func (ch *channel) wall(msg interface{}) {
	for c, _ := range ch.listeners {
		c.send(msg)
	}
}

// notify when vars change (needs sigil in the name)
func (ch *channel) notify(fullName string, value interface{}) {
	ch.wall(&setRequest{
		Cmd:     "s",
		Channel: ch.name,
		Var:     fullName,
		Value:   value,
	})
}

// re-computes magic values (no sigil needed)
func (ch *channel) invalidate(varName string) {
	if _, exists := ch.deps[varName]; exists {
		for _, dep := range ch.deps[varName] {
			oldVal := ch.cache[dep]
			newVal := ch.magic[dep]()
			if oldVal != newVal {
				ch.cache[dep] = newVal
				ch.notify("&"+dep, newVal)
			}
		}
	}
}

// gets value of a var (needs sigil)
func (ch *channel) value(fullName string, from *client) (val interface{}, e error) {
	// TODO: proper unicode handling
	prefix, varName := fullName[0], fullName[1:] // TODO: bounds check
	varType, exists := ch.index[varName]
	if !exists {
		log.Printf("[%s] unknown var: %s", ch.name, varName)
		e = errors.New("unknown var: " + fullName)
	} else if !checkPrefix(prefix, varType) {
		log.Printf("[%s] mismatched sigil: %s for %v", ch.name, fullName, varType)
		e = errors.New("mismatched sigil")
	}

	switch varType {
	case UserVar:
		val = ch.uservars[varName][from]
	case MagicVar:
		val = ch.cache[varName]
	case SystemVar:
		val = ch.vars["$"+varName]
	}

	return
}

// gets all values from a collection (uservars)
func (ch *channel) values(fullName string) (val map[*client]interface{}, e error) {
	// TODO: abstract this out
	prefix, varName := fullName[0], fullName[1:] // TODO: bounds check
	varType, exists := ch.index[varName]
	if !exists {
		log.Printf("[%s] unknown var: %s", ch.name, varName)
		e = errors.New("unknown var: " + fullName)
	} else if !checkPrefix(prefix, varType) {
		log.Printf("[%s] mismatched sigil: %s for %v", ch.name, fullName, varType)
		e = errors.New("mismatched sigil")
	}

	switch varType {
	case UserVar:
		val = ch.uservars[varName]
	}

	return
}

func (ch *channel) run() {
	log.Printf("Running channel: %s", ch.name)
	defer unregisterChannel(ch)

	for {
		select {
		case c := <-ch.join:
			ch.listeners[c] = true

			// new guy joined so we gotta set up his vars
			for varName, values := range ch.uservars {
				// TODO: some kind of default value setting, not just zero?
				values[c] = ch.types[varName].zero()
				ch.invalidate(varName)
			}

			// $listeners
			ct := len(ch.listeners)
			ch.vars["$listeners"] = ct
			if ch.index["listeners"] == SystemVar {
				ch.notify("$listeners", ct)
			}
		case c := <-ch.part:
			delete(ch.listeners, c)

			// goodbye, var cleanup
			for varName, values := range ch.uservars {
				if _, exists := values[c]; exists {
					delete(values, c)
					ch.invalidate(varName)
				}
			}

			// update $listeners
			ct := len(ch.listeners)
			ch.vars["$listeners"] = ct
			if ch.index["listeners"] == SystemVar {
				ch.notify("$listeners", ct)
			}

			// die?
			if ct == 0 {
				log.Printf("Dying: %s", ch.name)
				return
			}
		case getr := <-ch.get:
			val, err := ch.value(getr.Var, getr.From)
			if err != nil {
				getr.From.send(Error("g", err.Error()))
				continue
			}
			msg := &setRequest{
				Cmd:     "s",
				Channel: ch.name,
				Var:     getr.Var,
				Value:   val,
			}
			getr.From.send(msg)
		case setr := <-ch.set:
			// TODO: abstract prefix checking etc out
			prefix, varName := setr.Var[0], setr.Var[1:]
			varType, exists := ch.index[varName]
			if !exists {
				setr.From.send(Error("g", "no such var "+setr.Var))
				continue
			}
			if !checkPrefix(prefix, varType) {
				log.Printf("Mismatched sigil: %s for %v", setr.Var, varType)
			}

			switch varType {
			case UserVar:
				// did we get good data?
				type_ := ch.types[varName]
				if type_.is(setr.Value) {
					ch.uservars[varName][setr.From] = setr.Value
					ch.invalidate(varName)
				} else {
					setr.From.send(Error("s", "invalid type for "+setr.Var))
				}
			case ChannelVar:
				// TODO
			case BroadcastVar:
				b := ch.broadcasts[varName]
				if setr.From != nil {
					// TODO: should this be set by ReadOnly?
					setr.From.send(Error("s", "broadcast is read only: "+setr.Var))
				} else {
					if b.Type.is(setr.Value) {
						ch.notify(setr.Var, setr.Value)
					} else {
						log.Printf("[%s] invalid type for broadcast %s (%v)", ch.name, setr.Var, setr.Value)
					}
				}
			case WireVar:
				w := ch.wires[varName]
				send := setr.Value
				if w.rewrite {
					send = w.transform(ch, w, setr.From, setr.Value)
				}
				ch.notify(setr.Var, send)
			}
		}
	}
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

func registerChannel(ch *channel) {
	channelTableMutex.Lock()
	defer channelTableMutex.Unlock()

	if _, exists := channelTable[ch.name]; exists {
		panic("Remaking channel: " + ch.name)
	}
	channelTable[ch.name] = ch

	go ch.run()
}

func unregisterChannel(ch *channel) {
	channelTableMutex.Lock()
	defer channelTableMutex.Unlock()

	if _, exists := channelTable[ch.name]; !exists {
		panic("Deleting non-existent channel: " + ch.name)
	}
	delete(channelTable, ch.name)
}

func channelExists(name string) bool {
	channelTableMutex.RLock()
	defer channelTableMutex.RUnlock()

	_, exists := channelTable[name]
	return exists
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

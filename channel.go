package main

import (
	"encoding/json"
	"log"
	"sync"
	"unicode/utf8"
)

var channelTable = make(map[string]*channel)
var channelTableMutex = &sync.RWMutex{}

type channel struct {
	prefix     rune
	name       string
	restrict   []string
	listeners  map[*client]bool
	index      map[identifier]bool
	types      map[identifier]jsType
	broadcasts map[identifier]broadcast
	wires      map[identifier]wire
	vars       map[identifier]interface{}
	uservars   map[identifier]uservarMap
	magic      map[identifier]func() interface{}
	cache      map[identifier]interface{}
	deps       map[identifier][]identifier

	get     chan getter
	set     chan setter
	join    chan *client
	part    chan *client
	deliver chan order
}

func newChannel(name string) *channel {
	prefix, _ := utf8.DecodeRuneInString(name)
	cfg, exists := templates[prefix]
	if !exists {
		return nil
	}
	ch := &channel{
		name:       name,
		listeners:  make(map[*client]bool),
		index:      make(map[identifier]bool),
		types:      make(map[identifier]jsType),
		broadcasts: make(map[identifier]broadcast),
		wires:      make(map[identifier]wire),
		vars:       make(map[identifier]interface{}),
		uservars:   make(map[identifier]uservarMap),
		magic:      make(map[identifier]func() interface{}),
		cache:      make(map[identifier]interface{}),
		deps:       make(map[identifier][]identifier),

		get:     make(chan getter),
		set:     make(chan setter),
		join:    make(chan *client),
		part:    make(chan *client),
		deliver: make(chan order),
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

// notify when vars change
func (ch *channel) notify(v identifier, value interface{}) {
	ch.wall(setRequest{
		Cmd:     "s",
		Channel: ch.name,
		Var:     v,
		Value:   value,
	})
}

// re-computes magic values (no sigil needed)
func (ch *channel) invalidate(v identifier) {
	if _, exists := ch.deps[v]; exists {
		for _, dep := range ch.deps[v] {
			oldVal := ch.cache[dep]
			newVal := ch.magic[dep]()
			if oldVal != newVal {
				ch.cache[dep] = newVal
				ch.notify(dep, newVal)
			}
		}
	}
}

func (ch *channel) has(v identifier) bool {
	return ch.index[v]
}

// gets value of a var or returns an error
func (ch *channel) value(v identifier, from *client) (val interface{}, err *errorMessage) {
	if !ch.has(v) {
		return nil, channelError(ch, v, "no such var")
	}

	switch v.kind {
	case UserVar:
		if from != nil {
			val = ch.uservars[v][from]
		} else {
			val = "(nobody)"
		}
	case MagicVar:
		val = ch.cache[v]
	case SystemVar:
		val = ch.vars[v]
	default:
		err = channelError(ch, v, "unknown kind")
	}

	return
}

// gets all values from a collection (uservars)
func (ch *channel) values(v identifier) (val map[*client]interface{}, err *errorMessage) {
	if !ch.has(v) {
		return nil, channelError(ch, v, "no such var")
	}

	switch v.kind {
	case UserVar:
		val = ch.uservars[v]
	default:
		err = channelError(ch, v, "unhandled kind for values()")
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

			// welcome!
			// TODO: rejection
			c.send(joinPartRequest{
				Cmd:     "j",
				Channel: ch.name,
			})

			// new guy joined so we gotta set up his vars
			for name, values := range ch.uservars {
				// TODO: some kind of default value setting, not just zero?
				values[c] = ch.types[name].zero()
				ch.invalidate(name)
			}

			// $listeners
			ct := len(ch.listeners)
			if ch.has(listenersVar) {
				ch.vars[listenersVar] = ct
				ch.notify(listenersVar, ct)
			}
		case c := <-ch.part:
			delete(ch.listeners, c)

			// goodbye, var cleanup
			for name, values := range ch.uservars {
				if _, exists := values[c]; exists {
					delete(values, c)
					ch.invalidate(name)
				}
			}

			// update $listeners
			ct := len(ch.listeners)
			if ch.has(listenersVar) {
				ch.vars[listenersVar] = ct
				ch.notify(listenersVar, ct)
			}

			// die?
			if ct == 0 {
				log.Printf("Dying: %s", ch.name)
				return
			}
		case getr := <-ch.get:
			val, err := ch.value(getr.Var, getr.From)
			if err != nil {
				err.ReplyTo = "g"
				getr.From.send(err)
				continue
			}

			msg := &setRequest{
				Cmd:     "s",
				Channel: ch.name,
				Var:     getr.Var,
				Value:   val,
			}
			getr.From.send(msg)
		case o := <-ch.deliver:
			val, err := ch.value(o.get.Var, o.get.From)
			d := goods{
				value: val,
				err:   err,
			}
			o.to <- d
		case setr := <-ch.set:
			v := setr.Var
			if !ch.has(v) {
				if setr.From != nil {
					err := channelError(ch, v, "no such var")
					err.ReplyTo = "s"
					setr.From.send(err)
				}
				continue
			}

			switch v.kind {
			case UserVar:
				// did we get good data?
				type_ := ch.types[v]
				if type_.is(setr.Value) {
					ch.uservars[v][setr.From] = setr.Value
					ch.invalidate(v)
				} else {
					if setr.From != nil {
						err := channelError(ch, v, "invalid data: wrong type")
						err.ReplyTo = "s"
						setr.From.send(err)
					}
				}
			case ChannelVar:
				// TODO
			case BroadcastVar:
				b := ch.broadcasts[v]
				if setr.From != nil {
					// TODO: possibly let clients send to broadcasts too?
					err := channelError(ch, v, "You can't send to this")
					err.ReplyTo = "s"
					setr.From.send(err)
				} else {
					if b.Type.is(setr.Value) {
						ch.notify(setr.Var, setr.Value)
					} else {
						log.Printf("[%s] invalid type for broadcast %s (%v)", ch.name, setr.Var, setr.Value)
					}
				}
			case WireVar:
				w := ch.wires[v]
				msg := setr.Value
				if w.rewrite {
					msg = w.transform(ch, w, setr.From, setr.Value)
				}
				if setr.Overwrite != nil && w.outputType == jsObject {
					m := msg.(map[string]interface{})
					for k, v := range setr.Overwrite {
						m[k] = v
					}
				}
				ch.notify(v, msg)
			}
		}
	}
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

type getter struct {
	From *client
	Var  identifier
}

type setter struct {
	From      *client
	Var       identifier
	Value     interface{}
	Overwrite map[string]interface{}
}

type order struct {
	get getter
	to  chan<- goods
}

type goods struct {
	value interface{}
	err   *errorMessage
}

type uservarMap map[*client]interface{}

// wouldn't it be cool if json.Marshal used .String() so I didn't have to do this?
func (m uservarMap) MarshalJSON() (b []byte, err error) {
	idMap := make(map[string]interface{})
	for c, v := range m {
		idMap[string(c.id)] = v
	}
	b, err = json.Marshal(idMap)
	return
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

func channelError(ch *channel, v identifier, msg string) *errorMessage {
	return &errorMessage{
		Message: msg,
		Channel: ch.name,
		Var:     v,
	}
}

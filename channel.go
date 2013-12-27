package main

import (
	"log"
	"sync"
	"unicode/utf8"
)

var channelTable = make(map[string]*channel)
var channelTableMutex = &sync.RWMutex{}

type varKind int

const (
	ChannelVar varKind = iota
	UserVar
	MagicVar
	SystemVar
	BroadcastVar
	WireVar

	InvalidVar varKind = -1
)

var sigilTable = map[uint8]varKind{
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

type getter struct {
	From *client
	Var  string
}

type setter struct {
	From  *client
	Var   string
	Value interface{}
}

type channel struct {
	prefix     rune
	name       string
	restrict   []string
	listeners  map[*client]bool
	index      map[string]varKind
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
	prefix, _ := utf8.DecodeRuneInString(name)
	cfg, exists := templates[prefix]
	if !exists {
		return nil
	}
	ch := &channel{
		name:       name,
		listeners:  make(map[*client]bool),
		index:      make(map[string]varKind),
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
	ch.wall(setRequest{
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

// returns the kind information and sigil-less name of a variable, or an error
func (ch *channel) lookup(fullName string) (kind varKind, name string, err *errorMessage) {
	// the shortest variable is 2 characters
	if len(fullName) < 2 {
		return InvalidVar, "", channelError(ch, fullName, "Variable name too short.")
	}

	var sigil uint8
	var exists bool

	sigil, name = fullName[0], fullName[1:]
	kind, exists = ch.index[name]

	if !exists {
		log.Printf("[%s] unknown var: %s", ch.name, name)
		err = channelError(ch, fullName, "No such variable")
	} else {
		if sigilTable[sigil] != kind {
			log.Printf("[%s] mismatched sigil: %s for %v", ch.name, fullName, kind)
			err = channelError(ch, fullName, "Mismatched sigil")
		}
	}

	return
}

// gets value of a var (needs sigil)
func (ch *channel) value(fullName string, from *client) (val interface{}, err *errorMessage) {
	var kind varKind
	var name string
	kind, name, err = ch.lookup(fullName)

	switch kind {
	case UserVar:
		val = ch.uservars[name][from]
	case MagicVar:
		val = ch.cache[name]
	case SystemVar:
		val = ch.vars["$"+name]
	}

	return
}

// gets all values from a collection (uservars)
func (ch *channel) values(fullName string) (val map[*client]interface{}, err *errorMessage) {
	var kind varKind
	var name string
	kind, name, err = ch.lookup(fullName)

	switch kind {
	case UserVar:
		val = ch.uservars[name]
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
			for name, values := range ch.uservars {
				// TODO: some kind of default value setting, not just zero?
				values[c] = ch.types[name].zero()
				ch.invalidate(name)
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
			for name, values := range ch.uservars {
				if _, exists := values[c]; exists {
					delete(values, c)
					ch.invalidate(name)
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
		case setr := <-ch.set:
			kind, name, err := ch.lookup(setr.Var)
			if err != nil {
				if setr.From != nil {
					err.ReplyTo = "s"
					setr.From.send(err)
				}
			}
			switch kind {
			case UserVar:
				// did we get good data?
				type_ := ch.types[name]
				if type_.is(setr.Value) {
					ch.uservars[name][setr.From] = setr.Value
					ch.invalidate(name)
				} else {
					err := channelError(ch, setr.Var, "Invalid data: wrong type")
					err.ReplyTo = "s"
					setr.From.send(err)
				}
			case ChannelVar:
				// TODO
			case BroadcastVar:
				b := ch.broadcasts[name]
				if setr.From != nil {
					// TODO: possibly let clients send to broadcasts too?
					err := channelError(ch, setr.Var, "You can't send to this")
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
				w := ch.wires[name]
				msg := setr.Value
				if w.rewrite {
					msg = w.transform(ch, w, setr.From, setr.Value)
				}
				ch.notify(setr.Var, msg)
			}
		}
	}
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

func channelError(ch *channel, varName, msg string) *errorMessage {
	return &errorMessage{
		Message: msg,
		Channel: ch.name,
		Var:     varName,
	}
}

func checkSigil(sigil uint8, kind varKind) bool {
	return sigilTable[sigil] == kind
}

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
	prefix    rune
	name      string
	restrict  []string
	listeners map[*client]bool
	index     map[identifier]bool
	types     map[identifier]jsType
	wires     map[identifier]wire
	vars      map[identifier]interface{}
	uservars  map[identifier]uservarMap
	magic     map[identifier]func() interface{}
	cache     map[identifier]interface{}
	deps      map[identifier][]identifier

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
		name:      name,
		listeners: make(map[*client]bool),
		index:     make(map[identifier]bool),
		types:     make(map[identifier]jsType),
		wires:     make(map[identifier]wire),
		vars:      make(map[identifier]interface{}),
		uservars:  make(map[identifier]uservarMap),
		magic:     make(map[identifier]func() interface{}),
		cache:     make(map[identifier]interface{}),
		deps:      make(map[identifier][]identifier),

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
func (ch *channel) broadcast(msg interface{}) {
	for c, _ := range ch.listeners {
		c.send(msg)
	}
}

// notify when vars change
func (ch *channel) notify(v identifier, value interface{}) {
	ch.broadcast(setRequest{
		Cmd:     "s",
		Channel: ch.name,
		Var:     v,
		Value:   value,
	})
}

// notify when vars change (one user)
func (ch *channel) notifyOne(c *client, v identifier, value interface{}) {
	c.send(setRequest{
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
	_, exists := ch.index[v]
	return exists
}

func (ch *channel) hasUser(c *client) bool {
	_, exists := ch.listeners[c]
	return exists
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
			val = ch.uservars[v]
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

func (ch *channel) setVar(from, to *client, v identifier, value interface{}, overwrite map[string]interface{}) *errorMessage {
	canWrite, hasVar := ch.index[v]

	if !hasVar {
		err := channelError(ch, v, "no such var")
		return err
	}
	// TODO: only check this for uservars
	if to != nil && !ch.hasUser(to) {
		err := channelError(ch, v, "no such user here")
		return err
	}
	if from != nil {
		// later we might want to let users set other users's variables
		// but not now
		// also, don't let clients set read-only vars
		if to != from || !canWrite {
			err := channelError(ch, v, "can't set that")
			return err
		}
	}

	switch v.kind {
	case UserVar:
		// did we get good data?
		type_ := ch.types[v]
		if type_.is(value) {
			ch.uservars[v][to] = value
			if to != from {
				ch.notifyOne(to, v, value)
			}
			ch.invalidate(v)
		} else {
			if from != nil {
				err := channelError(ch, v, "wrong type")
				return err
			}
		}
	case ChannelVar:
		// TODO
	case WireVar:
		w := ch.wires[v]
		if !w.inputType.is(value) {
			err := channelError(ch, v, "wrong type")
			return err
		}
		msg := value
		if w.rewrite {
			msg = w.transform(ch, w, to, msg)
		}
		if overwrite != nil {
			if m, ok := msg.(map[string]interface{}); ok {
				for k, v := range overwrite {
					m[k] = v
				}
			}
		}
		ch.notify(v, msg)
	}
	return nil
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
			if ch.has(listenersSysVar) {
				ch.vars[listenersSysVar] = ct
				ch.notify(listenersSysVar, ct)
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
			if ch.has(listenersSysVar) {
				ch.vars[listenersSysVar] = ct
				ch.notify(listenersSysVar, ct)
			}

			// die?
			if ct == 0 {
				log.Printf("Dying: %s", ch.name)
				return
			}
		case get := <-ch.get:
			v, from := get.Var, get.From
			value, err := ch.value(v, from)
			if err != nil {
				err.ReplyTo = "g"
				from.send(err)
				continue
			}

			msg := &setRequest{
				Cmd:     "s",
				Channel: ch.name,
				Var:     v,
				Value:   value,
			}
			from.send(msg)
		case o := <-ch.deliver:
			if o.get.Var != blankIdentifier {
				value, err := ch.value(o.get.Var, o.get.From)
				d := goods{
					value: value,
					err:   err,
				}
				o.to <- d
			}
			if o.set.Var != blankIdentifier {
				err := ch.setVar(o.set.From, o.set.For, o.set.Var, o.set.Value, o.set.Overwrite)
				d := goods{
					value: o.set.Value,
					err:   err,
				}
				o.to <- d
			}
		case set := <-ch.set:
			err := ch.setVar(set.From, set.For, set.Var, set.Value, set.Overwrite)
			if err != nil {
				err.ReplyTo = "s"
				set.From.sendMaybe(err)
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
	For       *client
	Var       identifier
	Value     interface{}
	Overwrite map[string]interface{}
}

type order struct {
	get getter
	set setter
	to  chan<- goods
}

type goods struct {
	value interface{}
	err   *errorMessage
}

type uservarMap map[*client]interface{}

//wouldn't it be cool if json.Marshal used .String() (or encoding.TextMarshaler!) so I didn't have to do this?
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
		Cmd:     "!",
		Message: msg,
		Channel: ch.name,
		Var:     v,
	}
}

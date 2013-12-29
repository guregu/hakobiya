package main

// all known magic lives here
var grimoire = make(map[spell]magicEntry)

// magic signature
type spell struct {
	type_ jsType
	name  string
}

type magicEntry struct {
	f          magicMaker
	returnType jsType
}

// magic function generator
// func(*channel, src var name, params)
type magicMaker func(*channel, string, map[string]interface{}) func() interface{}

func registerMagic(sig spell, f magicMaker, returnType jsType) {
	grimoire[sig] = magicEntry{f, returnType}
}

func hasMagic(sig spell) bool {
	if _, ok := grimoire[sig]; ok {
		return true
	} else {
		if _, ok = grimoire[sig.generic()]; ok {
			return true
		}
	}
	return false
}

func defaultValue(sig spell) interface{} {
	if m, ok := grimoire[sig]; ok {
		return m.returnType.zero()
	}
	return nil
}

func makeMagic(ch *channel, src string, sig spell, params map[string]interface{}) func() interface{} {
	m, ok := grimoire[sig]
	if !ok {
		// is there a generic function?
		m, ok = grimoire[sig.generic()]
		if !ok {
			panic("unknown magic signature for: " + sig.String())
		}
	}
	return m.f(ch, src, params)
}

func (s spell) generic() spell {
	return spell{s.type_.any(), s.name}
}

func (s spell) String() string {
	return string(s.type_) + ":" + s.name
}

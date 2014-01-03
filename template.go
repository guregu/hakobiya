package main

import "unicode/utf8"

type channelTemplate struct {
	Prefix    string
	Expose    []identifier // special $vars to expose
	Restrict  []string
	Vars      map[string]*varDef `toml:"var"`
	Magic     map[string]*magicDef
	Broadcast map[string]*broadcast
	Wire      map[string]*wireDef
}

func (tmpl channelTemplate) apply(ch *channel) {
	prefix, _ := utf8.DecodeRuneInString(tmpl.Prefix)
	// prefix
	ch.prefix = prefix
	// restrict
	ch.restrict = tmpl.Restrict
	// expose
	for _, v := range tmpl.Expose {
		ch.index[v] = false // system vars are read-only
		switch v {
		case listenersVar:
			ch.vars[listenersVar] = 0
		default:
			panic("Unknown system var in expose: " + v.String())
		}
	}
	// user vars
	for name, def := range tmpl.Vars {
		v := identifier{
			sigil: '%',
			name:  name,
			kind:  UserVar,
		}
		ch.index[v] = !def.ReadOnly
		ch.uservars[v] = make(map[*client]interface{})
		ch.types[v] = def.Type
	}
	// TODO: channel vars?
	// magic
	for name, m := range tmpl.Magic {
		v := identifier{
			sigil: '&',
			name:  name,
			kind:  MagicVar,
		}
		ch.index[v] = false // all magic is read-only
		srcVar := tmpl.Vars[m.Src.name]
		s := spell{srcVar.Type, m.Func}
		ch.magic[v] = makeMagic(ch, m.Src, s, m.Params)
		ch.deps[m.Src] = append(ch.deps[m.Src], v)
		// set default value for magic cache
		ch.cache[v] = defaultValue(s)
	}
	// wires
	// TODO: some kind of generic function chain thingy
	for name, def := range tmpl.Wire {
		v := identifier{
			sigil: '=',
			name:  name,
			kind:  WireVar,
		}
		ch.index[v] = !def.ReadOnly
		w := wire{} // our baby wire
		w.inputType = def.Type
		w.outputType = def.Type
		if def.hasRewrite() {
			w.rewrite = true
			w.outputType = jsObject
			w.transform = func(ch *channel, _wire wire, from *client, input interface{}) interface{} {
				return def.Rewrite.transform(ch, from, input)
			}
		}
		ch.wires[v] = w
	}
}

func (tmpl channelTemplate) defines(v identifier) bool {
	switch v.kind {
	case UserVar:
		return tmpl.Vars[v.name] != nil
	case MagicVar:
		return tmpl.Magic[v.name] != nil
	case BroadcastVar:
		return tmpl.Broadcast[v.name] != nil
	case WireVar:
		return tmpl.Wire[v.name] != nil
	case SystemVar:
		return true // TODO: proper lookup
	}
	return false
}

type varDef struct {
	Type     jsType
	ReadOnly bool
	Default  interface{}
}

type magicDef struct {
	Src    identifier
	Func   string
	Param  interface{} // shortcut for Params["value"]
	Params map[string]interface{}
}

// there's a TOML parsing bug workaround here, see config.go
type wireDef struct {
	Type           jsType
	RewriteStrings map[string]string `toml:"rewrite"`
	Rewrite        rewriteDef        `toml:"-"`
	ReadOnly       bool
}

func (w wireDef) hasRewrite() bool {
	return len(w.RewriteStrings) > 0
}

type rewriteDef map[string]identifier

func (rw rewriteDef) transform(ch *channel, from *client, input interface{}) map[string]interface{} {
	transformed := make(map[string]interface{})
	for field, v := range rw {
		switch v.kind {
		case LiteralString:
			transformed[field] = v.name
		case SystemVar:
			// special case for $input
			if v.name == "input" {
				transformed[field] = input
				continue
			}
			fallthrough
		default:
			value, _ := ch.value(v, from)
			transformed[field] = value
		}
	}
	return transformed
}

package main

import "unicode/utf8"

type channelTemplate struct {
	Prefix    string
	Expose    []string // special $vars to expose
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
	// broadcasts
	for name, broadcastCfg := range tmpl.Broadcast {
		v := identifier{
			sigil: '#',
			name:  name,
			kind:  BroadcastVar,
		}
		ch.index[v] = true
		ch.broadcasts[v] = *broadcastCfg
	}
	// expose
	for _, b := range tmpl.Expose {
		// TODO there is a bug in the TOML library
		// that prevents arrays of indentifier, fix it
		v := identifier{}
		v.UnmarshalText([]byte(b))
		ch.index[v] = true
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
		ch.index[v] = true
		ch.uservars[v] = make(map[*client]interface{})
		ch.types[v] = def.Type
		// TODO set read only
	}
	// TODO: channel vars?
	// magic
	for name, m := range tmpl.Magic {
		v := identifier{
			sigil: '&',
			name:  name,
			kind:  MagicVar,
		}
		ch.index[v] = true
		srcVar := tmpl.Vars[m.Src.name]
		s := spell{srcVar.Type, m.Func}
		ch.magic[v] = makeMagic(ch, m.Src, s, m.Params)
		ch.deps[m.Src] = append(ch.deps[m.Src], v)
		// set default value for magic cache
		ch.cache[v] = defaultValue(s)
	}
	// wires
	// TODO: some kind of generic function chain thingy to make the logic here more sane
	for name, wireCfg := range tmpl.Wire {
		v := identifier{
			sigil: '=',
			name:  name,
			kind:  WireVar,
		}
		ch.index[v] = true
		w := wire{} // our baby wire
		if wireCfg.Input == nil {
			panic("no input definition for wire: " + name)
		} else {
			w.inputType = wireCfg.Input.Type
		}
		if wireCfg.Output == nil {
			w.outputType = w.inputType
		} else {
			w.outputType = wireCfg.Output.Type
			if wireCfg.Output.hasRewrite() {
				w.rewrite = true
				w.transform = func(ch *channel, _wire wire, from *client, input interface{}) interface{} {
					return wireCfg.Output.rewrite.transform(ch, from, input)
				}
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

type wireDef struct {
	Input  *wireDefInput
	Output *wireDefOutput
}

type wireDefInput struct {
	Type jsType
	Trim int
}

type wireDefOutput struct {
	Type           jsType
	RewriteStrings map[string]string `toml:"rewrite"` // TOML bug
	rewrite        rewriteDef        // TODO FIXME
}

func (w wireDefOutput) hasRewrite() bool {
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

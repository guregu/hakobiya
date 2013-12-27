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

func (cfg channelTemplate) apply(ch *channel) {
	prefix, _ := utf8.DecodeRuneInString(cfg.Prefix)
	// prefix
	ch.prefix = prefix
	// restrict
	ch.restrict = cfg.Restrict
	// broadcasts
	for name, broadcastCfg := range cfg.Broadcast {
		ch.broadcasts[name] = *broadcastCfg
	}
	// expose
	for _, ex := range cfg.Expose {
		varName := ex[1:]
		ch.index[varName] = SystemVar
		switch ex {
		case "$listeners":
			ch.magic["$listeners"] = func() interface{} {
				return len(ch.listeners)
			}
		default:
			panic("Unknown system var in expose: " + ex)
		}
	}
	// user vars
	for varName, v := range cfg.Vars {
		ch.index[varName] = UserVar
		ch.uservars[varName] = make(map[*client]interface{})
		ch.types[varName] = v.Type
		// TODO set read only
	}
	// TODO: channel vars?
	// magic
	for varName, m := range cfg.Magic {
		ch.index[varName] = MagicVar
		srcVar := m.Src[1:]
		v := cfg.Vars[srcVar]
		ch.magic[varName] = makeMagic(ch, m.Src, spell{v.Type, m.Func}, m.Params)
		ch.deps[srcVar] = append(ch.deps[srcVar], varName)
		// とりあえず run it once
		ch.cache[varName] = ch.magic[varName]()
	}
	// wires
	// TODO: some kind of generic function chain thingy to make the logic here more sane
	for varName, wireCfg := range cfg.Wire {
		ch.index[varName] = WireVar
		w := wire{} // our baby wire
		if wireCfg.Input == nil {
			panic("no input definition for wire: " + varName)
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
					return wireCfg.Output.Rewrite.rewrite(ch, from, input)
				}
			}
		}
		ch.wires[varName] = w
	}
}

type varDef struct {
	Type     jsType
	ReadOnly bool
	Default  interface{}
}

type magicDef struct {
	Src    string
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

type rewriteDef map[string]string

type wireDefOutput struct {
	Type    jsType
	Rewrite rewriteDef
}

func (w wireDefOutput) hasRewrite() bool {
	return len(w.Rewrite) > 0
}

func (rw rewriteDef) rewrite(ch *channel, from *client, input interface{}) map[string]interface{} {
	transformed := make(map[string]interface{})
	for fieldName, hVar := range rw {
		// literals:
		if hVar[0] == '\'' {
			transformed[fieldName] = hVar[1:]
		} else {
			switch hVar {
			case "$input":
				transformed[fieldName] = input
			default:
				v, _ := ch.value(hVar, from)
				transformed[fieldName] = v
			}
		}
	}
	return transformed
}

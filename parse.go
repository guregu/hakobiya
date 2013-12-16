package main

import "github.com/BurntSushi/toml"

type config struct {
	Server   serverConfig
	Channels []channelConfig `toml:"channel"`
	API      apiConfig
}

type serverConfig struct {
	Name string
	Bind string
	Path string
}

type channelConfig struct {
	Prefix    string
	Expose    []string // special $vars to expose
	Restrict  []string
	Vars      map[string]varDef `toml:"var"`
	Magic     map[string]magicDef
	Broadcast map[string]broadcast
	Wire      map[string]wireDef
}

type apiConfig struct {
	Enabled bool
	Path    string
	Key     string
}

// TODO: move validation elsewhere (only do it once)
func (cfg channelConfig) apply(ch *channel) {
	// prefix
	ch.prefix = cfg.Prefix
	// restrict
	ch.restrict = cfg.Restrict
	// broadcasts
	for name, broadcastCfg := range cfg.Broadcast {
		ch.broadcasts[name] = broadcastCfg
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
		validateOrPanic(v.Type, ch.prefix, varName)
		// TODO set read only
	}
	// TODO: channel vars?
	// magic
	for varName, m := range cfg.Magic {
		ch.index[varName] = MagicVar
		prefix, srcVar := m.Src[:1][0], m.Src[1:]
		if !checkPrefix(prefix, UserVar) {
			panic("magic var " + varName + " has invalid source var " + m.Src + ", expected a uservar (did you forget the %prefix?)")
		}
		v := cfg.Vars[srcVar]
		// you can use param as a shortcut for defining a params table to just set 'value'
		if m.Param != nil {
			if m.Params == nil {
				m.Params = make(map[string]interface{})
			} else {
				if _, exists := m.Params["value"]; exists {
					panic("magic " + varName + " has both param and params['value'] set, only set one!")
				}
			}
			m.Params["value"] = m.Param
		}
		ch.magic[varName] = makeMagic(ch, m.Src, magic{v.Type, m.Func}, m.Params)
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
			validateOrPanic(wireCfg.Input.Type, ch.prefix, varName)
		}
		if wireCfg.Output == nil {
			w.outputType = w.inputType
		} else {
			w.outputType = wireCfg.Output.Type
			validateOrPanic(wireCfg.Output.Type, ch.prefix, varName)
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

type wireDefOutput struct {
	Type    jsType
	Rewrite rewriteDef
}

func (w wireDefOutput) hasRewrite() bool {
	return len(w.Rewrite) > 0
}

type rewriteDef map[string]string

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

func parseConfig(file string) config {
	var conf config
	_, err := toml.DecodeFile(file, &conf)
	if err != nil {
		panic(err)
	}
	return conf
}

func validateOrPanic(t jsType, prefix, from string) {
	if !t.valid() {
		panic("[" + prefix + "] invalid type: '" + string(t) + "' for var " + from)
	}
}

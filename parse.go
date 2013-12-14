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

func (cfg channelConfig) apply(ch *channel) {
	// TODO: validate jsTypes

	// prefix
	ch.prefix = cfg.Prefix
	// restrict
	ch.restrict = cfg.Restrict
	// broadcasts
	for b_name, b := range cfg.Broadcast {
		ch.broadcasts[b_name] = b
	}
	// expose
	for _, ex := range cfg.Expose {
		v_name := ex[1:]
		ch.index[v_name] = SystemVar
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
	for v_name, v := range cfg.Vars {
		ch.index[v_name] = UserVar
		ch.uservars[v_name] = make(map[*client]interface{})
		ch.types[v_name] = jsType(v.Type) // TODO: validate types
		// TODO set read only
	}
	// TODO: channel vars?
	// magic
	for v_name, m := range cfg.Magic {
		ch.index[v_name] = MagicVar
		v := cfg.Vars[m.Var]
		ch.magic[v_name] = magic_func(ch, m.Var, v.Type, m.Map)
		ch.deps[m.Var] = append(ch.deps[m.Var], v_name)
		// とりあえず run it once
		ch.cache[v_name] = ch.magic[v_name]()
	}
	// wires
	// TODO: some kind of generic function chain thingy to make the logic here more sane
	// TODO: trim
	for v_name, w := range cfg.Wire {
		ch.index[v_name] = WireVar
		wyre := wire{} // our baby wire
		if w.Input == nil {
			panic("no input definition for wire: " + v_name)
		} else {
			wyre.inputType = w.Input.Type
		}
		if w.Output == nil {
			wyre.outputType = wyre.inputType
			// 	if w.Input.Trim > 0 {
			// 		switch (w.Input.Type) {
			// 		case jsString:
			// 		wyre.rewrite = true
			// 		wyre.transform = func(*channel, wire, *client, interface{}) {}
			// 	}
		} else {
			wyre.outputType = w.Output.Type
			if w.Output.hasRewrite() {
				wyre.rewrite = true
				wyre.transform = func(ch *channel, _wire wire, from *client, input interface{}) interface{} {
					return w.Output.Rewrite.rewrite(ch, from, input)
				}
			}
		}
		ch.wires[v_name] = wyre
	}
}

type varDef struct {
	Type     string
	ReadOnly bool
	Default  interface{}
}

type magicDef struct {
	Var string
	Map string
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
	ret := make(map[string]interface{})
	for fieldName, hVar := range rw {
		// literals:
		if hVar[0] == '\'' {
			ret[fieldName] = hVar[1:]
		} else {
			switch hVar {
			case "$input":
				ret[fieldName] = input
			default:
				v, _ := ch.value(hVar, from)
				ret[fieldName] = v
			}
		}
	}
	return ret
}

func parseConfig(file string) config {
	var conf config
	_, err := toml.DecodeFile(file, &conf)
	if err != nil {
		panic(err)
	}
	return conf
}

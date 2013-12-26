package main

import "github.com/BurntSushi/toml"
import "fmt"

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

var defaultServerConfig = serverConfig{
	Name: "Hakobiya",
	Bind: ":8080",
	Path: "/hakobiya",
}

type channelConfig struct {
	Prefix    string
	Expose    []string // special $vars to expose
	Restrict  []string
	Vars      map[string]*varDef `toml:"var"`
	Magic     map[string]*magicDef
	Broadcast map[string]*broadcast
	Wire      map[string]*wireDef
}

type apiConfig struct {
	Enabled bool
	Path    string
	Key     string
}

var defaultAPIConfig = apiConfig{
	Path: "/api",
}

func (cfg *config) prepare() {
	// [server]
	if cfg.Server.Name == "" {
		cfg.Server.Name = defaultServerConfig.Name
	}
	if cfg.Server.Bind == "" {
		cfg.Server.Bind = defaultServerConfig.Bind
	}
	if cfg.Server.Path == "" {
		cfg.Server.Path = defaultServerConfig.Path
	} else {
		cfg.Server.Path = fixPath(cfg.Server.Path)
	}

	// [api]
	if cfg.API.Path == "" {
		cfg.API.Path = defaultAPIConfig.Path
	} else {
		cfg.API.Path = fixPath(cfg.API.Path)
	}

	// [[channel]]
	for _, ch := range cfg.Channels {
		// [channel.var.*]
		for _, v := range ch.Vars {
			// sets type to 'any' if none
			v.Type = v.Type.rescue()
		}
		// [channel.broadcast.*]
		for _, b := range ch.Broadcast {
			b.Type = b.Type.rescue()
		}
		// [channel.wire.*]
		for _, w := range ch.Wire {
			w.Input.Type = w.Input.Type.rescue()
			if w.Output.hasRewrite() {
				// TODO: warn the user if they have it set to something other than 'object'?
				w.Output.Type = jsObject
			} else {
				w.Output.Type = w.Output.Type.rescue()
			}
		}
		// [channel.magic.*]
		for name, m := range ch.Magic {
			// you can use param as a shortcut for defining a params table to just set 'value'
			if m.Param != nil {
				if m.Params == nil {
					m.Params = make(map[string]interface{})
				} else {
					if _, exists := m.Params["value"]; exists {
						// TODO: ideally move this to check()
						panic("magic " + name + " has both param and params['value'] set, only set one!")
					}
				}
				m.Params["value"] = m.Param
			}
		}
	}
}

func (cfg config) check() (ok bool, errors []string) {
	channelMap := make(map[string]bool)

	for _, ch := range cfg.Channels {
		// channel basics
		if ch.Prefix == "" {
			errors = append(errors, "[[channel]] No 'prefix' set!")
		} else {
			if _, exists := channelMap[ch.Prefix]; exists {
				errors = append(errors, "[[channel]] Duplicate definition for prefix: "+ch.Prefix)
			} else {
				channelMap[ch.Prefix] = true
			}
		}
		// TODO: check ch.Expose
		// broadcast check
		for name, b := range ch.Broadcast {
			if !b.Type.valid() {
				errors = append(errors, fmt.Sprintf("(%s) [channel.broadcast.%s] Invalid type: %s", ch.Prefix, name, string(b.Type)))
			}
		}
		// uservar check
		for name, v := range ch.Vars {
			if !v.Type.valid() {
				errors = append(errors, fmt.Sprintf("(%s) [channel.var.%s] Invalid type: %s", ch.Prefix, name, string(v.Type)))
			}
		}
		// magic check
		for name, m := range ch.Magic {
			if m.Src == "" {
				errors = append(errors, fmt.Sprintf("(%s) [channel.magic.%s] Missing 'src' source variable definition!", ch.Prefix, name))
			} else {
				sigil, _, varOk := checkVarName(m.Src)
				if !varOk {
					errors = append(errors, fmt.Sprintf("(%s) [channel.magic.%s] Invalid source variable: %s",
						ch.Prefix, name, m.Src))
				}
				if !checkSigil(sigil, UserVar) {
					errors = append(errors, fmt.Sprintf("(%s) [channel.magic.%s] Invalid sigil for source variable: %s (expected %%...)",
						ch.Prefix, name, m.Src))
				}

			}
			if m.Func == "" {
				errors = append(errors, fmt.Sprintf("(%s) [channel.magic.%s] Missing 'func' magic function definition!", ch.Prefix, name))
			} else {
				_, naked, _ := checkVarName(m.Src)
				if srcVar, ok := ch.Vars[naked]; !ok {
					errors = append(errors, fmt.Sprintf("(%s) [channel.magic.%s] Source variable %s is not defined, did you forget [channel.var.%s]?",
						ch.Prefix, name, m.Src, naked))
				} else {
					if srcVar.Type.valid() {
						spl := spell{srcVar.Type, m.Func}
						if _, ok := Grimoire[spl]; !ok {
							generic := spell{srcVar.Type.any(), m.Func}
							if _, ok := Grimoire[generic]; !ok {
								errors = append(errors, fmt.Sprintf("(%s) [channel.magic.%s] No such magic spell: %s", ch.Prefix, name, spl))
							}
						}
					}
				}
			}
		}
		// wire check
		for name, w := range ch.Wire {
			if w.Input == nil {
				errors = append(errors, fmt.Sprintf("(%s) [channel.wire.%s.input] Missing!", ch.Prefix, name))
				continue
			}

			// TODO: in the future, type conversion etc
			if !w.Input.Type.valid() {
				errors = append(errors, fmt.Sprintf("(%s) [channel.wire.%s.input] Invalid type: %s", ch.Prefix, name, string(w.Input.Type)))
			}

			if w.Output != nil {
				if !w.Output.Type.valid() {
					errors = append(errors, fmt.Sprintf("(%s) [channel.wire.%s.output] Invalid type: %s", ch.Prefix, name, string(w.Output.Type)))
				}
				// rewrite check
				if w.Output != nil && w.Output.hasRewrite() {
					for n, v := range w.Output.Rewrite {
						if len(v) == 0 {
							errors = append(errors, fmt.Sprintf("(%s) [channel.wire.%s.output.rewrite] %s = blank!", ch.Prefix, name, n))
						} else {
							// if not a literal
							if v[0] != '\'' {
								_, _, ok := checkVarName(v)
								if !ok {
									errors = append(errors, fmt.Sprintf("(%s) [channel.wire.%s.output.rewrite] %s = %s, invalid var: %s",
										ch.Prefix, name, n, v, v))
								}
								// TODO: check if the var is defined or not
							}
						}
					}
				}
			}
		}
	}

	ok = len(errors) == 0
	return
}

func (cfg channelConfig) apply(ch *channel) {
	// prefix
	ch.prefix = cfg.Prefix
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

func parseConfig(file string) (cfg config, ok bool) {
	_, err := toml.DecodeFile(file, &cfg)
	if err != nil {
		panic(err)
	}
	cfg.prepare()

	var errors []string
	ok, errors = cfg.check()
	if !ok {
		fmt.Printf("ERROR: %d error(s) in %s:\n", len(errors), file)
		for n, e := range errors {
			fmt.Printf("#%d: %s\n", n+1, e)
		}
	}

	return cfg, ok
}

func fixPath(path string) string {
	if path[0] != '/' {
		path = "/" + path
	}
	if path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	return path
}

func checkVarName(v string) (sigil uint8, short string, ok bool) {
	if len(v) < 2 {
		return 0, "", false
	}

	sigil, short = v[0], v[1:]
	ok = true

	return
}

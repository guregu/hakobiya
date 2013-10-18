package main

import "github.com/BurntSushi/toml"

type config struct {
	Server   serverConfig
	Channels []channelConfig `toml:"channel"`
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
}

func (cfg channelConfig) apply(ch *channel) {
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
		ch.index[ex] = SystemVar
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
	for v_name, _ := range cfg.Vars {
		ch.index[v_name] = UserVar
		ch.uservars[v_name] = make(map[*client]interface{})
		// TODO set read only
	}
	// TODO: channel vars?
	// magic
	for v_name, m := range cfg.Magic {
		ch.index[v_name] = MagicVar
		v := cfg.Vars[m.Var]
		sig := v.Type + ":" + m.Map
		var fn func() interface{}
		switch sig {
		case "bool:any":
			fn = func() interface{} {
				for _, val := range ch.uservars[m.Var] {
					if val.(bool) {
						return true
					}
				}
				return false
			}
		case "bool:all":
			fn = func() interface{} {
				for _, val := range ch.uservars[m.Var] {
					if !val.(bool) {
						return false
					}
				}
				return true
			}
		case "bool:sum":
			fn = func() interface{} {
				ct := 0
				for _, val := range ch.uservars[m.Var] {
					if val.(bool) {
						ct++
					}
				}
				return ct
			}
		case "int:sum":
			fn = func() interface{} {
				sum := 0
				for _, val := range ch.uservars[m.Var] {
					sum += val.(int)
				}
				return sum
			}
		default:
			panic("Unknown magic signature: " + sig)
		}
		ch.magic[v_name] = fn
		ch.deps[m.Var] = append(ch.deps[m.Var], v_name)
	}
}

type varDef struct {
	Type     string
	ReadOnly bool
}

type magicDef struct {
	Var string
	Map string
}

func parseConfig(file string) config {
	var conf config
	_, err := toml.DecodeFile(file, &conf)
	if err != nil {
		panic(err)
	}
	return conf
}

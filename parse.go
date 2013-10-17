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
	// vars
	for v_name, v := range cfg.Vars {
		var typ varType
		switch v.Map {
		case "":
			typ = ChannelVar
		case "user":
			typ = UserVar
		case "sum":
			typ = MagicVar
		case "any":
			typ = MagicVar
		case "all":
			typ = MagicVar
		}
		ch.index[v_name] = typ

		if typ == MagicVar {
			var fn func() interface{}
			sig := v.Type + "_" + v.Map
			switch sig {
			case "bool_any":
				fn = func() interface{} {
					for _, vars := range ch.uservars {
						if vars[v_name].(bool) {
							return true
						}
					}
					return false
				}
			case "bool_all":
				fn = func() interface{} {
					for _, vars := range ch.uservars {
						if !vars[v_name].(bool) {
							return false
						}
					}
					return true
				}
			case "bool_sum":
				fn = func() interface{} {
					ct := 0
					for _, vars := range ch.uservars {
						if vars[v_name].(bool) {
							ct++
						}
					}
					return ct
				}
			case "int_sum":
				fn = func() interface{} {
					sum := 0
					for _, vars := range ch.uservars {
						sum += vars[v_name].(int)
					}
					return sum
				}
			default:
				panic("Unknown magic signature: " + sig)
			}
			ch.magic[v_name] = fn
		}
	}
}

type varDef struct {
	Type     string
	Map      string
	ReadOnly bool
}

func parseConfig(file string) config {
	var conf config
	_, err := toml.DecodeFile(file, &conf)
	if err != nil {
		panic(err)
	}
	return conf
}

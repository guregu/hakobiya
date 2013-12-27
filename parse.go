package main

import (
	"fmt"
	"os"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
)

type config struct {
	Server   serverConfig
	Static   staticConfig
	Channels []channelTemplate `toml:"channel"`
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

type staticConfig struct {
	Enabled bool
	Index   string
	Dirs    []string
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
	// static server stuff
	if cfg.Static.Enabled {
		if cfg.Static.Index == "" && len(cfg.Static.Dirs) == 0 {
			errors = append(errors, "[static] enabled set to true but not serving anything, try setting 'index' or 'dirs' or disabling it")
		}

		if cfg.Static.Index != "" {
			if stat, err := os.Stat(cfg.Static.Index); os.IsNotExist(err) {
				errors = append(errors, fmt.Sprintf("[static] index: no such file: %s", cfg.Static.Index))
			} else {
				if stat.IsDir() {
					errors = append(errors, fmt.Sprintf("[static] index needs a file, not a directory (%s)", cfg.Static.Index))
				}
			}
		}

		for _, dir := range cfg.Static.Dirs {
			if stat, err := os.Stat(dir); os.IsNotExist(err) {
				errors = append(errors, fmt.Sprintf("[static] dirs: no such directory: %s", dir))
			} else {
				if !stat.IsDir() {
					errors = append(errors, fmt.Sprintf("[static] dirs: not a directory: %s", dir))
				}
			}
		}
	}

	// channel stuff
	channelMap := make(map[string]bool)

	for _, ch := range cfg.Channels {
		// basics
		if ch.Prefix == "" {
			errors = append(errors, "[[channel]] No 'prefix' set!")
		} else {
			if utf8.RuneCountInString(ch.Prefix) > 1 {
				errors = append(errors, "[[channel]] prefix '"+ch.Prefix+"' too long, it should only be one character")
			}

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
				if srcVar, ok := ch.Vars[naked]; !ok && (m.Src != "") {
					errors = append(errors, fmt.Sprintf("(%s) [channel.magic.%s] Source variable %s is not defined, did you forget [channel.var.%s]?",
						ch.Prefix, name, m.Src, naked))
				} else {
					if srcVar.Type.valid() {
						sig := spell{srcVar.Type, m.Func}
						if !hasMagic(sig) {
							errors = append(errors, fmt.Sprintf("(%s) [channel.magic.%s] No such magic spell: %s", ch.Prefix, name, sig))
						}
					}
				}
			}
		}
		// wire check
		for name, w := range ch.Wire {
			// input stuff
			// TODO: in the future, type conversion etc
			if w.Input == nil {
				errors = append(errors, fmt.Sprintf("(%s) [channel.wire.%s.input] Missing!", ch.Prefix, name))
				continue
			}
			if !w.Input.Type.valid() {
				errors = append(errors, fmt.Sprintf("(%s) [channel.wire.%s.input] Invalid type: %s", ch.Prefix, name, string(w.Input.Type)))
			}
			if w.Output == nil {
				continue
			}

			// output stuff
			if !w.Output.Type.valid() {
				errors = append(errors, fmt.Sprintf("(%s) [channel.wire.%s.output] Invalid type: %s", ch.Prefix, name, string(w.Output.Type)))
			}
			if w.Output.hasRewrite() {
				for n, v := range w.Output.Rewrite {
					if len(v) == 0 {
						errors = append(errors, fmt.Sprintf("(%s) [channel.wire.%s.output.rewrite] %s = blank!", ch.Prefix, name, n))
						continue
					}

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

	ok = len(errors) == 0
	return
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

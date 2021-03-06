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

func parseConfig(file string) (cfg config, ok bool) {
	_, err := toml.DecodeFile(file, &cfg)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err.Error())
		return config{}, false
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
		// [channel.wire.*]
		for _, w := range ch.Wire {
			if w.hasRewrite() {
				// TOML bug hack
				// it can't handle map[string]identifier so we have to do this
				w.Rewrite = make(rewriteDef)
				for n, str := range w.RewriteStrings {
					v := identifier{}
					v.UnmarshalText([]byte(str))
					w.Rewrite[n] = v
				}
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
		// expose
		for _, v := range ch.Expose {
			if v.kind != SystemVar {
				errors = append(errors, fmt.Sprintf("(%s) [channel.expose] Not a system variable: %s", ch.Prefix, v))
			}
		}
		// broadcast check
		for name, b := range ch.Broadcast {
			if !b.Type.valid() {
				errors = append(errors, fmt.Sprintf("(%s) [channel.broadcast.%s] Invalid type: %s", ch.Prefix, name, b.Type))
			}
		}
		// uservar check
		for name, v := range ch.Vars {
			if !v.Type.valid() {
				errors = append(errors, fmt.Sprintf("(%s) [channel.var.%s] Invalid type: %s", ch.Prefix, name, v.Type))
			}
		}
		// magic check
		for name, m := range ch.Magic {
			if m.Func == "" {
				errors = append(errors, fmt.Sprintf("(%s) [channel.magic.%s] Missing 'func' magic function definition!", ch.Prefix, name))
			} else {
				if !ch.defines(m.Src) {
					errors = append(errors, fmt.Sprintf("(%s) [channel.magic.%s] Source variable %s is not defined, did you forget [channel.var.%s]?",
						ch.Prefix, name, m.Src, m.Src.name))
				} else {
					srcVar := ch.Vars[m.Src.name]
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
			if !w.Type.valid() {
				errors = append(errors, fmt.Sprintf("(%s) [channel.wire.%s.input] Invalid type: %s", ch.Prefix, name, string(w.Type)))
			}
			if w.hasRewrite() {
				for n, v := range w.Rewrite {
					if v.kind != LiteralString && !ch.defines(v) {
						errors = append(errors, fmt.Sprintf("(%s) [channel.wire.%s.output.rewrite] %s = %s, no such var: %s",
							ch.Prefix, name, n, v, v))
					}
				}
			}
		}
		// done
	}

	ok = len(errors) == 0
	return
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

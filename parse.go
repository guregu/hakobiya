package main

import "github.com/BurntSushi/toml"

type config struct {
	Server   serverConfig
	Channels []channelConfig
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

type varDef struct {
	Type     string
	Map      string
	ReadOnly bool
}

func parseConfig(file string) config {
	var conf config
	md, err := toml.DecodeFile(file, &conf)
	if err != nil {
		panic(err)
	}
	return conf
}

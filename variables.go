package main

import (
	"fmt"
	"unicode/utf8"
)

type varKind int

const (
	InvalidVar varKind = iota
	ChannelVar
	UserVar
	MagicVar
	SystemVar
	BroadcastVar
	WireVar

	LiteralString
)

var sigilTable = map[rune]varKind{
	'%':  UserVar,
	'&':  MagicVar,
	'$':  SystemVar,
	'#':  BroadcastVar,
	'=':  WireVar,
	'\'': LiteralString,
}

// system vars
var (
	listenersVar identifier // $listeners
)

type identifier struct {
	sigil rune
	name  string
	kind  varKind
}

func (id *identifier) MarshalText() (text []byte, err error) {
	return []byte(id.String()), nil
}

func (id *identifier) UnmarshalText(text []byte) error {
	if len(text) < 2 {
		return Error("", "invalid var: too short")
	}

	sigil, size := utf8.DecodeRune(text)
	id.sigil, id.name = sigil, string(text[size:])
	id.kind = sigilTable[sigil]
	if id.kind == InvalidVar {
		return Error("", "invalid var: "+string(text))
	}
	return nil
}

func (id identifier) MarshalJSON() (b []byte, err error) {
	s := fmt.Sprintf(`"%c%s"`, id.sigil, id.name)
	return []byte(s), nil
}

func (id identifier) String() string {
	return fmt.Sprintf("%c%s", id.sigil, id.name)
}

func init() {
	// set up system vars
	listenersVar.UnmarshalText([]byte("$listeners"))
}

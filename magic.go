package main

// magic function generator
// func(*channel, src var name, params)
type spell func(*channel, string, map[string]interface{}) func() interface{}

// magic signature
type magic struct {
	type_ jsType
	name  string
}

func (sig magic) String() string {
	return string(sig.type_) + ":" + sig.name
}

// all known magic function generators live here
var Grimoire = make(map[magic]spell)

func RegisterMagic(sig magic, f spell) {
	Grimoire[sig] = f
}

func makeMagic(ch *channel, src string, sig magic, params map[string]interface{}) func() interface{} {
	f, ok := Grimoire[sig]
	if !ok {
		panic("unknown magic signature for: " + sig.String())
	}
	return f(ch, src, params)
}

// returns true if any source value is true
func _bool_any(ch *channel, src string, params map[string]interface{}) func() interface{} {
	return func() interface{} {
		values, _ := ch.values(src)

		// special case: no one is here
		if len(values) == 0 {
			return false
		}

		for _, val := range values {
			if val.(bool) {
				return true
			}
		}
		return false
	}
}

// returns true if all source values are true
func _bool_all(ch *channel, src string, params map[string]interface{}) func() interface{} {
	return func() interface{} {
		values, _ := ch.values(src)

		// special case: no one is here
		if len(values) == 0 {
			return false
		}

		for _, val := range values {
			if !val.(bool) {
				return false
			}
		}
		return true
	}
}

// returns a count of the number of source values that are true
func _bool_sum(ch *channel, src string, params map[string]interface{}) func() interface{} {
	return func() interface{} {
		values, _ := ch.values(src)

		ct := 0
		for _, val := range values {
			if val.(bool) {
				ct++
			}
		}
		return ct
	}
}

func _int_sum(ch *channel, src string, params map[string]interface{}) func() interface{} {
	return func() interface{} {
		values, _ := ch.values(src)

		sum := 0
		for _, val := range values {
			sum += val.(int)
		}
		return sum
	}
}

func init() {
	// boolean magic
	RegisterMagic(magic{jsBool, "any"}, _bool_any)
	RegisterMagic(magic{jsBool, "all"}, _bool_all)
	RegisterMagic(magic{jsBool, "sum"}, _bool_sum)
	// integer magic
	RegisterMagic(magic{jsInt, "sum"}, _int_sum)
}

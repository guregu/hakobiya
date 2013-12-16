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
		// is there a generic function?
		f, ok = Grimoire[magic{sig.type_.any(), sig.name}]
		if !ok {
			panic("unknown magic signature for: " + sig.String())
		}
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

// returns the sum
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

// returns true if all values are the same
func _any_same(ch *channel, src string, params map[string]interface{}) func() interface{} {
	return func() interface{} {
		values, _ := ch.values(src)

		ct := len(values)
		if ct == 0 {
			// special case: no people
			return false
		}

		var first interface{}
		n := 0
		for _, v := range values {
			if n == 0 {
				first = v
			} else {
				if v != first {
					return false
				}
			}
			n++
		}

		return true
	}
}

func _any_all_equal(ch *channel, src string, params map[string]interface{}) func() interface{} {
	cmp, ok := params["value"]
	if !ok {
		panic("magic [any].all equal (using " + src + ") - missing 'value' parameter!")
	}
	return func() interface{} {
		values, _ := ch.values(src)

		if len(values) == 0 {
			return false
		}

		for _, v := range values {
			if v != cmp {
				return false
			}
		}
		return true
	}
}

func _any_any_equal(ch *channel, src string, params map[string]interface{}) func() interface{} {
	cmp, ok := params["value"]
	if !ok {
		panic("magic [any].any equal (using " + src + ") - missing 'value' parameter!")
	}
	return func() interface{} {
		values, _ := ch.values(src)

		if len(values) == 0 {
			return false
		}

		for _, v := range values {
			if v == cmp {
				return true
			}
		}

		return false
	}
}

func init() {
	// boolean magic
	RegisterMagic(magic{jsBool, "any"}, _bool_any)
	RegisterMagic(magic{jsBool, "all"}, _bool_all)
	RegisterMagic(magic{jsBool, "sum"}, _bool_sum)
	// integer magic
	RegisterMagic(magic{jsInt, "sum"}, _int_sum)
	// any type magic
	RegisterMagic(magic{jsAnything, "same"}, _any_same)
	RegisterMagic(magic{jsAnything, "all equal"}, _any_all_equal)
	RegisterMagic(magic{jsAnything, "any equal"}, _any_any_equal)
}

package main

// magic function generator
// func(*channel, src var name, params)
type magicMaker func(*channel, string, map[string]interface{}) func() interface{}

// magic signature
type spell struct {
	type_ jsType
	name  string
}

func (sig spell) String() string {
	return string(sig.type_) + ":" + sig.name
}

// all known magic function generators live here
var Grimoire = make(map[spell]magicMaker)

func RegisterMagic(sig spell, f magicMaker) {
	Grimoire[sig] = f
}

func makeMagic(ch *channel, src string, sig spell, params map[string]interface{}) func() interface{} {
	f, ok := Grimoire[sig]
	if !ok {
		// is there a generic function?
		f, ok = Grimoire[spell{sig.type_.any(), sig.name}]
		if !ok {
			panic("unknown magic signature for: " + sig.String())
		}
	}
	return f(ch, src, params)
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

// returns true if all source values equal the 'value' parameter
// if no 'value' param is given, checks if all values are non-zero
func _any_all(ch *channel, src string, params map[string]interface{}) func() interface{} {
	cmp, ok := params["value"]
	if ok {
		// we have a comparison value
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

	// no comaprison value, so see if every value is non-zero
	_, name, _ := ch.lookup(src)
	srcType := ch.types[name]
	return func() interface{} {
		values, _ := ch.values(src)

		if len(values) == 0 {
			return false
		}

		for _, val := range values {
			if val == srcType.zero() {
				return false
			}
		}

		return true
	}
}

// returns true if any of the source values equal the 'value' parameter
// if no 'value' param is given, checks if there are any non-zero values
func _any_any(ch *channel, src string, params map[string]interface{}) func() interface{} {
	cmp, ok := params["value"]
	if ok {
		// we have a comparison value
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

	// no comaprison value, so see if there's any non-zero values
	_, name, _ := ch.lookup(src)
	srcType := ch.types[name]
	return func() interface{} {
		values, _ := ch.values(src)

		if len(values) == 0 {
			return false
		}

		for _, v := range values {
			if v != srcType.zero() {
				return true
			}
		}

		return false
	}
}

func init() {
	// boolean magic
	RegisterMagic(spell{jsBool, "sum"}, _bool_sum)
	// integer magic
	RegisterMagic(spell{jsInt, "sum"}, _int_sum)
	// any type magic
	RegisterMagic(spell{jsAnything, "same"}, _any_same)
	RegisterMagic(spell{jsAnything, "any"}, _any_any)
	RegisterMagic(spell{jsAnything, "all"}, _any_all)
}

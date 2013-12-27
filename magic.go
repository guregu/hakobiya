package main

// magic function generator
// func(*channel, src var name, params)
type magicMaker func(*channel, string, map[string]interface{}) func() interface{}

// magic signature
type spell struct {
	type_ jsType
	name  string
}

// all known magic function generators live here
var grimoire = make(map[spell]magicMaker)

func registerMagic(sig spell, f magicMaker) {
	grimoire[sig] = f
}

func hasMagic(sig spell) bool {
	if _, ok := grimoire[sig]; ok {
		return true
	} else {
		generic := spell{sig.type_.any(), sig.name}
		if _, exists := grimoire[generic]; exists {
			return true
		}
	}
	return false
}

func makeMagic(ch *channel, src string, sig spell, params map[string]interface{}) func() interface{} {
	f, ok := grimoire[sig]
	if !ok {
		// is there a generic function?
		f, ok = grimoire[spell{sig.type_.any(), sig.name}]
		if !ok {
			panic("unknown magic signature for: " + sig.String())
		}
	}
	return f(ch, src, params)
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

// counts the number of sourve values that equal the 'value' parameter
// if no 'value' param is given, counts the number of non-zero values
func _any_count(ch *channel, src string, params map[string]interface{}) func() interface{} {
	cmp, ok := params["value"]
	if ok {
		return func() interface{} {
			values, _ := ch.values(src)
			ct := 0
			for _, v := range values {
				if v == cmp {
					ct++
				}
			}
			return ct
		}
	}

	// no comaprison value, so count the non-zero values
	_, name, _ := ch.lookup(src)
	srcType := ch.types[name]
	return func() interface{} {
		values, _ := ch.values(src)
		ct := 0
		for _, v := range values {
			if v != srcType.zero() {
				ct++
			}
		}
		return ct
	}
}

func _any_percent(ch *channel, src string, params map[string]interface{}) func() interface{} {
	countFunc := _any_count(ch, src, params)
	return func() interface{} {
		listeners := len(ch.listeners)

		if listeners == 0 {
			return 0.0
		}

		ct := float64(countFunc().(int))
		return ct / float64(listeners)
	}
}

func init() {
	// integer magic
	registerMagic(spell{jsInt, "sum"}, _int_sum)
	// any type magic
	registerMagic(spell{jsAnything, "same"}, _any_same)
	registerMagic(spell{jsAnything, "any"}, _any_any)
	registerMagic(spell{jsAnything, "all"}, _any_all)
	registerMagic(spell{jsAnything, "count"}, _any_count)
	registerMagic(spell{jsAnything, "percent"}, _any_percent)
}

func (sig spell) String() string {
	return string(sig.type_) + ":" + sig.name
}

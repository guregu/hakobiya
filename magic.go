package main

// all known magic lives here
var grimoire = make(map[spell]magicEntry)

// magic signature
type spell struct {
	type_ jsType
	name  string
}

type magicEntry struct {
	f          magicMaker
	returnType jsType
}

// magic function generator
// func(*channel, src var name, params)
type magicMaker func(*channel, string, map[string]interface{}) func() interface{}

func registerMagic(sig spell, f magicMaker, returnType jsType) {
	grimoire[sig] = magicEntry{f, returnType}
}

func hasMagic(sig spell) bool {
	if _, ok := grimoire[sig]; ok {
		return true
	} else {
		if _, ok = grimoire[sig.generic()]; ok {
			return true
		}
	}
	return false
}

func defaultValue(sig spell) interface{} {
	if m, ok := grimoire[sig]; ok {
		return m.returnType.zero()
	}
	return nil
}

func makeMagic(ch *channel, src string, sig spell, params map[string]interface{}) func() interface{} {
	m, ok := grimoire[sig]
	if !ok {
		// is there a generic function?
		m, ok = grimoire[sig.generic()]
		if !ok {
			panic("unknown magic signature for: " + sig.String())
		}
	}
	return m.f(ch, src, params)
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

// returns the average (rounded to an int)
func _int_avg(ch *channel, src string, params map[string]interface{}) func() interface{} {
	sumFunc := _int_sum(ch, src, params)
	return func() interface{} {
		sum, ct := sumFunc().(int), len(ch.listeners)
		return sum / ct
	}
}

// returns the maximum value
func _int_max(ch *channel, src string, params map[string]interface{}) func() interface{} {
	return func() interface{} {
		values, _ := ch.values(src)
		var max *int
		for _, val := range values {
			n := val.(int)
			if max == nil {
				max = &n
			} else {
				if n > *max {
					max = &n
				}
			}
		}
		return *max
	}
}

// returns the minimum value
func _int_min(ch *channel, src string, params map[string]interface{}) func() interface{} {
	return func() interface{} {
		values, _ := ch.values(src)
		var min *int
		for _, val := range values {
			n := val.(int)
			if min == nil {
				min = &n
			} else {
				if n < *min {
					min = &n
				}
			}
		}
		return *min
	}
}

// returns true if all values are the same
func _any_same(ch *channel, src string, params map[string]interface{}) func() interface{} {
	return func() interface{} {
		values, _ := ch.values(src)
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
		ct := float64(countFunc().(int))
		return ct / float64(listeners)
	}
}

func init() {
	// integer magic
	registerMagic(spell{jsInt, "sum"}, _int_sum, jsInt)
	registerMagic(spell{jsInt, "max"}, _int_max, jsInt)
	registerMagic(spell{jsInt, "min"}, _int_min, jsInt)
	registerMagic(spell{jsInt, "avg"}, _int_avg, jsInt)
	// any type magic
	registerMagic(spell{jsAnything, "same"}, _any_same, jsBool)
	registerMagic(spell{jsAnything, "any"}, _any_any, jsBool)
	registerMagic(spell{jsAnything, "all"}, _any_all, jsBool)
	registerMagic(spell{jsAnything, "count"}, _any_count, jsInt)
	registerMagic(spell{jsAnything, "percent"}, _any_percent, jsFloat)
}

func (s spell) generic() spell {
	return spell{s.type_.any(), s.name}
}

func (s spell) String() string {
	return string(s.type_) + ":" + s.name
}

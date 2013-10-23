package main

func magic_func(ch *channel, sourceVar, v_type, v_map string) func() interface{} {
	sig := v_type + ":" + v_map
	switch sig {
	case "bool:any":
		return func() interface{} {
			for _, val := range ch.uservars[sourceVar] {
				if val.(bool) {
					return true
				}
			}
			return false
		}
	case "bool:all":
		return func() interface{} {
			for _, val := range ch.uservars[sourceVar] {
				if !val.(bool) {
					return false
				}
			}
			return true
		}
	case "bool:sum":
		return func() interface{} {
			ct := 0
			for _, val := range ch.uservars[sourceVar] {
				if val.(bool) {
					ct++
				}
			}
			return ct
		}
	case "int:sum":
		return func() interface{} {
			sum := 0
			for _, val := range ch.uservars[sourceVar] {
				sum += val.(int)
			}
			return sum
		}
	default:
		panic("Unknown magic signature: " + sig)
	}
}

func zero_value(v_type string) interface{} {
	switch v_type {
	case "bool":
		return false
	case "int":
		return 0
	case "string":
		return ""
	default:
		return nil
	}
}

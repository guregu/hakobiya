package main

type jsType string

const (
	jsBool          jsType = "bool"
	jsBoolArray     jsType = "bool[]"
	jsInt           jsType = "int"
	jsIntArray      jsType = "int[]"
	jsFloat         jsType = "float"
	jsFloatArray    jsType = "float[]"
	jsString        jsType = "string"
	jsStringArray   jsType = "string[]"
	jsObject        jsType = "object"
	jsObjectArray   jsType = "object[]"
	jsAnything      jsType = ""
	jsAnythingArray jsType = "any[]"
	jsNone          jsType = "(none)"
)

func (me jsType) valid() bool {
	switch me {
	case jsBool, jsBoolArray,
		jsInt, jsIntArray,
		jsFloat, jsFloatArray,
		jsString, jsStringArray,
		jsObject, jsObjectArray,
		jsAnything, jsAnythingArray:
		return true
	}
	return false
}

func (me jsType) is(v interface{}) bool {
	switch me {
	case jsBool:
		_, ok := v.(bool)
		return ok
	case jsBoolArray:
		_, ok := v.([]bool)
		return ok
	case jsInt:
		_, ok := v.(int)
		return ok
	case jsIntArray:
		_, ok := v.([]int)
		return ok
	case jsFloat:
		switch v.(type) {
		case float32, float64:
			return true
		}
	case jsFloatArray:
		switch v.(type) {
		case []float32, []float64:
			return true
		}
	case jsString:
		_, ok := v.(string)
		return ok
	case jsStringArray:
		_, ok := v.([]string)
		return ok
	case jsAnythingArray:
		_, ok := v.([]interface{})
		return ok
	case jsAnything:
		return true
	default:
		panic(".is(): unknown jsType! " + me)
	}
	return false
}

func (me jsType) zero() interface{} {
	switch me {
	case jsBool:
		return false
	case jsBoolArray:
		return []bool{}
	case jsInt:
		return 0
	case jsIntArray:
		return []int{}
	case jsFloat:
		return 0.0
	case jsFloatArray:
		return []float64{}
	case jsString:
		return ""
	case jsStringArray:
		return []string{}
	case jsAnything:
		return ""
	case jsAnythingArray:
		return []interface{}{}
	default:
		panic(".zero(): unknown jsType! " + me)
	}
}

func (me jsType) any() jsType {
	switch me {
	case jsBool, jsInt, jsFloat, jsString, jsAnything:
		return jsAnything
	case jsBoolArray, jsIntArray, jsFloatArray, jsStringArray, jsAnythingArray:
		return jsAnythingArray
	default:
		panic(".any(): unknown jsType! " + me)
	}
}

func (me jsType) MarshalText() (text []byte, err error) {
	return []byte(me), nil
}

func (me *jsType) UnmarshalText(text []byte) error {
	// since jsAnything is "", this lets people manually specify "any" if they really want
	// unset jsType variables will be jsAnything by default
	if string(text) == "any" {
		*me = jsAnything
	} else {
		*me = jsType(text)
	}
	// TODO: return error if invalid?
	return nil
}

func (me jsType) String() string {
	return string(me)
}

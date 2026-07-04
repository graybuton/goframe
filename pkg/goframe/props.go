package goframe

import (
	"strconv"
	"strings"
)

// Props contains element attributes and event handlers.
type Props map[string]any

type domProp struct {
	value   string
	boolean bool
}

// ToString converts supported primitive values into text suitable for a Text
// node. Unsupported values become an empty string to keep the WASM runtime
// independent from fmt and reflection-heavy formatting.
func ToString(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case int:
		return strconv.Itoa(value)
	case int8:
		return strconv.FormatInt(int64(value), 10)
	case int16:
		return strconv.FormatInt(int64(value), 10)
	case int32:
		return strconv.FormatInt(int64(value), 10)
	case int64:
		return strconv.FormatInt(value, 10)
	case uint:
		return strconv.FormatUint(uint64(value), 10)
	case uint8:
		return strconv.FormatUint(uint64(value), 10)
	case uint16:
		return strconv.FormatUint(uint64(value), 10)
	case uint32:
		return strconv.FormatUint(uint64(value), 10)
	case uint64:
		return strconv.FormatUint(value, 10)
	case float32:
		return strconv.FormatFloat(float64(value), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(value)
	default:
		return ""
	}
}

func eventNameForProp(name string) (string, bool) {
	if len(name) <= 2 || !strings.EqualFold(name[:2], "on") {
		return "", false
	}
	return strings.ToLower(name[2:]), true
}

func normalizeAttributeName(name string) string {
	switch strings.ToLower(name) {
	case "class", "id", "value", "type", "placeholder", "name", "disabled",
		"checked", "selected", "href", "src", "alt", "title", "role":
		return strings.ToLower(name)
	case "classname":
		return "class"
	case "htmlfor":
		return "for"
	default:
		return name
	}
}

func splitProps(props Props) (map[string]domProp, map[string]any) {
	if len(props) == 0 {
		return nil, nil
	}

	var dom map[string]domProp
	var events map[string]any
	for name, value := range props {
		if value == nil {
			continue
		}
		if boolean, ok := value.(bool); ok && !boolean {
			continue
		}
		if eventName, ok := eventNameForProp(name); ok {
			if events == nil {
				events = make(map[string]any)
			}
			events[eventName] = value
			continue
		}
		name = normalizeAttributeName(name)
		if boolean, ok := value.(bool); ok {
			if boolean {
				if dom == nil {
					dom = make(map[string]domProp)
				}
				dom[name] = domProp{boolean: true}
			}
			continue
		}
		if dom == nil {
			dom = make(map[string]domProp)
		}
		dom[name] = domProp{value: ToString(value)}
	}
	return dom, events
}

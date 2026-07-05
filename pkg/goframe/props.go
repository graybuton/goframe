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

type splitDOMProp struct {
	name string
	prop domProp
}

type splitEventProp struct {
	name     string
	callback any
}

type splitDOMProps []splitDOMProp

type splitEventProps []splitEventProp

func (props splitDOMProps) get(name string) (domProp, bool) {
	for _, prop := range props {
		if prop.name == name {
			return prop.prop, true
		}
	}
	return domProp{}, false
}

func (props splitDOMProps) has(name string) bool {
	_, ok := props.get(name)
	return ok
}

func (props *splitDOMProps) set(name string, prop domProp) {
	for index := range *props {
		if (*props)[index].name == name {
			(*props)[index].prop = prop
			return
		}
	}
	*props = append(*props, splitDOMProp{name: name, prop: prop})
}

func (events splitEventProps) get(name string) (any, bool) {
	for _, event := range events {
		if event.name == name {
			return event.callback, true
		}
	}
	return nil, false
}

func (events *splitEventProps) set(name string, callback any) {
	for index := range *events {
		if (*events)[index].name == name {
			(*events)[index].callback = callback
			return
		}
	}
	*events = append(*events, splitEventProp{name: name, callback: callback})
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
	if len(name) <= 2 {
		return "", false
	}
	first, second := name[0], name[1]
	if (first != 'o' && first != 'O') || (second != 'n' && second != 'N') {
		return "", false
	}
	return strings.ToLower(name[2:]), true
}

func normalizeAttributeName(name string) string {
	lower := strings.ToLower(name)
	switch lower {
	case "class", "id", "value", "type", "placeholder", "name", "disabled",
		"checked", "selected", "href", "src", "alt", "title", "role":
		return lower
	case "classname":
		return "class"
	case "htmlfor":
		return "for"
	default:
		return name
	}
}

func splitProps(props Props) (splitDOMProps, splitEventProps) {
	if len(props) == 0 {
		return nil, nil
	}

	var dom splitDOMProps
	var events splitEventProps
	for name, value := range props {
		if value == nil {
			continue
		}
		if boolean, ok := value.(bool); ok && !boolean {
			continue
		}
		if eventName, ok := eventNameForProp(name); ok {
			if events == nil {
				events = make(splitEventProps, 0, len(props))
			}
			events.set(eventName, value)
			continue
		}
		name = normalizeAttributeName(name)
		if _, ok := value.(bool); ok {
			if dom == nil {
				dom = make(splitDOMProps, 0, len(props))
			}
			dom.set(name, domProp{boolean: true})
			continue
		}
		if dom == nil {
			dom = make(splitDOMProps, 0, len(props))
		}
		dom.set(name, domProp{value: ToString(value)})
	}
	return dom, events
}

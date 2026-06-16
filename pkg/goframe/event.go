package goframe

// Event is a small browser-event facade that keeps syscall/js out of
// application-facing APIs and host-platform builds.
type Event struct {
	preventDefault  func()
	stopPropagation func()
}

// PreventDefault asks the browser to cancel the event's default behavior.
func (event Event) PreventDefault() {
	if event.preventDefault != nil {
		event.preventDefault()
	}
}

// StopPropagation prevents the event from bubbling further.
func (event Event) StopPropagation() {
	if event.stopPropagation != nil {
		event.stopPropagation()
	}
}

// InputEvent exposes the current value of an input-like event target.
type InputEvent struct {
	Event
	value func() string
}

// Value returns the current value of the event target.
func (event InputEvent) Value() string {
	if event.value == nil {
		return ""
	}
	return event.value()
}

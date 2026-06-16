package goframe

import "testing"

func TestEventActionsAndInputValue(t *testing.T) {
	prevented := false
	stopped := false
	event := Event{
		preventDefault:  func() { prevented = true },
		stopPropagation: func() { stopped = true },
	}
	event.PreventDefault()
	event.StopPropagation()
	if !prevented || !stopped {
		t.Fatalf("event actions: prevented=%v stopped=%v", prevented, stopped)
	}

	input := InputEvent{Event: event, value: func() string { return "task" }}
	if got := input.Value(); got != "task" {
		t.Fatalf("InputEvent.Value() = %q, want task", got)
	}
	if got := (InputEvent{}).Value(); got != "" {
		t.Fatalf("empty InputEvent.Value() = %q, want empty", got)
	}
}

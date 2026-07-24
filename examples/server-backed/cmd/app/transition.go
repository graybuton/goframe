package main

import gf "github.com/graybuton/goframe/pkg/goframe"

type committedGreeting struct {
	Name    string
	Target  string
	Message string
	Ready   bool
}

func nextCommittedGreeting(
	current committedGreeting,
	requestedName string,
	requestedTarget string,
	resource gf.Resource[string],
) (committedGreeting, bool) {
	if !resource.Ready() {
		return current, false
	}
	next := committedGreeting{
		Name:    requestedName,
		Target:  requestedTarget,
		Message: resource.Value,
		Ready:   true,
	}
	if next == current {
		return current, false
	}
	return next, true
}

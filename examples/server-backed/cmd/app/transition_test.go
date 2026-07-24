package main

import (
	"errors"
	"testing"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

func TestNextCommittedGreetingIgnoresUnreadyResource(t *testing.T) {
	current := committedGreeting{
		Name:    "Ada",
		Target:  "/transition-greeting?name=Ada",
		Message: "Hello, Ada, from Go backend!",
		Ready:   true,
	}
	for _, resource := range []gf.Resource[string]{
		{Status: gf.ResourceLoading},
		{Status: gf.ResourceFailed, Err: errors.New("failed")},
	} {
		next, changed := nextCommittedGreeting(
			current,
			"Lin",
			"/transition-greeting?name=Lin",
			resource,
		)
		if changed {
			t.Fatalf("nextCommittedGreeting(%v) changed committed state", resource.Status)
		}
		if next != current {
			t.Fatalf("nextCommittedGreeting(%v) = %#v, want %#v", resource.Status, next, current)
		}
	}
}

func TestNextCommittedGreetingCommitsMatchingReadyPair(t *testing.T) {
	next, changed := nextCommittedGreeting(
		committedGreeting{},
		"Ada",
		"/transition-greeting?name=Ada",
		gf.Resource[string]{
			Status: gf.ResourceReady,
			Value:  "Hello, Ada, from Go backend!",
		},
	)
	if !changed {
		t.Fatal("nextCommittedGreeting did not commit ready resource")
	}
	want := committedGreeting{
		Name:    "Ada",
		Target:  "/transition-greeting?name=Ada",
		Message: "Hello, Ada, from Go backend!",
		Ready:   true,
	}
	if next != want {
		t.Fatalf("nextCommittedGreeting = %#v, want %#v", next, want)
	}

	repeated, changed := nextCommittedGreeting(
		next,
		"Ada",
		"/transition-greeting?name=Ada",
		gf.Resource[string]{
			Status: gf.ResourceReady,
			Value:  "Hello, Ada, from Go backend!",
		},
	)
	if changed {
		t.Fatal("repeated identical ready state changed committed snapshot")
	}
	if repeated != want {
		t.Fatalf("repeated ready state = %#v, want %#v", repeated, want)
	}
}

func TestNextCommittedGreetingReplacesPriorPairTogether(t *testing.T) {
	current := committedGreeting{
		Name:    "Ada",
		Target:  "/transition-greeting?name=Ada",
		Message: "Hello, Ada, from Go backend!",
		Ready:   true,
	}
	next, changed := nextCommittedGreeting(
		current,
		"Lin",
		"/transition-greeting?name=Lin",
		gf.Resource[string]{
			Status: gf.ResourceReady,
			Value:  "Hello, Lin, from Go backend!",
		},
	)
	if !changed {
		t.Fatal("newer ready target did not change committed snapshot")
	}
	want := committedGreeting{
		Name:    "Lin",
		Target:  "/transition-greeting?name=Lin",
		Message: "Hello, Lin, from Go backend!",
		Ready:   true,
	}
	if next != want {
		t.Fatalf("newer ready target = %#v, want %#v", next, want)
	}
}

package goframe

import "testing"

func TestComponentCreatesBoundary(t *testing.T) {
	render := func(value int) Node { return Text(ToString(value)) }
	node, ok := Component("Number", 42, render).(ComponentNode)
	if !ok {
		t.Fatal("Component did not return ComponentNode")
	}
	if node.Name != "Number" || node.Props != 42 {
		t.Fatalf("component = %#v", node)
	}
	if text := node.render().(TextNode); text.Value != "42" {
		t.Fatalf("rendered text = %q, want 42", text.Value)
	}
}

func TestDirectFunctionCallHasNoBoundary(t *testing.T) {
	render := func(value int) Node { return Text(ToString(value)) }
	if _, ok := render(1).(ComponentNode); ok {
		t.Fatal("direct function call unexpectedly created ComponentNode")
	}
	if _, ok := Component("Number", 1, render).(ComponentNode); !ok {
		t.Fatal("Component did not preserve boundary")
	}
}

func TestComponentIdentityUsesNameAndKey(t *testing.T) {
	render := func(value int) Node { return Text(ToString(value)) }
	first := Component("Number", 1, render)
	second := Component("Number", 2, render)
	other := Component("Other", 1, render)

	if !sameNodeIdentity(first, second) {
		t.Fatal("same component name should preserve identity")
	}
	if sameNodeIdentity(first, other) {
		t.Fatal("different component names should replace identity")
	}
	if matches := matchChildIndices([]string{"one"}, []string{"one"}); matches[0] != 0 {
		t.Fatalf("keyed component match = %v", matches)
	}
}

package goframe

import "testing"

func TestElBuildsTree(t *testing.T) {
	node := El("button", Props{"class": "primary"}, Text("Click"))

	element, ok := node.(VNode)
	if !ok {
		t.Fatalf("El returned %T, want VNode", node)
	}
	if element.Tag != "button" {
		t.Fatalf("Tag = %q, want button", element.Tag)
	}
	if got := element.Props["class"]; got != "primary" {
		t.Fatalf("class = %v, want primary", got)
	}
	if len(element.Children) != 1 {
		t.Fatalf("children = %d, want 1", len(element.Children))
	}
}

func TestToString(t *testing.T) {
	if got := ToString(42); got != "42" {
		t.Fatalf("ToString(42) = %q, want 42", got)
	}
	if got := ToString(true); got != "true" {
		t.Fatalf("ToString(true) = %q, want true", got)
	}
	if got := ToString(struct{}{}); got != "" {
		t.Fatalf("ToString(struct{}{}) = %q, want empty string", got)
	}
}

func TestDOMPropNormalization(t *testing.T) {
	for input, want := range map[string]string{
		"Value":       "value",
		"Placeholder": "placeholder",
		"Type":        "type",
		"ClassName":   "class",
		"htmlFor":     "for",
		"data-test":   "data-test",
	} {
		if got := normalizeAttributeName(input); got != want {
			t.Fatalf("normalizeAttributeName(%q) = %q, want %q", input, got, want)
		}
	}
	for input, want := range map[string]string{
		"onClick":  "click",
		"OnInput":  "input",
		"onChange": "change",
		"OnSubmit": "submit",
	} {
		got, ok := eventNameForProp(input)
		if !ok || got != want {
			t.Fatalf("eventNameForProp(%q) = %q, %v; want %q, true", input, got, ok, want)
		}
	}
	if _, ok := eventNameForProp("onclicked"); !ok {
		t.Fatal("onclicked should still be treated as a custom event prop")
	}
	if _, ok := eventNameForProp("class"); ok {
		t.Fatal("class should not be treated as an event prop")
	}
}

func TestFragmentAndChild(t *testing.T) {
	children := []Node{Text("one"), El("strong", nil, Text("two"))}

	fragment, ok := Fragment(children...).(FragmentNode)
	if !ok {
		t.Fatalf("Fragment returned unexpected type")
	}
	if len(fragment.Children) != 2 {
		t.Fatalf("fragment children = %d, want 2", len(fragment.Children))
	}

	childFragment, ok := Child(children).(FragmentNode)
	if !ok || len(childFragment.Children) != 2 {
		t.Fatalf("Child([]Node) = %#v, want two-child FragmentNode", childFragment)
	}
	if text, ok := Child(42).(TextNode); !ok || text.Value != "42" {
		t.Fatalf("Child(42) = %#v, want TextNode 42", text)
	}
	if _, ok := Child(nil).(EmptyNode); !ok {
		t.Fatalf("Child(nil) = %#v, want EmptyNode", Child(nil))
	}
}

func TestConditionalHelpers(t *testing.T) {
	node := Text("yes")
	if If(true, node) != node {
		t.Fatal("If(true) did not return its node")
	}
	if _, ok := If(false, node).(EmptyNode); !ok {
		t.Fatalf("If(false) = %#v, want EmptyNode", If(false, node))
	}
	if IfElse(true, node, Empty()) != node {
		t.Fatal("IfElse(true) did not return thenNode")
	}
	if IfElse(false, Empty(), node) != node {
		t.Fatal("IfElse(false) did not return elseNode")
	}
}

func TestListHelpers(t *testing.T) {
	items := []string{"a", "b"}
	nodes := For(items, func(item string) Node { return Text(item) })
	if len(nodes) != 2 || nodes[1].(TextNode).Value != "b" {
		t.Fatalf("For() = %#v", nodes)
	}

	indexed := ForIndexed(items, func(index int, item string) Node {
		return Text(ToString(index) + item)
	})
	if got := indexed[1].(TextNode).Value; got != "1b" {
		t.Fatalf("ForIndexed()[1] = %q, want 1b", got)
	}
}

func TestKeysRetainIdentityAndNode(t *testing.T) {
	node := Text("task")
	keyed, ok := Key("todo-1", node).(KeyedNode)
	if !ok || keyed.Key != "todo-1" || keyed.Node != node {
		t.Fatalf("Key() = %#v", keyed)
	}
	if got := WithKey(node, "todo-2").(KeyedNode).Key; got != "todo-2" {
		t.Fatalf("WithKey key = %q, want todo-2", got)
	}
}

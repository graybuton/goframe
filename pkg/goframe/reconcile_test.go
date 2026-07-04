package goframe

import (
	"reflect"
	"testing"
)

func TestUnwrapNodeRetainsOutermostKey(t *testing.T) {
	node := Key("outer", Key("inner", Text("value")))
	key, inner := unwrapNode(node)
	if key != "outer" {
		t.Fatalf("key = %q, want outer", key)
	}
	if text, ok := inner.(TextNode); !ok || text.Value != "value" {
		t.Fatalf("inner = %#v, want TextNode value", inner)
	}
}

func TestSameNodeIdentity(t *testing.T) {
	tests := []struct {
		name string
		old  Node
		new  Node
		want bool
	}{
		{"same element tag", El("div", nil), El("div", Props{"class": "new"}), true},
		{"different element tag", El("p", nil), El("div", nil), false},
		{"text values", Text("old"), Text("new"), true},
		{"fragments", Fragment(Text("old")), Fragment(Text("new")), true},
		{"empty", Empty(), Empty(), true},
		{"same component", Component("One", 1, func(int) Node { return Empty() }), Component("One", 2, func(int) Node { return Empty() }), true},
		{"different component", Component("One", 1, func(int) Node { return Empty() }), Component("Two", 1, func(int) Node { return Empty() }), false},
		{"different kinds", Text("old"), El("span", nil), false},
		{"keys do not change node kind", Key("old", Text("x")), Key("new", Text("y")), true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sameNodeIdentity(test.old, test.new); got != test.want {
				t.Fatalf("sameNodeIdentity() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestMatchChildIndices(t *testing.T) {
	tests := []struct {
		name string
		old  []string
		new  []string
		want []int
	}{
		{
			name: "unkeyed positional append",
			old:  []string{"", ""},
			new:  []string{"", "", ""},
			want: []int{0, 1, noChildMatch},
		},
		{
			name: "stable keyed order",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"a", "b", "c", "d"},
			want: []int{0, 1, 2, 3},
		},
		{
			name: "reverse keyed order",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"d", "c", "b", "a"},
			want: []int{3, 2, 1, 0},
		},
		{
			name: "rotate keyed order left",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"b", "c", "d", "a"},
			want: []int{1, 2, 3, 0},
		},
		{
			name: "rotate keyed order right",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"d", "a", "b", "c"},
			want: []int{3, 0, 1, 2},
		},
		{
			name: "keyed reorder and insert",
			old:  []string{"a", "b", "c"},
			new:  []string{"c", "b", "d", "a"},
			want: []int{2, 1, noChildMatch, 0},
		},
		{
			name: "insert around stable keyed children",
			old:  []string{"a", "b", "c"},
			new:  []string{"x", "a", "b", "c", "y"},
			want: []int{noChildMatch, 0, 1, 2, noChildMatch},
		},
		{
			name: "keyed removal keeps surviving identity",
			old:  []string{"todo-1", "todo-2"},
			new:  []string{"todo-2"},
			want: []int{1},
		},
		{
			name: "remove keyed children",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"a", "c"},
			want: []int{0, 2},
		},
		{
			name: "mixed children keep unkeyed order",
			old:  []string{"a", "", "b", ""},
			new:  []string{"b", "", "a", ""},
			want: []int{2, 1, 0, 3},
		},
		{
			name: "duplicate new key mounts duplicate",
			old:  []string{"a"},
			new:  []string{"a", "a"},
			want: []int{0, noChildMatch},
		},
		{
			name: "duplicate old key single new key",
			old:  []string{"a", "a", "b"},
			new:  []string{"a", "b"},
			want: []int{0, 2},
		},
		{
			name: "duplicate old key duplicate new key",
			old:  []string{"a", "a"},
			new:  []string{"a", "a"},
			want: []int{0, noChildMatch},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := matchChildIndices(test.old, test.new); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("matchChildIndices() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestConditionalSiblingPatchDecisionsPreserveStableNodes(t *testing.T) {
	oldNodes := []Node{
		If(true, El("p", Props{"class": "summary"}, Child(2), Text(" task(s)"))),
		If(true, Component("Button", struct{}{}, func(struct{}) Node { return El("button", nil) })),
		Component("TodoList", struct{}{}, func(struct{}) Node { return El("ul", nil) }),
	}
	newNodes := []Node{
		If(true, El("p", Props{"class": "summary"}, Child(1), Text(" task(s)"))),
		If(false, Component("Button", struct{}{}, func(struct{}) Node { return El("button", nil) })),
		Component("TodoList", struct{}{}, func(struct{}) Node { return El("ul", nil) }),
	}

	oldKeys := make([]string, len(oldNodes))
	for index, node := range oldNodes {
		oldKeys[index], oldNodes[index] = unwrapNode(node)
	}
	newKeys := make([]string, len(newNodes))
	for index, node := range newNodes {
		newKeys[index], newNodes[index] = unwrapNode(node)
	}

	matches := matchChildIndices(oldKeys, newKeys)
	if !reflect.DeepEqual(matches, []int{0, 1, 2}) {
		t.Fatalf("matches = %v, want [0 1 2]", matches)
	}
	if !sameNodeIdentity(oldNodes[matches[0]], newNodes[0]) {
		t.Fatal("summary true->true should patch the existing <p>")
	}
	if sameNodeIdentity(oldNodes[matches[1]], newNodes[1]) {
		t.Fatal("reverse button true->false should replace with empty placeholder")
	}
	if !sameNodeIdentity(oldNodes[matches[2]], newNodes[2]) {
		t.Fatal("TodoList true->true should patch the existing component")
	}
}

func TestSplitPropsNormalizesDOMAndEvents(t *testing.T) {
	dom, events := splitProps(Props{
		"ClassName":   "primary",
		"Value":       "task",
		"Disabled":    true,
		"Placeholder": false,
		"OnClick":     func() {},
		"onInput":     func(InputEvent) {},
	})

	if got, ok := dom.get("class"); !ok || got != (domProp{value: "primary"}) {
		t.Fatalf("class = %#v, %v", got, ok)
	}
	if got, ok := dom.get("value"); !ok || got != (domProp{value: "task"}) {
		t.Fatalf("value = %#v, %v", got, ok)
	}
	if got, ok := dom.get("disabled"); !ok || got != (domProp{boolean: true}) {
		t.Fatalf("disabled = %#v, %v", got, ok)
	}
	if dom.has("placeholder") {
		t.Fatal("false placeholder should be absent")
	}
	click, clickExists := events.get("click")
	input, inputExists := events.get("input")
	if len(events) != 2 || !clickExists || click == nil || !inputExists || input == nil {
		t.Fatalf("events = %#v", events)
	}
}

func TestSplitPropsHandlesUncomparableValues(t *testing.T) {
	dom, events := splitProps(Props{
		"data-slice": []string{"not", "stringifiable"},
		"data-map":   map[string]string{"a": "b"},
		"Hidden":     false,
	})

	if len(events) != 0 {
		t.Fatalf("events = %#v, want none", events)
	}
	if got, ok := dom.get("data-slice"); !ok || got != (domProp{}) {
		t.Fatalf("data-slice = %#v, %v; want empty string prop", got, ok)
	}
	if got, ok := dom.get("data-map"); !ok || got != (domProp{}) {
		t.Fatalf("data-map = %#v, %v; want empty string prop", got, ok)
	}
	if dom.has("hidden") {
		t.Fatal("false prop should be absent")
	}
}

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
			name: "keyed reorder and insert",
			old:  []string{"a", "b", "c"},
			new:  []string{"c", "b", "d", "a"},
			want: []int{2, 1, noChildMatch, 0},
		},
		{
			name: "keyed removal keeps surviving identity",
			old:  []string{"todo-1", "todo-2"},
			new:  []string{"todo-2"},
			want: []int{1},
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
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := matchChildIndices(test.old, test.new); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("matchChildIndices() = %v, want %v", got, test.want)
			}
		})
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

	if got := dom["class"]; got != (domProp{value: "primary"}) {
		t.Fatalf("class = %#v", got)
	}
	if got := dom["value"]; got != (domProp{value: "task"}) {
		t.Fatalf("value = %#v", got)
	}
	if got := dom["disabled"]; got != (domProp{boolean: true}) {
		t.Fatalf("disabled = %#v", got)
	}
	if _, exists := dom["placeholder"]; exists {
		t.Fatal("false placeholder should be absent")
	}
	if len(events) != 2 || events["click"] == nil || events["input"] == nil {
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
	if got := dom["data-slice"]; got != (domProp{}) {
		t.Fatalf("data-slice = %#v, want empty string prop", got)
	}
	if got := dom["data-map"]; got != (domProp{}) {
		t.Fatalf("data-map = %#v, want empty string prop", got)
	}
	if _, exists := dom["hidden"]; exists {
		t.Fatal("false prop should be absent")
	}
}

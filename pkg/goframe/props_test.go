package goframe

import "testing"

var benchmarkSplitPropsDOM map[string]domProp
var benchmarkSplitPropsEvents map[string]any

func TestSplitPropsContract(t *testing.T) {
	clickHandler := &struct{ name string }{"click"}
	inputHandler := &struct{ name string }{"input"}

	dom, events := splitProps(Props{
		"id":                "issue-1",
		"value":             "open",
		"name":              "status",
		"href":              "/issues/1",
		"data-custom":       "kept",
		"className":         "issue-row",
		"htmlFor":           "status-input",
		"OnClick":           clickHandler,
		"onInput":           inputHandler,
		"OnChange":          false,
		"disabled":          true,
		"checked":           false,
		"data-nil":          nil,
		"data-count":        42,
		"data-ratio":        3.5,
		"aria-expanded":     true,
		"data-unsupported":  struct{ label string }{"unsupported"},
		"data-bool-skipped": false,
	})

	wantDOM := map[string]domProp{
		"id":               {value: "issue-1"},
		"value":            {value: "open"},
		"name":             {value: "status"},
		"href":             {value: "/issues/1"},
		"data-custom":      {value: "kept"},
		"class":            {value: "issue-row"},
		"for":              {value: "status-input"},
		"disabled":         {boolean: true},
		"data-count":       {value: "42"},
		"data-ratio":       {value: "3.5"},
		"aria-expanded":    {boolean: true},
		"data-unsupported": {},
	}
	if len(dom) != len(wantDOM) {
		t.Fatalf("dom length = %d, want %d: %#v", len(dom), len(wantDOM), dom)
	}
	for name, want := range wantDOM {
		if got, ok := dom[name]; !ok || got != want {
			t.Fatalf("dom[%q] = %#v, %v; want %#v, true", name, got, ok, want)
		}
	}
	for _, name := range []string{"checked", "data-nil", "data-bool-skipped"} {
		if _, exists := dom[name]; exists {
			t.Fatalf("dom[%q] should be absent", name)
		}
	}

	if len(events) != 2 {
		t.Fatalf("events length = %d, want 2: %#v", len(events), events)
	}
	if got := events["click"]; got != clickHandler {
		t.Fatalf("events[click] = %#v, want original click handler", got)
	}
	if got := events["input"]; got != inputHandler {
		t.Fatalf("events[input] = %#v, want original input handler", got)
	}
	if _, exists := events["change"]; exists {
		t.Fatalf("events[change] should be absent")
	}
}

func TestSplitPropsEmptyProps(t *testing.T) {
	for _, test := range []struct {
		name  string
		props Props
	}{
		{name: "empty", props: Props{}},
		{name: "nil", props: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			dom, events := splitProps(test.props)
			if len(dom) != 0 {
				t.Fatalf("dom = %#v, want empty", dom)
			}
			if len(events) != 0 {
				t.Fatalf("events = %#v, want empty", events)
			}
		})
	}
}

func BenchmarkSplitPropsAllocations(b *testing.B) {
	tests := []struct {
		name       string
		props      Props
		wantDOM    int
		wantEvents int
	}{
		{
			name:       "empty_props",
			props:      Props{},
			wantDOM:    0,
			wantEvents: 0,
		},
		{
			name:       "nil_props",
			props:      nil,
			wantDOM:    0,
			wantEvents: 0,
		},
		{
			name: "small_static_element",
			props: Props{
				"id":        "summary",
				"className": "card",
				"title":     "Summary",
				"role":      "region",
			},
			wantDOM:    4,
			wantEvents: 0,
		},
		{
			name: "input_like_props",
			props: Props{
				"id":          "issue-filter",
				"name":        "filter",
				"value":       "open",
				"placeholder": "Filter issues",
				"disabled":    false,
				"OnInput":     func(InputEvent) {},
				"OnChange":    func(InputEvent) {},
			},
			wantDOM:    4,
			wantEvents: 2,
		},
		{
			name: "button_like_props",
			props: Props{
				"className": "button primary",
				"type":      "button",
				"disabled":  true,
				"OnClick":   func(Event) {},
			},
			wantDOM:    3,
			wantEvents: 1,
		},
		{
			name: "mixed_dashboard_row_props",
			props: Props{
				"id":            "issue-row-42",
				"className":     "issue-row selected",
				"data-testid":   "issue-row-42",
				"style":         "height:32px",
				"aria-rowindex": 42,
				"hidden":        false,
				"selected":      true,
				"OnClick":       func(Event) {},
				"OnMouseEnter":  func(Event) {},
			},
			wantDOM:    6,
			wantEvents: 2,
		},
	}

	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {
			dom, events := splitProps(test.props)
			if len(dom) != test.wantDOM || len(events) != test.wantEvents {
				b.Fatalf("sanity splitProps() dom=%d events=%d, want dom=%d events=%d", len(dom), len(events), test.wantDOM, test.wantEvents)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchmarkSplitPropsDOM, benchmarkSplitPropsEvents = splitProps(test.props)
			}
		})
	}
}

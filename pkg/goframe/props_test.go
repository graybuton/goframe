package goframe

import "testing"

var benchmarkSplitPropsDOM splitDOMProps
var benchmarkSplitPropsEvents splitEventProps

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
		"aria-hidden":       false,
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
		"aria-expanded":    {value: "true"},
		"aria-hidden":      {value: "false"},
		"data-unsupported": {},
	}
	if len(dom) != len(wantDOM) {
		t.Fatalf("dom length = %d, want %d: %#v", len(dom), len(wantDOM), dom)
	}
	for name, want := range wantDOM {
		if got, ok := dom.get(name); !ok || got != want {
			t.Fatalf("dom[%q] = %#v, %v; want %#v, true", name, got, ok, want)
		}
	}
	for _, name := range []string{"checked", "data-nil", "data-bool-skipped"} {
		if dom.has(name) {
			t.Fatalf("dom[%q] should be absent", name)
		}
	}

	if len(events) != 2 {
		t.Fatalf("events length = %d, want 2: %#v", len(events), events)
	}
	if got, ok := events.get("click"); !ok || got != clickHandler {
		t.Fatalf("events[click] = %#v, want original click handler", got)
	}
	if got, ok := events.get("input"); !ok || got != inputHandler {
		t.Fatalf("events[input] = %#v, want original input handler", got)
	}
	if _, exists := events.get("change"); exists {
		t.Fatalf("events[change] should be absent")
	}
}

func TestSplitPropsSerializesARIABooleanValues(t *testing.T) {
	tests := []struct {
		name  string
		prop  string
		value bool
		want  string
	}{
		{name: "expanded true", prop: "aria-expanded", value: true, want: "true"},
		{name: "expanded false", prop: "aria-expanded", value: false, want: "false"},
		{name: "hidden true", prop: "aria-hidden", value: true, want: "true"},
		{name: "hidden false", prop: "aria-hidden", value: false, want: "false"},
		{name: "invalid true", prop: "aria-invalid", value: true, want: "true"},
		{name: "invalid false", prop: "aria-invalid", value: false, want: "false"},
		{name: "upper prefix", prop: "ARIA-expanded", value: false, want: "false"},
		{name: "title prefix", prop: "Aria-hidden", value: true, want: "true"},
		{name: "mixed prefix", prop: "aRiA-invalid", value: false, want: "false"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dom, events := splitProps(Props{test.prop: test.value})
			if len(dom) != 1 {
				t.Fatalf("dom = %#v, want exactly one prop", dom)
			}
			if len(events) != 0 {
				t.Fatalf("events = %#v, want empty", events)
			}
			got, ok := dom.get(test.prop)
			if !ok {
				t.Fatalf("dom[%q] absent; dom = %#v", test.prop, dom)
			}
			if got != (domProp{value: test.want}) {
				t.Fatalf("dom[%q] = %#v, want %#v", test.prop, got, domProp{value: test.want})
			}
			if got.boolean {
				t.Fatalf("dom[%q].boolean = true, want false", test.prop)
			}
		})
	}
}

func TestSplitPropsARIAPrefixBoundariesUseGenericBooleanSemantics(t *testing.T) {
	for _, prop := range []string{"aria", "aria-", "ariax-expanded", "xaria-expanded"} {
		t.Run(prop+" true", func(t *testing.T) {
			dom, events := splitProps(Props{prop: true})
			if len(events) != 0 {
				t.Fatalf("events = %#v, want empty", events)
			}
			if got, ok := dom.get(prop); !ok || got != (domProp{boolean: true}) {
				t.Fatalf("dom[%q] = %#v, %v; want presence boolean", prop, got, ok)
			}
		})

		t.Run(prop+" false", func(t *testing.T) {
			dom, events := splitProps(Props{prop: false})
			if len(dom) != 0 {
				t.Fatalf("dom = %#v, want empty", dom)
			}
			if len(events) != 0 {
				t.Fatalf("events = %#v, want empty", events)
			}
		})
	}
}

func TestSplitPropsPreservesNonARIABooleanSemantics(t *testing.T) {
	for _, prop := range []string{"disabled", "checked", "selected", "hidden", "data-flag", "custom-flag"} {
		t.Run(prop+" true", func(t *testing.T) {
			dom, events := splitProps(Props{prop: true})
			if len(events) != 0 {
				t.Fatalf("events = %#v, want empty", events)
			}
			if got, ok := dom.get(prop); !ok || got != (domProp{boolean: true}) {
				t.Fatalf("dom[%q] = %#v, %v; want presence boolean", prop, got, ok)
			}
		})

		t.Run(prop+" false", func(t *testing.T) {
			dom, events := splitProps(Props{prop: false})
			if len(dom) != 0 {
				t.Fatalf("dom = %#v, want empty", dom)
			}
			if len(events) != 0 {
				t.Fatalf("events = %#v, want empty", events)
			}
		})
	}

	dom, events := splitProps(Props{"OnChange": false})
	if len(dom) != 0 || len(events) != 0 {
		t.Fatalf("false event split = %#v, %#v; want empty", dom, events)
	}
}

func TestSplitPropsPreservesNonBooleanARIAValues(t *testing.T) {
	dom, events := splitProps(Props{
		"aria-current":  "page",
		"aria-checked":  "mixed",
		"aria-rowindex": 3,
		"aria-label":    "Save",
	})

	want := map[string]domProp{
		"aria-current":  {value: "page"},
		"aria-checked":  {value: "mixed"},
		"aria-rowindex": {value: "3"},
		"aria-label":    {value: "Save"},
	}
	if len(dom) != len(want) {
		t.Fatalf("dom = %#v, want %d props", dom, len(want))
	}
	if len(events) != 0 {
		t.Fatalf("events = %#v, want empty", events)
	}
	for name, wantProp := range want {
		if got, ok := dom.get(name); !ok || got != wantProp {
			t.Fatalf("dom[%q] = %#v, %v; want %#v, true", name, got, ok, wantProp)
		}
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

func TestSplitPropsDeduplicatesNormalizedNames(t *testing.T) {
	clickHandler := &struct{ name string }{"click"}
	lowerClickHandler := &struct{ name string }{"lower-click"}

	dom, events := splitProps(Props{
		"class":     "base",
		"className": "primary",
		"OnClick":   clickHandler,
		"onclick":   lowerClickHandler,
	})

	if len(dom) != 1 {
		t.Fatalf("dom length = %d, want 1: %#v", len(dom), dom)
	}
	if got, ok := dom.get("class"); !ok || (got != (domProp{value: "base"}) && got != (domProp{value: "primary"})) {
		t.Fatalf("dom[class] = %#v, %v; want one normalized class prop", got, ok)
	}

	if len(events) != 1 {
		t.Fatalf("events length = %d, want 1: %#v", len(events), events)
	}
	if got, ok := events.get("click"); !ok || (got != any(clickHandler) && got != any(lowerClickHandler)) {
		t.Fatalf("events[click] = %#v, %v; want one normalized click callback", got, ok)
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

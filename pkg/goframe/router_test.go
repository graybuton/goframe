package goframe

import "testing"

func TestNormalizeRouteTarget(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "", want: "/"},
		{input: "#", want: "/"},
		{input: "#/", want: "/"},
		{input: "#/issues", want: "/issues"},
		{input: "/issues/", want: "/issues"},
		{input: "issues/42", want: "/issues/42"},
		{input: "#/issues/42?tab=comments", want: "/issues/42?tab=comments"},
	}
	for _, test := range tests {
		if got := normalizeRouteTarget(test.input); got != test.want {
			t.Fatalf("normalizeRouteTarget(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestRouteMatchingStaticPath(t *testing.T) {
	match, ok := matchRouterTarget([]Route{
		RoutePath("/", routeTestNode("home")),
		RoutePath("/issues", routeTestNode("issues")),
	}, "#/issues")

	if !ok {
		t.Fatal("expected route match")
	}
	if match.context.Path != "/issues" || match.context.Pattern != "/issues" {
		t.Fatalf("context = %#v", match.context)
	}
	if got := match.handler(match.context).(TextNode).Value; got != "issues" {
		t.Fatalf("handler text = %q, want issues", got)
	}
}

func TestRouteMatchingParams(t *testing.T) {
	match, ok := matchRouterTarget([]Route{
		RoutePath("/projects/:projectID/issues/:issueID", routeTestNode("details")),
	}, "#/projects/p1/issues/42?tab=comments")

	if !ok {
		t.Fatal("expected route match")
	}
	if got := match.context.Param("projectID"); got != "p1" {
		t.Fatalf("projectID = %q, want p1", got)
	}
	if got := match.context.Param("issueID"); got != "42" {
		t.Fatalf("issueID = %q, want 42", got)
	}
	if got := match.context.Param("missing"); got != "" {
		t.Fatalf("missing param = %q, want empty", got)
	}
	if match.context.RawQuery != "tab=comments" {
		t.Fatalf("raw query = %q, want tab=comments", match.context.RawQuery)
	}
}

func TestRouteMatchingDeclarationOrderWins(t *testing.T) {
	match, ok := matchRouterTarget([]Route{
		RoutePath("/issues/new", routeTestNode("new")),
		RoutePath("/issues/:id", routeTestNode("details")),
	}, "/issues/new")

	if !ok {
		t.Fatal("expected route match")
	}
	if got := match.handler(match.context).(TextNode).Value; got != "new" {
		t.Fatalf("matched handler = %q, want new", got)
	}
}

func TestNotFoundRouteHandlesUnmatchedPath(t *testing.T) {
	match, ok := matchRouterTarget([]Route{
		RoutePath("/", routeTestNode("home")),
		NotFoundRoute(routeTestNode("missing")),
	}, "#/missing")

	if !ok {
		t.Fatal("expected not-found match")
	}
	if match.context.Path != "/missing" || match.context.Pattern != "" {
		t.Fatalf("not-found context = %#v", match.context)
	}
	if got := match.handler(match.context).(TextNode).Value; got != "missing" {
		t.Fatalf("not-found handler = %q, want missing", got)
	}
}

func TestNoMatchWithoutNotFoundReturnsFalse(t *testing.T) {
	if _, ok := matchRouterTarget([]Route{
		RoutePath("/", routeTestNode("home")),
	}, "/missing"); ok {
		t.Fatal("unexpected route match")
	}
}

func TestHashHref(t *testing.T) {
	tests := []struct {
		to   string
		want string
	}{
		{to: "", want: "#/"},
		{to: "/", want: "#/"},
		{to: "/issues", want: "#/issues"},
		{to: "issues/42", want: "#/issues/42"},
		{to: "#/issues/42/", want: "#/issues/42"},
		{to: "/issues/42?tab=comments", want: "#/issues/42?tab=comments"},
	}
	for _, test := range tests {
		if got := HashHref(test.to); got != test.want {
			t.Fatalf("HashHref(%q) = %q, want %q", test.to, got, test.want)
		}
	}
}

func TestRoutePathValidation(t *testing.T) {
	assertPanic(t, "goframe: RoutePath requires a handler", func() {
		RoutePath("/", nil)
	})
	assertPanic(t, "goframe: route pattern must not be empty", func() {
		RoutePath("", routeTestNode("empty"))
	})
	assertPanic(t, "goframe: route parameter name must not be empty", func() {
		RoutePath("/:id/:", routeTestNode("bad"))
	})
}

func TestNotFoundRouteValidation(t *testing.T) {
	assertPanic(t, "goframe: NotFoundRoute requires a handler", func() {
		NotFoundRoute(nil)
	})
}

func TestRouterLinkRendersHashAnchor(t *testing.T) {
	node := RouterLink(RouterLinkProps{
		To:       "/issues",
		Class:    "nav-link",
		TestID:   "issues-link",
		Children: []Node{Text("Issues")},
	}).(VNode)

	if node.Tag != "a" {
		t.Fatalf("tag = %q, want a", node.Tag)
	}
	if node.Props["href"] != "#/issues" || node.Props["class"] != "nav-link" || node.Props["data-testid"] != "issues-link" {
		t.Fatalf("props = %#v", node.Props)
	}
	if got := node.Children[0].(TextNode).Value; got != "Issues" {
		t.Fatalf("child = %q, want Issues", got)
	}
}

func TestRouterViewCreatesRouteBoundaryKeyedByPattern(t *testing.T) {
	router := NewHashRouter([]Route{
		RoutePath("/", routeTestNode("home")),
	})
	node := RouterView(router).(ComponentNode)
	if node.Name != "RouterView" {
		t.Fatalf("RouterView component name = %q", node.Name)
	}

	instance := newComponentInstance(node, "", nil, nil)
	rendered := renderComponentInstance(instance)
	keyed := requireKeyedNode(t, rendered)
	if keyed.Key != "route:/" {
		t.Fatalf("route key = %q, want route:/", keyed.Key)
	}
	routeNode := requireComponentNode(t, keyed.Node)
	if routeNode.Name != "RouterRoute" {
		t.Fatalf("route component = %q, want RouterRoute", routeNode.Name)
	}
}

func TestRouteContextParamHandlesNilParams(t *testing.T) {
	if got := (RouteContext{}).Param("id"); got != "" {
		t.Fatalf("nil params Param = %q, want empty", got)
	}
}

func routeTestNode(label string) RouteHandler {
	return func(RouteContext) Node {
		return Text(label)
	}
}

func requireComponentNode(t *testing.T, node Node) ComponentNode {
	t.Helper()
	component, ok := node.(ComponentNode)
	if !ok {
		t.Fatalf("node = %#v, want ComponentNode", node)
	}
	return component
}

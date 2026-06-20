package goframe

import "strings"

// RouteContext describes the currently matched route.
type RouteContext struct {
	Path     string
	Pattern  string
	Params   map[string]string
	RawQuery string
}

// Param returns a route parameter by name, or an empty string when absent.
func (ctx RouteContext) Param(name string) string {
	if ctx.Params == nil {
		return ""
	}
	return ctx.Params[name]
}

// RouteHandler renders one matched route.
type RouteHandler func(RouteContext) Node

// Route describes one router entry.
type Route struct {
	pattern  string
	segments []routeSegment
	handler  RouteHandler
	notFound bool
}

type routeSegment struct {
	literal string
	param   string
}

// RoutePath creates an exact or parameterized route.
func RoutePath(pattern string, handler RouteHandler) Route {
	if handler == nil {
		panic("goframe: RoutePath requires a handler")
	}
	pattern = normalizeRoutePattern(pattern)
	return Route{
		pattern:  pattern,
		segments: parseRoutePattern(pattern),
		handler:  handler,
	}
}

// NotFoundRoute creates a fallback route for unmatched paths.
func NotFoundRoute(handler RouteHandler) Route {
	if handler == nil {
		panic("goframe: NotFoundRoute requires a handler")
	}
	return Route{
		handler:  handler,
		notFound: true,
	}
}

// Router stores a route table.
type Router struct {
	routes []Route
}

// NewHashRouter creates a hash-based browser router.
func NewHashRouter(routes []Route) *Router {
	copied := make([]Route, len(routes))
	copy(copied, routes)
	return &Router{routes: copied}
}

// RouterLinkProps configures a hash router anchor.
type RouterLinkProps struct {
	To       string
	Children []Node
	Class    string
	TestID   string
}

var (
	routerViewComponentType  = NewComponentType("goframe.RouterView", "RouterView")
	routerRouteComponentType = NewComponentType("goframe.RouterRoute", "RouterRoute")
)

// RouterLink renders an anchor using hash router href semantics.
func RouterLink(props RouterLinkProps) Node {
	linkProps := Props{
		"href": HashHref(props.To),
	}
	if props.Class != "" {
		linkProps["class"] = props.Class
	}
	if props.TestID != "" {
		linkProps["data-testid"] = props.TestID
	}
	return El("a", linkProps, props.Children...)
}

// RouterView renders the current route for a hash router.
func RouterView(router *Router) Node {
	return ComponentT(routerViewComponentType, routerViewProps{router: router}, renderRouterView)
}

type routerViewProps struct {
	router *Router
}

type routerRouteProps struct {
	handler RouteHandler
	context RouteContext
}

func renderRouterView(props routerViewProps) Node {
	if props.router == nil {
		panic("goframe: RouterView requires router")
	}
	current, setCurrent := UseState(routerCurrentTarget())
	UseEffect(func() Cleanup {
		return routerSubscribeHashChange(func(next string) {
			setCurrent(next)
		})
	})

	match, ok := matchRouterTarget(props.router.routes, current)
	if !ok {
		return Empty()
	}
	return Key(match.key, ComponentT(routerRouteComponentType, routerRouteProps{
		handler: match.handler,
		context: match.context,
	}, renderRouterRoute))
}

func renderRouterRoute(props routerRouteProps) Node {
	return props.handler(props.context)
}

type routeMatch struct {
	key     string
	handler RouteHandler
	context RouteContext
}

func matchRouterTarget(routes []Route, target string) (routeMatch, bool) {
	path, rawQuery := splitRouteTarget(normalizeRouteTarget(target))
	var fallback *Route
	for index := range routes {
		route := &routes[index]
		if route.notFound {
			if fallback == nil {
				fallback = route
			}
			continue
		}
		params, ok := matchRouteSegments(route.segments, path)
		if !ok {
			continue
		}
		return routeMatch{
			key:     "route:" + route.pattern,
			handler: route.handler,
			context: RouteContext{
				Path:     path,
				Pattern:  route.pattern,
				Params:   params,
				RawQuery: rawQuery,
			},
		}, true
	}
	if fallback == nil {
		return routeMatch{}, false
	}
	return routeMatch{
		key:     "route:not-found",
		handler: fallback.handler,
		context: RouteContext{
			Path:     path,
			RawQuery: rawQuery,
		},
	}, true
}

// HashHref converts a route path to a hash href.
func HashHref(to string) string {
	return "#" + normalizeRouteTarget(to)
}

func normalizeRoutePattern(pattern string) string {
	if pattern == "" {
		panic("goframe: route pattern must not be empty")
	}
	pattern, _ = splitRouteTarget(normalizeRouteTarget(pattern))
	return pattern
}

func parseRoutePattern(pattern string) []routeSegment {
	parts := routeParts(pattern)
	segments := make([]routeSegment, len(parts))
	for index, part := range parts {
		if strings.HasPrefix(part, ":") {
			name := part[1:]
			if name == "" {
				panic("goframe: route parameter name must not be empty")
			}
			segments[index] = routeSegment{param: name}
			continue
		}
		segments[index] = routeSegment{literal: part}
	}
	return segments
}

func matchRouteSegments(pattern []routeSegment, path string) (map[string]string, bool) {
	parts := routeParts(path)
	if len(pattern) != len(parts) {
		return nil, false
	}
	var params map[string]string
	for index, segment := range pattern {
		part := parts[index]
		if segment.param != "" {
			if part == "" {
				return nil, false
			}
			if params == nil {
				params = make(map[string]string)
			}
			params[segment.param] = part
			continue
		}
		if segment.literal != part {
			return nil, false
		}
	}
	if params == nil {
		params = map[string]string{}
	}
	return params, true
}

func normalizeRouteTarget(target string) string {
	if target == "" || target == "#" {
		return "/"
	}
	if strings.HasPrefix(target, "#") {
		target = target[1:]
	}
	if target == "" {
		return "/"
	}
	if !strings.HasPrefix(target, "/") {
		target = "/" + target
	}
	path, rawQuery := splitRouteTarget(target)
	path = trimTrailingRouteSlash(path)
	if rawQuery != "" {
		return path + "?" + rawQuery
	}
	return path
}

func splitRouteTarget(target string) (path string, rawQuery string) {
	if target == "" {
		return "/", ""
	}
	index := strings.IndexByte(target, '?')
	if index < 0 {
		return target, ""
	}
	return target[:index], target[index+1:]
}

func trimTrailingRouteSlash(path string) string {
	for len(path) > 1 && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	if path == "" {
		return "/"
	}
	return path
}

func routeParts(path string) []string {
	path = trimTrailingRouteSlash(path)
	if path == "/" {
		return nil
	}
	return strings.Split(strings.TrimPrefix(path, "/"), "/")
}

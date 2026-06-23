package goframe

import "strings"

// QueryValues stores parsed route query values. Repeated query keys are kept
// in insertion order for that key.
type QueryValues map[string][]string

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

// Query parses RawQuery into route query values.
func (ctx RouteContext) Query() QueryValues {
	return ParseQuery(ctx.RawQuery)
}

// Get returns the first query value for name, or an empty string when absent.
func (values QueryValues) Get(name string) string {
	if values == nil {
		return ""
	}
	items := values[name]
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

// Has reports whether name is present in the query.
func (values QueryValues) Has(name string) bool {
	if values == nil {
		return false
	}
	_, ok := values[name]
	return ok
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

// WithQuery returns path with query values applied. Existing query text on path
// is replaced.
func WithQuery(path string, values QueryValues) string {
	path, _ = splitRouteTarget(normalizeRouteTarget(path))
	encoded := values.Encode()
	if encoded == "" {
		return path
	}
	return path + "?" + encoded
}

// ParseQuery parses raw route query text into QueryValues. Malformed percent
// escapes are preserved literally rather than panicking.
func ParseQuery(raw string) QueryValues {
	values := QueryValues{}
	if raw == "" {
		return values
	}
	parts := strings.Split(raw, "&")
	for _, part := range parts {
		if part == "" {
			continue
		}
		name := part
		value := ""
		hasValue := false
		if index := strings.IndexByte(part, '='); index >= 0 {
			name = part[:index]
			value = part[index+1:]
			hasValue = true
		}
		name = decodeQueryComponent(name)
		if name == "" {
			continue
		}
		if hasValue {
			value = decodeQueryComponent(value)
			values[name] = append(values[name], value)
		} else {
			values[name] = append(values[name], "")
		}
	}
	return values
}

// Encode serializes query values with deterministic key ordering.
func (values QueryValues) Encode() string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		if key != "" {
			keys = append(keys, key)
		}
	}
	sortRouteQueryKeys(keys)
	var builder strings.Builder
	for _, key := range keys {
		items := values[key]
		if len(items) == 0 {
			appendQuerySeparator(&builder)
			builder.WriteString(encodeQueryComponent(key))
			continue
		}
		for _, value := range items {
			appendQuerySeparator(&builder)
			builder.WriteString(encodeQueryComponent(key))
			builder.WriteByte('=')
			builder.WriteString(encodeQueryComponent(value))
		}
	}
	return builder.String()
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

func appendQuerySeparator(builder *strings.Builder) {
	if builder.Len() > 0 {
		builder.WriteByte('&')
	}
}

func sortRouteQueryKeys(keys []string) {
	for index := 1; index < len(keys); index++ {
		key := keys[index]
		position := index - 1
		for position >= 0 && keys[position] > key {
			keys[position+1] = keys[position]
			position--
		}
		keys[position+1] = key
	}
}

func encodeQueryComponent(value string) string {
	var builder strings.Builder
	for index := 0; index < len(value); index++ {
		char := value[index]
		if isQueryUnreserved(char) {
			builder.WriteByte(char)
			continue
		}
		if char == ' ' {
			builder.WriteByte('+')
			continue
		}
		builder.WriteByte('%')
		builder.WriteByte(hexDigit(char >> 4))
		builder.WriteByte(hexDigit(char))
	}
	return builder.String()
}

func decodeQueryComponent(value string) string {
	var builder strings.Builder
	for index := 0; index < len(value); index++ {
		char := value[index]
		if char == '+' {
			builder.WriteByte(' ')
			continue
		}
		if char == '%' && index+2 < len(value) {
			high, highOK := fromHex(value[index+1])
			low, lowOK := fromHex(value[index+2])
			if highOK && lowOK {
				builder.WriteByte(high<<4 | low)
				index += 2
				continue
			}
		}
		builder.WriteByte(char)
	}
	return builder.String()
}

func isQueryUnreserved(char byte) bool {
	return (char >= 'a' && char <= 'z') ||
		(char >= 'A' && char <= 'Z') ||
		(char >= '0' && char <= '9') ||
		char == '-' ||
		char == '_' ||
		char == '.' ||
		char == '~'
}

func hexDigit(value byte) byte {
	value = value & 0x0f
	if value < 10 {
		return '0' + value
	}
	return 'A' + value - 10
}

func fromHex(char byte) (byte, bool) {
	switch {
	case char >= '0' && char <= '9':
		return char - '0', true
	case char >= 'a' && char <= 'f':
		return char - 'a' + 10, true
	case char >= 'A' && char <= 'F':
		return char - 'A' + 10, true
	default:
		return 0, false
	}
}

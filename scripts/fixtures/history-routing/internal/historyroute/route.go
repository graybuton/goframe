package historyroute

import (
	"errors"
	pathpkg "path"
	"strings"
)

// Match is the fixture's deterministic view of one application target.
type Match struct {
	Name           string
	Target         string
	Path           string
	RawQuery       string
	Param          string
	QueryValue     string
	NotFound       bool
	MalformedPath  bool
	MalformedQuery bool
}

// NormalizeBase returns a leading- and trailing-slash deployment base.
func NormalizeBase(base string) (string, error) {
	if base == "" {
		return "/", nil
	}
	if strings.ContainsAny(base, "?#\\") {
		return "", errors.New("deployment base must be a URL path")
	}
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	for _, part := range strings.Split(base, "/") {
		if part == ".." {
			return "", errors.New("deployment base must not contain parent traversal")
		}
	}
	base = pathpkg.Clean(base)
	if base == "." || base == "/" {
		return "/", nil
	}
	return strings.TrimSuffix(base, "/") + "/", nil
}

// NormalizeTarget applies the fixture's route and trailing-slash policy.
func NormalizeTarget(target string) string {
	path, rawQuery := splitTarget(target)
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	path = pathpkg.Clean(path)
	if path == "." || path == "" {
		path = "/"
	}
	if path == "/settings" {
		path = "/settings/"
	}
	if rawQuery == "" {
		return path
	}
	return path + "?" + rawQuery
}

// TargetFromLocation strips base and preserves the browser query text.
func TargetFromLocation(base, pathname, search string) (string, bool, error) {
	base, err := NormalizeBase(base)
	if err != nil {
		return "", false, err
	}
	pathname = normalizeLocationPath(pathname)
	internal, ok := stripBase(base, pathname)
	if !ok {
		return "", false, nil
	}
	search = strings.TrimPrefix(search, "?")
	if search != "" {
		internal += "?" + search
	}
	return NormalizeTarget(internal), true, nil
}

// LocationForTarget applies base to one normalized internal target.
func LocationForTarget(base, target string) (string, error) {
	base, err := NormalizeBase(base)
	if err != nil {
		return "", err
	}
	target = NormalizeTarget(target)
	path, rawQuery := splitTarget(target)
	location := path
	if base != "/" {
		prefix := strings.TrimSuffix(base, "/")
		if path == "/" {
			location = base
		} else {
			location = prefix + path
		}
	}
	if rawQuery != "" {
		location += "?" + rawQuery
	}
	return location, nil
}

// MatchTarget matches one normalized application target.
func MatchTarget(target string) Match {
	target = NormalizeTarget(target)
	path, rawQuery := splitTarget(target)
	query, queryOK := parseQuery(rawQuery)
	result := Match{
		Target:         target,
		Path:           path,
		RawQuery:       rawQuery,
		MalformedQuery: !queryOK,
	}

	switch {
	case path == "/":
		result.Name = "home"
		return result
	case path == "/settings/":
		result.Name = "settings"
		return result
	case path == "/search":
		result.Name = "search"
		if queryOK {
			result.QueryValue = query["q"]
		}
		return result
	}

	parts := routeParts(path)
	if len(parts) == 2 && parts[0] == "users" {
		param, pathOK := decodePercent(parts[1], false)
		if pathOK && param != "" && !strings.Contains(param, "/") {
			result.Name = "user"
			result.Param = param
			if queryOK {
				result.QueryValue = query["tab"]
			}
			return result
		}
		result.MalformedPath = !pathOK
	}

	result.Name = "not-found"
	result.NotFound = true
	return result
}

func normalizeLocationPath(pathname string) string {
	if pathname == "" {
		return "/"
	}
	if !strings.HasPrefix(pathname, "/") {
		pathname = "/" + pathname
	}
	cleaned := pathpkg.Clean(pathname)
	if cleaned == "." || cleaned == "" {
		return "/"
	}
	if strings.HasSuffix(pathname, "/") && cleaned != "/" {
		cleaned += "/"
	}
	return cleaned
}

func stripBase(base, pathname string) (string, bool) {
	if base == "/" {
		return pathname, true
	}
	prefix := strings.TrimSuffix(base, "/")
	if pathname == base {
		return "/", true
	}
	if !strings.HasPrefix(pathname, prefix+"/") {
		return "", false
	}
	internal := strings.TrimPrefix(pathname, prefix)
	if internal == "" {
		internal = "/"
	}
	return internal, true
}

func splitTarget(target string) (string, string) {
	if index := strings.IndexByte(target, '?'); index >= 0 {
		return target[:index], target[index+1:]
	}
	return target, ""
}

func routeParts(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func parseQuery(raw string) (map[string]string, bool) {
	values := map[string]string{}
	if raw == "" {
		return values, true
	}
	for _, item := range strings.Split(raw, "&") {
		if item == "" {
			continue
		}
		name := item
		value := ""
		if index := strings.IndexByte(item, '='); index >= 0 {
			name = item[:index]
			value = item[index+1:]
		}
		decodedName, nameOK := decodePercent(name, true)
		decodedValue, valueOK := decodePercent(value, true)
		if !nameOK || !valueOK {
			return map[string]string{}, false
		}
		if _, exists := values[decodedName]; !exists {
			values[decodedName] = decodedValue
		}
	}
	return values, true
}

func decodePercent(value string, plusAsSpace bool) (string, bool) {
	var builder strings.Builder
	for index := 0; index < len(value); index++ {
		current := value[index]
		if plusAsSpace && current == '+' {
			builder.WriteByte(' ')
			continue
		}
		if current != '%' {
			builder.WriteByte(current)
			continue
		}
		if index+2 >= len(value) {
			return value, false
		}
		high, highOK := hexValue(value[index+1])
		low, lowOK := hexValue(value[index+2])
		if !highOK || !lowOK {
			return value, false
		}
		builder.WriteByte(high<<4 | low)
		index += 2
	}
	return builder.String(), true
}

func hexValue(value byte) (byte, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10, true
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10, true
	default:
		return 0, false
	}
}

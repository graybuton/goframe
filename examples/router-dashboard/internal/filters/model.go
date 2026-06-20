package filters

import (
	"strings"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

type FilterControlsProps struct {
	Query       string
	Status      string
	ResultCount int
	TotalCount  int
}

func FilterKey(query string, status string) string {
	return strings.TrimSpace(query) + "|" + normalizeStatusForControl(status)
}

func filterTarget(query string, status string) string {
	values := gf.QueryValues{}
	query = strings.TrimSpace(query)
	status = normalizeStatusForControl(status)
	if query != "" {
		values["q"] = []string{query}
	}
	if status != "all" {
		values["status"] = []string{status}
	}
	return gf.WithQuery("/issues", values)
}

func normalizeStatusForControl(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case "open", "review", "closed":
		return normalized
	default:
		return "all"
	}
}

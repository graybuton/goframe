package shared

import gf "github.com/graybuton/goframe/pkg/goframe"

func FormatCount(count int) string {
	return gf.ToString(count) + " cross-package issues"
}

func FormatPriority(priority string) string {
	return "priority: " + priority
}

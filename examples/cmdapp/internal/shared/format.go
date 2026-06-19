package shared

import gf "github.com/graybuton/goframe/pkg/goframe"

func FormatCount(count int) string {
	return gf.ToString(count) + " child-entry tasks"
}

func FormatPriority(priority string) string {
	return "priority: " + priority
}

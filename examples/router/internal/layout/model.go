package layout

import gf "github.com/graybuton/goframe/pkg/goframe"

type ShellProps struct {
	OnOpenFirstIssue func()
	Children         []gf.Node
}

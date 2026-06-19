package issues

type Issue struct {
	ID       string
	Title    string
	Priority string
}

type IssueListProps struct {
	Items []Issue
}

type IssueRowProps struct {
	Issue Issue
}

func DemoIssues() []Issue {
	return []Issue{
		{ID: "MP-1", Title: "Generate GOX in internal packages", Priority: "high"},
		{ID: "MP-2", Title: "Preserve import-path component identity", Priority: "medium"},
		{ID: "MP-3", Title: "Keep source tree clean", Priority: "low"},
	}
}

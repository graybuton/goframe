package pages

import data "github.com/graybuton/goframe/examples/router-dashboard/internal/data"

type HomeProps struct {
	Title string
}

type IssueListProps struct {
	Items       []data.Issue
	TotalCount  int
	Query       string
	Status      string
	CurrentPath string
}

type IssueRowProps struct {
	Issue data.Issue
}

type IssueDetailsProps struct {
	Issue data.Issue
	Found bool
	ID    string
}

type IssueEditProps struct {
	Issue data.Issue
	Found bool
	ID    string
}

type MissingIssueProps struct {
	ID string
}

type NotFoundProps struct {
	Path string
}

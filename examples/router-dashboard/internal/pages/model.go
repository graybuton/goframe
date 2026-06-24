package pages

import data "github.com/graybuton/goframe/examples/router-dashboard/internal/data"

type HomeProps struct {
	Title string
}

type IssueListProps struct {
	Query  string
	Status string
}

type IssueRowProps struct {
	Issue data.Issue
}

type IssueDetailsProps struct {
	ID    string
	Crash bool
}

type IssueEditProps struct {
	ID string
}

type MissingIssueProps struct {
	ID string
}

type NotFoundProps struct {
	Path string
}

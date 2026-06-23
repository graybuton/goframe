package pages

type HomeProps struct{}

type IssuesProps struct {
	Items []Issue
}

type IssueDetailsProps struct {
	ID       string
	RawQuery string
}

type NotFoundProps struct {
	Path string
}

type IssueLinkProps struct {
	Item Issue
}

type Issue struct {
	ID    string
	Title string
	Owner string
}

func DemoIssues() []Issue {
	return []Issue{
		{ID: "1", Title: "Document hash router semantics", Owner: "Runtime"},
		{ID: "2", Title: "Verify browser back and forward", Owner: "Smoke"},
		{ID: "3", Title: "Keep layout mounted across routes", Owner: "DX"},
	}
}

func (issue Issue) Path() string {
	return "/issues/" + issue.ID
}

func (issue Issue) TestID() string {
	if issue.ID == "1" {
		return "router-link-first-issue"
	}
	return "router-link-issue-" + issue.ID
}

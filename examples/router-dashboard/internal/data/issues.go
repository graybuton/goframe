package data

import "strings"

type Issue struct {
	ID          string
	Title       string
	Status      string
	Owner       string
	Priority    string
	Description string
}

type IssueFilter struct {
	Query  string
	Status string
}

func DemoIssues() []Issue {
	return []Issue{
		{ID: "RD-1", Title: "Auth token refresh fails on slow networks", Status: "open", Owner: "Identity", Priority: "high", Description: "Refresh retry state is not visible in the admin screen."},
		{ID: "RD-2", Title: "Billing dashboard needs clearer empty state", Status: "review", Owner: "Billing", Priority: "medium", Description: "Empty invoices should explain the current filter."},
		{ID: "RD-3", Title: "Export CSV action should preserve filters", Status: "open", Owner: "Reports", Priority: "medium", Description: "CSV export currently ignores the active query string."},
		{ID: "RD-4", Title: "Audit log row spacing is inconsistent", Status: "closed", Owner: "Platform", Priority: "low", Description: "Visual polish issue in dense table mode."},
		{ID: "RD-5", Title: "Support queue route needs not-found copy", Status: "review", Owner: "Support", Priority: "low", Description: "Unknown support ticket routes should be clearer."},
		{ID: "RD-6", Title: "Search input should keep URL state", Status: "open", Owner: "Experience", Priority: "high", Description: "Filter state should survive reload and browser back."},
		{ID: "RD-7", Title: "Admin form validation needs touched state", Status: "open", Owner: "Experience", Priority: "medium", Description: "Field errors should not appear before user interaction."},
		{ID: "RD-8", Title: "Router smoke should cover query filters", Status: "closed", Owner: "Runtime", Priority: "medium", Description: "The preview app should lock down URL-driven filters."},
	}
}

func FindIssue(id string) (Issue, bool) {
	for _, issue := range DemoIssues() {
		if issue.ID == id {
			return issue, true
		}
	}
	return Issue{}, false
}

func FilterIssues(items []Issue, filter IssueFilter) []Issue {
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	status := NormalizeStatus(filter.Status)
	filtered := make([]Issue, 0, len(items))
	for _, issue := range items {
		if status != "" && issue.Status != status {
			continue
		}
		if query != "" && !matchesIssue(issue, query) {
			continue
		}
		filtered = append(filtered, issue)
	}
	return filtered
}

func NormalizeStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "open", "review", "closed":
		return status
	default:
		return ""
	}
}

func StatusLabel(status string) string {
	switch NormalizeStatus(status) {
	case "open":
		return "Open"
	case "review":
		return "In review"
	case "closed":
		return "Closed"
	default:
		return "All statuses"
	}
}

func IssuePath(id string) string {
	return "/issues/" + id
}

func IssueEditPath(id string) string {
	return "/issues/" + id + "/edit"
}

func matchesIssue(issue Issue, query string) bool {
	return strings.Contains(strings.ToLower(issue.ID), query) ||
		strings.Contains(strings.ToLower(issue.Title), query) ||
		strings.Contains(strings.ToLower(issue.Owner), query) ||
		strings.Contains(strings.ToLower(issue.Description), query)
}

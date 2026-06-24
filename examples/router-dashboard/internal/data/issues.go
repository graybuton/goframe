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

func FindIssue(items []Issue, id string) (Issue, bool) {
	for _, issue := range items {
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

func NormalizePriority(priority string) string {
	priority = strings.ToLower(strings.TrimSpace(priority))
	switch priority {
	case "high", "medium", "low":
		return priority
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

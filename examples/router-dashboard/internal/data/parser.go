package data

import "strings"

type ParseError string

func (err ParseError) Error() string {
	return string(err)
}

func ParseIssues(text string) ([]Issue, error) {
	lines := strings.Split(text, "\n")
	issues := make([]Issue, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 6 {
			return nil, ParseError("invalid issue data line")
		}
		issue := Issue{
			ID:          strings.TrimSpace(parts[0]),
			Title:       strings.TrimSpace(parts[1]),
			Status:      NormalizeStatus(parts[2]),
			Priority:    NormalizePriority(parts[3]),
			Owner:       strings.TrimSpace(parts[4]),
			Description: strings.TrimSpace(parts[5]),
		}
		if issue.ID == "" || issue.Title == "" || issue.Owner == "" || issue.Description == "" {
			return nil, ParseError("issue data contains an empty required field")
		}
		if issue.Status == "" {
			return nil, ParseError("issue data contains an invalid status")
		}
		if issue.Priority == "" {
			return nil, ParseError("issue data contains an invalid priority")
		}
		issues = append(issues, issue)
	}
	if len(issues) == 0 {
		return nil, ParseError("issue data is empty")
	}
	return issues, nil
}

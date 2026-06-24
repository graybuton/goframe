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
		if len(parts) != 3 {
			return nil, ParseError("invalid issue data line")
		}
		issues = append(issues, Issue{
			ID:     strings.TrimSpace(parts[0]),
			Title:  strings.TrimSpace(parts[1]),
			Status: strings.TrimSpace(parts[2]),
		})
	}
	if len(issues) == 0 {
		return nil, ParseError("issue data is empty")
	}
	return issues, nil
}

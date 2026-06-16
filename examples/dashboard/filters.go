package main

import (
	"sort"
	"strconv"
	"strings"
)

func filterIssues(items []Issue, query string, status Status, priority Priority) []Issue {
	query = strings.ToLower(strings.TrimSpace(query))
	filtered := make([]Issue, 0, len(items))
	for _, item := range items {
		if status != StatusAll && item.Status != status {
			continue
		}
		if priority != PriorityAll && item.Priority != priority {
			continue
		}
		if query != "" && !matchesIssueQuery(item, query) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func sortIssues(items []Issue, mode SortMode) []Issue {
	sorted := copyIssues(items)
	sort.SliceStable(sorted, func(i, j int) bool {
		left := sorted[i]
		right := sorted[j]
		switch mode {
		case SortByPriority:
			if priorityRank(left.Priority) != priorityRank(right.Priority) {
				return priorityRank(left.Priority) > priorityRank(right.Priority)
			}
		case SortByEvents:
			if left.Events != right.Events {
				return left.Events > right.Events
			}
		case SortByOwner:
			if left.Owner != right.Owner {
				return left.Owner < right.Owner
			}
		default:
			if left.UpdatedAt != right.UpdatedAt {
				return left.UpdatedAt > right.UpdatedAt
			}
		}
		return left.ID < right.ID
	})
	return sorted
}

func visibleIssues(items []Issue, filters FilterState) []Issue {
	return sortIssues(filterIssues(items, filters.Query, filters.Status, filters.Priority), filters.Sort)
}

func dashboardMetrics(items []Issue, visible int) Metrics {
	metrics := Metrics{Total: len(items), Visible: visible}
	for _, item := range items {
		metrics.EventCount += item.Events
		switch item.Status {
		case StatusOpen:
			metrics.Open++
		case StatusBlocked:
			metrics.Blocked++
		case StatusResolved:
			metrics.Resolved++
		}
		if item.Priority == PriorityCritical {
			metrics.Critical++
		}
	}
	return metrics
}

func findIssue(items []Issue, id int) (Issue, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return Issue{}, false
}

func firstIssueID(items []Issue) int {
	if len(items) == 0 {
		return 0
	}
	return items[0].ID
}

func summaryText(visible, total int) string {
	return "Showing " + strconv.Itoa(visible) + " of " + strconv.Itoa(total) + " issues"
}

func matchesIssueQuery(item Issue, query string) bool {
	return strings.Contains(strings.ToLower(item.Title), query) ||
		strings.Contains(strings.ToLower(item.Owner), query) ||
		strings.Contains(strings.ToLower(item.Service), query)
}

func priorityRank(priority Priority) int {
	switch priority {
	case PriorityCritical:
		return 4
	case PriorityHigh:
		return 3
	case PriorityMedium:
		return 2
	case PriorityLow:
		return 1
	default:
		return 0
	}
}

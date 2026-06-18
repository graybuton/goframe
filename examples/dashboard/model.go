package main

type Status = string

const (
	StatusAll        Status = "all"
	StatusOpen       Status = "open"
	StatusInProgress Status = "in-progress"
	StatusBlocked    Status = "blocked"
	StatusResolved   Status = "resolved"
)

type Priority = string

const (
	PriorityAll      Priority = "all"
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

type SortMode = string

const (
	SortByUpdated  SortMode = "updated"
	SortByPriority SortMode = "priority"
	SortByEvents   SortMode = "events"
	SortByOwner    SortMode = "owner"
)

const dashboardItemCount = 300
const dashboardTableHeight = 560
const dashboardRowHeight = 48
const dashboardRowOverscan = 8
const dashboardTableColumnCount = 7

type Issue struct {
	ID         int
	Title      string
	Owner      string
	Status     Status
	Priority   Priority
	Service    string
	SearchText string
	UpdatedAt  int
	Events     int
}

type Metrics struct {
	Total      int
	Open       int
	Blocked    int
	Critical   int
	Resolved   int
	Visible    int
	EventCount int
}

type FilterState struct {
	Query    string
	Status   Status
	Priority Priority
	Sort     SortMode
}

func statusLabel(status Status) string {
	switch status {
	case StatusAll:
		return "All statuses"
	case StatusOpen:
		return "Open"
	case StatusInProgress:
		return "In progress"
	case StatusBlocked:
		return "Blocked"
	case StatusResolved:
		return "Resolved"
	default:
		return string(status)
	}
}

func priorityLabel(priority Priority) string {
	switch priority {
	case PriorityAll:
		return "All priorities"
	case PriorityLow:
		return "Low"
	case PriorityMedium:
		return "Medium"
	case PriorityHigh:
		return "High"
	case PriorityCritical:
		return "Critical"
	default:
		return string(priority)
	}
}

func sortLabel(mode SortMode) string {
	switch mode {
	case SortByUpdated:
		return "Recently updated"
	case SortByPriority:
		return "Priority"
	case SortByEvents:
		return "Event count"
	case SortByOwner:
		return "Owner"
	default:
		return string(mode)
	}
}

func statusClass(status Status) string {
	return "status status-" + string(status)
}

func priorityClass(priority Priority) string {
	return "priority priority-" + string(priority)
}

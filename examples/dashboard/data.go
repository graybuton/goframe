package main

func makeDemoIssues(count int) []Issue {
	owners := []string{"Ava", "Noah", "Mina", "Theo", "Iris", "Kai", "Lena", "Omar"}
	services := []string{"api", "billing", "search", "worker", "auth", "storage", "edge", "console"}
	statuses := []Status{StatusOpen, StatusInProgress, StatusBlocked, StatusResolved}
	priorities := []Priority{PriorityLow, PriorityMedium, PriorityHigh, PriorityCritical}
	verbs := []string{"Investigate", "Stabilize", "Review", "Patch", "Monitor", "Tune"}
	objects := []string{"latency", "queue depth", "deploy health", "cache churn", "alert volume", "error rate"}

	issues := make([]Issue, 0, count)
	for index := 0; index < count; index++ {
		id := index + 1
		status := statuses[(index*7+3)%len(statuses)]
		priority := priorities[(index*5+1)%len(priorities)]
		service := services[(index*11+2)%len(services)]
		owner := owners[(index*13+5)%len(owners)]
		if id%19 == 0 {
			status = StatusBlocked
		}
		if id%23 == 0 {
			priority = PriorityCritical
		}
		title := verbs[index%len(verbs)] + " " + service + " " + objects[(index*3)%len(objects)]
		issues = append(issues, Issue{
			ID:         id,
			Title:      title,
			Owner:      owner,
			Status:     status,
			Priority:   priority,
			Service:    service,
			SearchText: searchText(title, owner, service),
			UpdatedAt:  9000 - ((index * 37) % 8000),
			Events:     1 + ((index * 17) % 90),
		})
	}
	return issues
}

func resetDemoIssues() []Issue {
	return makeDemoIssues(dashboardItemCount)
}

func simulateIssueUpdate(items []Issue) []Issue {
	next := copyIssues(items)
	if len(next) == 0 {
		return next
	}
	for index := range next {
		if next[index].Status != StatusResolved {
			next[index].Events += 3
			next[index].UpdatedAt += 1000
			if next[index].Priority == PriorityLow {
				next[index].Priority = PriorityMedium
			}
			if next[index].Status == StatusOpen {
				next[index].Status = StatusInProgress
			}
			return next
		}
	}
	next[0].Events++
	next[0].UpdatedAt += 1000
	return next
}

func toggleIssueStatus(items []Issue, id int) []Issue {
	next := copyIssues(items)
	for index := range next {
		if next[index].ID != id {
			continue
		}
		if next[index].Status == StatusResolved {
			next[index].Status = StatusOpen
		} else {
			next[index].Status = StatusResolved
		}
		next[index].UpdatedAt += 500
		return next
	}
	return next
}

func copyIssues(items []Issue) []Issue {
	next := make([]Issue, len(items))
	copy(next, items)
	return next
}

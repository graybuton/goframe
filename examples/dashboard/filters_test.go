package main

import "testing"

func TestFilterIssues(t *testing.T) {
	items := []Issue{
		testIssue(1, "Investigate api latency", "Ava", StatusOpen, PriorityHigh, "api"),
		testIssue(2, "Patch billing queue", "Noah", StatusBlocked, PriorityCritical, "billing"),
		testIssue(3, "Review search deploy", "Mina", StatusResolved, PriorityLow, "search"),
	}

	got := filterIssues(items, "billing", StatusBlocked, PriorityCritical)
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("filtered = %#v, want only issue 2", got)
	}

	got = filterIssues(items, "ava", StatusAll, PriorityAll)
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("owner query filtered = %#v, want only issue 1", got)
	}
}

func testIssue(id int, title, owner string, status Status, priority Priority, service string) Issue {
	return Issue{
		ID:         id,
		Title:      title,
		Owner:      owner,
		Status:     status,
		Priority:   priority,
		Service:    service,
		SearchText: searchText(title, owner, service),
	}
}

func TestSortIssues(t *testing.T) {
	items := []Issue{
		{ID: 1, Priority: PriorityLow, UpdatedAt: 10, Events: 3, Owner: "Zed"},
		{ID: 2, Priority: PriorityCritical, UpdatedAt: 5, Events: 9, Owner: "Ava"},
		{ID: 3, Priority: PriorityHigh, UpdatedAt: 20, Events: 1, Owner: "Mina"},
	}

	if got := sortIssues(items, SortByUpdated); got[0].ID != 3 {
		t.Fatalf("updated sort first = %d, want 3", got[0].ID)
	}
	if got := sortIssues(items, SortByPriority); got[0].ID != 2 {
		t.Fatalf("priority sort first = %d, want 2", got[0].ID)
	}
	if got := sortIssues(items, SortByOwner); got[0].ID != 2 {
		t.Fatalf("owner sort first = %d, want 2", got[0].ID)
	}
}

func TestDashboardMetrics(t *testing.T) {
	items := []Issue{
		{Status: StatusOpen, Priority: PriorityCritical, Events: 2},
		{Status: StatusBlocked, Priority: PriorityHigh, Events: 5},
		{Status: StatusResolved, Priority: PriorityCritical, Events: 7},
	}

	got := dashboardMetrics(items, 2)
	want := Metrics{Total: 3, Open: 1, Blocked: 1, Critical: 2, Resolved: 1, Visible: 2, EventCount: 14}
	if got != want {
		t.Fatalf("metrics = %#v, want %#v", got, want)
	}
}

func TestSummaryText(t *testing.T) {
	if got := summaryText(7, 300); got != "Showing 7 of 300 issues" {
		t.Fatalf("summaryText = %q", got)
	}
}

func TestSimulateIssueUpdate(t *testing.T) {
	items := []Issue{{ID: 1, Status: StatusOpen, Priority: PriorityLow, Events: 1, UpdatedAt: 10}}
	got := simulateIssueUpdate(items)
	if got[0].Events != 4 || got[0].Priority != PriorityMedium || got[0].Status != StatusInProgress {
		t.Fatalf("simulate update = %#v", got[0])
	}
	if items[0].Events != 1 || items[0].Priority != PriorityLow || items[0].Status != StatusOpen {
		t.Fatalf("simulate update mutated original = %#v", items[0])
	}
}

func BenchmarkFilterIssues300(b *testing.B) {
	items := makeDemoIssues(300)
	b.ReportAllocs()
	for range b.N {
		_ = filterIssues(items, "billing", StatusAll, PriorityAll)
	}
}

func BenchmarkSortIssues300(b *testing.B) {
	items := makeDemoIssues(300)
	b.ReportAllocs()
	for range b.N {
		_ = sortIssues(items, SortByPriority)
	}
}

func BenchmarkDashboardMetrics300(b *testing.B) {
	items := makeDemoIssues(300)
	b.ReportAllocs()
	for range b.N {
		_ = dashboardMetrics(items, 300)
	}
}

func BenchmarkFindIssue300(b *testing.B) {
	items := makeDemoIssues(300)
	b.ReportAllocs()
	for range b.N {
		_, _ = findIssue(items, 240)
	}
}

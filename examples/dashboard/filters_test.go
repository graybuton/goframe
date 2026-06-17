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

func TestReduceIssuesToggle(t *testing.T) {
	items := []Issue{{ID: 1, Status: StatusOpen, UpdatedAt: 10}, {ID: 2, Status: StatusBlocked, UpdatedAt: 20}}

	got := reduceIssues(items, IssueAction{Kind: IssueActionToggle, ID: 2})

	if got[1].Status != StatusResolved || got[1].UpdatedAt != 520 {
		t.Fatalf("toggle action issue 2 = %#v", got[1])
	}
	if got[0] != items[0] {
		t.Fatalf("toggle action changed unrelated issue: %#v", got[0])
	}
	if items[1].Status != StatusBlocked || items[1].UpdatedAt != 20 {
		t.Fatalf("toggle action mutated original = %#v", items[1])
	}
}

func TestReduceIssuesSimulate(t *testing.T) {
	items := []Issue{
		{ID: 1, Status: StatusResolved, Priority: PriorityHigh, Events: 5, UpdatedAt: 10},
		{ID: 2, Status: StatusOpen, Priority: PriorityLow, Events: 1, UpdatedAt: 20},
	}

	got := reduceIssues(items, IssueAction{Kind: IssueActionSimulate})

	if got[1].Status != StatusInProgress || got[1].Priority != PriorityMedium || got[1].Events != 4 || got[1].UpdatedAt != 1020 {
		t.Fatalf("simulate action issue 2 = %#v", got[1])
	}
	if items[1].Status != StatusOpen || items[1].Priority != PriorityLow || items[1].Events != 1 || items[1].UpdatedAt != 20 {
		t.Fatalf("simulate action mutated original = %#v", items[1])
	}
}

func TestReduceIssuesReset(t *testing.T) {
	items := []Issue{{ID: 999, Status: StatusResolved}}

	got := reduceIssues(items, IssueAction{Kind: IssueActionReset})

	if len(got) != dashboardItemCount || got[0].ID != 1 {
		t.Fatalf("reset action returned %d items, first %#v", len(got), got[0])
	}
	if items[0].ID != 999 {
		t.Fatalf("reset action mutated original = %#v", items[0])
	}
}

func TestReduceIssuesUnknownActionReturnsSameSlice(t *testing.T) {
	items := []Issue{{ID: 1}}

	got := reduceIssues(items, IssueAction{Kind: IssueActionKind(99)})

	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("unknown action = %#v", got)
	}
	if len(got) > 0 {
		got[0].ID = 2
	}
	if items[0].ID != 2 {
		t.Fatal("unknown action should return the original slice for no-op actions")
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

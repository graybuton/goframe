package documentstate

import (
	"reflect"
	"testing"
)

func TestCoordinatorBaselineAndFirstOwner(t *testing.T) {
	baseline := State{Title: "baseline", Description: "authored"}
	coordinator := New(baseline)

	assertSnapshot(t, coordinator.Snapshot(), Snapshot{State: baseline})
	transition := mustSet(t, coordinator, "route", State{
		Title:       "Home",
		Description: "Home route",
	})
	if transition.Change != ChangeAdded {
		t.Fatalf("first change = %v, want added", transition.Change)
	}
	assertSnapshot(t, transition.Snapshot, Snapshot{
		State:       State{Title: "Home", Description: "Home route"},
		ActiveOwner: "route",
		HasOwner:    true,
	})
}

func TestCoordinatorNestedUpdateAndRemoval(t *testing.T) {
	coordinator := New(State{Title: "baseline", Description: "authored"})
	mustSet(t, coordinator, "route", State{
		Title:       "User 42",
		Description: "Profile 42",
	})
	mustSet(t, coordinator, "editor", State{
		Title:       "Editing User 42",
		Description: "Editing 42",
	})

	updated := mustSet(t, coordinator, "route", State{
		Title:       "User 7",
		Description: "Profile 7",
	})
	if updated.Change != ChangeUpdated {
		t.Fatalf("parent change = %v, want updated", updated.Change)
	}
	assertSnapshot(t, updated.Snapshot, Snapshot{
		State:       State{Title: "Editing User 42", Description: "Editing 42"},
		ActiveOwner: "editor",
		HasOwner:    true,
	})
	if got := coordinator.OwnerKeys(); !reflect.DeepEqual(got, []string{"route", "editor"}) {
		t.Fatalf("owner order after parent update = %v", got)
	}

	mustSet(t, coordinator, "editor", State{
		Title:       "Editing User 7",
		Description: "Editing 7",
	})
	removed := mustRemove(t, coordinator, "editor")
	assertSnapshot(t, removed.Snapshot, Snapshot{
		State:       State{Title: "User 7", Description: "Profile 7"},
		ActiveOwner: "route",
		HasOwner:    true,
	})
}

func TestCoordinatorOutOfOrderAndFinalRemoval(t *testing.T) {
	baseline := State{Title: "baseline", Description: "authored"}
	coordinator := New(baseline)
	mustSet(t, coordinator, "route", State{Title: "route", Description: "route"})
	mustSet(t, coordinator, "dialog", State{Title: "dialog", Description: "dialog"})
	mustSet(t, coordinator, "tooltip", State{Title: "tooltip", Description: "tooltip"})

	nonTop := mustRemove(t, coordinator, "dialog")
	assertSnapshot(t, nonTop.Snapshot, Snapshot{
		State:       State{Title: "tooltip", Description: "tooltip"},
		ActiveOwner: "tooltip",
		HasOwner:    true,
	})
	if got := coordinator.OwnerKeys(); !reflect.DeepEqual(got, []string{"route", "tooltip"}) {
		t.Fatalf("owner order after non-top removal = %v", got)
	}

	top := mustRemove(t, coordinator, "tooltip")
	if top.Snapshot.ActiveOwner != "route" {
		t.Fatalf("active owner after top removal = %q", top.Snapshot.ActiveOwner)
	}
	final := mustRemove(t, coordinator, "route")
	assertSnapshot(t, final.Snapshot, Snapshot{State: baseline})
	if coordinator.OwnerCount() != 0 {
		t.Fatalf("owner count = %d, want 0", coordinator.OwnerCount())
	}
}

func TestCoordinatorRepeatedAndDuplicateOwnerUpdates(t *testing.T) {
	coordinator := New(State{})
	first := State{Title: "first", Description: "pair"}
	mustSet(t, coordinator, "route", first)

	identical := mustSet(t, coordinator, "route", first)
	if identical.Change != ChangeNone {
		t.Fatalf("identical change = %v, want none", identical.Change)
	}
	if coordinator.OwnerCount() != 1 {
		t.Fatalf("owner count after duplicate key = %d, want 1", coordinator.OwnerCount())
	}

	mustSet(t, coordinator, "nested", State{Title: "nested", Description: "pair"})
	changed := mustSet(t, coordinator, "route", State{Title: "latest", Description: "pair"})
	if changed.Snapshot.ActiveOwner != "nested" {
		t.Fatalf("owner update changed priority: active = %q", changed.Snapshot.ActiveOwner)
	}
}

func TestCoordinatorRejectsMalformedOwnerKeys(t *testing.T) {
	coordinator := New(State{})
	for _, owner := range []string{"", " ", "\t\n"} {
		if _, err := coordinator.Set(owner, State{}); err == nil {
			t.Fatalf("Set(%q) succeeded", owner)
		}
		if _, err := coordinator.Remove(owner); err == nil {
			t.Fatalf("Remove(%q) succeeded", owner)
		}
	}
}

func TestCoordinatorPreservesStatePairsAndCallerValues(t *testing.T) {
	baseline := State{Title: "baseline title", Description: "baseline description"}
	desired := State{Title: "route title", Description: "route description"}
	coordinator := New(baseline)
	transition := mustSet(t, coordinator, "route", desired)

	if baseline != (State{Title: "baseline title", Description: "baseline description"}) {
		t.Fatalf("baseline input changed: %#v", baseline)
	}
	if desired != (State{Title: "route title", Description: "route description"}) {
		t.Fatalf("desired input changed: %#v", desired)
	}
	if transition.Snapshot.State != desired {
		t.Fatalf("selected pair = %#v, want %#v", transition.Snapshot.State, desired)
	}
}

func mustSet(t *testing.T, coordinator *Coordinator, owner string, state State) Transition {
	t.Helper()
	transition, err := coordinator.Set(owner, state)
	if err != nil {
		t.Fatal(err)
	}
	return transition
}

func mustRemove(t *testing.T, coordinator *Coordinator, owner string) Transition {
	t.Helper()
	transition, err := coordinator.Remove(owner)
	if err != nil {
		t.Fatal(err)
	}
	return transition
}

func assertSnapshot(t *testing.T, got, want Snapshot) {
	t.Helper()
	if got != want {
		t.Fatalf("snapshot = %#v, want %#v", got, want)
	}
}

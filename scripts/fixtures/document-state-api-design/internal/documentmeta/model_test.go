package documentmeta

import (
	"reflect"
	"testing"
)

type conformanceDriver interface {
	mount(name string) conformanceOwner
	snapshot() Snapshot
	ownerIDs() []uint64
}

type conformanceOwner interface {
	publish(Metadata) (Transition, error)
	release() (Transition, error)
}

type tokenDriver struct {
	coordinator *Coordinator
}

type tokenOwner struct {
	coordinator *Coordinator
	owner       *Owner
}

func (driver *tokenDriver) mount(string) conformanceOwner {
	return &tokenOwner{
		coordinator: driver.coordinator,
		owner:       driver.coordinator.NewOwner(),
	}
}

func (driver *tokenDriver) snapshot() Snapshot {
	return driver.coordinator.Snapshot()
}

func (driver *tokenDriver) ownerIDs() []uint64 {
	return driver.coordinator.OwnerIDs()
}

func (owner *tokenOwner) publish(metadata Metadata) (Transition, error) {
	return owner.coordinator.Publish(owner.owner, metadata)
}

func (owner *tokenOwner) release() (Transition, error) {
	return owner.coordinator.Release(owner.owner)
}

type controlDriver struct {
	coordinator *Coordinator
	owners      *StringOwners
}

type controlOwner struct {
	owners *StringOwners
	key    string
}

func (driver *controlDriver) mount(name string) conformanceOwner {
	return &controlOwner{owners: driver.owners, key: name}
}

func (driver *controlDriver) snapshot() Snapshot {
	return driver.coordinator.Snapshot()
}

func (driver *controlDriver) ownerIDs() []uint64 {
	return driver.coordinator.OwnerIDs()
}

func (owner *controlOwner) publish(metadata Metadata) (Transition, error) {
	return owner.owners.Publish(owner.key, metadata)
}

func (owner *controlOwner) release() (Transition, error) {
	return owner.owners.Release(owner.key)
}

func TestPureCoordinatorConformance(t *testing.T) {
	for _, model := range []string{"control", "opaque-token"} {
		t.Run(model, func(t *testing.T) {
			baseline := Metadata{Title: "authored", Description: "baseline"}
			coordinator := New(baseline)
			var driver conformanceDriver
			if model == "control" {
				driver = &controlDriver{
					coordinator: coordinator,
					owners:      NewStringOwners(coordinator),
				}
			} else {
				driver = &tokenDriver{coordinator: coordinator}
			}
			exerciseConformance(t, driver, baseline)
		})
	}
}

func TestCoordinatorInactiveOwnersDoNotReserveIDs(t *testing.T) {
	coordinator := New(Metadata{})
	first := coordinator.NewOwner()
	second := coordinator.NewOwner()

	if first.ID() != 0 || second.ID() != 0 {
		t.Fatalf("inactive owner IDs = %d, %d; want 0, 0", first.ID(), second.ID())
	}
	transition := mustCoordinatorPublish(t, coordinator, second, Metadata{Title: "second"})
	if transition.OwnerID != 1 || second.ID() != 1 {
		t.Fatalf("first committed owner ID = %d, want 1", transition.OwnerID)
	}
	if first.ID() != 0 {
		t.Fatalf("unpublished owner ID = %d, want 0", first.ID())
	}
	assertStatistics(t, coordinator.Stats(), Statistics{
		TokenCreations:         2,
		CommittedIDAssignments: 1,
		ActiveAdditions:        1,
		ActiveOwnerCount:       1,
		LastCommittedOwnerID:   1,
	})
}

func TestCoordinatorInactiveOwnerLeavesNoCommittedIDGap(t *testing.T) {
	coordinator := New(Metadata{})
	discarded := coordinator.NewOwner()
	committed := coordinator.NewOwner()

	transition := mustCoordinatorPublish(t, coordinator, committed, Metadata{Title: "committed"})
	if discarded.ID() != 0 {
		t.Fatalf("discarded owner ID = %d, want 0", discarded.ID())
	}
	if transition.OwnerID != 1 {
		t.Fatalf("first committed owner ID = %d, want 1", transition.OwnerID)
	}
}

func TestCoordinatorUpdatePreservesIdentityPriorityAndStatistics(t *testing.T) {
	coordinator := New(Metadata{})
	first := coordinator.NewOwner()
	second := coordinator.NewOwner()
	mustCoordinatorPublish(t, coordinator, first, Metadata{Title: "first"})
	mustCoordinatorPublish(t, coordinator, second, Metadata{Title: "second"})
	beforeID := first.ID()
	beforeOrder := coordinator.OwnerIDs()

	transition := mustCoordinatorPublish(t, coordinator, first, Metadata{
		Title:       "first updated",
		Description: "pair updated",
	})
	if transition.Change != ChangeUpdated {
		t.Fatalf("update change = %s, want updated", transition.Change.String())
	}
	if first.ID() != beforeID {
		t.Fatalf("owner ID changed from %d to %d", beforeID, first.ID())
	}
	if got := coordinator.OwnerIDs(); !reflect.DeepEqual(got, beforeOrder) {
		t.Fatalf("owner order = %v, want %v", got, beforeOrder)
	}
	if transition.Snapshot.ActiveOwnerID != second.ID() {
		t.Fatalf(
			"selected owner ID = %d, want %d",
			transition.Snapshot.ActiveOwnerID,
			second.ID(),
		)
	}
	assertStatistics(t, coordinator.Stats(), Statistics{
		TokenCreations:         2,
		CommittedIDAssignments: 2,
		ActiveAdditions:        2,
		Updates:                1,
		ActiveOwnerCount:       2,
		LastCommittedOwnerID:   2,
	})
}

func TestCoordinatorDuplicatePublicationDoesNotChangeStatistics(t *testing.T) {
	coordinator := New(Metadata{})
	owner := coordinator.NewOwner()
	metadata := Metadata{Title: "same", Description: "pair"}
	mustCoordinatorPublish(t, coordinator, owner, metadata)
	before := coordinator.Stats()

	transition := mustCoordinatorPublish(t, coordinator, owner, metadata)
	if transition.Change != ChangeNone {
		t.Fatalf("duplicate change = %s, want none", transition.Change.String())
	}
	assertStatistics(t, coordinator.Stats(), before)
}

func TestCoordinatorReleaseUnpublishedOwnerIsNoOp(t *testing.T) {
	coordinator := New(Metadata{Title: "authored"})
	owner := coordinator.NewOwner()
	before := coordinator.Stats()

	transition, err := coordinator.Release(owner)
	if err != nil {
		t.Fatal(err)
	}
	if transition.Change != ChangeNone {
		t.Fatalf("unpublished release change = %s, want none", transition.Change.String())
	}
	if owner.ID() != 0 {
		t.Fatalf("unpublished owner ID = %d, want 0", owner.ID())
	}
	assertStatistics(t, coordinator.Stats(), before)
}

func TestCoordinatorRemountUsesNextCommittedID(t *testing.T) {
	coordinator := New(Metadata{})
	first := coordinator.NewOwner()
	mustCoordinatorPublish(t, coordinator, first, Metadata{Title: "first"})
	if _, err := coordinator.Release(first); err != nil {
		t.Fatal(err)
	}
	second := coordinator.NewOwner()
	transition := mustCoordinatorPublish(t, coordinator, second, Metadata{Title: "second"})

	if transition.OwnerID != first.ID()+1 {
		t.Fatalf(
			"remount owner ID = %d, want %d",
			transition.OwnerID,
			first.ID()+1,
		)
	}
	assertStatistics(t, coordinator.Stats(), Statistics{
		TokenCreations:         2,
		CommittedIDAssignments: 2,
		ActiveAdditions:        2,
		Releases:               1,
		ActiveOwnerCount:       1,
		LastCommittedOwnerID:   2,
	})
}

func exerciseConformance(t *testing.T, driver conformanceDriver, baseline Metadata) {
	t.Helper()
	routeA := Metadata{Title: "route A", Description: "route A pair"}
	routeA2 := Metadata{Title: "route A2", Description: "route A2 pair"}
	editorB := Metadata{Title: "editor B", Description: "editor B pair"}
	dialogC := Metadata{Title: "dialog C", Description: "dialog C pair"}

	route := driver.mount("route")
	first := mustPublish(t, route, routeA)
	assertTransition(t, first, ChangeAdded, routeA, 1)

	editor := driver.mount("editor")
	second := mustPublish(t, editor, editorB)
	assertTransition(t, second, ChangeAdded, editorB, 2)

	order := append([]uint64(nil), driver.ownerIDs()...)
	updated := mustPublish(t, route, routeA2)
	assertTransition(t, updated, ChangeUpdated, editorB, 2)
	if got := driver.ownerIDs(); !reflect.DeepEqual(got, order) {
		t.Fatalf("owner update changed priority: got %v, want %v", got, order)
	}

	removed := mustRelease(t, editor)
	assertTransition(t, removed, ChangeRemoved, routeA2, 1)

	editor = driver.mount("editor-remount")
	mustPublish(t, editor, editorB)
	dialog := driver.mount("dialog")
	mustPublish(t, dialog, dialogC)
	nonTop := mustRelease(t, editor)
	assertTransition(t, nonTop, ChangeRemoved, dialogC, 2)

	identical := mustPublish(t, dialog, dialogC)
	if identical.Change != ChangeNone {
		t.Fatalf("identical pair change = %s, want none", identical.Change.String())
	}
	assertMetadataPair(t, identical.Snapshot.Metadata, dialogC)

	speculative := driver.mount("speculative")
	if got := driver.snapshot(); got.Metadata != dialogC || got.OwnerCount != 2 {
		t.Fatalf("inactive speculative owner changed snapshot: %#v", got)
	}
	_ = speculative

	mustRelease(t, dialog)
	final := mustRelease(t, route)
	assertTransition(t, final, ChangeRemoved, baseline, 0)

	duplicate := mustRelease(t, route)
	if duplicate.Change != ChangeNone {
		t.Fatalf("duplicate release change = %s, want none", duplicate.Change.String())
	}

	remounted := driver.mount("route-new-lifetime")
	next := mustPublish(t, remounted, routeA)
	if next.Snapshot.ActiveOwnerID == first.OwnerID {
		t.Fatalf("remount reused owner identity %d", first.OwnerID)
	}
}

func TestCoordinatorRejectsMissingOrForeignOwners(t *testing.T) {
	coordinator := New(Metadata{})
	if _, err := coordinator.Publish(nil, Metadata{}); err == nil {
		t.Fatal("Publish(nil) succeeded")
	}
	if _, err := coordinator.Release(nil); err == nil {
		t.Fatal("Release(nil) succeeded")
	}
	foreign := New(Metadata{}).NewOwner()
	if _, err := coordinator.Publish(foreign, Metadata{}); err == nil {
		t.Fatal("Publish(foreign) succeeded")
	}
	mustCoordinatorPublish(t, foreign.coordinator, foreign, Metadata{Title: "foreign"})
	if _, err := coordinator.Publish(foreign, Metadata{}); err == nil {
		t.Fatal("Publish(published foreign) succeeded")
	}
	if _, err := coordinator.Release(foreign); err == nil {
		t.Fatal("Release(published foreign) succeeded")
	}
	if _, err := (*Coordinator)(nil).Publish(foreign, Metadata{}); err == nil {
		t.Fatal("nil coordinator Publish succeeded")
	}
	if _, err := (*StringOwners)(nil).Publish("route", Metadata{}); err == nil {
		t.Fatal("nil string-owner bindings Publish succeeded")
	}
}

func TestCoordinatorComparesMetadataAsOnePair(t *testing.T) {
	coordinator := New(Metadata{})
	owner := coordinator.NewOwner()
	initial := Metadata{Title: "same title", Description: "description A"}
	mustCoordinatorPublish(t, coordinator, owner, initial)

	changed := mustCoordinatorPublish(t, coordinator, owner, Metadata{
		Title:       "same title",
		Description: "description B",
	})
	if changed.Change != ChangeUpdated {
		t.Fatalf("description-only change = %s, want updated", changed.Change.String())
	}
	if changed.Snapshot.Metadata.Description != "description B" {
		t.Fatalf("selected pair = %#v", changed.Snapshot.Metadata)
	}
	titleChanged := mustCoordinatorPublish(t, coordinator, owner, Metadata{
		Title:       "new title",
		Description: "description B",
	})
	if titleChanged.Change != ChangeUpdated {
		t.Fatalf("title-only change = %s, want updated", titleChanged.Change.String())
	}
	assertMetadataPair(t, titleChanged.Snapshot.Metadata, Metadata{
		Title:       "new title",
		Description: "description B",
	})
}

func TestCoordinatorOwnerIdentityDerivations(t *testing.T) {
	coordinator := New(Metadata{})

	hookSlotOne := coordinator.NewOwner()
	hookSlotTwo := coordinator.NewOwner()
	if hookSlotOne.ID() != 0 || hookSlotTwo.ID() != 0 {
		t.Fatal("inactive hook slots reserved owner identity")
	}
	mustCoordinatorPublish(t, coordinator, hookSlotOne, Metadata{Title: "hook one"})
	mustCoordinatorPublish(t, coordinator, hookSlotTwo, Metadata{Title: "hook two"})
	hookSlotOneID := hookSlotOne.ID()
	if hookSlotOne.ID() == hookSlotTwo.ID() {
		t.Fatalf("two hook slots share owner %d", hookSlotOne.ID())
	}
	mustCoordinatorPublish(t, coordinator, hookSlotOne, Metadata{Title: "hook one updated"})
	if hookSlotOne.ID() != hookSlotOneID {
		t.Fatal("hook owner identity is not stable")
	}

	componentInstanceOne := coordinator.NewOwner()
	componentInstanceTwo := coordinator.NewOwner()
	mustCoordinatorPublish(t, coordinator, componentInstanceOne, Metadata{Title: "component one"})
	mustCoordinatorPublish(t, coordinator, componentInstanceTwo, Metadata{Title: "component two"})
	if componentInstanceOne.ID() == componentInstanceTwo.ID() {
		t.Fatalf("two component instances share owner %d", componentInstanceOne.ID())
	}

	handleSlot := coordinator.NewOwner()
	forwarded := handleSlot
	if handleSlot.ID() != forwarded.ID() {
		t.Fatal("helper forwarding changed handle owner identity")
	}
	first := mustCoordinatorPublish(t, coordinator, handleSlot, Metadata{Title: "handle"})
	second := mustCoordinatorPublish(t, coordinator, forwarded, Metadata{Title: "handle"})
	if first.Change != ChangeAdded || second.Change != ChangeNone {
		t.Fatalf("forwarded handle changes = %s, %s", first.Change.String(), second.Change.String())
	}
}

func mustPublish(t *testing.T, owner conformanceOwner, metadata Metadata) Transition {
	t.Helper()
	transition, err := owner.publish(metadata)
	if err != nil {
		t.Fatal(err)
	}
	return transition
}

func mustRelease(t *testing.T, owner conformanceOwner) Transition {
	t.Helper()
	transition, err := owner.release()
	if err != nil {
		t.Fatal(err)
	}
	return transition
}

func mustCoordinatorPublish(
	t *testing.T,
	coordinator *Coordinator,
	owner *Owner,
	metadata Metadata,
) Transition {
	t.Helper()
	transition, err := coordinator.Publish(owner, metadata)
	if err != nil {
		t.Fatal(err)
	}
	return transition
}

func assertTransition(
	t *testing.T,
	transition Transition,
	change Change,
	metadata Metadata,
	ownerCount int,
) {
	t.Helper()
	if transition.Change != change {
		t.Fatalf("change = %s, want %s", transition.Change.String(), change.String())
	}
	assertMetadataPair(t, transition.Snapshot.Metadata, metadata)
	if transition.Snapshot.OwnerCount != ownerCount {
		t.Fatalf("owner count = %d, want %d", transition.Snapshot.OwnerCount, ownerCount)
	}
}

func assertMetadataPair(t *testing.T, got, want Metadata) {
	t.Helper()
	if got != want {
		t.Fatalf("metadata = %#v, want %#v", got, want)
	}
}

func assertStatistics(t *testing.T, got, want Statistics) {
	t.Helper()
	if got != want {
		t.Fatalf("statistics = %#v, want %#v", got, want)
	}
}

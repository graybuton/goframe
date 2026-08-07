package goframe

import "testing"

func TestDocumentMetadataBoundaryFailureBeforeParticipantRetainsOwner(t *testing.T) {
	coordinator, boundaryOwner, boundary, ownerA, metadataA :=
		committedDocumentMetadataBoundaryOwner(t)
	metadataB := documentMetadataValue{title: "B", description: "Description B"}

	failing := testComponentInstanceWithParent("OwnerB", boundaryOwner, func() Node {
		panic("failed before document metadata")
	})
	coordinator.beginUpdate()
	previous := beginProtectedSubtreeLifecycle(boundary)
	renderComponentInstance(failing)
	finishProtectedSubtreeLifecycle(boundary, previous)
	deactivateComponent(ownerA)
	coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(t, coordinator, metadataA.owner, metadataA.value, 1)
	if snapshot := coordinator.snapshot(); snapshot.failedBoundaryCount != 1 ||
		snapshot.retainedReleaseCount != 1 {
		t.Fatalf("failed boundary snapshot = %#v", snapshot)
	}

	clearErrorBoundary(boundary)
	retry := testComponentInstanceWithParent("OwnerB", boundaryOwner, func() Node {
		useDocumentMetadata(metadataB)
		return Empty()
	})
	coordinator.beginUpdate()
	previous = beginProtectedSubtreeLifecycle(boundary)
	renderComponentInstance(retry)
	finishProtectedSubtreeLifecycle(boundary, previous)
	coordinator.commitUpdate()

	ownerB := documentMetadataOwnerAtStateSlot(retry, 0)
	assertDocumentMetadataSnapshot(t, coordinator, ownerB, metadataB, 1)
	if metadataA.owner.state != documentMetadataOwnerReleased ||
		metadataA.owner.boundary != nil || ownerB.id != 2 {
		t.Fatalf("retry owners: A=%#v B=%#v", metadataA.owner, ownerB)
	}
	assertDocumentMetadataCoordinatorFinalized(t, coordinator)
}

func TestDocumentMetadataOwnerlessRecoveryRevealsParentOwner(t *testing.T) {
	coordinator := newDocumentMetadataCoordinator(
		documentMetadataValue{},
		testDocumentMetadataPublisher(func(documentMetadataValue) {}),
		nil,
	)
	installDocumentMetadataCoordinator(coordinator)
	t.Cleanup(uninstallDocumentMetadataCoordinator)
	parentMetadata := documentMetadataValue{title: "Parent", description: "Parent description"}
	parent := coordinator.newOwner()
	coordinator.beginUpdate()
	coordinator.stagePublish(parent, parentMetadata)
	coordinator.commitUpdate()

	boundaryOwner := testComponentInstance("Boundary", func() Node { return Empty() }, nil)
	boundary := ensureErrorBoundaryState(boundaryOwner)
	childMetadata := documentMetadataValue{title: "Child", description: "Child description"}
	child := testComponentInstanceWithParent("Child", boundaryOwner, func() Node {
		useDocumentMetadata(childMetadata)
		return Empty()
	})
	coordinator.beginUpdate()
	previous := beginProtectedSubtreeLifecycle(boundary)
	renderComponentInstance(child)
	finishProtectedSubtreeLifecycle(boundary, previous)
	coordinator.commitUpdate()
	childOwner := documentMetadataOwnerAtStateSlot(child, 0)

	failing := testComponentInstanceWithParent("Failing", boundaryOwner, func() Node {
		panic("failed before document metadata")
	})
	coordinator.beginUpdate()
	previous = beginProtectedSubtreeLifecycle(boundary)
	renderComponentInstance(failing)
	finishProtectedSubtreeLifecycle(boundary, previous)
	deactivateComponent(child)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, childOwner, childMetadata, 2)

	clearErrorBoundary(boundary)
	coordinator.beginUpdate()
	previous = beginProtectedSubtreeLifecycle(boundary)
	finishProtectedSubtreeLifecycle(boundary, previous)
	coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(t, coordinator, parent, parentMetadata, 1)
	if childOwner.state != documentMetadataOwnerReleased || childOwner.boundary != nil {
		t.Fatalf("recovered child owner = %#v", childOwner)
	}
	assertDocumentMetadataCoordinatorFinalized(t, coordinator)
}

func TestDocumentMetadataBoundarySiblingFailureRetainsOwner(t *testing.T) {
	coordinator, boundaryOwner, boundary, ownerA, metadataA :=
		committedDocumentMetadataBoundaryOwner(t)
	metadataB := documentMetadataValue{title: "B", description: "Description B"}
	ownerB := testComponentInstanceWithParent("OwnerB", boundaryOwner, func() Node {
		useDocumentMetadata(metadataB)
		return Empty()
	})
	failingSibling := testComponentInstanceWithParent("FailingSibling", boundaryOwner, func() Node {
		panic("failed after document metadata")
	})

	coordinator.beginUpdate()
	previous := beginProtectedSubtreeLifecycle(boundary)
	renderComponentInstance(ownerB)
	renderComponentInstance(failingSibling)
	finishProtectedSubtreeLifecycle(boundary, previous)
	deactivateComponent(ownerA)
	coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(t, coordinator, metadataA.owner, metadataA.value, 1)
	if owner := documentMetadataOwnerAtStateSlot(ownerB, 0); owner != nil {
		t.Fatalf("failed sibling committed owner B = %#v", owner)
	}
}

func TestDocumentMetadataOwnerlessBoundaryRecoveryConsumesRetainedRelease(t *testing.T) {
	coordinator, boundaryOwner, boundary, ownerA, metadataA :=
		committedDocumentMetadataBoundaryOwner(t)
	metadataB := documentMetadataValue{title: "B", description: "Description B"}
	failing := testComponentInstanceWithParent("OwnerB", boundaryOwner, func() Node {
		useDocumentMetadata(metadataB)
		panic("failed owner")
	})

	coordinator.beginUpdate()
	previous := beginProtectedSubtreeLifecycle(boundary)
	renderComponentInstance(failing)
	finishProtectedSubtreeLifecycle(boundary, previous)
	deactivateComponent(ownerA)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, metadataA.owner, metadataA.value, 1)

	clearErrorBoundary(boundary)
	coordinator.beginUpdate()
	previous = beginProtectedSubtreeLifecycle(boundary)
	finishProtectedSubtreeLifecycle(boundary, previous)
	coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(t, coordinator, nil, documentMetadataValue{}, 0)
	if snapshot := coordinator.snapshot(); snapshot.failedBoundaryCount != 0 ||
		snapshot.retainedReleaseCount != 0 || snapshot.batchActive {
		t.Fatalf("ownerless recovery snapshot = %#v", snapshot)
	}
}

func TestDocumentMetadataRepeatedBoundaryFailuresAndAbandonmentClearState(t *testing.T) {
	coordinator, boundaryOwner, boundary, ownerA, metadataA :=
		committedDocumentMetadataBoundaryOwner(t)

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			clearErrorBoundary(boundary)
		}
		failing := testComponentInstanceWithParent("Failing", boundaryOwner, func() Node {
			panic("failed before document metadata")
		})
		coordinator.beginUpdate()
		previous := beginProtectedSubtreeLifecycle(boundary)
		renderComponentInstance(failing)
		finishProtectedSubtreeLifecycle(boundary, previous)
		if attempt == 0 {
			deactivateComponent(ownerA)
		}
		coordinator.commitUpdate()

		assertDocumentMetadataSnapshot(t, coordinator, metadataA.owner, metadataA.value, 1)
		if snapshot := coordinator.snapshot(); snapshot.failedBoundaryCount != 1 ||
			snapshot.retainedReleaseCount != 1 {
			t.Fatalf("failed attempt %d snapshot = %#v", attempt+1, snapshot)
		}
	}

	coordinator.beginUpdate()
	deactivateComponent(boundaryOwner)
	coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(t, coordinator, nil, documentMetadataValue{}, 0)
	if metadataA.owner.state != documentMetadataOwnerReleased ||
		metadataA.owner.boundary != nil {
		t.Fatalf("abandoned owner A = %#v", metadataA.owner)
	}
	assertDocumentMetadataCoordinatorFinalized(t, coordinator)
}

func TestDocumentMetadataNestedOwnerUsesFinalOuterBoundary(t *testing.T) {
	coordinator := newDocumentMetadataCoordinator(
		documentMetadataValue{},
		testDocumentMetadataPublisher(func(documentMetadataValue) {}),
		nil,
	)
	installDocumentMetadataCoordinator(coordinator)
	t.Cleanup(uninstallDocumentMetadataCoordinator)

	outerOwner := testComponentInstance("OuterBoundary", func() Node { return Empty() }, nil)
	outer := ensureErrorBoundaryState(outerOwner)
	innerOwner := testComponentInstanceWithParent(
		"InnerBoundary",
		outerOwner,
		func() Node { return Empty() },
	)
	inner := ensureErrorBoundaryState(innerOwner)
	metadataA := documentMetadataValue{title: "A", description: "Description A"}
	ownerA := testComponentInstanceWithParent("OwnerA", innerOwner, func() Node {
		useDocumentMetadata(metadataA)
		return Empty()
	})

	coordinator.beginUpdate()
	previousOuter := beginProtectedSubtreeLifecycle(outer)
	previousInner := beginProtectedSubtreeLifecycle(inner)
	renderComponentInstance(ownerA)
	finishProtectedSubtreeLifecycle(inner, previousInner)
	finishProtectedSubtreeLifecycle(outer, previousOuter)
	coordinator.commitUpdate()

	owner := documentMetadataOwnerAtStateSlot(ownerA, 0)
	if owner == nil {
		t.Fatal("nested owner was not committed")
	}
	if owner.boundary != outer {
		t.Fatalf("nested owner boundary = %p, want outer %p", owner.boundary, outer)
	}

	failingSibling := testComponentInstanceWithParent("FailingSibling", outerOwner, func() Node {
		panic("outer sibling failed")
	})
	coordinator.beginUpdate()
	previousOuter = beginProtectedSubtreeLifecycle(outer)
	renderComponentInstance(failingSibling)
	finishProtectedSubtreeLifecycle(outer, previousOuter)
	deactivateComponent(ownerA)
	coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(t, coordinator, owner, metadataA, 1)

	clearErrorBoundary(outer)
	metadataB := documentMetadataValue{title: "B", description: "Description B"}
	ownerB := testComponentInstanceWithParent("OwnerB", innerOwner, func() Node {
		useDocumentMetadata(metadataB)
		return Empty()
	})
	coordinator.beginUpdate()
	previousOuter = beginProtectedSubtreeLifecycle(outer)
	previousInner = beginProtectedSubtreeLifecycle(inner)
	renderComponentInstance(ownerB)
	finishProtectedSubtreeLifecycle(inner, previousInner)
	finishProtectedSubtreeLifecycle(outer, previousOuter)
	coordinator.commitUpdate()
	committedB := documentMetadataOwnerAtStateSlot(ownerB, 0)
	assertDocumentMetadataSnapshot(t, coordinator, committedB, metadataB, 1)
	if committedB.boundary != outer || owner.state != documentMetadataOwnerReleased {
		t.Fatalf("nested retry owners: A=%#v B=%#v", owner, committedB)
	}

	failingAgain := testComponentInstanceWithParent("FailingAgain", outerOwner, func() Node {
		panic("outer sibling failed again")
	})
	coordinator.beginUpdate()
	previousOuter = beginProtectedSubtreeLifecycle(outer)
	renderComponentInstance(failingAgain)
	finishProtectedSubtreeLifecycle(outer, previousOuter)
	deactivateComponent(ownerB)
	coordinator.commitUpdate()
	clearErrorBoundary(outer)
	coordinator.beginUpdate()
	previousOuter = beginProtectedSubtreeLifecycle(outer)
	finishProtectedSubtreeLifecycle(outer, previousOuter)
	coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(t, coordinator, nil, documentMetadataValue{}, 0)
	if committedB.state != documentMetadataOwnerReleased || committedB.boundary != nil {
		t.Fatalf("nested ownerless recovery owner B = %#v", committedB)
	}
	assertDocumentMetadataCoordinatorFinalized(t, coordinator)
}

type committedBoundaryMetadata struct {
	owner *documentMetadataOwner
	value documentMetadataValue
}

func assertDocumentMetadataCoordinatorFinalized(
	t *testing.T,
	coordinator *documentMetadataCoordinator,
) {
	t.Helper()
	if snapshot := coordinator.snapshot(); snapshot.failedBoundaryCount != 0 ||
		snapshot.retainedReleaseCount != 0 || snapshot.batchActive {
		t.Fatalf("coordinator retained transaction state = %#v", snapshot)
	}
	if len(coordinator.batch.operations) != 0 || len(coordinator.batch.events) != 0 {
		t.Fatalf("coordinator retained batch values = %#v", coordinator.batch)
	}
}

func committedDocumentMetadataBoundaryOwner(t *testing.T) (
	*documentMetadataCoordinator,
	*componentInstance,
	*errorBoundaryState,
	*componentInstance,
	committedBoundaryMetadata,
) {
	t.Helper()
	coordinator := newDocumentMetadataCoordinator(
		documentMetadataValue{},
		testDocumentMetadataPublisher(func(documentMetadataValue) {}),
		nil,
	)
	installDocumentMetadataCoordinator(coordinator)
	t.Cleanup(uninstallDocumentMetadataCoordinator)
	boundaryOwner := testComponentInstance("Boundary", func() Node { return Empty() }, nil)
	boundary := ensureErrorBoundaryState(boundaryOwner)
	value := documentMetadataValue{title: "A", description: "Description A"}
	ownerA := testComponentInstanceWithParent("OwnerA", boundaryOwner, func() Node {
		useDocumentMetadata(value)
		return Empty()
	})

	coordinator.beginUpdate()
	previous := beginProtectedSubtreeLifecycle(boundary)
	renderComponentInstance(ownerA)
	finishProtectedSubtreeLifecycle(boundary, previous)
	coordinator.commitUpdate()
	owner := documentMetadataOwnerAtStateSlot(ownerA, 0)
	assertDocumentMetadataSnapshot(t, coordinator, owner, value, 1)
	return coordinator, boundaryOwner, boundary, ownerA, committedBoundaryMetadata{
		owner: owner,
		value: value,
	}
}

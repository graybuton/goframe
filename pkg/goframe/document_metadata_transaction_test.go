package goframe

import (
	"errors"
	"reflect"
	"testing"
)

func TestDocumentMetadataTransactionCommitAndRollback(t *testing.T) {
	baseline := documentMetadataValue{title: "Authored", description: "Baseline"}
	metadataA := documentMetadataValue{title: "A", description: "Description A"}
	metadataA2 := documentMetadataValue{title: "A2", description: "Description A2"}
	metadataB := documentMetadataValue{title: "B", description: "Description B"}
	metadataC := documentMetadataValue{title: "C", description: "Description C"}

	var publications []documentMetadataValue
	coordinator := newDocumentMetadataCoordinator(
		baseline,
		testDocumentMetadataPublisher(func(value documentMetadataValue) {
			publications = append(publications, value)
		}),
		nil,
	)
	ownerA := coordinator.newOwner()
	ownerB := coordinator.newOwner()
	ownerC := coordinator.newOwner()

	coordinator.beginUpdate()
	coordinator.stagePublish(ownerA, metadataA)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, ownerA, metadataA, 1)
	if ownerA.id != 1 {
		t.Fatalf("owner A id = %d, want 1", ownerA.id)
	}

	coordinator.beginUpdate()
	coordinator.stagePublish(ownerB, metadataB)
	coordinator.stageRelease(ownerA)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, ownerB, metadataB, 1)
	if ownerB.id != 2 {
		t.Fatalf("owner B id = %d, want 2", ownerB.id)
	}
	if !reflect.DeepEqual(publications, []documentMetadataValue{metadataA, metadataB}) {
		t.Fatalf("direct replacement publications = %#v", publications)
	}

	coordinator.beginUpdate()
	coordinator.stagePublish(ownerB, metadataA2)
	coordinator.stagePublish(ownerC, metadataC)
	coordinator.stageRelease(ownerB)
	coordinator.rollbackUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, ownerB, metadataB, 1)
	if ownerC.id != 0 || ownerC.state != documentMetadataOwnerPending {
		t.Fatalf("rolled-back owner C = %#v, want pending without id", ownerC)
	}
	if len(publications) != 2 {
		t.Fatalf("rollback published %d values, want 0", len(publications)-2)
	}

	coordinator.beginUpdate()
	coordinator.stagePublish(ownerB, metadataA2)
	coordinator.stagePublish(ownerC, metadataC)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, ownerC, metadataC, 1)
	if ownerC.id != 3 {
		t.Fatalf("owner C id = %d, want 3", ownerC.id)
	}
	if ownerB.state != documentMetadataOwnerReleased {
		t.Fatalf("owner B state = %v, want released", ownerB.state)
	}

	coordinator.beginUpdate()
	coordinator.stageRelease(ownerC)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, nil, baseline, 0)
	if got := publications[len(publications)-1]; got != baseline {
		t.Fatalf("final publication = %#v, want baseline %#v", got, baseline)
	}
	if coordinator.statistics.baselineRestorations != 1 {
		t.Fatalf("baseline restorations = %d, want 1", coordinator.statistics.baselineRestorations)
	}
}

func TestDocumentMetadataTransactionPriorityAndNoOpPublication(t *testing.T) {
	baseline := documentMetadataValue{title: "Authored", description: "Baseline"}
	metadataA := documentMetadataValue{title: "A", description: "Description A"}
	metadataA2 := documentMetadataValue{title: "A2", description: "Description A2"}
	metadataB := documentMetadataValue{title: "B", description: "Description B"}

	var publications []documentMetadataValue
	coordinator := newDocumentMetadataCoordinator(baseline, testDocumentMetadataPublisher(func(value documentMetadataValue) {
		publications = append(publications, value)
	}), nil)
	ownerA := coordinator.newOwner()
	ownerB := coordinator.newOwner()

	coordinator.beginUpdate()
	coordinator.stagePublish(ownerA, metadataA)
	coordinator.stagePublish(ownerB, metadataB)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, ownerB, metadataB, 2)

	coordinator.beginUpdate()
	coordinator.stagePublish(ownerA, metadataA2)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, ownerB, metadataB, 2)
	if len(publications) != 1 {
		t.Fatalf("non-selected update publications = %d, want 0", len(publications)-1)
	}

	coordinator.beginUpdate()
	coordinator.stagePublish(ownerB, metadataB)
	coordinator.commitUpdate()
	if len(publications) != 1 {
		t.Fatalf("same-value update publications = %d, want 0", len(publications)-1)
	}
	if ownerB.id != 2 {
		t.Fatalf("same-value update changed owner B id to %d", ownerB.id)
	}

	coordinator.beginUpdate()
	coordinator.stageRelease(ownerA)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, ownerB, metadataB, 1)
	if len(publications) != 1 {
		t.Fatalf("non-selected release publications = %d, want 0", len(publications)-1)
	}

	coordinator.beginUpdate()
	coordinator.stageRelease(ownerB)
	coordinator.commitUpdate()
	if len(publications) != 2 || publications[1] != baseline {
		t.Fatalf("selected release publications = %#v", publications)
	}
}

func TestDocumentMetadataTransactionMixedOperationsUseOperationOrder(t *testing.T) {
	baseline := documentMetadataValue{title: "Authored", description: "Baseline"}
	metadataA := documentMetadataValue{title: "A", description: "Description A"}
	metadataA2 := documentMetadataValue{title: "A2", description: "Description A2"}
	metadataB := documentMetadataValue{title: "B", description: "Description B"}
	metadataC := documentMetadataValue{title: "C", description: "Description C"}

	coordinator := newDocumentMetadataCoordinator(baseline, testDocumentMetadataPublisher(func(documentMetadataValue) {}), nil)
	ownerA := coordinator.newOwner()
	ownerB := coordinator.newOwner()
	ownerC := coordinator.newOwner()
	coordinator.beginUpdate()
	coordinator.stagePublish(ownerA, metadataA)
	coordinator.stagePublish(ownerB, metadataB)
	coordinator.commitUpdate()

	coordinator.beginUpdate()
	coordinator.stageRelease(ownerB)
	coordinator.stagePublish(ownerA, metadataA2)
	coordinator.stagePublish(ownerC, metadataC)
	coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(t, coordinator, ownerC, metadataC, 2)
	if got := coordinator.ownerIDs(); !reflect.DeepEqual(got, []uint64{1, 3}) {
		t.Fatalf("mixed operation owner order = %v, want [1 3]", got)
	}
	if coordinator.owners[0].metadata != metadataA2 {
		t.Fatalf("parent update = %#v, want %#v", coordinator.owners[0].metadata, metadataA2)
	}
}

func TestDocumentMetadataTransactionDoesNotCommitShortLivedOwner(t *testing.T) {
	baseline := documentMetadataValue{title: "Authored", description: "Baseline"}
	metadata := documentMetadataValue{title: "Temporary", description: "Temporary description"}
	var publications []documentMetadataValue
	coordinator := newDocumentMetadataCoordinator(baseline, testDocumentMetadataPublisher(func(value documentMetadataValue) {
		publications = append(publications, value)
	}), nil)
	owner := coordinator.newOwner()

	coordinator.beginUpdate()
	coordinator.stagePublish(owner, metadata)
	coordinator.stageRelease(owner)
	coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(t, coordinator, nil, baseline, 0)
	if owner.id != 0 || owner.state != documentMetadataOwnerReleased {
		t.Fatalf("short-lived owner = %#v, want released without committed id", owner)
	}
	if coordinator.statistics.committedIDAssignments != 0 ||
		coordinator.statistics.activeAdditions != 0 ||
		coordinator.statistics.documentPublications != 0 ||
		len(publications) != 0 {
		t.Fatalf("short-lived owner statistics=%#v publications=%#v", coordinator.statistics, publications)
	}
}

func TestDocumentMetadataTransactionRejectsInvalidOwnership(t *testing.T) {
	metadata := documentMetadataValue{title: "A", description: "Description A"}
	first := newDocumentMetadataCoordinator(documentMetadataValue{}, testDocumentMetadataPublisher(func(documentMetadataValue) {}), nil)
	second := newDocumentMetadataCoordinator(documentMetadataValue{}, testDocumentMetadataPublisher(func(documentMetadataValue) {}), nil)
	owner := first.newOwner()

	first.beginUpdate()
	assertPanic(t, "goframe: document metadata owner belongs to another coordinator", func() {
		second.stagePublish(owner, metadata)
	})
	first.rollbackUpdate()

	first.beginUpdate()
	first.stagePublish(owner, metadata)
	first.commitUpdate()

	first.beginUpdate()
	first.stageRelease(owner)
	first.stageRelease(owner)
	assertPanic(t, "goframe: document metadata owner was released more than once", func() {
		first.commitUpdate()
	})
	if first.batch.active {
		first.rollbackUpdate()
	}

	first.beginUpdate()
	first.commitUpdate()
	first.beginUpdate()
	assertPanic(t, "goframe: document metadata owner is already released", func() {
		first.stagePublish(owner, metadata)
	})
	first.rollbackUpdate()
}

func TestDocumentMetadataRenderParticipantStagesCommitAndRollback(t *testing.T) {
	metadataA := documentMetadataValue{title: "A", description: "Description A"}
	metadataB := documentMetadataValue{title: "B", description: "Description B"}
	coordinator := newDocumentMetadataCoordinator(documentMetadataValue{}, testDocumentMetadataPublisher(func(documentMetadataValue) {}), nil)
	installDocumentMetadataCoordinator(coordinator)
	t.Cleanup(uninstallDocumentMetadataCoordinator)

	instance := testComponentInstance("DocumentOwner", func() Node {
		useDocumentMetadata(metadataA)
		return Empty()
	}, nil)
	coordinator.beginUpdate()
	renderComponentInstance(instance)
	if owner := documentMetadataOwnerAtStateSlot(instance, 0); owner == nil || owner.id != 0 {
		t.Fatalf("owner before update commit = %#v, want pending id 0", owner)
	}
	coordinator.commitUpdate()
	owner := documentMetadataOwnerAtStateSlot(instance, 0)
	assertDocumentMetadataSnapshot(t, coordinator, owner, metadataA, 1)

	instance.node.render = func() Node {
		useDocumentMetadata(metadataB)
		panic("render failed")
	}
	coordinator.beginUpdate()
	renderComponentInstance(instance)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, owner, metadataA, 1)

	coordinator.beginUpdate()
	deactivateComponent(instance)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, nil, documentMetadataValue{}, 0)
}

func TestDocumentMetadataFailedInitialRenderCommitsNoOwner(t *testing.T) {
	coordinator := newDocumentMetadataCoordinator(documentMetadataValue{}, testDocumentMetadataPublisher(func(documentMetadataValue) {}), nil)
	installDocumentMetadataCoordinator(coordinator)
	t.Cleanup(uninstallDocumentMetadataCoordinator)

	instance := testComponentInstance("FailedDocumentOwner", func() Node {
		useDocumentMetadata(documentMetadataValue{title: "failed", description: "failed"})
		panic("render failed")
	}, nil)
	coordinator.beginUpdate()
	renderComponentInstance(instance)
	coordinator.commitUpdate()

	if len(instance.stateSlots) != 0 || coordinator.snapshot().ownerCount != 0 {
		t.Fatalf("failed initial render state slots=%d snapshot=%#v", len(instance.stateSlots), coordinator.snapshot())
	}
	if coordinator.statistics.committedIDAssignments != 0 || coordinator.statistics.documentPublications != 0 {
		t.Fatalf("failed initial render statistics = %#v", coordinator.statistics)
	}
}

func TestDocumentMetadataProtectedLifecycleCommitAndRollback(t *testing.T) {
	metadataA := documentMetadataValue{title: "A", description: "Description A"}
	metadataB := documentMetadataValue{title: "B", description: "Description B"}
	coordinator := newDocumentMetadataCoordinator(documentMetadataValue{}, testDocumentMetadataPublisher(func(documentMetadataValue) {}), nil)
	installDocumentMetadataCoordinator(coordinator)
	t.Cleanup(uninstallDocumentMetadataCoordinator)

	boundary := testComponentInstance("Boundary", func() Node { return Empty() }, nil)
	state := ensureErrorBoundaryState(boundary)
	ownerA := testComponentInstanceWithParent("OwnerA", boundary, func() Node {
		useDocumentMetadata(metadataA)
		return Empty()
	})
	ownerB := testComponentInstanceWithParent("OwnerB", boundary, func() Node {
		useDocumentMetadata(metadataB)
		return Empty()
	})

	coordinator.beginUpdate()
	previous := beginProtectedSubtreeLifecycle(state)
	renderComponentInstance(ownerA)
	finishProtectedSubtreeLifecycle(state, previous)
	coordinator.commitUpdate()
	committedA := documentMetadataOwnerAtStateSlot(ownerA, 0)
	assertDocumentMetadataSnapshot(t, coordinator, committedA, metadataA, 1)

	coordinator.beginUpdate()
	previous = beginProtectedSubtreeLifecycle(state)
	renderComponentInstance(ownerB)
	state.phase = errorBoundaryCaptured
	finishProtectedSubtreeLifecycle(state, previous)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, committedA, metadataA, 1)
	if len(ownerB.stateSlots) != 0 {
		t.Fatalf("rolled-back protected owner retained %d state slots", len(ownerB.stateSlots))
	}

	state.phase = errorBoundaryProtected
	state.info = ErrorInfo{}
	coordinator.beginUpdate()
	previous = beginProtectedSubtreeLifecycle(state)
	renderComponentInstance(ownerB)
	finishProtectedSubtreeLifecycle(state, previous)
	coordinator.stageRelease(committedA)
	coordinator.commitUpdate()
	committedB := documentMetadataOwnerAtStateSlot(ownerB, 0)
	assertDocumentMetadataSnapshot(t, coordinator, committedB, metadataB, 1)
}

func TestDocumentMetadataFailedBoundaryReplacementRetainsOwnerUntilRetry(t *testing.T) {
	baseline := documentMetadataValue{title: "Authored", description: "Baseline"}
	metadataA := documentMetadataValue{title: "A", description: "Description A"}
	metadataB := documentMetadataValue{title: "B", description: "Description B"}
	var publications []documentMetadataValue
	coordinator := newDocumentMetadataCoordinator(baseline, testDocumentMetadataPublisher(func(value documentMetadataValue) {
		publications = append(publications, value)
	}), nil)
	boundaryOwner := testComponentInstance("Boundary", func() Node { return Empty() }, nil)
	boundary := ensureErrorBoundaryState(boundaryOwner)
	ownerA := coordinator.newOwner()
	ownerB := coordinator.newOwner()

	coordinator.beginUpdate()
	coordinator.stagePublishForBoundary(ownerA, metadataA, boundary)
	coordinator.stageBoundaryOutcome(
		boundary,
		boundary,
		protectedSubtreeLifecycleCommitted,
	)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, ownerA, metadataA, 1)

	coordinator.beginUpdate()
	coordinator.stageFailedPublish(ownerB, metadataB, boundary)
	coordinator.stageBoundaryOutcome(
		boundary,
		boundary,
		protectedSubtreeLifecycleFailed,
	)
	coordinator.commitUpdate()
	coordinator.beginUpdate()
	coordinator.stageRelease(ownerA)
	coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(t, coordinator, ownerA, metadataA, 1)
	if ownerA.state != documentMetadataOwnerActive || ownerB.id != 0 {
		t.Fatalf("failed replacement owners: A=%#v B=%#v", ownerA, ownerB)
	}
	if coordinator.snapshot().failedBoundaryCount != 1 ||
		coordinator.snapshot().retainedReleaseCount != 1 ||
		coordinator.snapshot().batchActive {
		t.Fatalf("failed replacement state = %#v", coordinator.snapshot())
	}
	if coordinator.statistics.releases != 0 ||
		coordinator.statistics.baselineRestorations != 0 {
		t.Fatalf("failed replacement statistics = %#v", coordinator.statistics)
	}

	coordinator.beginUpdate()
	coordinator.stagePublishForBoundary(ownerB, metadataB, boundary)
	coordinator.stageBoundaryOutcome(
		boundary,
		boundary,
		protectedSubtreeLifecycleCommitted,
	)
	coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(t, coordinator, ownerB, metadataB, 1)
	if ownerA.state != documentMetadataOwnerReleased || ownerB.id != 2 {
		t.Fatalf("retried replacement owners: A=%#v B=%#v", ownerA, ownerB)
	}
	if coordinator.snapshot().failedBoundaryCount != 0 ||
		coordinator.snapshot().retainedReleaseCount != 0 ||
		coordinator.snapshot().batchActive {
		t.Fatalf("retried replacement state = %#v", coordinator.snapshot())
	}
	if coordinator.statistics.releases != 1 ||
		coordinator.statistics.baselineRestorations != 0 {
		t.Fatalf("retried replacement statistics = %#v", coordinator.statistics)
	}
	if !reflect.DeepEqual(publications, []documentMetadataValue{metadataA, metadataB}) {
		t.Fatalf("replacement publications = %#v, want A -> B", publications)
	}
}

func TestDocumentMetadataAbandonedFailedBoundaryReleasesRetainedOwner(t *testing.T) {
	baseline := documentMetadataValue{title: "Authored", description: "Baseline"}
	metadataA := documentMetadataValue{title: "A", description: "Description A"}
	coordinator := newDocumentMetadataCoordinator(baseline, testDocumentMetadataPublisher(func(documentMetadataValue) {}), nil)
	installDocumentMetadataCoordinator(coordinator)
	t.Cleanup(uninstallDocumentMetadataCoordinator)
	boundaryOwner := testComponentInstance("Boundary", func() Node { return Empty() }, nil)
	boundary := ensureErrorBoundaryState(boundaryOwner)
	ownerA := coordinator.newOwner()
	failed := coordinator.newOwner()

	coordinator.beginUpdate()
	coordinator.stagePublishForBoundary(ownerA, metadataA, boundary)
	coordinator.stageBoundaryOutcome(
		boundary,
		boundary,
		protectedSubtreeLifecycleCommitted,
	)
	coordinator.commitUpdate()
	coordinator.beginUpdate()
	coordinator.stageFailedPublish(failed, metadataA, boundary)
	coordinator.stageBoundaryOutcome(
		boundary,
		boundary,
		protectedSubtreeLifecycleFailed,
	)
	coordinator.commitUpdate()
	coordinator.beginUpdate()
	coordinator.stageRelease(ownerA)
	coordinator.commitUpdate()

	coordinator.beginUpdate()
	deactivateComponent(boundaryOwner)
	coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(t, coordinator, nil, baseline, 0)
	if ownerA.state != documentMetadataOwnerReleased ||
		coordinator.snapshot().failedBoundaryCount != 0 ||
		coordinator.snapshot().retainedReleaseCount != 0 {
		t.Fatalf("abandoned boundary state: owner=%#v snapshot=%#v", ownerA, coordinator.snapshot())
	}
}

func TestDocumentMetadataPairPublicationIsFailureAtomic(t *testing.T) {
	previous := documentMetadataValue{title: "A", description: "Description A"}
	next := documentMetadataValue{title: "B", description: "Description B"}
	titleFailure := errors.New("write title")
	descriptionFailure := errors.New("write description")
	restoreTitleFailure := errors.New("restore title")
	restoreDescriptionFailure := errors.New("restore description")

	t.Run("title write fails before mutation", func(t *testing.T) {
		current := previous
		descriptionWrites := 0
		err := writeDocumentMetadataPair(
			previous,
			next,
			func(string) error { return titleFailure },
			func(value string) error {
				descriptionWrites++
				current.description = value
				return nil
			},
		)
		if !errors.Is(err, titleFailure) || current != previous || descriptionWrites != 0 {
			t.Fatalf("title failure: error=%v current=%#v description writes=%d", err, current, descriptionWrites)
		}
	})

	t.Run("description failure restores the previous pair", func(t *testing.T) {
		current := previous
		descriptionAttempts := 0
		err := writeDocumentMetadataPair(
			previous,
			next,
			func(value string) error {
				current.title = value
				return nil
			},
			func(value string) error {
				descriptionAttempts++
				if descriptionAttempts == 1 {
					return descriptionFailure
				}
				current.description = value
				return nil
			},
		)
		if !errors.Is(err, descriptionFailure) || current != previous || descriptionAttempts != 2 {
			t.Fatalf("description failure: error=%v current=%#v attempts=%d", err, current, descriptionAttempts)
		}
	})

	t.Run("restoration failures remain visible", func(t *testing.T) {
		current := previous
		titleAttempts := 0
		descriptionAttempts := 0
		err := writeDocumentMetadataPair(
			previous,
			next,
			func(value string) error {
				titleAttempts++
				if titleAttempts == 2 {
					return restoreTitleFailure
				}
				current.title = value
				return nil
			},
			func(string) error {
				descriptionAttempts++
				if descriptionAttempts == 1 {
					return descriptionFailure
				}
				return restoreDescriptionFailure
			},
		)
		if !errors.Is(err, descriptionFailure) ||
			!errors.Is(err, restoreTitleFailure) ||
			!errors.Is(err, restoreDescriptionFailure) {
			t.Fatalf("restoration error = %v", err)
		}
		if current.title != next.title {
			t.Fatalf("failed title restoration changed current title to %q", current.title)
		}
	})
}

func TestDocumentMetadataPublicationFailurePreservesCommittedState(t *testing.T) {
	baseline := documentMetadataValue{title: "Authored", description: "Baseline"}
	metadataA := documentMetadataValue{title: "A", description: "Description A"}
	metadataB := documentMetadataValue{title: "B", description: "Description B"}
	publicationFailure := errors.New("forced publication failure")
	document := baseline
	failPublication := false
	coordinator := newDocumentMetadataCoordinator(
		baseline,
		func(previous, next documentMetadataValue) error {
			if document != previous {
				t.Fatalf("publisher previous=%#v document=%#v", previous, document)
			}
			if failPublication {
				return publicationFailure
			}
			document = next
			return nil
		},
		nil,
	)
	ownerA := coordinator.newOwner()
	ownerB := coordinator.newOwner()

	coordinator.beginUpdate()
	coordinator.stagePublish(ownerA, metadataA)
	coordinator.commitUpdate()
	beforeStatistics := coordinator.statistics
	failPublication = true

	coordinator.beginUpdate()
	coordinator.stagePublish(ownerB, metadataB)
	coordinator.stageRelease(ownerA)
	err := recoverDocumentMetadataError(t, coordinator.commitUpdate)
	if !errors.Is(err, publicationFailure) {
		t.Fatalf("publication error = %v", err)
	}
	assertDocumentMetadataSnapshot(t, coordinator, ownerA, metadataA, 1)
	if document != metadataA || ownerA.state != documentMetadataOwnerActive ||
		ownerB.id != 0 || ownerB.state != documentMetadataOwnerPending ||
		coordinator.batch.active || len(coordinator.batch.operations) != 0 ||
		len(coordinator.batch.events) != 0 ||
		coordinator.snapshot().retainedReleaseCount != 1 ||
		coordinator.statistics != beforeStatistics {
		t.Fatalf("failed publication: document=%#v A=%#v B=%#v batch=%#v statistics=%#v want=%#v",
			document, ownerA, ownerB, coordinator.batch, coordinator.statistics, beforeStatistics)
	}

	failPublication = false
	coordinator.beginUpdate()
	coordinator.stagePublish(ownerB, metadataB)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, ownerB, metadataB, 1)
	if document != metadataB || ownerA.state != documentMetadataOwnerReleased || ownerB.id != 2 {
		t.Fatalf("retry: document=%#v A=%#v B=%#v", document, ownerA, ownerB)
	}
}

func TestDocumentMetadataObserverFailureDoesNotRollBackCommit(t *testing.T) {
	baseline := documentMetadataValue{title: "Authored", description: "Baseline"}
	metadataA := documentMetadataValue{title: "A", description: "Description A"}
	metadataB := documentMetadataValue{title: "B", description: "Description B"}
	observerFailure := errors.New("forced observer failure")
	document := baseline
	failObserver := false
	var coordinator *documentMetadataCoordinator
	var ownerB *documentMetadataOwner
	coordinator = newDocumentMetadataCoordinator(
		baseline,
		func(previous, next documentMetadataValue) error {
			if document != previous {
				t.Fatalf("publisher previous=%#v document=%#v", previous, document)
			}
			document = next
			return nil
		},
		func(event documentMetadataEvent) error {
			if failObserver && event.kind == "update-commit" {
				assertDocumentMetadataSnapshot(t, coordinator, ownerB, metadataB, 1)
				if coordinator.batch.active {
					t.Fatal("observer ran before the committed batch became inactive")
				}
				return observerFailure
			}
			return nil
		},
	)
	ownerA := coordinator.newOwner()
	ownerB = coordinator.newOwner()

	coordinator.beginUpdate()
	coordinator.stagePublish(ownerA, metadataA)
	coordinator.commitUpdate()
	failObserver = true

	coordinator.beginUpdate()
	coordinator.stagePublish(ownerB, metadataB)
	coordinator.stageRelease(ownerA)
	err := recoverDocumentMetadataError(t, coordinator.commitUpdate)
	if !errors.Is(err, observerFailure) {
		t.Fatalf("observer error = %v", err)
	}
	assertDocumentMetadataSnapshot(t, coordinator, ownerB, metadataB, 1)
	if document != metadataB || ownerA.state != documentMetadataOwnerReleased ||
		ownerB.id != 2 || coordinator.batch.active ||
		len(coordinator.batch.operations) != 0 || len(coordinator.batch.events) != 0 ||
		coordinator.statistics.rollbacks != 0 {
		t.Fatalf("observer failure: document=%#v A=%#v B=%#v batch=%#v statistics=%#v",
			document, ownerA, ownerB, coordinator.batch, coordinator.statistics)
	}
}

func TestDocumentMetadataNestedProtectedLifecycleDelegatesToOuterBoundary(t *testing.T) {
	metadata := documentMetadataValue{title: "Nested", description: "Nested description"}
	coordinator := newDocumentMetadataCoordinator(documentMetadataValue{}, testDocumentMetadataPublisher(func(documentMetadataValue) {}), nil)
	installDocumentMetadataCoordinator(coordinator)
	t.Cleanup(uninstallDocumentMetadataCoordinator)

	outer := testComponentInstance("OuterBoundary", func() Node { return Empty() }, nil)
	outerState := ensureErrorBoundaryState(outer)
	inner := testComponentInstanceWithParent("InnerBoundary", outer, func() Node { return Empty() })
	innerState := ensureErrorBoundaryState(inner)
	owner := testComponentInstanceWithParent("NestedOwner", inner, func() Node {
		useDocumentMetadata(metadata)
		return Empty()
	})

	coordinator.beginUpdate()
	previousOuter := beginProtectedSubtreeLifecycle(outerState)
	previousInner := beginProtectedSubtreeLifecycle(innerState)
	renderComponentInstance(owner)
	finishProtectedSubtreeLifecycle(innerState, previousInner)
	if !owner.lifecycleAttempt.active || len(outerState.attempts) != 1 {
		t.Fatalf("inner success did not delegate attempt: active=%t outer attempts=%d",
			owner.lifecycleAttempt.active, len(outerState.attempts))
	}
	outerState.phase = errorBoundaryCaptured
	finishProtectedSubtreeLifecycle(outerState, previousOuter)
	coordinator.commitUpdate()

	if len(owner.stateSlots) != 0 || coordinator.snapshot().ownerCount != 0 {
		t.Fatalf("outer rollback committed nested owner: slots=%d snapshot=%#v",
			len(owner.stateSlots), coordinator.snapshot())
	}
	if coordinator.statistics.committedIDAssignments != 0 {
		t.Fatalf("outer rollback assigned %d owner ids", coordinator.statistics.committedIDAssignments)
	}
}

func assertDocumentMetadataSnapshot(
	t *testing.T,
	coordinator *documentMetadataCoordinator,
	owner *documentMetadataOwner,
	metadata documentMetadataValue,
	ownerCount int,
) {
	t.Helper()
	snapshot := coordinator.snapshot()
	if snapshot.owner != owner || snapshot.metadata != metadata || snapshot.ownerCount != ownerCount {
		t.Fatalf("snapshot = %#v, want owner=%p metadata=%#v count=%d", snapshot, owner, metadata, ownerCount)
	}
}

func documentMetadataOwnerAtStateSlot(instance *componentInstance, index int) *documentMetadataOwner {
	if instance == nil || index < 0 || index >= len(instance.stateSlots) {
		return nil
	}
	owner, _ := instance.stateSlots[index].value.(*documentMetadataOwner)
	return owner
}

func recoverDocumentMetadataError(t *testing.T, operation func()) (err error) {
	t.Helper()
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("operation did not panic")
		}
		var ok bool
		err, ok = recovered.(error)
		if !ok {
			t.Fatalf("panic = %#v, want error", recovered)
		}
	}()
	operation()
	return nil
}

func testDocumentMetadataPublisher(
	apply func(documentMetadataValue),
) documentMetadataPublisher {
	return func(_ documentMetadataValue, next documentMetadataValue) error {
		apply(next)
		return nil
	}
}

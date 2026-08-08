package goframe

import (
	"errors"
	"reflect"
	"testing"
)

func TestDocumentMetadataFailedHandoffIgnoresUnrelatedUpdate(t *testing.T) {
	baseline := documentMetadataValue{title: "Authored", description: "Baseline"}
	metadataA := documentMetadataValue{title: "A", description: "Description A"}
	metadataB := documentMetadataValue{title: "B", description: "Description B"}
	publicationFailure := errors.New("forced publication failure")
	document := baseline
	failPublication := false
	var publications []documentMetadataValue
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
			publications = append(publications, next)
			return nil
		},
		nil,
	)
	installDocumentMetadataCoordinator(coordinator)
	t.Cleanup(uninstallDocumentMetadataCoordinator)

	rendersA := 0
	cleanupA := 0
	ownerAComponent := testComponentInstance("OwnerA", func() Node {
		rendersA++
		useDocumentMetadata(metadataA)
		UseUnmount(func() {
			cleanupA++
		})
		return Empty()
	}, nil)
	rendersB := 0
	ownerBComponent := testComponentInstance("OwnerB", func() Node {
		rendersB++
		useDocumentMetadata(metadataB)
		return Empty()
	}, nil)
	shellRenders := 0
	shell := testComponentInstance("Shell", func() Node {
		shellRenders++
		return Empty()
	}, nil)

	coordinator.beginUpdate()
	renderComponentInstance(ownerAComponent)
	coordinator.commitUpdate()
	ownerA := documentMetadataOwnerAtStateSlot(ownerAComponent, 0)
	assertDocumentMetadataSnapshot(t, coordinator, ownerA, metadataA, 1)

	failPublication = true
	coordinator.beginUpdate()
	renderComponentInstance(ownerBComponent)
	deactivateComponent(ownerAComponent)
	err := recoverDocumentMetadataError(t, coordinator.commitUpdate)
	if !errors.Is(err, publicationFailure) {
		t.Fatalf("publication error = %v", err)
	}
	ownerB := documentMetadataOwnerAtStateSlot(ownerBComponent, 0)
	if ownerB == nil || ownerB.id != 0 || rendersB != 1 || cleanupA != 1 ||
		len(coordinator.pendingHandoffs) != 1 {
		t.Fatalf("failed handoff: B=%#v renders=%d cleanup A=%d", ownerB, rendersB, cleanupA)
	}
	assertDocumentMetadataSnapshot(t, coordinator, ownerA, metadataA, 1)

	failPublication = false
	coordinator.beginUpdate()
	renderComponentInstance(shell)
	coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(t, coordinator, ownerA, metadataA, 1)
	if document != metadataA || ownerB.id != 0 || rendersB != 1 ||
		rendersA != 1 || shellRenders != 1 ||
		coordinator.snapshot().retainedReleaseCount != 1 ||
		len(coordinator.pendingHandoffs) != 1 ||
		coordinator.statistics.baselineRestorations != 0 ||
		!reflect.DeepEqual(publications, []documentMetadataValue{metadataA}) {
		t.Fatalf("unrelated update consumed handoff: document=%#v A renders=%d B=%#v B renders=%d shell=%d snapshot=%#v statistics=%#v publications=%#v",
			document, rendersA, ownerB, rendersB, shellRenders,
			coordinator.snapshot(), coordinator.statistics, publications)
	}

	coordinator.beginUpdate()
	renderComponentInstance(ownerBComponent)
	coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(t, coordinator, ownerB, metadataB, 1)
	if document != metadataB || ownerB.id != 2 || rendersB != 2 || cleanupA != 1 ||
		ownerA.state != documentMetadataOwnerReleased ||
		coordinator.snapshot().retainedReleaseCount != 0 ||
		len(coordinator.pendingHandoffs) != 0 ||
		!reflect.DeepEqual(publications, []documentMetadataValue{metadataA, metadataB}) {
		t.Fatalf("causal retry: document=%#v A=%#v B=%#v renders=%d cleanup A=%d snapshot=%#v publications=%#v",
			document, ownerA, ownerB, rendersB, cleanupA,
			coordinator.snapshot(), publications)
	}
}

func TestDocumentMetadataPendingReplacementAbandonmentResolvesHandoff(t *testing.T) {
	baseline := documentMetadataValue{title: "Authored", description: "Baseline"}
	metadataA := documentMetadataValue{title: "A", description: "Description A"}
	metadataB := documentMetadataValue{title: "B", description: "Description B"}
	publicationFailure := errors.New("forced publication failure")
	document := baseline
	failPublication := false
	var publications []documentMetadataValue
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
			publications = append(publications, next)
			return nil
		},
		nil,
	)
	installDocumentMetadataCoordinator(coordinator)
	t.Cleanup(uninstallDocumentMetadataCoordinator)
	var runtimeErrors []ErrorInfo
	restoreErrors := SetErrorHandler(func(info ErrorInfo) {
		runtimeErrors = append(runtimeErrors, info)
	})
	t.Cleanup(restoreErrors)

	cleanupA := 0
	ownerAComponent := testComponentInstance("OwnerA", func() Node {
		useDocumentMetadata(metadataA)
		UseUnmount(func() {
			cleanupA++
		})
		return Empty()
	}, nil)
	cleanupB := 0
	rendersB := 0
	ownerBComponent := testComponentInstance("OwnerB", func() Node {
		rendersB++
		useDocumentMetadata(metadataB)
		UseUnmount(func() {
			cleanupB++
		})
		return Empty()
	}, nil)

	coordinator.beginUpdate()
	renderComponentInstance(ownerAComponent)
	coordinator.commitUpdate()
	ownerA := documentMetadataOwnerAtStateSlot(ownerAComponent, 0)

	failPublication = true
	coordinator.beginUpdate()
	renderComponentInstance(ownerBComponent)
	deactivateComponent(ownerAComponent)
	err := recoverDocumentMetadataError(t, coordinator.commitUpdate)
	if !errors.Is(err, publicationFailure) {
		t.Fatalf("publication error = %v", err)
	}
	ownerB := documentMetadataOwnerAtStateSlot(ownerBComponent, 0)
	if ownerB == nil || ownerB.id != 0 || cleanupA != 1 || rendersB != 1 {
		t.Fatalf("failed handoff: A cleanup=%d B=%#v renders=%d", cleanupA, ownerB, rendersB)
	}

	failPublication = false
	coordinator.beginUpdate()
	deactivateComponent(ownerBComponent)
	coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(t, coordinator, nil, baseline, 0)
	if document != baseline || ownerA.state != documentMetadataOwnerReleased ||
		ownerB.state != documentMetadataOwnerReleased || ownerB.id != 0 ||
		cleanupA != 1 || cleanupB != 1 || rendersB != 1 ||
		coordinator.snapshot().retainedReleaseCount != 0 ||
		len(coordinator.pendingHandoffs) != 0 || len(runtimeErrors) != 0 ||
		!reflect.DeepEqual(publications, []documentMetadataValue{metadataA, baseline}) {
		t.Fatalf("pending abandonment: document=%#v A=%#v B=%#v cleanups=%d/%d renders=%d snapshot=%#v errors=%#v publications=%#v",
			document, ownerA, ownerB, cleanupA, cleanupB, rendersB,
			coordinator.snapshot(), runtimeErrors, publications)
	}

	statistics := coordinator.statistics
	coordinator.beginUpdate()
	coordinator.commitUpdate()
	if document != baseline || coordinator.statistics.documentPublications != statistics.documentPublications ||
		coordinator.statistics.baselineRestorations != statistics.baselineRestorations ||
		len(publications) != 2 {
		t.Fatalf("empty update changed abandoned handoff: document=%#v before=%#v after=%#v publications=%#v",
			document, statistics, coordinator.statistics, publications)
	}
}

func TestDocumentMetadataLifecycleDetachSurvivesPublicationFailure(t *testing.T) {
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
	installDocumentMetadataCoordinator(coordinator)
	t.Cleanup(uninstallDocumentMetadataCoordinator)
	cleanupA := 0
	ownerAComponent := testComponentInstance("OwnerA", func() Node {
		useDocumentMetadata(metadataA)
		UseUnmount(func() {
			cleanupA++
		})
		return Empty()
	}, nil)
	ownerBComponent := testComponentInstance("OwnerB", func() Node {
		useDocumentMetadata(metadataB)
		return Empty()
	}, nil)

	coordinator.beginUpdate()
	renderComponentInstance(ownerAComponent)
	coordinator.commitUpdate()
	ownerA := documentMetadataOwnerAtStateSlot(ownerAComponent, 0)
	assertDocumentMetadataSnapshot(t, coordinator, ownerA, metadataA, 1)
	failPublication = true

	coordinator.beginUpdate()
	renderComponentInstance(ownerBComponent)
	deactivateComponent(ownerAComponent)
	if cleanupA != 1 {
		t.Fatalf("owner A cleanup count = %d, want 1", cleanupA)
	}
	err := recoverDocumentMetadataError(t, coordinator.commitUpdate)
	if !errors.Is(err, publicationFailure) {
		t.Fatalf("publication error = %v", err)
	}
	ownerB := documentMetadataOwnerAtStateSlot(ownerBComponent, 0)
	assertDocumentMetadataSnapshot(t, coordinator, ownerA, metadataA, 1)
	if document != metadataA || ownerB == nil || ownerB.id != 0 ||
		coordinator.snapshot().retainedReleaseCount != 1 {
		t.Fatalf("failed handoff: document=%#v B=%#v snapshot=%#v",
			document, ownerB, coordinator.snapshot())
	}

	failPublication = false
	coordinator.beginUpdate()
	renderComponentInstance(ownerBComponent)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, ownerB, metadataB, 1)
	if cleanupA != 1 || ownerA.state != documentMetadataOwnerReleased ||
		ownerA.boundary != nil || ownerB.id != 2 ||
		coordinator.snapshot().retainedReleaseCount != 0 {
		t.Fatalf("retried handoff: cleanup=%d A=%#v B=%#v snapshot=%#v",
			cleanupA, ownerA, ownerB, coordinator.snapshot())
	}

	coordinator.beginUpdate()
	deactivateComponent(ownerBComponent)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, nil, baseline, 0)
	if document != baseline {
		t.Fatalf("owner B unmount document = %#v, want baseline %#v", document, baseline)
	}
	assertDocumentMetadataCoordinatorFinalized(t, coordinator)
	if len(coordinator.retainedDetachIntents) != 0 || len(coordinator.retainedDetachSet) != 0 {
		t.Fatalf("lifecycle retry retained detach state: intents=%#v set=%#v",
			coordinator.retainedDetachIntents, coordinator.retainedDetachSet)
	}
}

func TestDocumentMetadataPublicationRetryPreservesTeardownRelease(t *testing.T) {
	baseline := documentMetadataValue{title: "Authored", description: "Baseline"}
	metadataA := documentMetadataValue{title: "A", description: "Description A"}
	metadataB := documentMetadataValue{title: "B", description: "Description B"}
	publicationFailure := errors.New("forced publication failure")
	failPublication := false
	coordinator := newDocumentMetadataCoordinator(
		baseline,
		func(_, _ documentMetadataValue) error {
			if failPublication {
				return publicationFailure
			}
			return nil
		},
		nil,
	)
	ownerA := coordinator.newOwner()
	ownerB := coordinator.newOwner()

	coordinator.beginUpdate()
	coordinator.stagePublish(ownerA, metadataA)
	coordinator.commitUpdate()
	failPublication = true

	coordinator.beginUpdate()
	coordinator.stagePublish(ownerB, metadataB)
	coordinator.stageRelease(ownerA)
	err := recoverDocumentMetadataError(t, coordinator.commitUpdate)
	if !errors.Is(err, publicationFailure) {
		t.Fatalf("publication error = %v", err)
	}
	assertDocumentMetadataSnapshot(t, coordinator, ownerA, metadataA, 1)

	failPublication = false
	coordinator.beginUpdate()
	coordinator.stagePublish(ownerB, metadataB)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, ownerB, metadataB, 1)
	if ownerA.state != documentMetadataOwnerReleased {
		t.Fatalf("owner A state = %v, want released", ownerA.state)
	}

	coordinator.beginUpdate()
	coordinator.stageRelease(ownerB)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, nil, baseline, 0)
	assertDocumentMetadataCoordinatorFinalized(t, coordinator)
}

func TestDocumentMetadataFinalReleaseRetriesAfterPublicationFailure(t *testing.T) {
	baseline := documentMetadataValue{title: "Authored", description: "Baseline"}
	metadataA := documentMetadataValue{title: "A", description: "Description A"}
	publicationFailure := errors.New("forced publication failure")
	failPublication := false
	coordinator := newDocumentMetadataCoordinator(
		baseline,
		func(_, _ documentMetadataValue) error {
			if failPublication {
				return publicationFailure
			}
			return nil
		},
		nil,
	)
	ownerA := coordinator.newOwner()
	coordinator.beginUpdate()
	coordinator.stagePublish(ownerA, metadataA)
	coordinator.commitUpdate()
	failPublication = true

	coordinator.beginUpdate()
	coordinator.stageRelease(ownerA)
	err := recoverDocumentMetadataError(t, coordinator.commitUpdate)
	if !errors.Is(err, publicationFailure) {
		t.Fatalf("publication error = %v", err)
	}
	assertDocumentMetadataSnapshot(t, coordinator, ownerA, metadataA, 1)
	if coordinator.snapshot().retainedReleaseCount != 1 ||
		len(coordinator.retainedDetachIntents) != 1 {
		t.Fatalf("failed final release did not retain detach intent: snapshot=%#v intents=%#v",
			coordinator.snapshot(), coordinator.retainedDetachIntents)
	}

	failPublication = false
	coordinator.beginUpdate()
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, nil, baseline, 0)
	assertDocumentMetadataCoordinatorFinalized(t, coordinator)
	if len(coordinator.retainedDetachIntents) != 0 || len(coordinator.retainedDetachSet) != 0 {
		t.Fatalf("final release retained detach state: intents=%#v set=%#v",
			coordinator.retainedDetachIntents, coordinator.retainedDetachSet)
	}
}

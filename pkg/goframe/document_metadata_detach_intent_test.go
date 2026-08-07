package goframe

import (
	"errors"
	"testing"
)

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

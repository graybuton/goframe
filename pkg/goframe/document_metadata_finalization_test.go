package goframe

import (
	"errors"
	"reflect"
	"testing"
)

func TestDocumentMetadataOwnerlessRecoveryFinalizationSurvivesPublicationFailure(t *testing.T) {
	fixture := newFailedDocumentMetadataBoundaryFixture(t)
	fixture.failPublication = true
	clearErrorBoundary(fixture.boundary)
	resetGeneration := fixture.boundary.generation

	fixture.coordinator.beginUpdate()
	previous := beginProtectedSubtreeLifecycle(fixture.boundary)
	finishProtectedSubtreeLifecycle(fixture.boundary, previous)
	err := recoverDocumentMetadataError(t, fixture.coordinator.commitUpdate)
	if !errors.Is(err, fixture.publicationFailure) {
		t.Fatalf("ownerless recovery publication error = %v", err)
	}
	assertDocumentMetadataSnapshot(
		t,
		fixture.coordinator,
		fixture.ownerA,
		fixture.metadataA,
		1,
	)
	if fixture.document != fixture.metadataA || fixture.boundary.generation != resetGeneration ||
		fixture.cleanupA != 1 || fixture.coordinator.snapshot().failedBoundaryCount != 1 ||
		fixture.coordinator.snapshot().retainedReleaseCount != 1 ||
		fixture.coordinator.snapshot().batchActive ||
		len(fixture.coordinator.pendingFinalizations) != 1 ||
		fixture.coordinator.pendingFinalizations[fixture.boundary] == nil ||
		fixture.coordinator.pendingFinalizations[fixture.boundary].kind !=
			documentMetadataBoundaryRecovered {
		t.Fatalf("failed ownerless finalization: document=%#v generation=%d cleanup=%d snapshot=%#v",
			fixture.document, fixture.boundary.generation, fixture.cleanupA,
			fixture.coordinator.snapshot())
	}
	fixture.coordinator.beginUpdate()
	err = recoverDocumentMetadataError(t, fixture.coordinator.commitUpdate)
	if !errors.Is(err, fixture.publicationFailure) ||
		len(fixture.coordinator.pendingFinalizations) != 1 ||
		fixture.boundary.generation != resetGeneration || fixture.cleanupA != 1 {
		t.Fatalf("repeated ownerless finalization failure: error=%v generation=%d cleanup=%d finalizations=%#v",
			err, fixture.boundary.generation, fixture.cleanupA,
			fixture.coordinator.pendingFinalizations)
	}
	assertDocumentMetadataSnapshot(
		t,
		fixture.coordinator,
		fixture.ownerA,
		fixture.metadataA,
		1,
	)

	fixture.failPublication = false
	fixture.coordinator.beginUpdate()
	fixture.coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(
		t,
		fixture.coordinator,
		nil,
		fixture.baseline,
		0,
	)
	if fixture.document != fixture.baseline || fixture.boundary.generation != resetGeneration ||
		fixture.cleanupA != 1 || fixture.ownerA.state != documentMetadataOwnerReleased ||
		fixture.ownerA.boundary != nil ||
		!reflect.DeepEqual(fixture.publications, []documentMetadataValue{
			fixture.metadataA,
			fixture.baseline,
		}) {
		t.Fatalf("ownerless finalization retry: document=%#v generation=%d cleanup=%d owner=%#v publications=%#v",
			fixture.document, fixture.boundary.generation, fixture.cleanupA,
			fixture.ownerA, fixture.publications)
	}
	assertDocumentMetadataCoordinatorFinalized(t, fixture.coordinator)
}

func TestDocumentMetadataBoundaryAbandonmentFinalizationSurvivesPublicationFailure(t *testing.T) {
	fixture := newFailedDocumentMetadataBoundaryFixture(t)
	boundaryUnmounts := 0
	fixture.boundaryOwner.unmountSlots = append(
		fixture.boundaryOwner.unmountSlots,
		func() { boundaryUnmounts++ },
	)
	fixture.failPublication = true

	fixture.coordinator.beginUpdate()
	deactivateComponent(fixture.boundaryOwner)
	err := recoverDocumentMetadataError(t, fixture.coordinator.commitUpdate)
	if !errors.Is(err, fixture.publicationFailure) {
		t.Fatalf("boundary abandonment publication error = %v", err)
	}
	assertDocumentMetadataSnapshot(
		t,
		fixture.coordinator,
		fixture.ownerA,
		fixture.metadataA,
		1,
	)
	if fixture.document != fixture.metadataA || fixture.cleanupA != 1 || boundaryUnmounts != 1 ||
		fixture.coordinator.snapshot().failedBoundaryCount != 1 ||
		fixture.coordinator.snapshot().retainedReleaseCount != 1 ||
		fixture.coordinator.snapshot().batchActive ||
		len(fixture.coordinator.pendingFinalizations) != 1 ||
		fixture.coordinator.pendingFinalizations[fixture.boundary] == nil ||
		fixture.coordinator.pendingFinalizations[fixture.boundary].kind !=
			documentMetadataAbandonBoundary {
		t.Fatalf("failed abandonment finalization: document=%#v cleanup=%d boundary unmounts=%d snapshot=%#v",
			fixture.document, fixture.cleanupA, boundaryUnmounts,
			fixture.coordinator.snapshot())
	}

	fixture.failPublication = false
	fixture.coordinator.beginUpdate()
	fixture.coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(
		t,
		fixture.coordinator,
		nil,
		fixture.baseline,
		0,
	)
	if fixture.document != fixture.baseline || fixture.cleanupA != 1 || boundaryUnmounts != 1 ||
		fixture.ownerA.state != documentMetadataOwnerReleased || fixture.ownerA.boundary != nil ||
		!reflect.DeepEqual(fixture.publications, []documentMetadataValue{
			fixture.metadataA,
			fixture.baseline,
		}) {
		t.Fatalf("abandonment finalization retry: document=%#v cleanup=%d boundary unmounts=%d owner=%#v publications=%#v",
			fixture.document, fixture.cleanupA, boundaryUnmounts,
			fixture.ownerA, fixture.publications)
	}
	assertDocumentMetadataCoordinatorFinalized(t, fixture.coordinator)
}

func TestDocumentMetadataNestedFinalizationSurvivesPublicationFailure(t *testing.T) {
	baseline := documentMetadataValue{title: "Authored", description: "Baseline"}
	metadataA := documentMetadataValue{title: "A", description: "Description A"}
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

	outerOwner := testComponentInstance("OuterBoundary", func() Node { return Empty() }, nil)
	outer := ensureErrorBoundaryState(outerOwner)
	innerOwner := testComponentInstanceWithParent(
		"InnerBoundary",
		outerOwner,
		func() Node { return Empty() },
	)
	inner := ensureErrorBoundaryState(innerOwner)
	cleanupA := 0
	ownerAComponent := testComponentInstanceWithParent("OwnerA", innerOwner, func() Node {
		useDocumentMetadata(metadataA)
		UseUnmount(func() { cleanupA++ })
		return Empty()
	})

	coordinator.beginUpdate()
	previousOuter := beginProtectedSubtreeLifecycle(outer)
	previousInner := beginProtectedSubtreeLifecycle(inner)
	renderComponentInstance(ownerAComponent)
	finishProtectedSubtreeLifecycle(inner, previousInner)
	finishProtectedSubtreeLifecycle(outer, previousOuter)
	coordinator.commitUpdate()
	ownerA := documentMetadataOwnerAtStateSlot(ownerAComponent, 0)
	if ownerA == nil || ownerA.boundary != outer {
		t.Fatalf("nested owner = %#v, want final outer boundary %p", ownerA, outer)
	}

	failingSibling := testComponentInstanceWithParent("FailingSibling", outerOwner, func() Node {
		panic("outer sibling failed")
	})
	coordinator.beginUpdate()
	previousOuter = beginProtectedSubtreeLifecycle(outer)
	renderComponentInstance(failingSibling)
	finishProtectedSubtreeLifecycle(outer, previousOuter)
	deactivateComponent(ownerAComponent)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, ownerA, metadataA, 1)
	if cleanupA != 1 || !coordinator.failedBoundaries[outer] ||
		coordinator.failedBoundaries[inner] || coordinator.retainedReleases[inner] != nil {
		t.Fatalf("nested failure: cleanup=%d failed=%#v retained=%#v",
			cleanupA, coordinator.failedBoundaries, coordinator.retainedReleases)
	}

	clearErrorBoundary(outer)
	resetGeneration := outer.generation
	failPublication = true
	coordinator.beginUpdate()
	previousOuter = beginProtectedSubtreeLifecycle(outer)
	previousInner = beginProtectedSubtreeLifecycle(inner)
	finishProtectedSubtreeLifecycle(inner, previousInner)
	finishProtectedSubtreeLifecycle(outer, previousOuter)
	err := recoverDocumentMetadataError(t, coordinator.commitUpdate)
	if !errors.Is(err, publicationFailure) {
		t.Fatalf("nested finalization publication error = %v", err)
	}
	assertDocumentMetadataSnapshot(t, coordinator, ownerA, metadataA, 1)
	if document != metadataA || cleanupA != 1 || outer.generation != resetGeneration ||
		!coordinator.failedBoundaries[outer] || coordinator.failedBoundaries[inner] ||
		coordinator.retainedReleases[inner] != nil ||
		len(coordinator.pendingFinalizations) != 1 ||
		coordinator.pendingFinalizations[outer] == nil ||
		coordinator.pendingFinalizations[inner] != nil {
		t.Fatalf("failed nested finalization: document=%#v cleanup=%d generation=%d failed=%#v retained=%#v",
			document, cleanupA, outer.generation,
			coordinator.failedBoundaries, coordinator.retainedReleases)
	}

	failPublication = false
	coordinator.beginUpdate()
	coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(t, coordinator, nil, baseline, 0)
	if document != baseline || cleanupA != 1 || outer.generation != resetGeneration ||
		ownerA.state != documentMetadataOwnerReleased || ownerA.boundary != nil ||
		coordinator.failedBoundaries[outer] || coordinator.failedBoundaries[inner] ||
		coordinator.retainedReleases[outer] != nil || coordinator.retainedReleases[inner] != nil ||
		!reflect.DeepEqual(publications, []documentMetadataValue{metadataA, baseline}) {
		t.Fatalf("nested finalization retry: document=%#v cleanup=%d generation=%d owner=%#v failed=%#v retained=%#v publications=%#v",
			document, cleanupA, outer.generation, ownerA,
			coordinator.failedBoundaries, coordinator.retainedReleases, publications)
	}
	assertDocumentMetadataCoordinatorFinalized(t, coordinator)
}

func TestDocumentMetadataBoundaryPendingReplacementAbandonmentResolvesHandoff(t *testing.T) {
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

	boundaryOwner := testComponentInstance("Boundary", func() Node { return Empty() }, nil)
	boundary := ensureErrorBoundaryState(boundaryOwner)
	cleanupA := 0
	ownerAComponent := testComponentInstanceWithParent("OwnerA", boundaryOwner, func() Node {
		useDocumentMetadata(metadataA)
		UseUnmount(func() { cleanupA++ })
		return Empty()
	})
	cleanupB := 0
	ownerBComponent := testComponentInstanceWithParent("OwnerB", boundaryOwner, func() Node {
		useDocumentMetadata(metadataB)
		UseUnmount(func() { cleanupB++ })
		return Empty()
	})

	coordinator.beginUpdate()
	previous := beginProtectedSubtreeLifecycle(boundary)
	renderComponentInstance(ownerAComponent)
	finishProtectedSubtreeLifecycle(boundary, previous)
	coordinator.commitUpdate()
	ownerA := documentMetadataOwnerAtStateSlot(ownerAComponent, 0)

	failing := testComponentInstanceWithParent("Failing", boundaryOwner, func() Node {
		panic("failed before document metadata")
	})
	coordinator.beginUpdate()
	previous = beginProtectedSubtreeLifecycle(boundary)
	renderComponentInstance(failing)
	finishProtectedSubtreeLifecycle(boundary, previous)
	deactivateComponent(ownerAComponent)
	coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, coordinator, ownerA, metadataA, 1)

	clearErrorBoundary(boundary)
	failPublication = true
	coordinator.beginUpdate()
	previous = beginProtectedSubtreeLifecycle(boundary)
	renderComponentInstance(ownerBComponent)
	finishProtectedSubtreeLifecycle(boundary, previous)
	err := recoverDocumentMetadataError(t, coordinator.commitUpdate)
	if !errors.Is(err, publicationFailure) {
		t.Fatalf("replacement publication error = %v", err)
	}
	ownerB := documentMetadataOwnerAtStateSlot(ownerBComponent, 0)
	if ownerB == nil || ownerB.id != 0 || document != metadataA || cleanupA != 1 ||
		len(coordinator.pendingHandoffs) != 1 ||
		len(coordinator.pendingFinalizations) != 0 {
		t.Fatalf("failed boundary handoff: document=%#v A cleanup=%d B=%#v handoffs=%#v finalizations=%#v",
			document, cleanupA, ownerB,
			coordinator.pendingHandoffs, coordinator.pendingFinalizations)
	}

	failPublication = false
	coordinator.beginUpdate()
	deactivateComponent(ownerBComponent)
	coordinator.commitUpdate()

	assertDocumentMetadataSnapshot(t, coordinator, nil, baseline, 0)
	if document != baseline || cleanupA != 1 || cleanupB != 1 || ownerB.id != 0 ||
		ownerB.state != documentMetadataOwnerReleased {
		t.Fatalf("boundary successor abandonment: document=%#v cleanups=%d/%d B=%#v",
			document, cleanupA, cleanupB, ownerB)
	}
	assertDocumentMetadataCoordinatorFinalized(t, coordinator)
}

type failedDocumentMetadataBoundaryFixture struct {
	baseline           documentMetadataValue
	metadataA          documentMetadataValue
	publicationFailure error
	document           documentMetadataValue
	failPublication    bool
	publications       []documentMetadataValue
	coordinator        *documentMetadataCoordinator
	boundaryOwner      *componentInstance
	boundary           *errorBoundaryState
	ownerA             *documentMetadataOwner
	cleanupA           int
}

func newFailedDocumentMetadataBoundaryFixture(
	t *testing.T,
) *failedDocumentMetadataBoundaryFixture {
	t.Helper()
	fixture := &failedDocumentMetadataBoundaryFixture{
		baseline:           documentMetadataValue{title: "Authored", description: "Baseline"},
		metadataA:          documentMetadataValue{title: "A", description: "Description A"},
		publicationFailure: errors.New("forced publication failure"),
	}
	fixture.document = fixture.baseline
	fixture.coordinator = newDocumentMetadataCoordinator(
		fixture.baseline,
		func(previous, next documentMetadataValue) error {
			if fixture.document != previous {
				t.Fatalf("publisher previous=%#v document=%#v", previous, fixture.document)
			}
			if fixture.failPublication {
				return fixture.publicationFailure
			}
			fixture.document = next
			fixture.publications = append(fixture.publications, next)
			return nil
		},
		nil,
	)
	installDocumentMetadataCoordinator(fixture.coordinator)
	t.Cleanup(uninstallDocumentMetadataCoordinator)
	fixture.boundaryOwner = testComponentInstance("Boundary", func() Node { return Empty() }, nil)
	fixture.boundary = ensureErrorBoundaryState(fixture.boundaryOwner)
	ownerAComponent := testComponentInstanceWithParent(
		"OwnerA",
		fixture.boundaryOwner,
		func() Node {
			useDocumentMetadata(fixture.metadataA)
			UseUnmount(func() { fixture.cleanupA++ })
			return Empty()
		},
	)

	fixture.coordinator.beginUpdate()
	previous := beginProtectedSubtreeLifecycle(fixture.boundary)
	renderComponentInstance(ownerAComponent)
	finishProtectedSubtreeLifecycle(fixture.boundary, previous)
	fixture.coordinator.commitUpdate()
	fixture.ownerA = documentMetadataOwnerAtStateSlot(ownerAComponent, 0)
	assertDocumentMetadataSnapshot(
		t,
		fixture.coordinator,
		fixture.ownerA,
		fixture.metadataA,
		1,
	)

	failing := testComponentInstanceWithParent(
		"Failing",
		fixture.boundaryOwner,
		func() Node { panic("failed before document metadata") },
	)
	fixture.coordinator.beginUpdate()
	previous = beginProtectedSubtreeLifecycle(fixture.boundary)
	renderComponentInstance(failing)
	finishProtectedSubtreeLifecycle(fixture.boundary, previous)
	deactivateComponent(ownerAComponent)
	fixture.coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(
		t,
		fixture.coordinator,
		fixture.ownerA,
		fixture.metadataA,
		1,
	)
	if fixture.cleanupA != 1 || fixture.coordinator.snapshot().failedBoundaryCount != 1 ||
		fixture.coordinator.snapshot().retainedReleaseCount != 1 {
		t.Fatalf("failed boundary fixture: cleanup=%d snapshot=%#v",
			fixture.cleanupA, fixture.coordinator.snapshot())
	}
	return fixture
}

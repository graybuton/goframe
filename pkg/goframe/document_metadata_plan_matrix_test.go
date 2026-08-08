package goframe

import (
	"errors"
	"reflect"
	"testing"
)

type documentMetadataPlanHarness struct {
	t                  *testing.T
	baseline           documentMetadataValue
	document           documentMetadataValue
	publicationFailure error
	failPublications   int
	publications       []documentMetadataValue
	coordinator        *documentMetadataCoordinator
}

type documentMetadataPlanOwner struct {
	component *componentInstance
	owner     *documentMetadataOwner
	metadata  documentMetadataValue
	renders   int
	cleanups  int
}

func newDocumentMetadataPlanHarness(t *testing.T) *documentMetadataPlanHarness {
	t.Helper()
	harness := &documentMetadataPlanHarness{
		t:                  t,
		baseline:           documentMetadataValue{title: "Authored", description: "Baseline"},
		publicationFailure: errors.New("forced plan publication failure"),
	}
	harness.document = harness.baseline
	harness.coordinator = newDocumentMetadataCoordinator(
		harness.baseline,
		func(previous, next documentMetadataValue) error {
			if harness.document != previous {
				t.Fatalf("publisher previous=%#v document=%#v", previous, harness.document)
			}
			if harness.failPublications > 0 {
				harness.failPublications--
				return harness.publicationFailure
			}
			harness.document = next
			harness.publications = append(harness.publications, next)
			return nil
		},
		nil,
	)
	installDocumentMetadataCoordinator(harness.coordinator)
	t.Cleanup(uninstallDocumentMetadataCoordinator)
	return harness
}

func (harness *documentMetadataPlanHarness) newOwner(
	name string,
	parent *componentInstance,
	metadata documentMetadataValue,
) *documentMetadataPlanOwner {
	harness.t.Helper()
	owner := &documentMetadataPlanOwner{metadata: metadata}
	render := func() Node {
		owner.renders++
		useDocumentMetadata(owner.metadata)
		UseUnmount(func() { owner.cleanups++ })
		return Empty()
	}
	if parent == nil {
		owner.component = testComponentInstance(name, render, nil)
	} else {
		owner.component = testComponentInstanceWithParent(name, parent, render)
	}
	return owner
}

func (owner *documentMetadataPlanOwner) token() *documentMetadataOwner {
	if current := documentMetadataOwnerAtStateSlot(owner.component, 0); current != nil {
		owner.owner = current
	}
	return owner.owner
}

func (harness *documentMetadataPlanHarness) commitInitial(
	owner *documentMetadataPlanOwner,
) *documentMetadataOwner {
	harness.t.Helper()
	harness.coordinator.beginUpdate()
	renderComponentInstance(owner.component)
	harness.coordinator.commitUpdate()
	token := owner.token()
	if token == nil || token.id == 0 {
		harness.t.Fatalf("initial owner = %#v, want committed token", token)
	}
	return token
}

func (harness *documentMetadataPlanHarness) failReplacement(
	outgoing *documentMetadataPlanOwner,
	pending ...*documentMetadataPlanOwner,
) error {
	harness.t.Helper()
	harness.failPublications++
	harness.coordinator.beginUpdate()
	for _, owner := range pending {
		renderComponentInstance(owner.component)
	}
	deactivateComponent(outgoing.component)
	err := recoverDocumentMetadataError(harness.t, harness.coordinator.commitUpdate)
	if !errors.Is(err, harness.publicationFailure) {
		harness.t.Fatalf("replacement error = %v", err)
	}
	return err
}

func (harness *documentMetadataPlanHarness) renderAndCommit(
	owners ...*documentMetadataPlanOwner,
) {
	harness.t.Helper()
	harness.coordinator.beginUpdate()
	for _, owner := range owners {
		renderComponentInstance(owner.component)
	}
	harness.coordinator.commitUpdate()
}

func (harness *documentMetadataPlanHarness) renderAndFail(
	owners ...*documentMetadataPlanOwner,
) error {
	harness.t.Helper()
	harness.failPublications++
	harness.coordinator.beginUpdate()
	for _, owner := range owners {
		renderComponentInstance(owner.component)
	}
	err := recoverDocumentMetadataError(harness.t, harness.coordinator.commitUpdate)
	if !errors.Is(err, harness.publicationFailure) {
		harness.t.Fatalf("publication error = %v", err)
	}
	return err
}

func documentMetadataPlanValue(name string) documentMetadataValue {
	return documentMetadataValue{
		title:       name,
		description: "Description " + name,
	}
}

func assertDocumentMetadataPlanCommitted(
	t *testing.T,
	harness *documentMetadataPlanHarness,
	selected *documentMetadataPlanOwner,
	owners ...*documentMetadataPlanOwner,
) {
	t.Helper()
	selectedToken := selected.token()
	assertDocumentMetadataSnapshot(
		t,
		harness.coordinator,
		selectedToken,
		selected.metadata,
		len(owners),
	)
	if harness.document != selected.metadata {
		t.Fatalf("document = %#v, want %#v", harness.document, selected.metadata)
	}
	if len(harness.coordinator.owners) != len(owners) {
		t.Fatalf("owner records = %#v, want %d", harness.coordinator.owners, len(owners))
	}
	for index, owner := range owners {
		if harness.coordinator.owners[index].owner != owner.token() ||
			harness.coordinator.owners[index].metadata != owner.metadata {
			t.Fatalf("owner[%d] = %#v, want token=%p metadata=%#v",
				index, harness.coordinator.owners[index], owner.token(), owner.metadata)
		}
	}
}

func assertDocumentMetadataPlanPending(
	t *testing.T,
	harness *documentMetadataPlanHarness,
	committed *documentMetadataPlanOwner,
	pending ...*documentMetadataPlanOwner,
) {
	t.Helper()
	assertDocumentMetadataSnapshot(
		t,
		harness.coordinator,
		committed.token(),
		committed.metadata,
		1,
	)
	if harness.document != committed.metadata || len(harness.coordinator.pendingHandoffOrder) != 1 {
		t.Fatalf("pending plan: document=%#v handoffs=%#v",
			harness.document, harness.coordinator.pendingHandoffOrder)
	}
	handoff := harness.coordinator.pendingHandoffOrder[0]
	if handoff.id == 0 || len(handoff.owners) != len(pending) {
		t.Fatalf("pending owners = %#v, want %d", handoff.owners, len(pending))
	}
	for index, owner := range pending {
		token := owner.token()
		if token == nil || token.id != 0 || token.state != documentMetadataOwnerPending ||
			handoff.owners[index].owner != token ||
			handoff.owners[index].metadata != owner.metadata ||
			harness.coordinator.pendingHandoffs[token] != handoff {
			t.Fatalf("pending owner[%d] token=%#v plan=%#v", index, token, handoff)
		}
	}
}

func assertDocumentMetadataPlanReadiness(
	t *testing.T,
	harness *documentMetadataPlanHarness,
	ready ...*documentMetadataPlanOwner,
) {
	t.Helper()
	readySet := make(map[*documentMetadataOwner]bool, len(ready))
	for _, owner := range ready {
		readySet[owner.token()] = true
	}
	for _, handoff := range harness.coordinator.pendingHandoffOrder {
		for _, pending := range handoff.owners {
			if pending.ready != readySet[pending.owner] {
				t.Fatalf("pending owner %p ready=%t, want %t",
					pending.owner, pending.ready, readySet[pending.owner])
			}
		}
	}
}

func TestDocumentMetadataPendingOwnerRetryPreservesOriginalPriority(t *testing.T) {
	runDocumentMetadataPendingOwnerRetryPreservesOriginalPriority(t)
}

func runDocumentMetadataPendingOwnerRetryPreservesOriginalPriority(t *testing.T) {
	harness := newDocumentMetadataPlanHarness(t)
	ownerA := harness.newOwner("OwnerA", nil, documentMetadataPlanValue("A"))
	ownerB := harness.newOwner("OwnerB", nil, documentMetadataPlanValue("B"))
	ownerC := harness.newOwner("OwnerC", nil, documentMetadataPlanValue("C"))
	harness.commitInitial(ownerA)
	harness.failReplacement(ownerA, ownerB, ownerC)
	assertDocumentMetadataPlanPending(t, harness, ownerA, ownerB, ownerC)
	assertDocumentMetadataPlanReadiness(t, harness)

	harness.renderAndCommit(ownerC, ownerB)

	assertDocumentMetadataPlanCommitted(t, harness, ownerC, ownerB, ownerC)
	if ownerB.token().id != 2 || ownerC.token().id != 3 || ownerA.token().state != documentMetadataOwnerReleased ||
		!reflect.DeepEqual(harness.publications, []documentMetadataValue{ownerA.metadata, ownerC.metadata}) {
		t.Fatalf("reverse retry: ids=%v A=%#v publications=%#v",
			harness.coordinator.ownerIDs(), ownerA.token(), harness.publications)
	}
	assertDocumentMetadataCoordinatorFinalized(t, harness.coordinator)
}

func TestDocumentMetadataPendingOwnerProgressPersistsAcrossUpdates(t *testing.T) {
	runDocumentMetadataPendingOwnerProgressPersistsAcrossUpdates(t)
}

func runDocumentMetadataPendingOwnerProgressPersistsAcrossUpdates(t *testing.T) {
	harness := newDocumentMetadataPlanHarness(t)
	ownerA := harness.newOwner("OwnerA", nil, documentMetadataPlanValue("A"))
	ownerB := harness.newOwner("OwnerB", nil, documentMetadataPlanValue("B"))
	ownerC := harness.newOwner("OwnerC", nil, documentMetadataPlanValue("C"))
	harness.commitInitial(ownerA)
	harness.failReplacement(ownerA, ownerB, ownerC)
	assertDocumentMetadataPlanReadiness(t, harness)

	ownerB.metadata = documentMetadataPlanValue("B2")
	harness.renderAndCommit(ownerB)
	assertDocumentMetadataPlanPending(t, harness, ownerA, ownerB, ownerC)
	assertDocumentMetadataPlanReadiness(t, harness, ownerB)
	if len(harness.publications) != 1 || ownerC.renders != 1 {
		t.Fatalf("partial B progress: publications=%#v C renders=%d", harness.publications, ownerC.renders)
	}

	harness.renderAndCommit(ownerC)

	assertDocumentMetadataPlanCommitted(t, harness, ownerC, ownerB, ownerC)
	if harness.coordinator.owners[0].metadata != ownerB.metadata ||
		ownerB.renders != 2 || ownerC.renders != 2 ||
		!reflect.DeepEqual(harness.publications, []documentMetadataValue{ownerA.metadata, ownerC.metadata}) {
		t.Fatalf("partial progress resolution: owners=%#v renders=%d/%d publications=%#v",
			harness.coordinator.owners, ownerB.renders, ownerC.renders, harness.publications)
	}
	assertDocumentMetadataCoordinatorFinalized(t, harness.coordinator)
}

func TestDocumentMetadataExternalSuccessorAbsorbsPendingFinalization(t *testing.T) {
	runDocumentMetadataSuccessorAbsorbsPendingFinalization(t, false)
}

func TestDocumentMetadataCrossBoundarySuccessorAbsorbsPendingFinalization(t *testing.T) {
	runDocumentMetadataSuccessorAbsorbsPendingFinalization(t, true)
}

func runDocumentMetadataSuccessorAbsorbsPendingFinalization(
	t *testing.T,
	crossBoundary bool,
) {
	fixture := newFailedDocumentMetadataBoundaryFixture(t)
	fixture.failPublication = true
	clearErrorBoundary(fixture.boundary)
	fixture.coordinator.beginUpdate()
	previous := beginProtectedSubtreeLifecycle(fixture.boundary)
	finishProtectedSubtreeLifecycle(fixture.boundary, previous)
	err := recoverDocumentMetadataError(t, fixture.coordinator.commitUpdate)
	if !errors.Is(err, fixture.publicationFailure) {
		t.Fatalf("ownerless finalization error = %v", err)
	}
	if len(fixture.coordinator.pendingFinalizations) != 1 {
		t.Fatalf("standalone finalizations = %#v, want one", fixture.coordinator.pendingFinalizations)
	}

	metadataB := documentMetadataPlanValue("B")
	var boundaryOwner *componentInstance
	var boundary *errorBoundaryState
	if crossBoundary {
		boundaryOwner = testComponentInstance("BoundaryY", func() Node { return Empty() }, nil)
		boundary = ensureErrorBoundaryState(boundaryOwner)
	}
	rendersB := 0
	ownerBComponent := func() *componentInstance {
		render := func() Node {
			rendersB++
			useDocumentMetadata(metadataB)
			return Empty()
		}
		if boundaryOwner == nil {
			return testComponentInstance("OwnerB", render, nil)
		}
		return testComponentInstanceWithParent("OwnerB", boundaryOwner, render)
	}()

	fixture.coordinator.beginUpdate()
	if boundary != nil {
		previous = beginProtectedSubtreeLifecycle(boundary)
	}
	renderComponentInstance(ownerBComponent)
	if boundary != nil {
		finishProtectedSubtreeLifecycle(boundary, previous)
	}
	err = recoverDocumentMetadataError(t, fixture.coordinator.commitUpdate)
	if !errors.Is(err, fixture.publicationFailure) {
		t.Fatalf("successor publication error = %v", err)
	}
	ownerB := documentMetadataOwnerAtStateSlot(ownerBComponent, 0)
	handoff := fixture.coordinator.pendingHandoffs[ownerB]
	if ownerB == nil || ownerB.id != 0 || handoff == nil ||
		len(handoff.finalizations) != 1 || handoff.finalizations[0].boundary != fixture.boundary ||
		len(fixture.coordinator.pendingFinalizations) != 0 {
		t.Fatalf("absorbed finalization: B=%#v handoff=%#v standalone=%#v",
			ownerB, handoff, fixture.coordinator.pendingFinalizations)
	}

	fixture.failPublication = false
	shellRenders := 0
	shell := testComponentInstance("Shell", func() Node {
		shellRenders++
		return Empty()
	}, nil)
	fixture.coordinator.beginUpdate()
	renderComponentInstance(shell)
	fixture.coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, fixture.coordinator, fixture.ownerA, fixture.metadataA, 1)
	if fixture.document != fixture.metadataA || ownerB.id != 0 || rendersB != 1 ||
		shellRenders != 1 || len(fixture.publications) != 1 {
		t.Fatalf("unrelated update: document=%#v B=%#v renders=%d shell=%d publications=%#v",
			fixture.document, ownerB, rendersB, shellRenders, fixture.publications)
	}

	fixture.coordinator.beginUpdate()
	if boundary != nil {
		previous = beginProtectedSubtreeLifecycle(boundary)
	}
	renderComponentInstance(ownerBComponent)
	if boundary != nil {
		finishProtectedSubtreeLifecycle(boundary, previous)
	}
	fixture.coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, fixture.coordinator, ownerB, metadataB, 1)
	if fixture.ownerA.state != documentMetadataOwnerReleased || ownerB.id != 2 ||
		rendersB != 2 || fixture.cleanupA != 1 ||
		!reflect.DeepEqual(fixture.publications, []documentMetadataValue{fixture.metadataA, metadataB}) {
		t.Fatalf("successor retry: A=%#v B=%#v renders=%d cleanup=%d publications=%#v",
			fixture.ownerA, ownerB, rendersB, fixture.cleanupA, fixture.publications)
	}
	assertDocumentMetadataCoordinatorFinalized(t, fixture.coordinator)
}

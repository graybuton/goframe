package goframe

import (
	"errors"
	"fmt"
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

func TestDocumentMetadataUnresolvedOwnershipPlanMatrix(t *testing.T) {
	cells := []struct {
		name string
		run  func(*testing.T)
	}{
		{"01_direct_successful_A_to_B", runDocumentMetadataPlanDirectSuccess},
		{"02_failed_A_to_B_unrelated_then_retry", runDocumentMetadataFailedHandoffIgnoresUnrelatedMatrix},
		{"03_failed_A_to_B_B_abandonment", runDocumentMetadataPendingAbandonmentMatrix},
		{"04_repeated_failed_B_then_retry", runDocumentMetadataRepeatedFailureMatrix},
		{"05_B_latest_metadata_changes", runDocumentMetadataLatestPendingMetadataMatrix},
		{"06_B_C_retry_original_order", func(t *testing.T) { runDocumentMetadataTwoOwnerRetryMatrix(t, false) }},
		{"07_B_C_retry_reverse_order", runDocumentMetadataPendingOwnerRetryPreservesOriginalPriority},
		{"08_B_ready_then_C_ready", runDocumentMetadataPendingOwnerProgressPersistsAcrossUpdates},
		{"09_C_ready_then_B_ready", runDocumentMetadataReversePartialProgressMatrix},
		{"10_B_ready_C_abandoned", func(t *testing.T) { runDocumentMetadataReadyThenAbandonedMatrix(t, true) }},
		{"11_C_ready_B_abandoned", func(t *testing.T) { runDocumentMetadataReadyThenAbandonedMatrix(t, false) }},
		{"12_C_abandoned_then_B_retry", func(t *testing.T) { runDocumentMetadataAbandonThenRetryMatrix(t, true) }},
		{"13_B_abandoned_then_C_retry", func(t *testing.T) { runDocumentMetadataAbandonThenRetryMatrix(t, false) }},
		{"14_all_pending_abandoned", runDocumentMetadataAllPendingAbandonedMatrix},
		{"15_second_publication_failure_after_ready", runDocumentMetadataSecondReadyFailureMatrix},
		{"16_three_owner_publish_permutations", runDocumentMetadataThreeOwnerPermutationsMatrix},
		{"17_pending_owner_newer_boundary_failure", TestDocumentMetadataPendingHandoffDoesNotRecoverNewBoundaryFailure},
		{"18_finalization_successor_same_boundary", TestDocumentMetadataNewSuccessorSupersedesPendingOwnerlessFinalization},
		{"19_finalization_successor_outside_boundary", func(t *testing.T) { runDocumentMetadataSuccessorAbsorbsPendingFinalization(t, false) }},
		{"20_finalization_successor_cross_boundary", func(t *testing.T) { runDocumentMetadataSuccessorAbsorbsPendingFinalization(t, true) }},
		{"21_abandonment_finalization_external_successor", runDocumentMetadataAbandonmentFinalizationSuccessorMatrix},
		{"22_nested_final_outer_recovery", TestDocumentMetadataNestedFinalizationSurvivesPublicationFailure},
		{"23_new_outer_failure_supersedes_recovery", runDocumentMetadataOuterFailureSupersedesFinalizationMatrix},
		{"24_finalization_multiple_reverse_retry", runDocumentMetadataFinalizationMultipleOwnersMatrix},
		{"25_selected_release_reveals_parent", runDocumentMetadataSelectedReleaseRevealsParentMatrix},
		{"26_plan_retains_unrelated_committed_owners", runDocumentMetadataPlanRetainsCommittedOwnersMatrix},
		{"27_final_release_retries", TestDocumentMetadataFinalReleaseRetriesAfterPublicationFailure},
		{"28_application_teardown_restores_baseline_once", runDocumentMetadataApplicationTeardownMatrix},
		{"29_repeated_mount_handoff_semantics", runDocumentMetadataRepeatedMountMatrix},
		{"30_observer_failure_after_internal_commit", TestDocumentMetadataObserverFailureDoesNotRollBackCommit},
	}
	if len(cells) != 30 {
		t.Fatalf("matrix cells = %d, want 30", len(cells))
	}
	for _, cell := range cells {
		cell := cell
		t.Run(cell.name, cell.run)
	}
}

func runDocumentMetadataPlanDirectSuccess(t *testing.T) {
	harness := newDocumentMetadataPlanHarness(t)
	ownerA := harness.newOwner("OwnerA", nil, documentMetadataPlanValue("A"))
	ownerB := harness.newOwner("OwnerB", nil, documentMetadataPlanValue("B"))
	harness.commitInitial(ownerA)
	harness.coordinator.beginUpdate()
	renderComponentInstance(ownerB.component)
	deactivateComponent(ownerA.component)
	harness.coordinator.commitUpdate()
	assertDocumentMetadataPlanCommitted(t, harness, ownerB, ownerB)
	if ownerA.cleanups != 1 || ownerB.token().id != 2 ||
		!reflect.DeepEqual(harness.publications, []documentMetadataValue{ownerA.metadata, ownerB.metadata}) {
		t.Fatalf("direct handoff: cleanup=%d B=%#v publications=%#v",
			ownerA.cleanups, ownerB.token(), harness.publications)
	}
}

func runDocumentMetadataFailedHandoffIgnoresUnrelatedMatrix(t *testing.T) {
	TestDocumentMetadataFailedHandoffIgnoresUnrelatedUpdate(t)
}

func runDocumentMetadataPendingAbandonmentMatrix(t *testing.T) {
	TestDocumentMetadataPendingReplacementAbandonmentResolvesHandoff(t)
}

func runDocumentMetadataRepeatedFailureMatrix(t *testing.T) {
	harness, ownerA, ownerB := newDocumentMetadataSinglePendingPlan(t)
	harness.renderAndFail(ownerB)
	assertDocumentMetadataPlanPending(t, harness, ownerA, ownerB)
	assertDocumentMetadataPlanReadiness(t, harness, ownerB)
	harness.coordinator.beginUpdate()
	harness.coordinator.commitUpdate()
	assertDocumentMetadataPlanCommitted(t, harness, ownerB, ownerB)
	if ownerB.renders != 2 || ownerB.token().id != 2 || len(harness.publications) != 2 {
		t.Fatalf("repeated retry: renders=%d B=%#v publications=%#v",
			ownerB.renders, ownerB.token(), harness.publications)
	}
}

func runDocumentMetadataLatestPendingMetadataMatrix(t *testing.T) {
	harness, ownerA, ownerB := newDocumentMetadataSinglePendingPlan(t)
	ownerB.metadata = documentMetadataPlanValue("B2")
	harness.renderAndFail(ownerB)
	assertDocumentMetadataPlanPending(t, harness, ownerA, ownerB)
	assertDocumentMetadataPlanReadiness(t, harness, ownerB)
	harness.coordinator.beginUpdate()
	harness.coordinator.commitUpdate()
	assertDocumentMetadataPlanCommitted(t, harness, ownerB, ownerB)
	if harness.document != documentMetadataPlanValue("B2") || ownerA.token().state != documentMetadataOwnerReleased {
		t.Fatalf("latest metadata: document=%#v A=%#v B=%#v",
			harness.document, ownerA.token(), ownerB.token())
	}
}

func newDocumentMetadataSinglePendingPlan(
	t *testing.T,
) (*documentMetadataPlanHarness, *documentMetadataPlanOwner, *documentMetadataPlanOwner) {
	t.Helper()
	harness := newDocumentMetadataPlanHarness(t)
	ownerA := harness.newOwner("OwnerA", nil, documentMetadataPlanValue("A"))
	ownerB := harness.newOwner("OwnerB", nil, documentMetadataPlanValue("B"))
	harness.commitInitial(ownerA)
	harness.failReplacement(ownerA, ownerB)
	assertDocumentMetadataPlanPending(t, harness, ownerA, ownerB)
	assertDocumentMetadataPlanReadiness(t, harness)
	return harness, ownerA, ownerB
}

func newDocumentMetadataTwoPendingPlan(
	t *testing.T,
) (*documentMetadataPlanHarness, *documentMetadataPlanOwner, *documentMetadataPlanOwner, *documentMetadataPlanOwner) {
	t.Helper()
	harness := newDocumentMetadataPlanHarness(t)
	ownerA := harness.newOwner("OwnerA", nil, documentMetadataPlanValue("A"))
	ownerB := harness.newOwner("OwnerB", nil, documentMetadataPlanValue("B"))
	ownerC := harness.newOwner("OwnerC", nil, documentMetadataPlanValue("C"))
	harness.commitInitial(ownerA)
	harness.failReplacement(ownerA, ownerB, ownerC)
	assertDocumentMetadataPlanPending(t, harness, ownerA, ownerB, ownerC)
	assertDocumentMetadataPlanReadiness(t, harness)
	return harness, ownerA, ownerB, ownerC
}

func runDocumentMetadataTwoOwnerRetryMatrix(t *testing.T, reverse bool) {
	harness, ownerA, ownerB, ownerC := newDocumentMetadataTwoPendingPlan(t)
	if reverse {
		harness.renderAndCommit(ownerC, ownerB)
	} else {
		harness.renderAndCommit(ownerB, ownerC)
	}
	assertDocumentMetadataPlanCommitted(t, harness, ownerC, ownerB, ownerC)
	if ownerA.token().state != documentMetadataOwnerReleased ||
		!reflect.DeepEqual(harness.coordinator.ownerIDs(), []uint64{2, 3}) {
		t.Fatalf("two-owner retry: ids=%v A=%#v", harness.coordinator.ownerIDs(), ownerA.token())
	}
}

func runDocumentMetadataReversePartialProgressMatrix(t *testing.T) {
	harness, ownerA, ownerB, ownerC := newDocumentMetadataTwoPendingPlan(t)
	harness.renderAndCommit(ownerC)
	assertDocumentMetadataPlanPending(t, harness, ownerA, ownerB, ownerC)
	assertDocumentMetadataPlanReadiness(t, harness, ownerC)
	harness.renderAndCommit(ownerB)
	assertDocumentMetadataPlanCommitted(t, harness, ownerC, ownerB, ownerC)
	if ownerB.renders != 2 || ownerC.renders != 2 {
		t.Fatalf("reverse partial renders = %d/%d", ownerB.renders, ownerC.renders)
	}
}

func runDocumentMetadataReadyThenAbandonedMatrix(t *testing.T, keepB bool) {
	harness, _, ownerB, ownerC := newDocumentMetadataTwoPendingPlan(t)
	ready := ownerC
	abandoned := ownerB
	selected := ownerC
	if keepB {
		ready = ownerB
		abandoned = ownerC
		selected = ownerB
	}
	harness.renderAndCommit(ready)
	harness.coordinator.beginUpdate()
	deactivateComponent(abandoned.component)
	harness.coordinator.commitUpdate()
	assertDocumentMetadataPlanCommitted(t, harness, selected, selected)
	if abandoned.token().id != 0 || abandoned.token().state != documentMetadataOwnerReleased ||
		abandoned.cleanups != 1 {
		t.Fatalf("abandoned owner = %#v cleanup=%d", abandoned.token(), abandoned.cleanups)
	}
}

func runDocumentMetadataAbandonThenRetryMatrix(t *testing.T, abandonC bool) {
	harness, _, ownerB, ownerC := newDocumentMetadataTwoPendingPlan(t)
	abandoned := ownerB
	remaining := ownerC
	if abandonC {
		abandoned = ownerC
		remaining = ownerB
	}
	harness.coordinator.beginUpdate()
	deactivateComponent(abandoned.component)
	harness.coordinator.commitUpdate()
	harness.renderAndCommit(remaining)
	assertDocumentMetadataPlanCommitted(t, harness, remaining, remaining)
	if abandoned.token().id != 0 || abandoned.token().state != documentMetadataOwnerReleased {
		t.Fatalf("abandoned owner = %#v", abandoned.token())
	}
}

func runDocumentMetadataAllPendingAbandonedMatrix(t *testing.T) {
	harness, _, ownerB, ownerC := newDocumentMetadataTwoPendingPlan(t)
	harness.coordinator.beginUpdate()
	deactivateComponent(ownerB.component)
	deactivateComponent(ownerC.component)
	harness.coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, harness.coordinator, nil, harness.baseline, 0)
	if harness.document != harness.baseline || ownerB.token().id != 0 || ownerC.token().id != 0 ||
		!reflect.DeepEqual(harness.publications, []documentMetadataValue{
			documentMetadataPlanValue("A"), harness.baseline,
		}) {
		t.Fatalf("all abandoned: document=%#v B=%#v C=%#v publications=%#v",
			harness.document, ownerB.token(), ownerC.token(), harness.publications)
	}
	assertDocumentMetadataCoordinatorFinalized(t, harness.coordinator)
}

func runDocumentMetadataSecondReadyFailureMatrix(t *testing.T) {
	harness, ownerA, ownerB, ownerC := newDocumentMetadataTwoPendingPlan(t)
	harness.renderAndFail(ownerB, ownerC)
	assertDocumentMetadataPlanPending(t, harness, ownerA, ownerB, ownerC)
	assertDocumentMetadataPlanReadiness(t, harness, ownerB, ownerC)
	harness.coordinator.beginUpdate()
	harness.coordinator.commitUpdate()
	assertDocumentMetadataPlanCommitted(t, harness, ownerC, ownerB, ownerC)
	if ownerB.renders != 2 || ownerC.renders != 2 || len(harness.publications) != 2 {
		t.Fatalf("ready retry: renders=%d/%d publications=%#v",
			ownerB.renders, ownerC.renders, harness.publications)
	}
}

func runDocumentMetadataThreeOwnerPermutationsMatrix(t *testing.T) {
	permutations := [][]int{
		{0, 1, 2}, {0, 2, 1}, {1, 0, 2},
		{1, 2, 0}, {2, 0, 1}, {2, 1, 0},
	}
	for _, permutation := range permutations {
		permutation := append([]int(nil), permutation...)
		t.Run(fmt.Sprint(permutation), func(t *testing.T) {
			harness := newDocumentMetadataPlanHarness(t)
			ownerA := harness.newOwner("OwnerA", nil, documentMetadataPlanValue("A"))
			owners := []*documentMetadataPlanOwner{
				harness.newOwner("OwnerB", nil, documentMetadataPlanValue("B")),
				harness.newOwner("OwnerC", nil, documentMetadataPlanValue("C")),
				harness.newOwner("OwnerD", nil, documentMetadataPlanValue("D")),
			}
			harness.commitInitial(ownerA)
			harness.failReplacement(ownerA, owners...)
			retry := make([]*documentMetadataPlanOwner, 0, len(owners))
			for _, index := range permutation {
				retry = append(retry, owners[index])
			}
			harness.renderAndCommit(retry...)
			assertDocumentMetadataPlanCommitted(t, harness, owners[2], owners...)
			if !reflect.DeepEqual(harness.coordinator.ownerIDs(), []uint64{2, 3, 4}) {
				t.Fatalf("permutation %v ids = %v", permutation, harness.coordinator.ownerIDs())
			}
		})
	}
}

func runDocumentMetadataAbandonmentFinalizationSuccessorMatrix(t *testing.T) {
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
		t.Fatalf("abandonment finalization error = %v", err)
	}
	if boundaryUnmounts != 1 || len(fixture.coordinator.pendingFinalizations) != 1 ||
		fixture.coordinator.pendingFinalizations[fixture.boundary] == nil ||
		fixture.coordinator.pendingFinalizations[fixture.boundary].kind != documentMetadataAbandonBoundary {
		t.Fatalf("abandonment state: unmounts=%d finalizations=%#v",
			boundaryUnmounts, fixture.coordinator.pendingFinalizations)
	}

	metadataB := documentMetadataPlanValue("B")
	rendersB := 0
	ownerBComponent := testComponentInstance("OwnerB", func() Node {
		rendersB++
		useDocumentMetadata(metadataB)
		return Empty()
	}, nil)
	fixture.coordinator.beginUpdate()
	renderComponentInstance(ownerBComponent)
	err = recoverDocumentMetadataError(t, fixture.coordinator.commitUpdate)
	if !errors.Is(err, fixture.publicationFailure) {
		t.Fatalf("external successor error = %v", err)
	}
	ownerB := documentMetadataOwnerAtStateSlot(ownerBComponent, 0)
	handoff := fixture.coordinator.pendingHandoffs[ownerB]
	if ownerB == nil || ownerB.id != 0 || handoff == nil ||
		len(handoff.finalizations) != 1 ||
		handoff.finalizations[0].kind != documentMetadataAbandonBoundary ||
		len(fixture.coordinator.pendingFinalizations) != 0 {
		t.Fatalf("external abandonment handoff: B=%#v handoff=%#v standalone=%#v",
			ownerB, handoff, fixture.coordinator.pendingFinalizations)
	}

	fixture.failPublication = false
	fixture.coordinator.beginUpdate()
	fixture.coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, fixture.coordinator, fixture.ownerA, fixture.metadataA, 1)
	if fixture.document != fixture.metadataA || ownerB.id != 0 || rendersB != 1 {
		t.Fatalf("unrelated update: document=%#v B=%#v renders=%d",
			fixture.document, ownerB, rendersB)
	}
	fixture.coordinator.beginUpdate()
	renderComponentInstance(ownerBComponent)
	fixture.coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, fixture.coordinator, ownerB, metadataB, 1)
	if ownerB.id != 2 || fixture.ownerA.state != documentMetadataOwnerReleased ||
		boundaryUnmounts != 1 || fixture.cleanupA != 1 ||
		!reflect.DeepEqual(fixture.publications, []documentMetadataValue{fixture.metadataA, metadataB}) {
		t.Fatalf("abandonment successor: A=%#v B=%#v unmounts=%d cleanup=%d publications=%#v",
			fixture.ownerA, ownerB, boundaryUnmounts, fixture.cleanupA, fixture.publications)
	}
	assertDocumentMetadataCoordinatorFinalized(t, fixture.coordinator)
}

func runDocumentMetadataOuterFailureSupersedesFinalizationMatrix(t *testing.T) {
	fixture := newFailedDocumentMetadataBoundaryFixture(t)
	fixture.failPublication = true
	clearErrorBoundary(fixture.boundary)
	fixture.coordinator.beginUpdate()
	previousOuter := beginProtectedSubtreeLifecycle(fixture.boundary)
	finishProtectedSubtreeLifecycle(fixture.boundary, previousOuter)
	err := recoverDocumentMetadataError(t, fixture.coordinator.commitUpdate)
	if !errors.Is(err, fixture.publicationFailure) {
		t.Fatalf("ownerless finalization error = %v", err)
	}

	innerOwner := testComponentInstanceWithParent(
		"InnerBoundary",
		fixture.boundaryOwner,
		func() Node { return Empty() },
	)
	inner := ensureErrorBoundaryState(innerOwner)
	metadataB := documentMetadataPlanValue("B")
	cleanupB := 0
	ownerBComponent := testComponentInstanceWithParent("OwnerB", innerOwner, func() Node {
		useDocumentMetadata(metadataB)
		UseUnmount(func() { cleanupB++ })
		return Empty()
	})
	fixture.coordinator.beginUpdate()
	previousOuter = beginProtectedSubtreeLifecycle(fixture.boundary)
	previousInner := beginProtectedSubtreeLifecycle(inner)
	renderComponentInstance(ownerBComponent)
	finishProtectedSubtreeLifecycle(inner, previousInner)
	finishProtectedSubtreeLifecycle(fixture.boundary, previousOuter)
	err = recoverDocumentMetadataError(t, fixture.coordinator.commitUpdate)
	if !errors.Is(err, fixture.publicationFailure) {
		t.Fatalf("nested successor error = %v", err)
	}
	ownerB := documentMetadataOwnerAtStateSlot(ownerBComponent, 0)
	handoff := fixture.coordinator.pendingHandoffs[ownerB]
	if handoff == nil || len(handoff.finalizations) != 1 ||
		handoff.finalizations[0].boundary != fixture.boundary {
		t.Fatalf("nested successor finalization = %#v", handoff)
	}

	fixture.failPublication = false
	failing := testComponentInstanceWithParent(
		"OuterFailingSibling",
		fixture.boundaryOwner,
		func() Node { panic("new outer failure") },
	)
	fixture.coordinator.beginUpdate()
	previousOuter = beginProtectedSubtreeLifecycle(fixture.boundary)
	renderComponentInstance(failing)
	finishProtectedSubtreeLifecycle(fixture.boundary, previousOuter)
	deactivateComponent(ownerBComponent)
	fixture.coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, fixture.coordinator, fixture.ownerA, fixture.metadataA, 1)
	if fixture.document != fixture.metadataA || ownerB.id != 0 ||
		ownerB.state != documentMetadataOwnerReleased || cleanupB != 1 ||
		len(fixture.coordinator.pendingHandoffs) != 0 ||
		len(fixture.coordinator.pendingFinalizations) != 0 ||
		!fixture.coordinator.failedBoundaries[fixture.boundary] {
		t.Fatalf("new outer failure: document=%#v B=%#v cleanup=%d failed=%#v handoffs=%#v finalizations=%#v",
			fixture.document, ownerB, cleanupB, fixture.coordinator.failedBoundaries,
			fixture.coordinator.pendingHandoffs, fixture.coordinator.pendingFinalizations)
	}

	clearErrorBoundary(fixture.boundary)
	fixture.coordinator.beginUpdate()
	previousOuter = beginProtectedSubtreeLifecycle(fixture.boundary)
	finishProtectedSubtreeLifecycle(fixture.boundary, previousOuter)
	fixture.coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, fixture.coordinator, nil, fixture.baseline, 0)
	assertDocumentMetadataCoordinatorFinalized(t, fixture.coordinator)
}

func runDocumentMetadataFinalizationMultipleOwnersMatrix(t *testing.T) {
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

	metadataB := documentMetadataPlanValue("B")
	metadataC := documentMetadataPlanValue("C")
	rendersB := 0
	rendersC := 0
	ownerBComponent := testComponentInstance("OwnerB", func() Node {
		rendersB++
		useDocumentMetadata(metadataB)
		return Empty()
	}, nil)
	ownerCComponent := testComponentInstance("OwnerC", func() Node {
		rendersC++
		useDocumentMetadata(metadataC)
		return Empty()
	}, nil)
	fixture.coordinator.beginUpdate()
	renderComponentInstance(ownerBComponent)
	renderComponentInstance(ownerCComponent)
	err = recoverDocumentMetadataError(t, fixture.coordinator.commitUpdate)
	if !errors.Is(err, fixture.publicationFailure) {
		t.Fatalf("multiple successor error = %v", err)
	}
	ownerB := documentMetadataOwnerAtStateSlot(ownerBComponent, 0)
	ownerC := documentMetadataOwnerAtStateSlot(ownerCComponent, 0)
	handoff := fixture.coordinator.pendingHandoffs[ownerB]
	if handoff == nil || fixture.coordinator.pendingHandoffs[ownerC] != handoff ||
		len(handoff.owners) != 2 || len(handoff.finalizations) != 1 {
		t.Fatalf("multiple finalization handoff: B=%#v C=%#v handoff=%#v",
			ownerB, ownerC, handoff)
	}

	fixture.failPublication = false
	fixture.coordinator.beginUpdate()
	renderComponentInstance(ownerCComponent)
	renderComponentInstance(ownerBComponent)
	fixture.coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, fixture.coordinator, ownerC, metadataC, 2)
	if !reflect.DeepEqual(fixture.coordinator.ownerIDs(), []uint64{2, 3}) ||
		fixture.coordinator.owners[0].owner != ownerB ||
		fixture.coordinator.owners[1].owner != ownerC ||
		rendersB != 2 || rendersC != 2 ||
		!reflect.DeepEqual(fixture.publications, []documentMetadataValue{fixture.metadataA, metadataC}) {
		t.Fatalf("multiple finalization retry: owners=%#v renders=%d/%d publications=%#v",
			fixture.coordinator.owners, rendersB, rendersC, fixture.publications)
	}
	assertDocumentMetadataCoordinatorFinalized(t, fixture.coordinator)
}

func runDocumentMetadataSelectedReleaseRevealsParentMatrix(t *testing.T) {
	harness := newDocumentMetadataPlanHarness(t)
	parent := harness.newOwner("Parent", nil, documentMetadataPlanValue("Parent"))
	child := harness.newOwner("Child", nil, documentMetadataPlanValue("Child"))
	harness.coordinator.beginUpdate()
	renderComponentInstance(parent.component)
	renderComponentInstance(child.component)
	harness.coordinator.commitUpdate()
	assertDocumentMetadataPlanCommitted(t, harness, child, parent, child)
	harness.coordinator.beginUpdate()
	deactivateComponent(child.component)
	harness.coordinator.commitUpdate()
	assertDocumentMetadataPlanCommitted(t, harness, parent, parent)
	if child.cleanups != 1 || child.token().state != documentMetadataOwnerReleased ||
		!reflect.DeepEqual(harness.publications, []documentMetadataValue{child.metadata, parent.metadata}) {
		t.Fatalf("parent reveal: child=%#v cleanup=%d publications=%#v",
			child.token(), child.cleanups, harness.publications)
	}
}

func runDocumentMetadataPlanRetainsCommittedOwnersMatrix(t *testing.T) {
	harness := newDocumentMetadataPlanHarness(t)
	parent := harness.newOwner("Parent", nil, documentMetadataPlanValue("Parent"))
	ownerA := harness.newOwner("OwnerA", nil, documentMetadataPlanValue("A"))
	ownerB := harness.newOwner("OwnerB", nil, documentMetadataPlanValue("B"))
	ownerC := harness.newOwner("OwnerC", nil, documentMetadataPlanValue("C"))
	harness.coordinator.beginUpdate()
	renderComponentInstance(parent.component)
	renderComponentInstance(ownerA.component)
	harness.coordinator.commitUpdate()
	parent.token()
	ownerA.token()
	harness.failReplacement(ownerA, ownerB, ownerC)
	harness.renderAndCommit(ownerC, ownerB)
	assertDocumentMetadataPlanCommitted(t, harness, ownerC, parent, ownerB, ownerC)
	if !reflect.DeepEqual(harness.coordinator.ownerIDs(), []uint64{1, 3, 4}) ||
		parent.token().state != documentMetadataOwnerActive || ownerA.token().state != documentMetadataOwnerReleased {
		t.Fatalf("retained topology: ids=%v parent=%#v A=%#v",
			harness.coordinator.ownerIDs(), parent.token(), ownerA.token())
	}
}

func runDocumentMetadataApplicationTeardownMatrix(t *testing.T) {
	harness := newDocumentMetadataPlanHarness(t)
	ownerA := harness.newOwner("OwnerA", nil, documentMetadataPlanValue("A"))
	harness.commitInitial(ownerA)
	harness.coordinator.beginUpdate()
	deactivateComponent(ownerA.component)
	harness.coordinator.commitUpdate()
	assertDocumentMetadataSnapshot(t, harness.coordinator, nil, harness.baseline, 0)
	statistics := harness.coordinator.statistics
	harness.coordinator.beginUpdate()
	harness.coordinator.commitUpdate()
	if ownerA.cleanups != 1 || harness.coordinator.statistics.baselineRestorations != 1 ||
		harness.coordinator.statistics.documentPublications != statistics.documentPublications ||
		!reflect.DeepEqual(harness.publications, []documentMetadataValue{ownerA.metadata, harness.baseline}) {
		t.Fatalf("application teardown: cleanup=%d statistics=%#v publications=%#v",
			ownerA.cleanups, harness.coordinator.statistics, harness.publications)
	}
	assertDocumentMetadataCoordinatorFinalized(t, harness.coordinator)
}

func runDocumentMetadataRepeatedMountMatrix(t *testing.T) {
	harness := newDocumentMetadataPlanHarness(t)
	ownerA := harness.newOwner("MountA", nil, documentMetadataPlanValue("A"))
	ownerB := harness.newOwner("MountB", nil, documentMetadataPlanValue("B"))
	ownerC := harness.newOwner("MountC", nil, documentMetadataPlanValue("C"))
	harness.commitInitial(ownerA)
	harness.coordinator.beginUpdate()
	renderComponentInstance(ownerB.component)
	deactivateComponent(ownerA.component)
	harness.coordinator.commitUpdate()
	harness.coordinator.beginUpdate()
	renderComponentInstance(ownerC.component)
	deactivateComponent(ownerB.component)
	harness.coordinator.commitUpdate()
	assertDocumentMetadataPlanCommitted(t, harness, ownerC, ownerC)
	if ownerA.cleanups != 1 || ownerB.cleanups != 1 || ownerC.token().id != 3 ||
		!reflect.DeepEqual(harness.publications, []documentMetadataValue{
			ownerA.metadata, ownerB.metadata, ownerC.metadata,
		}) {
		t.Fatalf("repeated mount: cleanups=%d/%d ids=%v publications=%#v",
			ownerA.cleanups, ownerB.cleanups, harness.coordinator.ownerIDs(), harness.publications)
	}
}

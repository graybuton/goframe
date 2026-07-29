package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/graybuton/goframe/scripts/fixtures/document-state-api-design/internal/documentmeta"
)

func TestDocumentMetadataHandleRejectsPrimaryUpdateWhileDuplicated(t *testing.T) {
	baseline := documentmeta.Metadata{
		Title:       "authored title",
		Description: "authored description",
	}
	metadataA := documentmeta.Metadata{
		Title:       "title A",
		Description: "description A",
	}
	metadataB := documentmeta.Metadata{
		Title:       "title B",
		Description: "description B",
	}
	metadataC := documentmeta.Metadata{
		Title:       "title C",
		Description: "description C",
	}
	coordinator := documentmeta.New(baseline)
	handle := &documentMetadataOwnerHandle{owner: coordinator.NewOwner()}
	primary := &documentMetadataPublication{}
	duplicate := &documentMetadataPublication{}

	added, coalesced, err := reconcileDocumentMetadataPublication(
		coordinator,
		handle,
		primary,
		metadataA,
	)
	if err != nil {
		t.Fatal(err)
	}
	if added.Change != documentmeta.ChangeAdded || coalesced {
		t.Fatalf(
			"primary publication = (%s, coalesced %t), want (added, false)",
			added.Change.String(),
			coalesced,
		)
	}
	ownerID := handle.owner.ID()
	ownerOrder := coordinator.OwnerIDs()

	transition, coalesced, err := reconcileDocumentMetadataPublication(
		coordinator,
		handle,
		duplicate,
		metadataA,
	)
	if err != nil {
		t.Fatal(err)
	}
	if transition.Change != documentmeta.ChangeNone || !coalesced {
		t.Fatalf(
			"duplicate publication = (%s, coalesced %t), want (none, true)",
			transition.Change.String(),
			coalesced,
		)
	}
	if duplicate.metadata != primary.metadata ||
		handle.owner.ID() != added.OwnerID {
		t.Fatalf(
			"forwarded identity = (metadata %#v, owner %d), want (%#v, %d)",
			duplicate.metadata,
			handle.owner.ID(),
			primary.metadata,
			added.OwnerID,
		)
	}
	beforeDuplicateNoOpStats := coordinator.Stats()
	transition, coalesced, err = reconcileDocumentMetadataPublication(
		coordinator,
		handle,
		duplicate,
		metadataA,
	)
	if err != nil {
		t.Fatal(err)
	}
	if transition.Change != documentmeta.ChangeNone || coalesced {
		t.Fatalf(
			"active duplicate no-op = (%s, coalesced %t), want (none, false)",
			transition.Change.String(),
			coalesced,
		)
	}
	if got := coordinator.Stats(); got != beforeDuplicateNoOpStats {
		t.Fatalf(
			"active duplicate no-op statistics = %#v, want %#v",
			got,
			beforeDuplicateNoOpStats,
		)
	}

	beforeHandle := *handle
	beforePrimary := *primary
	beforeDuplicate := *duplicate
	beforeSnapshot := coordinator.Snapshot()
	beforeStats := coordinator.Stats()
	transition, coalesced, err = reconcileDocumentMetadataPublication(
		coordinator,
		handle,
		primary,
		metadataB,
	)
	if !errors.Is(err, errDocumentMetadataPublicationConflict) {
		t.Fatalf("primary conflict error = %v", err)
	}
	if got, want := err.Error(),
		"document-state API design: one owner handle has conflicting active publications"; got != want {
		t.Fatalf("primary conflict diagnostic = %q, want %q", got, want)
	}
	if transition.Change != documentmeta.ChangeNone || coalesced {
		t.Fatalf(
			"conflicting publication = (%s, coalesced %t), want (none, false)",
			transition.Change.String(),
			coalesced,
		)
	}
	assertDocumentMetadataHandleState(
		t,
		coordinator,
		handle,
		primary,
		duplicate,
		beforeHandle,
		beforePrimary,
		beforeDuplicate,
		beforeSnapshot,
		beforeStats,
	)

	released, err := releaseDocumentMetadataPublication(
		coordinator,
		handle,
		duplicate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if released.Change != documentmeta.ChangeNone {
		t.Fatalf(
			"duplicate release change = %s, want none",
			released.Change.String(),
		)
	}
	if handle.activePublications != 1 || duplicate.active {
		t.Fatalf(
			"duplicate release state = (count %d, active %t), want (1, false)",
			handle.activePublications,
			duplicate.active,
		)
	}

	updated, coalesced, err := reconcileDocumentMetadataPublication(
		coordinator,
		handle,
		primary,
		metadataC,
	)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Change != documentmeta.ChangeUpdated || coalesced {
		t.Fatalf(
			"sole-primary update = (%s, coalesced %t), want (updated, false)",
			updated.Change.String(),
			coalesced,
		)
	}
	if handle.owner.ID() != ownerID || updated.OwnerID != ownerID {
		t.Fatalf(
			"sole-primary owner IDs = handle %d, transition %d; want %d",
			handle.owner.ID(),
			updated.OwnerID,
			ownerID,
		)
	}
	if got := coordinator.OwnerIDs(); !reflect.DeepEqual(got, ownerOrder) {
		t.Fatalf("owner order after update = %v, want %v", got, ownerOrder)
	}
	if got := coordinator.Snapshot().Metadata; got != metadataC {
		t.Fatalf("updated coordinator metadata = %#v, want %#v", got, metadataC)
	}
	if got := coordinator.Stats().Updates; got != beforeStats.Updates+1 {
		t.Fatalf("coordinator updates = %d, want %d", got, beforeStats.Updates+1)
	}

	final, err := releaseDocumentMetadataPublication(
		coordinator,
		handle,
		primary,
	)
	if err != nil {
		t.Fatal(err)
	}
	if final.Change != documentmeta.ChangeRemoved {
		t.Fatalf("final release change = %s, want removed", final.Change.String())
	}
	if final.Snapshot.Metadata != baseline {
		t.Fatalf(
			"final metadata = %#v, want baseline %#v",
			final.Snapshot.Metadata,
			baseline,
		)
	}
	if handle.activePublications != 0 || handle.primary != nil {
		t.Fatalf(
			"final handle state = (count %d, primary %p), want (0, nil)",
			handle.activePublications,
			handle.primary,
		)
	}
	repeated, err := releaseDocumentMetadataPublication(
		coordinator,
		handle,
		primary,
	)
	if err != nil {
		t.Fatal(err)
	}
	if repeated.Change != documentmeta.ChangeNone {
		t.Fatalf(
			"repeated release change = %s, want none",
			repeated.Change.String(),
		)
	}
	if got := coordinator.Stats().Releases; got != 1 {
		t.Fatalf("coordinator releases = %d, want 1", got)
	}
}

func TestDocumentMetadataHandleRejectsMismatchedInitialDuplicate(t *testing.T) {
	metadataA := documentmeta.Metadata{Title: "A", Description: "pair A"}
	metadataB := documentmeta.Metadata{Title: "B", Description: "pair B"}
	coordinator := documentmeta.New(documentmeta.Metadata{})
	handle := &documentMetadataOwnerHandle{owner: coordinator.NewOwner()}
	primary := &documentMetadataPublication{}
	duplicate := &documentMetadataPublication{}
	if _, _, err := reconcileDocumentMetadataPublication(
		coordinator,
		handle,
		primary,
		metadataA,
	); err != nil {
		t.Fatal(err)
	}

	beforeHandle := *handle
	beforePrimary := *primary
	beforeDuplicate := *duplicate
	beforeSnapshot := coordinator.Snapshot()
	beforeStats := coordinator.Stats()
	transition, coalesced, err := reconcileDocumentMetadataPublication(
		coordinator,
		handle,
		duplicate,
		metadataB,
	)
	if !errors.Is(err, errDocumentMetadataPublicationConflict) {
		t.Fatalf("mismatched duplicate error = %v", err)
	}
	if transition.Change != documentmeta.ChangeNone || coalesced {
		t.Fatalf(
			"mismatched duplicate = (%s, coalesced %t), want (none, false)",
			transition.Change.String(),
			coalesced,
		)
	}
	assertDocumentMetadataHandleState(
		t,
		coordinator,
		handle,
		primary,
		duplicate,
		beforeHandle,
		beforePrimary,
		beforeDuplicate,
		beforeSnapshot,
		beforeStats,
	)
}

func TestDocumentMetadataHandlePublicationUnderflowIsAtomic(t *testing.T) {
	metadata := documentmeta.Metadata{Title: "A", Description: "pair A"}
	coordinator := documentmeta.New(documentmeta.Metadata{})
	handle := &documentMetadataOwnerHandle{owner: coordinator.NewOwner()}
	publication := &documentMetadataPublication{}
	if _, _, err := reconcileDocumentMetadataPublication(
		coordinator,
		handle,
		publication,
		metadata,
	); err != nil {
		t.Fatal(err)
	}
	handle.activePublications = 0

	beforeHandle := *handle
	beforePublication := *publication
	beforeSnapshot := coordinator.Snapshot()
	beforeStats := coordinator.Stats()
	transition, err := releaseDocumentMetadataPublication(
		coordinator,
		handle,
		publication,
	)
	if !errors.Is(err, errDocumentMetadataPublicationUnderflow) {
		t.Fatalf("underflow error = %v", err)
	}
	if transition.Change != documentmeta.ChangeNone {
		t.Fatalf("underflow change = %s, want none", transition.Change.String())
	}
	if *handle != beforeHandle {
		t.Fatalf("handle changed after underflow: got %#v, want %#v", *handle, beforeHandle)
	}
	if *publication != beforePublication {
		t.Fatalf(
			"publication changed after underflow: got %#v, want %#v",
			*publication,
			beforePublication,
		)
	}
	if got := coordinator.Snapshot(); got != beforeSnapshot {
		t.Fatalf("snapshot after underflow = %#v, want %#v", got, beforeSnapshot)
	}
	if got := coordinator.Stats(); got != beforeStats {
		t.Fatalf("statistics after underflow = %#v, want %#v", got, beforeStats)
	}
}

func assertDocumentMetadataHandleState(
	t *testing.T,
	coordinator *documentmeta.Coordinator,
	handle *documentMetadataOwnerHandle,
	primary *documentMetadataPublication,
	duplicate *documentMetadataPublication,
	wantHandle documentMetadataOwnerHandle,
	wantPrimary documentMetadataPublication,
	wantDuplicate documentMetadataPublication,
	wantSnapshot documentmeta.Snapshot,
	wantStats documentmeta.Statistics,
) {
	t.Helper()
	if *handle != wantHandle {
		t.Fatalf("handle = %#v, want %#v", *handle, wantHandle)
	}
	if *primary != wantPrimary {
		t.Fatalf("primary publication = %#v, want %#v", *primary, wantPrimary)
	}
	if *duplicate != wantDuplicate {
		t.Fatalf("duplicate publication = %#v, want %#v", *duplicate, wantDuplicate)
	}
	if got := coordinator.Snapshot(); got != wantSnapshot {
		t.Fatalf("coordinator snapshot = %#v, want %#v", got, wantSnapshot)
	}
	if got := coordinator.Stats(); got != wantStats {
		t.Fatalf("coordinator statistics = %#v, want %#v", got, wantStats)
	}
}

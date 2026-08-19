//go:build js && wasm && goframe_document_state_experiment

package goframe

// DocumentMetadataHandoffExperimentValue is fixture-only metadata used by the
// transactional document-state browser experiment. It is absent from normal
// builds and is not a public API candidate.
type DocumentMetadataHandoffExperimentValue struct {
	Title       string
	Description string
}

// DocumentMetadataHandoffExperimentEvent is one fixture-only lifecycle event.
type DocumentMetadataHandoffExperimentEvent struct {
	Kind        string
	BatchID     uint64
	OwnerID     uint64
	OwnerCount  int
	Title       string
	Description string
}

// DocumentMetadataHandoffExperimentSnapshot is the fixture-only committed
// ownership snapshot.
type DocumentMetadataHandoffExperimentSnapshot struct {
	ActiveOwnerID        uint64
	OwnerIDs             []uint64
	OwnerTitles          []string
	OwnerCount           int
	FailedBoundaryCount  int
	RetainedReleaseCount int
	PendingPlanCount     int
	PendingOwnerCount    int
	PendingOwnerIDs      []uint64
	PendingOwnerTitles   []string
	PendingFinalizations int
	BatchActive          bool
	Title                string
	Description          string
}

// DocumentMetadataHandoffExperimentStatistics contains fixture-only counters.
type DocumentMetadataHandoffExperimentStatistics struct {
	TokenCreations         int
	CommittedIDAssignments int
	ActiveAdditions        int
	Updates                int
	Releases               int
	DuplicatePublications  int
	Rollbacks              int
	UpdateBatches          int
	DocumentPublications   int
	BaselineRestorations   int
}

// InstallDocumentMetadataHandoffExperiment installs the private coordinator for
// the dedicated browser fixture. The build tag keeps this bridge out of normal
// applications and package documentation.
func InstallDocumentMetadataHandoffExperiment(
	baseline DocumentMetadataHandoffExperimentValue,
	apply func(DocumentMetadataHandoffExperimentValue),
	observe func(DocumentMetadataHandoffExperimentEvent),
) {
	if apply == nil {
		panicRuntimeInvariant("goframe: document metadata experiment requires an apply callback")
	}
	coordinator := newDocumentMetadataCoordinator(
		documentMetadataExperimentValue(baseline),
		func(previous, next documentMetadataValue) error {
			publicationError := applyDocumentMetadataExperimentValue(apply, next)
			if publicationError == nil {
				return nil
			}
			restoreError := applyDocumentMetadataExperimentValue(apply, previous)
			if restoreError != nil {
				restoreError = wrapDocumentMetadataError(
					"restore document metadata experiment value",
					restoreError,
				)
			}
			return joinDocumentMetadataErrors(publicationError, restoreError)
		},
		func(event documentMetadataEvent) (err error) {
			if observe == nil {
				return nil
			}
			defer func() {
				if recovered := recover(); recovered != nil {
					err = recoveredDocumentMetadataError(
						"observe document metadata experiment event",
						recovered,
					)
				}
			}()
			observe(DocumentMetadataHandoffExperimentEvent{
				Kind:        event.kind,
				BatchID:     event.batchID,
				OwnerID:     event.ownerID,
				OwnerCount:  event.ownerCount,
				Title:       event.metadata.title,
				Description: event.metadata.description,
			})
			return nil
		},
	)
	installDocumentMetadataCoordinator(coordinator)
}

func applyDocumentMetadataExperimentValue(
	apply func(DocumentMetadataHandoffExperimentValue),
	value documentMetadataValue,
) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = recoveredDocumentMetadataError(
				"apply document metadata experiment value",
				recovered,
			)
		}
	}()
	apply(DocumentMetadataHandoffExperimentValue{
		Title:       value.title,
		Description: value.description,
	})
	return nil
}

// UseDocumentMetadataHandoffExperiment participates in the private document
// transaction from the dedicated browser fixture.
func UseDocumentMetadataHandoffExperiment(
	metadata DocumentMetadataHandoffExperimentValue,
) {
	useDocumentMetadata(documentMetadataExperimentValue(metadata))
}

// CurrentDocumentMetadataHandoffExperiment returns the committed fixture-only
// ownership snapshot.
func CurrentDocumentMetadataHandoffExperiment() DocumentMetadataHandoffExperimentSnapshot {
	coordinator := activeDocumentMetadataCoordinator
	if coordinator == nil {
		return DocumentMetadataHandoffExperimentSnapshot{}
	}
	snapshot := coordinator.snapshot()
	var ownerID uint64
	if snapshot.owner != nil {
		ownerID = snapshot.owner.id
	}
	ownerIDs := coordinator.ownerIDs()
	ownerTitles := make([]string, len(coordinator.owners))
	for index, record := range coordinator.owners {
		ownerTitles[index] = record.metadata.title
	}
	pendingOwnerCount := 0
	pendingFinalizations := len(coordinator.pendingFinalizationOrder)
	pendingOwnerIDs := make([]uint64, 0)
	pendingOwnerTitles := make([]string, 0)
	for _, handoff := range coordinator.pendingHandoffOrder {
		pendingFinalizations += len(handoff.finalizations)
		for _, pending := range handoff.owners {
			if pending.abandoned {
				continue
			}
			pendingOwnerCount++
			pendingOwnerIDs = append(pendingOwnerIDs, pending.owner.id)
			pendingOwnerTitles = append(
				pendingOwnerTitles,
				pending.metadata.title,
			)
		}
	}
	return DocumentMetadataHandoffExperimentSnapshot{
		ActiveOwnerID:        ownerID,
		OwnerIDs:             ownerIDs,
		OwnerTitles:          ownerTitles,
		OwnerCount:           snapshot.ownerCount,
		FailedBoundaryCount:  snapshot.failedBoundaryCount,
		RetainedReleaseCount: snapshot.retainedReleaseCount,
		PendingPlanCount:     len(coordinator.pendingHandoffOrder),
		PendingOwnerCount:    pendingOwnerCount,
		PendingOwnerIDs:      pendingOwnerIDs,
		PendingOwnerTitles:   pendingOwnerTitles,
		PendingFinalizations: pendingFinalizations,
		BatchActive:          snapshot.batchActive,
		Title:                snapshot.metadata.title,
		Description:          snapshot.metadata.description,
	}
}

// DocumentMetadataHandoffExperimentStats returns fixture-only lifecycle
// counters for browser assertions.
func DocumentMetadataHandoffExperimentStats() DocumentMetadataHandoffExperimentStatistics {
	coordinator := activeDocumentMetadataCoordinator
	if coordinator == nil {
		return DocumentMetadataHandoffExperimentStatistics{}
	}
	statistics := coordinator.statistics
	return DocumentMetadataHandoffExperimentStatistics{
		TokenCreations:         statistics.tokenCreations,
		CommittedIDAssignments: statistics.committedIDAssignments,
		ActiveAdditions:        statistics.activeAdditions,
		Updates:                statistics.updates,
		Releases:               statistics.releases,
		DuplicatePublications:  statistics.duplicatePublications,
		Rollbacks:              statistics.rollbacks,
		UpdateBatches:          statistics.updateBatches,
		DocumentPublications:   statistics.documentPublications,
		BaselineRestorations:   statistics.baselineRestorations,
	}
}

func documentMetadataExperimentValue(
	value DocumentMetadataHandoffExperimentValue,
) documentMetadataValue {
	return documentMetadataValue{
		title:       value.Title,
		description: value.Description,
	}
}

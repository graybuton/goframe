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
	OwnerCount           int
	FailedBoundaryCount  int
	RetainedReleaseCount int
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
		panic("goframe: document metadata experiment requires an apply callback")
	}
	coordinator := newDocumentMetadataCoordinator(
		documentMetadataExperimentValue(baseline),
		func(value documentMetadataValue) {
			apply(DocumentMetadataHandoffExperimentValue{
				Title:       value.title,
				Description: value.description,
			})
		},
		func(event documentMetadataEvent) {
			if observe == nil {
				return
			}
			observe(DocumentMetadataHandoffExperimentEvent{
				Kind:        event.kind,
				BatchID:     event.batchID,
				OwnerID:     event.ownerID,
				OwnerCount:  event.ownerCount,
				Title:       event.metadata.title,
				Description: event.metadata.description,
			})
		},
	)
	installDocumentMetadataCoordinator(coordinator)
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
	return DocumentMetadataHandoffExperimentSnapshot{
		ActiveOwnerID:        ownerID,
		OwnerCount:           snapshot.ownerCount,
		FailedBoundaryCount:  snapshot.failedBoundaryCount,
		RetainedReleaseCount: snapshot.retainedReleaseCount,
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

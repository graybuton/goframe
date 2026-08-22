//go:build !js || !wasm || goframe_document_state_experiment

package goframe

type documentMetadataValue struct {
	title       string
	description string
}

type documentMetadataPublisher func(
	previous documentMetadataValue,
	next documentMetadataValue,
) error

type documentMetadataObserver func(documentMetadataEvent) error

type documentMetadataWrappedError struct {
	message string
	cause   error
}

func (err *documentMetadataWrappedError) Error() string {
	if err == nil {
		return ""
	}
	if err.cause == nil {
		return err.message
	}
	return err.message + ": " + err.cause.Error()
}

func (err *documentMetadataWrappedError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.cause
}

type documentMetadataJoinedError struct {
	errors []error
}

func (err *documentMetadataJoinedError) Error() string {
	if err == nil {
		return ""
	}
	message := ""
	for _, current := range err.errors {
		if current == nil {
			continue
		}
		if message != "" {
			message += "\n"
		}
		message += current.Error()
	}
	return message
}

func (err *documentMetadataJoinedError) Unwrap() []error {
	if err == nil {
		return nil
	}
	return err.errors
}

func wrapDocumentMetadataError(message string, cause error) error {
	return &documentMetadataWrappedError{message: message, cause: cause}
}

func joinDocumentMetadataErrors(values ...error) error {
	errors := make([]error, 0, len(values))
	for _, err := range values {
		if err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) == 0 {
		return nil
	}
	if len(errors) == 1 {
		return errors[0]
	}
	return &documentMetadataJoinedError{errors: errors}
}

func recoveredDocumentMetadataError(prefix string, recovered any) error {
	if err, ok := recovered.(error); ok {
		return wrapDocumentMetadataError(prefix, err)
	}
	if message, ok := recovered.(string); ok {
		return &documentMetadataWrappedError{message: prefix + ": " + message}
	}
	return &documentMetadataWrappedError{message: prefix}
}

func writeDocumentMetadataPair(
	previous documentMetadataValue,
	next documentMetadataValue,
	writeTitle func(string) error,
	writeDescription func(string) error,
) error {
	if err := writeTitle(next.title); err != nil {
		return wrapDocumentMetadataError("write document title", err)
	}
	if err := writeDescription(next.description); err != nil {
		publicationError := wrapDocumentMetadataError("write document description", err)
		restoreTitleError := writeTitle(previous.title)
		if restoreTitleError != nil {
			restoreTitleError = wrapDocumentMetadataError("restore document title", restoreTitleError)
		}
		restoreDescriptionError := writeDescription(previous.description)
		if restoreDescriptionError != nil {
			restoreDescriptionError = wrapDocumentMetadataError(
				"restore document description",
				restoreDescriptionError,
			)
		}
		return joinDocumentMetadataErrors(
			publicationError,
			restoreTitleError,
			restoreDescriptionError,
		)
	}
	return nil
}

type documentMetadataOwnerState uint8

const (
	documentMetadataOwnerPending documentMetadataOwnerState = iota
	documentMetadataOwnerActive
	documentMetadataOwnerReleased
)

type documentMetadataOwner struct {
	coordinator *documentMetadataCoordinator
	id          uint64
	state       documentMetadataOwnerState
	boundary    *errorBoundaryState
	pending     documentMetadataRenderState
}

type documentMetadataRenderState struct {
	attempt  *renderLifecycleAttempt
	metadata documentMetadataValue
	boundary *errorBoundaryState
}

type documentMetadataOwnerRecord struct {
	owner    *documentMetadataOwner
	metadata documentMetadataValue
}

type documentMetadataPendingOwner struct {
	owner     *documentMetadataOwner
	boundary  *errorBoundaryState
	metadata  documentMetadataValue
	ready     bool
	abandoned bool
}

type documentMetadataPendingHandoff struct {
	id              uint64
	owners          []*documentMetadataPendingOwner
	ownerSet        map[*documentMetadataOwner]*documentMetadataPendingOwner
	releases        []*documentMetadataOwner
	releaseSet      map[*documentMetadataOwner]bool
	finalizations   []*documentMetadataBoundaryFinalization
	finalizationSet map[*errorBoundaryState]*documentMetadataBoundaryFinalization
}

type documentMetadataPendingOwnerProgress struct {
	metadata  documentMetadataValue
	boundary  *errorBoundaryState
	ready     bool
	abandoned bool
}

type documentMetadataBoundaryFinalization struct {
	boundary *errorBoundaryState
	kind     documentMetadataOperationKind
}

type documentMetadataOperationKind uint8

const (
	documentMetadataPublish documentMetadataOperationKind = iota + 1
	documentMetadataRelease
	documentMetadataFailedPublish
	documentMetadataBoundaryCommitted
	documentMetadataBoundaryRecovered
	documentMetadataBoundaryFailed
	documentMetadataDelegateBoundary
	documentMetadataAbandonBoundary
)

type documentMetadataOperation struct {
	kind          documentMetadataOperationKind
	owner         *documentMetadataOwner
	metadata      documentMetadataValue
	boundary      *errorBoundaryState
	finalBoundary *errorBoundaryState
}

type documentMetadataBatch struct {
	active     bool
	id         uint64
	operations []documentMetadataOperation
	events     []documentMetadataEvent
}

type documentMetadataSnapshot struct {
	owner                *documentMetadataOwner
	metadata             documentMetadataValue
	ownerCount           int
	failedBoundaryCount  int
	retainedReleaseCount int
	batchActive          bool
}

type documentMetadataStatistics struct {
	tokenCreations         int
	committedIDAssignments int
	activeAdditions        int
	updates                int
	releases               int
	duplicatePublications  int
	rollbacks              int
	updateBatches          int
	documentPublications   int
	baselineRestorations   int
}

type documentMetadataEvent struct {
	kind       string
	batchID    uint64
	ownerID    uint64
	ownerCount int
	metadata   documentMetadataValue
}

type documentMetadataCoordinator struct {
	baseline                 documentMetadataValue
	current                  documentMetadataValue
	nextID                   uint64
	nextHandoffID            uint64
	owners                   []documentMetadataOwnerRecord
	failedBoundaries         map[*errorBoundaryState]bool
	retainedReleases         map[*errorBoundaryState]map[*documentMetadataOwner]bool
	retainedDetachIntents    []*documentMetadataOwner
	retainedDetachSet        map[*documentMetadataOwner]bool
	pendingHandoffs          map[*documentMetadataOwner]*documentMetadataPendingHandoff
	pendingHandoffOrder      []*documentMetadataPendingHandoff
	pendingFinalizations     map[*errorBoundaryState]*documentMetadataBoundaryFinalization
	pendingFinalizationOrder []*documentMetadataBoundaryFinalization
	batch                    documentMetadataBatch
	publish                  documentMetadataPublisher
	observe                  documentMetadataObserver
	statistics               documentMetadataStatistics
}

func newDocumentMetadataCoordinator(
	baseline documentMetadataValue,
	publish documentMetadataPublisher,
	observe documentMetadataObserver,
) *documentMetadataCoordinator {
	if publish == nil {
		panicRuntimeInvariant("goframe: document metadata coordinator requires a publication callback")
	}
	return &documentMetadataCoordinator{
		baseline: baseline,
		current:  baseline,
		publish:  publish,
		observe:  observe,
	}
}

func (coordinator *documentMetadataCoordinator) newOwner() *documentMetadataOwner {
	if coordinator == nil {
		panicRuntimeInvariant("goframe: document metadata coordinator is nil")
	}
	owner := &documentMetadataOwner{coordinator: coordinator}
	coordinator.statistics.tokenCreations++
	coordinator.report("owner-created", owner, documentMetadataValue{}, len(coordinator.owners))
	return owner
}

func (coordinator *documentMetadataCoordinator) beginUpdate() {
	if coordinator == nil {
		panicRuntimeInvariant("goframe: document metadata coordinator is nil")
	}
	if coordinator.batch.active {
		panicRuntimeInvariant("goframe: document metadata update is already active")
	}
	coordinator.batch.active = true
	coordinator.batch.id++
	coordinator.batch.operations = coordinator.batch.operations[:0]
	coordinator.batch.events = coordinator.batch.events[:0]
	coordinator.report("update-begin", nil, coordinator.current, len(coordinator.owners))
}

func (coordinator *documentMetadataCoordinator) stagePublish(
	owner *documentMetadataOwner,
	metadata documentMetadataValue,
) {
	coordinator.stagePublishForBoundary(owner, metadata, nil)
}

func (coordinator *documentMetadataCoordinator) stagePublishForBoundary(
	owner *documentMetadataOwner,
	metadata documentMetadataValue,
	boundary *errorBoundaryState,
) {
	coordinator.validateOwner(owner)
	coordinator.requireActiveUpdate()
	if owner.state == documentMetadataOwnerReleased {
		panicRuntimeInvariant("goframe: document metadata owner is already released")
	}
	coordinator.batch.operations = append(
		coordinator.batch.operations,
		documentMetadataOperation{
			kind:     documentMetadataPublish,
			owner:    owner,
			metadata: metadata,
			boundary: boundary,
		},
	)
	coordinator.report("publish-staged", owner, metadata, len(coordinator.owners))
}

func (coordinator *documentMetadataCoordinator) stageFailedPublish(
	owner *documentMetadataOwner,
	metadata documentMetadataValue,
	boundary *errorBoundaryState,
) {
	coordinator.validateOwner(owner)
	coordinator.requireActiveUpdate()
	coordinator.batch.operations = append(
		coordinator.batch.operations,
		documentMetadataOperation{
			kind:     documentMetadataFailedPublish,
			owner:    owner,
			metadata: metadata,
			boundary: boundary,
		},
	)
	coordinator.report("publish-rolled-back", owner, metadata, len(coordinator.owners))
}

func (coordinator *documentMetadataCoordinator) stageBoundaryOutcome(
	state *errorBoundaryState,
	final *errorBoundaryState,
	outcome protectedSubtreeLifecycleOutcome,
) {
	coordinator.requireActiveUpdate()
	if state == nil || final == nil {
		panicRuntimeInvariant("goframe: document metadata boundary is nil")
	}
	kind := documentMetadataBoundaryFailed
	switch outcome {
	case protectedSubtreeLifecycleCommitted:
		kind = documentMetadataBoundaryCommitted
		if coordinator.failedBoundaries[final] {
			kind = documentMetadataBoundaryRecovered
		}
	case protectedSubtreeLifecycleFailed:
	case protectedSubtreeLifecycleDelegated:
		kind = documentMetadataDelegateBoundary
	default:
		panicRuntimeInvariant("goframe: invalid protected subtree lifecycle outcome")
	}
	coordinator.batch.operations = append(
		coordinator.batch.operations,
		documentMetadataOperation{
			kind:          kind,
			boundary:      state,
			finalBoundary: final,
		},
	)
}

func (coordinator *documentMetadataCoordinator) stageBoundaryAbandon(
	boundary *errorBoundaryState,
) {
	coordinator.requireActiveUpdate()
	if boundary == nil {
		panicRuntimeInvariant("goframe: document metadata boundary is nil")
	}
	coordinator.batch.operations = append(
		coordinator.batch.operations,
		documentMetadataOperation{
			kind:     documentMetadataAbandonBoundary,
			boundary: boundary,
		},
	)
}

func (coordinator *documentMetadataCoordinator) stageRelease(owner *documentMetadataOwner) {
	coordinator.validateOwner(owner)
	coordinator.requireActiveUpdate()
	if owner.state == documentMetadataOwnerReleased {
		panicRuntimeInvariant("goframe: document metadata owner is already released")
	}
	staged := false
	for _, operation := range coordinator.batch.operations {
		if operation.kind == documentMetadataRelease && operation.owner == owner {
			staged = true
			break
		}
	}
	if coordinator.retainedDetachSet[owner] && !staged {
		panicRuntimeInvariant("goframe: document metadata owner was released more than once")
	}
	_, pendingSuccessor := coordinator.pendingHandoffs[owner]
	if !pendingSuccessor && !coordinator.retainedDetachSet[owner] {
		if coordinator.retainedDetachSet == nil {
			coordinator.retainedDetachSet = make(map[*documentMetadataOwner]bool)
		}
		coordinator.retainedDetachSet[owner] = true
		coordinator.retainedDetachIntents = append(
			coordinator.retainedDetachIntents,
			owner,
		)
	}
	coordinator.batch.operations = append(
		coordinator.batch.operations,
		documentMetadataOperation{
			kind:  documentMetadataRelease,
			owner: owner,
		},
	)
	coordinator.report("release-staged", owner, documentMetadataValue{}, len(coordinator.owners))
}

func (coordinator *documentMetadataCoordinator) commitUpdate() {
	coordinator.requireActiveUpdate()

	operationsInBatch := append(
		[]documentMetadataOperation(nil),
		coordinator.batch.operations...,
	)
	delegatedBoundaries := make(map[*errorBoundaryState]*errorBoundaryState)
	for _, operation := range operationsInBatch {
		if operation.kind == documentMetadataDelegateBoundary {
			delegatedBoundaries[operation.boundary] = operation.finalBoundary
		}
	}
	for index := range operationsInBatch {
		operationsInBatch[index].boundary = resolveDocumentMetadataBoundary(
			operationsInBatch[index].boundary,
			delegatedBoundaries,
		)
		operationsInBatch[index].finalBoundary = resolveDocumentMetadataBoundary(
			operationsInBatch[index].finalBoundary,
			delegatedBoundaries,
		)
	}

	currentBoundaryOperations := make(
		map[*errorBoundaryState]documentMetadataOperationKind,
	)
	for _, operation := range operationsInBatch {
		switch operation.kind {
		case documentMetadataBoundaryCommitted,
			documentMetadataBoundaryRecovered,
			documentMetadataBoundaryFailed,
			documentMetadataAbandonBoundary:
			currentBoundaryOperations[operation.boundary] = operation.kind
		}
	}

	pendingProgress := make(
		map[*documentMetadataPendingOwner]documentMetadataPendingOwnerProgress,
	)
	for _, handoff := range coordinator.pendingHandoffOrder {
		for _, pending := range handoff.owners {
			pendingProgress[pending] = documentMetadataPendingOwnerProgress{
				metadata:  pending.metadata,
				boundary:  pending.boundary,
				ready:     pending.ready,
				abandoned: pending.abandoned,
			}
		}
	}
	stagedReleases := make(map[*documentMetadataOwner]bool)
	for _, operation := range operationsInBatch {
		switch operation.kind {
		case documentMetadataRelease:
			stagedReleases[operation.owner] = true
		case documentMetadataPublish:
		}
		handoff := coordinator.pendingHandoffs[operation.owner]
		if handoff == nil {
			continue
		}
		pending := handoff.ownerSet[operation.owner]
		progress := pendingProgress[pending]
		switch operation.kind {
		case documentMetadataRelease:
			progress.abandoned = true
			progress.ready = false
		case documentMetadataPublish:
			progress.metadata = operation.metadata
			progress.boundary = operation.boundary
			progress.ready = true
		}
		pendingProgress[pending] = progress
	}
	resolvingHandoffs := make(map[*documentMetadataPendingHandoff]bool)
	abandonedPendingOwners := make(
		map[*documentMetadataOwner]*documentMetadataPendingHandoff,
	)
	for _, handoff := range coordinator.pendingHandoffOrder {
		remaining := 0
		allRemainingReady := true
		for _, pending := range handoff.owners {
			progress := pendingProgress[pending]
			if progress.abandoned {
				abandonedPendingOwners[pending.owner] = handoff
				continue
			}
			remaining++
			if !progress.ready {
				allRemainingReady = false
			}
		}
		if remaining != 0 && !allRemainingReady {
			continue
		}
		resolvingHandoffs[handoff] = true
	}
	operations := make(
		[]documentMetadataOperation,
		0,
		len(coordinator.retainedDetachIntents)+len(coordinator.batch.operations),
	)
	for _, operation := range operationsInBatch {
		if coordinator.pendingHandoffs[operation.owner] != nil {
			switch operation.kind {
			case documentMetadataRelease, documentMetadataPublish:
				continue
			}
		}
		operations = append(operations, operation)
	}
	for _, owner := range coordinator.retainedDetachIntents {
		if !stagedReleases[owner] {
			operations = append(operations, documentMetadataOperation{
				kind:  documentMetadataRelease,
				owner: owner,
			})
		}
	}
	resolvedHandoffs := make(map[*documentMetadataPendingHandoff]bool)
	for _, handoff := range coordinator.pendingHandoffOrder {
		if !resolvingHandoffs[handoff] {
			continue
		}
		for _, pending := range handoff.owners {
			progress := pendingProgress[pending]
			if progress.abandoned {
				continue
			}
			operations = append(operations, documentMetadataOperation{
				kind:     documentMetadataPublish,
				owner:    pending.owner,
				metadata: progress.metadata,
				boundary: progress.boundary,
			})
		}
		for _, owner := range handoff.releases {
			if stagedReleases[owner] {
				continue
			}
			operations = append(operations, documentMetadataOperation{
				kind:  documentMetadataRelease,
				owner: owner,
			})
			stagedReleases[owner] = true
		}
		resolvedHandoffs[handoff] = true
	}
	effectiveBoundaryOperations := make(
		map[*errorBoundaryState]documentMetadataOperationKind,
		len(currentBoundaryOperations),
	)
	for boundary, kind := range currentBoundaryOperations {
		effectiveBoundaryOperations[boundary] = kind
	}
	for _, handoff := range coordinator.pendingHandoffOrder {
		if !resolvingHandoffs[handoff] {
			continue
		}
		for _, finalization := range handoff.finalizations {
			if effectiveBoundaryOperations[finalization.boundary] != 0 {
				continue
			}
			operations = append(operations, documentMetadataOperation{
				kind:     finalization.kind,
				boundary: finalization.boundary,
			})
			effectiveBoundaryOperations[finalization.boundary] = finalization.kind
		}
	}
	for _, finalization := range coordinator.pendingFinalizationOrder {
		if effectiveBoundaryOperations[finalization.boundary] != 0 {
			continue
		}
		operations = append(operations, documentMetadataOperation{
			kind:     finalization.kind,
			boundary: finalization.boundary,
		})
		effectiveBoundaryOperations[finalization.boundary] = finalization.kind
	}

	owners := append([]documentMetadataOwnerRecord(nil), coordinator.owners...)
	indices := make(map[*documentMetadataOwner]int, len(owners))
	for index := range owners {
		indices[owners[index].owner] = index
	}
	failedBoundaries := cloneDocumentMetadataFailedBoundaries(
		coordinator.failedBoundaries,
	)
	retainedReleases := cloneDocumentMetadataRetainedReleases(
		coordinator.retainedReleases,
	)
	abandonedBoundaries := make(map[*errorBoundaryState]bool)
	recoveredBoundaries := make(map[*errorBoundaryState]bool)
	for _, operation := range operations {
		switch operation.kind {
		case documentMetadataBoundaryFailed:
			failedBoundaries[operation.boundary] = true
		case documentMetadataBoundaryCommitted,
			documentMetadataBoundaryRecovered:
			recoveredBoundaries[operation.boundary] = true
		case documentMetadataAbandonBoundary:
			abandonedBoundaries[operation.boundary] = true
		}
	}

	released := make(map[*documentMetadataOwner]bool)
	consumedRetained := make(map[*documentMetadataOwner]bool)
	finalizedBoundaries := make(map[*errorBoundaryState]bool)
	for boundary := range recoveredBoundaries {
		finalizedBoundaries[boundary] = true
	}
	for boundary := range abandonedBoundaries {
		finalizedBoundaries[boundary] = true
	}
	for boundary := range finalizedBoundaries {
		delete(failedBoundaries, boundary)
		for owner := range retainedReleases[boundary] {
			var removed bool
			owners, removed = removeDocumentMetadataOwner(owners, indices, owner)
			if removed {
				released[owner] = true
				consumedRetained[owner] = true
			}
		}
		delete(retainedReleases, boundary)
	}

	duplicatePublications := 0
	boundaries := make(map[*documentMetadataOwner]documentMetadataOperation)

	for _, operation := range operations {
		switch operation.kind {
		case documentMetadataPublish:
			if released[operation.owner] {
				panicRuntimeInvariant("goframe: document metadata owner cannot publish after release")
			}
			if operation.owner.id == 0 {
				boundaries[operation.owner] = operation
			}
			if index, ok := indices[operation.owner]; ok {
				if owners[index].metadata == operation.metadata {
					duplicatePublications++
				}
				owners[index].metadata = operation.metadata
				continue
			}
			indices[operation.owner] = len(owners)
			owners = append(owners, documentMetadataOwnerRecord{
				owner:    operation.owner,
				metadata: operation.metadata,
			})
		case documentMetadataRelease:
			if released[operation.owner] {
				if consumedRetained[operation.owner] {
					continue
				}
				panicRuntimeInvariant("goframe: document metadata owner was released more than once")
			}
			index, ok := indices[operation.owner]
			if !ok {
				panicRuntimeInvariant("goframe: document metadata owner is not active")
			}
			boundary := operation.owner.boundary
			if boundary != nil &&
				failedBoundaries[boundary] &&
				!finalizedBoundaries[boundary] {
				if retainedReleases[boundary] == nil {
					retainedReleases[boundary] = make(map[*documentMetadataOwner]bool)
				}
				retainedReleases[boundary][operation.owner] = true
				released[operation.owner] = true
				continue
			}
			owners = removeDocumentMetadataOwnerAt(owners, indices, index)
			released[operation.owner] = true
		case documentMetadataFailedPublish,
			documentMetadataBoundaryCommitted,
			documentMetadataBoundaryRecovered,
			documentMetadataBoundaryFailed,
			documentMetadataDelegateBoundary,
			documentMetadataAbandonBoundary:
		default:
			panicRuntimeInvariant("goframe: invalid document metadata operation")
		}
	}

	previous := make(map[*documentMetadataOwner]documentMetadataValue, len(coordinator.owners))
	for _, record := range coordinator.owners {
		previous[record.owner] = record.metadata
	}
	active := make(map[*documentMetadataOwner]bool, len(owners))
	nextID := coordinator.nextID
	assignments := make(map[*documentMetadataOwner]uint64)
	additions := 0
	updates := 0
	for _, record := range owners {
		active[record.owner] = true
		oldMetadata, existed := previous[record.owner]
		if !existed {
			additions++
		}
		if existed && oldMetadata != record.metadata {
			updates++
		}
		if record.owner.id == 0 {
			nextID++
			assignments[record.owner] = nextID
		}
	}
	releases := 0
	for _, record := range coordinator.owners {
		if !active[record.owner] {
			releases++
		}
	}

	next := coordinator.baseline
	if len(owners) > 0 {
		next = owners[len(owners)-1].metadata
	}
	publish := next != coordinator.current
	if publish {
		if err := coordinator.publish(coordinator.current, next); err != nil {
			coordinator.applyPendingOwnerProgress(pendingProgress)
			failedHandoff := coordinator.retainFailedHandoff(
				operations,
				owners,
				active,
			)
			affectedHandoffs := make(map[*documentMetadataPendingHandoff]bool)
			for handoff := range resolvedHandoffs {
				affectedHandoffs[handoff] = true
			}
			if failedHandoff != nil {
				affectedHandoffs[failedHandoff] = true
			}
			for owner, handoff := range abandonedPendingOwners {
				if pending := handoff.ownerSet[owner]; pending != nil {
					pending.abandoned = true
				}
			}
			handoffBoundaries := coordinator.retainHandoffFinalizations(
				affectedHandoffs,
				operations,
				currentBoundaryOperations,
			)
			coordinator.retainBoundaryFinalizations(
				operations,
				currentBoundaryOperations,
				handoffBoundaries,
			)
			coordinator.discardUpdate()
			panic(wrapDocumentMetadataError("goframe: document metadata publication failed", err))
		}
	}

	for _, record := range coordinator.owners {
		if !active[record.owner] {
			record.owner.state = documentMetadataOwnerReleased
			record.owner.boundary = nil
		}
	}
	for owner := range released {
		if !active[owner] {
			owner.state = documentMetadataOwnerReleased
			owner.boundary = nil
		}
	}
	for _, record := range owners {
		if id := assignments[record.owner]; id != 0 {
			record.owner.id = id
			coordinator.report("owner-committed", record.owner, record.metadata, len(owners))
		}
		if operation, ok := boundaries[record.owner]; ok {
			record.owner.boundary = operation.boundary
		}
		record.owner.state = documentMetadataOwnerActive
	}

	previousCurrent := coordinator.current
	coordinator.owners = owners
	coordinator.nextID = nextID
	coordinator.current = next
	coordinator.failedBoundaries = failedBoundaries
	coordinator.retainedReleases = retainedReleases
	coordinator.clearRetainedDetachIntents()
	coordinator.applyPendingOwnerProgress(pendingProgress)
	coordinator.clearSupersededHandoffFinalizations(currentBoundaryOperations)
	coordinator.clearAppliedBoundaryFinalizations(operations)
	for owner, handoff := range abandonedPendingOwners {
		if !active[owner] {
			owner.state = documentMetadataOwnerReleased
			owner.boundary = nil
		}
		coordinator.removePendingHandoffOwner(handoff, owner)
	}
	for handoff := range resolvedHandoffs {
		coordinator.removePendingHandoff(handoff)
	}
	coordinator.statistics.committedIDAssignments += len(assignments)
	coordinator.statistics.activeAdditions += additions
	coordinator.statistics.updates += updates
	coordinator.statistics.releases += releases
	coordinator.statistics.duplicatePublications += duplicatePublications
	coordinator.statistics.updateBatches++
	if publish {
		coordinator.statistics.documentPublications++
		if previousCurrent != coordinator.baseline && next == coordinator.baseline {
			coordinator.statistics.baselineRestorations++
		}
		coordinator.report("document-published", coordinator.selectedOwner(), next, len(owners))
	}
	coordinator.report("update-commit", coordinator.selectedOwner(), next, len(owners))
	events := coordinator.finishUpdate()
	coordinator.notify(events)
}

func (coordinator *documentMetadataCoordinator) applyPendingOwnerProgress(
	progress map[*documentMetadataPendingOwner]documentMetadataPendingOwnerProgress,
) {
	for pending, next := range progress {
		if pending == nil {
			continue
		}
		pending.metadata = next.metadata
		pending.boundary = next.boundary
		pending.ready = next.ready
		pending.abandoned = next.abandoned
	}
}

func (coordinator *documentMetadataCoordinator) clearRetainedDetachIntents() {
	clear(coordinator.retainedDetachIntents)
	clear(coordinator.retainedDetachSet)
	coordinator.retainedDetachIntents = coordinator.retainedDetachIntents[:0]
}

func (coordinator *documentMetadataCoordinator) retainFailedHandoff(
	operations []documentMetadataOperation,
	owners []documentMetadataOwnerRecord,
	active map[*documentMetadataOwner]bool,
) *documentMetadataPendingHandoff {
	if len(owners) == 0 {
		return nil
	}
	published := make(map[*documentMetadataOwner]documentMetadataOperation)
	for _, operation := range operations {
		if operation.kind == documentMetadataPublish {
			published[operation.owner] = operation
		}
	}
	pendingOwners := make([]documentMetadataOperation, 0, len(owners))
	for _, record := range owners {
		owner := record.owner
		operation, ok := published[owner]
		if !ok || owner == nil || owner.id != 0 ||
			owner.state != documentMetadataOwnerPending {
			continue
		}
		pendingOwners = append(pendingOwners, operation)
	}
	if len(pendingOwners) == 0 {
		return nil
	}

	var outgoing []*documentMetadataOwner
	outgoingSet := make(map[*documentMetadataOwner]bool)
	for _, owner := range coordinator.retainedDetachIntents {
		if owner == nil || active[owner] || owner.id == 0 {
			continue
		}
		outgoing = append(outgoing, owner)
		outgoingSet[owner] = true
	}
	for _, pending := range pendingOwners {
		for owner := range coordinator.retainedReleases[pending.boundary] {
			if owner == nil || active[owner] || owner.id == 0 || outgoingSet[owner] {
				continue
			}
			outgoing = append(outgoing, owner)
			outgoingSet[owner] = true
		}
	}
	var causalFinalizations []*documentMetadataBoundaryFinalization
	for _, finalization := range append(
		[]*documentMetadataBoundaryFinalization(nil),
		coordinator.pendingFinalizationOrder...,
	) {
		causal := false
		for owner := range coordinator.retainedReleases[finalization.boundary] {
			if owner == nil || active[owner] || owner.id == 0 {
				continue
			}
			causal = true
			if outgoingSet[owner] {
				continue
			}
			outgoing = append(outgoing, owner)
			outgoingSet[owner] = true
		}
		if causal {
			causalFinalizations = append(causalFinalizations, finalization)
		}
	}
	var handoff *documentMetadataPendingHandoff
	selectHandoff := func(candidate *documentMetadataPendingHandoff) {
		if candidate == nil {
			return
		}
		if handoff != nil && handoff != candidate {
			panicRuntimeInvariant("goframe: overlapping document metadata ownership plans")
		}
		handoff = candidate
	}
	for _, pending := range pendingOwners {
		selectHandoff(coordinator.pendingHandoffs[pending.owner])
	}
	for _, existing := range coordinator.pendingHandoffOrder {
		for _, owner := range outgoing {
			if existing.releaseSet[owner] {
				selectHandoff(existing)
			}
		}
	}
	if coordinator.pendingHandoffs == nil {
		coordinator.pendingHandoffs = make(
			map[*documentMetadataOwner]*documentMetadataPendingHandoff,
		)
	}
	if handoff == nil {
		coordinator.nextHandoffID++
		handoff = &documentMetadataPendingHandoff{
			id:         coordinator.nextHandoffID,
			ownerSet:   make(map[*documentMetadataOwner]*documentMetadataPendingOwner),
			releaseSet: make(map[*documentMetadataOwner]bool),
		}
		coordinator.pendingHandoffOrder = append(
			coordinator.pendingHandoffOrder,
			handoff,
		)
	}
	for _, operation := range pendingOwners {
		pending := handoff.ownerSet[operation.owner]
		if pending == nil {
			pending = &documentMetadataPendingOwner{
				owner:    operation.owner,
				boundary: operation.boundary,
				metadata: operation.metadata,
			}
			handoff.ownerSet[operation.owner] = pending
			handoff.owners = append(handoff.owners, pending)
			coordinator.pendingHandoffs[operation.owner] = handoff
		}
	}
	for _, owner := range outgoing {
		if handoff.releaseSet[owner] {
			continue
		}
		handoff.releaseSet[owner] = true
		handoff.releases = append(handoff.releases, owner)
		coordinator.removeRetainedDetachIntent(owner)
	}
	for _, finalization := range causalFinalizations {
		if coordinator.handoffConsumesBoundaryFinalization(
			handoff,
			finalization.boundary,
		) {
			coordinator.retainHandoffFinalization(
				handoff,
				finalization.boundary,
				finalization.kind,
			)
		}
	}
	return handoff
}

func (coordinator *documentMetadataCoordinator) removePendingHandoff(
	handoff *documentMetadataPendingHandoff,
) {
	if handoff == nil {
		return
	}
	for _, pending := range handoff.owners {
		delete(coordinator.pendingHandoffs, pending.owner)
	}
	for index, pending := range coordinator.pendingHandoffOrder {
		if pending != handoff {
			continue
		}
		copy(
			coordinator.pendingHandoffOrder[index:],
			coordinator.pendingHandoffOrder[index+1:],
		)
		last := len(coordinator.pendingHandoffOrder) - 1
		coordinator.pendingHandoffOrder[last] = nil
		coordinator.pendingHandoffOrder = coordinator.pendingHandoffOrder[:last]
		return
	}
}

func (coordinator *documentMetadataCoordinator) removePendingHandoffOwner(
	handoff *documentMetadataPendingHandoff,
	owner *documentMetadataOwner,
) {
	if handoff == nil || handoff.ownerSet[owner] == nil {
		return
	}
	delete(coordinator.pendingHandoffs, owner)
	delete(handoff.ownerSet, owner)
	for index, pending := range handoff.owners {
		if pending.owner != owner {
			continue
		}
		copy(handoff.owners[index:], handoff.owners[index+1:])
		last := len(handoff.owners) - 1
		handoff.owners[last] = nil
		handoff.owners = handoff.owners[:last]
		break
	}
}

func (coordinator *documentMetadataCoordinator) retainHandoffFinalizations(
	handoffs map[*documentMetadataPendingHandoff]bool,
	operations []documentMetadataOperation,
	current map[*errorBoundaryState]documentMetadataOperationKind,
) map[*errorBoundaryState]bool {
	boundaries := make(map[*errorBoundaryState]bool)
	for handoff := range handoffs {
		for _, finalization := range handoff.finalizations {
			boundaries[finalization.boundary] = true
		}
		for boundary, kind := range current {
			if kind == documentMetadataBoundaryFailed &&
				handoff.finalizationSet[boundary] != nil {
				coordinator.removeHandoffFinalization(handoff, boundary)
				coordinator.removePendingBoundaryFinalization(boundary)
				delete(boundaries, boundary)
			}
		}
		for _, operation := range operations {
			if operation.boundary == nil ||
				!coordinator.handoffConsumesBoundaryFinalization(
					handoff,
					operation.boundary,
				) ||
				current[operation.boundary] == documentMetadataBoundaryFailed {
				continue
			}
			switch operation.kind {
			case documentMetadataBoundaryRecovered,
				documentMetadataAbandonBoundary:
				coordinator.retainHandoffFinalization(
					handoff,
					operation.boundary,
					operation.kind,
				)
				boundaries[operation.boundary] = true
			}
		}
	}
	return boundaries
}

func (coordinator *documentMetadataCoordinator) handoffConsumesBoundaryFinalization(
	handoff *documentMetadataPendingHandoff,
	boundary *errorBoundaryState,
) bool {
	if handoff == nil || boundary == nil {
		return false
	}
	for owner := range coordinator.retainedReleases[boundary] {
		if handoff.releaseSet[owner] {
			return true
		}
	}
	return false
}

func (coordinator *documentMetadataCoordinator) retainHandoffFinalization(
	handoff *documentMetadataPendingHandoff,
	boundary *errorBoundaryState,
	kind documentMetadataOperationKind,
) {
	if handoff == nil || boundary == nil {
		return
	}
	if kind != documentMetadataBoundaryRecovered &&
		kind != documentMetadataAbandonBoundary {
		panicRuntimeInvariant("goframe: invalid document metadata handoff finalization")
	}
	if handoff.finalizationSet == nil {
		handoff.finalizationSet = make(
			map[*errorBoundaryState]*documentMetadataBoundaryFinalization,
		)
	}
	if finalization := handoff.finalizationSet[boundary]; finalization != nil {
		if kind == documentMetadataAbandonBoundary {
			finalization.kind = kind
		}
		coordinator.removePendingBoundaryFinalization(boundary)
		return
	}
	finalization := &documentMetadataBoundaryFinalization{
		boundary: boundary,
		kind:     kind,
	}
	handoff.finalizationSet[boundary] = finalization
	handoff.finalizations = append(handoff.finalizations, finalization)
	coordinator.removePendingBoundaryFinalization(boundary)
}

func (coordinator *documentMetadataCoordinator) clearSupersededHandoffFinalizations(
	current map[*errorBoundaryState]documentMetadataOperationKind,
) {
	for boundary, kind := range current {
		if kind != documentMetadataBoundaryFailed {
			continue
		}
		for _, handoff := range coordinator.pendingHandoffOrder {
			coordinator.removeHandoffFinalization(handoff, boundary)
		}
	}
}

func (coordinator *documentMetadataCoordinator) removeHandoffFinalization(
	handoff *documentMetadataPendingHandoff,
	boundary *errorBoundaryState,
) {
	if handoff == nil {
		return
	}
	finalization := handoff.finalizationSet[boundary]
	if finalization == nil {
		return
	}
	delete(handoff.finalizationSet, boundary)
	for index, pending := range handoff.finalizations {
		if pending != finalization {
			continue
		}
		copy(handoff.finalizations[index:], handoff.finalizations[index+1:])
		last := len(handoff.finalizations) - 1
		handoff.finalizations[last] = nil
		handoff.finalizations = handoff.finalizations[:last]
		return
	}
}

func (coordinator *documentMetadataCoordinator) retainBoundaryFinalizations(
	operations []documentMetadataOperation,
	current map[*errorBoundaryState]documentMetadataOperationKind,
	handoffBoundaries map[*errorBoundaryState]bool,
) {
	for boundary, kind := range current {
		if kind == documentMetadataBoundaryFailed {
			coordinator.removePendingBoundaryFinalization(boundary)
		}
	}
	for _, operation := range operations {
		if operation.boundary == nil || handoffBoundaries[operation.boundary] ||
			current[operation.boundary] == documentMetadataBoundaryFailed {
			continue
		}
		switch operation.kind {
		case documentMetadataBoundaryRecovered,
			documentMetadataAbandonBoundary:
			coordinator.retainBoundaryFinalization(
				operation.boundary,
				operation.kind,
			)
		}
	}
}

func (coordinator *documentMetadataCoordinator) retainBoundaryFinalization(
	boundary *errorBoundaryState,
	kind documentMetadataOperationKind,
) {
	if boundary == nil {
		return
	}
	if kind != documentMetadataBoundaryRecovered &&
		kind != documentMetadataAbandonBoundary {
		panicRuntimeInvariant("goframe: invalid document metadata boundary finalization")
	}
	if coordinator.pendingFinalizations == nil {
		coordinator.pendingFinalizations = make(
			map[*errorBoundaryState]*documentMetadataBoundaryFinalization,
		)
	}
	if finalization := coordinator.pendingFinalizations[boundary]; finalization != nil {
		if kind == documentMetadataAbandonBoundary {
			finalization.kind = kind
		}
		return
	}
	finalization := &documentMetadataBoundaryFinalization{
		boundary: boundary,
		kind:     kind,
	}
	coordinator.pendingFinalizations[boundary] = finalization
	coordinator.pendingFinalizationOrder = append(
		coordinator.pendingFinalizationOrder,
		finalization,
	)
}

func (coordinator *documentMetadataCoordinator) clearAppliedBoundaryFinalizations(
	operations []documentMetadataOperation,
) {
	for _, operation := range operations {
		switch operation.kind {
		case documentMetadataBoundaryRecovered,
			documentMetadataBoundaryFailed,
			documentMetadataAbandonBoundary:
			coordinator.removePendingBoundaryFinalization(operation.boundary)
		}
	}
}

func (coordinator *documentMetadataCoordinator) removePendingBoundaryFinalization(
	boundary *errorBoundaryState,
) {
	finalization := coordinator.pendingFinalizations[boundary]
	if finalization == nil {
		return
	}
	delete(coordinator.pendingFinalizations, boundary)
	for index, pending := range coordinator.pendingFinalizationOrder {
		if pending != finalization {
			continue
		}
		copy(
			coordinator.pendingFinalizationOrder[index:],
			coordinator.pendingFinalizationOrder[index+1:],
		)
		last := len(coordinator.pendingFinalizationOrder) - 1
		coordinator.pendingFinalizationOrder[last] = nil
		coordinator.pendingFinalizationOrder = coordinator.pendingFinalizationOrder[:last]
		return
	}
}

func (coordinator *documentMetadataCoordinator) removeRetainedDetachIntent(
	owner *documentMetadataOwner,
) {
	if !coordinator.retainedDetachSet[owner] {
		return
	}
	delete(coordinator.retainedDetachSet, owner)
	for index, retained := range coordinator.retainedDetachIntents {
		if retained != owner {
			continue
		}
		copy(
			coordinator.retainedDetachIntents[index:],
			coordinator.retainedDetachIntents[index+1:],
		)
		last := len(coordinator.retainedDetachIntents) - 1
		coordinator.retainedDetachIntents[last] = nil
		coordinator.retainedDetachIntents = coordinator.retainedDetachIntents[:last]
		return
	}
}

func resolveDocumentMetadataBoundary(
	boundary *errorBoundaryState,
	delegated map[*errorBoundaryState]*errorBoundaryState,
) *errorBoundaryState {
	for boundary != nil {
		final := delegated[boundary]
		if final == nil || final == boundary {
			return boundary
		}
		boundary = final
	}
	return nil
}

func cloneDocumentMetadataFailedBoundaries(
	source map[*errorBoundaryState]bool,
) map[*errorBoundaryState]bool {
	cloned := make(map[*errorBoundaryState]bool, len(source))
	for boundary := range source {
		cloned[boundary] = true
	}
	return cloned
}

func cloneDocumentMetadataRetainedReleases(
	source map[*errorBoundaryState]map[*documentMetadataOwner]bool,
) map[*errorBoundaryState]map[*documentMetadataOwner]bool {
	cloned := make(
		map[*errorBoundaryState]map[*documentMetadataOwner]bool,
		len(source),
	)
	for boundary, owners := range source {
		clonedOwners := make(map[*documentMetadataOwner]bool, len(owners))
		for owner := range owners {
			clonedOwners[owner] = true
		}
		cloned[boundary] = clonedOwners
	}
	return cloned
}

func removeDocumentMetadataOwner(
	owners []documentMetadataOwnerRecord,
	indices map[*documentMetadataOwner]int,
	owner *documentMetadataOwner,
) ([]documentMetadataOwnerRecord, bool) {
	index, ok := indices[owner]
	if !ok {
		return owners, false
	}
	return removeDocumentMetadataOwnerAt(owners, indices, index), true
}

func removeDocumentMetadataOwnerAt(
	owners []documentMetadataOwnerRecord,
	indices map[*documentMetadataOwner]int,
	index int,
) []documentMetadataOwnerRecord {
	owner := owners[index].owner
	delete(indices, owner)
	copy(owners[index:], owners[index+1:])
	owners = owners[:len(owners)-1]
	for next := index; next < len(owners); next++ {
		indices[owners[next].owner] = next
	}
	return owners
}

func (coordinator *documentMetadataCoordinator) rollbackUpdate() {
	coordinator.requireActiveUpdate()
	coordinator.statistics.rollbacks++
	coordinator.statistics.updateBatches++
	coordinator.report("update-rollback", coordinator.selectedOwner(), coordinator.current, len(coordinator.owners))
	events := coordinator.finishUpdate()
	coordinator.notify(events)
}

func (coordinator *documentMetadataCoordinator) finishUpdate() []documentMetadataEvent {
	events := append([]documentMetadataEvent(nil), coordinator.batch.events...)
	clear(coordinator.batch.operations)
	clear(coordinator.batch.events)
	coordinator.batch.operations = coordinator.batch.operations[:0]
	coordinator.batch.events = coordinator.batch.events[:0]
	coordinator.batch.active = false
	return events
}

func (coordinator *documentMetadataCoordinator) discardUpdate() {
	clear(coordinator.batch.operations)
	clear(coordinator.batch.events)
	coordinator.batch.operations = coordinator.batch.operations[:0]
	coordinator.batch.events = coordinator.batch.events[:0]
	coordinator.batch.active = false
}

func (coordinator *documentMetadataCoordinator) snapshot() documentMetadataSnapshot {
	if coordinator == nil {
		return documentMetadataSnapshot{}
	}
	return documentMetadataSnapshot{
		owner:                coordinator.selectedOwner(),
		metadata:             coordinator.current,
		ownerCount:           len(coordinator.owners),
		failedBoundaryCount:  len(coordinator.failedBoundaries),
		retainedReleaseCount: coordinator.retainedReleaseCount(),
		batchActive:          coordinator.batch.active,
	}
}

func (coordinator *documentMetadataCoordinator) retainedReleaseCount() int {
	retained := make(map[*documentMetadataOwner]bool)
	for _, owner := range coordinator.retainedDetachIntents {
		retained[owner] = true
	}
	for _, handoff := range coordinator.pendingHandoffOrder {
		for _, owner := range handoff.releases {
			retained[owner] = true
		}
	}
	for _, owners := range coordinator.retainedReleases {
		for owner := range owners {
			retained[owner] = true
		}
	}
	return len(retained)
}

func (coordinator *documentMetadataCoordinator) ownerIDs() []uint64 {
	ids := make([]uint64, len(coordinator.owners))
	for index, record := range coordinator.owners {
		ids[index] = record.owner.id
	}
	return ids
}

func (coordinator *documentMetadataCoordinator) selectedOwner() *documentMetadataOwner {
	if coordinator == nil || len(coordinator.owners) == 0 {
		return nil
	}
	return coordinator.owners[len(coordinator.owners)-1].owner
}

func (coordinator *documentMetadataCoordinator) validateOwner(owner *documentMetadataOwner) {
	if coordinator == nil {
		panicRuntimeInvariant("goframe: document metadata coordinator is nil")
	}
	if owner == nil {
		panicRuntimeInvariant("goframe: document metadata owner is nil")
	}
	if owner.coordinator != coordinator {
		panicRuntimeInvariant("goframe: document metadata owner belongs to another coordinator")
	}
}

func (coordinator *documentMetadataCoordinator) requireActiveUpdate() {
	if coordinator == nil || !coordinator.batch.active {
		panicRuntimeInvariant("goframe: document metadata operation requires an active update")
	}
}

func (coordinator *documentMetadataCoordinator) report(
	kind string,
	owner *documentMetadataOwner,
	metadata documentMetadataValue,
	ownerCount int,
) {
	if coordinator == nil || coordinator.observe == nil {
		return
	}
	var ownerID uint64
	if owner != nil {
		ownerID = owner.id
	}
	event := documentMetadataEvent{
		kind:       kind,
		batchID:    coordinator.batch.id,
		ownerID:    ownerID,
		ownerCount: ownerCount,
		metadata:   metadata,
	}
	if coordinator.batch.active {
		coordinator.batch.events = append(coordinator.batch.events, event)
		return
	}
	coordinator.notify([]documentMetadataEvent{event})
}

func (coordinator *documentMetadataCoordinator) notify(
	events []documentMetadataEvent,
) {
	if coordinator == nil || coordinator.observe == nil {
		return
	}
	for _, event := range events {
		if err := coordinator.observe(event); err != nil {
			panic(wrapDocumentMetadataError("goframe: document metadata observer failed", err))
		}
	}
}

func (owner *documentMetadataOwner) prepareRender(
	attempt *renderLifecycleAttempt,
	metadata documentMetadataValue,
) {
	if owner == nil || owner.coordinator == nil {
		panicRuntimeInvariant("goframe: document metadata owner is nil")
	}
	if owner.state == documentMetadataOwnerReleased {
		panicRuntimeInvariant("goframe: document metadata owner is already released")
	}
	if owner.pending.attempt != nil {
		panicRuntimeInvariant("goframe: document metadata owner already participated in this render attempt")
	}
	boundary := currentDocumentMetadataBoundary()
	owner.pending = documentMetadataRenderState{
		attempt:  attempt,
		metadata: metadata,
		boundary: boundary,
	}
	attempt.participants = append(attempt.participants, owner)
}

func (owner *documentMetadataOwner) finishRender(
	attempt *renderLifecycleAttempt,
	commit bool,
) {
	pending := owner.pending
	if pending.attempt != attempt {
		return
	}
	owner.pending = documentMetadataRenderState{}
	if commit {
		owner.coordinator.stagePublishForBoundary(
			owner,
			pending.metadata,
			pending.boundary,
		)
		return
	}
	owner.coordinator.stageFailedPublish(
		owner,
		pending.metadata,
		pending.boundary,
	)
}

func currentDocumentMetadataBoundary() *errorBoundaryState {
	boundary := currentProtectedLifecycleBoundary
	if boundary == nil {
		return nil
	}
	for instance := currentComponent; instance != nil; instance = instance.parent {
		if instance.errorBoundary == boundary {
			return boundary
		}
	}
	panicRuntimeInvariant("goframe: document metadata boundary owner is unavailable")
	return nil
}

var activeDocumentMetadataCoordinator *documentMetadataCoordinator

func installDocumentMetadataCoordinator(coordinator *documentMetadataCoordinator) {
	if coordinator == nil {
		panicRuntimeInvariant("goframe: document metadata coordinator is nil")
	}
	if activeDocumentMetadataCoordinator != nil {
		panicRuntimeInvariant("goframe: document metadata coordinator is already installed")
	}
	activeDocumentMetadataCoordinator = coordinator
	reportProtectedSubtreeLifecycleOutcome = func(
		state *errorBoundaryState,
		final *errorBoundaryState,
		outcome protectedSubtreeLifecycleOutcome,
	) {
		coordinator.stageBoundaryOutcome(state, final, outcome)
	}
	reportProtectedSubtreeLifecycleAbandon = coordinator.stageBoundaryAbandon
}

func uninstallDocumentMetadataCoordinator() {
	if activeDocumentMetadataCoordinator != nil && activeDocumentMetadataCoordinator.batch.active {
		panicRuntimeInvariant("goframe: cannot uninstall document metadata coordinator during an update")
	}
	reportProtectedSubtreeLifecycleOutcome = nil
	reportProtectedSubtreeLifecycleAbandon = nil
	activeDocumentMetadataCoordinator = nil
}

func useDocumentMetadata(metadata documentMetadataValue) {
	coordinator := activeDocumentMetadataCoordinator
	if coordinator == nil {
		panicRuntimeInvariant("goframe: document metadata coordinator is not installed")
	}
	state := useStateSlot[*documentMetadataOwner](nil, "document metadata")
	owner := state.get()
	if owner == nil {
		owner = coordinator.newOwner()
		state.slot.value = owner
	}
	if owner.coordinator != coordinator {
		panicRuntimeInvariant("goframe: document metadata owner belongs to another coordinator")
	}
	owner.prepareRender(requireLifecycleRenderAttempt(currentComponent), metadata)
	UseUnmount(func() {
		owner.coordinator.stageRelease(owner)
	})
}

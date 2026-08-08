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
	owner    *documentMetadataOwner
	boundary *errorBoundaryState
}

type documentMetadataPendingHandoff struct {
	owners     []*documentMetadataPendingOwner
	ownerSet   map[*documentMetadataOwner]*documentMetadataPendingOwner
	releases   []*documentMetadataOwner
	releaseSet map[*documentMetadataOwner]bool
	boundary   *errorBoundaryState
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
		panic("goframe: document metadata coordinator requires a publication callback")
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
		panic("goframe: document metadata coordinator is nil")
	}
	owner := &documentMetadataOwner{coordinator: coordinator}
	coordinator.statistics.tokenCreations++
	coordinator.report("owner-created", owner, documentMetadataValue{}, len(coordinator.owners))
	return owner
}

func (coordinator *documentMetadataCoordinator) beginUpdate() {
	if coordinator == nil {
		panic("goframe: document metadata coordinator is nil")
	}
	if coordinator.batch.active {
		panic("goframe: document metadata update is already active")
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
		panic("goframe: document metadata owner is already released")
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
		panic("goframe: document metadata boundary is nil")
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
		panic("goframe: invalid protected subtree lifecycle outcome")
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
		panic("goframe: document metadata boundary is nil")
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
		panic("goframe: document metadata owner is already released")
	}
	staged := false
	for _, operation := range coordinator.batch.operations {
		if operation.kind == documentMetadataRelease && operation.owner == owner {
			staged = true
			break
		}
	}
	if coordinator.retainedDetachSet[owner] && !staged {
		panic("goframe: document metadata owner was released more than once")
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

	pendingOwnerReleases := make(map[*documentMetadataOwner]bool)
	pendingOwnerPublishes := make(map[*documentMetadataOwner]bool)
	stagedReleases := make(map[*documentMetadataOwner]bool)
	for _, operation := range coordinator.batch.operations {
		switch operation.kind {
		case documentMetadataRelease:
			stagedReleases[operation.owner] = true
			if coordinator.pendingHandoffs[operation.owner] != nil {
				pendingOwnerReleases[operation.owner] = true
			}
		case documentMetadataPublish:
			if coordinator.pendingHandoffs[operation.owner] != nil {
				pendingOwnerPublishes[operation.owner] = true
			}
		}
	}
	resolvingHandoffs := make(map[*documentMetadataPendingHandoff]bool)
	abandonedPendingOwners := make(
		map[*documentMetadataOwner]*documentMetadataPendingHandoff,
	)
	allowedPendingPublishes := make(map[*documentMetadataOwner]bool)
	for _, handoff := range coordinator.pendingHandoffOrder {
		remaining := 0
		allRemainingPublished := true
		for _, pending := range handoff.owners {
			if pendingOwnerReleases[pending.owner] {
				abandonedPendingOwners[pending.owner] = handoff
				continue
			}
			remaining++
			if !pendingOwnerPublishes[pending.owner] {
				allRemainingPublished = false
			}
		}
		if remaining != 0 && !allRemainingPublished {
			continue
		}
		resolvingHandoffs[handoff] = true
		for _, pending := range handoff.owners {
			if !pendingOwnerReleases[pending.owner] {
				allowedPendingPublishes[pending.owner] = true
			}
		}
	}
	operations := make(
		[]documentMetadataOperation,
		0,
		len(coordinator.retainedDetachIntents)+len(coordinator.batch.operations),
	)
	for _, operation := range coordinator.batch.operations {
		if coordinator.pendingHandoffs[operation.owner] != nil {
			switch operation.kind {
			case documentMetadataRelease:
				continue
			case documentMetadataPublish:
				if !allowedPendingPublishes[operation.owner] {
					continue
				}
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
		if handoff.boundary != nil {
			operations = append(operations, documentMetadataOperation{
				kind:     documentMetadataBoundaryRecovered,
				boundary: handoff.boundary,
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
	delegatedBoundaries := make(map[*errorBoundaryState]*errorBoundaryState)
	for _, operation := range operations {
		if operation.kind == documentMetadataDelegateBoundary {
			delegatedBoundaries[operation.boundary] = operation.finalBoundary
		}
	}
	for index := range operations {
		operations[index].boundary = resolveDocumentMetadataBoundary(
			operations[index].boundary,
			delegatedBoundaries,
		)
		operations[index].finalBoundary = resolveDocumentMetadataBoundary(
			operations[index].finalBoundary,
			delegatedBoundaries,
		)
	}
	currentBoundaryOperations := make(
		map[*errorBoundaryState]documentMetadataOperationKind,
	)
	for _, operation := range operations {
		switch operation.kind {
		case documentMetadataBoundaryCommitted,
			documentMetadataBoundaryRecovered,
			documentMetadataBoundaryFailed,
			documentMetadataAbandonBoundary:
			currentBoundaryOperations[operation.boundary] = operation.kind
		}
	}
	for _, finalization := range coordinator.pendingFinalizationOrder {
		if currentBoundaryOperations[finalization.boundary] != 0 {
			continue
		}
		operations = append(operations, documentMetadataOperation{
			kind:     finalization.kind,
			boundary: finalization.boundary,
		})
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
				panic("goframe: document metadata owner cannot publish after release")
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
				panic("goframe: document metadata owner was released more than once")
			}
			index, ok := indices[operation.owner]
			if !ok {
				panic("goframe: document metadata owner is not active")
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
			panic("goframe: invalid document metadata operation")
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
			failedSuccessor := coordinator.retainFailedHandoff(
				operations,
				owners,
				active,
			)
			handoffBoundaries := make(map[*errorBoundaryState]bool)
			for handoff := range resolvedHandoffs {
				if handoff.boundary != nil {
					handoffBoundaries[handoff.boundary] = true
				}
			}
			if failedSuccessor != nil && failedSuccessor.boundary != nil {
				handoffBoundaries[failedSuccessor.boundary] = true
			}
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
	var handoff *documentMetadataPendingHandoff
	for _, pending := range pendingOwners {
		if existing := coordinator.pendingHandoffs[pending.owner]; existing != nil {
			handoff = existing
			break
		}
	}
	if handoff == nil {
		selected := coordinator.selectedOwner()
		for _, existing := range coordinator.pendingHandoffOrder {
			if existing.releaseSet[selected] {
				handoff = existing
				break
			}
		}
	}
	if len(outgoing) == 0 && handoff == nil {
		return nil
	}
	if coordinator.pendingHandoffs == nil {
		coordinator.pendingHandoffs = make(
			map[*documentMetadataOwner]*documentMetadataPendingHandoff,
		)
	}
	if handoff == nil {
		handoff = &documentMetadataPendingHandoff{
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
			pending = &documentMetadataPendingOwner{owner: operation.owner}
			handoff.ownerSet[operation.owner] = pending
			handoff.owners = append(handoff.owners, pending)
			coordinator.pendingHandoffs[operation.owner] = handoff
		}
		pending.boundary = operation.boundary
	}
	for _, owner := range outgoing {
		if handoff.releaseSet[owner] {
			continue
		}
		handoff.releaseSet[owner] = true
		handoff.releases = append(handoff.releases, owner)
		coordinator.removeRetainedDetachIntent(owner)
	}
	if len(handoff.owners) != 0 {
		handoff.boundary = handoff.owners[len(handoff.owners)-1].boundary
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
	if len(handoff.owners) == 0 {
		handoff.boundary = nil
		return
	}
	handoff.boundary = handoff.owners[len(handoff.owners)-1].boundary
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
		panic("goframe: invalid document metadata boundary finalization")
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
		panic("goframe: document metadata coordinator is nil")
	}
	if owner == nil {
		panic("goframe: document metadata owner is nil")
	}
	if owner.coordinator != coordinator {
		panic("goframe: document metadata owner belongs to another coordinator")
	}
}

func (coordinator *documentMetadataCoordinator) requireActiveUpdate() {
	if coordinator == nil || !coordinator.batch.active {
		panic("goframe: document metadata operation requires an active update")
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
		panic("goframe: document metadata owner is nil")
	}
	if owner.state == documentMetadataOwnerReleased {
		panic("goframe: document metadata owner is already released")
	}
	if owner.pending.attempt != nil {
		panic("goframe: document metadata owner already participated in this render attempt")
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
	panic("goframe: document metadata boundary owner is unavailable")
}

var activeDocumentMetadataCoordinator *documentMetadataCoordinator

func installDocumentMetadataCoordinator(coordinator *documentMetadataCoordinator) {
	if coordinator == nil {
		panic("goframe: document metadata coordinator is nil")
	}
	if activeDocumentMetadataCoordinator != nil {
		panic("goframe: document metadata coordinator is already installed")
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
		panic("goframe: cannot uninstall document metadata coordinator during an update")
	}
	reportProtectedSubtreeLifecycleOutcome = nil
	reportProtectedSubtreeLifecycleAbandon = nil
	activeDocumentMetadataCoordinator = nil
}

func useDocumentMetadata(metadata documentMetadataValue) {
	coordinator := activeDocumentMetadataCoordinator
	if coordinator == nil {
		panic("goframe: document metadata coordinator is not installed")
	}
	state := useStateSlot[*documentMetadataOwner](nil, "document metadata")
	owner := state.get()
	if owner == nil {
		owner = coordinator.newOwner()
		state.slot.value = owner
	}
	if owner.coordinator != coordinator {
		panic("goframe: document metadata owner belongs to another coordinator")
	}
	owner.prepareRender(requireLifecycleRenderAttempt(currentComponent), metadata)
	UseUnmount(func() {
		owner.coordinator.stageRelease(owner)
	})
}

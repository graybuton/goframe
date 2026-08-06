//go:build !js || !wasm || goframe_document_state_experiment

package goframe

type documentMetadataValue struct {
	title       string
	description string
}

type documentMetadataOwnerState uint8

const (
	documentMetadataOwnerPending documentMetadataOwnerState = iota
	documentMetadataOwnerActive
	documentMetadataOwnerReleased
)

type documentMetadataOwner struct {
	coordinator   *documentMetadataCoordinator
	id            uint64
	state         documentMetadataOwnerState
	boundary      *errorBoundaryState
	boundaryOwner *componentInstance
	pending       documentMetadataRenderState
}

type documentMetadataRenderState struct {
	attempt       *renderLifecycleAttempt
	metadata      documentMetadataValue
	boundary      *errorBoundaryState
	boundaryOwner *componentInstance
}

type documentMetadataOwnerRecord struct {
	owner    *documentMetadataOwner
	metadata documentMetadataValue
}

type documentMetadataOperationKind uint8

const (
	documentMetadataPublish documentMetadataOperationKind = iota + 1
	documentMetadataRelease
	documentMetadataFailedPublish
	documentMetadataAbandonBoundary
)

type documentMetadataOperation struct {
	kind          documentMetadataOperationKind
	owner         *documentMetadataOwner
	metadata      documentMetadataValue
	boundary      *errorBoundaryState
	boundaryOwner *componentInstance
}

type documentMetadataBatch struct {
	active     bool
	id         uint64
	operations []documentMetadataOperation
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
	baseline             documentMetadataValue
	current              documentMetadataValue
	nextID               uint64
	owners               []documentMetadataOwnerRecord
	failedBoundaries     map[*errorBoundaryState]bool
	retainedReleases     map[*errorBoundaryState]map[*documentMetadataOwner]bool
	registeredBoundaries map[*errorBoundaryState]bool
	batch                documentMetadataBatch
	apply                func(documentMetadataValue)
	observe              func(documentMetadataEvent)
	statistics           documentMetadataStatistics
}

func newDocumentMetadataCoordinator(
	baseline documentMetadataValue,
	apply func(documentMetadataValue),
	observe func(documentMetadataEvent),
) *documentMetadataCoordinator {
	if apply == nil {
		panic("goframe: document metadata coordinator requires a publication callback")
	}
	return &documentMetadataCoordinator{
		baseline: baseline,
		current:  baseline,
		apply:    apply,
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
	coordinator.statistics.updateBatches++
	coordinator.report("update-begin", nil, coordinator.current, len(coordinator.owners))
}

func (coordinator *documentMetadataCoordinator) stagePublish(
	owner *documentMetadataOwner,
	metadata documentMetadataValue,
) {
	coordinator.stagePublishForBoundary(owner, metadata, nil, nil)
}

func (coordinator *documentMetadataCoordinator) stagePublishForBoundary(
	owner *documentMetadataOwner,
	metadata documentMetadataValue,
	boundary *errorBoundaryState,
	boundaryOwner *componentInstance,
) {
	coordinator.validateOwner(owner)
	coordinator.requireActiveUpdate()
	if owner.state == documentMetadataOwnerReleased {
		panic("goframe: document metadata owner is already released")
	}
	if owner.id != 0 &&
		(owner.boundary != boundary || owner.boundaryOwner != boundaryOwner) {
		panic("goframe: document metadata owner changed error boundary")
	}
	coordinator.batch.operations = append(
		coordinator.batch.operations,
		documentMetadataOperation{
			kind:          documentMetadataPublish,
			owner:         owner,
			metadata:      metadata,
			boundary:      boundary,
			boundaryOwner: boundaryOwner,
		},
	)
	coordinator.report("publish-staged", owner, metadata, len(coordinator.owners))
}

func (coordinator *documentMetadataCoordinator) stageFailedPublish(
	owner *documentMetadataOwner,
	metadata documentMetadataValue,
	boundary *errorBoundaryState,
	boundaryOwner *componentInstance,
) {
	coordinator.validateOwner(owner)
	coordinator.requireActiveUpdate()
	coordinator.batch.operations = append(
		coordinator.batch.operations,
		documentMetadataOperation{
			kind:          documentMetadataFailedPublish,
			owner:         owner,
			metadata:      metadata,
			boundary:      boundary,
			boundaryOwner: boundaryOwner,
		},
	)
	coordinator.report("publish-rolled-back", owner, metadata, len(coordinator.owners))
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
	for _, operation := range coordinator.batch.operations {
		switch operation.kind {
		case documentMetadataFailedPublish:
			if operation.boundary != nil {
				failedBoundaries[operation.boundary] = true
			}
		case documentMetadataAbandonBoundary:
			abandonedBoundaries[operation.boundary] = true
		}
	}

	released := make(map[*documentMetadataOwner]bool)
	consumedRetained := make(map[*documentMetadataOwner]bool)
	for boundary := range abandonedBoundaries {
		delete(failedBoundaries, boundary)
		for owner := range retainedReleases[boundary] {
			var removed bool
			owners, removed = removeDocumentMetadataOwner(owners, indices, owner)
			if removed {
				released[owner] = true
			}
		}
		delete(retainedReleases, boundary)
	}

	duplicatePublications := 0
	boundaries := make(map[*documentMetadataOwner]documentMetadataOperation)

	for _, operation := range coordinator.batch.operations {
		switch operation.kind {
		case documentMetadataPublish:
			if released[operation.owner] {
				panic("goframe: document metadata owner cannot publish after release")
			}
			if operation.boundary != nil {
				for retained := range retainedReleases[operation.boundary] {
					if retained == operation.owner {
						continue
					}
					var removed bool
					owners, removed = removeDocumentMetadataOwner(owners, indices, retained)
					if removed {
						released[retained] = true
						consumedRetained[retained] = true
					}
				}
				delete(retainedReleases, operation.boundary)
				delete(failedBoundaries, operation.boundary)
			}
			boundaries[operation.owner] = operation
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
				!abandonedBoundaries[boundary] {
				if retainedReleases[boundary] == nil {
					retainedReleases[boundary] = make(map[*documentMetadataOwner]bool)
				}
				retainedReleases[boundary][operation.owner] = true
				released[operation.owner] = true
				continue
			}
			owners = removeDocumentMetadataOwnerAt(owners, indices, index)
			released[operation.owner] = true
		case documentMetadataFailedPublish, documentMetadataAbandonBoundary:
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
		coordinator.apply(next)
	}

	for _, record := range coordinator.owners {
		if !active[record.owner] {
			record.owner.state = documentMetadataOwnerReleased
		}
	}
	for owner := range released {
		if !active[owner] {
			owner.state = documentMetadataOwnerReleased
		}
	}
	for _, record := range owners {
		if id := assignments[record.owner]; id != 0 {
			record.owner.id = id
			coordinator.report("owner-committed", record.owner, record.metadata, len(owners))
		}
		if operation, ok := boundaries[record.owner]; ok {
			record.owner.boundary = operation.boundary
			record.owner.boundaryOwner = operation.boundaryOwner
		}
		record.owner.state = documentMetadataOwnerActive
	}

	previousCurrent := coordinator.current
	coordinator.owners = owners
	coordinator.nextID = nextID
	coordinator.current = next
	coordinator.failedBoundaries = failedBoundaries
	coordinator.retainedReleases = retainedReleases
	for boundary := range abandonedBoundaries {
		delete(coordinator.registeredBoundaries, boundary)
	}
	for _, record := range owners {
		coordinator.registerBoundaryCleanup(record.owner)
	}
	for _, operation := range coordinator.batch.operations {
		if operation.kind == documentMetadataFailedPublish &&
			!abandonedBoundaries[operation.boundary] {
			coordinator.registerBoundaryCleanupFor(
				operation.boundary,
				operation.boundaryOwner,
			)
		}
	}
	coordinator.statistics.committedIDAssignments += len(assignments)
	coordinator.statistics.activeAdditions += additions
	coordinator.statistics.updates += updates
	coordinator.statistics.releases += releases
	coordinator.statistics.duplicatePublications += duplicatePublications
	if publish {
		coordinator.statistics.documentPublications++
		if previousCurrent != coordinator.baseline && next == coordinator.baseline {
			coordinator.statistics.baselineRestorations++
		}
		coordinator.report("document-published", coordinator.selectedOwner(), next, len(owners))
	}
	coordinator.report("update-commit", coordinator.selectedOwner(), next, len(owners))
	coordinator.finishUpdate()
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

func (coordinator *documentMetadataCoordinator) registerBoundaryCleanup(
	owner *documentMetadataOwner,
) {
	coordinator.registerBoundaryCleanupFor(owner.boundary, owner.boundaryOwner)
}

func (coordinator *documentMetadataCoordinator) registerBoundaryCleanupFor(
	boundary *errorBoundaryState,
	boundaryOwner *componentInstance,
) {
	if boundary == nil || boundaryOwner == nil ||
		coordinator.registeredBoundaries[boundary] {
		return
	}
	if coordinator.registeredBoundaries == nil {
		coordinator.registeredBoundaries = make(map[*errorBoundaryState]bool)
	}
	coordinator.registeredBoundaries[boundary] = true
	boundaryOwner.unmountSlots = append(boundaryOwner.unmountSlots, func() {
		coordinator.stageBoundaryAbandon(boundary)
	})
}

func (coordinator *documentMetadataCoordinator) rollbackUpdate() {
	coordinator.requireActiveUpdate()
	coordinator.statistics.rollbacks++
	coordinator.report("update-rollback", coordinator.selectedOwner(), coordinator.current, len(coordinator.owners))
	coordinator.finishUpdate()
}

func (coordinator *documentMetadataCoordinator) finishUpdate() {
	clear(coordinator.batch.operations)
	coordinator.batch.operations = coordinator.batch.operations[:0]
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
	count := 0
	for _, owners := range coordinator.retainedReleases {
		count += len(owners)
	}
	return count
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
	coordinator.observe(documentMetadataEvent{
		kind:       kind,
		batchID:    coordinator.batch.id,
		ownerID:    ownerID,
		ownerCount: ownerCount,
		metadata:   metadata,
	})
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
	boundary, boundaryOwner := currentDocumentMetadataBoundary()
	owner.pending = documentMetadataRenderState{
		attempt:       attempt,
		metadata:      metadata,
		boundary:      boundary,
		boundaryOwner: boundaryOwner,
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
			pending.boundaryOwner,
		)
		return
	}
	owner.coordinator.stageFailedPublish(
		owner,
		pending.metadata,
		pending.boundary,
		pending.boundaryOwner,
	)
}

func currentDocumentMetadataBoundary() (
	*errorBoundaryState,
	*componentInstance,
) {
	boundary := currentProtectedLifecycleBoundary
	if boundary == nil {
		return nil, nil
	}
	for instance := currentComponent; instance != nil; instance = instance.parent {
		if instance.errorBoundary == boundary {
			return boundary, instance
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
}

func uninstallDocumentMetadataCoordinator() {
	if activeDocumentMetadataCoordinator != nil && activeDocumentMetadataCoordinator.batch.active {
		panic("goframe: cannot uninstall document metadata coordinator during an update")
	}
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

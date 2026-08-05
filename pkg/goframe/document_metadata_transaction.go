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
	coordinator *documentMetadataCoordinator
	id          uint64
	state       documentMetadataOwnerState
	pending     documentMetadataRenderState
}

type documentMetadataRenderState struct {
	attempt  *renderLifecycleAttempt
	metadata documentMetadataValue
}

type documentMetadataOwnerRecord struct {
	owner    *documentMetadataOwner
	metadata documentMetadataValue
}

type documentMetadataOperationKind uint8

const (
	documentMetadataPublish documentMetadataOperationKind = iota + 1
	documentMetadataRelease
)

type documentMetadataOperation struct {
	kind     documentMetadataOperationKind
	owner    *documentMetadataOwner
	metadata documentMetadataValue
}

type documentMetadataBatch struct {
	active     bool
	id         uint64
	operations []documentMetadataOperation
}

type documentMetadataSnapshot struct {
	owner      *documentMetadataOwner
	metadata   documentMetadataValue
	ownerCount int
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
	baseline   documentMetadataValue
	current    documentMetadataValue
	nextID     uint64
	owners     []documentMetadataOwnerRecord
	batch      documentMetadataBatch
	apply      func(documentMetadataValue)
	observe    func(documentMetadataEvent)
	statistics documentMetadataStatistics
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
		},
	)
	coordinator.report("publish-staged", owner, metadata, len(coordinator.owners))
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
	released := make(map[*documentMetadataOwner]bool)
	duplicatePublications := 0

	for _, operation := range coordinator.batch.operations {
		switch operation.kind {
		case documentMetadataPublish:
			if released[operation.owner] {
				panic("goframe: document metadata owner cannot publish after release")
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
				panic("goframe: document metadata owner was released more than once")
			}
			index, ok := indices[operation.owner]
			if !ok {
				panic("goframe: document metadata owner is not active")
			}
			delete(indices, operation.owner)
			copy(owners[index:], owners[index+1:])
			owners = owners[:len(owners)-1]
			for next := index; next < len(owners); next++ {
				indices[owners[next].owner] = next
			}
			released[operation.owner] = true
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
		record.owner.state = documentMetadataOwnerActive
	}

	previousCurrent := coordinator.current
	coordinator.owners = owners
	coordinator.nextID = nextID
	coordinator.current = next
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
		owner:      coordinator.selectedOwner(),
		metadata:   coordinator.current,
		ownerCount: len(coordinator.owners),
	}
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
	owner.pending = documentMetadataRenderState{
		attempt:  attempt,
		metadata: metadata,
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
		owner.coordinator.stagePublish(owner, pending.metadata)
		return
	}
	owner.coordinator.report("publish-rolled-back", owner, pending.metadata, len(owner.coordinator.owners))
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

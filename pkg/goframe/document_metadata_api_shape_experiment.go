//go:build goframe_document_state_experiment

package goframe

// DocumentMetadata is fixture-only input for the private API-shape comparison.
// It is excluded from ordinary builds and is not a compatibility candidate.
type DocumentMetadata struct {
	Title       string
	Description string
}

// DocumentMetadataComponentProps is fixture-only input for the non-DOM
// component projection. Go requires the component function and metadata value
// type to have different identifiers, so the executable GOX spelling is
// DocumentMetadataComponent rather than DocumentMetadata.
type DocumentMetadataComponentProps struct {
	Metadata DocumentMetadata
	Children []Node
}

// UseDocumentMetadata is the fixture-only implicit-hook projection. One call
// owns one stable metadata lifetime at that hook slot.
func UseDocumentMetadata(metadata DocumentMetadata) {
	owner := useDocumentMetadataAPIShapeOwner("UseDocumentMetadata")
	owner.prepareRender(
		requireLifecycleRenderAttempt(currentComponent),
		documentMetadataFromAPIShape(metadata),
	)
	UseUnmount(func() {
		owner.owner.coordinator.stageRelease(owner.owner)
	})
}

// DocumentMetadataComponent is the fixture-only non-DOM component projection.
// It preserves its children in a fragment and emits no host element.
func DocumentMetadataComponent(props DocumentMetadataComponentProps) Node {
	UseDocumentMetadata(props.Metadata)
	return Fragment(props.Children...)
}

// DocumentMetadataOwner is a fixture-only explicit owner handle. It is absent
// from ordinary builds and is not a public API or compatibility candidate.
type DocumentMetadataOwner struct {
	owner        *documentMetadataAPIShapeOwner
	publications []*documentMetadataAPIShapePublication
	primary      *documentMetadataAPIShapePublication
	metadata     documentMetadataValue
	renderBatch  uint64
	renderValues map[*documentMetadataAPIShapePublication]documentMetadataValue
	failedBatch  uint64
}

const documentMetadataAPIShapePublicationConflict = "document metadata API shape experiment: one owner handle has conflicting active publications"

// ID returns the committed fixture-only owner identity.
func (owner *DocumentMetadataOwner) ID() uint64 {
	if owner == nil || owner.owner == nil || owner.owner.owner == nil {
		return 0
	}
	return owner.owner.owner.id
}

// ActivePublications returns the fixture-only active publication count.
func (owner *DocumentMetadataOwner) ActivePublications() int {
	if owner == nil {
		return 0
	}
	return len(owner.publications)
}

// UseDocumentMetadataOwner is the fixture-only explicit-handle projection.
// The returned identity is stable at this hook slot.
func UseDocumentMetadataOwner() *DocumentMetadataOwner {
	state := useStateSlot[*DocumentMetadataOwner](nil, "UseDocumentMetadataOwner")
	owner := state.get()
	if owner == nil {
		owner = &DocumentMetadataOwner{
			owner: newDocumentMetadataAPIShapeOwner(),
		}
		state.slot.value = owner
	}
	owner.validate()
	return owner
}

// UseOwnedDocumentMetadata is the fixture-only owned-publication projection.
// Multiple calls using one handle must publish an identical pair while they
// are simultaneously active.
func UseOwnedDocumentMetadata(
	owner *DocumentMetadataOwner,
	metadata DocumentMetadata,
) {
	if owner == nil {
		panic("document metadata API shape experiment: owner handle is nil")
	}
	owner.validate()
	state := useStateSlot[*documentMetadataAPIShapePublication](
		nil,
		"UseOwnedDocumentMetadata",
	)
	publication := state.get()
	if publication == nil {
		publication = &documentMetadataAPIShapePublication{owner: owner}
		state.slot.value = publication
	}
	if publication.owner != owner {
		panic("document metadata API shape experiment: publication changed owner handles")
	}
	publication.prepareRender(
		requireLifecycleRenderAttempt(currentComponent),
		documentMetadataFromAPIShape(metadata),
	)
	UseUnmount(func() {
		owner.releasePublication(publication)
	})
}

type documentMetadataAPIShapeOwner struct {
	owner          *documentMetadataOwner
	tokenCommitted bool
	pending        documentMetadataRenderState
}

func newDocumentMetadataAPIShapeOwner() *documentMetadataAPIShapeOwner {
	coordinator := activeDocumentMetadataCoordinator
	if coordinator == nil {
		panic("goframe: document metadata coordinator is not installed")
	}
	coordinator.requireActiveUpdate()
	return &documentMetadataAPIShapeOwner{
		owner: &documentMetadataOwner{coordinator: coordinator},
	}
}

func useDocumentMetadataAPIShapeOwner(
	hookName string,
) *documentMetadataAPIShapeOwner {
	state := useStateSlot[*documentMetadataAPIShapeOwner](nil, hookName)
	owner := state.get()
	if owner == nil {
		owner = newDocumentMetadataAPIShapeOwner()
		state.slot.value = owner
	}
	owner.validate()
	return owner
}

func (owner *documentMetadataAPIShapeOwner) validate() {
	coordinator := activeDocumentMetadataCoordinator
	if owner == nil || owner.owner == nil || owner.owner.coordinator == nil {
		panic("goframe: document metadata owner is nil")
	}
	if coordinator == nil {
		panic("goframe: document metadata coordinator is not installed")
	}
	if owner.owner.coordinator != coordinator {
		panic("goframe: document metadata owner belongs to another coordinator")
	}
	if owner.owner.state == documentMetadataOwnerReleased {
		panic("goframe: document metadata owner is already released")
	}
}

func (owner *documentMetadataAPIShapeOwner) prepareRender(
	attempt *renderLifecycleAttempt,
	metadata documentMetadataValue,
) {
	owner.validate()
	if owner.pending.attempt != nil {
		panic("goframe: document metadata owner already participated in this render attempt")
	}
	owner.pending = documentMetadataRenderState{
		attempt:  attempt,
		metadata: metadata,
		boundary: currentDocumentMetadataBoundary(),
	}
	attempt.participants = append(attempt.participants, owner)
}

func (owner *documentMetadataAPIShapeOwner) finishRender(
	attempt *renderLifecycleAttempt,
	commit bool,
) {
	pending := owner.pending
	if pending.attempt != attempt {
		return
	}
	owner.pending = documentMetadataRenderState{}
	if !commit {
		owner.owner.coordinator.stageFailedPublish(
			owner.owner,
			pending.metadata,
			pending.boundary,
		)
		return
	}
	owner.commitToken()
	owner.owner.coordinator.stagePublishForBoundary(
		owner.owner,
		pending.metadata,
		pending.boundary,
	)
}

func (owner *documentMetadataAPIShapeOwner) commitToken() {
	if owner.tokenCommitted {
		return
	}
	owner.tokenCommitted = true
	coordinator := owner.owner.coordinator
	coordinator.statistics.tokenCreations++
	coordinator.report(
		"owner-created",
		owner.owner,
		documentMetadataValue{},
		len(coordinator.owners),
	)
}

type documentMetadataAPIShapePublication struct {
	owner    *DocumentMetadataOwner
	active   bool
	metadata documentMetadataValue
	pending  documentMetadataRenderState
}

func (publication *documentMetadataAPIShapePublication) prepareRender(
	attempt *renderLifecycleAttempt,
	metadata documentMetadataValue,
) {
	if publication == nil || publication.owner == nil {
		panic("document metadata API shape experiment: publication is nil")
	}
	if publication.pending.attempt != nil {
		panic("document metadata API shape experiment: publication participated twice in one render")
	}
	publication.owner.validatePublication(publication, metadata)
	publication.pending = documentMetadataRenderState{
		attempt:  attempt,
		metadata: metadata,
		boundary: currentDocumentMetadataBoundary(),
	}
	attempt.participants = append(attempt.participants, publication)
}

func (publication *documentMetadataAPIShapePublication) finishRender(
	attempt *renderLifecycleAttempt,
	commit bool,
) {
	pending := publication.pending
	if pending.attempt != attempt {
		return
	}
	publication.pending = documentMetadataRenderState{}
	owner := publication.owner
	if !commit {
		owner.stageFailedPublication(pending)
		return
	}
	owner.commitPublication(publication, pending)
}

func (owner *DocumentMetadataOwner) validate() {
	if owner == nil || owner.owner == nil {
		panic("document metadata API shape experiment: owner handle is nil")
	}
	owner.owner.validate()
}

func (owner *DocumentMetadataOwner) validatePublication(
	publication *documentMetadataAPIShapePublication,
	metadata documentMetadataValue,
) {
	owner.validate()
	coordinator := owner.owner.owner.coordinator
	coordinator.requireActiveUpdate()
	if owner.renderBatch != coordinator.batch.id {
		owner.renderBatch = coordinator.batch.id
		owner.renderValues = make(
			map[*documentMetadataAPIShapePublication]documentMetadataValue,
		)
	}
	owner.renderValues[publication] = metadata
	if publication.active && publication.metadata != metadata &&
		owner.primary != publication {
		panic(documentMetadataAPIShapePublicationConflict)
	}

	var expected documentMetadataValue
	hasExpected := false
	check := func(value documentMetadataValue) {
		if !hasExpected {
			expected = value
			hasExpected = true
			return
		}
		if expected != value {
			panic(documentMetadataAPIShapePublicationConflict)
		}
	}
	seen := make(map[*documentMetadataAPIShapePublication]bool)
	for _, active := range owner.publications {
		value := active.metadata
		if rendered, ok := owner.renderValues[active]; ok {
			value = rendered
		}
		check(value)
		seen[active] = true
	}
	for rendered, value := range owner.renderValues {
		if seen[rendered] {
			continue
		}
		check(value)
	}
}

func (owner *DocumentMetadataOwner) stageFailedPublication(
	pending documentMetadataRenderState,
) {
	coordinator := owner.owner.owner.coordinator
	if owner.failedBatch == coordinator.batch.id {
		return
	}
	owner.failedBatch = coordinator.batch.id
	coordinator.stageFailedPublish(
		owner.owner.owner,
		pending.metadata,
		pending.boundary,
	)
}

func (owner *DocumentMetadataOwner) commitPublication(
	publication *documentMetadataAPIShapePublication,
	pending documentMetadataRenderState,
) {
	previousCount := len(owner.publications)
	previousMetadata := publication.metadata
	if !publication.active {
		publication.active = true
		publication.metadata = pending.metadata
		owner.publications = append(owner.publications, publication)
		if previousCount == 0 {
			owner.primary = publication
			owner.metadata = pending.metadata
			owner.owner.commitToken()
			owner.owner.owner.coordinator.stagePublishForBoundary(
				owner.owner.owner,
				pending.metadata,
				pending.boundary,
			)
		}
		return
	}

	publication.metadata = pending.metadata
	if previousMetadata == pending.metadata {
		if owner.primary == publication {
			owner.owner.owner.coordinator.stagePublishForBoundary(
				owner.owner.owner,
				pending.metadata,
				pending.boundary,
			)
		}
		return
	}
	owner.metadata = pending.metadata
	owner.owner.owner.coordinator.stagePublishForBoundary(
		owner.owner.owner,
		pending.metadata,
		pending.boundary,
	)
}

func (owner *DocumentMetadataOwner) releasePublication(
	publication *documentMetadataAPIShapePublication,
) {
	owner.validate()
	if publication == nil || publication.owner != owner || !publication.active {
		return
	}
	publication.active = false
	for index, active := range owner.publications {
		if active != publication {
			continue
		}
		copy(owner.publications[index:], owner.publications[index+1:])
		last := len(owner.publications) - 1
		owner.publications[last] = nil
		owner.publications = owner.publications[:last]
		break
	}
	if owner.primary == publication {
		owner.primary = nil
	}
	if len(owner.publications) != 0 {
		return
	}
	owner.owner.owner.coordinator.stageRelease(owner.owner.owner)
}

func documentMetadataFromAPIShape(
	metadata DocumentMetadata,
) documentMetadataValue {
	return documentMetadataValue{
		title:       metadata.Title,
		description: metadata.Description,
	}
}

package main

import (
	"errors"

	gf "github.com/graybuton/goframe/pkg/goframe"
	"github.com/graybuton/goframe/scripts/fixtures/document-state-api-design/internal/documentmeta"
)

type candidateOwnerProps struct {
	Mode        string
	Role        string
	Metadata    documentmeta.Metadata
	Control     *documentmeta.StringOwners
	OnSnapshot  func(documentmeta.Snapshot)
	Children    []gf.Node
	Failure     bool
	Duplicate   bool
	RenderNonce int
}

type componentDocumentMetadataProps struct {
	Role        string
	Metadata    documentmeta.Metadata
	Children    []gf.Node
	Failure     bool
	RenderNonce int
}

type documentMetadataOwnerHandle struct {
	owner              *documentmeta.Owner
	activePublications int
	metadata           documentmeta.Metadata
	primary            *documentMetadataPublication
}

type documentMetadataPublication struct {
	active   bool
	metadata documentmeta.Metadata
}

var (
	errDocumentMetadataPublicationConflict = errors.New(
		"document-state API design: one owner handle has conflicting active publications",
	)
	errDocumentMetadataPublicationUnderflow = errors.New(
		"document-state API design: document owner handle publication underflow",
	)
)

var (
	candidateOwnerType = gf.NewComponentType(
		"fixture.document-state-api-design.CandidateOwner",
		"DocumentAPIDesignCandidateOwner",
	)
	componentDocumentMetadataType = gf.NewComponentType(
		"fixture.document-state-api-design.ComponentDocumentMetadata",
		"DocumentAPIDesignComponentOwner",
	)
)

func candidateOwnerNode(props candidateOwnerProps) gf.Node {
	return gf.Key(
		"candidate-owner:"+props.Role,
		gf.ComponentT(candidateOwnerType, props, CandidateOwner),
	)
}

func CandidateOwner(props candidateOwnerProps) gf.Node {
	switch props.Mode {
	case "control":
		useControlDocumentMetadata(props)
	case "hook":
		useHookDocumentMetadata(props.Role, props.Metadata)
	case "component":
		return gf.ComponentT(
			componentDocumentMetadataType,
			componentDocumentMetadataProps{
				Role:        props.Role,
				Metadata:    props.Metadata,
				Children:    props.Children,
				Failure:     props.Failure,
				RenderNonce: props.RenderNonce,
			},
			ComponentDocumentMetadata,
		)
	case "handle":
		handle := useDocumentMetadataOwnerHandle(props.Role)
		if props.Duplicate {
			useOwnedDocumentMetadataThroughHelper(handle, props.Role, props.Metadata)
		}
		useOwnedDocumentMetadata(handle, props.Role, props.Metadata)
	default:
		panic("document-state API design: unsupported candidate mode")
	}
	if props.Failure {
		panic("document-state API design speculative render failure")
	}
	recordCandidateRender(props.Mode, props.Role, props.RenderNonce)
	return gf.Fragment(props.Children...)
}

func useControlDocumentMetadata(props candidateOwnerProps) {
	if props.Control == nil || props.OnSnapshot == nil {
		panic("document-state API design: control bindings are missing")
	}
	key := props.Role
	metadata := props.Metadata
	gf.UseEffect(func() gf.Cleanup {
		transition, err := props.Control.Publish(key, metadata)
		if err != nil {
			panic("document-state API design control publish: " + err.Error())
		}
		publishCandidateTransition(
			"control",
			key,
			transition,
			props.Control.Stats(),
			props.OnSnapshot,
		)
		return nil
	}, gf.Deps(key, metadata.Title, metadata.Description))
	gf.UseUnmount(func() {
		transition, err := props.Control.Release(key)
		if err != nil {
			panic("document-state API design control release: " + err.Error())
		}
		publishCandidateTransition(
			"control",
			key,
			transition,
			props.Control.Stats(),
			props.OnSnapshot,
		)
	})
}

func useHookDocumentMetadata(role string, metadata documentmeta.Metadata) {
	bindings := requireDocumentBindings()
	owner, setOwner := gf.UseState[*documentmeta.Owner](nil)
	recordCandidateOwnerRender("hook", role, owner.ID())

	gf.UseEffect(func() gf.Cleanup {
		setOwner(bindings.coordinator.NewOwner())
		recordCoordinatorStatistics(bindings.coordinator.Stats())
		return nil
	}, gf.Once())
	gf.UseEffect(func() gf.Cleanup {
		if owner == nil {
			return nil
		}
		transition, err := bindings.coordinator.Publish(owner, metadata)
		if err != nil {
			panic("document-state API design hook publish: " + err.Error())
		}
		publishCandidateTransition(
			"hook",
			role,
			transition,
			bindings.coordinator.Stats(),
			bindings.onSnapshot,
		)
		return nil
	}, gf.Deps(owner != nil, metadata.Title, metadata.Description))
	gf.UseUnmount(func() {
		if owner == nil {
			return
		}
		transition, err := bindings.coordinator.Release(owner)
		if err != nil {
			panic("document-state API design hook release: " + err.Error())
		}
		publishCandidateTransition(
			"hook",
			role,
			transition,
			bindings.coordinator.Stats(),
			bindings.onSnapshot,
		)
	})
}

func ComponentDocumentMetadata(props componentDocumentMetadataProps) gf.Node {
	bindings := requireDocumentBindings()
	owner, setOwner := gf.UseState[*documentmeta.Owner](nil)
	recordCandidateOwnerRender("component", props.Role, owner.ID())
	metadata := props.Metadata

	gf.UseEffect(func() gf.Cleanup {
		setOwner(bindings.coordinator.NewOwner())
		recordCoordinatorStatistics(bindings.coordinator.Stats())
		return nil
	}, gf.Once())
	gf.UseEffect(func() gf.Cleanup {
		if owner == nil {
			return nil
		}
		transition, err := bindings.coordinator.Publish(owner, metadata)
		if err != nil {
			panic("document-state API design component publish: " + err.Error())
		}
		if transition.Change == documentmeta.ChangeAdded {
			recordComponentOwnerMount(props.Role, owner.ID())
		}
		publishCandidateTransition(
			"component",
			props.Role,
			transition,
			bindings.coordinator.Stats(),
			bindings.onSnapshot,
		)
		return nil
	}, gf.Deps(owner != nil, metadata.Title, metadata.Description))
	gf.UseUnmount(func() {
		if owner == nil || owner.ID() == 0 {
			return
		}
		recordComponentOwnerUnmount(props.Role, owner.ID())
		transition, err := bindings.coordinator.Release(owner)
		if err != nil {
			panic("document-state API design component release: " + err.Error())
		}
		publishCandidateTransition(
			"component",
			props.Role,
			transition,
			bindings.coordinator.Stats(),
			bindings.onSnapshot,
		)
	})
	if props.Failure {
		panic("document-state API design speculative render failure")
	}
	recordCandidateRender("component", props.Role, props.RenderNonce)
	return gf.Fragment(props.Children...)
}

func useDocumentMetadataOwnerHandle(role string) *documentMetadataOwnerHandle {
	bindings := requireDocumentBindings()
	handle, setHandle := gf.UseState[*documentMetadataOwnerHandle](nil)
	gf.UseEffect(func() gf.Cleanup {
		next := &documentMetadataOwnerHandle{
			owner: bindings.coordinator.NewOwner(),
		}
		recordHandleCreation(role)
		recordCoordinatorStatistics(bindings.coordinator.Stats())
		setHandle(next)
		return nil
	}, gf.Once())
	recordCandidateOwnerRender("handle", role, documentMetadataHandleOwnerID(handle))
	return handle
}

func useOwnedDocumentMetadataThroughHelper(
	handle *documentMetadataOwnerHandle,
	role string,
	metadata documentmeta.Metadata,
) {
	useOwnedDocumentMetadataSlot(handle, role, metadata, true, true)
}

func useOwnedDocumentMetadata(
	handle *documentMetadataOwnerHandle,
	role string,
	metadata documentmeta.Metadata,
) {
	useOwnedDocumentMetadataSlot(handle, role, metadata, false, true)
}

func useOptionalOwnedDocumentMetadata(
	handle *documentMetadataOwnerHandle,
	role string,
	metadata documentmeta.Metadata,
	active bool,
) {
	useOwnedDocumentMetadataSlot(handle, role, metadata, false, active)
}

func useOwnedDocumentMetadataSlot(
	handle *documentMetadataOwnerHandle,
	role string,
	metadata documentmeta.Metadata,
	forwarded bool,
	active bool,
) {
	bindings := requireDocumentBindings()
	publication, setPublication := gf.UseState[*documentMetadataPublication](nil)

	gf.UseEffect(func() gf.Cleanup {
		recordPublicationCreation(role, forwarded)
		setPublication(&documentMetadataPublication{})
		return nil
	}, gf.Once())
	gf.UseEffect(func() gf.Cleanup {
		if handle == nil || handle.owner == nil || publication == nil {
			return nil
		}
		if !active {
			transition, err := releaseDocumentMetadataPublication(
				bindings.coordinator,
				handle,
				publication,
			)
			if err != nil {
				panic(err.Error())
			}
			recordHandlePublicationState(
				role,
				"release",
				handle.owner.ID(),
				handle.activePublications,
			)
			if transition.Change != documentmeta.ChangeNone {
				publishCandidateTransition(
					"handle",
					role,
					transition,
					bindings.coordinator.Stats(),
					bindings.onSnapshot,
				)
			}
			return nil
		}
		transition, coalesced, err := reconcileDocumentMetadataPublication(
			bindings.coordinator,
			handle,
			publication,
			metadata,
		)
		if err != nil {
			panic(err.Error())
		}
		action := transition.Change.String()
		if coalesced {
			action = "coalesced"
			recordHandleDuplicateCoalesced(handle.owner.ID())
		}
		recordHandlePublicationState(
			role,
			action,
			handle.owner.ID(),
			handle.activePublications,
		)
		if transition.Change != documentmeta.ChangeNone {
			publishCandidateTransition(
				"handle",
				role,
				transition,
				bindings.coordinator.Stats(),
				bindings.onSnapshot,
			)
		}
		if forwarded {
			recordHandleForward(handle.owner.ID())
		}
		return nil
	}, gf.Deps(
		handle != nil,
		publication != nil,
		active,
		metadata.Title,
		metadata.Description,
	))
	gf.UseUnmount(func() {
		if handle == nil || handle.owner == nil || publication == nil ||
			!publication.active {
			return
		}
		transition, err := releaseDocumentMetadataPublication(
			bindings.coordinator,
			handle,
			publication,
		)
		if err != nil {
			panic(err.Error())
		}
		recordHandlePublicationState(
			role,
			"unmount",
			handle.owner.ID(),
			handle.activePublications,
		)
		if transition.Change != documentmeta.ChangeNone {
			publishCandidateTransition(
				"handle",
				role,
				transition,
				bindings.coordinator.Stats(),
				bindings.onSnapshot,
			)
		}
	})
}

func reconcileDocumentMetadataPublication(
	coordinator *documentmeta.Coordinator,
	handle *documentMetadataOwnerHandle,
	publication *documentMetadataPublication,
	metadata documentmeta.Metadata,
) (documentmeta.Transition, bool, error) {
	if !publication.active {
		if handle.activePublications < 0 {
			return documentmeta.Transition{}, false,
				errDocumentMetadataPublicationUnderflow
		}
		if handle.activePublications == 0 {
			transition, err := coordinator.Publish(handle.owner, metadata)
			if err != nil {
				return documentmeta.Transition{}, false, errors.New(
					"document-state API design handle publish: " + err.Error(),
				)
			}
			publication.active = true
			publication.metadata = metadata
			handle.activePublications = 1
			handle.metadata = metadata
			handle.primary = publication
			return transition, false, nil
		}
		if handle.metadata != metadata {
			return documentmeta.Transition{}, false,
				errDocumentMetadataPublicationConflict
		}
		publication.active = true
		publication.metadata = metadata
		handle.activePublications++
		return documentmeta.Transition{}, true, nil
	}

	if publication.metadata == metadata && handle.metadata == metadata {
		return documentmeta.Transition{}, false, nil
	}
	if handle.primary != publication || handle.activePublications > 1 {
		return documentmeta.Transition{}, false,
			errDocumentMetadataPublicationConflict
	}
	if handle.activePublications <= 0 {
		return documentmeta.Transition{}, false,
			errDocumentMetadataPublicationUnderflow
	}

	transition, err := coordinator.Publish(handle.owner, metadata)
	if err != nil {
		return documentmeta.Transition{}, false, errors.New(
			"document-state API design handle update: " + err.Error(),
		)
	}
	publication.metadata = metadata
	handle.metadata = metadata
	return transition, false, nil
}

func releaseDocumentMetadataPublication(
	coordinator *documentmeta.Coordinator,
	handle *documentMetadataOwnerHandle,
	publication *documentMetadataPublication,
) (documentmeta.Transition, error) {
	if !publication.active {
		return documentmeta.Transition{}, nil
	}
	if handle.activePublications <= 0 {
		return documentmeta.Transition{},
			errDocumentMetadataPublicationUnderflow
	}
	if handle.activePublications > 1 {
		publication.active = false
		handle.activePublications--
		return documentmeta.Transition{}, nil
	}

	transition, err := coordinator.Release(handle.owner)
	if err != nil {
		return documentmeta.Transition{}, errors.New(
			"document-state API design handle release: " + err.Error(),
		)
	}
	publication.active = false
	handle.activePublications = 0
	handle.primary = nil
	return transition, nil
}

func documentMetadataHandleOwnerID(handle *documentMetadataOwnerHandle) uint64 {
	if handle == nil || handle.owner == nil {
		return 0
	}
	return handle.owner.ID()
}

func documentMetadataHandlePublicationCount(
	handle *documentMetadataOwnerHandle,
) int {
	if handle == nil {
		return 0
	}
	return handle.activePublications
}

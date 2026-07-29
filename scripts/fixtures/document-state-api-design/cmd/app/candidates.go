package main

import (
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
		publishCandidateTransition("control", key, transition, props.OnSnapshot)
		return nil
	}, gf.Deps(key, metadata.Title, metadata.Description))
	gf.UseUnmount(func() {
		transition, err := props.Control.Release(key)
		if err != nil {
			panic("document-state API design control release: " + err.Error())
		}
		publishCandidateTransition("control", key, transition, props.OnSnapshot)
	})
}

func useHookDocumentMetadata(role string, metadata documentmeta.Metadata) {
	bindings := requireDocumentBindings()
	owner, _ := gf.UseState(bindings.coordinator.NewOwner())
	recordCandidateOwnerRender("hook", role, owner.ID())

	gf.UseEffect(func() gf.Cleanup {
		transition, err := bindings.coordinator.Publish(owner, metadata)
		if err != nil {
			panic("document-state API design hook publish: " + err.Error())
		}
		publishCandidateTransition("hook", role, transition, bindings.onSnapshot)
		return nil
	}, gf.Deps(metadata.Title, metadata.Description))
	gf.UseUnmount(func() {
		transition, err := bindings.coordinator.Release(owner)
		if err != nil {
			panic("document-state API design hook release: " + err.Error())
		}
		publishCandidateTransition("hook", role, transition, bindings.onSnapshot)
	})
}

func ComponentDocumentMetadata(props componentDocumentMetadataProps) gf.Node {
	bindings := requireDocumentBindings()
	owner, _ := gf.UseState(bindings.coordinator.NewOwner())
	recordCandidateOwnerRender("component", props.Role, owner.ID())
	metadata := props.Metadata

	gf.UseEffect(func() gf.Cleanup {
		recordComponentOwnerMount(props.Role, owner.ID())
		transition, err := bindings.coordinator.Publish(owner, metadata)
		if err != nil {
			panic("document-state API design component publish: " + err.Error())
		}
		publishCandidateTransition("component", props.Role, transition, bindings.onSnapshot)
		return nil
	}, gf.Deps(metadata.Title, metadata.Description))
	gf.UseUnmount(func() {
		recordComponentOwnerUnmount(props.Role, owner.ID())
		transition, err := bindings.coordinator.Release(owner)
		if err != nil {
			panic("document-state API design component release: " + err.Error())
		}
		publishCandidateTransition("component", props.Role, transition, bindings.onSnapshot)
	})
	if props.Failure {
		panic("document-state API design speculative render failure")
	}
	recordCandidateRender("component", props.Role, props.RenderNonce)
	return gf.Fragment(props.Children...)
}

func useDocumentMetadataOwnerHandle(role string) *documentMetadataOwnerHandle {
	bindings := requireDocumentBindings()
	handle, _ := gf.UseState(&documentMetadataOwnerHandle{
		owner: bindings.coordinator.NewOwner(),
	})
	recordCandidateOwnerRender("handle", role, handle.owner.ID())
	return handle
}

func useOwnedDocumentMetadataThroughHelper(
	handle *documentMetadataOwnerHandle,
	role string,
	metadata documentmeta.Metadata,
) {
	recordHandleForward(handle.owner.ID())
	useOwnedDocumentMetadata(handle, role, metadata)
}

func useOwnedDocumentMetadata(
	handle *documentMetadataOwnerHandle,
	role string,
	metadata documentmeta.Metadata,
) {
	bindings := requireDocumentBindings()
	if handle == nil || handle.owner == nil {
		panic("document-state API design: document owner handle is nil")
	}
	publication, _ := gf.UseState(&documentMetadataPublication{})

	gf.UseEffect(func() gf.Cleanup {
		switch {
		case !publication.active && handle.activePublications == 0:
			publication.active = true
			publication.metadata = metadata
			handle.activePublications = 1
			handle.metadata = metadata
			handle.primary = publication
			transition, err := bindings.coordinator.Publish(handle.owner, metadata)
			if err != nil {
				panic("document-state API design handle publish: " + err.Error())
			}
			publishCandidateTransition("handle", role, transition, bindings.onSnapshot)
		case !publication.active && handle.metadata == metadata:
			publication.active = true
			publication.metadata = metadata
			handle.activePublications++
			recordHandleDuplicateCoalesced(handle.owner.ID())
		case publication.active && handle.primary == publication:
			publication.metadata = metadata
			handle.metadata = metadata
			transition, err := bindings.coordinator.Publish(handle.owner, metadata)
			if err != nil {
				panic("document-state API design handle update: " + err.Error())
			}
			publishCandidateTransition("handle", role, transition, bindings.onSnapshot)
		case publication.active && handle.metadata == metadata:
			publication.metadata = metadata
		default:
			panic("document-state API design: one owner handle has conflicting active publications")
		}
		return nil
	}, gf.Deps(metadata.Title, metadata.Description))
	gf.UseUnmount(func() {
		if !publication.active || handle.activePublications <= 0 {
			panic("document-state API design: document owner handle publication underflow")
		}
		publication.active = false
		handle.activePublications--
		if handle.activePublications != 0 {
			return
		}
		handle.primary = nil
		transition, err := bindings.coordinator.Release(handle.owner)
		if err != nil {
			panic("document-state API design handle release: " + err.Error())
		}
		publishCandidateTransition("handle", role, transition, bindings.onSnapshot)
	})
}

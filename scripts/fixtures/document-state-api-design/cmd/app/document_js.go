//go:build js && wasm

package main

import (
	"errors"
	"fmt"
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
	"github.com/graybuton/goframe/scripts/fixtures/document-state-api-design/internal/documentmeta"
)

type browserDocumentAdapter struct {
	title       js.Value
	description js.Value
	unrelated   js.Value
	baseline    documentmeta.Metadata
	current     documentmeta.Metadata
}

func newBrowserDocumentAdapter() (*browserDocumentAdapter, error) {
	document := js.Global().Get("document")
	if document.IsUndefined() || document.IsNull() {
		return nil, errors.New("document is unavailable")
	}
	titles := document.Call("querySelectorAll", "head title")
	if titles.Get("length").Int() != 1 {
		return nil, fmt.Errorf("expected one authored title, found %d", titles.Get("length").Int())
	}
	descriptions := document.Call("querySelectorAll", `head meta[name="description"]`)
	if descriptions.Get("length").Int() != 1 {
		return nil, fmt.Errorf(
			"expected one authored description, found %d",
			descriptions.Get("length").Int(),
		)
	}
	unrelated := document.Call("querySelector", `head meta[name="fixture-unrelated"]`)
	if unrelated.IsUndefined() || unrelated.IsNull() {
		return nil, errors.New("authored unrelated metadata is missing")
	}

	title := titles.Index(0)
	description := descriptions.Index(0)
	baseline := documentmeta.Metadata{
		Title:       title.Get("textContent").String(),
		Description: description.Call("getAttribute", "content").String(),
	}
	return &browserDocumentAdapter{
		title:       title,
		description: description,
		unrelated:   unrelated,
		baseline:    baseline,
		current:     baseline,
	}, nil
}

func (adapter *browserDocumentAdapter) Baseline() documentmeta.Metadata {
	if adapter == nil {
		return documentmeta.Metadata{}
	}
	return adapter.baseline
}

func (adapter *browserDocumentAdapter) Apply(metadata documentmeta.Metadata) error {
	if adapter == nil {
		return errors.New("document adapter is nil")
	}
	if !adapter.title.Get("isConnected").Bool() {
		return errors.New("authored title element is disconnected")
	}
	if !adapter.description.Get("isConnected").Bool() {
		return errors.New("authored description element is disconnected")
	}
	if adapter.unrelated.Call("getAttribute", "content").String() != "preserve-me" {
		return errors.New("unrelated authored metadata changed")
	}

	previous := adapter.current
	if adapter.title.Get("textContent").String() != metadata.Title {
		adapter.title.Set("textContent", metadata.Title)
	}
	if adapter.description.Call("getAttribute", "content").String() != metadata.Description {
		adapter.description.Call("setAttribute", "content", metadata.Description)
	}
	adapter.current = metadata
	incrementEvidence("documentCommits")
	if previous != adapter.baseline && metadata == adapter.baseline {
		incrementEvidence("baselineRestorations")
	}
	return nil
}

func initDocumentAPIDesignEvidence(mode string) {
	evidence := js.Global().Get("Object").New()
	evidence.Set("mode", mode)
	for _, field := range []string{
		"transitions",
		"adds",
		"updates",
		"removes",
		"noops",
		"renders",
		"documentCommits",
		"baselineRestorations",
		"scopeMounts",
		"scopeUnmounts",
		"componentMounts",
		"componentUnmounts",
		"handleForwards",
		"handleDuplicateCoalesces",
		"handleCreations",
		"publicationCreations",
		"zeroIDOwnerRenders",
		"errorBoundaryCaptures",
	} {
		evidence.Set(field, 0)
	}
	for _, field := range []string{
		"events",
		"ownerRenders",
		"componentLifetimes",
		"handleForwardedOwnerIDs",
		"handleDuplicateOwnerIDs",
		"handleCreationEvents",
		"publicationCreationEvents",
		"coordinatorStatisticsEvents",
		"runtimeErrors",
	} {
		evidence.Set(field, js.Global().Get("Array").New())
	}
	js.Global().Set("goframeDocumentAPIDesignEvidence", evidence)
}

func recordCandidateTransition(
	candidate string,
	role string,
	transition documentmeta.Transition,
) {
	incrementEvidence("transitions")
	switch transition.Change {
	case documentmeta.ChangeAdded:
		incrementEvidence("adds")
	case documentmeta.ChangeUpdated:
		incrementEvidence("updates")
	case documentmeta.ChangeRemoved:
		incrementEvidence("removes")
	default:
		incrementEvidence("noops")
	}
	event := js.Global().Get("Object").New()
	event.Set("candidate", candidate)
	event.Set("role", role)
	event.Set("change", transition.Change.String())
	event.Set("ownerID", transition.OwnerID)
	event.Set("activeOwnerID", transition.Snapshot.ActiveOwnerID)
	event.Set("ownerCount", transition.Snapshot.OwnerCount)
	event.Set("title", transition.Snapshot.Metadata.Title)
	event.Set("description", transition.Snapshot.Metadata.Description)
	evidence().Get("events").Call("push", event)
}

func recordCandidateRender(candidate string, role string, nonce int) {
	incrementEvidence("renders")
	event := js.Global().Get("Object").New()
	event.Set("candidate", candidate)
	event.Set("role", role)
	event.Set("nonce", nonce)
	evidence().Get("ownerRenders").Call("push", event)
}

func recordCandidateOwnerRender(candidate string, role string, ownerID uint64) {
	incrementEvidence("renders")
	if ownerID == 0 {
		incrementEvidence("zeroIDOwnerRenders")
	}
	event := js.Global().Get("Object").New()
	event.Set("candidate", candidate)
	event.Set("role", role)
	event.Set("ownerID", ownerID)
	evidence().Get("ownerRenders").Call("push", event)
}

func recordComponentOwnerMount(role string, ownerID uint64) {
	incrementEvidence("componentMounts")
	recordComponentLifetime("mount", role, ownerID)
}

func recordComponentOwnerUnmount(role string, ownerID uint64) {
	incrementEvidence("componentUnmounts")
	recordComponentLifetime("unmount", role, ownerID)
}

func recordComponentLifetime(action string, role string, ownerID uint64) {
	event := js.Global().Get("Object").New()
	event.Set("action", action)
	event.Set("role", role)
	event.Set("ownerID", ownerID)
	evidence().Get("componentLifetimes").Call("push", event)
}

func recordHandleForward(ownerID uint64) {
	incrementEvidence("handleForwards")
	evidence().Get("handleForwardedOwnerIDs").Call("push", ownerID)
}

func recordHandleDuplicateCoalesced(ownerID uint64) {
	incrementEvidence("handleDuplicateCoalesces")
	evidence().Get("handleDuplicateOwnerIDs").Call("push", ownerID)
}

func recordHandleCreation(role string) {
	incrementEvidence("handleCreations")
	recordLifecycleCreation("handleCreationEvents", role, false)
}

func recordPublicationCreation(role string, forwarded bool) {
	incrementEvidence("publicationCreations")
	recordLifecycleCreation("publicationCreationEvents", role, forwarded)
}

func recordLifecycleCreation(field string, role string, forwarded bool) {
	event := js.Global().Get("Object").New()
	event.Set("role", role)
	event.Set("forwarded", forwarded)
	setCurrentEvidencePhase(event)
	evidence().Get(field).Call("push", event)
}

func recordCoordinatorStatistics(statistics documentmeta.Statistics) {
	current := coordinatorStatisticsValue(statistics)
	evidence().Set("coordinatorStatistics", current)
	event := coordinatorStatisticsValue(statistics)
	setCurrentEvidencePhase(event)
	evidence().Get("coordinatorStatisticsEvents").Call("push", event)
}

func coordinatorStatisticsValue(statistics documentmeta.Statistics) js.Value {
	value := js.Global().Get("Object").New()
	value.Set("tokenCreations", statistics.TokenCreations)
	value.Set("committedIDAssignments", statistics.CommittedIDAssignments)
	value.Set("activeAdditions", statistics.ActiveAdditions)
	value.Set("updates", statistics.Updates)
	value.Set("releases", statistics.Releases)
	value.Set("activeOwnerCount", statistics.ActiveOwnerCount)
	value.Set("lastCommittedOwnerID", statistics.LastCommittedOwnerID)
	return value
}

func setCurrentEvidencePhase(value js.Value) {
	head := js.Global().Get("__documentAPIDesignHeadEvidence")
	if head.IsUndefined() || head.IsNull() {
		return
	}
	value.Set("phase", head.Get("phase"))
}

func recordScopeMount() {
	incrementEvidence("scopeMounts")
}

func recordScopeUnmount() {
	incrementEvidence("scopeUnmounts")
}

func recordDocumentAPIDesignError(info gf.ErrorInfo) {
	incrementEvidence("errorBoundaryCaptures")
	report := js.Global().Get("Object").New()
	report.Set("phase", info.Phase.String())
	report.Set("component", info.Component)
	report.Set("operation", info.Operation)
	report.Set("panic", gf.ToString(info.Panic))
	evidence().Get("runtimeErrors").Call("push", report)
}

func incrementEvidence(field string) {
	current := evidence()
	current.Set(field, current.Get(field).Int()+1)
}

func evidence() js.Value {
	return js.Global().Get("goframeDocumentAPIDesignEvidence")
}

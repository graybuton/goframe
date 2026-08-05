//go:build js && wasm && goframe_document_state_experiment

package main

import (
	"strings"
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

type metadata = gf.DocumentMetadataHandoffExperimentValue

type appProps struct {
	scenario string
}

type ownerProps struct {
	role     string
	metadata metadata
	failure  bool
	children []gf.Node
}

type replacementState struct {
	parent metadata
	showB  bool
	showC  bool
}

var (
	appType = gf.NewComponentType(
		"fixture.document-state-handoff.App",
		"DocumentStateHandoffApp",
	)
	ownerType = gf.NewComponentType(
		"fixture.document-state-handoff.Owner",
		"DocumentStateHandoffOwner",
	)
	directType = gf.NewComponentType(
		"fixture.document-state-handoff.Direct",
		"DocumentStateHandoffDirect",
	)
	nestedType = gf.NewComponentType(
		"fixture.document-state-handoff.Nested",
		"DocumentStateHandoffNested",
	)
	nonselectedType = gf.NewComponentType(
		"fixture.document-state-handoff.Nonselected",
		"DocumentStateHandoffNonselected",
	)
	sameValueType = gf.NewComponentType(
		"fixture.document-state-handoff.SameValue",
		"DocumentStateHandoffSameValue",
	)
	failedInitialType = gf.NewComponentType(
		"fixture.document-state-handoff.FailedInitial",
		"DocumentStateHandoffFailedInitial",
	)
	failedReplacementType = gf.NewComponentType(
		"fixture.document-state-handoff.FailedReplacement",
		"DocumentStateHandoffFailedReplacement",
	)
	multipleType = gf.NewComponentType(
		"fixture.document-state-handoff.Multiple",
		"DocumentStateHandoffMultiple",
	)
	lifetimeType = gf.NewComponentType(
		"fixture.document-state-handoff.Lifetime",
		"DocumentStateHandoffLifetime",
	)

	retainedFunctions []js.Func
)

func main() {
	initializeEvidence()
	installDebugProbes()
	adapter := newDocumentAdapter()
	gf.InstallDocumentMetadataHandoffExperiment(
		adapter.baseline,
		adapter.apply,
		recordOwnershipEvent,
	)
	gf.SetErrorHandler(recordRuntimeError)
	installMountActions()

	scenario := queryParameter("scenario")
	if scenario == "" {
		scenario = "direct"
	}
	gf.Mount("root", func() gf.Node {
		return gf.ComponentT(appType, appProps{scenario: scenario}, renderApp)
	})
	select {}
}

func renderApp(props appProps) gf.Node {
	gf.UseEffect(func() gf.Cleanup {
		incrementEvidence("effectFlushes")
		return nil
	}, gf.EveryRender())
	var scenario gf.Node
	switch props.scenario {
	case "direct", "repeated-mount", "teardown":
		scenario = gf.ComponentT(directType, struct{}{}, renderDirectScenario)
	case "nested":
		scenario = gf.ComponentT(nestedType, struct{}{}, renderNestedScenario)
	case "nonselected":
		scenario = gf.ComponentT(nonselectedType, struct{}{}, renderNonselectedScenario)
	case "same-value":
		scenario = gf.ComponentT(sameValueType, struct{}{}, renderSameValueScenario)
	case "failed-initial":
		scenario = gf.ComponentT(failedInitialType, struct{}{}, renderFailedInitialScenario)
	case "failed-replacement":
		scenario = gf.ComponentT(failedReplacementType, struct{}{}, renderFailedReplacementScenario)
	case "multiple":
		scenario = gf.ComponentT(multipleType, struct{}{}, renderMultipleScenario)
	case "lifetime":
		scenario = gf.ComponentT(lifetimeType, struct{}{}, renderLifetimeScenario)
	default:
		panic("document-state handoff fixture: unknown scenario " + props.scenario)
	}
	return gf.El(
		"main",
		gf.Props{
			"data-testid":   "handoff-app",
			"data-scenario": props.scenario,
		},
		gf.El("h1", nil, gf.Text("Transactional document-state handoff")),
		scenario,
	)
}

func renderDirectScenario(struct{}) gf.Node {
	target, setTarget := gf.UseState("a")
	value := metadataA()
	if target == "b" {
		value = metadataB()
	}
	return scenarioSection(
		button("replace-owner", "Replace A with B", func() {
			setTarget("b")
		}),
		ownerNode("direct-"+target, target, value, false),
	)
}

func renderNestedScenario(struct{}) gf.Node {
	parent, setParent := gf.UseState(metadataA())
	parentActive, setParentActive := gf.UseState(true)
	nested, setNested := gf.UseState(true)
	children := []gf.Node{}
	if nested {
		children = append(children, ownerNode("nested-b", "nested-b", metadataB(), false))
	}
	parentNode := gf.Empty()
	if parentActive {
		parentNode = ownerNode("parent-a", "parent", parent, false, children...)
	}
	return scenarioSection(
		button("update-parent", "Update parent", func() {
			setParent(metadataA2())
		}),
		button("release-nested", "Release nested", func() {
			setNested(false)
		}),
		button("release-parent", "Release parent", func() {
			setParentActive(false)
		}),
		parentNode,
	)
}

func renderNonselectedScenario(struct{}) gf.Node {
	showA, setShowA := gf.UseState(true)
	children := []gf.Node{}
	if showA {
		children = append(children, ownerNode("nonselected-a", "nonselected-a", metadataA(), false))
	}
	children = append(children, ownerNode("selected-b", "selected-b", metadataB(), false))
	return scenarioSection(
		button("release-nonselected", "Release non-selected", func() {
			setShowA(false)
		}),
		gf.Fragment(children...),
	)
}

func renderSameValueScenario(struct{}) gf.Node {
	nonce, setNonce := gf.UseState(0)
	return scenarioSection(
		button("publish-same", "Publish same value", func() {
			setNonce(nonce + 1)
		}),
		ownerNode("same-owner", "same-owner", metadataA(), false,
			gf.El("span", gf.Props{"data-testid": "same-nonce"}, gf.Text(gf.ToString(nonce))),
		),
	)
}

func renderFailedInitialScenario(struct{}) gf.Node {
	return scenarioSection(gf.ErrorBoundary(gf.ErrorBoundaryProps{
		Fallback: func(gf.ErrorBoundaryContext) gf.Node {
			return gf.El("p", gf.Props{"data-testid": "failed-initial-fallback"}, gf.Text("failed initial"))
		},
		Children: []gf.Node{
			ownerNode("failed-initial-owner", "failed-initial", metadataFailure(), true),
		},
	}))
}

func renderFailedReplacementScenario(struct{}) gf.Node {
	active, setActive := gf.UseState(false)
	failure, setFailure := gf.UseState(true)
	child := gf.Empty()
	if active {
		child = gf.ErrorBoundary(gf.ErrorBoundaryProps{
			Fallback: func(context gf.ErrorBoundaryContext) gf.Node {
				return button("retry-owner", "Retry owner", func() {
					setFailure(false)
					context.Reset()
				})
			},
			Children: []gf.Node{
				ownerNode("replacement-b", "replacement-b", metadataB(), failure),
			},
		})
	}
	return scenarioSection(
		button("activate-failed-owner", "Activate failing owner", func() {
			setActive(true)
		}),
		ownerNode("replacement-parent", "replacement-parent", metadataA(), false, child),
	)
}

func renderMultipleScenario(struct{}) gf.Node {
	state, setState := gf.UseState(replacementState{
		parent: metadataA(),
		showB:  true,
	})
	children := []gf.Node{
		ownerNode("multiple-a", "multiple-a", state.parent, false),
	}
	if state.showB {
		children = append(children, ownerNode("multiple-b", "multiple-b", metadataB(), false))
	}
	if state.showC {
		children = append(children, ownerNode("multiple-c", "multiple-c", metadataC(), false))
	}
	return scenarioSection(
		button("run-multiple", "Run multiple operations", func() {
			setState(replacementState{
				parent: metadataA2(),
				showC:  true,
			})
		}),
		gf.Fragment(children...),
	)
}

func renderLifetimeScenario(struct{}) gf.Node {
	active, setActive := gf.UseState(true)
	generation, setGeneration := gf.UseState(1)
	child := gf.Empty()
	if active {
		child = ownerNode(
			"lifetime-"+gf.ToString(generation),
			"lifetime",
			metadataA(),
			false,
		)
	}
	return scenarioSection(
		button("unmount-owner", "Unmount owner", func() {
			setActive(false)
		}),
		button("remount-owner", "Remount owner", func() {
			setGeneration(generation + 1)
			setActive(true)
		}),
		child,
	)
}

func renderOwner(props ownerProps) gf.Node {
	recordOwnerRender(props.role)
	gf.UseDocumentMetadataHandoffExperiment(props.metadata)
	if props.failure {
		panic("document-state handoff fixture: forced owner render failure")
	}
	return gf.Fragment(props.children...)
}

func ownerNode(
	key string,
	role string,
	value metadata,
	failure bool,
	children ...gf.Node,
) gf.Node {
	return gf.Key(key, gf.ComponentT(ownerType, ownerProps{
		role:     role,
		metadata: value,
		failure:  failure,
		children: children,
	}, renderOwner))
}

func scenarioSection(children ...gf.Node) gf.Node {
	return gf.El("section", gf.Props{"data-testid": "handoff-scenario"}, children...)
}

func button(testID string, label string, action func()) gf.Node {
	return gf.El("button", gf.Props{
		"type":        "button",
		"data-testid": testID,
		"onClick":     action,
	}, gf.Text(label))
}

type documentAdapter struct {
	title       js.Value
	description js.Value
	unrelated   js.Value
	baseline    metadata
	current     metadata
}

func newDocumentAdapter() *documentAdapter {
	document := js.Global().Get("document")
	title := document.Call("querySelector", "head title")
	description := document.Call("querySelector", `head meta[name="description"]`)
	unrelated := document.Call("querySelector", `head meta[name="fixture-unrelated"]`)
	if title.IsNull() || description.IsNull() || unrelated.IsNull() {
		panic("document-state handoff fixture: authored head nodes are missing")
	}
	baseline := metadata{
		Title:       title.Get("textContent").String(),
		Description: description.Call("getAttribute", "content").String(),
	}
	return &documentAdapter{
		title:       title,
		description: description,
		unrelated:   unrelated,
		baseline:    baseline,
		current:     baseline,
	}
}

func (adapter *documentAdapter) apply(value metadata) {
	if !adapter.title.Get("isConnected").Bool() ||
		!adapter.description.Get("isConnected").Bool() {
		panic("document-state handoff fixture: authored metadata node was disconnected")
	}
	if adapter.unrelated.Call("getAttribute", "content").String() != "preserve-me" {
		panic("document-state handoff fixture: unrelated metadata changed")
	}
	previous := adapter.current
	if adapter.title.Get("textContent").String() != value.Title {
		adapter.title.Set("textContent", value.Title)
	}
	if adapter.description.Call("getAttribute", "content").String() != value.Description {
		adapter.description.Call("setAttribute", "content", value.Description)
	}
	adapter.current = value
	incrementEvidence("documentApplies")
	if previous != adapter.baseline && value == adapter.baseline {
		incrementEvidence("baselineRestorations")
	}
	appendPairEvidence("documentSnapshots", value)
}

func initializeEvidence() {
	evidence := js.Global().Get("Object").New()
	for _, name := range []string{
		"documentApplies",
		"baselineRestorations",
		"effectFlushes",
		"renderReports",
		"componentRenders",
		"componentPatches",
		"runtimeErrors",
	} {
		evidence.Set(name, 0)
	}
	for _, name := range []string{
		"ownershipEvents",
		"documentSnapshots",
		"ownerRenders",
		"runtimeReports",
	} {
		evidence.Set(name, js.Global().Get("Array").New())
	}
	evidence.Set("snapshot", js.Global().Get("Object").New())
	evidence.Set("statistics", js.Global().Get("Object").New())
	js.Global().Set("goframeDocumentHandoffEvidence", evidence)
}

func installDebugProbes() {
	retainGlobalFunction("goframeRenderProbe", func(args []js.Value) {
		incrementEvidence("renderReports")
	})
	retainGlobalFunction("goframeComponentRenderProbe", func(args []js.Value) {
		incrementEvidence("componentRenders")
	})
	retainGlobalFunction("goframeComponentPatchProbe", func(args []js.Value) {
		incrementEvidence("componentPatches")
	})
}

func installMountActions() {
	retainGlobalFunction("goframeDocumentHandoffRepeatedMount", func(args []js.Value) {
		gf.Mount("root", func() gf.Node {
			return scenarioSection(ownerNode("repeated-b", "repeated-b", metadataB(), false))
		})
	})
	retainGlobalFunction("goframeDocumentHandoffTeardown", func(args []js.Value) {
		gf.Mount("root", func() gf.Node {
			return gf.El("main", gf.Props{"data-testid": "ownerless-app"}, gf.Text("ownerless"))
		})
	})
}

func retainGlobalFunction(name string, callback func([]js.Value)) {
	function := js.FuncOf(func(this js.Value, args []js.Value) any {
		callback(args)
		return nil
	})
	retainedFunctions = append(retainedFunctions, function)
	js.Global().Set(name, function)
}

func recordOwnershipEvent(event gf.DocumentMetadataHandoffExperimentEvent) {
	value := js.Global().Get("Object").New()
	value.Set("kind", event.Kind)
	value.Set("batchID", event.BatchID)
	value.Set("ownerID", event.OwnerID)
	value.Set("ownerCount", event.OwnerCount)
	value.Set("title", event.Title)
	value.Set("description", event.Description)
	evidence().Get("ownershipEvents").Call("push", value)
	updateCommittedEvidence()
}

func updateCommittedEvidence() {
	snapshot := gf.CurrentDocumentMetadataHandoffExperiment()
	snapshotValue := js.Global().Get("Object").New()
	snapshotValue.Set("activeOwnerID", snapshot.ActiveOwnerID)
	snapshotValue.Set("ownerCount", snapshot.OwnerCount)
	snapshotValue.Set("title", snapshot.Title)
	snapshotValue.Set("description", snapshot.Description)
	evidence().Set("snapshot", snapshotValue)

	statistics := gf.DocumentMetadataHandoffExperimentStats()
	statisticsValue := js.Global().Get("Object").New()
	statisticsValue.Set("tokenCreations", statistics.TokenCreations)
	statisticsValue.Set("committedIDAssignments", statistics.CommittedIDAssignments)
	statisticsValue.Set("activeAdditions", statistics.ActiveAdditions)
	statisticsValue.Set("updates", statistics.Updates)
	statisticsValue.Set("releases", statistics.Releases)
	statisticsValue.Set("duplicatePublications", statistics.DuplicatePublications)
	statisticsValue.Set("rollbacks", statistics.Rollbacks)
	statisticsValue.Set("updateBatches", statistics.UpdateBatches)
	statisticsValue.Set("documentPublications", statistics.DocumentPublications)
	statisticsValue.Set("baselineRestorations", statistics.BaselineRestorations)
	evidence().Set("statistics", statisticsValue)
}

func recordOwnerRender(role string) {
	value := js.Global().Get("Object").New()
	value.Set("role", role)
	evidence().Get("ownerRenders").Call("push", value)
}

func recordRuntimeError(info gf.ErrorInfo) {
	incrementEvidence("runtimeErrors")
	value := js.Global().Get("Object").New()
	value.Set("phase", info.Phase.String())
	value.Set("component", info.Component)
	value.Set("operation", info.Operation)
	value.Set("panic", gf.ToString(info.Panic))
	evidence().Get("runtimeReports").Call("push", value)
}

func appendPairEvidence(name string, value metadata) {
	pair := js.Global().Get("Object").New()
	pair.Set("title", value.Title)
	pair.Set("description", value.Description)
	evidence().Get(name).Call("push", pair)
}

func incrementEvidence(name string) {
	current := evidence()
	current.Set(name, current.Get(name).Int()+1)
}

func evidence() js.Value {
	return js.Global().Get("goframeDocumentHandoffEvidence")
}

func queryParameter(name string) string {
	search := js.Global().Get("location").Get("search").String()
	parameters := js.Global().Get("URLSearchParams").New(search)
	value := parameters.Call("get", name)
	if value.IsNull() || value.IsUndefined() {
		return ""
	}
	return strings.TrimSpace(value.String())
}

func metadataA() metadata {
	return metadata{Title: "Owner A · GoFrame", Description: "Description A"}
}

func metadataA2() metadata {
	return metadata{Title: "Owner A2 · GoFrame", Description: "Description A2"}
}

func metadataB() metadata {
	return metadata{Title: "Owner B · GoFrame", Description: "Description B"}
}

func metadataC() metadata {
	return metadata{Title: "Owner C · GoFrame", Description: "Description C"}
}

func metadataFailure() metadata {
	return metadata{Title: "Failed owner · GoFrame", Description: "Failed owner description"}
}

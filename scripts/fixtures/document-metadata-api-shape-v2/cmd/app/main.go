//go:build js && wasm && goframe_document_state_experiment

package main

import (
	"strings"
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

type metadata = gf.DocumentMetadataHandoffExperimentValue

type appProps struct {
	mode     string
	scenario string
}

type ownerProps struct {
	role          string
	metadata      metadata
	failureBefore bool
	failureAfter  bool
	children      []gf.Node
}

type replacementState struct {
	parent metadata
	showB  bool
	showC  bool
}

type finalizationScenarioProps struct {
	prefix        string
	crossBoundary bool
}

type finalizationBoundaryProps struct {
	prefix string
}

type successorHostProps struct {
	prefix        string
	crossBoundary bool
}

type hookSlotsProps struct {
	first metadata
}

type componentContractProps struct {
	value    metadata
	children []gf.Node
}

type handlePublicationProps struct {
	owner    *gf.DocumentMetadataOwner
	value    metadata
	testID   string
	children []gf.Node
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
	controlledOwnerType = gf.NewComponentType(
		"fixture.document-state-handoff.ControlledOwner",
		"DocumentStateHandoffControlledOwner",
	)
	shellType = gf.NewComponentType(
		"fixture.document-state-handoff.Shell",
		"DocumentStateHandoffShell",
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
	panicBeforeMetadataType = gf.NewComponentType(
		"fixture.document-state-handoff.PanicBeforeMetadata",
		"DocumentStateHandoffPanicBeforeMetadata",
	)
	siblingFailureType = gf.NewComponentType(
		"fixture.document-state-handoff.SiblingFailure",
		"DocumentStateHandoffSiblingFailure",
	)
	ownerlessRecoveryType = gf.NewComponentType(
		"fixture.document-state-handoff.OwnerlessRecovery",
		"DocumentStateHandoffOwnerlessRecovery",
	)
	nestedOuterFailureType = gf.NewComponentType(
		"fixture.document-state-handoff.NestedOuterFailure",
		"DocumentStateHandoffNestedOuterFailure",
	)
	publicationFailureType = gf.NewComponentType(
		"fixture.document-state-handoff.PublicationFailure",
		"DocumentStateHandoffPublicationFailure",
	)
	reverseRetryType = gf.NewComponentType(
		"fixture.document-state-handoff.ReverseRetry",
		"DocumentStateHandoffReverseRetry",
	)
	partialReadinessType = gf.NewComponentType(
		"fixture.document-state-handoff.PartialReadiness",
		"DocumentStateHandoffPartialReadiness",
	)
	additivePlanType = gf.NewComponentType(
		"fixture.document-state-handoff.AdditivePlan",
		"DocumentStateHandoffAdditivePlan",
	)
	initialPlanType = gf.NewComponentType(
		"fixture.document-state-handoff.InitialPlan",
		"DocumentStateHandoffInitialPlan",
	)
	finalizationAbsorptionType = gf.NewComponentType(
		"fixture.document-state-handoff.FinalizationAbsorption",
		"DocumentStateHandoffFinalizationAbsorption",
	)
	finalizationBoundaryType = gf.NewComponentType(
		"fixture.document-state-handoff.FinalizationBoundary",
		"DocumentStateHandoffFinalizationBoundary",
	)
	successorHostType = gf.NewComponentType(
		"fixture.document-state-handoff.SuccessorHost",
		"DocumentStateHandoffSuccessorHost",
	)
	newerBoundaryFailureType = gf.NewComponentType(
		"fixture.document-state-handoff.NewerBoundaryFailure",
		"DocumentStateHandoffNewerBoundaryFailure",
	)
	failingSiblingType = gf.NewComponentType(
		"fixture.document-state-handoff.FailingSibling",
		"DocumentStateHandoffFailingSibling",
	)
	multipleType = gf.NewComponentType(
		"fixture.document-state-handoff.Multiple",
		"DocumentStateHandoffMultiple",
	)
	lifetimeType = gf.NewComponentType(
		"fixture.document-state-handoff.Lifetime",
		"DocumentStateHandoffLifetime",
	)
	hookContractType = gf.NewComponentType(
		"fixture.document-metadata-api-shape-v2.HookContract",
		"DocumentMetadataAPIShapeHookContract",
	)
	hookSlotsType = gf.NewComponentType(
		"fixture.document-metadata-api-shape-v2.HookSlots",
		"DocumentMetadataAPIShapeHookSlots",
	)
	componentContractType = gf.NewComponentType(
		"fixture.document-metadata-api-shape-v2.ComponentContract",
		"DocumentMetadataAPIShapeComponentContract",
	)
	componentProjectionType = gf.NewComponentType(
		"fixture.document-metadata-api-shape-v2.ComponentProjection",
		"DocumentMetadataAPIShapeComponentProjection",
	)
	handleContractType = gf.NewComponentType(
		"fixture.document-metadata-api-shape-v2.HandleContract",
		"DocumentMetadataAPIShapeHandleContract",
	)
	handlePublicationType = gf.NewComponentType(
		"fixture.document-metadata-api-shape-v2.HandlePublication",
		"DocumentMetadataAPIShapeHandlePublication",
	)

	retainedFunctions      []js.Func
	activeDocumentAdapter  *documentAdapter
	activeCandidateMode    string
	activeComparisonHandle *gf.DocumentMetadataOwner
	controlledOwnerUpdates = make(map[string]func())
)

func main() {
	initializeEvidence()
	installDebugProbes()
	adapter := newDocumentAdapter()
	activeDocumentAdapter = adapter
	gf.InstallDocumentMetadataHandoffExperiment(
		adapter.baseline,
		adapter.apply,
		recordOwnershipEvent,
	)
	gf.SetErrorHandler(recordRuntimeError)
	installMountActions()

	mode := queryParameter("mode")
	if !validCandidateMode(mode) {
		mode = "control"
	}
	activeCandidateMode = mode
	scenario := queryParameter("scenario")
	if scenario == "" {
		scenario = "direct"
	}
	gf.Mount("root", func() gf.Node {
		return gf.ComponentT(appType, appProps{mode: mode, scenario: scenario}, renderApp)
	})
	select {}
}

func validCandidateMode(mode string) bool {
	switch mode {
	case "control", "hook", "component", "handle":
		return true
	default:
		return false
	}
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
	case "panic-before-metadata":
		scenario = gf.ComponentT(panicBeforeMetadataType, struct{}{}, renderPanicBeforeMetadataScenario)
	case "sibling-failure":
		scenario = gf.ComponentT(siblingFailureType, struct{}{}, renderSiblingFailureScenario)
	case "ownerless-recovery":
		scenario = gf.ComponentT(ownerlessRecoveryType, struct{}{}, renderOwnerlessRecoveryScenario)
	case "nested-outer-failure":
		scenario = gf.ComponentT(nestedOuterFailureType, struct{}{}, renderNestedOuterFailureScenario)
	case "publication-failure":
		scenario = gf.ComponentT(publicationFailureType, struct{}{}, renderPublicationFailureScenario)
	case "plan-reversed":
		scenario = gf.ComponentT(reverseRetryType, struct{}{}, renderReverseRetryScenario)
	case "plan-partial":
		scenario = gf.ComponentT(partialReadinessType, struct{}{}, renderPartialReadinessScenario)
	case "plan-additive":
		scenario = gf.ComponentT(additivePlanType, struct{}{}, renderAdditivePlanScenario)
	case "plan-initial":
		scenario = gf.ComponentT(initialPlanType, struct{}{}, renderInitialPlanScenario)
	case "plan-finalization-external":
		scenario = gf.ComponentT(
			finalizationAbsorptionType,
			finalizationScenarioProps{prefix: "external"},
			renderFinalizationAbsorptionScenario,
		)
	case "plan-finalization-cross-boundary":
		scenario = gf.ComponentT(
			finalizationAbsorptionType,
			finalizationScenarioProps{prefix: "cross", crossBoundary: true},
			renderFinalizationAbsorptionScenario,
		)
	case "plan-newer-boundary-failure":
		scenario = gf.ComponentT(
			newerBoundaryFailureType,
			struct{}{},
			renderNewerBoundaryFailureScenario,
		)
	case "multiple":
		scenario = gf.ComponentT(multipleType, struct{}{}, renderMultipleScenario)
	case "lifetime":
		scenario = gf.ComponentT(lifetimeType, struct{}{}, renderLifetimeScenario)
	case "hook-contract":
		scenario = gf.ComponentT(hookContractType, struct{}{}, renderHookContractScenario)
	case "component-contract":
		scenario = gf.ComponentT(componentContractType, struct{}{}, renderComponentContractScenario)
	case "handle-contract":
		scenario = gf.ComponentT(handleContractType, struct{}{}, renderHandleContractScenario)
	default:
		panic("document-state handoff fixture: unknown scenario " + props.scenario)
	}
	return gf.El(
		"main",
		gf.Props{
			"data-testid":   "handoff-app",
			"data-mode":     props.mode,
			"data-scenario": props.scenario,
		},
		gf.El("h1", nil, gf.Text("Transactional document metadata API shapes")),
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
	target, setTarget := gf.UseState("a")
	failure, setFailure := gf.UseState(true)
	value := metadataA()
	if target == "b" {
		value = metadataB()
	}
	return scenarioSection(
		button("activate-failed-owner", "Activate failing owner", func() {
			setTarget("b")
		}),
		gf.ErrorBoundary(gf.ErrorBoundaryProps{
			Fallback: func(context gf.ErrorBoundaryContext) gf.Node {
				return button("retry-owner", "Retry owner", func() {
					setFailure(false)
					context.Reset()
				})
			},
			Children: []gf.Node{
				ownerNode(
					"replacement-"+target,
					"replacement-"+target,
					value,
					target == "b" && failure,
				),
			},
		}),
	)
}

func renderPanicBeforeMetadataScenario(struct{}) gf.Node {
	target, setTarget := gf.UseState("a")
	failure, setFailure := gf.UseState(true)
	value := metadataA()
	if target == "b" {
		value = metadataB()
	}
	return scenarioSection(
		button("activate-pre-metadata-failure", "Activate pre-metadata failure", func() {
			setTarget("b")
		}),
		gf.ErrorBoundary(gf.ErrorBoundaryProps{
			Fallback: func(context gf.ErrorBoundaryContext) gf.Node {
				return button("retry-pre-metadata", "Retry pre-metadata owner", func() {
					setFailure(false)
					context.Reset()
				})
			},
			Children: []gf.Node{
				ownerNodeWithFailure(
					"pre-metadata-"+target,
					"pre-metadata-"+target,
					value,
					target == "b" && failure,
					false,
				),
			},
		}),
	)
}

func renderSiblingFailureScenario(struct{}) gf.Node {
	target, setTarget := gf.UseState("a")
	failure, setFailure := gf.UseState(true)
	value := metadataA()
	if target == "b" {
		value = metadataB()
	}
	children := []gf.Node{
		ownerNode("sibling-owner-"+target, "sibling-owner-"+target, value, false),
	}
	if target == "b" && failure {
		children = append(children, failingSiblingNode("sibling-failure"))
	}
	return scenarioSection(
		button("activate-sibling-failure", "Activate sibling failure", func() {
			setTarget("b")
		}),
		gf.ErrorBoundary(gf.ErrorBoundaryProps{
			Fallback: func(context gf.ErrorBoundaryContext) gf.Node {
				return button("retry-sibling-failure", "Retry sibling failure", func() {
					setFailure(false)
					context.Reset()
				})
			},
			Children: children,
		}),
	)
}

func renderOwnerlessRecoveryScenario(struct{}) gf.Node {
	target, setTarget := gf.UseState("a")
	ownerless, setOwnerless := gf.UseState(false)
	children := []gf.Node{
		ownerNodeWithFailure(
			"ownerless-"+target,
			"ownerless-"+target,
			metadataA(),
			target == "b",
			false,
		),
	}
	if ownerless {
		children = []gf.Node{
			gf.El("p", gf.Props{"data-testid": "ownerless-recovery-content"}, gf.Text("ownerless recovery")),
		}
	}
	return scenarioSection(
		button("activate-ownerless-failure", "Activate ownerless failure", func() {
			setTarget("b")
		}),
		gf.ErrorBoundary(gf.ErrorBoundaryProps{
			Fallback: func(context gf.ErrorBoundaryContext) gf.Node {
				return button("recover-ownerless", "Recover without owner", func() {
					setOwnerless(true)
					context.Reset()
				})
			},
			Children: children,
		}),
	)
}

func renderNestedOuterFailureScenario(struct{}) gf.Node {
	target, setTarget := gf.UseState("a")
	failure, setFailure := gf.UseState(true)
	ownerless, setOwnerless := gf.UseState(false)
	value := metadataA()
	if target == "b" {
		value = metadataB()
	}
	children := []gf.Node{
		gf.ErrorBoundary(gf.ErrorBoundaryProps{
			Fallback: func(gf.ErrorBoundaryContext) gf.Node {
				return gf.El("p", gf.Props{"data-testid": "unexpected-inner-fallback"}, gf.Text("inner fallback"))
			},
			Children: []gf.Node{
				ownerNode("nested-outer-"+target, "nested-outer-"+target, value, false),
			},
		}),
	}
	if target == "b" && failure {
		children = append(children, failingSiblingNode("outer-sibling-failure"))
	}
	if ownerless {
		children = []gf.Node{
			gf.El("p", gf.Props{"data-testid": "nested-ownerless-content"}, gf.Text("nested ownerless recovery")),
		}
	}
	return scenarioSection(
		button("activate-nested-outer-failure", "Activate outer failure", func() {
			setTarget("b")
		}),
		gf.ErrorBoundary(gf.ErrorBoundaryProps{
			Fallback: func(context gf.ErrorBoundaryContext) gf.Node {
				return gf.Fragment(
					button("retry-nested-outer", "Retry outer owner", func() {
						setFailure(false)
						context.Reset()
					}),
					button("recover-nested-ownerless", "Recover outer ownerless", func() {
						setFailure(false)
						setOwnerless(true)
						context.Reset()
					}),
				)
			},
			Children: children,
		}),
	)
}

func renderPublicationFailureScenario(struct{}) gf.Node {
	target, setTarget := gf.UseState("a")
	nonce, setNonce := gf.UseState(0)
	active, setActive := gf.UseState(true)
	value := metadataA()
	if target == "b" {
		value = metadataB()
	}
	owner := gf.Empty()
	if active {
		owner = ownerNode(
			"publication-"+target,
			"publication-"+target,
			value,
			false,
			gf.El("span", gf.Props{"data-testid": "publication-nonce"}, gf.Text(gf.ToString(nonce))),
		)
	}
	return scenarioSection(
		button("activate-publication-failure", "Activate publication failure", func() {
			activeDocumentAdapter.failNextPublication = true
			setTarget("b")
		}),
		button("retry-publication", "Retry publication", func() {
			setNonce(nonce + 1)
		}),
		button("unmount-publication-owner", "Unmount publication owner", func() {
			setActive(false)
		}),
		shellNode("publication-shell"),
		owner,
	)
}

func renderReverseRetryScenario(struct{}) gf.Node {
	return renderPendingOwnersScenario("reversed", false)
}

func renderPartialReadinessScenario(struct{}) gf.Node {
	return renderPendingOwnersScenario("partial", true)
}

func renderPendingOwnersScenario(prefix string, partial bool) gf.Node {
	active, setActive := gf.UseState(false)
	owners := gf.Node(ownerNode(prefix+"-a", prefix+"-a", metadataA(), false))
	if active {
		owners = gf.Fragment(
			controlledOwnerNode(prefix+"-b", prefix+"-b", metadataB()),
			controlledOwnerNode(prefix+"-c", prefix+"-c", metadataC()),
		)
	}
	controls := []gf.Node{
		button("activate-plan-"+prefix, "Activate pending owners", func() {
			activeDocumentAdapter.failNextPublication = true
			setActive(true)
		}),
		shellNode(prefix + "-shell"),
	}
	if partial {
		controls = append(
			controls,
			button("retry-plan-partial-b", "Retry B", func() {
				rerenderControlledOwner("partial-b")
			}),
			button("retry-plan-partial-c", "Retry C", func() {
				rerenderControlledOwner("partial-c")
			}),
		)
	} else {
		controls = append(controls, button("retry-plan-reversed", "Retry C then B", func() {
			rerenderControlledOwner("reversed-c")
			rerenderControlledOwner("reversed-b")
		}))
	}
	controls = append(controls, owners)
	return scenarioSection(controls...)
}

func renderAdditivePlanScenario(struct{}) gf.Node {
	showB, setShowB := gf.UseState(false)
	children := []gf.Node{
		button("activate-plan-additive", "Activate additive B", func() {
			activeDocumentAdapter.failNextPublication = true
			setShowB(true)
		}),
		button("retry-plan-additive", "Retry additive B", func() {
			rerenderControlledOwner("additive-b")
		}),
		button("abandon-plan-additive", "Abandon additive B", func() {
			setShowB(false)
		}),
		shellNode("additive-shell"),
		ownerNode("additive-a", "additive-a", metadataA(), false),
	}
	if showB {
		children = append(children, controlledOwnerNode("additive-b", "additive-b", metadataB()))
	}
	return scenarioSection(children...)
}

func renderInitialPlanScenario(struct{}) gf.Node {
	showB, setShowB := gf.UseState(false)
	children := []gf.Node{
		button("activate-plan-initial", "Activate first B", func() {
			activeDocumentAdapter.failNextPublication = true
			setShowB(true)
		}),
		button("retry-plan-initial", "Retry first B", func() {
			rerenderControlledOwner("initial-b")
		}),
		button("abandon-plan-initial", "Abandon first B", func() {
			setShowB(false)
		}),
		shellNode("initial-shell"),
	}
	if showB {
		children = append(children, controlledOwnerNode("initial-b", "initial-b", metadataB()))
	}
	return scenarioSection(children...)
}

func renderFinalizationAbsorptionScenario(props finalizationScenarioProps) gf.Node {
	return scenarioSection(
		gf.Key(props.prefix+"-boundary-driver", gf.ComponentT(
			finalizationBoundaryType,
			finalizationBoundaryProps{prefix: props.prefix},
			renderFinalizationBoundary,
		)),
		gf.Key(props.prefix+"-successor-host", gf.ComponentT(
			successorHostType,
			successorHostProps{
				prefix:        props.prefix,
				crossBoundary: props.crossBoundary,
			},
			renderSuccessorHost,
		)),
	)
}

func renderFinalizationBoundary(props finalizationBoundaryProps) gf.Node {
	phase, setPhase := gf.UseState("a")
	children := []gf.Node{
		ownerNode(props.prefix+"-a", props.prefix+"-a", metadataA(), false),
	}
	if phase == "fail" {
		children = []gf.Node{failingSiblingNode(props.prefix + "-boundary-failure")}
	} else if phase == "ownerless" {
		children = []gf.Node{gf.El(
			"p",
			gf.Props{"data-testid": props.prefix + "-ownerless"},
			gf.Text("ownerless"),
		)}
	}
	return gf.Fragment(
		button(props.prefix+"-fail-boundary", "Fail boundary", func() {
			setPhase("fail")
		}),
		gf.Key(props.prefix+"-boundary-x", gf.ErrorBoundary(gf.ErrorBoundaryProps{
			Fallback: func(context gf.ErrorBoundaryContext) gf.Node {
				return button(props.prefix+"-recover-ownerless", "Recover ownerless", func() {
					activeDocumentAdapter.failNextPublication = true
					setPhase("ownerless")
					context.Reset()
				})
			},
			Children: children,
		})),
	)
}

func renderSuccessorHost(props successorHostProps) gf.Node {
	showB, setShowB := gf.UseState(false)
	owner := gf.Node(gf.Empty())
	if showB {
		owner = controlledOwnerNode(props.prefix+"-b", props.prefix+"-b", metadataB())
	}
	if props.crossBoundary {
		owner = gf.Key(props.prefix+"-boundary-y", gf.ErrorBoundary(gf.ErrorBoundaryProps{
			Fallback: func(gf.ErrorBoundaryContext) gf.Node {
				return gf.El(
					"p",
					gf.Props{"data-testid": props.prefix + "-unexpected-y-fallback"},
					gf.Text("unexpected boundary Y fallback"),
				)
			},
			Children: []gf.Node{owner},
		}))
	}
	return gf.Fragment(
		button(props.prefix+"-mount-successor", "Mount failing successor", func() {
			activeDocumentAdapter.failNextPublication = true
			setShowB(true)
		}),
		button(props.prefix+"-retry-successor", "Retry successor", func() {
			rerenderControlledOwner(props.prefix + "-b")
		}),
		shellNode(props.prefix+"-shell"),
		owner,
	)
}

func renderNewerBoundaryFailureScenario(struct{}) gf.Node {
	phase, setPhase := gf.UseState("a")
	children := []gf.Node{
		ownerNode("supersede-a", "supersede-a", metadataA(), false),
	}
	switch phase {
	case "fail-initial":
		children = []gf.Node{failingSiblingNode("supersede-initial-failure")}
	case "ownerless", "final":
		children = []gf.Node{gf.El(
			"p",
			gf.Props{"data-testid": "supersede-ownerless"},
			gf.Text("ownerless"),
		)}
	case "b":
		children = []gf.Node{controlledOwnerNode("supersede-b", "supersede-b", metadataB())}
	case "fail-newer":
		children = []gf.Node{
			controlledOwnerNode("supersede-b", "supersede-b", metadataB()),
			failingSiblingNode("supersede-newer-failure"),
		}
	}
	fallback := func(context gf.ErrorBoundaryContext) gf.Node {
		if phase == "fail-newer" {
			return button("supersede-final-recover", "Recover final ownerless state", func() {
				setPhase("final")
				context.Reset()
			})
		}
		return button("supersede-recover-ownerless", "Recover ownerless", func() {
			activeDocumentAdapter.failNextPublication = true
			setPhase("ownerless")
			context.Reset()
		})
	}
	return scenarioSection(
		button("supersede-fail-initial", "Fail initial owner", func() {
			setPhase("fail-initial")
		}),
		button("supersede-mount-b", "Mount pending B", func() {
			activeDocumentAdapter.failNextPublication = true
			setPhase("b")
		}),
		button("supersede-fail-newer", "Fail boundary again", func() {
			setPhase("fail-newer")
		}),
		shellNode("supersede-shell"),
		gf.Key("supersede-boundary-x", gf.ErrorBoundary(gf.ErrorBoundaryProps{
			Fallback: fallback,
			Children: children,
		})),
	)
}

func renderControlledOwner(props ownerProps) gf.Node {
	nonce, setNonce := gf.UseState(0)
	controlledOwnerUpdates[props.role] = func() {
		setNonce(nonce + 1)
	}
	recordOwnerRender(props.role)
	children := renderCandidateOwnerProjection(activeCandidateMode, props.metadata, props.children)
	gf.UseUnmount(func() {
		delete(controlledOwnerUpdates, props.role)
		recordOwnerCleanup(props.role)
	})
	if props.failureBefore {
		panic("document-state handoff fixture: forced controlled owner render failure before metadata")
	}
	if props.failureAfter {
		panic("document-state handoff fixture: forced controlled owner render failure")
	}
	return gf.Fragment(
		gf.El(
			"span",
			gf.Props{
				"data-controlled-owner": props.role,
				"hidden":                true,
			},
			gf.Text(gf.ToString(nonce)),
		),
		children,
	)
}

func controlledOwnerNode(key string, role string, value metadata) gf.Node {
	return gf.Key(key, gf.ComponentT(controlledOwnerType, ownerProps{
		role:     role,
		metadata: value,
	}, renderControlledOwner))
}

func rerenderControlledOwner(role string) {
	update := controlledOwnerUpdates[role]
	if update == nil {
		panic("document-state handoff fixture: controlled owner is unavailable: " + role)
	}
	update()
}

func renderShell(role string) gf.Node {
	value, setValue := gf.UseState(0)
	return gf.Fragment(
		button("update-"+role, "Update unrelated shell", func() {
			setValue(value + 1)
		}),
		gf.El(
			"span",
			gf.Props{"data-shell-role": role},
			gf.Text(gf.ToString(value)),
		),
	)
}

func shellNode(role string) gf.Node {
	return gf.Key(role, gf.ComponentT(shellType, role, renderShell))
}

func failingSiblingNode(role string) gf.Node {
	return gf.ComponentT(failingSiblingType, role, func(role string) gf.Node {
		recordOwnerRender(role)
		panic("document-state handoff fixture: forced sibling render failure")
	})
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

func renderHookContractScenario(struct{}) gf.Node {
	active, setActive := gf.UseState(true)
	firstUpdated, setFirstUpdated := gf.UseState(false)
	first := metadataA()
	if firstUpdated {
		first = metadataA2()
	}
	owners := gf.Empty()
	if active {
		owners = gf.ComponentT(hookSlotsType, hookSlotsProps{first: first}, renderHookSlotsProjection)
	}
	return scenarioSection(
		button("update-first-hook-slot", "Update first hook slot", func() {
			setFirstUpdated(true)
		}),
		button("release-hook-slots", "Release hook slots", func() {
			setActive(false)
		}),
		owners,
	)
}

func renderComponentContractScenario(struct{}) gf.Node {
	active, setActive := gf.UseState(true)
	updated, setUpdated := gf.UseState(false)
	value := metadataA()
	if updated {
		value = metadataA2()
	}
	projection := gf.Empty()
	if active {
		projection = gf.ComponentT(
			componentProjectionType,
			componentContractProps{
				value: value,
				children: []gf.Node{gf.El(
					"span",
					gf.Props{"data-testid": "component-contract-child"},
					gf.Text("stable child"),
				)},
			},
			renderComponentContractProjection,
		)
	}
	return scenarioSection(
		button("update-component-metadata", "Update component metadata", func() {
			setUpdated(true)
		}),
		button("release-component-owner", "Release component owner", func() {
			setActive(false)
		}),
		projection,
	)
}

func renderHandleContractScenario(struct{}) gf.Node {
	primaryActive, setPrimaryActive := gf.UseState(true)
	primaryUpdated, setPrimaryUpdated := gf.UseState(false)
	duplicateActive, setDuplicateActive := gf.UseState(false)
	duplicateConflict, setDuplicateConflict := gf.UseState(false)
	owner := gf.UseDocumentMetadataOwner()
	activeComparisonHandle = owner
	primaryValue := metadataA()
	if primaryUpdated {
		primaryValue = metadataB()
	}
	children := []gf.Node{
		button("add-handle-duplicate", "Add identical duplicate", func() {
			setDuplicateActive(true)
		}),
		button("conflict-handle-duplicate", "Conflict duplicate", func() {
			setDuplicateConflict(true)
		}),
		button("release-handle-duplicate", "Release duplicate", func() {
			setDuplicateActive(false)
		}),
		button("update-handle-primary", "Update primary", func() {
			setPrimaryUpdated(true)
		}),
		button("release-handle-primary", "Release primary", func() {
			setPrimaryActive(false)
		}),
	}
	if primaryActive {
		children = append(children, gf.Key("handle-primary", gf.ComponentT(
			handlePublicationType,
			handlePublicationProps{
				owner:  owner,
				value:  primaryValue,
				testID: "handle-primary-publication",
			},
			renderHandlePublicationProjection,
		)))
	}
	if duplicateActive {
		duplicateValue := metadataA()
		if duplicateConflict {
			duplicateValue = metadataB()
		}
		children = append(children, gf.Key("handle-duplicate-boundary", gf.ErrorBoundary(
			gf.ErrorBoundaryProps{
				Fallback: func(gf.ErrorBoundaryContext) gf.Node {
					return gf.El(
						"span",
						gf.Props{"data-testid": "handle-conflict-fallback"},
						gf.Text("conflict rejected"),
					)
				},
				Children: []gf.Node{gf.ComponentT(
					handlePublicationType,
					handlePublicationProps{
						owner:  owner,
						value:  duplicateValue,
						testID: "handle-duplicate-publication",
					},
					renderHandlePublicationProjection,
				)},
			},
		)))
	}
	return scenarioSection(children...)
}

func renderOwner(props ownerProps) gf.Node {
	recordOwnerRender(props.role)
	if props.failureBefore {
		panic("document-state handoff fixture: forced owner render failure before metadata")
	}
	children := renderCandidateOwnerProjection(activeCandidateMode, props.metadata, props.children)
	gf.UseUnmount(func() {
		recordOwnerCleanup(props.role)
	})
	if props.failureAfter {
		panic("document-state handoff fixture: forced owner render failure")
	}
	return children
}

func ownerNode(
	key string,
	role string,
	value metadata,
	failure bool,
	children ...gf.Node,
) gf.Node {
	return ownerNodeWithFailure(key, role, value, false, failure, children...)
}

func ownerNodeWithFailure(
	key string,
	role string,
	value metadata,
	failureBefore bool,
	failureAfter bool,
	children ...gf.Node,
) gf.Node {
	return gf.Key(key, gf.ComponentT(ownerType, ownerProps{
		role:          role,
		metadata:      value,
		failureBefore: failureBefore,
		failureAfter:  failureAfter,
		children:      children,
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
	title               js.Value
	description         js.Value
	unrelated           js.Value
	baseline            metadata
	current             metadata
	failNextPublication bool
}

type documentMetadataWriteFailure struct {
	message string
	cause   error
}

func (failure *documentMetadataWriteFailure) Error() string {
	if failure.cause == nil {
		return failure.message
	}
	return failure.message + ": " + failure.cause.Error()
}

func (failure *documentMetadataWriteFailure) Unwrap() error {
	return failure.cause
}

type documentMetadataWriteFailures []error

func (failures documentMetadataWriteFailures) Error() string {
	message := ""
	for _, failure := range failures {
		if failure == nil {
			continue
		}
		if message != "" {
			message += "\n"
		}
		message += failure.Error()
	}
	return message
}

func (failures documentMetadataWriteFailures) Unwrap() []error {
	return failures
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
	if adapter.failNextPublication {
		adapter.failNextPublication = false
		incrementEvidence("publicationFailures")
		panic("document-state handoff fixture: forced publication failure")
	}
	if err := writeDocumentMetadataPair(
		previous,
		value,
		adapter.writeTitle,
		adapter.writeDescription,
	); err != nil {
		panic(err)
	}
	adapter.current = value
	incrementEvidence("documentApplies")
	if previous != adapter.baseline && value == adapter.baseline {
		incrementEvidence("baselineRestorations")
	}
	appendPairEvidence("documentSnapshots", value)
}

func (adapter *documentAdapter) writeTitle(value string) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = documentMetadataWriteError("write title", recovered)
		}
	}()
	if adapter.title.Get("textContent").String() != value {
		adapter.title.Set("textContent", value)
	}
	return nil
}

func (adapter *documentAdapter) writeDescription(value string) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = documentMetadataWriteError("write description", recovered)
		}
	}()
	if adapter.description.Call("getAttribute", "content").String() != value {
		adapter.description.Call("setAttribute", "content", value)
	}
	return nil
}

func writeDocumentMetadataPair(
	previous metadata,
	next metadata,
	writeTitle func(string) error,
	writeDescription func(string) error,
) error {
	if err := writeTitle(next.Title); err != nil {
		return err
	}
	if err := writeDescription(next.Description); err != nil {
		restoreTitleError := writeTitle(previous.Title)
		restoreDescriptionError := writeDescription(previous.Description)
		failures := make(documentMetadataWriteFailures, 0, 3)
		for _, failure := range []error{err, restoreTitleError, restoreDescriptionError} {
			if failure != nil {
				failures = append(failures, failure)
			}
		}
		if len(failures) == 1 {
			return failures[0]
		}
		return failures
	}
	return nil
}

func documentMetadataWriteError(operation string, recovered any) error {
	if err, ok := recovered.(error); ok {
		return &documentMetadataWriteFailure{
			message: "document-state handoff fixture: " + operation,
			cause:   err,
		}
	}
	message := "document-state handoff fixture: " + operation
	if value, ok := recovered.(string); ok {
		message += ": " + value
	}
	return &documentMetadataWriteFailure{message: message}
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
		"publicationFailures",
	} {
		evidence.Set(name, 0)
	}
	for _, name := range []string{
		"ownershipEvents",
		"documentSnapshots",
		"ownerRenders",
		"ownerCleanups",
		"runtimeReports",
	} {
		evidence.Set(name, js.Global().Get("Array").New())
	}
	evidence.Set("snapshot", js.Global().Get("Object").New())
	evidence.Set("statistics", js.Global().Get("Object").New())
	evidence.Set("candidate", js.Global().Get("Object").New())
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
	retainGlobalFunction("goframeDocumentHandoffRefreshEvidence", func(args []js.Value) {
		updateCommittedEvidence()
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
	ownerIDs := js.Global().Get("Array").New()
	for _, ownerID := range snapshot.OwnerIDs {
		ownerIDs.Call("push", ownerID)
	}
	snapshotValue.Set("ownerIDs", ownerIDs)
	ownerTitles := js.Global().Get("Array").New()
	for _, title := range snapshot.OwnerTitles {
		ownerTitles.Call("push", title)
	}
	snapshotValue.Set("ownerTitles", ownerTitles)
	snapshotValue.Set("ownerCount", snapshot.OwnerCount)
	snapshotValue.Set("failedBoundaryCount", snapshot.FailedBoundaryCount)
	snapshotValue.Set("retainedReleaseCount", snapshot.RetainedReleaseCount)
	snapshotValue.Set("pendingPlanCount", snapshot.PendingPlanCount)
	snapshotValue.Set("pendingOwnerCount", snapshot.PendingOwnerCount)
	pendingOwnerIDs := js.Global().Get("Array").New()
	for _, ownerID := range snapshot.PendingOwnerIDs {
		pendingOwnerIDs.Call("push", ownerID)
	}
	snapshotValue.Set("pendingOwnerIDs", pendingOwnerIDs)
	pendingOwnerTitles := js.Global().Get("Array").New()
	for _, title := range snapshot.PendingOwnerTitles {
		pendingOwnerTitles.Call("push", title)
	}
	snapshotValue.Set("pendingOwnerTitles", pendingOwnerTitles)
	snapshotValue.Set("pendingFinalizations", snapshot.PendingFinalizations)
	snapshotValue.Set("batchActive", snapshot.BatchActive)
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

	candidateValue := js.Global().Get("Object").New()
	if activeComparisonHandle != nil {
		candidateValue.Set("handleID", activeComparisonHandle.ID())
		candidateValue.Set("activePublications", activeComparisonHandle.ActivePublications())
	}
	evidence().Set("candidate", candidateValue)
}

func recordOwnerRender(role string) {
	value := js.Global().Get("Object").New()
	value.Set("role", role)
	evidence().Get("ownerRenders").Call("push", value)
}

func recordOwnerCleanup(role string) {
	value := js.Global().Get("Object").New()
	value.Set("role", role)
	evidence().Get("ownerCleanups").Call("push", value)
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

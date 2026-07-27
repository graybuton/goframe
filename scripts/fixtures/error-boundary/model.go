//go:build js && wasm

package main

import (
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

var (
	protectedEffectCount         int
	protectedCleanupCount        int
	transactionAttemptPhase      = "initial"
	transactionProbe             protectedTransactionProbe
	localTransactionVersion      = "A"
	localTransactionAttemptPhase = "initial"
	localUpdateProbe             localUpdateLifecycleProbe
	localTransactionProbe        protectedTransactionProbe
	nestedFallbackProbe          nestedFallbackTransactionProbe
)

type protectedTransactionProbe struct {
	aEffectSetups              int
	aEffectCleanups            int
	aUnmountCallbacks          int
	aResourceStarts            int
	aResourceCleanups          int
	aLaterSiblingSetups        int
	attemptedBEffectSetups     int
	attemptedBUnmountCallbacks int
	attemptedBResourceStarts   int
	attemptedBResourceCleanups int
	attemptedBLaterSetups      int
	retryBEffectSetups         int
	retryBResourceStarts       int
	retryBResourceCleanups     int
	retryBLaterSetups          int
}

type localUpdateLifecycleProbe struct {
	ownerRenders                 int
	siblingRenders               int
	siblingEffectSetups          int
	siblingEffectCleanups        int
	nestedInnerOwnerRenders      int
	nestedInnerSiblingRenders    int
	nestedOuterSiblingRenders    int
	nestedInnerSiblingSetups     int
	nestedInnerSiblingCleanups   int
	nestedOuterSiblingSetups     int
	nestedOuterSiblingCleanups   int
	localTransactionOwnerRenders int
}

type nestedFallbackTransactionProbe struct {
	ownerRenders       int
	effectSetups       int
	effectCleanups     int
	unmountCallbacks   int
	resourceStarts     int
	resourceCleanups   int
	laterSiblingSetups int
}

type RiskyPanelProps struct {
	Broken bool
}

type BoundaryFallbackProps struct {
	Info  gf.ErrorInfo
	Reset func()
}

type NestedRiskyProps struct {
	Broken bool
}

type NestedBoundaryScenarioProps struct {
	Broken        bool
	FallbackCrash bool
}

type NestedFallbackTransactionScenarioProps struct {
	Broken bool
}

type NestedFallbackInitialRiskyProps struct {
	Broken bool
}

type InnerFallbackProps struct {
	Crash bool
}

type NoBoundaryRiskyProps struct {
	Broken bool
}

type TransactionOwnerProps struct {
	Version string
	Broken  bool
}

type TransactionRiskyProps struct {
	Broken bool
}

type TransactionLaterEffectProps struct {
	Version string
	Phase   string
}

type LocalStateOwnerProps struct{}

type LocalUpdateSiblingProps struct{}

type LocalInnerOwnerProps struct{}

type NestedInnerSiblingProps struct{}

type NestedOuterSiblingProps struct{}

type LocalTransactionOwnerProps struct{}

func RiskyPanel(props RiskyPanelProps) gf.Node {
	count, setCount := gf.UseState(0)
	gf.UseEffect(func() gf.Cleanup {
		protectedEffectCount++
		syncBoundaryProbe()
		return func() {
			protectedCleanupCount++
			syncBoundaryProbe()
		}
	}, gf.EveryRender())
	if props.Broken {
		panic("protected render boom")
	}
	return gf.El("section", gf.Props{"data-testid": "eb-protected"},
		gf.El("p", gf.Props{"data-testid": "eb-protected-state"}, gf.Text(gf.ToString(count))),
		gf.El("button", gf.Props{
			"data-testid": "eb-protected-increment",
			"OnClick": func() {
				setCount(count + 1)
			},
		}, gf.Text("Increment protected state")),
	)
}

func BoundaryFallback(props BoundaryFallbackProps) gf.Node {
	return gf.El("section", gf.Props{"data-testid": "eb-fallback"},
		gf.El("p", gf.Props{"data-testid": "eb-error-component"}, gf.Text(props.Info.Component)),
		gf.El("p", gf.Props{"data-testid": "eb-error-operation"}, gf.Text(props.Info.Operation)),
		gf.El("button", gf.Props{
			"data-testid": "eb-retry",
			"OnClick":     props.Reset,
		}, gf.Text("Retry")),
	)
}

func makeBoundaryFallback(setBroken func(bool)) func(gf.ErrorBoundaryContext) gf.Node {
	return func(ctx gf.ErrorBoundaryContext) gf.Node {
		return BoundaryFallback(BoundaryFallbackProps{
			Info: ctx.Info,
			Reset: func() {
				setBroken(false)
				ctx.Reset()
			},
		})
	}
}

func NestedRisky(props NestedRiskyProps) gf.Node {
	if props.Broken {
		panic("nested render boom")
	}
	return gf.El("section", gf.Props{"data-testid": "eb-nested-protected"}, gf.Text("nested healthy"))
}

func NestedBoundaryScenario(props NestedBoundaryScenarioProps) gf.Node {
	return gf.ErrorBoundary(gf.ErrorBoundaryProps{
		Fallback: OuterFallback,
		Children: []gf.Node{
			gf.ErrorBoundary(gf.ErrorBoundaryProps{
				Fallback: func(gf.ErrorBoundaryContext) gf.Node {
					return gf.Component("InnerFallback", InnerFallbackProps{Crash: props.FallbackCrash}, InnerFallback)
				},
				Children: []gf.Node{
					gf.Component("NestedRisky", NestedRiskyProps{Broken: props.Broken}, NestedRisky),
				},
			}),
		},
	})
}

func InnerFallback(props InnerFallbackProps) gf.Node {
	if props.Crash {
		panic("inner fallback boom")
	}
	return gf.El("section", gf.Props{"data-testid": "eb-nested-inner-fallback"}, gf.Text("inner fallback"))
}

func OuterFallback(gf.ErrorBoundaryContext) gf.Node {
	return gf.El("section", gf.Props{"data-testid": "eb-nested-outer-fallback"}, gf.Text("outer fallback"))
}

func NestedFallbackTransactionScenario(props NestedFallbackTransactionScenarioProps) gf.Node {
	return gf.ErrorBoundary(gf.ErrorBoundaryProps{
		Fallback: nestedFallbackTransactionOuterFallback,
		Children: []gf.Node{
			gf.ErrorBoundary(gf.ErrorBoundaryProps{
				Fallback: func(gf.ErrorBoundaryContext) gf.Node {
					return gf.Component(
						"FallbackLifecycleOwner",
						struct{}{},
						FallbackLifecycleOwner,
					)
				},
				Children: []gf.Node{
					gf.Component(
						"InitialRiskyChild",
						NestedFallbackInitialRiskyProps{Broken: props.Broken},
						NestedFallbackInitialRiskyChild,
					),
				},
			}),
		},
	})
}

func NestedFallbackInitialRiskyChild(props NestedFallbackInitialRiskyProps) gf.Node {
	if props.Broken {
		panic("nested fallback initial protected boom")
	}
	return gf.El(
		"section",
		gf.Props{"data-testid": "eb-nested-fallback-protected"},
		gf.Text("nested fallback protected"),
	)
}

func FallbackLifecycleOwner(struct{}) gf.Node {
	nestedFallbackProbe.ownerRenders++
	syncBoundaryProbe()
	gf.UseEffect(func() gf.Cleanup {
		nestedFallbackProbe.effectSetups++
		syncBoundaryProbe()
		return func() {
			nestedFallbackProbe.effectCleanups++
			syncBoundaryProbe()
		}
	})
	gf.UseUnmount(func() {
		nestedFallbackProbe.unmountCallbacks++
		syncBoundaryProbe()
	})
	_, _ = gf.UseResource("nested-fallback", func(
		key string,
		resolve func(string),
		reject func(error),
	) gf.Cleanup {
		nestedFallbackProbe.resourceStarts++
		syncBoundaryProbe()
		return func() {
			nestedFallbackProbe.resourceCleanups++
			syncBoundaryProbe()
		}
	})
	return gf.El(
		"section",
		gf.Props{"data-testid": "eb-nested-fallback-owner"},
		gf.Component(
			"FallbackRiskyDescendant",
			struct{}{},
			FallbackRiskyDescendant,
		),
		gf.Component(
			"FallbackLaterEffect",
			struct{}{},
			FallbackLaterEffect,
		),
	)
}

func FallbackRiskyDescendant(struct{}) gf.Node {
	panic("nested fallback descendant boom")
}

func FallbackLaterEffect(struct{}) gf.Node {
	gf.UseEffect(func() gf.Cleanup {
		nestedFallbackProbe.laterSiblingSetups++
		syncBoundaryProbe()
		return nil
	})
	return gf.Empty()
}

func nestedFallbackTransactionOuterFallback(gf.ErrorBoundaryContext) gf.Node {
	return gf.El(
		"section",
		gf.Props{"data-testid": "eb-nested-fallback-outer"},
		gf.Text("nested fallback outer"),
	)
}

func NoBoundaryRisky(props NoBoundaryRiskyProps) gf.Node {
	if props.Broken {
		panic("no boundary boom")
	}
	return gf.El("section", gf.Props{"data-testid": "eb-no-boundary-healthy"}, gf.Text("no boundary healthy"))
}

func TransactionOwner(props TransactionOwnerProps) gf.Node {
	version := props.Version
	phase := transactionAttemptPhase
	gf.UseEffect(func() gf.Cleanup {
		recordTransactionEffectSetup(version, phase)
		return func() {
			recordTransactionEffectCleanup(version, phase)
		}
	}, gf.Deps(version))
	gf.UseUnmount(func() {
		recordTransactionUnmount(version, phase)
	})
	_, _ = gf.UseResource(version, func(
		key string,
		resolve func(string),
		reject func(error),
	) gf.Cleanup {
		recordTransactionResourceStart(key, phase)
		return func() {
			recordTransactionResourceCleanup(key, phase)
		}
	})
	return gf.El("section", gf.Props{"data-testid": "eb-transaction-owner"},
		gf.El("p", gf.Props{"data-testid": "eb-transaction-version"}, gf.Text(version)),
		gf.Component("TransactionRiskyDescendant", TransactionRiskyProps{
			Broken: props.Broken,
		}, TransactionRiskyDescendant),
		gf.Component("TransactionLaterEffect", TransactionLaterEffectProps{
			Version: version,
			Phase:   phase,
		}, TransactionLaterEffect),
	)
}

func TransactionRiskyDescendant(props TransactionRiskyProps) gf.Node {
	if props.Broken {
		panic("protected transaction descendant boom")
	}
	return gf.El("p", gf.Props{"data-testid": "eb-transaction-descendant"}, gf.Text("healthy"))
}

func TransactionLaterEffect(props TransactionLaterEffectProps) gf.Node {
	gf.UseEffect(func() gf.Cleanup {
		recordTransactionLaterSetup(props.Version, props.Phase)
		return nil
	}, gf.Deps(props.Version))
	return gf.Empty()
}

func LocalStateOwner(LocalStateOwnerProps) gf.Node {
	localUpdateProbe.ownerRenders++
	syncBoundaryProbe()
	count, setCount := gf.UseState(0)
	return gf.El("section", gf.Props{"data-testid": "eb-local-owner"},
		gf.El("p", gf.Props{"data-testid": "eb-local-owner-state"}, gf.Text(gf.ToString(count))),
		gf.El("button", gf.Props{
			"data-testid": "eb-local-owner-update",
			"OnClick": func() {
				setCount(count + 1)
			},
		}, gf.Text("Update local owner")),
	)
}

func LocalUpdateSibling(LocalUpdateSiblingProps) gf.Node {
	localUpdateProbe.siblingRenders++
	syncBoundaryProbe()
	gf.UseEffect(func() gf.Cleanup {
		localUpdateProbe.siblingEffectSetups++
		syncBoundaryProbe()
		return func() {
			localUpdateProbe.siblingEffectCleanups++
			syncBoundaryProbe()
		}
	}, gf.EveryRender())
	return gf.El("p", gf.Props{"data-testid": "eb-local-sibling"}, gf.Text("local sibling"))
}

func LocalInnerOwner(LocalInnerOwnerProps) gf.Node {
	localUpdateProbe.nestedInnerOwnerRenders++
	syncBoundaryProbe()
	count, setCount := gf.UseState(0)
	return gf.El("section", gf.Props{"data-testid": "eb-nested-local-owner"},
		gf.El("p", gf.Props{"data-testid": "eb-nested-local-owner-state"}, gf.Text(gf.ToString(count))),
		gf.El("button", gf.Props{
			"data-testid": "eb-nested-local-owner-update",
			"OnClick": func() {
				setCount(count + 1)
			},
		}, gf.Text("Update nested local owner")),
	)
}

func NestedInnerSibling(NestedInnerSiblingProps) gf.Node {
	localUpdateProbe.nestedInnerSiblingRenders++
	syncBoundaryProbe()
	gf.UseEffect(func() gf.Cleanup {
		localUpdateProbe.nestedInnerSiblingSetups++
		syncBoundaryProbe()
		return func() {
			localUpdateProbe.nestedInnerSiblingCleanups++
			syncBoundaryProbe()
		}
	}, gf.EveryRender())
	return gf.El("p", gf.Props{"data-testid": "eb-nested-inner-sibling"}, gf.Text("nested inner sibling"))
}

func NestedOuterSibling(NestedOuterSiblingProps) gf.Node {
	localUpdateProbe.nestedOuterSiblingRenders++
	syncBoundaryProbe()
	gf.UseEffect(func() gf.Cleanup {
		localUpdateProbe.nestedOuterSiblingSetups++
		syncBoundaryProbe()
		return func() {
			localUpdateProbe.nestedOuterSiblingCleanups++
			syncBoundaryProbe()
		}
	}, gf.EveryRender())
	return gf.El("p", gf.Props{"data-testid": "eb-nested-outer-sibling"}, gf.Text("nested outer sibling"))
}

func LocalTransactionOwner(LocalTransactionOwnerProps) gf.Node {
	localUpdateProbe.localTransactionOwnerRenders++
	syncBoundaryProbe()
	version, setVersion := gf.UseState(localTransactionVersion)
	phase := localTransactionAttemptPhase
	gf.UseEffect(func() gf.Cleanup {
		recordLocalTransactionEffectSetup(version, phase)
		return func() {
			recordLocalTransactionEffectCleanup(version, phase)
		}
	}, gf.Deps(version))
	gf.UseUnmount(func() {
		recordLocalTransactionUnmount(version, phase)
	})
	_, _ = gf.UseResource(version, func(
		key string,
		resolve func(string),
		reject func(error),
	) gf.Cleanup {
		recordLocalTransactionResourceStart(key, phase)
		return func() {
			recordLocalTransactionResourceCleanup(key, phase)
		}
	})
	return gf.El("section", gf.Props{"data-testid": "eb-local-transaction-owner"},
		gf.El("p", gf.Props{"data-testid": "eb-local-transaction-version"}, gf.Text(version)),
		gf.El("button", gf.Props{
			"data-testid": "eb-local-transaction-trigger",
			"OnClick": func() {
				localTransactionVersion = "B"
				localTransactionAttemptPhase = "attempt"
				setVersion("B")
			},
		}, gf.Text("Trigger local transaction error")),
		gf.Component("LocalTransactionRiskyDescendant", TransactionRiskyProps{
			Broken: version == "B" && phase == "attempt",
		}, TransactionRiskyDescendant),
		gf.Component("LocalTransactionLaterEffect", TransactionLaterEffectProps{
			Version: version,
			Phase:   phase,
		}, LocalTransactionLaterEffect),
	)
}

func LocalTransactionLaterEffect(props TransactionLaterEffectProps) gf.Node {
	gf.UseEffect(func() gf.Cleanup {
		recordLocalTransactionLaterSetup(props.Version, props.Phase)
		return nil
	}, gf.Deps(props.Version))
	return gf.Empty()
}

func localUpdateFallback(gf.ErrorBoundaryContext) gf.Node {
	return gf.El("section", gf.Props{"data-testid": "eb-local-unexpected-fallback"},
		gf.Text("unexpected local update fallback"),
	)
}

func nestedLocalUpdateFallback(gf.ErrorBoundaryContext) gf.Node {
	return gf.El("section", gf.Props{"data-testid": "eb-nested-local-unexpected-fallback"},
		gf.Text("unexpected nested local update fallback"),
	)
}

func localTransactionFallback(ctx gf.ErrorBoundaryContext) gf.Node {
	return gf.El("section", gf.Props{"data-testid": "eb-local-transaction-fallback"},
		gf.El("p", gf.Props{"data-testid": "eb-local-transaction-error-component"}, gf.Text(ctx.Info.Component)),
		gf.El("button", gf.Props{
			"data-testid": "eb-local-transaction-retry",
			"OnClick": func() {
				localTransactionAttemptPhase = "retry"
				ctx.Reset()
			},
		}, gf.Text("Retry local transaction")),
	)
}

func makeTransactionFallback(setBroken func(bool)) func(gf.ErrorBoundaryContext) gf.Node {
	return func(ctx gf.ErrorBoundaryContext) gf.Node {
		return gf.El("section", gf.Props{"data-testid": "eb-transaction-fallback"},
			gf.El("p", gf.Props{"data-testid": "eb-transaction-error-component"}, gf.Text(ctx.Info.Component)),
			gf.El("button", gf.Props{
				"data-testid": "eb-transaction-retry",
				"OnClick": func() {
					transactionAttemptPhase = "retry"
					setBroken(false)
					ctx.Reset()
				},
			}, gf.Text("Retry transaction")),
		)
	}
}

func recordTransactionEffectSetup(version, phase string) {
	switch {
	case version == "A":
		transactionProbe.aEffectSetups++
	case phase == "attempt":
		transactionProbe.attemptedBEffectSetups++
	case phase == "retry":
		transactionProbe.retryBEffectSetups++
	}
	syncBoundaryProbe()
}

func recordTransactionEffectCleanup(version, phase string) {
	if version == "A" {
		transactionProbe.aEffectCleanups++
	}
	syncBoundaryProbe()
}

func recordTransactionUnmount(version, phase string) {
	switch {
	case version == "A":
		transactionProbe.aUnmountCallbacks++
	case phase == "attempt":
		transactionProbe.attemptedBUnmountCallbacks++
	}
	syncBoundaryProbe()
}

func recordTransactionResourceStart(version, phase string) {
	switch {
	case version == "A":
		transactionProbe.aResourceStarts++
	case phase == "attempt":
		transactionProbe.attemptedBResourceStarts++
	case phase == "retry":
		transactionProbe.retryBResourceStarts++
	}
	syncBoundaryProbe()
}

func recordTransactionResourceCleanup(version, phase string) {
	switch {
	case version == "A":
		transactionProbe.aResourceCleanups++
	case phase == "attempt":
		transactionProbe.attemptedBResourceCleanups++
	case phase == "retry":
		transactionProbe.retryBResourceCleanups++
	}
	syncBoundaryProbe()
}

func recordTransactionLaterSetup(version, phase string) {
	switch {
	case version == "A":
		transactionProbe.aLaterSiblingSetups++
	case phase == "attempt":
		transactionProbe.attemptedBLaterSetups++
	case phase == "retry":
		transactionProbe.retryBLaterSetups++
	}
	syncBoundaryProbe()
}

func recordLocalTransactionEffectSetup(version, phase string) {
	switch {
	case version == "A":
		localTransactionProbe.aEffectSetups++
	case phase == "attempt":
		localTransactionProbe.attemptedBEffectSetups++
	case phase == "retry":
		localTransactionProbe.retryBEffectSetups++
	}
	syncBoundaryProbe()
}

func recordLocalTransactionEffectCleanup(version, phase string) {
	if version == "A" {
		localTransactionProbe.aEffectCleanups++
	}
	syncBoundaryProbe()
}

func recordLocalTransactionUnmount(version, phase string) {
	switch {
	case version == "A":
		localTransactionProbe.aUnmountCallbacks++
	case phase == "attempt":
		localTransactionProbe.attemptedBUnmountCallbacks++
	}
	syncBoundaryProbe()
}

func recordLocalTransactionResourceStart(version, phase string) {
	switch {
	case version == "A":
		localTransactionProbe.aResourceStarts++
	case phase == "attempt":
		localTransactionProbe.attemptedBResourceStarts++
	case phase == "retry":
		localTransactionProbe.retryBResourceStarts++
	}
	syncBoundaryProbe()
}

func recordLocalTransactionResourceCleanup(version, phase string) {
	switch {
	case version == "A":
		localTransactionProbe.aResourceCleanups++
	case phase == "attempt":
		localTransactionProbe.attemptedBResourceCleanups++
	case phase == "retry":
		localTransactionProbe.retryBResourceCleanups++
	}
	syncBoundaryProbe()
}

func recordLocalTransactionLaterSetup(version, phase string) {
	switch {
	case version == "A":
		localTransactionProbe.aLaterSiblingSetups++
	case phase == "attempt":
		localTransactionProbe.attemptedBLaterSetups++
	case phase == "retry":
		localTransactionProbe.retryBLaterSetups++
	}
	syncBoundaryProbe()
}

func initBoundaryProbe() {
	protectedEffectCount = 0
	protectedCleanupCount = 0
	transactionAttemptPhase = "initial"
	transactionProbe = protectedTransactionProbe{}
	localTransactionVersion = "A"
	localTransactionAttemptPhase = "initial"
	localUpdateProbe = localUpdateLifecycleProbe{}
	localTransactionProbe = protectedTransactionProbe{}
	nestedFallbackProbe = nestedFallbackTransactionProbe{}
	js.Global().Set("goframeErrorBoundaryReports", js.Global().Get("Array").New())
	syncBoundaryProbe()
}

func syncBoundaryProbe() {
	js.Global().Set("goframeErrorBoundaryEffectCount", protectedEffectCount)
	js.Global().Set("goframeErrorBoundaryCleanupCount", protectedCleanupCount)
	probe := js.Global().Get("Object").New()
	probe.Set("aEffectSetups", transactionProbe.aEffectSetups)
	probe.Set("aEffectCleanups", transactionProbe.aEffectCleanups)
	probe.Set("aUnmountCallbacks", transactionProbe.aUnmountCallbacks)
	probe.Set("aResourceStarts", transactionProbe.aResourceStarts)
	probe.Set("aResourceCleanups", transactionProbe.aResourceCleanups)
	probe.Set("aLaterSiblingSetups", transactionProbe.aLaterSiblingSetups)
	probe.Set("attemptedBEffectSetups", transactionProbe.attemptedBEffectSetups)
	probe.Set("attemptedBUnmountCallbacks", transactionProbe.attemptedBUnmountCallbacks)
	probe.Set("attemptedBResourceStarts", transactionProbe.attemptedBResourceStarts)
	probe.Set("attemptedBResourceCleanups", transactionProbe.attemptedBResourceCleanups)
	probe.Set("attemptedBLaterSetups", transactionProbe.attemptedBLaterSetups)
	probe.Set("retryBEffectSetups", transactionProbe.retryBEffectSetups)
	probe.Set("retryBResourceStarts", transactionProbe.retryBResourceStarts)
	probe.Set("retryBResourceCleanups", transactionProbe.retryBResourceCleanups)
	probe.Set("retryBLaterSetups", transactionProbe.retryBLaterSetups)
	js.Global().Set("goframeProtectedTransactionProbe", probe)

	local := js.Global().Get("Object").New()
	local.Set("ownerRenders", localUpdateProbe.ownerRenders)
	local.Set("siblingRenders", localUpdateProbe.siblingRenders)
	local.Set("siblingEffectSetups", localUpdateProbe.siblingEffectSetups)
	local.Set("siblingEffectCleanups", localUpdateProbe.siblingEffectCleanups)
	local.Set("nestedInnerOwnerRenders", localUpdateProbe.nestedInnerOwnerRenders)
	local.Set("nestedInnerSiblingRenders", localUpdateProbe.nestedInnerSiblingRenders)
	local.Set("nestedOuterSiblingRenders", localUpdateProbe.nestedOuterSiblingRenders)
	local.Set("nestedInnerSiblingSetups", localUpdateProbe.nestedInnerSiblingSetups)
	local.Set("nestedInnerSiblingCleanups", localUpdateProbe.nestedInnerSiblingCleanups)
	local.Set("nestedOuterSiblingSetups", localUpdateProbe.nestedOuterSiblingSetups)
	local.Set("nestedOuterSiblingCleanups", localUpdateProbe.nestedOuterSiblingCleanups)
	local.Set("localTransactionOwnerRenders", localUpdateProbe.localTransactionOwnerRenders)
	js.Global().Set("goframeLocalUpdateProbe", local)

	localTransaction := js.Global().Get("Object").New()
	localTransaction.Set("aEffectSetups", localTransactionProbe.aEffectSetups)
	localTransaction.Set("aEffectCleanups", localTransactionProbe.aEffectCleanups)
	localTransaction.Set("aUnmountCallbacks", localTransactionProbe.aUnmountCallbacks)
	localTransaction.Set("aResourceStarts", localTransactionProbe.aResourceStarts)
	localTransaction.Set("aResourceCleanups", localTransactionProbe.aResourceCleanups)
	localTransaction.Set("aLaterSiblingSetups", localTransactionProbe.aLaterSiblingSetups)
	localTransaction.Set("attemptedBEffectSetups", localTransactionProbe.attemptedBEffectSetups)
	localTransaction.Set("attemptedBUnmountCallbacks", localTransactionProbe.attemptedBUnmountCallbacks)
	localTransaction.Set("attemptedBResourceStarts", localTransactionProbe.attemptedBResourceStarts)
	localTransaction.Set("attemptedBResourceCleanups", localTransactionProbe.attemptedBResourceCleanups)
	localTransaction.Set("attemptedBLaterSetups", localTransactionProbe.attemptedBLaterSetups)
	localTransaction.Set("retryBEffectSetups", localTransactionProbe.retryBEffectSetups)
	localTransaction.Set("retryBResourceStarts", localTransactionProbe.retryBResourceStarts)
	localTransaction.Set("retryBResourceCleanups", localTransactionProbe.retryBResourceCleanups)
	localTransaction.Set("retryBLaterSetups", localTransactionProbe.retryBLaterSetups)
	js.Global().Set("goframeLocalTransactionProbe", localTransaction)

	nestedFallback := js.Global().Get("Object").New()
	nestedFallback.Set("ownerRenders", nestedFallbackProbe.ownerRenders)
	nestedFallback.Set("effectSetups", nestedFallbackProbe.effectSetups)
	nestedFallback.Set("effectCleanups", nestedFallbackProbe.effectCleanups)
	nestedFallback.Set("unmountCallbacks", nestedFallbackProbe.unmountCallbacks)
	nestedFallback.Set("resourceStarts", nestedFallbackProbe.resourceStarts)
	nestedFallback.Set("resourceCleanups", nestedFallbackProbe.resourceCleanups)
	nestedFallback.Set("laterSiblingSetups", nestedFallbackProbe.laterSiblingSetups)
	js.Global().Set("goframeNestedFallbackTransactionProbe", nestedFallback)
}

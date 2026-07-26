//go:build js && wasm

package main

import (
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

var (
	protectedEffectCount    int
	protectedCleanupCount   int
	transactionAttemptPhase = "initial"
	transactionProbe        protectedTransactionProbe
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

func initBoundaryProbe() {
	protectedEffectCount = 0
	protectedCleanupCount = 0
	transactionAttemptPhase = "initial"
	transactionProbe = protectedTransactionProbe{}
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
}

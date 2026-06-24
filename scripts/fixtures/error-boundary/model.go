//go:build js && wasm

package main

import (
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

var (
	protectedEffectCount  int
	protectedCleanupCount int
)

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

func initBoundaryProbe() {
	protectedEffectCount = 0
	protectedCleanupCount = 0
	js.Global().Set("goframeErrorBoundaryReports", js.Global().Get("Array").New())
	syncBoundaryProbe()
}

func syncBoundaryProbe() {
	js.Global().Set("goframeErrorBoundaryEffectCount", protectedEffectCount)
	js.Global().Set("goframeErrorBoundaryCleanupCount", protectedCleanupCount)
}

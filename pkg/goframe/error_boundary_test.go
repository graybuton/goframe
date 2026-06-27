package goframe

import "testing"

func TestErrorBoundaryCapturesDescendantRenderPanic(t *testing.T) {
	resetRuntimeBoundaryTestState()
	errors := captureRuntimeErrors(t)
	var fallbackInfo ErrorInfo
	boundary := testErrorBoundaryInstance("route-a", func(ctx ErrorBoundaryContext) Node {
		fallbackInfo = ctx.Info
		return Text("fallback")
	}, nil)
	renderComponentInstance(boundary)

	child := testComponentInstanceWithParent("Risky", boundary, func() Node {
		panic("render boom")
	})
	rendered := renderComponentInstance(child)

	if _, ok := rendered.(EmptyNode); !ok {
		t.Fatalf("failed render = %#v, want EmptyNode", rendered)
	}
	requireRuntimeError(t, errors(), ErrorPhaseRender, "Risky", "component render", "render boom")
	if len(errors()) != 1 {
		t.Fatalf("runtime reports = %d, want 1", len(errors()))
	}
	if boundary.errorBoundary.phase == errorBoundaryProtected {
		t.Fatal("boundary did not enter failed state")
	}

	fallback := requireKeyedNode(t, renderComponentInstance(boundary))
	if fallback.Key != errorBoundaryFallbackKey(boundary.errorBoundary.generation) {
		t.Fatalf("fallback key = %q", fallback.Key)
	}
	if got := fallback.Node.(TextNode).Value; got != "fallback" {
		t.Fatalf("fallback text = %q, want fallback", got)
	}
	if fallbackInfo.Panic != "render boom" || fallbackInfo.Component != "Risky" {
		t.Fatalf("fallback info = %#v", fallbackInfo)
	}
	if len(errors()) != 1 {
		t.Fatalf("fallback rerender reported original error again: %#v", errors())
	}
}

func TestErrorBoundaryFirstErrorWinsUntilReset(t *testing.T) {
	resetRuntimeBoundaryTestState()
	boundary := testErrorBoundaryInstance("", func(ctx ErrorBoundaryContext) Node {
		return Text(ToString(ctx.Info.Panic))
	}, nil)
	renderComponentInstance(boundary)

	renderComponentInstance(testComponentInstanceWithParent("First", boundary, func() Node {
		panic("first")
	}))
	renderComponentInstance(testComponentInstanceWithParent("Second", boundary, func() Node {
		panic("second")
	}))

	if got := boundary.errorBoundary.info.Panic; got != "first" {
		t.Fatalf("stored panic = %v, want first", got)
	}
}

func TestErrorBoundaryCapturedPhaseRemainsEligibleBeforeFallback(t *testing.T) {
	resetRuntimeBoundaryTestState()
	errors := captureRuntimeErrors(t)
	outer := testErrorBoundaryInstance("", func(ErrorBoundaryContext) Node {
		return Text("outer")
	}, nil)
	renderComponentInstance(outer)
	inner := testErrorBoundaryInstanceWithParent(outer, "", func(ErrorBoundaryContext) Node {
		return Text("inner")
	}, nil)
	renderComponentInstance(inner)

	renderComponentInstance(testComponentInstanceWithParent("FirstRisky", inner, func() Node {
		panic("first protected")
	}))
	if inner.errorBoundary.phase != errorBoundaryCaptured {
		t.Fatalf("inner phase after first capture = %d, want captured", inner.errorBoundary.phase)
	}
	renderComponentInstance(testComponentInstanceWithParent("SecondRisky", inner, func() Node {
		panic("second protected")
	}))

	if got := inner.errorBoundary.info.Panic; got != "first protected" {
		t.Fatalf("inner original panic = %v, want first protected", got)
	}
	if outer.errorBoundary.phase != errorBoundaryProtected {
		t.Fatal("captured inner boundary should remain eligible before fallback activation")
	}
	if len(errors()) != 2 {
		t.Fatalf("runtime reports = %d, want both protected panics: %#v", len(errors()), errors())
	}
}

func TestNestedErrorBoundaryNearestWinsAndFallbackPanicBubbles(t *testing.T) {
	resetRuntimeBoundaryTestState()
	outer := testErrorBoundaryInstance("", func(ErrorBoundaryContext) Node {
		return Text("outer")
	}, nil)
	renderComponentInstance(outer)
	inner := testErrorBoundaryInstanceWithParent(outer, "", func(ErrorBoundaryContext) Node {
		return Text("inner")
	}, nil)
	renderComponentInstance(inner)

	renderComponentInstance(testComponentInstanceWithParent("Risky", inner, func() Node {
		panic("inner child")
	}))

	if inner.errorBoundary.phase == errorBoundaryProtected {
		t.Fatal("inner boundary should catch nearest child")
	}
	if outer.errorBoundary.phase != errorBoundaryProtected {
		t.Fatal("outer boundary should stay healthy for inner child error")
	}

	inner.node = ErrorBoundary(ErrorBoundaryProps{
		Fallback: func(ErrorBoundaryContext) Node {
			panic("fallback boom")
		},
	}).(ComponentNode)
	rendered := renderComponentInstance(inner)
	if _, ok := rendered.(EmptyNode); !ok {
		t.Fatalf("inner fallback panic render = %#v, want EmptyNode", rendered)
	}
	if outer.errorBoundary.phase == errorBoundaryProtected {
		t.Fatal("outer boundary should catch inner fallback panic")
	}
	if got := outer.errorBoundary.info.Component; got != "ErrorBoundary" {
		t.Fatalf("outer captured component = %q, want ErrorBoundary", got)
	}
}

func TestErrorBoundaryFallbackComponentPanicSkipsDisplayingBoundary(t *testing.T) {
	resetRuntimeBoundaryTestState()
	errors := captureRuntimeErrors(t)
	outer := testErrorBoundaryInstance("", func(ErrorBoundaryContext) Node {
		return Text("outer")
	}, nil)
	renderComponentInstance(outer)
	inner := testErrorBoundaryInstanceWithParent(outer, "", func(ErrorBoundaryContext) Node {
		return Component("FallbackExploder", struct{}{}, func(struct{}) Node {
			panic("fallback component boom")
		})
	}, nil)
	renderComponentInstance(inner)

	renderComponentInstance(testComponentInstanceWithParent("Risky", inner, func() Node {
		panic("protected boom")
	}))
	if inner.errorBoundary.phase == errorBoundaryProtected {
		t.Fatal("inner boundary should capture protected child before fallback")
	}
	if got := inner.errorBoundary.info.Panic; got != "protected boom" {
		t.Fatalf("inner original panic = %v, want protected boom", got)
	}
	if outer.errorBoundary.phase != errorBoundaryProtected {
		t.Fatal("outer should stay healthy after inner protected child error")
	}

	fallback := requireKeyedNode(t, renderComponentInstance(inner))
	fallbackComponent, ok := fallback.Node.(ComponentNode)
	if !ok {
		t.Fatalf("inner fallback node = %#v, want ComponentNode", fallback.Node)
	}
	rendered := renderComponentInstance(newComponentInstance(fallbackComponent, "", inner, nil))
	if _, ok := rendered.(EmptyNode); !ok {
		t.Fatalf("fallback component panic render = %#v, want EmptyNode", rendered)
	}

	if got := inner.errorBoundary.info.Panic; got != "protected boom" {
		t.Fatalf("inner incident was replaced by fallback panic: %v", got)
	}
	if outer.errorBoundary.phase == errorBoundaryProtected {
		t.Fatal("outer boundary should capture fallback component panic")
	}
	if got := outer.errorBoundary.info.Component; got != "FallbackExploder" {
		t.Fatalf("outer captured component = %q, want FallbackExploder", got)
	}
	if len(errors()) != 2 {
		t.Fatalf("runtime reports = %d, want exactly 2: %#v", len(errors()), errors())
	}
	renderComponentInstance(outer)
	renderComponentInstance(inner)
	if len(errors()) != 2 {
		t.Fatalf("rerender without new panic changed reports to %d: %#v", len(errors()), errors())
	}
}

func TestErrorBoundaryFallbackComponentPanicWithoutOuterDoesNotSelfCapture(t *testing.T) {
	resetRuntimeBoundaryTestState()
	errors := captureRuntimeErrors(t)
	boundary := testErrorBoundaryInstance("", func(ErrorBoundaryContext) Node {
		return Component("FallbackExploder", struct{}{}, func(struct{}) Node {
			panic("fallback component boom")
		})
	}, nil)
	renderComponentInstance(boundary)
	renderComponentInstance(testComponentInstanceWithParent("Risky", boundary, func() Node {
		panic("protected boom")
	}))

	fallback := requireKeyedNode(t, renderComponentInstance(boundary))
	if boundary.errorBoundary.phase != errorBoundaryFallback {
		t.Fatalf("boundary phase = %d, want fallback", boundary.errorBoundary.phase)
	}
	fallbackComponent, ok := fallback.Node.(ComponentNode)
	if !ok {
		t.Fatalf("fallback node = %#v, want ComponentNode", fallback.Node)
	}
	renderComponentInstance(newComponentInstance(fallbackComponent, "", boundary, nil))

	if got := boundary.errorBoundary.info.Panic; got != "protected boom" {
		t.Fatalf("boundary self-captured fallback panic and replaced incident with %v", got)
	}
	if boundary.errorBoundary.phase != errorBoundaryFallback {
		t.Fatalf("boundary phase after fallback panic = %d, want fallback", boundary.errorBoundary.phase)
	}
	if len(errors()) != 2 {
		t.Fatalf("runtime reports = %d, want protected plus fallback panics: %#v", len(errors()), errors())
	}
}

func TestErrorBoundaryDoesNotCatchRuntimeInvariantPanic(t *testing.T) {
	resetRuntimeBoundaryTestState()
	boundary := testErrorBoundaryInstance("", func(ErrorBoundaryContext) Node {
		return Text("fallback")
	}, nil)
	renderComponentInstance(boundary)

	child := testComponentInstanceWithParent("Invariant", boundary, func() Node {
		panic("goframe: invariant")
	})
	assertPanic(t, "goframe: invariant", func() {
		renderComponentInstance(child)
	})
	if boundary.errorBoundary.phase != errorBoundaryProtected {
		t.Fatal("boundary should not catch runtime invariant panic")
	}
}

func TestErrorBoundaryManualResetRemountsProtectedSubtree(t *testing.T) {
	resetRuntimeBoundaryTestState()
	var reset func()
	boundary := testErrorBoundaryInstance("", func(ctx ErrorBoundaryContext) Node {
		reset = ctx.Reset
		return Text("fallback")
	}, nil)
	initial := requireKeyedNode(t, renderComponentInstance(boundary))

	renderComponentInstance(testComponentInstanceWithParent("Risky", boundary, func() Node {
		panic("boom")
	}))
	renderComponentInstance(boundary)
	reset()
	reset()

	if boundary.errorBoundary.phase != errorBoundaryProtected {
		t.Fatal("boundary stayed failed after reset")
	}
	if !boundary.dirty {
		t.Fatal("reset should mark boundary dirty")
	}
	next := requireKeyedNode(t, renderComponentInstance(boundary))
	if initial.Key == next.Key {
		t.Fatalf("protected key did not advance after reset: %q", next.Key)
	}

	deactivateComponent(boundary)
	reset()
}

func TestErrorBoundaryResetAllowsNewIncident(t *testing.T) {
	resetRuntimeBoundaryTestState()
	errors := captureRuntimeErrors(t)
	var reset func()
	boundary := testErrorBoundaryInstance("", func(ctx ErrorBoundaryContext) Node {
		reset = ctx.Reset
		return Text("fallback")
	}, nil)
	renderComponentInstance(boundary)

	renderComponentInstance(testComponentInstanceWithParent("RiskyFirst", boundary, func() Node {
		panic("first")
	}))
	renderComponentInstance(boundary)
	if boundary.errorBoundary.phase != errorBoundaryFallback || reset == nil {
		t.Fatalf("boundary phase=%d reset nil=%v, want fallback with reset", boundary.errorBoundary.phase, reset == nil)
	}

	reset()
	renderComponentInstance(boundary)
	if boundary.errorBoundary.phase != errorBoundaryProtected {
		t.Fatalf("boundary phase after reset = %d, want protected", boundary.errorBoundary.phase)
	}

	renderComponentInstance(testComponentInstanceWithParent("RiskySecond", boundary, func() Node {
		panic("second")
	}))
	renderComponentInstance(boundary)

	if got := boundary.errorBoundary.info.Panic; got != "second" {
		t.Fatalf("captured panic after reset = %v, want second", got)
	}
	if len(errors()) != 2 {
		t.Fatalf("runtime reports = %d, want first and second incidents: %#v", len(errors()), errors())
	}
	requireRuntimeError(t, errors(), ErrorPhaseRender, "RiskyFirst", "component render", "first")
	requireRuntimeError(t, errors(), ErrorPhaseRender, "RiskySecond", "component render", "second")
}

func TestErrorBoundaryResetKeyClearsFailedBoundaryOnly(t *testing.T) {
	resetRuntimeBoundaryTestState()
	boundary := testErrorBoundaryInstance("a", func(ErrorBoundaryContext) Node {
		return Text("fallback")
	}, nil)
	initial := requireKeyedNode(t, renderComponentInstance(boundary))

	boundary.node = ErrorBoundary(ErrorBoundaryProps{
		ResetKey: "b",
		Fallback: func(ErrorBoundaryContext) Node { return Text("fallback") },
	}).(ComponentNode)
	healthy := requireKeyedNode(t, renderComponentInstance(boundary))
	if healthy.Key != initial.Key {
		t.Fatalf("healthy ResetKey change remounted subtree: %q -> %q", initial.Key, healthy.Key)
	}

	renderComponentInstance(testComponentInstanceWithParent("Risky", boundary, func() Node {
		panic("boom")
	}))
	boundary.node = ErrorBoundary(ErrorBoundaryProps{
		ResetKey: "c",
		Fallback: func(ErrorBoundaryContext) Node { return Text("fallback") },
	}).(ComponentNode)
	reset := requireKeyedNode(t, renderComponentInstance(boundary))
	if boundary.errorBoundary.phase != errorBoundaryProtected {
		t.Fatal("ResetKey change should clear failed boundary")
	}
	if reset.Key == healthy.Key {
		t.Fatalf("failed ResetKey change did not advance generation: %q", reset.Key)
	}
}

func TestErrorBoundaryUsesLatestFallbackPropsWhileFailed(t *testing.T) {
	resetRuntimeBoundaryTestState()
	boundary := testErrorBoundaryInstance("", func(ErrorBoundaryContext) Node {
		return Text("old")
	}, nil)
	renderComponentInstance(boundary)
	renderComponentInstance(testComponentInstanceWithParent("Risky", boundary, func() Node {
		panic("boom")
	}))

	boundary.node = ErrorBoundary(ErrorBoundaryProps{
		Fallback: func(ErrorBoundaryContext) Node {
			return Text("new")
		},
	}).(ComponentNode)
	fallback := requireKeyedNode(t, renderComponentInstance(boundary))
	if got := fallback.Node.(TextNode).Value; got != "new" {
		t.Fatalf("fallback = %q, want latest fallback", got)
	}
}

func TestErrorBoundaryCancelsPendingEffectsUnderFailedSubtree(t *testing.T) {
	resetRuntimeBoundaryTestState()
	runs := 0
	boundary := testErrorBoundaryInstance("", func(ErrorBoundaryContext) Node {
		return Text("fallback")
	}, nil)
	renderComponentInstance(boundary)
	child := testComponentInstanceWithParent("EffectThenPanic", boundary, func() Node {
		UseEffect(func() Cleanup {
			runs++
			return nil
		})
		panic("boom")
	})

	renderComponentInstance(child)
	flushPendingEffects()

	if runs != 0 {
		t.Fatalf("failed render effect runs = %d, want 0", runs)
	}
}

func TestErrorBoundaryUnmountsPreviousCleanupOnceAfterFailedUpdate(t *testing.T) {
	resetRuntimeBoundaryTestState()
	cleanups := 0
	panicNow := false
	boundary := testErrorBoundaryInstance("", func(ErrorBoundaryContext) Node {
		return Text("fallback")
	}, nil)
	renderComponentInstance(boundary)
	child := testComponentInstanceWithParent("CleanupChild", boundary, func() Node {
		UseEffect(func() Cleanup {
			return func() {
				cleanups++
			}
		})
		if panicNow {
			panic("boom")
		}
		return Text("child")
	})

	renderComponentInstance(child)
	flushPendingEffects()
	panicNow = true
	renderComponentInstance(child)
	flushPendingEffects()
	deactivateComponent(child)

	if cleanups != 1 {
		t.Fatalf("cleanups = %d, want previous cleanup once", cleanups)
	}
}

func TestErrorBoundaryContextSubscriptionReleasedWithFailedSubtree(t *testing.T) {
	resetRuntimeBoundaryTestState()
	ctx := CreateContext(contextValueFixture{Count: 1})
	boundary := testErrorBoundaryInstance("", func(ErrorBoundaryContext) Node {
		return Text("fallback")
	}, nil)
	renderComponentInstance(boundary)
	child := testComponentInstanceWithParent("ContextChild", boundary, func() Node {
		_ = UseContextSelector(ctx, func(value contextValueFixture) int {
			return value.Count
		})
		panic("boom")
	})

	renderComponentInstance(child)
	if len(contextSubscriptionsByID[ctx.id]) != 1 {
		t.Fatalf("subscriptions after failed render = %#v, want one before unmount", contextSubscriptionsByID)
	}
	deactivateComponent(child)
	if len(contextSubscriptionsByID[ctx.id]) != 0 {
		t.Fatalf("subscriptions after failed subtree release = %#v, want none", contextSubscriptionsByID)
	}
}

func TestErrorBoundaryDirtyDescendantPiercesMemoAncestor(t *testing.T) {
	resetRuntimeBoundaryTestState()
	parent := dirtyCleanInstance("MemoParent", nil)
	boundary := testErrorBoundaryInstanceWithParent(parent, "", func(ErrorBoundaryContext) Node {
		return Text("fallback")
	}, nil)
	renderComponentInstance(boundary)

	renderComponentInstance(testComponentInstanceWithParent("Risky", boundary, func() Node {
		panic("boom")
	}))

	if parent.dirtyDescendants != 1 {
		t.Fatalf("memo ancestor dirty descendants = %d, want 1", parent.dirtyDescendants)
	}
	next := Component("MemoParent", memoizedPropsFixture{ID: 1}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	parent.memoEqual = memoizeProps[memoizedPropsFixture]
	parent.node = next
	parent.dirty = false
	if shouldSkipComponentRender(parent, next, "") {
		t.Fatal("memo ancestor should not skip over failed boundary update")
	}
}

func TestErrorBoundaryDoesNotCaptureEffectOrMemoPhases(t *testing.T) {
	resetRuntimeBoundaryTestState()
	boundary := testErrorBoundaryInstance("", func(ErrorBoundaryContext) Node {
		return Text("fallback")
	}, nil)
	renderComponentInstance(boundary)
	effectChild := testComponentInstanceWithParent("EffectExploder", boundary, func() Node {
		UseEffect(func() Cleanup {
			panic("effect boom")
		})
		return Text("child")
	})
	renderComponentInstance(effectChild)
	flushPendingEffects()
	if boundary.errorBoundary.phase != errorBoundaryProtected {
		t.Fatal("effect setup panic should not switch boundary")
	}

	node := Component("MemoExploder", panicMemoPropsFixture{}, func(panicMemoPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	memoChild := newComponentInstance(node, "", boundary, nil)
	memoChild.dirty = false
	if shouldSkipComponentRender(memoChild, node, "") {
		t.Fatal("memo panic should not skip")
	}
	if boundary.errorBoundary.phase != errorBoundaryProtected {
		t.Fatal("memo comparator panic should not switch boundary")
	}
}

func TestInitialContextSelectorPanicCanBeCapturedByBoundaryRenderPath(t *testing.T) {
	resetRuntimeBoundaryTestState()
	ctx := CreateContext(contextValueFixture{Count: 1})
	boundary := testErrorBoundaryInstance("", func(ErrorBoundaryContext) Node {
		return Text("fallback")
	}, nil)
	renderComponentInstance(boundary)

	renderComponentInstance(testComponentInstanceWithParent("SelectorExploder", boundary, func() Node {
		_ = UseContextSelector(ctx, func(contextValueFixture) int {
			panic("selector boom")
		})
		return Empty()
	}))

	if boundary.errorBoundary.phase == errorBoundaryProtected {
		t.Fatal("initial selector panic should flow through render boundary")
	}
}

func TestVirtualRenderCallbacksKeepLocalFallbackBehavior(t *testing.T) {
	resetRuntimeBoundaryTestState()
	boundary := testErrorBoundaryInstance("", func(ErrorBoundaryContext) Node {
		return Text("fallback")
	}, nil)
	renderComponentInstance(boundary)

	_ = renderVirtualListItem(func(VirtualItem[int]) Node {
		panic("virtual boom")
	}, VirtualItem[int]{Item: 1})

	if boundary.errorBoundary.phase != errorBoundaryProtected {
		t.Fatal("virtual callback local fallback should not switch surrounding boundary")
	}
}

func testErrorBoundaryInstance(resetKey string, fallback func(ErrorBoundaryContext) Node, children []Node) *componentInstance {
	return testErrorBoundaryInstanceWithParent(nil, resetKey, fallback, children)
}

func testErrorBoundaryInstanceWithParent(parent *componentInstance, resetKey string, fallback func(ErrorBoundaryContext) Node, children []Node) *componentInstance {
	node := ErrorBoundary(ErrorBoundaryProps{
		ResetKey: resetKey,
		Fallback: fallback,
		Children: children,
	}).(ComponentNode)
	return newComponentInstance(node, "", parent, nil)
}

func testComponentInstanceWithParent(name string, parent *componentInstance, render func() Node) *componentInstance {
	node := Component(name, struct{}{}, func(struct{}) Node {
		return render()
	}).(ComponentNode)
	return newComponentInstance(node, "", parent, nil)
}

func resetRuntimeBoundaryTestState() {
	resetEffectsForTest()
	contextSubscriptionsByID = nil
}

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
	if !boundary.errorBoundary.failed {
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

	if !inner.errorBoundary.failed {
		t.Fatal("inner boundary should catch nearest child")
	}
	if outer.errorBoundary.failed {
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
	if !outer.errorBoundary.failed {
		t.Fatal("outer boundary should catch inner fallback panic")
	}
	if got := outer.errorBoundary.info.Component; got != "ErrorBoundary" {
		t.Fatalf("outer captured component = %q, want ErrorBoundary", got)
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
	if boundary.errorBoundary.failed {
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

	if boundary.errorBoundary.failed {
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
	if boundary.errorBoundary.failed {
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
	if boundary.errorBoundary.failed {
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
	if boundary.errorBoundary.failed {
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

	if !boundary.errorBoundary.failed {
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

	if boundary.errorBoundary.failed {
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

package goframe

import (
	"reflect"
	"testing"
)

func TestProtectedSubtreeEffectStateRollsBackAfterDescendantFailure(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()

	setupA := 0
	setupB := 0
	cleanupA := 0
	effectA := func() Cleanup {
		setupA++
		return func() {
			cleanupA++
		}
	}
	effectB := func() Cleanup {
		setupB++
		return nil
	}
	currentEffect := effectA
	currentDep := "A"
	owner := testComponentInstanceWithParent("EffectOwner", boundary, func() Node {
		UseEffect(currentEffect, Deps(currentDep))
		return Empty()
	})

	runProtectedSubtreeTestAttempt(boundary, func() {
		renderComponentInstance(owner)
	})
	flushPendingEffects()
	if setupA != 1 {
		t.Fatalf("A effect setups = %d, want 1", setupA)
	}

	currentEffect = effectB
	currentDep = "B"
	runProtectedSubtreeTestAttempt(boundary, func() {
		renderComponentInstance(owner)
		renderComponentInstance(transactionTestRisky(boundary, "effect descendant boom"))
	})

	if len(owner.effectSlots) != 1 {
		t.Fatalf("effect slots = %d, want 1", len(owner.effectSlots))
	}
	slot := owner.effectSlots[0]
	if !depsEqual(slot.deps, Deps("A")) {
		t.Fatalf("committed effect deps changed A -> B: %#v", slot.deps)
	}
	if reflect.ValueOf(slot.effect).Pointer() != reflect.ValueOf(effectA).Pointer() {
		t.Fatal("committed effect closure changed A -> B")
	}
	if slot.pending || slot.queued || len(pendingEffects) != 0 {
		t.Fatalf("failed B effect pending=%v queued=%v global=%d, want false/false/0",
			slot.pending, slot.queued, len(pendingEffects))
	}
	flushPendingEffects()
	if setupB != 0 || cleanupA != 0 {
		t.Fatalf("after rollback setupB=%d cleanupA=%d, want 0/0", setupB, cleanupA)
	}

	deactivateComponent(owner)
	if cleanupA != 1 {
		t.Fatalf("A cleanup after fallback release = %d, want 1", cleanupA)
	}
}

func TestProtectedSubtreeUnmountStateRollsBackAfterDescendantFailure(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()

	events := []string{}
	cleanupA := func() {
		events = append(events, "unmount A")
	}
	cleanupB := func() {
		events = append(events, "unmount B")
	}
	currentCleanup := Cleanup(cleanupA)
	owner := testComponentInstanceWithParent("UnmountOwner", boundary, func() Node {
		UseUnmount(currentCleanup)
		return Empty()
	})

	runProtectedSubtreeTestAttempt(boundary, func() {
		renderComponentInstance(owner)
	})
	currentCleanup = cleanupB
	runProtectedSubtreeTestAttempt(boundary, func() {
		renderComponentInstance(owner)
		renderComponentInstance(transactionTestRisky(boundary, "unmount descendant boom"))
	})

	if len(owner.unmountSlots) != 1 {
		t.Fatalf("unmount slots = %d, want 1", len(owner.unmountSlots))
	}
	if reflect.ValueOf(owner.unmountSlots[0]).Pointer() != reflect.ValueOf(cleanupA).Pointer() {
		t.Fatal("committed UseUnmount callback changed A -> B")
	}
	deactivateComponent(owner)
	assertEffectEvents(t, events, []string{"unmount A"})
}

func TestProtectedSubtreeResourceStateRollsBackAfterDescendantFailure(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()

	loader := &resourceTestLoader{}
	key := "a"
	var resource Resource[string]
	owner := testComponentInstanceWithParent("ResourceOwner", boundary, func() Node {
		resource, _ = UseResource(key, loader.load)
		return Empty()
	})

	runProtectedSubtreeTestAttempt(boundary, func() {
		renderComponentInstance(owner)
	})
	flushPendingEffects()

	control := resourceControlForTest[string](t, owner)
	generationA := control.generation
	runA := control.current
	if !resource.Loading() || runA == nil || !runA.active {
		t.Fatalf("committed A resource=%#v run=%#v, want loading active A", resource, runA)
	}

	key = "b"
	runProtectedSubtreeTestAttempt(boundary, func() {
		renderComponentInstance(owner)
		renderComponentInstance(transactionTestRisky(boundary, "resource descendant boom"))
	})

	if control.key != "a" {
		t.Fatalf("committed resource key changed a -> %q", control.key)
	}
	if control.generation != generationA {
		t.Fatalf("committed resource generation changed %d -> %d", generationA, control.generation)
	}
	if !control.snapshot.Loading() {
		t.Fatalf("committed resource snapshot changed from loading A: %#v", control.snapshot)
	}
	if control.current != runA || !runA.active {
		t.Fatalf("committed A run current=%p active=%v, want %p/true",
			control.current, runA.active, runA)
	}
	if control.pending.attempt != nil {
		t.Fatalf("resource pending attempt retained after rollback: %#v", control.pending)
	}
	if loader.cleanups != 0 {
		t.Fatalf("A cleanup during rollback = %d, want 0", loader.cleanups)
	}
	if got := loader.starts; len(got) != 1 || got[0] != "a" {
		t.Fatalf("loader starts after failed B = %#v, want [a]", got)
	}

	deactivateComponent(owner)
	if loader.cleanups != 1 {
		t.Fatalf("A cleanup after fallback release = %d, want 1", loader.cleanups)
	}
	if len(loader.starts) != 1 {
		t.Fatalf("B loader starts after fallback release = %d, want 0", len(loader.starts)-1)
	}
}

func TestProtectedSubtreeLaterSiblingEffectCannotFlushAfterCapture(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()

	laterSetups := 0
	later := testComponentInstanceWithParent("LaterEffect", boundary, func() Node {
		UseEffect(func() Cleanup {
			laterSetups++
			return nil
		})
		return Empty()
	})

	runProtectedSubtreeTestAttempt(boundary, func() {
		renderComponentInstance(transactionTestRisky(boundary, "early descendant boom"))
		renderComponentInstance(later)
	})
	flushPendingEffects()

	if laterSetups != 0 {
		t.Fatalf("later protected sibling effect ran %d time(s), want 0", laterSetups)
	}
	if len(later.effectSlots) != 0 {
		t.Fatalf("later sibling committed effect slots = %d, want 0", len(later.effectSlots))
	}
}

func TestProtectedSubtreeManualResetRetryCommitsHealthyLifecycleOnce(t *testing.T) {
	isolateLifecycleTestState(t)

	var reset func()
	boundary := testErrorBoundaryInstance("", func(ctx ErrorBoundaryContext) Node {
		reset = ctx.Reset
		return Text("fallback")
	}, nil)
	renderComponentInstance(boundary)

	events := []string{}
	loader := &resourceTestLoader{}
	version := "A"
	owner := transactionTestLifecycleOwner(boundary, &version, &events, loader)

	runProtectedSubtreeTestAttempt(boundary, func() {
		renderComponentInstance(owner)
	})
	flushPendingEffects()

	version = "B"
	runProtectedSubtreeTestAttempt(boundary, func() {
		renderComponentInstance(owner)
		renderComponentInstance(transactionTestRisky(boundary, "retry descendant boom"))
	})
	renderComponentInstance(boundary)
	if reset == nil {
		t.Fatal("fallback did not provide reset")
	}
	deactivateComponent(owner)

	assertEffectEvents(t, events, []string{"effect A", "effect cleanup A", "unmount A"})
	if got := loader.starts; len(got) != 1 || got[0] != "A" {
		t.Fatalf("loader starts before retry = %#v, want [A]", got)
	}
	if loader.cleanups != 1 {
		t.Fatalf("A resource cleanups before retry = %d, want 1", loader.cleanups)
	}

	reset()
	renderComponentInstance(boundary)
	retry := transactionTestLifecycleOwner(boundary, &version, &events, loader)
	runProtectedSubtreeTestAttempt(boundary, func() {
		renderComponentInstance(retry)
	})
	flushPendingEffects()

	assertEffectEvents(t, events, []string{
		"effect A",
		"effect cleanup A",
		"unmount A",
		"effect B",
	})
	if got := loader.starts; len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Fatalf("loader starts after retry = %#v, want [A B]", got)
	}
	if retry.lifecycleAttempt.active || len(pendingEffects) != 0 {
		t.Fatalf("retry left lifecycle active=%v pending=%d, want false/0",
			retry.lifecycleAttempt.active, len(pendingEffects))
	}
}

func TestBoundaryTransactionNestedCaptureRollsBackInnerOnly(t *testing.T) {
	isolateLifecycleTestState(t)
	outer := transactionTestBoundary()

	events := []string{}
	outerOwner := testComponentInstanceWithParent("OuterOwner", outer, func() Node {
		UseEffect(func() Cleanup {
			events = append(events, "outer owner")
			return nil
		})
		return Empty()
	})
	inner := testErrorBoundaryInstanceWithParent(outerOwner, "", func(ErrorBoundaryContext) Node {
		return Text("inner fallback")
	}, nil)
	innerOwner := testComponentInstanceWithParent("InnerOwner", inner, func() Node {
		UseEffect(func() Cleanup {
			events = append(events, "inner owner")
			return nil
		})
		return Empty()
	})
	outerSibling := testComponentInstanceWithParent("OuterSibling", outerOwner, func() Node {
		UseEffect(func() Cleanup {
			events = append(events, "outer sibling")
			return nil
		})
		return Empty()
	})

	runProtectedSubtreeTestAttempt(outer, func() {
		renderComponentInstance(outerOwner)
		renderComponentInstance(inner)
		runProtectedSubtreeTestAttempt(inner, func() {
			renderComponentInstance(innerOwner)
			renderComponentInstance(transactionTestRisky(inner, "inner descendant boom"))
		})
		renderComponentInstance(outerSibling)
	})
	flushPendingEffects()

	if inner.errorBoundary.phase != errorBoundaryCaptured {
		t.Fatalf("inner boundary phase = %d, want captured", inner.errorBoundary.phase)
	}
	if outer.errorBoundary.phase != errorBoundaryProtected {
		t.Fatalf("outer boundary phase = %d, want protected", outer.errorBoundary.phase)
	}
	if len(innerOwner.effectSlots) != 0 {
		t.Fatalf("inner committed effect slots = %d, want 0", len(innerOwner.effectSlots))
	}
	assertEffectEvents(t, events, []string{"outer owner", "outer sibling"})
}

func TestBoundaryTransactionSuccessfulInnerRollsBackWithOuterFailure(t *testing.T) {
	isolateLifecycleTestState(t)
	outer := transactionTestBoundary()

	innerSetups := 0
	inner := testErrorBoundaryInstanceWithParent(outer, "", func(ErrorBoundaryContext) Node {
		return Text("inner fallback")
	}, nil)
	innerOwner := testComponentInstanceWithParent("InnerOwner", inner, func() Node {
		UseEffect(func() Cleanup {
			innerSetups++
			return nil
		})
		return Empty()
	})

	runProtectedSubtreeTestAttempt(outer, func() {
		renderComponentInstance(inner)
		runProtectedSubtreeTestAttempt(inner, func() {
			renderComponentInstance(innerOwner)
		})
		renderComponentInstance(transactionTestRisky(outer, "outer descendant boom"))
	})
	flushPendingEffects()

	if outer.errorBoundary.phase != errorBoundaryCaptured {
		t.Fatalf("outer boundary phase = %d, want captured", outer.errorBoundary.phase)
	}
	if inner.errorBoundary.phase != errorBoundaryProtected {
		t.Fatalf("inner boundary phase = %d, want protected", inner.errorBoundary.phase)
	}
	if len(innerOwner.effectSlots) != 0 {
		t.Fatalf("inner effect slots after outer rollback = %d, want 0", len(innerOwner.effectSlots))
	}
	if innerSetups != 0 {
		t.Fatalf("inner effect setups after outer rollback = %d, want 0", innerSetups)
	}
}

func TestBoundaryTransactionInvariantEscapeRollsBackAndRestoresState(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()

	events := []string{}
	label := "A"
	dep := 1
	owner := testComponentInstanceWithParent("InvariantOwner", boundary, func() Node {
		committedLabel := label
		UseEffect(func() Cleanup {
			events = append(events, "setup "+committedLabel)
			return func() {
				events = append(events, "cleanup "+committedLabel)
			}
		}, Deps(dep))
		return Empty()
	})
	runProtectedSubtreeTestAttempt(boundary, func() {
		renderComponentInstance(owner)
	})
	flushPendingEffects()

	label = "B"
	dep = 2
	invariant := testComponentInstanceWithParent("InvariantDescendant", boundary, func() Node {
		UseEffect(func() Cleanup { return nil }, Deps(struct{}{}))
		return Empty()
	})
	assertPanic(t,
		"goframe: unsupported effect dependency type; reduce complex values to string, id, version, or counter",
		func() {
			runProtectedSubtreeTestAttempt(boundary, func() {
				renderComponentInstance(owner)
				renderComponentInstance(invariant)
			})
		})

	if currentComponent != nil {
		t.Fatalf("current component after invariant escape = %p, want nil", currentComponent)
	}
	if currentProtectedSubtreeTransaction != nil {
		t.Fatalf("protected transaction after invariant escape = %p, want nil",
			currentProtectedSubtreeTransaction)
	}
	if owner.lifecycleAttempt.active || len(owner.lifecycleAttempt.effects) != 0 {
		t.Fatalf("owner lifecycle attempt after invariant escape = %#v, want cleared", owner.lifecycleAttempt)
	}
	if len(owner.effectSlots) != 1 || !depsEqual(owner.effectSlots[0].deps, Deps(1)) {
		t.Fatalf("committed owner effect changed after invariant escape: %#v", owner.effectSlots)
	}
	if len(pendingEffects) != 0 {
		t.Fatalf("pending effects after invariant escape = %d, want 0", len(pendingEffects))
	}
	if boundary.errorBoundary.phase != errorBoundaryProtected {
		t.Fatalf("boundary captured invariant, phase=%d", boundary.errorBoundary.phase)
	}

	label = "C"
	dep = 3
	runProtectedSubtreeTestAttempt(boundary, func() {
		renderComponentInstance(owner)
	})
	flushPendingEffects()
	deactivateComponent(owner)
	assertEffectEvents(t, events, []string{
		"setup A",
		"cleanup A",
		"setup C",
		"cleanup C",
	})
}

func TestProtectedSubtreeSuccessfulUpdatePreservesLifecycleTiming(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()

	events := []string{}
	loader := &resourceTestLoader{}
	version := "A"
	owner := testComponentInstanceWithParent("SuccessfulOwner", boundary, func() Node {
		committedVersion := version
		UseEffect(func() Cleanup {
			events = append(events, "effect "+committedVersion)
			return func() {
				events = append(events, "effect cleanup "+committedVersion)
			}
		}, Deps(committedVersion))
		UseUnmount(func() {
			events = append(events, "unmount "+committedVersion)
		})
		_, _ = UseResource(committedVersion, loader.load)
		return Empty()
	})

	runProtectedSubtreeTestAttempt(boundary, func() {
		renderComponentInstance(owner)
	})
	flushPendingEffects()
	control := resourceControlForTest[string](t, owner)
	runA := control.current

	version = "B"
	runProtectedSubtreeTestAttempt(boundary, func() {
		renderComponentInstance(owner)
		if control.key != "A" || control.current != runA || !runA.active {
			t.Fatalf("resource committed before subtree success: key=%q current=%p active=%v",
				control.key, control.current, runA.active)
		}
		if len(events) != 1 || loader.cleanups != 0 || len(loader.starts) != 1 {
			t.Fatalf("lifecycle ran during reconciliation events=%#v cleanups=%d starts=%#v",
				events, loader.cleanups, loader.starts)
		}
	})

	if control.key != "B" || control.generation != 1 || runA.active {
		t.Fatalf("resource after successful commit key=%q generation=%d old active=%v, want B/1/false",
			control.key, control.generation, runA.active)
	}
	if len(events) != 1 || loader.cleanups != 0 || len(loader.starts) != 1 {
		t.Fatalf("lifecycle ran before effect flush events=%#v cleanups=%d starts=%#v",
			events, loader.cleanups, loader.starts)
	}

	flushPendingEffects()
	assertEffectEvents(t, events, []string{"effect A", "effect cleanup A", "effect B"})
	if loader.cleanups != 1 {
		t.Fatalf("resource cleanups after successful update = %d, want 1", loader.cleanups)
	}
	if got := loader.starts; len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Fatalf("resource starts after successful update = %#v, want [A B]", got)
	}

	deactivateComponent(owner)
	assertEffectEvents(t, events, []string{
		"effect A",
		"effect cleanup A",
		"effect B",
		"effect cleanup B",
		"unmount B",
	})
	if loader.cleanups != 2 {
		t.Fatalf("resource cleanups after unmount = %d, want 2", loader.cleanups)
	}
}

func TestProtectedSubtreeSuccessfulLifecycleOrderRemainsParentFirst(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()

	events := []string{}
	parent := testComponentInstanceWithParent("EffectParent", boundary, func() Node {
		UseEffect(func() Cleanup {
			events = append(events, "parent")
			return nil
		})
		return Empty()
	})
	child := testComponentInstanceWithParent("EffectChild", parent, func() Node {
		UseEffect(func() Cleanup {
			events = append(events, "child")
			return nil
		})
		return Empty()
	})

	runProtectedSubtreeTestAttempt(boundary, func() {
		renderComponentInstance(parent)
		renderComponentInstance(child)
	})
	if len(pendingEffects) != 2 ||
		pendingEffects[0].owner != parent ||
		pendingEffects[1].owner != child {
		t.Fatalf("successful transaction effect order = %#v, want parent then child", pendingEffects)
	}
	flushPendingEffects()
	assertEffectEvents(t, events, []string{"parent", "child"})
}

func runProtectedSubtreeTestAttempt(boundary *componentInstance, reconcile func()) {
	runProtectedSubtreeLifecycleTransaction(boundary, reconcile)
}

func transactionTestBoundary() *componentInstance {
	boundary := testErrorBoundaryInstance("", func(ErrorBoundaryContext) Node {
		return Text("fallback")
	}, nil)
	renderComponentInstance(boundary)
	return boundary
}

func transactionTestRisky(parent *componentInstance, panicValue string) *componentInstance {
	return testComponentInstanceWithParent("RiskyDescendant", parent, func() Node {
		panic(panicValue)
	})
}

func transactionTestLifecycleOwner(
	parent *componentInstance,
	version *string,
	events *[]string,
	loader *resourceTestLoader,
) *componentInstance {
	return testComponentInstanceWithParent("LifecycleOwner", parent, func() Node {
		committedVersion := *version
		UseEffect(func() Cleanup {
			*events = append(*events, "effect "+committedVersion)
			return func() {
				*events = append(*events, "effect cleanup "+committedVersion)
			}
		}, Deps(committedVersion))
		UseUnmount(func() {
			*events = append(*events, "unmount "+committedVersion)
		})
		_, _ = UseResource(committedVersion, loader.load)
		return Empty()
	})
}

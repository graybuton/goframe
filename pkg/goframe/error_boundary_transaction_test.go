package goframe

import (
	"reflect"
	"testing"
)

func TestDirtyOwnerBelowProtectedBoundaryRemainsScheduled(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()
	owner := dirtyCleanInstance("DirtyOwner", boundary)

	var scheduled []*componentInstance
	owner.scheduleUpdate = func(instance *componentInstance) {
		scheduled = append(scheduled, instance)
	}

	markComponentDirty(owner)

	assertInstances(t, scheduled, []*componentInstance{owner})
	if got := lifecycleStateForDirtyUpdate(owner); got != boundary.errorBoundary {
		t.Fatalf("protected lifecycle state = %p, want boundary %p", got, boundary.errorBoundary)
	}
	if !owner.dirty || !owner.dirtyCounted {
		t.Fatalf("owner dirty=%v counted=%v, want true/true", owner.dirty, owner.dirtyCounted)
	}
	if boundary.dirty {
		t.Fatal("protected boundary was marked dirty for a descendant update")
	}
	if boundary.dirtyDescendants != 1 {
		t.Fatalf("boundary dirty descendants = %d, want 1", boundary.dirtyDescendants)
	}
}

func TestNestedDirtyOwnerRemainsScheduled(t *testing.T) {
	isolateLifecycleTestState(t)
	outer := transactionTestBoundary()
	inner := testErrorBoundaryInstanceWithParent(outer, "", func(ErrorBoundaryContext) Node {
		return Text("inner fallback")
	}, nil)
	renderComponentInstance(inner)
	owner := dirtyCleanInstance("NestedDirtyOwner", inner)

	var scheduled []*componentInstance
	owner.scheduleUpdate = func(instance *componentInstance) {
		scheduled = append(scheduled, instance)
	}

	markComponentDirty(owner)

	assertInstances(t, scheduled, []*componentInstance{owner})
	if got := lifecycleStateForDirtyUpdate(owner); got != inner.errorBoundary {
		t.Fatalf("protected lifecycle state = %p, want inner boundary %p", got, inner.errorBoundary)
	}
	if got := lifecycleStateForDirtyUpdate(inner); got != nil {
		t.Fatalf("dirty boundary selected ancestor lifecycle state %p, want nil", got)
	}
	if outer.dirty || inner.dirty {
		t.Fatalf("boundaries dirty outer=%v inner=%v, want false/false", outer.dirty, inner.dirty)
	}
	if outer.dirtyDescendants != 1 || inner.dirtyDescendants != 1 {
		t.Fatalf("dirty descendants outer=%d inner=%d, want 1/1",
			outer.dirtyDescendants, inner.dirtyDescendants)
	}
}

func TestDirtyFallbackChildRemainsScheduledBelowCapturedBoundary(t *testing.T) {
	isolateLifecycleTestState(t)
	outer := transactionTestBoundary()
	inner := testErrorBoundaryInstanceWithParent(outer, "", func(ErrorBoundaryContext) Node {
		return Text("inner fallback")
	}, nil)
	renderComponentInstance(inner)
	inner.errorBoundary.phase = errorBoundaryFallback
	owner := dirtyCleanInstance("FallbackDirtyOwner", inner)

	var scheduled []*componentInstance
	owner.scheduleUpdate = func(instance *componentInstance) {
		scheduled = append(scheduled, instance)
	}

	markComponentDirty(owner)

	assertInstances(t, scheduled, []*componentInstance{owner})
	if got := lifecycleStateForDirtyUpdate(owner); got != outer.errorBoundary {
		t.Fatalf("protected lifecycle state = %p, want outer boundary %p", got, outer.errorBoundary)
	}
	if outer.dirty || inner.dirty {
		t.Fatalf("boundaries dirty outer=%v inner=%v, want false/false", outer.dirty, inner.dirty)
	}
	if outer.dirtyDescendants != 1 || inner.dirtyDescendants != 1 {
		t.Fatalf("dirty descendants outer=%d inner=%d, want 1/1",
			outer.dirtyDescendants, inner.dirtyDescendants)
	}
}

func TestCapturedDirtyBatchLifecycleStateSelection(t *testing.T) {
	tests := []struct {
		name  string
		build func() (*componentInstance, *errorBoundaryState)
	}{
		{
			name: "owner under captured boundary",
			build: func() (*componentInstance, *errorBoundaryState) {
				boundary := transactionTestBoundary()
				boundary.errorBoundary.phase = errorBoundaryCaptured
				return dirtyCleanInstance("DirtyOwner", boundary), boundary.errorBoundary
			},
		},
		{
			name: "protected inner under captured outer",
			build: func() (*componentInstance, *errorBoundaryState) {
				outer := transactionTestBoundary()
				outer.errorBoundary.phase = errorBoundaryCaptured
				inner := transactionTestBoundaryWithParent(outer)
				return dirtyCleanInstance("DirtyOwner", inner), outer.errorBoundary
			},
		},
		{
			name: "owner under captured inner and protected outer",
			build: func() (*componentInstance, *errorBoundaryState) {
				outer := transactionTestBoundary()
				inner := transactionTestBoundaryWithParent(outer)
				inner.errorBoundary.phase = errorBoundaryCaptured
				return dirtyCleanInstance("DirtyOwner", inner), inner.errorBoundary
			},
		},
		{
			name: "captured boundary instance under protected outer",
			build: func() (*componentInstance, *errorBoundaryState) {
				outer := transactionTestBoundary()
				inner := transactionTestBoundaryWithParent(outer)
				inner.errorBoundary.phase = errorBoundaryCaptured
				return inner, outer.errorBoundary
			},
		},
		{
			name: "protected boundary instance under captured outer",
			build: func() (*componentInstance, *errorBoundaryState) {
				outer := transactionTestBoundary()
				outer.errorBoundary.phase = errorBoundaryCaptured
				inner := transactionTestBoundaryWithParent(outer)
				return inner, outer.errorBoundary
			},
		},
		{
			name: "fallback child under protected outer",
			build: func() (*componentInstance, *errorBoundaryState) {
				outer := transactionTestBoundary()
				inner := transactionTestBoundaryWithParent(outer)
				inner.errorBoundary.phase = errorBoundaryFallback
				return dirtyCleanInstance("DirtyFallbackChild", inner), outer.errorBoundary
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateLifecycleTestState(t)
			instance, want := test.build()
			if got := lifecycleStateForDirtyUpdate(instance); got != want {
				t.Fatalf("dirty lifecycle state = %p phase=%v, want %p phase=%v",
					got, boundaryPhaseForTest(got), want, boundaryPhaseForTest(want))
			}
		})
	}
}

func TestCapturedDirtyBatchIndependentBranchRemainsRunnable(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()
	boundary.errorBoundary.phase = errorBoundaryCaptured
	stale := dirtyCleanInstance("StaleOwner", boundary)
	independent := dirtyCleanInstance("IndependentOwner", nil)

	if got := lifecycleStateForDirtyUpdate(stale); got != boundary.errorBoundary {
		t.Fatalf("stale owner state = %p, want captured boundary %p",
			got, boundary.errorBoundary)
	}
	if got := lifecycleStateForDirtyUpdate(independent); got != nil {
		t.Fatalf("independent owner state = %p, want nil", got)
	}
}

func TestCapturedDirtyBatchDiscardBalancesBookkeeping(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()
	boundary.errorBoundary.phase = errorBoundaryCaptured
	markComponentDirty(boundary)

	owner := dirtyCleanInstance("StaleOwner", boundary)
	renders := 0
	owner.update = func() {
		renders++
	}
	markComponentDirty(owner)

	state := lifecycleStateForDirtyUpdate(owner)
	if state != boundary.errorBoundary || state.phase != errorBoundaryCaptured {
		t.Fatalf("dirty lifecycle state = %p phase=%v, want captured boundary %p",
			state, boundaryPhaseForTest(state), boundary.errorBoundary)
	}
	if state.phase == errorBoundaryCaptured {
		clearComponentDirty(owner)
	} else {
		owner.update()
	}

	if renders != 0 {
		t.Fatalf("discarded owner renders = %d, want 0", renders)
	}
	if owner.dirty || owner.dirtyCounted {
		t.Fatalf("discarded owner dirty=%v counted=%v, want false/false",
			owner.dirty, owner.dirtyCounted)
	}
	if boundary.dirtyDescendants != 0 {
		t.Fatalf("captured boundary dirty descendants = %d, want 0",
			boundary.dirtyDescendants)
	}
	if !boundary.dirty || !boundary.dirtyCounted {
		t.Fatalf("captured boundary dirty=%v counted=%v, want true/true",
			boundary.dirty, boundary.dirtyCounted)
	}
}

func TestNestedFallbackTransactionSelectsProtectedAncestorByPhase(t *testing.T) {
	for _, test := range []struct {
		name  string
		phase errorBoundaryPhase
		outer bool
	}{
		{name: "protected", phase: errorBoundaryProtected},
		{name: "captured", phase: errorBoundaryCaptured, outer: true},
		{name: "fallback", phase: errorBoundaryFallback, outer: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateLifecycleTestState(t)
			outer := transactionTestBoundary()
			inner := testErrorBoundaryInstanceWithParent(outer, "", func(ErrorBoundaryContext) Node {
				return Text("inner fallback")
			}, nil)
			renderComponentInstance(inner)
			inner.errorBoundary.phase = test.phase

			want := (*errorBoundaryState)(nil)
			if test.outer {
				want = outer.errorBoundary
			}
			if got := lifecycleStateForDirtyUpdate(inner); got != want {
				t.Fatalf("%s inner boundary selected state %p, want %p",
					test.name, got, want)
			}
		})
	}
}

func TestNestedFallbackTransactionWithoutProtectedAncestorSelectsNil(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()

	for _, phase := range []errorBoundaryPhase{
		errorBoundaryCaptured,
		errorBoundaryFallback,
	} {
		boundary.errorBoundary.phase = phase
		if got := lifecycleStateForDirtyUpdate(boundary); got != nil {
			t.Fatalf("boundary phase %d selected state %p without protected ancestor, want nil",
				phase, got)
		}
	}
}

func TestNestedFallbackTransactionSelectsNearestProtectedAncestor(t *testing.T) {
	isolateLifecycleTestState(t)
	outer := transactionTestBoundary()
	middle := testErrorBoundaryInstanceWithParent(outer, "", func(ErrorBoundaryContext) Node {
		return Text("middle fallback")
	}, nil)
	renderComponentInstance(middle)
	middle.errorBoundary.phase = errorBoundaryFallback
	inner := testErrorBoundaryInstanceWithParent(middle, "", func(ErrorBoundaryContext) Node {
		return Text("inner fallback")
	}, nil)
	renderComponentInstance(inner)
	inner.errorBoundary.phase = errorBoundaryCaptured

	if got := lifecycleStateForDirtyUpdate(inner); got != outer.errorBoundary {
		t.Fatalf("captured inner selected state %p through fallback middle, want outer %p",
			got, outer.errorBoundary)
	}

	nearer := testErrorBoundaryInstanceWithParent(middle, "", func(ErrorBoundaryContext) Node {
		return Text("nearer fallback")
	}, nil)
	renderComponentInstance(nearer)
	inner.parent = nearer
	if got := lifecycleStateForDirtyUpdate(inner); got != nearer.errorBoundary {
		t.Fatalf("captured inner selected state %p, want nearest protected %p",
			got, nearer.errorBoundary)
	}
}

func TestNestedFallbackTransactionRestoresSelectedOuterState(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		isolateLifecycleTestState(t)
		outer := transactionTestBoundary()
		inner := testErrorBoundaryInstanceWithParent(outer, "", func(ErrorBoundaryContext) Node {
			return Text("inner fallback")
		}, nil)
		renderComponentInstance(inner)
		inner.errorBoundary.phase = errorBoundaryFallback
		owner := testComponentInstanceWithParent("FallbackOwner", inner, func() Node {
			UseEffect(func() Cleanup { return nil })
			return Empty()
		})

		state := lifecycleStateForDirtyUpdate(inner)
		if state != outer.errorBoundary {
			t.Fatalf("selected state = %p, want outer %p", state, outer.errorBoundary)
		}
		runProtectedLifecycleStateTestAttempt(state, func() {
			renderComponentInstance(owner)
		})

		if currentProtectedLifecycleBoundary != nil {
			t.Fatal("successful nested fallback transaction left current state installed")
		}
		if state.attempts != nil {
			t.Fatalf("successful nested fallback transaction retained attempts: %#v", state.attempts)
		}
		if owner.lifecycleAttempt.active {
			t.Fatal("successful nested fallback transaction left owner attempt active")
		}
		if len(owner.effectSlots) != 1 || len(pendingEffects) != 1 {
			t.Fatalf("successful nested fallback commit slots=%d pending=%d, want 1/1",
				len(owner.effectSlots), len(pendingEffects))
		}
	})

	t.Run("failure", func(t *testing.T) {
		isolateLifecycleTestState(t)
		outer := transactionTestBoundary()
		inner := testErrorBoundaryInstanceWithParent(outer, "", func(ErrorBoundaryContext) Node {
			return Text("inner fallback")
		}, nil)
		renderComponentInstance(inner)
		inner.errorBoundary.phase = errorBoundaryFallback
		owner := testComponentInstanceWithParent("FallbackOwner", inner, func() Node {
			UseEffect(func() Cleanup { return nil })
			UseUnmount(func() {})
			return Empty()
		})

		state := lifecycleStateForDirtyUpdate(inner)
		if state != outer.errorBoundary {
			t.Fatalf("selected state = %p, want outer %p", state, outer.errorBoundary)
		}
		runProtectedLifecycleStateTestAttempt(state, func() {
			renderComponentInstance(owner)
			renderComponentInstance(transactionTestRisky(inner, "nested fallback descendant boom"))
		})

		if outer.errorBoundary.phase != errorBoundaryCaptured {
			t.Fatalf("outer phase = %d, want captured", outer.errorBoundary.phase)
		}
		if currentProtectedLifecycleBoundary != nil {
			t.Fatal("failed nested fallback transaction left current state installed")
		}
		if state.attempts != nil {
			t.Fatalf("failed nested fallback transaction retained attempts: %#v", state.attempts)
		}
		if owner.lifecycleAttempt.active {
			t.Fatal("failed nested fallback transaction left owner attempt active")
		}
		if len(owner.effectSlots) != 0 || len(owner.unmountSlots) != 0 || len(pendingEffects) != 0 {
			t.Fatalf("failed nested fallback committed effects=%d unmounts=%d pending=%d, want 0/0/0",
				len(owner.effectSlots), len(owner.unmountSlots), len(pendingEffects))
		}
	})
}

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
	if currentProtectedLifecycleBoundary != nil {
		t.Fatal("protected lifecycle boundary remained installed after invariant escape")
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

func TestProtectedSubtreeLifecycleHooksInstallAndRestoreLazily(t *testing.T) {
	isolateLifecycleTestState(t)
	previousCurrent := currentProtectedLifecycleBoundary
	previousBegin := beginProtectedLifecycle
	previousFinish := finishProtectedLifecycle
	currentProtectedLifecycleBoundary = nil
	beginProtectedLifecycle = nil
	finishProtectedLifecycle = nil
	t.Cleanup(func() {
		currentProtectedLifecycleBoundary = previousCurrent
		beginProtectedLifecycle = previousBegin
		finishProtectedLifecycle = previousFinish
	})
	if beginProtectedLifecycle != nil ||
		finishProtectedLifecycle != nil {
		t.Fatal("protected lifecycle hooks installed before an ErrorBoundary")
	}

	ordinary := testComponentInstance("Ordinary", func() Node {
		UseEffect(func() Cleanup { return nil })
		return Empty()
	}, nil)
	renderComponentInstance(ordinary)
	if ordinary.errorBoundary != nil {
		t.Fatal("ordinary component installed protected lifecycle hooks")
	}
	if currentProtectedLifecycleBoundary != nil {
		t.Fatal("ordinary component render installed protected lifecycle boundary")
	}
	if beginProtectedLifecycle != nil ||
		finishProtectedLifecycle != nil {
		t.Fatal("ordinary component render installed protected lifecycle hooks")
	}
	if len(ordinary.effectSlots) != 1 {
		t.Fatalf("ordinary component effect slots = %d, want immediate commit", len(ordinary.effectSlots))
	}

	outer := transactionTestBoundary()
	if beginProtectedLifecycle == nil ||
		finishProtectedLifecycle == nil {
		t.Fatal("ErrorBoundary did not install protected lifecycle hooks")
	}
	inner := testErrorBoundaryInstanceWithParent(outer, "", func(ErrorBoundaryContext) Node {
		return Text("inner fallback")
	}, nil)
	renderComponentInstance(inner)
	if currentProtectedLifecycleBoundary != nil {
		t.Fatal("idle ErrorBoundary left protected lifecycle boundary installed")
	}

	runProtectedSubtreeTestAttempt(outer, func() {
		if currentProtectedLifecycleBoundary != outer.errorBoundary {
			t.Fatal("outer transaction did not install its protected lifecycle boundary")
		}
		runProtectedSubtreeTestAttempt(inner, func() {
			if currentProtectedLifecycleBoundary != inner.errorBoundary {
				t.Fatal("inner transaction did not install its protected lifecycle boundary")
			}
		})
		if currentProtectedLifecycleBoundary != outer.errorBoundary {
			t.Fatal("inner transaction did not restore outer protected lifecycle boundary")
		}
	})
	if currentProtectedLifecycleBoundary != nil {
		t.Fatal("outer transaction did not restore default protected lifecycle boundary")
	}
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
	var state *errorBoundaryState
	if boundary != nil &&
		boundary.errorBoundary != nil &&
		boundary.errorBoundary.phase == errorBoundaryProtected {
		state = boundary.errorBoundary
	}
	if state == nil {
		panic("goframe: test boundary has no protected lifecycle reconcile hook")
	}
	previous := beginProtectedLifecycle(state)
	defer finishProtectedLifecycle(state, previous)
	reconcile()
}

func runProtectedLifecycleStateTestAttempt(state *errorBoundaryState, reconcile func()) {
	previous := beginProtectedLifecycle(state)
	defer finishProtectedLifecycle(state, previous)
	reconcile()
}

func transactionTestBoundary() *componentInstance {
	boundary := testErrorBoundaryInstance("", func(ErrorBoundaryContext) Node {
		return Text("fallback")
	}, nil)
	renderComponentInstance(boundary)
	return boundary
}

func transactionTestBoundaryWithParent(parent *componentInstance) *componentInstance {
	boundary := testErrorBoundaryInstanceWithParent(parent, "", func(ErrorBoundaryContext) Node {
		return Text("fallback")
	}, nil)
	renderComponentInstance(boundary)
	return boundary
}

func boundaryPhaseForTest(state *errorBoundaryState) any {
	if state == nil {
		return nil
	}
	return state.phase
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

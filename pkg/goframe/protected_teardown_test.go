package goframe

import "testing"

func TestProtectedTeardownSuccessfulTransactionReleasesInOrder(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()
	events := []string{}
	first := protectedTeardownForTest(&events, "first")
	second := protectedTeardownForTest(&events, "second")

	runProtectedSubtreeTestAttempt(boundary, func() {
		if !stageProtectedSubtreeTeardownForTest(first) ||
			!stageProtectedSubtreeTeardownForTest(second) {
			t.Fatal("active protected transaction did not accept teardown")
		}
		if len(events) != 0 {
			t.Fatalf("teardown ran during reconciliation: %#v", events)
		}
		assertProtectedTeardownState(t, boundary.errorBoundary, 2, 0)
	})

	assertEffectEvents(t, events, []string{"first", "second"})
	assertProtectedTeardownState(t, boundary.errorBoundary, 0, 0)
	if currentProtectedLifecycleBoundary != nil {
		t.Fatal("successful teardown transaction left current boundary installed")
	}
}

func TestProtectedTeardownCaptureRetainsOwnership(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()
	events := []string{}
	teardown := protectedTeardownForTest(&events, "retained")
	owner := testComponentInstanceWithParent("AttemptOwner", boundary, func() Node {
		UseEffect(func() Cleanup { return nil })
		return Empty()
	})

	runProtectedSubtreeTestAttempt(boundary, func() {
		stageProtectedSubtreeTeardownForTest(teardown)
		renderComponentInstance(owner)
		renderComponentInstance(transactionTestRisky(boundary, "capture teardown"))
	})

	if len(events) != 0 {
		t.Fatalf("captured transaction released teardown: %#v", events)
	}
	if len(owner.effectSlots) != 0 || owner.lifecycleAttempt.active {
		t.Fatalf("captured owner committed lifecycle: slots=%d attempt=%#v",
			len(owner.effectSlots), owner.lifecycleAttempt)
	}
	assertProtectedTeardownState(t, boundary.errorBoundary, 0, 1)
	if currentProtectedLifecycleBoundary != nil {
		t.Fatal("captured teardown transaction left current boundary installed")
	}
}

func TestProtectedTeardownSuccessfulRetryFinalizesRetainedOnce(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()
	events := []string{}
	teardown := protectedTeardownForTest(&events, "retained")
	retainProtectedTeardownForTest(t, boundary, teardown)

	clearErrorBoundary(boundary.errorBoundary)
	runProtectedSubtreeTestAttempt(boundary, func() {})

	assertEffectEvents(t, events, []string{"retained"})
	assertProtectedTeardownState(t, boundary.errorBoundary, 0, 0)

	runProtectedSubtreeTestAttempt(boundary, func() {})
	assertEffectEvents(t, events, []string{"retained"})
}

func TestProtectedTeardownRepeatedFailureDoesNotDuplicateOwnership(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()
	events := []string{}
	teardown := protectedTeardownForTest(&events, "retained")
	retainProtectedTeardownForTest(t, boundary, teardown)

	clearErrorBoundary(boundary.errorBoundary)
	runProtectedSubtreeTestAttempt(boundary, func() {
		if !stageProtectedSubtreeTeardownForTest(teardown) {
			t.Fatal("repeated failed attempt did not retain existing teardown")
		}
		renderComponentInstance(transactionTestRisky(boundary, "repeat capture teardown"))
	})

	if len(events) != 0 {
		t.Fatalf("repeated failed attempt released teardown: %#v", events)
	}
	assertProtectedTeardownState(t, boundary.errorBoundary, 0, 1)

	deactivateComponent(boundary)
	assertEffectEvents(t, events, []string{"retained"})
}

func TestProtectedTeardownNestedSuccessMergesIntoOuter(t *testing.T) {
	isolateLifecycleTestState(t)
	outer := transactionTestBoundary()
	inner := transactionTestBoundaryWithParent(outer)
	events := []string{}
	teardown := protectedTeardownForTest(&events, "inner")

	runProtectedSubtreeTestAttempt(outer, func() {
		runProtectedSubtreeTestAttempt(inner, func() {
			stageProtectedSubtreeTeardownForTest(teardown)
		})
		if len(events) != 0 {
			t.Fatalf("inner success released before outer commit: %#v", events)
		}
		assertProtectedTeardownState(t, inner.errorBoundary, 0, 0)
		assertProtectedTeardownState(t, outer.errorBoundary, 1, 0)
	})

	assertEffectEvents(t, events, []string{"inner"})
	assertProtectedTeardownState(t, outer.errorBoundary, 0, 0)
}

func TestProtectedTeardownOuterCaptureRetainsMergedInnerOwnership(t *testing.T) {
	isolateLifecycleTestState(t)
	outer := transactionTestBoundary()
	inner := transactionTestBoundaryWithParent(outer)
	events := []string{}
	teardown := protectedTeardownForTest(&events, "inner")

	runProtectedSubtreeTestAttempt(outer, func() {
		runProtectedSubtreeTestAttempt(inner, func() {
			stageProtectedSubtreeTeardownForTest(teardown)
		})
		renderComponentInstance(transactionTestRisky(outer, "outer capture teardown"))
	})

	if len(events) != 0 {
		t.Fatalf("outer capture released merged teardown: %#v", events)
	}
	assertProtectedTeardownState(t, outer.errorBoundary, 0, 1)

	deactivateComponent(outer)
	assertEffectEvents(t, events, []string{"inner"})
}

func TestProtectedTeardownFallbackWithoutOuterFinalizesPending(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()
	events := []string{}
	teardown := protectedTeardownForTest(&events, "fallback")
	retainProtectedTeardownForTest(t, boundary, teardown)
	boundary.errorBoundary.phase = errorBoundaryFallback

	finalizePendingProtectedSubtreeTeardownForTest(boundary.errorBoundary)

	assertEffectEvents(t, events, []string{"fallback"})
	assertProtectedTeardownState(t, boundary.errorBoundary, 0, 0)
}

func TestProtectedTeardownFallbackTransfersToOuter(t *testing.T) {
	for _, test := range []struct {
		name    string
		capture bool
	}{
		{name: "outer success"},
		{name: "outer capture", capture: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateLifecycleTestState(t)
			outer := transactionTestBoundary()
			inner := transactionTestBoundaryWithParent(outer)
			events := []string{}
			teardown := protectedTeardownForTest(&events, "fallback")
			retainProtectedTeardownForTest(t, inner, teardown)
			inner.errorBoundary.phase = errorBoundaryFallback

			runProtectedSubtreeTestAttempt(outer, func() {
				finalizePendingProtectedSubtreeTeardownForTest(inner.errorBoundary)
				if len(events) != 0 {
					t.Fatalf("fallback released before outer outcome: %#v", events)
				}
				assertProtectedTeardownState(t, inner.errorBoundary, 0, 0)
				assertProtectedTeardownState(t, outer.errorBoundary, 1, 0)
				if test.capture {
					renderComponentInstance(transactionTestRisky(outer, "outer fallback capture"))
				}
			})

			if test.capture {
				if len(events) != 0 {
					t.Fatalf("captured outer released fallback teardown: %#v", events)
				}
				assertProtectedTeardownState(t, outer.errorBoundary, 0, 1)
				deactivateComponent(outer)
			}
			assertEffectEvents(t, events, []string{"fallback"})
		})
	}
}

func TestProtectedTeardownBoundaryDestructionReleasesPendingOnce(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()
	events := []string{}
	teardown := protectedTeardownForTest(&events, "destroy")
	retainProtectedTeardownForTest(t, boundary, teardown)

	deactivateComponent(boundary)
	deactivateComponent(boundary)

	assertEffectEvents(t, events, []string{"destroy"})
	if boundary.errorBoundary != nil {
		t.Fatal("destroyed boundary retained error boundary state")
	}
}

func TestProtectedTeardownInvariantEscapeRetainsOwnership(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()
	events := []string{}
	teardown := protectedTeardownForTest(&events, "invariant")

	assertPanic(t, "goframe: protected teardown invariant", func() {
		runProtectedSubtreeTestAttempt(boundary, func() {
			stageProtectedSubtreeTeardownForTest(teardown)
			panic("goframe: protected teardown invariant")
		})
	})

	if len(events) != 0 {
		t.Fatalf("invariant escape released teardown: %#v", events)
	}
	if currentProtectedLifecycleBoundary != nil {
		t.Fatal("invariant escape left current boundary installed")
	}
	assertProtectedTeardownState(t, boundary.errorBoundary, 0, 1)

	deactivateComponent(boundary)
	assertEffectEvents(t, events, []string{"invariant"})
}

func TestProtectedTeardownReleaseDetachesJournalBeforeCallbacks(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()
	events := []string{}
	first := protectedTeardownForTestCallback(func() {
		assertProtectedTeardownState(t, boundary.errorBoundary, 0, 0)
		events = append(events, "first")
	})
	second := protectedTeardownForTest(&events, "second")

	runProtectedSubtreeTestAttempt(boundary, func() {
		stageProtectedSubtreeTeardownForTest(first)
		stageProtectedSubtreeTeardownForTest(second)
	})

	assertEffectEvents(t, events, []string{"first", "second"})
}

func TestProtectedTeardownRepeatedStagingReleasesOnce(t *testing.T) {
	isolateLifecycleTestState(t)
	boundary := transactionTestBoundary()
	events := []string{}
	teardown := protectedTeardownForTest(&events, "once")

	runProtectedSubtreeTestAttempt(boundary, func() {
		stageProtectedSubtreeTeardownForTest(teardown)
		stageProtectedSubtreeTeardownForTest(teardown)
		stageProtectedSubtreeTeardownForTest(teardown)
	})

	assertEffectEvents(t, events, []string{"once"})
}

func protectedTeardownForTest(events *[]string, label string) *protectedSubtreeTeardown {
	return protectedTeardownForTestCallback(func() {
		*events = append(*events, label)
	})
}

func retainProtectedTeardownForTest(
	t *testing.T,
	boundary *componentInstance,
	teardown *protectedSubtreeTeardown,
) {
	t.Helper()
	runProtectedSubtreeTestAttempt(boundary, func() {
		if !stageProtectedSubtreeTeardownForTest(teardown) {
			t.Fatal("active protected transaction did not accept teardown")
		}
		renderComponentInstance(transactionTestRisky(boundary, "retain teardown"))
	})
	if boundary.errorBoundary.phase != errorBoundaryCaptured {
		t.Fatalf("boundary phase = %d, want captured", boundary.errorBoundary.phase)
	}
	assertProtectedTeardownState(t, boundary.errorBoundary, 0, 1)
}

func assertProtectedTeardownState(
	t *testing.T,
	state *errorBoundaryState,
	active int,
	pending int,
) {
	t.Helper()
	gotActive, gotPending := protectedTeardownStateForTest(state)
	if gotActive != active {
		t.Fatalf("active teardown journal length = %d, want %d", gotActive, active)
	}
	if gotPending != pending {
		t.Fatalf("pending teardown journal length = %d, want %d", gotPending, pending)
	}
}

package goframe

import "testing"

func TestUseMountRunsOnceAfterRender(t *testing.T) {
	resetEffectsForTest()
	runs := 0
	instance := testComponentInstance("Mount", func() Node {
		UseMount(func() Cleanup {
			runs++
			return nil
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	if runs != 0 {
		t.Fatalf("mount effect ran during render")
	}
	flushPendingEffects()
	renderComponentInstance(instance)
	flushPendingEffects()

	if runs != 1 {
		t.Fatalf("mount effect runs = %d, want 1", runs)
	}
}

func TestUseMountCleanupRunsOnUnmount(t *testing.T) {
	resetEffectsForTest()
	cleanups := 0
	instance := testComponentInstance("MountCleanup", func() Node {
		UseMount(func() Cleanup {
			return func() {
				cleanups++
			}
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	deactivateComponent(instance)

	if cleanups != 1 {
		t.Fatalf("cleanups = %d, want 1", cleanups)
	}
}

func TestUseUnmountCleanupRunsOnUnmount(t *testing.T) {
	resetEffectsForTest()
	cleanups := 0
	instance := testComponentInstance("Unmount", func() Node {
		UseUnmount(func() {
			cleanups++
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	deactivateComponent(instance)

	if cleanups != 1 {
		t.Fatalf("cleanups = %d, want 1", cleanups)
	}
}

func TestUseEffectRunsAfterMount(t *testing.T) {
	resetEffectsForTest()
	runs := 0
	instance := testComponentInstance("Effect", func() Node {
		UseEffect(func() Cleanup {
			runs++
			return nil
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	if runs != 0 {
		t.Fatalf("effect ran during render")
	}
	flushPendingEffects()

	if runs != 1 {
		t.Fatalf("effect runs = %d, want 1", runs)
	}
}

func TestUseEffectRerunsWhenDepsChange(t *testing.T) {
	resetEffectsForTest()
	var value string
	var setValue func(string)
	runs := 0
	cleanups := 0
	instance := testComponentInstance("EffectDeps", func() Node {
		value, setValue = UseState("first")
		UseEffect(func() Cleanup {
			runs++
			return func() {
				cleanups++
			}
		}, Deps(value))
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	setValue("second")
	renderComponentInstance(instance)
	flushPendingEffects()

	if runs != 2 || cleanups != 1 {
		t.Fatalf("runs=%d cleanups=%d, want 2/1", runs, cleanups)
	}
}

func TestUseEffectSkipsWhenDepsUnchanged(t *testing.T) {
	resetEffectsForTest()
	runs := 0
	instance := testComponentInstance("EffectSameDeps", func() Node {
		value, _ := UseState(1)
		UseEffect(func() Cleanup {
			runs++
			return nil
		}, Deps(value))
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	renderComponentInstance(instance)
	flushPendingEffects()

	if runs != 1 {
		t.Fatalf("runs after unchanged deps = %d, want 1", runs)
	}
}

func TestUseEffectAlwaysDepsRunsEveryRender(t *testing.T) {
	resetEffectsForTest()
	runs := 0
	instance := testComponentInstance("AlwaysDeps", func() Node {
		UseEffect(func() Cleanup {
			runs++
			return nil
		}, EveryRender())
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	renderComponentInstance(instance)
	flushPendingEffects()

	if runs != 2 {
		t.Fatalf("runs = %d, want 2", runs)
	}
}

func TestUseEffectCleanupRunsOnUnmount(t *testing.T) {
	resetEffectsForTest()
	cleanups := 0
	instance := testComponentInstance("EffectUnmount", func() Node {
		UseEffect(func() Cleanup {
			return func() {
				cleanups++
			}
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	deactivateComponent(instance)

	if cleanups != 1 {
		t.Fatalf("cleanups = %d, want 1", cleanups)
	}
}

func TestUseEffectStateUpdateSchedulesAfterRender(t *testing.T) {
	resetEffectsForTest()
	schedules := 0
	var value int
	var setValue func(int)
	instance := testComponentInstance("EffectSet", func() Node {
		value, setValue = UseState(0)
		UseEffect(func() Cleanup {
			if value == 0 {
				setValue(1)
			}
			return nil
		})
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})

	renderComponentInstance(instance)
	if value != 0 || schedules != 0 {
		t.Fatalf("state=%d schedules=%d before effect flush, want 0/0", value, schedules)
	}
	flushPendingEffects()

	if got := instance.stateSlots[0].value; got != 1 || !instance.dirty || schedules != 1 {
		t.Fatalf("state=%v dirty=%v schedules=%d, want effect-scheduled update", got, instance.dirty, schedules)
	}
}

func TestUseEffectOutsideComponentPanics(t *testing.T) {
	resetEffectsForTest()
	currentComponent = nil
	defer func() {
		if recovered := recover(); recovered != "goframe: UseEffect must be called during component render" {
			t.Fatalf("panic = %v", recovered)
		}
	}()
	UseEffect(func() Cleanup { return nil })
}

func TestUseUnmountOutsideComponentPanics(t *testing.T) {
	resetEffectsForTest()
	currentComponent = nil
	defer func() {
		if recovered := recover(); recovered != "goframe: UseUnmount must be called during component render" {
			t.Fatalf("panic = %v", recovered)
		}
	}()
	UseUnmount(func() {})
}

func TestLifecycleHookTypeMismatchPanics(t *testing.T) {
	resetEffectsForTest()
	useMount := false
	instance := testComponentInstance("HookMismatch", func() Node {
		if useMount {
			UseMount(func() Cleanup { return nil })
		} else {
			UseEffect(func() Cleanup { return nil })
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	useMount = true
	defer func() {
		if recovered := recover(); recovered != "goframe: lifecycle hook type changed between component renders" {
			t.Fatalf("panic = %v", recovered)
		}
	}()
	renderComponentInstance(instance)
}

func TestUnmountedPendingEffectIsSkipped(t *testing.T) {
	resetEffectsForTest()
	runs := 0
	instance := testComponentInstance("UnmountedPending", func() Node {
		UseEffect(func() Cleanup {
			runs++
			return nil
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	deactivateComponent(instance)
	flushPendingEffects()

	if runs != 0 {
		t.Fatalf("runs = %d, want skipped pending effect", runs)
	}
}

func TestUseEffectUnsupportedDependencyPanics(t *testing.T) {
	resetEffectsForTest()
	instance := testComponentInstance("UnsupportedDep", func() Node {
		UseEffect(func() Cleanup { return nil }, Deps(struct{ Name string }{"bad"}))
		return Empty()
	}, nil)

	defer func() {
		if recovered := recover(); recovered != "goframe: unsupported effect dependency type; reduce complex values to string, id, version, or counter" {
			t.Fatalf("panic = %v", recovered)
		}
	}()
	renderComponentInstance(instance)
}

func TestUseEffectChangedDepsRetryAfterFailedRender(t *testing.T) {
	isolateLifecycleTestState(t)
	events := []string{}
	dep := 1
	label := "A"
	fail := false
	instance := testComponentInstance("EffectTransaction", func() Node {
		committedLabel := label
		UseEffect(func() Cleanup {
			events = append(events, "setup "+committedLabel)
			return func() {
				events = append(events, "cleanup "+committedLabel)
			}
		}, Deps(dep))
		if fail {
			panic("failed render")
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	dep = 2
	label = "B"
	fail = true
	renderComponentInstance(instance)
	flushPendingEffects()
	assertEffectEvents(t, events, []string{"setup A"})

	label = "C"
	fail = false
	renderComponentInstance(instance)
	flushPendingEffects()
	deactivateComponent(instance)

	assertEffectEvents(t, events, []string{
		"setup A",
		"cleanup A",
		"setup C",
		"cleanup C",
	})
}

func TestUseEffectCommittedPendingStateSurvivesFailedRender(t *testing.T) {
	isolateLifecycleTestState(t)
	events := []string{}
	label := "A"
	fail := false
	instance := testComponentInstance("PendingEffectTransaction", func() Node {
		committedLabel := label
		UseEffect(func() Cleanup {
			events = append(events, "setup "+committedLabel)
			return nil
		})
		if fail {
			panic("failed render")
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	if len(pendingEffects) != 1 {
		t.Fatalf("pending effects after committed render = %d, want 1", len(pendingEffects))
	}
	label = "B"
	fail = true
	renderComponentInstance(instance)
	if len(pendingEffects) != 1 {
		t.Fatalf("pending effects after failed render = %d, want preserved queue entry", len(pendingEffects))
	}

	flushPendingEffects()
	assertEffectEvents(t, events, []string{"setup A"})
	if len(pendingEffects) != 0 {
		t.Fatalf("pending effects after flush = %d, want 0", len(pendingEffects))
	}
}

func TestUseEffectSuccessfulRerenderBeforeFlushUsesLatestCommittedClosure(t *testing.T) {
	isolateLifecycleTestState(t)
	events := []string{}
	label := "A"
	instance := testComponentInstance("CoalescedEffectTransaction", func() Node {
		committedLabel := label
		UseEffect(func() Cleanup {
			events = append(events, "setup "+committedLabel)
			return nil
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	label = "B"
	renderComponentInstance(instance)
	if len(pendingEffects) != 1 {
		t.Fatalf("pending effects before flush = %d, want one coalesced slot", len(pendingEffects))
	}
	flushPendingEffects()

	assertEffectEvents(t, events, []string{"setup B"})
}

func TestUseEffectInitialFailedRenderQueuesNothing(t *testing.T) {
	isolateLifecycleTestState(t)
	setups := 0
	fail := true
	instance := testComponentInstance("InitialEffectTransaction", func() Node {
		UseEffect(func() Cleanup {
			setups++
			return nil
		})
		if fail {
			panic("failed render")
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	if setups != 0 || len(instance.effectSlots) != 0 || len(pendingEffects) != 0 {
		t.Fatalf("failed initial render setups=%d slots=%d pending=%d, want 0/0/0",
			setups, len(instance.effectSlots), len(pendingEffects))
	}

	fail = false
	renderComponentInstance(instance)
	flushPendingEffects()
	if setups != 1 || len(instance.effectSlots) != 1 {
		t.Fatalf("successful retry setups=%d slots=%d, want 1/1", setups, len(instance.effectSlots))
	}
}

func TestUseEffectEveryRenderFailedAttemptQueuesNothing(t *testing.T) {
	isolateLifecycleTestState(t)
	events := []string{}
	label := "A"
	fail := false
	instance := testComponentInstance("EveryRenderTransaction", func() Node {
		committedLabel := label
		UseEffect(func() Cleanup {
			events = append(events, "setup "+committedLabel)
			return nil
		}, EveryRender())
		if fail {
			panic("failed render")
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	label = "B"
	fail = true
	renderComponentInstance(instance)
	flushPendingEffects()

	assertEffectEvents(t, events, []string{"setup A"})
	if len(pendingEffects) != 0 {
		t.Fatalf("pending EveryRender effects after failed attempt = %d, want 0", len(pendingEffects))
	}
}

func TestUseUnmountFailedRenderPreservesCommittedCleanup(t *testing.T) {
	isolateLifecycleTestState(t)
	events := []string{}
	label := "A"
	fail := false
	instance := testComponentInstance("UnmountTransaction", func() Node {
		committedLabel := label
		UseUnmount(func() {
			events = append(events, "cleanup "+committedLabel)
		})
		if fail {
			panic("failed render")
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	label = "B"
	fail = true
	renderComponentInstance(instance)
	deactivateComponent(instance)

	assertEffectEvents(t, events, []string{"cleanup A"})
}

func TestUseUnmountInitialFailedRenderCommitsNoCleanup(t *testing.T) {
	isolateLifecycleTestState(t)
	events := []string{}
	failed := testComponentInstance("InitialUnmountFailure", func() Node {
		UseUnmount(func() {
			events = append(events, "cleanup failed")
		})
		panic("failed render")
	}, nil)

	renderComponentInstance(failed)
	if len(failed.unmountSlots) != 0 {
		t.Fatalf("unmount slots after failed initial render = %d, want 0", len(failed.unmountSlots))
	}
	deactivateComponent(failed)
	assertEffectEvents(t, events, nil)

	fail := true
	retry := testComponentInstance("InitialUnmountRetry", func() Node {
		UseUnmount(func() {
			events = append(events, "cleanup committed")
		})
		if fail {
			panic("failed render")
		}
		return Empty()
	}, nil)
	renderComponentInstance(retry)
	fail = false
	renderComponentInstance(retry)
	deactivateComponent(retry)
	assertEffectEvents(t, events, []string{"cleanup committed"})
}

func TestLifecycleRollbackRunsBeforeInvariantRepanic(t *testing.T) {
	isolateLifecycleTestState(t)
	events := []string{}
	dep := 1
	label := "A"
	panicInvariant := false
	instance := testComponentInstance("InvariantLifecycleTransaction", func() Node {
		committedLabel := label
		UseEffect(func() Cleanup {
			events = append(events, "setup "+committedLabel)
			return func() {
				events = append(events, "cleanup "+committedLabel)
			}
		}, Deps(dep))
		UseUnmount(func() {
			events = append(events, "unmount "+committedLabel)
		})
		if panicInvariant {
			UseEffect(func() Cleanup { return nil }, Deps(struct{}{}))
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	dep = 2
	label = "B"
	panicInvariant = true
	assertPanic(t,
		"goframe: unsupported effect dependency type; reduce complex values to string, id, version, or counter",
		func() {
			renderComponentInstance(instance)
		})
	if instance.lifecycleAttempt.active || len(instance.lifecycleAttempt.effects) != 0 || len(instance.lifecycleAttempt.unmounts) != 0 {
		t.Fatalf("lifecycle attempt after invariant panic = %#v, want rolled back", instance.lifecycleAttempt)
	}
	if len(instance.effectSlots) != 1 || !depsEqual(instance.effectSlots[0].deps, Deps(1)) {
		t.Fatalf("committed effect state changed after invariant panic: %#v", instance.effectSlots)
	}

	label = "C"
	panicInvariant = false
	renderComponentInstance(instance)
	flushPendingEffects()
	deactivateComponent(instance)
	assertEffectEvents(t, events, []string{
		"setup A",
		"cleanup A",
		"setup C",
		"cleanup C",
		"unmount C",
	})
}

func isolateLifecycleTestState(t *testing.T) {
	t.Helper()
	previousPending := pendingEffects
	previousCurrent := currentComponent
	previousFlushing := flushingEffects
	pendingEffects = nil
	currentComponent = nil
	flushingEffects = false
	t.Cleanup(func() {
		pendingEffects = previousPending
		currentComponent = previousCurrent
		flushingEffects = previousFlushing
	})
}

func assertEffectEvents(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("events = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("events = %#v, want %#v", got, want)
		}
	}
}

func resetEffectsForTest() {
	pendingEffects = nil
	currentComponent = nil
}

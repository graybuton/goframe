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

func resetEffectsForTest() {
	pendingEffects = nil
	currentComponent = nil
}

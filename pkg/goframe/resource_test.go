package goframe

import (
	"errors"
	"testing"
)

type resourceTestLoader struct {
	starts   []string
	resolves []func(string)
	rejects  []func(error)
	cleanups int
	cleanup  func()
}

func (loader *resourceTestLoader) load(key string, resolve func(string), reject func(error)) Cleanup {
	loader.starts = append(loader.starts, key)
	loader.resolves = append(loader.resolves, resolve)
	loader.rejects = append(loader.rejects, reject)
	return func() {
		loader.cleanups++
		if loader.cleanup != nil {
			loader.cleanup()
		}
	}
}

func TestUseResourceInitialRenderStartsAfterEffect(t *testing.T) {
	resetEffectsForTest()
	loader := &resourceTestLoader{}
	var resource Resource[string]
	instance := testComponentInstance("Resource", func() Node {
		resource, _ = UseResource("open", loader.load)
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	if !resource.Loading() || resource.Value != "" || resource.Err != nil {
		t.Fatalf("initial resource = %#v, want loading zero value", resource)
	}
	if len(loader.starts) != 0 {
		t.Fatalf("loader starts during render = %d, want 0", len(loader.starts))
	}

	flushPendingEffects()
	if got := loader.starts; len(got) != 1 || got[0] != "open" {
		t.Fatalf("loader starts = %#v, want [open]", got)
	}
}

func TestUseResourceResolveProducesReady(t *testing.T) {
	resetEffectsForTest()
	loader := &resourceTestLoader{}
	schedules := 0
	var resource Resource[string]
	instance := testComponentInstance("Resource", func() Node {
		resource, _ = UseResource("open", loader.load)
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})

	renderComponentInstance(instance)
	flushPendingEffects()
	loader.resolves[0]("ready")
	if schedules != 1 || !instance.dirty {
		t.Fatalf("schedules=%d dirty=%v, want one dirty update", schedules, instance.dirty)
	}
	renderComponentInstance(instance)

	if !resource.Ready() || resource.Value != "ready" || resource.Err != nil {
		t.Fatalf("resource after resolve = %#v, want ready value", resource)
	}
}

func TestUseResourceRejectProducesFailed(t *testing.T) {
	resetEffectsForTest()
	loader := &resourceTestLoader{}
	var resource Resource[string]
	instance := testComponentInstance("Resource", func() Node {
		resource, _ = UseResource("missing", loader.load)
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	loader.rejects[0](errors.New("not found"))
	renderComponentInstance(instance)

	if !resource.Failed() || resource.Value != "" || resource.Err == nil || resource.Err.Error() != "not found" {
		t.Fatalf("resource after reject = %#v, want failed not found", resource)
	}
}

func TestUseResourceNilRejectProducesNonNilError(t *testing.T) {
	resetEffectsForTest()
	loader := &resourceTestLoader{}
	var resource Resource[string]
	instance := testComponentInstance("Resource", func() Node {
		resource, _ = UseResource("missing", loader.load)
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	loader.rejects[0](nil)
	renderComponentInstance(instance)

	if !resource.Failed() || resource.Err == nil || resource.Err.Error() != "goframe: resource rejected without error" {
		t.Fatalf("resource after nil reject = %#v, want failed internal error", resource)
	}
}

func TestUseResourceFirstCompletionWins(t *testing.T) {
	resetEffectsForTest()
	loader := &resourceTestLoader{}
	schedules := 0
	var resource Resource[string]
	instance := testComponentInstance("Resource", func() Node {
		resource, _ = UseResource("key", loader.load)
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})

	renderComponentInstance(instance)
	flushPendingEffects()
	loader.resolves[0]("first")
	loader.rejects[0](errors.New("late"))
	loader.resolves[0]("later")
	renderComponentInstance(instance)

	if schedules != 1 {
		t.Fatalf("schedules = %d, want one first-completion update", schedules)
	}
	if !resource.Ready() || resource.Value != "first" || resource.Err != nil {
		t.Fatalf("resource after multiple completions = %#v, want first ready", resource)
	}
}

func TestUseResourceRejectThenResolveKeepsFailed(t *testing.T) {
	resetEffectsForTest()
	loader := &resourceTestLoader{}
	var resource Resource[string]
	instance := testComponentInstance("Resource", func() Node {
		resource, _ = UseResource("key", loader.load)
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	loader.rejects[0](errors.New("first failure"))
	loader.resolves[0]("late")
	renderComponentInstance(instance)

	if !resource.Failed() || resource.Err == nil || resource.Err.Error() != "first failure" {
		t.Fatalf("resource after reject then resolve = %#v, want first failure", resource)
	}
}

func TestUseResourceSameKeyRerenderAndLoaderChangeDoNotReload(t *testing.T) {
	resetEffectsForTest()
	first := &resourceTestLoader{}
	second := &resourceTestLoader{}
	useSecond := false
	instance := testComponentInstance("Resource", func() Node {
		loader := first.load
		if useSecond {
			loader = second.load
		}
		_, _ = UseResource("same", loader)
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	renderComponentInstance(instance)
	flushPendingEffects()
	useSecond = true
	renderComponentInstance(instance)
	flushPendingEffects()

	if got := len(first.starts); got != 1 {
		t.Fatalf("first loader starts = %d, want 1", got)
	}
	if got := len(second.starts); got != 0 {
		t.Fatalf("second loader starts = %d, want 0 before reload/key change", got)
	}
}

func TestUseResourceKeyChangeInvalidatesLateCallbacks(t *testing.T) {
	resetEffectsForTest()
	loader := &resourceTestLoader{}
	key := "a"
	schedules := 0
	var resource Resource[string]
	instance := testComponentInstance("Resource", func() Node {
		resource, _ = UseResource(key, loader.load)
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})

	renderComponentInstance(instance)
	flushPendingEffects()
	oldResolve := loader.resolves[0]
	key = "b"
	renderComponentInstance(instance)

	oldResolve("late-a")
	if schedules != 0 || instance.dirty {
		t.Fatalf("late old resolve schedules=%d dirty=%v, want no stale dirty update", schedules, instance.dirty)
	}
	if !resource.Loading() {
		t.Fatalf("resource after key change before new effect = %#v, want loading", resource)
	}

	flushPendingEffects()
	if loader.cleanups != 1 {
		t.Fatalf("cleanups after key change = %d, want 1", loader.cleanups)
	}
	loader.resolves[1]("ready-b")
	renderComponentInstance(instance)
	if !resource.Ready() || resource.Value != "ready-b" {
		t.Fatalf("resource after new key resolve = %#v, want ready-b", resource)
	}
}

func TestUseResourceLateOldRejectDoesNotDirtyComponent(t *testing.T) {
	resetEffectsForTest()
	loader := &resourceTestLoader{}
	key := "a"
	schedules := 0
	instance := testComponentInstance("Resource", func() Node {
		_, _ = UseResource(key, loader.load)
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})

	renderComponentInstance(instance)
	flushPendingEffects()
	oldReject := loader.rejects[0]
	key = "b"
	renderComponentInstance(instance)
	oldReject(errors.New("late"))

	if schedules != 0 || instance.dirty {
		t.Fatalf("late old reject schedules=%d dirty=%v, want no stale dirty update", schedules, instance.dirty)
	}
}

func TestUseResourceManualReloadInvalidatesLateCallbacks(t *testing.T) {
	resetEffectsForTest()
	loader := &resourceTestLoader{}
	schedules := 0
	var reload func()
	var resource Resource[string]
	instance := testComponentInstance("Resource", func() Node {
		resource, reload = UseResource("same", loader.load)
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})

	renderComponentInstance(instance)
	flushPendingEffects()
	oldResolve := loader.resolves[0]
	reload()
	if schedules != 1 || !instance.dirty {
		t.Fatalf("reload schedules=%d dirty=%v, want one dirty update", schedules, instance.dirty)
	}
	renderComponentInstance(instance)
	oldResolve("late")
	if schedules != 1 || instance.dirty {
		t.Fatalf("late pre-reload resolve schedules=%d dirty=%v, want no extra dirty update", schedules, instance.dirty)
	}
	if !resource.Loading() {
		t.Fatalf("resource after reload render = %#v, want loading", resource)
	}
	flushPendingEffects()
	if loader.cleanups != 1 || len(loader.starts) != 2 {
		t.Fatalf("cleanups=%d starts=%d, want cleanup old and start new", loader.cleanups, len(loader.starts))
	}
}

func TestUseResourceOldReloadClosureUsesLatestKey(t *testing.T) {
	resetEffectsForTest()
	loader := &resourceTestLoader{}
	key := "a"
	var firstReload func()
	instance := testComponentInstance("Resource", func() Node {
		_, reload := UseResource(key, loader.load)
		if firstReload == nil {
			firstReload = reload
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	key = "b"
	renderComponentInstance(instance)
	flushPendingEffects()
	firstReload()
	renderComponentInstance(instance)
	flushPendingEffects()

	if got := loader.starts; len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "b" {
		t.Fatalf("starts after old reload = %#v, want [a b b]", got)
	}
}

func TestUseResourceCleanupOnReloadKeyChangeAndUnmount(t *testing.T) {
	resetEffectsForTest()
	loader := &resourceTestLoader{}
	key := "a"
	var reload func()
	instance := testComponentInstance("Resource", func() Node {
		_, reload = UseResource(key, loader.load)
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	reload()
	renderComponentInstance(instance)
	flushPendingEffects()
	key = "b"
	renderComponentInstance(instance)
	flushPendingEffects()
	deactivateComponent(instance)

	if loader.cleanups != 3 {
		t.Fatalf("cleanups = %d, want one per started generation", loader.cleanups)
	}
}

func TestUseResourceCompletionAfterUnmountIsNoop(t *testing.T) {
	resetEffectsForTest()
	loader := &resourceTestLoader{}
	schedules := 0
	var reload func()
	instance := testComponentInstance("Resource", func() Node {
		_, reload = UseResource("key", loader.load)
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})

	renderComponentInstance(instance)
	flushPendingEffects()
	resolve := loader.resolves[0]
	deactivateComponent(instance)
	resolve("late")
	reload()

	if schedules != 0 || instance.dirty {
		t.Fatalf("after unmount schedules=%d dirty=%v, want no-op", schedules, instance.dirty)
	}
	if loader.cleanups != 1 {
		t.Fatalf("cleanups after unmount = %d, want 1", loader.cleanups)
	}
}

func TestUseResourceCleanupTriggeredCompletionIsIgnored(t *testing.T) {
	resetEffectsForTest()
	loader := &resourceTestLoader{}
	key := "a"
	schedules := 0
	var oldResolve func(string)
	loader.cleanup = func() {
		oldResolve("from-cleanup")
	}
	var resource Resource[string]
	instance := testComponentInstance("Resource", func() Node {
		resource, _ = UseResource(key, loader.load)
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})

	renderComponentInstance(instance)
	flushPendingEffects()
	oldResolve = loader.resolves[0]
	key = "b"
	renderComponentInstance(instance)
	flushPendingEffects()

	if schedules != 0 || instance.dirty {
		t.Fatalf("cleanup-triggered resolve schedules=%d dirty=%v, want no stale update", schedules, instance.dirty)
	}
	if !resource.Loading() {
		t.Fatalf("resource after key change cleanup = %#v, want loading", resource)
	}
}

func TestUseResourceLoaderPanicReportsEffectAndFails(t *testing.T) {
	resetEffectsForTest()
	errorsSeen := captureRuntimeErrors(t)
	starts := 0
	var resource Resource[string]
	instance := testComponentInstance("ResourceExploder", func() Node {
		resource, _ = UseResource("panic", func(string, func(string), func(error)) Cleanup {
			starts++
			panic("loader boom")
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	renderComponentInstance(instance)

	requireRuntimeError(t, errorsSeen(), ErrorPhaseEffect, "ResourceExploder", "UseEffect", "loader boom")
	if !resource.Failed() || resource.Err == nil || resource.Err.Error() != "goframe: resource loader panicked" {
		t.Fatalf("resource after loader panic = %#v, want failed loader panic", resource)
	}
	if starts != 1 || len(errorsSeen()) != 1 {
		t.Fatalf("starts=%d errors=%d, want one loader start and one report", starts, len(errorsSeen()))
	}
	requireResourceEffectSlotCompleted(t, instance)
	flushPendingEffects()
	renderComponentInstance(instance)
	flushPendingEffects()
	if starts != 1 || len(errorsSeen()) != 1 {
		t.Fatalf("after same-key rerenders starts=%d errors=%d, want no automatic retry", starts, len(errorsSeen()))
	}
	if !resource.Failed() || resource.Err == nil || resource.Err.Error() != "goframe: resource loader panicked" {
		t.Fatalf("resource after same-key rerenders = %#v, want stable failed state", resource)
	}
}

func TestUseResourceLoaderPanicParentRerenderDoesNotRetry(t *testing.T) {
	resetEffectsForTest()
	errorsSeen := captureRuntimeErrors(t)
	starts := 0
	var resource Resource[string]
	instance := testComponentInstance("ResourceParentRerender", func() Node {
		resource, _ = UseResource("panic", func(string, func(string), func(error)) Cleanup {
			starts++
			panic("loader boom")
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	renderComponentInstance(instance)
	flushPendingEffects()
	renderComponentInstance(instance)
	flushPendingEffects()

	if starts != 1 || len(errorsSeen()) != 1 {
		t.Fatalf("parent rerenders starts=%d errors=%d, want no automatic retry", starts, len(errorsSeen()))
	}
	if !resource.Failed() {
		t.Fatalf("resource after parent rerenders = %#v, want failed", resource)
	}
	requireResourceEffectSlotCompleted(t, instance)
}

func TestUseResourceResolveThenPanicKeepsReadyAndDoesNotRetry(t *testing.T) {
	resetEffectsForTest()
	errorsSeen := captureRuntimeErrors(t)
	starts := 0
	var resource Resource[string]
	instance := testComponentInstance("ResourceResolveThenPanic", func() Node {
		resource, _ = UseResource("key", func(_ string, resolve func(string), _ func(error)) Cleanup {
			starts++
			resolve("ready")
			panic("late panic")
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	renderComponentInstance(instance)
	flushPendingEffects()
	renderComponentInstance(instance)
	flushPendingEffects()

	requireRuntimeError(t, errorsSeen(), ErrorPhaseEffect, "ResourceResolveThenPanic", "UseEffect", "late panic")
	if starts != 1 || len(errorsSeen()) != 1 {
		t.Fatalf("starts=%d errors=%d, want one start/report", starts, len(errorsSeen()))
	}
	if !resource.Ready() || resource.Value != "ready" || resource.Err != nil {
		t.Fatalf("resource after resolve then panic = %#v, want ready first completion", resource)
	}
	requireResourceEffectSlotCompleted(t, instance)
}

func TestUseResourceRejectThenPanicKeepsOriginalErrorAndDoesNotRetry(t *testing.T) {
	resetEffectsForTest()
	errorsSeen := captureRuntimeErrors(t)
	original := errors.New("original failure")
	starts := 0
	var resource Resource[string]
	instance := testComponentInstance("ResourceRejectThenPanic", func() Node {
		resource, _ = UseResource("key", func(_ string, _ func(string), reject func(error)) Cleanup {
			starts++
			reject(original)
			panic("late panic")
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	renderComponentInstance(instance)
	flushPendingEffects()
	renderComponentInstance(instance)
	flushPendingEffects()

	requireRuntimeError(t, errorsSeen(), ErrorPhaseEffect, "ResourceRejectThenPanic", "UseEffect", "late panic")
	if starts != 1 || len(errorsSeen()) != 1 {
		t.Fatalf("starts=%d errors=%d, want one start/report", starts, len(errorsSeen()))
	}
	if !resource.Failed() || resource.Err != original {
		t.Fatalf("resource after reject then panic = %#v, want original failure", resource)
	}
	requireResourceEffectSlotCompleted(t, instance)
}

func TestUseResourceManualReloadAfterLoaderPanicRetriesExplicitly(t *testing.T) {
	resetEffectsForTest()
	errorsSeen := captureRuntimeErrors(t)
	starts := 0
	var reload func()
	var resource Resource[string]
	instance := testComponentInstance("ResourceReloadPanic", func() Node {
		resource, reload = UseResource("key", func(string, func(string), func(error)) Cleanup {
			starts++
			panic("loader boom")
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	renderComponentInstance(instance)
	flushPendingEffects()
	if starts != 1 || len(errorsSeen()) != 1 || !resource.Failed() {
		t.Fatalf("before reload starts=%d errors=%d resource=%#v, want first failed generation", starts, len(errorsSeen()), resource)
	}

	reload()
	renderComponentInstance(instance)
	if !resource.Loading() {
		t.Fatalf("resource after reload render = %#v, want loading", resource)
	}
	flushPendingEffects()
	renderComponentInstance(instance)
	flushPendingEffects()

	if starts != 2 || len(errorsSeen()) != 2 {
		t.Fatalf("after reload starts=%d errors=%d, want explicit second generation only", starts, len(errorsSeen()))
	}
	if !resource.Failed() || resource.Err == nil || resource.Err.Error() != "goframe: resource loader panicked" {
		t.Fatalf("resource after reload panic = %#v, want failed", resource)
	}
	requireResourceEffectSlotCompleted(t, instance)
}

func TestUseResourceKeyChangeAfterLoaderPanicRetriesExplicitly(t *testing.T) {
	resetEffectsForTest()
	errorsSeen := captureRuntimeErrors(t)
	key := "a"
	starts := []string{}
	resolves := map[string]func(string){}
	var resource Resource[string]
	instance := testComponentInstance("ResourceKeyPanic", func() Node {
		resource, _ = UseResource(key, func(key string, resolve func(string), _ func(error)) Cleanup {
			starts = append(starts, key)
			resolves[key] = resolve
			panic("loader boom " + key)
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	renderComponentInstance(instance)
	flushPendingEffects()
	key = "b"
	renderComponentInstance(instance)
	if !resource.Loading() {
		t.Fatalf("resource after key change render = %#v, want loading", resource)
	}
	flushPendingEffects()
	resolves["a"]("late-a")
	renderComponentInstance(instance)
	flushPendingEffects()

	if len(starts) != 2 || starts[0] != "a" || starts[1] != "b" {
		t.Fatalf("starts = %#v, want [a b]", starts)
	}
	if len(errorsSeen()) != 2 {
		t.Fatalf("errors = %d, want two key-change reports", len(errorsSeen()))
	}
	if !resource.Failed() || resource.Value != "" {
		t.Fatalf("resource after key change panic and old resolve = %#v, want failed B and no late A value", resource)
	}
	requireResourceEffectSlotCompleted(t, instance)
}

func TestUseResourceLoaderPanicDoesNotRegisterCleanup(t *testing.T) {
	resetEffectsForTest()
	errorsSeen := captureRuntimeErrors(t)
	starts := 0
	cleanups := 0
	key := "a"
	var reload func()
	instance := testComponentInstance("ResourcePanicCleanup", func() Node {
		_, reload = UseResource(key, func(string, func(string), func(error)) Cleanup {
			starts++
			panic("loader boom")
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	renderComponentInstance(instance)
	reload()
	renderComponentInstance(instance)
	flushPendingEffects()
	key = "b"
	renderComponentInstance(instance)
	flushPendingEffects()
	deactivateComponent(instance)

	if starts != 3 || len(errorsSeen()) != 3 {
		t.Fatalf("starts=%d errors=%d, want explicit initial/reload/key generations", starts, len(errorsSeen()))
	}
	if cleanups != 0 {
		t.Fatalf("cleanups = %d, want no synthetic cleanup after panic before return", cleanups)
	}
}

func TestUseResourceCleanupPanicReportsEffectCleanup(t *testing.T) {
	resetEffectsForTest()
	errorsSeen := captureRuntimeErrors(t)
	key := "a"
	instance := testComponentInstance("ResourceCleanupExploder", func() Node {
		_, _ = UseResource("a", func(string, func(string), func(error)) Cleanup {
			return func() {
				panic("cleanup boom")
			}
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()
	key = "b"
	instance.node = Component("ResourceCleanupExploder", struct{}{}, func(struct{}) Node {
		_, _ = UseResource(key, func(string, func(string), func(error)) Cleanup {
			return nil
		})
		return Empty()
	}).(ComponentNode)
	renderComponentInstance(instance)
	flushPendingEffects()

	requireRuntimeError(t, errorsSeen(), ErrorPhaseEffectCleanup, "ResourceCleanupExploder", "UseEffect cleanup", "cleanup boom")
}

func TestUseResourceRejectDoesNotActivateErrorBoundary(t *testing.T) {
	resetEffectsForTest()
	errorsSeen := captureRuntimeErrors(t)
	loader := &resourceTestLoader{}
	var resource Resource[string]
	boundary := testErrorBoundaryInstance("", func(ErrorBoundaryContext) Node {
		return Text("fallback")
	}, nil)
	child := testComponentInstanceWithParent("ResourceChild", boundary, func() Node {
		resource, _ = UseResource("missing", loader.load)
		return Empty()
	})

	renderComponentInstance(boundary)
	renderComponentInstance(child)
	flushPendingEffects()
	loader.rejects[0](errors.New("ordinary load failure"))
	renderComponentInstance(child)

	if boundary.errorBoundary.phase != errorBoundaryProtected {
		t.Fatalf("boundary phase after resource reject = %d, want protected", boundary.errorBoundary.phase)
	}
	if len(errorsSeen()) != 0 {
		t.Fatalf("runtime errors after ordinary resource reject = %d, want 0", len(errorsSeen()))
	}
	if !resource.Failed() {
		t.Fatalf("resource after ordinary reject = %#v, want failed UI state", resource)
	}
}

func TestUseResourceLoaderPanicDoesNotActivateErrorBoundary(t *testing.T) {
	resetEffectsForTest()
	errorsSeen := captureRuntimeErrors(t)
	var resource Resource[string]
	boundary := testErrorBoundaryInstance("", func(ErrorBoundaryContext) Node {
		return Text("fallback")
	}, nil)
	child := testComponentInstanceWithParent("ResourceChild", boundary, func() Node {
		resource, _ = UseResource("panic", func(string, func(string), func(error)) Cleanup {
			panic("loader boom")
		})
		return Empty()
	})

	renderComponentInstance(boundary)
	renderComponentInstance(child)
	flushPendingEffects()
	renderComponentInstance(child)

	if boundary.errorBoundary.phase != errorBoundaryProtected {
		t.Fatalf("boundary phase after resource loader panic = %d, want protected", boundary.errorBoundary.phase)
	}
	requireRuntimeError(t, errorsSeen(), ErrorPhaseEffect, "ResourceChild", "UseEffect", "loader boom")
	if len(errorsSeen()) != 1 {
		t.Fatalf("runtime errors after loader panic = %d, want 1", len(errorsSeen()))
	}
	if !resource.Failed() {
		t.Fatalf("resource after loader panic = %#v, want failed UI state", resource)
	}
}

func TestUseResourceDirtyUpdatePiercesMemoizedAncestor(t *testing.T) {
	resetEffectsForTest()
	loader := &resourceTestLoader{}
	parent := dirtyCleanInstance("MemoParent", nil)
	child := testComponentInstanceWithParent("ResourceChild", parent, func() Node {
		_, _ = UseResource("key", loader.load)
		return Empty()
	})
	renderComponentInstance(child)
	flushPendingEffects()

	loader.resolves[0]("ready")
	if parent.dirtyDescendants != 1 || !child.dirty {
		t.Fatalf("parent dirty descendants=%d child dirty=%v, want resource update visible through memo ancestor",
			parent.dirtyDescendants, child.dirty)
	}
}

func TestUseResourceHookKindDiagnostics(t *testing.T) {
	resetEffectsForTest()
	useResource := false
	loader := &resourceTestLoader{}
	instance := testComponentInstance("ResourceKind", func() Node {
		if useResource {
			_, _ = UseResource("key", loader.load)
		} else {
			_, _ = UseState(0)
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	useResource = true
	assertPanic(t, "goframe: hook at state slot 0 changed from UseState to UseResource", func() {
		renderComponentInstance(instance)
	})
}

func TestUseResourceNilLoaderPanics(t *testing.T) {
	resetEffectsForTest()
	instance := testComponentInstance("ResourceNilLoader", func() Node {
		_, _ = UseResource[string]("key", nil)
		return Empty()
	}, nil)

	assertPanic(t, "goframe: UseResource requires a loader", func() {
		renderComponentInstance(instance)
	})
}

func requireResourceEffectSlotCompleted(t *testing.T, instance *componentInstance) {
	t.Helper()
	if len(instance.effectSlots) != 1 {
		t.Fatalf("effect slots = %d, want 1", len(instance.effectSlots))
	}
	slot := instance.effectSlots[0]
	if !slot.hasRun || slot.pending || slot.queued || slot.running || slot.cleanup != nil {
		t.Fatalf("resource effect slot = %#v, want hasRun=true pending=false queued=false running=false cleanup=nil", slot)
	}
}

package goframe

import "testing"

type panicMemoPropsFixture struct{}

func (panicMemoPropsFixture) MemoEqual(panicMemoPropsFixture) bool {
	panic("memo boom")
}

func TestSetErrorHandlerReportsAndRestores(t *testing.T) {
	errors := []ErrorInfo{}
	restore := SetErrorHandler(func(info ErrorInfo) {
		errors = append(errors, info)
	})

	reportRuntimeError(ErrorInfo{
		Phase:     ErrorPhaseEvent,
		Component: "Button",
		Operation: "click",
		Panic:     "boom",
	})
	if len(errors) != 1 || errors[0].Phase != ErrorPhaseEvent || errors[0].Panic != "boom" {
		t.Fatalf("errors = %#v, want one event error", errors)
	}

	restore()
	reportRuntimeError(ErrorInfo{Phase: ErrorPhaseEvent, Panic: "ignored"})
	if len(errors) != 1 {
		t.Fatalf("errors after restore = %d, want unchanged", len(errors))
	}
}

func TestRenderPanicReportsAndReturnsEmpty(t *testing.T) {
	errors := captureRuntimeErrors(t)
	instance := testComponentInstance("Exploder", func() Node {
		panic("render boom")
	}, nil)

	node := renderComponentInstance(instance)

	if _, ok := node.(EmptyNode); !ok {
		t.Fatalf("render fallback = %#v, want EmptyNode", node)
	}
	requireRuntimeError(t, errors(), ErrorPhaseRender, "Exploder", "component render", "render boom")
}

func TestEffectSetupPanicReportsWithoutRegisteringCleanup(t *testing.T) {
	resetEffectsForTest()
	errors := captureRuntimeErrors(t)
	instance := testComponentInstance("EffectExploder", func() Node {
		UseEffect(func() Cleanup {
			panic("effect boom")
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	flushPendingEffects()

	requireRuntimeError(t, errors(), ErrorPhaseEffect, "EffectExploder", "UseEffect", "effect boom")
	if slot := instance.effectSlots[0]; slot.hasRun || slot.cleanup != nil || slot.running {
		t.Fatalf("effect slot after panic = %#v, want not run and no cleanup", slot)
	}
}

func TestEffectCleanupPanicReportsAndContinues(t *testing.T) {
	resetEffectsForTest()
	errors := captureRuntimeErrors(t)
	cleanups := 0
	instance := testComponentInstance("CleanupExploder", func() Node {
		UseEffect(func() Cleanup {
			return func() {
				panic("cleanup boom")
			}
		})
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

	requireRuntimeError(t, errors(), ErrorPhaseEffectCleanup, "CleanupExploder", "UseEffect cleanup", "cleanup boom")
	if cleanups != 1 {
		t.Fatalf("remaining cleanups = %d, want 1", cleanups)
	}
}

func TestUnmountCleanupPanicReportsAndContinues(t *testing.T) {
	resetEffectsForTest()
	errors := captureRuntimeErrors(t)
	cleanups := 0
	instance := testComponentInstance("UnmountExploder", func() Node {
		UseUnmount(func() {
			panic("unmount boom")
		})
		UseUnmount(func() {
			cleanups++
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	deactivateComponent(instance)

	requireRuntimeError(t, errors(), ErrorPhaseUnmountCleanup, "UnmountExploder", "UseUnmount cleanup", "unmount boom")
	if cleanups != 1 {
		t.Fatalf("remaining unmount cleanups = %d, want 1", cleanups)
	}
}

func TestMemoComparatorPanicReportsAndDoesNotSkip(t *testing.T) {
	errors := captureRuntimeErrors(t)
	node := Component("MemoExploder", panicMemoPropsFixture{}, func(panicMemoPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	instance := newComponentInstance(node, "row-1", nil, nil)
	instance.dirty = false
	next := Component("MemoExploder", panicMemoPropsFixture{}, func(panicMemoPropsFixture) Node {
		return Empty()
	}).(ComponentNode)

	if shouldSkipComponentRender(instance, next, "row-1") {
		t.Fatal("memo comparator panic must fall back to no skip")
	}
	requireRuntimeError(t, errors(), ErrorPhaseMemo, "MemoExploder", "MemoEqual", "memo boom")
}

func TestContextSelectorInitialPanicReportsContextAndRender(t *testing.T) {
	errors := captureRuntimeErrors(t)
	ctx := CreateContext(contextValueFixture{Count: 1})
	consumer := contextTestInstance("SelectorExploder", nil, func() {
		_ = UseContextSelector(ctx, func(contextValueFixture) int {
			panic("selector boom")
		})
	})

	node := renderComponentInstance(consumer)

	if _, ok := node.(EmptyNode); !ok {
		t.Fatalf("render fallback = %#v, want EmptyNode", node)
	}
	requireRuntimeError(t, errors(), ErrorPhaseContext, "SelectorExploder", "UseContextSelector", "selector boom")
	requireRuntimeError(t, errors(), ErrorPhaseRender, "SelectorExploder", "component render", "selector boom")
}

func TestContextSelectorUpdatePanicReportsAndKeepsConsumerClean(t *testing.T) {
	errors := captureRuntimeErrors(t)
	ctx := CreateContext(contextValueFixture{Count: 1})
	provider := contextProviderInstance("Provider", nil, ctx, contextValueFixture{Count: 1})
	renderComponentInstance(provider)
	panicOnUpdate := false
	consumer := contextSelectorConsumer(provider, ctx, func(value contextValueFixture) int {
		if panicOnUpdate {
			panic("selector update boom")
		}
		return value.Count
	})
	renderComponentInstance(consumer)

	panicOnUpdate = true
	updateContextProvider(provider, ctx, contextValueFixture{Count: 2})

	requireRuntimeError(t, errors(), ErrorPhaseContext, "Consumer", "UseContextSelector", "selector update boom")
	if consumer.dirty {
		t.Fatal("consumer should stay clean when selector update panics")
	}
	if got := consumer.contextSlots[0].selected; got != 1 {
		t.Fatalf("selected after selector panic = %#v, want previous 1", got)
	}
}

func TestVirtualRenderCallbacksReportAndFallback(t *testing.T) {
	errors := captureRuntimeErrors(t)

	listItem := renderVirtualListItem(func(VirtualItem[int]) Node {
		panic("item boom")
	}, VirtualItem[int]{Item: 1})
	if _, ok := listItem.(EmptyNode); !ok {
		t.Fatalf("list item fallback = %#v, want EmptyNode", listItem)
	}

	row := renderVirtualTableRow(func(VirtualRow[int]) Node {
		panic("row boom")
	}, VirtualRow[int]{Item: 1}, 7)
	rowNode := requireVNode(t, row)
	if rowNode.Tag != "tr" {
		t.Fatalf("row fallback tag = %q, want tr", rowNode.Tag)
	}

	requireRuntimeError(t, errors(), ErrorPhaseRender, "VirtualList", "VirtualList.RenderItem", "item boom")
	requireRuntimeError(t, errors(), ErrorPhaseRender, "VirtualTable", "VirtualTable.RenderRow", "row boom")
}

func captureRuntimeErrors(t *testing.T) func() []ErrorInfo {
	t.Helper()
	errors := []ErrorInfo{}
	restore := SetErrorHandler(func(info ErrorInfo) {
		errors = append(errors, info)
	})
	t.Cleanup(restore)
	return func() []ErrorInfo {
		return errors
	}
}

func requireRuntimeError(t *testing.T, errors []ErrorInfo, phase ErrorPhase, component string, operation string, panicValue any) {
	t.Helper()
	for _, info := range errors {
		if info.Phase == phase && info.Component == component && info.Operation == operation && info.Panic == panicValue {
			return
		}
	}
	t.Fatalf("missing runtime error phase=%s component=%q operation=%q panic=%v in %#v",
		phase.String(), component, operation, panicValue, errors)
}

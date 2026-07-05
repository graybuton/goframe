package goframe

import "testing"

func TestUseStatePersistsWithinComponent(t *testing.T) {
	var value int
	var setValue func(int)
	instance := testComponentInstance("Counter", func() Node {
		value, setValue = UseState(0)
		return Text(ToString(value))
	}, nil)

	renderComponentInstance(instance)
	setValue(7)
	renderComponentInstance(instance)

	if value != 7 {
		t.Fatalf("state after component rerender = %d, want 7", value)
	}
}

func TestUseStateIsComponentScoped(t *testing.T) {
	var firstValue, secondValue int
	var setFirst func(int)
	first := testComponentInstance("First", func() Node {
		firstValue, setFirst = UseState(1)
		return Empty()
	}, nil)
	second := testComponentInstance("Second", func() Node {
		secondValue, _ = UseState(2)
		return Empty()
	}, nil)

	renderComponentInstance(first)
	renderComponentInstance(second)
	setFirst(10)
	renderComponentInstance(first)
	renderComponentInstance(second)

	if firstValue != 10 || secondValue != 2 {
		t.Fatalf("component states = %d, %d; want 10, 2", firstValue, secondValue)
	}
}

func TestUseStateSupportsMultipleSlots(t *testing.T) {
	var count int
	var label string
	var setCount func(int)
	var setLabel func(string)
	instance := testComponentInstance("Multi", func() Node {
		count, setCount = UseState(0)
		label, setLabel = UseState("first")
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	setCount(2)
	setLabel("second")
	renderComponentInstance(instance)

	if count != 2 || label != "second" {
		t.Fatalf("slots = %d, %q; want 2, second", count, label)
	}
}

func TestUseStateOutsideComponentPanics(t *testing.T) {
	currentComponent = nil
	defer func() {
		if recovered := recover(); recovered != "goframe: UseState must be called during component render" {
			t.Fatalf("panic = %v", recovered)
		}
	}()
	UseState(0)
}

func TestStateMarksOwnerDirtyAndSchedulesIt(t *testing.T) {
	var setState func(int)
	var scheduled *componentInstance
	instance := testComponentInstance("Owner", func() Node {
		_, setState = UseState(0)
		return Empty()
	}, func(instance *componentInstance) {
		scheduled = instance
	})
	renderComponentInstance(instance)

	setState(1)

	if !instance.dirty || scheduled != instance {
		t.Fatalf("dirty=%v scheduled=%p owner=%p", instance.dirty, scheduled, instance)
	}
}

func TestStateSetSameComparableValueDoesNotSchedule(t *testing.T) {
	var setState func(string)
	schedules := 0
	instance := testComponentInstance("Owner", func() Node {
		_, setState = UseState("same")
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})
	renderComponentInstance(instance)

	setState("same")

	if instance.dirty || schedules != 0 {
		t.Fatalf("dirty=%v schedules=%d, want no-op", instance.dirty, schedules)
	}
}

func TestDirtyChildDoesNotMarkRootOrSibling(t *testing.T) {
	var setState func(int)
	rootSchedules := 0
	childSchedules := 0
	siblingSchedules := 0
	root := testComponentInstance("Root", func() Node { return Empty() }, func(*componentInstance) {
		rootSchedules++
	})
	child := testComponentInstance("Child", func() Node {
		_, setState = UseState(0)
		return Empty()
	}, func(*componentInstance) {
		childSchedules++
	})
	sibling := testComponentInstance("Sibling", func() Node { return Empty() }, func(*componentInstance) {
		siblingSchedules++
	})
	renderComponentInstance(root)
	renderComponentInstance(child)
	renderComponentInstance(sibling)

	setState(1)

	if root.dirty || sibling.dirty || rootSchedules != 0 || siblingSchedules != 0 {
		t.Fatalf("root dirty=%v schedules=%d, sibling dirty=%v schedules=%d",
			root.dirty, rootSchedules, sibling.dirty, siblingSchedules)
	}
	if !child.dirty || childSchedules != 1 {
		t.Fatalf("child dirty=%v schedules=%d, want dirty scheduled child", child.dirty, childSchedules)
	}
}

func TestStateSetDuringRenderRemainsDirty(t *testing.T) {
	schedules := 0
	instance := testComponentInstance("RenderSet", func() Node {
		value, setValue := UseState(0)
		if value == 0 {
			setValue(1)
		}
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})

	renderComponentInstance(instance)

	if !instance.dirty || schedules != 1 {
		t.Fatalf("dirty=%v schedules=%d, want dirty scheduled component", instance.dirty, schedules)
	}
}

func TestUnmountedStateDoesNotSchedule(t *testing.T) {
	var setState func(int)
	schedules := 0
	instance := testComponentInstance("Unmounted", func() Node {
		_, setState = UseState(0)
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})
	renderComponentInstance(instance)
	deactivateComponent(instance)

	setState(1)

	if schedules != 0 || instance.dirty {
		t.Fatalf("schedules=%d dirty=%v, want inactive", schedules, instance.dirty)
	}
}

func TestUseReducerReturnsInitialState(t *testing.T) {
	var value int
	instance := testComponentInstance("Reducer", func() Node {
		value, _ = UseReducer(3, func(state int, action int) int {
			return state + action
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)

	if value != 3 {
		t.Fatalf("reducer initial state = %d, want 3", value)
	}
}

func TestUseReducerDispatchAppliesToLatestState(t *testing.T) {
	var value int
	var dispatch func(int)
	instance := testComponentInstance("Reducer", func() Node {
		value, dispatch = UseReducer(0, func(state int, action int) int {
			return state + action
		})
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	dispatch(1)
	dispatch(1)
	renderComponentInstance(instance)

	if value != 2 {
		t.Fatalf("reducer state = %d, want 2", value)
	}
}

func TestUseReducerStaleDispatchUsesLatestState(t *testing.T) {
	var value int
	var firstDispatch func(int)
	var latestDispatch func(int)
	instance := testComponentInstance("Reducer", func() Node {
		value, latestDispatch = UseReducer(0, func(state int, action int) int {
			return state + action
		})
		if firstDispatch == nil {
			firstDispatch = latestDispatch
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	latestDispatch(5)
	renderComponentInstance(instance)
	firstDispatch(7)
	renderComponentInstance(instance)

	if value != 12 {
		t.Fatalf("stale dispatch reducer state = %d, want 12", value)
	}
}

func TestUseReducerStaleDispatchUsesLatestReducer(t *testing.T) {
	var value int
	var firstDispatch func(int)
	var currentDispatch func(int)
	useMultiplier := false
	instance := testComponentInstance("Reducer", func() Node {
		if useMultiplier {
			value, currentDispatch = UseReducer(0, func(state int, action int) int {
				return state + action*10
			})
		} else {
			value, currentDispatch = UseReducer(0, func(state int, action int) int {
				return state + action
			})
		}
		if firstDispatch == nil {
			firstDispatch = currentDispatch
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	currentDispatch(1)
	renderComponentInstance(instance)
	useMultiplier = true
	renderComponentInstance(instance)
	firstDispatch(1)
	renderComponentInstance(instance)

	if value != 11 {
		t.Fatalf("stale dispatch used wrong reducer, state = %d, want 11", value)
	}
}

func TestUseReducerStateTypeMismatchPanics(t *testing.T) {
	useString := false
	instance := testComponentInstance("Reducer", func() Node {
		if useString {
			_, _ = UseReducer("", func(state string, action string) string {
				return state + action
			})
		} else {
			_, _ = UseReducer(0, func(state int, action int) int {
				return state + action
			})
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	useString = true

	assertPanic(t, "goframe: UseReducer state type changed between component renders", func() {
		renderComponentInstance(instance)
	})
}

func TestStateHookKindMismatchPanics(t *testing.T) {
	useReducer := false
	instance := testComponentInstance("StateKind", func() Node {
		if useReducer {
			_, _ = UseReducer(0, func(state int, action int) int {
				return state + action
			})
		} else {
			_, _ = UseState(0)
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	useReducer = true

	assertPanic(t, "goframe: hook at state slot 0 changed from UseState to UseReducer", func() {
		renderComponentInstance(instance)
	})
}

func TestReducerHookKindMismatchPanics(t *testing.T) {
	useState := false
	instance := testComponentInstance("ReducerKind", func() Node {
		if useState {
			_, _ = UseState(0)
		} else {
			_, _ = UseReducer(0, func(state int, action int) int {
				return state + action
			})
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	useState = true

	assertPanic(t, "goframe: hook at state slot 0 changed from UseReducer to UseState", func() {
		renderComponentInstance(instance)
	})
}

func TestUseReducerReducerTypeMismatchPanics(t *testing.T) {
	useStringAction := false
	instance := testComponentInstance("Reducer", func() Node {
		if useStringAction {
			_, _ = UseReducer(0, func(state int, action string) int {
				if action == "" {
					return state
				}
				return state + 1
			})
		} else {
			_, _ = UseReducer(0, func(state int, action int) int {
				return state + action
			})
		}
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	useStringAction = true

	assertPanic(t, "goframe: UseReducer reducer type changed between component renders", func() {
		renderComponentInstance(instance)
	})
}

func TestUseReducerOutsideComponentPanics(t *testing.T) {
	currentComponent = nil
	assertPanic(t, "goframe: UseReducer must be called during component render", func() {
		UseReducer(0, func(state int, action int) int {
			return state + action
		})
	})
}

func TestUseReducerNilReducerPanics(t *testing.T) {
	instance := testComponentInstance("Reducer", func() Node {
		_, _ = UseReducer[int, int](0, nil)
		return Empty()
	}, nil)

	assertPanic(t, "goframe: UseReducer reducer must not be nil", func() {
		renderComponentInstance(instance)
	})
}

func TestUseReducerDispatchAfterUnmountDoesNotSchedule(t *testing.T) {
	var dispatch func(int)
	reducerCalls := 0
	schedules := 0
	instance := testComponentInstance("Reducer", func() Node {
		_, dispatch = UseReducer(0, func(state int, action int) int {
			reducerCalls++
			return state + action
		})
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})
	renderComponentInstance(instance)
	deactivateComponent(instance)

	dispatch(1)

	if schedules != 0 || reducerCalls != 0 || instance.dirty {
		t.Fatalf("schedules=%d reducerCalls=%d dirty=%v, want inactive no-op", schedules, reducerCalls, instance.dirty)
	}
}

func TestUseReducerDispatchDuringRenderRemainsDirty(t *testing.T) {
	schedules := 0
	instance := testComponentInstance("Reducer", func() Node {
		value, dispatch := UseReducer(0, func(state int, action int) int {
			return state + action
		})
		if value == 0 {
			dispatch(1)
		}
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})

	renderComponentInstance(instance)

	if !instance.dirty || schedules != 1 {
		t.Fatalf("dirty=%v schedules=%d, want dirty scheduled component", instance.dirty, schedules)
	}
}

func TestUseReducerDispatchSamePrimitiveValueDoesNotSchedule(t *testing.T) {
	var dispatch func(int)
	schedules := 0
	instance := testComponentInstance("Reducer", func() Node {
		_, dispatch = UseReducer(1, func(state int, action int) int {
			return state
		})
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})
	renderComponentInstance(instance)

	dispatch(1)

	if instance.dirty || schedules != 0 {
		t.Fatalf("dirty=%v schedules=%d, want no-op", instance.dirty, schedules)
	}
}

func TestUpdateBatchCoalescesPendingRequests(t *testing.T) {
	var batch updateBatch
	var queued []func()
	enqueue := func(update func()) {
		queued = append(queued, update)
	}
	updates := 0

	batch.request(enqueue, func() { updates++ })
	batch.request(enqueue, func() { updates++ })
	batch.request(enqueue, func() { updates++ })

	if len(queued) != 1 {
		t.Fatalf("queued updates = %d, want 1", len(queued))
	}
	if updates != 0 {
		t.Fatalf("updates before queued callback = %d, want 0", updates)
	}
	queued[0]()
	if updates != 1 {
		t.Fatalf("updates = %d, want 1", updates)
	}
}

func TestUpdateBatchAllowsRequestAfterFlush(t *testing.T) {
	var batch updateBatch
	var queued []func()
	enqueue := func(update func()) {
		queued = append(queued, update)
	}
	updates := 0

	batch.request(enqueue, func() { updates++ })
	queued[0]()

	batch.request(enqueue, func() { updates++ })
	if len(queued) != 2 {
		t.Fatalf("queued updates after flush = %d, want 2", len(queued))
	}
	queued[1]()
	if updates != 2 {
		t.Fatalf("updates = %d, want 2", updates)
	}
}

func TestUpdateBatchResetClearsPendingRequest(t *testing.T) {
	var batch updateBatch
	var queued []func()
	enqueue := func(update func()) {
		queued = append(queued, update)
	}

	batch.request(enqueue, func() {})
	batch.reset()
	batch.request(enqueue, func() {})

	if len(queued) != 2 {
		t.Fatalf("queued updates after reset = %d, want 2", len(queued))
	}
}

func TestUpdateBatchResetInvalidatesQueuedRequest(t *testing.T) {
	var batch updateBatch
	var queued []func()
	enqueue := func(update func()) {
		queued = append(queued, update)
	}
	updateA := 0
	updateB := 0

	batch.request(enqueue, func() { updateA++ })
	if len(queued) != 1 {
		t.Fatalf("queued updates = %d, want 1", len(queued))
	}
	if updateA != 0 {
		t.Fatalf("updateA before queued callback = %d, want 0", updateA)
	}

	batch.reset()
	batch.request(enqueue, func() { updateB++ })
	if len(queued) != 2 {
		t.Fatalf("queued updates after reset = %d, want 2", len(queued))
	}

	queued[0]()
	if updateA != 0 {
		t.Fatalf("stale updateA ran %d time(s), want 0", updateA)
	}
	if updateB != 0 {
		t.Fatalf("updateB before current callback = %d, want 0", updateB)
	}

	queued[1]()
	if updateB != 1 {
		t.Fatalf("updateB = %d, want 1", updateB)
	}
	if len(queued) != 2 {
		t.Fatalf("queued updates after callbacks = %d, want 2", len(queued))
	}
}

func testComponentInstance(name string, render func() Node, schedule func(*componentInstance)) *componentInstance {
	node := Component(name, struct{}{}, func(struct{}) Node {
		return render()
	}).(ComponentNode)
	return newComponentInstance(node, "", nil, schedule)
}

func assertPanic(t *testing.T, want any, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != want {
			t.Fatalf("panic = %v, want %v", recovered, want)
		}
	}()
	fn()
}

package goframe

import "testing"

func TestUseStatePersistsWithinComponent(t *testing.T) {
	var state *State[int]
	instance := testComponentInstance("Counter", func() Node {
		state = UseState(0)
		return Text(ToString(state.Get()))
	}, nil)

	renderComponentInstance(instance)
	state.Set(7)
	renderComponentInstance(instance)

	if got := state.Get(); got != 7 {
		t.Fatalf("state after component rerender = %d, want 7", got)
	}
}

func TestUseStateIsComponentScoped(t *testing.T) {
	var firstState, secondState *State[int]
	first := testComponentInstance("First", func() Node {
		firstState = UseState(1)
		return Empty()
	}, nil)
	second := testComponentInstance("Second", func() Node {
		secondState = UseState(2)
		return Empty()
	}, nil)

	renderComponentInstance(first)
	renderComponentInstance(second)
	firstState.Set(10)
	renderComponentInstance(first)
	renderComponentInstance(second)

	if firstState.Get() != 10 || secondState.Get() != 2 {
		t.Fatalf("component states = %d, %d; want 10, 2", firstState.Get(), secondState.Get())
	}
}

func TestUseStateSupportsMultipleSlots(t *testing.T) {
	var count *State[int]
	var label *State[string]
	instance := testComponentInstance("Multi", func() Node {
		count = UseState(0)
		label = UseState("first")
		return Empty()
	}, nil)

	renderComponentInstance(instance)
	count.Set(2)
	label.Set("second")
	renderComponentInstance(instance)

	if count.Get() != 2 || label.Get() != "second" {
		t.Fatalf("slots = %d, %q; want 2, second", count.Get(), label.Get())
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
	var state *State[int]
	var scheduled *componentInstance
	instance := testComponentInstance("Owner", func() Node {
		state = UseState(0)
		return Empty()
	}, func(instance *componentInstance) {
		scheduled = instance
	})
	renderComponentInstance(instance)

	state.Set(1)

	if !instance.dirty || scheduled != instance {
		t.Fatalf("dirty=%v scheduled=%p owner=%p", instance.dirty, scheduled, instance)
	}
}

func TestStateSetSameComparableValueDoesNotSchedule(t *testing.T) {
	var state *State[string]
	schedules := 0
	instance := testComponentInstance("Owner", func() Node {
		state = UseState("same")
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})
	renderComponentInstance(instance)

	state.Set("same")

	if instance.dirty || schedules != 0 {
		t.Fatalf("dirty=%v schedules=%d, want no-op", instance.dirty, schedules)
	}
}

func TestDirtyChildDoesNotMarkRootOrSibling(t *testing.T) {
	var state *State[int]
	rootSchedules := 0
	childSchedules := 0
	siblingSchedules := 0
	root := testComponentInstance("Root", func() Node { return Empty() }, func(*componentInstance) {
		rootSchedules++
	})
	child := testComponentInstance("Child", func() Node {
		state = UseState(0)
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

	state.Set(1)

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
		state := UseState(0)
		if state.Get() == 0 {
			state.Set(1)
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
	var state *State[int]
	schedules := 0
	instance := testComponentInstance("Unmounted", func() Node {
		state = UseState(0)
		return Empty()
	}, func(*componentInstance) {
		schedules++
	})
	renderComponentInstance(instance)
	deactivateComponent(instance)

	state.Set(1)

	if schedules != 0 || instance.dirty {
		t.Fatalf("schedules=%d dirty=%v, want inactive", schedules, instance.dirty)
	}
}

func TestUpdateBatchCoalescesRequests(t *testing.T) {
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
	queued[0]()
	if updates != 1 {
		t.Fatalf("updates = %d, want 1", updates)
	}

	batch.request(enqueue, func() { updates++ })
	if len(queued) != 2 {
		t.Fatalf("queued updates after flush = %d, want 2", len(queued))
	}
}

func testComponentInstance(name string, render func() Node, schedule func(*componentInstance)) *componentInstance {
	node := Component(name, struct{}{}, func(struct{}) Node {
		return render()
	}).(ComponentNode)
	return newComponentInstance(node, "", nil, schedule)
}

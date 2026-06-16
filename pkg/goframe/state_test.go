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

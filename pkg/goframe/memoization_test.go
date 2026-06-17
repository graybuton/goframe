package goframe

import "testing"

type memoizedPropsFixture struct {
	ID      int
	Version int
	Label   string
}

func (props memoizedPropsFixture) MemoEqual(next memoizedPropsFixture) bool {
	return props.ID == next.ID &&
		props.Version == next.Version &&
		props.Label == next.Label
}

func TestShouldSkipComponentRenderRequiresMemoizedProps(t *testing.T) {
	node := Component("NoMemo", memoizedPropsFixture{ID: 1}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	instance := newComponentInstance(node, "row-1", nil, nil)
	instance.dirty = false
	next := Component("NoMemo", memoizedPropsFixture{ID: 2}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)

	if shouldSkipComponentRender(instance, next, "row-1") {
		t.Fatal("component without MemoEqual should not be skipped")
	}
}

func TestShouldSkipComponentRenderSkipsWhenEqualAndClean(t *testing.T) {
	node := Component("Memo", memoizedPropsFixture{ID: 1, Version: 1, Label: "a"}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	instance := newComponentInstance(node, "row-1", nil, nil)
	instance.dirty = false
	next := Component("Memo", memoizedPropsFixture{ID: 1, Version: 1, Label: "a"}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)

	if !shouldSkipComponentRender(instance, next, "row-1") {
		t.Fatal("expected memoized component to skip render")
	}
}

func TestShouldSkipComponentRenderSkipsOnlyWhenNameAndKeyMatch(t *testing.T) {
	node := Component("Memo", memoizedPropsFixture{ID: 1, Version: 2}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	instance := newComponentInstance(node, "row-1", nil, nil)
	instance.dirty = false

	wrongKey := Component("Memo", memoizedPropsFixture{ID: 1, Version: 2}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	if shouldSkipComponentRender(instance, wrongKey, "row-2") {
		t.Fatal("component with different key must not be skipped")
	}

	wrongName := Component("Other", memoizedPropsFixture{ID: 1, Version: 2}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	if shouldSkipComponentRender(instance, wrongName, "row-1") {
		t.Fatal("component with different name must not be skipped")
	}
}

func TestShouldSkipComponentRenderSkipsWhenNotDirtyAndDirtyPropsEqual(t *testing.T) {
	node := Component("Memo", memoizedPropsFixture{ID: 1, Version: 2, Label: "a"}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	instance := newComponentInstance(node, "row-1", nil, nil)
	instance.dirty = false
	changed := Component("Memo", memoizedPropsFixture{ID: 1, Version: 3, Label: "a"}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)

	if shouldSkipComponentRender(instance, changed, "row-1") {
		t.Fatal("memo comparator should block skip on changed props")
	}
}

func TestShouldSkipComponentRenderDoesNotSkipWhenDirty(t *testing.T) {
	node := Component("Memo", memoizedPropsFixture{ID: 1, Version: 1}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	instance := newComponentInstance(node, "row-1", nil, nil)
	instance.dirty = true
	next := Component("Memo", memoizedPropsFixture{ID: 1, Version: 1}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)

	if shouldSkipComponentRender(instance, next, "row-1") {
		t.Fatal("dirty component must rerender")
	}
}

func TestShouldSkipComponentRenderDoesNotSkipWhenDescendantDirty(t *testing.T) {
	parent := dirtyCleanInstance("Parent", nil)
	node := Component("Memo", memoizedPropsFixture{ID: 1, Version: 1}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	child := newComponentInstance(node, "memo", parent, nil)
	child.dirty = false
	grandchild := dirtyCleanInstance("GrandChild", child)
	next := Component("Memo", memoizedPropsFixture{ID: 1, Version: 1}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)

	markComponentDirty(grandchild)

	if child.dirtyDescendants != 1 || parent.dirtyDescendants != 1 {
		t.Fatalf("dirty descendants child=%d parent=%d, want 1 each", child.dirtyDescendants, parent.dirtyDescendants)
	}
	if shouldSkipComponentRender(child, next, "memo") {
		t.Fatal("memoized component with dirty descendant must not skip")
	}

	renderComponentInstance(grandchild)

	if child.dirtyDescendants != 0 || parent.dirtyDescendants != 0 {
		t.Fatalf("dirty descendants after render child=%d parent=%d, want 0 each", child.dirtyDescendants, parent.dirtyDescendants)
	}
	if !shouldSkipComponentRender(child, next, "memo") {
		t.Fatal("clean memoized component with equal props should skip after descendant update")
	}
}

func TestDirtyQueuePruningDoesNotMakeMemoSkipLoseDirtyDescendant(t *testing.T) {
	parent := dirtyCleanInstance("Parent", nil)
	node := Component("Memo", memoizedPropsFixture{ID: 1, Version: 1}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	child := newComponentInstance(node, "memo", parent, nil)
	child.dirty = false
	grandchild := dirtyCleanInstance("GrandChild", child)
	next := Component("Memo", memoizedPropsFixture{ID: 1, Version: 1}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)

	markComponentDirty(grandchild)
	markComponentDirty(parent)

	pruned := pruneDirtyComponents([]*componentInstance{parent, grandchild})
	if len(pruned) != 1 || pruned[0] != parent {
		t.Fatalf("pruned dirty queue = %v, want parent only", instanceNames(pruned))
	}
	if shouldSkipComponentRender(child, next, "memo") {
		t.Fatal("memo skip would hide a grandchild pruned from dirty queue")
	}
}

func TestMemoizePropsRejectsIncompatibleValues(t *testing.T) {
	if memoizeProps[memoizedPropsFixture](memoizedPropsFixture{ID: 1}, memoizedPropsFixture{ID: 1}) != true {
		t.Fatal("expected equal memoized props to compare true")
	}
	if memoizeProps[memoizedPropsFixture](memoizedPropsFixture{ID: 1}, memoizedPropsFixture{ID: 2}) != false {
		t.Fatal("expected changed memoized props to compare false")
	}
}

func dirtyCleanInstance(name string, parent *componentInstance) *componentInstance {
	node := Component(name, struct{}{}, func(struct{}) Node {
		return Empty()
	}).(ComponentNode)
	instance := newComponentInstance(node, "", parent, nil)
	instance.dirty = false
	return instance
}

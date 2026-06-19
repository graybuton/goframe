package goframe

import "testing"

func TestNewComponentTypeRequiresID(t *testing.T) {
	assertPanic(t, "goframe: empty component id", func() {
		NewComponentType("", "Header")
	})
}

func TestNewComponentTypeUsesIDAsDebugFallback(t *testing.T) {
	componentType := NewComponentType("main.Header", "")
	node := ComponentT(componentType, 1, func(int) Node { return Empty() }).(ComponentNode)

	if node.Name != "main.Header" {
		t.Fatalf("debug name = %q, want id fallback", node.Name)
	}
	if got := nodeComponentIdentity(node); got != typedComponentIdentity("main.Header") {
		t.Fatalf("identity = %q, want typed main.Header", componentIdentityString(got))
	}
}

func TestComponentTPreservesDebugNameAndTypedIdentity(t *testing.T) {
	componentType := NewComponentType("main.Number", "Number")
	node := ComponentT(componentType, 42, func(value int) Node {
		return Text(ToString(value))
	}).(ComponentNode)

	if node.Name != "Number" || node.Props != 42 {
		t.Fatalf("component = %#v", node)
	}
	if got := nodeComponentIdentity(node); got != typedComponentIdentity("main.Number") {
		t.Fatalf("identity = %q, want typed main.Number", componentIdentityString(got))
	}
	if text := node.render().(TextNode); text.Value != "42" {
		t.Fatalf("rendered text = %q, want 42", text.Value)
	}
}

func TestComponentTRejectsZeroComponentType(t *testing.T) {
	assertPanic(t, "goframe: invalid component type", func() {
		ComponentT(ComponentType{}, 0, func(int) Node { return Empty() })
	})
}

func TestTypedComponentIdentityUsesIDAndKey(t *testing.T) {
	render := func(value int) Node { return Text(ToString(value)) }
	header := NewComponentType("main.Header", "Header")
	otherHeader := NewComponentType("other.Header", "Header")

	first := ComponentT(header, 1, render)
	second := ComponentT(header, 2, render)
	other := ComponentT(otherHeader, 1, render)

	if !sameNodeIdentity(first, second) {
		t.Fatal("same typed component id should preserve identity")
	}
	if sameNodeIdentity(first, other) {
		t.Fatal("different typed component ids must replace identity even with same debug name")
	}
	if matches := matchChildIndices([]string{"one"}, []string{"one"}); matches[0] != 0 {
		t.Fatalf("keyed component match = %v", matches)
	}
}

func TestLegacyAndTypedComponentIdentitiesDoNotCollide(t *testing.T) {
	render := func(value int) Node { return Text(ToString(value)) }
	legacy := Component("Header", 1, render)
	typed := ComponentT(NewComponentType("Header", "Header"), 1, render)

	if sameNodeIdentity(legacy, typed) {
		t.Fatal("legacy string identity must not collide with typed identity using the same id text")
	}
}

func TestTypedComponentMemoizationUsesTypedIdentity(t *testing.T) {
	componentType := NewComponentType("main.Memo", "Memo")
	node := ComponentT(componentType, memoizedPropsFixture{ID: 1, Version: 1}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	instance := newComponentInstance(node, "row-1", nil, nil)
	instance.dirty = false
	next := ComponentT(componentType, memoizedPropsFixture{ID: 1, Version: 1}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)

	if !shouldSkipComponentRender(instance, next, "row-1") {
		t.Fatal("expected clean typed memoized component with equal props to skip")
	}

	otherType := NewComponentType("other.Memo", "Memo")
	other := ComponentT(otherType, memoizedPropsFixture{ID: 1, Version: 1}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	if shouldSkipComponentRender(instance, other, "row-1") {
		t.Fatal("typed memoized component must not skip across different type ids")
	}
}

func TestTypedMemoizedComponentDoesNotSkipDirtyDescendant(t *testing.T) {
	parent := dirtyCleanInstance("Parent", nil)
	componentType := NewComponentType("main.Memo", "Memo")
	node := ComponentT(componentType, memoizedPropsFixture{ID: 1, Version: 1}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	child := newComponentInstance(node, "memo", parent, nil)
	child.dirty = false
	grandchild := dirtyCleanInstance("GrandChild", child)
	next := ComponentT(componentType, memoizedPropsFixture{ID: 1, Version: 1}, func(memoizedPropsFixture) Node {
		return Empty()
	}).(ComponentNode)

	markComponentDirty(grandchild)

	if shouldSkipComponentRender(child, next, "memo") {
		t.Fatal("typed memoized component with dirty descendant must not skip")
	}
}

func componentIdentityString(identity componentIdentity) string {
	if identity.typed {
		return "type:" + identity.id
	}
	return "legacy:" + identity.id
}

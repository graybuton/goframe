package goframe

import "testing"

type contextValueFixture struct {
	Count  int
	Accent string
}

type contextMemoPropsFixture struct {
	ID int
}

func (props contextMemoPropsFixture) MemoEqual(next contextMemoPropsFixture) bool {
	return props.ID == next.ID
}

func TestUseContextReturnsDefaultWithoutProvider(t *testing.T) {
	ctx := CreateContext("default")
	got := ""
	consumer := contextTestInstance("Consumer", nil, func() {
		got = UseContext(ctx)
	})

	renderComponentInstance(consumer)

	if got != "default" {
		t.Fatalf("context value = %q, want default", got)
	}
}

func TestProvideContextVisibleToDescendant(t *testing.T) {
	ctx := CreateContext("default")
	provider := contextProviderInstance("Provider", nil, ctx, "provided")
	renderComponentInstance(provider)

	got := ""
	consumer := contextTestInstance("Consumer", provider, func() {
		got = UseContext(ctx)
	})
	renderComponentInstance(consumer)

	if got != "provided" {
		t.Fatalf("context value = %q, want provided", got)
	}
}

func TestNearestProviderWins(t *testing.T) {
	ctx := CreateContext("default")
	outer := contextProviderInstance("Outer", nil, ctx, "outer")
	renderComponentInstance(outer)
	inner := contextProviderInstance("Inner", outer, ctx, "inner")
	renderComponentInstance(inner)

	got := ""
	consumer := contextTestInstance("Consumer", inner, func() {
		got = UseContext(ctx)
	})
	renderComponentInstance(consumer)

	if got != "inner" {
		t.Fatalf("context value = %q, want inner", got)
	}
}

func TestNestedProvidersIsolateSubscriptions(t *testing.T) {
	ctx := CreateContext(contextValueFixture{Count: 1, Accent: "outer"})
	outer := contextProviderInstance("Outer", nil, ctx, contextValueFixture{Count: 1, Accent: "outer"})
	renderComponentInstance(outer)
	inner := contextProviderInstance("Inner", outer, ctx, contextValueFixture{Count: 2, Accent: "inner"})
	renderComponentInstance(inner)

	got := 0
	consumer := contextTestInstance("Consumer", inner, func() {
		got = UseContextSelector(ctx, func(value contextValueFixture) int {
			return value.Count
		})
	})
	renderComponentInstance(consumer)
	if got != 2 {
		t.Fatalf("selected context value = %d, want 2", got)
	}

	outer.node = Component("Outer", contextValueFixture{Count: 9, Accent: "outer"}, func(value contextValueFixture) Node {
		ProvideContext(ctx, value)
		return Empty()
	}).(ComponentNode)
	renderComponentInstance(outer)

	if consumer.dirty {
		t.Fatal("consumer under inner provider should not be dirtied by outer provider update")
	}
}

func TestUseContextSelectorReturnsSelectedValue(t *testing.T) {
	ctx := CreateContext(contextValueFixture{Count: 1, Accent: "blue"})
	provider := contextProviderInstance("Provider", nil, ctx, contextValueFixture{Count: 3, Accent: "green"})
	renderComponentInstance(provider)

	got := ""
	consumer := contextTestInstance("Consumer", provider, func() {
		got = UseContextSelector(ctx, func(value contextValueFixture) string {
			return value.Accent
		})
	})
	renderComponentInstance(consumer)

	if got != "green" {
		t.Fatalf("selected context value = %q, want green", got)
	}
}

func TestContextSelectorDirtyWhenSelectedValueChanges(t *testing.T) {
	ctx := CreateContext(contextValueFixture{})
	provider := contextProviderInstance("Provider", nil, ctx, contextValueFixture{Count: 1, Accent: "blue"})
	renderComponentInstance(provider)
	consumer := contextSelectorConsumer(provider, ctx, func(value contextValueFixture) int {
		return value.Count
	})
	renderComponentInstance(consumer)

	updateContextProvider(provider, ctx, contextValueFixture{Count: 2, Accent: "blue"})

	if !consumer.dirty {
		t.Fatal("selector consumer should be dirty when selected value changes")
	}
}

func TestContextSelectorCleanWhenSelectedValueUnchanged(t *testing.T) {
	ctx := CreateContext(contextValueFixture{})
	provider := contextProviderInstance("Provider", nil, ctx, contextValueFixture{Count: 1, Accent: "blue"})
	renderComponentInstance(provider)
	consumer := contextSelectorConsumer(provider, ctx, func(value contextValueFixture) int {
		return value.Count
	})
	renderComponentInstance(consumer)

	updateContextProvider(provider, ctx, contextValueFixture{Count: 1, Accent: "green"})

	if consumer.dirty {
		t.Fatal("selector consumer should stay clean when selected value is unchanged")
	}
}

func TestContextOnlyChangedSelectedConsumersAreDirty(t *testing.T) {
	ctx := CreateContext(contextValueFixture{})
	provider := contextProviderInstance("Provider", nil, ctx, contextValueFixture{Count: 1, Accent: "blue"})
	renderComponentInstance(provider)
	countConsumer := contextSelectorConsumer(provider, ctx, func(value contextValueFixture) int {
		return value.Count
	})
	accentConsumer := contextSelectorConsumer(provider, ctx, func(value contextValueFixture) string {
		return value.Accent
	})
	sibling := dirtyCleanInstance("Sibling", provider)
	renderComponentInstance(countConsumer)
	renderComponentInstance(accentConsumer)

	updateContextProvider(provider, ctx, contextValueFixture{Count: 2, Accent: "blue"})

	if !countConsumer.dirty {
		t.Fatal("count consumer should be dirty")
	}
	if accentConsumer.dirty {
		t.Fatal("accent consumer should stay clean")
	}
	if sibling.dirty {
		t.Fatal("unrelated sibling should stay clean")
	}
}

func TestUseContextBroadConsumerRerendersOnProviderUpdate(t *testing.T) {
	ctx := CreateContext(contextValueFixture{})
	provider := contextProviderInstance("Provider", nil, ctx, contextValueFixture{Count: 1})
	renderComponentInstance(provider)
	consumer := contextTestInstance("Consumer", provider, func() {
		_ = UseContext(ctx)
	})
	renderComponentInstance(consumer)

	updateContextProvider(provider, ctx, contextValueFixture{Count: 2})

	if !consumer.dirty {
		t.Fatal("UseContext consumer should be dirtied by provider update")
	}
}

func TestContextConsumerUnmountUnsubscribes(t *testing.T) {
	ctx := CreateContext(contextValueFixture{})
	provider := contextProviderInstance("Provider", nil, ctx, contextValueFixture{Count: 1})
	renderComponentInstance(provider)
	consumer := contextSelectorConsumer(provider, ctx, func(value contextValueFixture) int {
		return value.Count
	})
	renderComponentInstance(consumer)
	providerSlot := provider.contextProviders[ctx.id]
	if len(providerSlot.subscribers) != 1 {
		t.Fatalf("subscribers = %d, want 1", len(providerSlot.subscribers))
	}

	deactivateComponent(consumer)

	if len(providerSlot.subscribers) != 0 {
		t.Fatalf("subscribers after unmount = %d, want 0", len(providerSlot.subscribers))
	}
}

func TestContextProviderUnmountClearsSubscribers(t *testing.T) {
	ctx := CreateContext(contextValueFixture{})
	provider := contextProviderInstance("Provider", nil, ctx, contextValueFixture{Count: 1})
	renderComponentInstance(provider)
	consumer := contextSelectorConsumer(provider, ctx, func(value contextValueFixture) int {
		return value.Count
	})
	renderComponentInstance(consumer)
	subscription := consumer.contextSlots[0]
	providerSlot := provider.contextProviders[ctx.id]

	deactivateComponent(provider)

	if len(providerSlot.subscribers) != 0 {
		t.Fatalf("provider subscribers after unmount = %d, want 0", len(providerSlot.subscribers))
	}
	if subscription.provider != nil {
		t.Fatal("subscription should no longer point at unmounted provider")
	}
}

func TestContextProviderRemovalDirtiesConsumersForDefault(t *testing.T) {
	ctx := CreateContext(contextValueFixture{Count: 0})
	providing := true
	provider := contextTestInstance("Provider", nil, func() {
		if providing {
			ProvideContext(ctx, contextValueFixture{Count: 1})
		}
	})
	renderComponentInstance(provider)
	consumer := contextSelectorConsumer(provider, ctx, func(value contextValueFixture) int {
		return value.Count
	})
	renderComponentInstance(consumer)

	providing = false
	renderComponentInstance(provider)

	if !consumer.dirty {
		t.Fatal("consumer should be dirtied when provider is removed and default selection differs")
	}
}

func TestContextDirtyConsumerPreventsMemoAncestorSkip(t *testing.T) {
	ctx := CreateContext(contextValueFixture{})
	provider := contextProviderInstance("Provider", nil, ctx, contextValueFixture{Count: 1})
	renderComponentInstance(provider)
	memoNode := Component("Memo", contextMemoPropsFixture{ID: 1}, func(contextMemoPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	memo := newComponentInstance(memoNode, "memo", provider, nil)
	memo.dirty = false
	consumer := contextSelectorConsumer(memo, ctx, func(value contextValueFixture) int {
		return value.Count
	})
	renderComponentInstance(consumer)

	updateContextProvider(provider, ctx, contextValueFixture{Count: 2})

	if memo.dirtyDescendants != 1 {
		t.Fatalf("memo dirty descendants = %d, want 1", memo.dirtyDescendants)
	}
	next := Component("Memo", contextMemoPropsFixture{ID: 1}, func(contextMemoPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	if shouldSkipComponentRender(memo, next, "memo") {
		t.Fatal("memoized ancestor must not skip dirty context consumer")
	}
}

func TestContextHooksOutsideComponentPanic(t *testing.T) {
	ctx := CreateContext(0)
	assertPanic(t, "goframe: ProvideContext must be called during component render", func() {
		ProvideContext(ctx, 1)
	})
	assertPanic(t, "goframe: UseContext must be called during component render", func() {
		UseContext(ctx)
	})
	assertPanic(t, "goframe: UseContextSelector must be called during component render", func() {
		UseContextSelector(ctx, func(value int) int {
			return value
		})
	})
}

func TestContextHookKindMismatchPanics(t *testing.T) {
	ctx := CreateContext(contextValueFixture{})
	useSelector := false
	instance := contextTestInstance("Consumer", nil, func() {
		if useSelector {
			_ = UseContextSelector(ctx, func(value contextValueFixture) int {
				return value.Count
			})
			return
		}
		_ = UseContext(ctx)
	})
	renderComponentInstance(instance)

	useSelector = true
	assertPanic(t, "goframe: context hook at slot 0 changed from UseContext to UseContextSelector", func() {
		renderComponentInstance(instance)
	})
}

func contextProviderInstance[T any](name string, parent *componentInstance, ctx *Context[T], value T) *componentInstance {
	return contextTestInstance(name, parent, func() {
		ProvideContext(ctx, value)
	})
}

func contextSelectorConsumer[T any, S comparable](
	parent *componentInstance,
	ctx *Context[T],
	selector func(T) S,
) *componentInstance {
	return contextTestInstance("Consumer", parent, func() {
		_ = UseContextSelector(ctx, selector)
	})
}

func contextTestInstance(name string, parent *componentInstance, render func()) *componentInstance {
	node := Component(name, struct{}{}, func(struct{}) Node {
		render()
		return Empty()
	}).(ComponentNode)
	instance := newComponentInstance(node, "", parent, nil)
	instance.dirty = false
	return instance
}

func updateContextProvider[T any](instance *componentInstance, ctx *Context[T], value T) {
	instance.node = Component(instance.name, value, func(value T) Node {
		ProvideContext(ctx, value)
		return Empty()
	}).(ComponentNode)
	renderComponentInstance(instance)
}

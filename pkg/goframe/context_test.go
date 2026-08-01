package goframe

import "testing"

type contextValueFixture struct {
	Count  int
	Accent string
}

type contextTopologyValueFixture struct {
	Count       int
	PanicSelect bool
}

type contextSelectorPanicFixture struct {
	name string
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

func TestContextProviderAppearanceDirtiesDefaultConsumerThroughMemoAncestor(t *testing.T) {
	ctx := CreateContext(contextValueFixture{Count: 0})
	providing := false
	provider := contextTestInstance("Provider", nil, func() {
		if providing {
			ProvideContext(ctx, contextValueFixture{Count: 7})
		}
	})
	renderComponentInstance(provider)

	memoNode := Component("Memo", contextMemoPropsFixture{ID: 1}, func(contextMemoPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	memo := newComponentInstance(memoNode, "memo", provider, nil)
	memo.dirty = false

	got := -1
	consumer := contextTestInstance("Consumer", memo, func() {
		got = UseContextSelector(ctx, func(value contextValueFixture) int {
			return value.Count
		})
	})
	renderComponentInstance(consumer)
	if got != 0 {
		t.Fatalf("initial selected context value = %d, want default 0", got)
	}

	providing = true
	renderComponentInstance(provider)

	if !consumer.dirty {
		t.Fatal("consumer should be dirtied when provider appears above it")
	}
	if memo.dirtyDescendants != 1 {
		t.Fatalf("memo dirty descendants = %d, want 1", memo.dirtyDescendants)
	}
	next := Component("Memo", contextMemoPropsFixture{ID: 1}, func(contextMemoPropsFixture) Node {
		return Empty()
	}).(ComponentNode)
	if shouldSkipComponentRender(memo, next, "memo") {
		t.Fatal("memoized ancestor must not skip context provider appearance")
	}

	renderComponentInstance(consumer)
	if got != 7 {
		t.Fatalf("selected context value after provider appears = %d, want 7", got)
	}
}

func TestContextInnerProviderAppearanceRebindsConsumerFromOuterProvider(t *testing.T) {
	ctx := CreateContext(contextValueFixture{Count: 0})
	outer := contextProviderInstance("Outer", nil, ctx, contextValueFixture{Count: 1})
	renderComponentInstance(outer)

	innerProviding := false
	inner := contextTestInstance("Inner", outer, func() {
		if innerProviding {
			ProvideContext(ctx, contextValueFixture{Count: 2})
		}
	})
	renderComponentInstance(inner)

	got := -1
	consumer := contextTestInstance("Consumer", inner, func() {
		got = UseContextSelector(ctx, func(value contextValueFixture) int {
			return value.Count
		})
	})
	renderComponentInstance(consumer)
	if got != 1 {
		t.Fatalf("initial selected context value = %d, want outer 1", got)
	}

	innerProviding = true
	renderComponentInstance(inner)

	if !consumer.dirty {
		t.Fatal("consumer should be dirtied when inner provider appears")
	}
	renderComponentInstance(consumer)
	if got != 2 {
		t.Fatalf("selected context value after inner provider appears = %d, want inner 2", got)
	}
}

func TestContextProviderTopologyChangeDirtiesConsumerWhenSelectionIsEqual(t *testing.T) {
	ctx := CreateContext(contextValueFixture{Count: 1})
	providing := false
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

	providing = true
	renderComponentInstance(provider)

	if !consumer.dirty {
		t.Fatal("consumer should be dirtied when nearest provider changes even if selected value is equal")
	}
}

func TestContextInnerProviderRemovalRebindsConsumerToOuterProvider(t *testing.T) {
	ctx := CreateContext(contextValueFixture{Count: 0})
	outer := contextProviderInstance("Outer", nil, ctx, contextValueFixture{Count: 1})
	renderComponentInstance(outer)

	innerProviding := true
	inner := contextTestInstance("Inner", outer, func() {
		if innerProviding {
			ProvideContext(ctx, contextValueFixture{Count: 2})
		}
	})
	renderComponentInstance(inner)

	got := -1
	consumer := contextTestInstance("Consumer", inner, func() {
		got = UseContextSelector(ctx, func(value contextValueFixture) int {
			return value.Count
		})
	})
	renderComponentInstance(consumer)
	if got != 2 {
		t.Fatalf("initial selected context value = %d, want inner 2", got)
	}

	innerProviding = false
	renderComponentInstance(inner)

	if !consumer.dirty {
		t.Fatal("consumer should be dirtied when inner provider is removed")
	}
	renderComponentInstance(consumer)
	if got != 1 {
		t.Fatalf("selected context value after inner provider removal = %d, want outer 1", got)
	}
}

func TestContextSelectorTopologyPanicRebindsDefaultToProvider(t *testing.T) {
	isolateContextSubscriptions(t)
	errors := captureRuntimeErrors(t)
	panicValue := &contextSelectorPanicFixture{name: "default-to-provider"}
	ctx := CreateContext(contextTopologyValueFixture{Count: 1})
	providing := false
	provided := contextTopologyValueFixture{}
	provider := contextTestInstance("Provider", nil, func() {
		if providing {
			ProvideContext(ctx, provided)
		}
	})
	renderComponentInstance(provider)

	selectorCalls := 0
	selected := -1
	consumer := contextTestInstance("Consumer", provider, func() {
		selected = UseContextSelector(ctx, contextTopologySelector(
			&selectorCalls,
			panicValue,
		))
	})
	renderComponentInstance(consumer)
	slot := consumer.contextSlots[0]
	if slot.provider != nil {
		t.Fatalf("initial provider = %#v, want default", slot.provider)
	}

	providing = true
	provided = contextTopologyValueFixture{Count: 2, PanicSelect: true}
	renderComponentInstance(provider)
	newProvider := provider.contextProviders[ctx.id]

	assertFailedContextTopologyRefresh(t, contextTopologyFailureExpectation{
		errors:           errors(),
		panicValue:       panicValue,
		slot:             slot,
		provider:         newProvider,
		previous:         nil,
		selected:         1,
		consumer:         consumer,
		ancestors:        []*componentInstance{provider},
		selectorCalls:    selectorCalls,
		wantSelectorCall: 2,
	})

	provided = contextTopologyValueFixture{Count: 2}
	renderComponentInstance(provider)
	assertRecoveredContextSelection(t, slot, consumer, 2, selectorCalls, 3)
	if provider.dirtyDescendants != 1 {
		t.Fatalf("provider dirty descendants after recovery = %d, want 1", provider.dirtyDescendants)
	}
	renderComponentInstance(consumer)
	if selected != 2 {
		t.Fatalf("rendered selection after recovery = %d, want 2", selected)
	}

	deactivateComponent(consumer)
	deactivateComponent(provider)
}

func TestContextSelectorTopologyPanicRebindsOuterToInnerProvider(t *testing.T) {
	isolateContextSubscriptions(t)
	errors := captureRuntimeErrors(t)
	panicValue := &contextSelectorPanicFixture{name: "outer-to-inner"}
	ctx := CreateContext(contextTopologyValueFixture{})
	outer := contextProviderInstance("Outer", nil, ctx, contextTopologyValueFixture{Count: 1})
	renderComponentInstance(outer)
	outerProvider := outer.contextProviders[ctx.id]

	innerProviding := false
	innerValue := contextTopologyValueFixture{}
	inner := contextTestInstance("Inner", outer, func() {
		if innerProviding {
			ProvideContext(ctx, innerValue)
		}
	})
	renderComponentInstance(inner)

	selectorCalls := 0
	selected := -1
	consumer := contextTestInstance("Consumer", inner, func() {
		selected = UseContextSelector(ctx, contextTopologySelector(
			&selectorCalls,
			panicValue,
		))
	})
	renderComponentInstance(consumer)
	slot := consumer.contextSlots[0]

	innerProviding = true
	innerValue = contextTopologyValueFixture{Count: 2, PanicSelect: true}
	renderComponentInstance(inner)
	innerProvider := inner.contextProviders[ctx.id]

	assertFailedContextTopologyRefresh(t, contextTopologyFailureExpectation{
		errors:           errors(),
		panicValue:       panicValue,
		slot:             slot,
		provider:         innerProvider,
		previous:         outerProvider,
		selected:         1,
		consumer:         consumer,
		ancestors:        []*componentInstance{inner, outer},
		selectorCalls:    selectorCalls,
		wantSelectorCall: 2,
	})

	updateContextProvider(outer, ctx, contextTopologyValueFixture{Count: 9})
	if selectorCalls != 2 {
		t.Fatalf("selector calls after shadowed outer update = %d, want 2", selectorCalls)
	}
	if slot.selected != 1 || consumer.dirty {
		t.Fatalf("state after shadowed outer update = selected %#v dirty %t, want 1 false", slot.selected, consumer.dirty)
	}

	innerValue = contextTopologyValueFixture{Count: 2}
	renderComponentInstance(inner)
	assertRecoveredContextSelection(t, slot, consumer, 2, selectorCalls, 3)
	if inner.dirtyDescendants != 1 || outer.dirtyDescendants != 1 {
		t.Fatalf("dirty descendants after inner recovery = inner %d outer %d, want 1 each",
			inner.dirtyDescendants, outer.dirtyDescendants)
	}
	renderComponentInstance(consumer)
	if selected != 2 {
		t.Fatalf("rendered selection after inner recovery = %d, want 2", selected)
	}

	deactivateComponent(consumer)
	deactivateComponent(inner)
	deactivateComponent(outer)
}

func TestContextSelectorTopologyPanicRebindsInnerToOuterProvider(t *testing.T) {
	isolateContextSubscriptions(t)
	errors := captureRuntimeErrors(t)
	panicValue := &contextSelectorPanicFixture{name: "inner-to-outer"}
	ctx := CreateContext(contextTopologyValueFixture{})
	outer := contextProviderInstance("Outer", nil, ctx, contextTopologyValueFixture{
		Count:       3,
		PanicSelect: true,
	})
	renderComponentInstance(outer)
	outerProvider := outer.contextProviders[ctx.id]

	innerProviding := true
	innerValue := contextTopologyValueFixture{Count: 2}
	inner := contextTestInstance("Inner", outer, func() {
		if innerProviding {
			ProvideContext(ctx, innerValue)
		}
	})
	renderComponentInstance(inner)
	innerProvider := inner.contextProviders[ctx.id]

	selectorCalls := 0
	selected := -1
	consumer := contextTestInstance("Consumer", inner, func() {
		selected = UseContextSelector(ctx, contextTopologySelector(
			&selectorCalls,
			panicValue,
		))
	})
	renderComponentInstance(consumer)
	slot := consumer.contextSlots[0]

	innerProviding = false
	renderComponentInstance(inner)

	assertFailedContextTopologyRefresh(t, contextTopologyFailureExpectation{
		errors:           errors(),
		panicValue:       panicValue,
		slot:             slot,
		provider:         outerProvider,
		previous:         innerProvider,
		selected:         2,
		consumer:         consumer,
		ancestors:        []*componentInstance{inner, outer},
		selectorCalls:    selectorCalls,
		wantSelectorCall: 2,
	})

	notifyContextSubscribers(innerProvider, contextTopologyValueFixture{Count: 8})
	if selectorCalls != 2 {
		t.Fatalf("selector calls after removed inner update = %d, want 2", selectorCalls)
	}

	updateContextProvider(outer, ctx, contextTopologyValueFixture{Count: 3})
	assertRecoveredContextSelection(t, slot, consumer, 3, selectorCalls, 3)
	renderComponentInstance(consumer)
	if selected != 3 {
		t.Fatalf("rendered selection after outer recovery = %d, want 3", selected)
	}

	deactivateComponent(consumer)
	deactivateComponent(inner)
	deactivateComponent(outer)
}

func TestContextSelectorTopologyPanicRebindsProviderToDefault(t *testing.T) {
	isolateContextSubscriptions(t)
	errors := captureRuntimeErrors(t)
	panicValue := &contextSelectorPanicFixture{name: "provider-to-default"}
	ctx := CreateContext(contextTopologyValueFixture{
		Count:       0,
		PanicSelect: true,
	})
	providing := true
	providerValue := contextTopologyValueFixture{Count: 2}
	provider := contextTestInstance("Provider", nil, func() {
		if providing {
			ProvideContext(ctx, providerValue)
		}
	})
	renderComponentInstance(provider)
	removedProvider := provider.contextProviders[ctx.id]

	selectorCalls := 0
	selected := -1
	consumer := contextTestInstance("Consumer", provider, func() {
		selected = UseContextSelector(ctx, contextTopologySelector(
			&selectorCalls,
			panicValue,
		))
	})
	renderComponentInstance(consumer)
	slot := consumer.contextSlots[0]

	providing = false
	renderComponentInstance(provider)

	assertFailedContextTopologyRefresh(t, contextTopologyFailureExpectation{
		errors:           errors(),
		panicValue:       panicValue,
		slot:             slot,
		provider:         nil,
		previous:         removedProvider,
		selected:         2,
		consumer:         consumer,
		ancestors:        []*componentInstance{provider},
		selectorCalls:    selectorCalls,
		wantSelectorCall: 2,
	})

	notifyContextSubscribers(removedProvider, contextTopologyValueFixture{Count: 8})
	if selectorCalls != 2 {
		t.Fatalf("selector calls after detached provider update = %d, want 2", selectorCalls)
	}

	providerValue = contextTopologyValueFixture{Count: 4}
	providing = true
	renderComponentInstance(provider)
	newProvider := provider.contextProviders[ctx.id]
	if slot.provider != newProvider {
		t.Fatalf("provider after safe appearance = %#v, want %#v", slot.provider, newProvider)
	}
	assertRecoveredContextSelection(t, slot, consumer, 4, selectorCalls, 3)
	renderComponentInstance(consumer)
	if selected != 4 {
		t.Fatalf("rendered selection after new provider recovery = %d, want 4", selected)
	}

	deactivateComponent(consumer)
	deactivateComponent(provider)
}

func TestContextSelectorSameProviderPanicKeepsBindingAndRecovers(t *testing.T) {
	isolateContextSubscriptions(t)
	errors := captureRuntimeErrors(t)
	panicValue := &contextSelectorPanicFixture{name: "same-provider"}
	ctx := CreateContext(contextTopologyValueFixture{})
	provider := contextProviderInstance("Provider", nil, ctx, contextTopologyValueFixture{Count: 1})
	renderComponentInstance(provider)
	providerSlot := provider.contextProviders[ctx.id]

	selectorCalls := 0
	consumer := contextSelectorConsumer(provider, ctx, contextTopologySelector(
		&selectorCalls,
		panicValue,
	))
	renderComponentInstance(consumer)
	slot := consumer.contextSlots[0]

	updateContextProvider(provider, ctx, contextTopologyValueFixture{
		Count:       2,
		PanicSelect: true,
	})

	if slot.provider != providerSlot {
		t.Fatalf("provider after same-provider panic = %#v, want %#v", slot.provider, providerSlot)
	}
	if !providerSlot.subscribers[slot] {
		t.Fatal("same provider lost subscription after selector panic")
	}
	if slot.selected != 1 || consumer.dirty || provider.dirtyDescendants != 0 {
		t.Fatalf("state after same-provider panic = selected %#v dirty %t descendants %d, want 1 false 0",
			slot.selected, consumer.dirty, provider.dirtyDescendants)
	}
	if selectorCalls != 2 {
		t.Fatalf("selector calls after same-provider panic = %d, want 2", selectorCalls)
	}
	assertSingleContextSelectorError(t, errors(), panicValue)

	updateContextProvider(provider, ctx, contextTopologyValueFixture{Count: 2})
	assertRecoveredContextSelection(t, slot, consumer, 2, selectorCalls, 3)

	deactivateComponent(consumer)
	deactivateComponent(provider)
}

func TestContextSelectorTopologyPanicRebindUnmountReleasesSubscription(t *testing.T) {
	isolateContextSubscriptions(t)
	errors := captureRuntimeErrors(t)
	panicValue := &contextSelectorPanicFixture{name: "unmount-after-rebind"}
	ctx := CreateContext(contextTopologyValueFixture{})
	outer := contextProviderInstance("Outer", nil, ctx, contextTopologyValueFixture{Count: 1})
	renderComponentInstance(outer)

	innerProviding := false
	inner := contextTestInstance("Inner", outer, func() {
		if innerProviding {
			ProvideContext(ctx, contextTopologyValueFixture{Count: 2, PanicSelect: true})
		}
	})
	renderComponentInstance(inner)

	selectorCalls := 0
	consumer := contextSelectorConsumer(inner, ctx, contextTopologySelector(
		&selectorCalls,
		panicValue,
	))
	renderComponentInstance(consumer)
	slot := consumer.contextSlots[0]

	innerProviding = true
	renderComponentInstance(inner)
	innerProvider := inner.contextProviders[ctx.id]
	if slot.provider != innerProvider {
		t.Fatalf("provider after failed topology refresh = %#v, want inner %#v", slot.provider, innerProvider)
	}
	assertSingleContextSelectorError(t, errors(), panicValue)

	deactivateComponent(consumer)
	if slot.provider != nil {
		t.Fatalf("provider after unmount = %#v, want nil", slot.provider)
	}
	if innerProvider.subscribers[slot] {
		t.Fatal("inner provider retained inactive subscription")
	}
	if slots := contextSubscriptionsByID[ctx.id]; slots[slot] || len(slots) != 0 {
		t.Fatalf("global subscriptions after unmount = %#v, want none", slots)
	}
	selectorCallsBeforeUpdate := selectorCalls
	notifyContextSubscribers(innerProvider, contextTopologyValueFixture{Count: 3})
	if selectorCalls != selectorCallsBeforeUpdate || consumer.dirty {
		t.Fatalf("inactive consumer after provider update = calls %d dirty %t, want %d false",
			selectorCalls, consumer.dirty, selectorCallsBeforeUpdate)
	}

	deactivateComponent(inner)
	deactivateComponent(outer)
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

type contextTopologyFailureExpectation struct {
	errors           []ErrorInfo
	panicValue       any
	slot             *contextSubscription
	provider         *contextProvider
	previous         *contextProvider
	selected         int
	consumer         *componentInstance
	ancestors        []*componentInstance
	selectorCalls    int
	wantSelectorCall int
}

func isolateContextSubscriptions(t *testing.T) {
	t.Helper()
	previous := contextSubscriptionsByID
	contextSubscriptionsByID = nil
	t.Cleanup(func() {
		contextSubscriptionsByID = previous
	})
}

func contextTopologySelector(
	calls *int,
	panicValue any,
) func(contextTopologyValueFixture) int {
	return func(value contextTopologyValueFixture) int {
		(*calls)++
		if value.PanicSelect {
			panic(panicValue)
		}
		return value.Count
	}
}

func assertFailedContextTopologyRefresh(t *testing.T, want contextTopologyFailureExpectation) {
	t.Helper()
	if want.slot.selected != want.selected {
		t.Fatalf("selected after failed topology refresh = %#v, want previous %d", want.slot.selected, want.selected)
	}
	if want.consumer.dirty {
		t.Fatal("consumer should remain clean after failed topology selector evaluation")
	}
	for _, ancestor := range want.ancestors {
		if ancestor.dirtyDescendants != 0 {
			t.Fatalf("%s dirty descendants after failed topology refresh = %d, want 0",
				ancestor.name, ancestor.dirtyDescendants)
		}
	}
	if want.selectorCalls != want.wantSelectorCall {
		t.Fatalf("selector calls after failed topology refresh = %d, want %d",
			want.selectorCalls, want.wantSelectorCall)
	}
	assertSingleContextSelectorError(t, want.errors, want.panicValue)
	if want.slot.provider != want.provider {
		t.Fatalf("provider after failed topology refresh = %#v, want %#v", want.slot.provider, want.provider)
	}
	if want.previous != nil && want.previous.subscribers[want.slot] {
		t.Fatal("previous provider retained subscription after failed topology refresh")
	}
	if want.provider != nil && !want.provider.subscribers[want.slot] {
		t.Fatal("new provider is missing subscription after failed topology refresh")
	}
}

func assertSingleContextSelectorError(t *testing.T, errors []ErrorInfo, panicValue any) {
	t.Helper()
	if len(errors) != 1 {
		t.Fatalf("context errors = %#v, want exactly one", errors)
	}
	info := errors[0]
	if info.Phase != ErrorPhaseContext ||
		info.Component != "Consumer" ||
		info.Operation != "UseContextSelector" ||
		info.Panic != panicValue {
		t.Fatalf("context error = %#v, want phase context component Consumer operation UseContextSelector panic %#v",
			info, panicValue)
	}
}

func assertRecoveredContextSelection(
	t *testing.T,
	slot *contextSubscription,
	consumer *componentInstance,
	wantSelected int,
	selectorCalls int,
	wantSelectorCalls int,
) {
	t.Helper()
	if slot.selected != wantSelected {
		t.Fatalf("selected after safe provider update = %#v, want %d", slot.selected, wantSelected)
	}
	if !consumer.dirty {
		t.Fatal("consumer should be dirty after safe provider update")
	}
	if selectorCalls != wantSelectorCalls {
		t.Fatalf("selector calls after safe provider update = %d, want %d", selectorCalls, wantSelectorCalls)
	}
}

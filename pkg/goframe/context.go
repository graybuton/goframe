package goframe

var nextContextID int

// contextSubscriptionsByID lets provider appearance/removal recompute nearest
// provider topology for consumers that were previously subscribed to default or
// to an outer provider. Ordinary provider value updates use provider.subscribers.
var contextSubscriptionsByID map[int]map[*contextSubscription]bool

// Context carries a typed value through a scoped component subtree.
type Context[T any] struct {
	id           int
	defaultValue T
}

// CreateContext creates a typed context with a default value used when no
// provider exists above the current component.
func CreateContext[T any](defaultValue T) *Context[T] {
	nextContextID++
	return &Context[T]{
		id:           nextContextID,
		defaultValue: defaultValue,
	}
}

type contextProvider struct {
	id          int
	owner       *componentInstance
	value       any
	subscribers map[*contextSubscription]bool
}

type contextSubscription struct {
	owner        *componentInstance
	provider     *contextProvider
	contextID    int
	kind         string
	selected     any
	defaultValue any
	update       func(*contextSubscription, any) bool
}

// ProvideContext provides value for descendant components of the current
// component. It must be called during component render.
func ProvideContext[T any](ctx *Context[T], value T) {
	if ctx == nil {
		panic("goframe: ProvideContext requires a context")
	}
	instance := requireCurrentComponent("ProvideContext")
	provider, created := ensureContextProvider(instance, ctx.id, value)
	provider.value = value
	recordProvidedContext(instance, ctx.id)
	if created {
		refreshContextTopology(ctx.id)
		return
	}
	notifyContextSubscribers(provider, value)
}

// UseContext returns the nearest provider value, or the context default when
// no provider exists. It subscribes broadly; performance-sensitive consumers
// should prefer UseContextSelector.
func UseContext[T any](ctx *Context[T]) T {
	if ctx == nil {
		panic("goframe: UseContext requires a context")
	}
	instance := requireCurrentComponent("UseContext")
	provider := findContextProvider(instance.parent, ctx.id)
	value := ctx.defaultValue
	if provider != nil {
		value = provider.value.(T)
	}
	subscribeContext(instance, ctx.id, "UseContext", provider, value, ctx.defaultValue, func(slot *contextSubscription, raw any) bool {
		slot.selected = raw
		return true
	})
	return value
}

// UseContextSelector returns selector(nearest value). S must be comparable so
// the runtime can dirty this consumer only when the selected value changes.
func UseContextSelector[T any, S comparable](ctx *Context[T], selector func(T) S) S {
	if ctx == nil {
		panic("goframe: UseContextSelector requires a context")
	}
	if selector == nil {
		panic("goframe: UseContextSelector requires a selector")
	}
	instance := requireCurrentComponent("UseContextSelector")
	provider := findContextProvider(instance.parent, ctx.id)
	value := ctx.defaultValue
	if provider != nil {
		value = provider.value.(T)
	}
	selected := selector(value)
	subscribeContext(instance, ctx.id, "UseContextSelector", provider, selected, ctx.defaultValue, func(slot *contextSubscription, raw any) bool {
		next := selector(raw.(T))
		previous, ok := slot.selected.(S)
		if ok && previous == next {
			return false
		}
		slot.selected = next
		return true
	})
	return selected
}

func ensureContextProvider[T any](instance *componentInstance, contextID int, value T) (*contextProvider, bool) {
	if instance.contextProviders == nil {
		instance.contextProviders = make(map[int]*contextProvider)
	}
	provider := instance.contextProviders[contextID]
	if provider == nil {
		provider = &contextProvider{
			id:          contextID,
			owner:       instance,
			value:       value,
			subscribers: make(map[*contextSubscription]bool),
		}
		instance.contextProviders[contextID] = provider
		return provider, true
	}
	return provider, false
}

func recordProvidedContext(instance *componentInstance, contextID int) {
	for _, existing := range instance.providedContexts {
		if existing == contextID {
			return
		}
	}
	instance.providedContexts = append(instance.providedContexts, contextID)
}

func finishComponentContextRender(instance *componentInstance) {
	releaseUnusedContextSubscriptions(instance)
	releaseUnprovidedContexts(instance)
}

func releaseUnusedContextSubscriptions(instance *componentInstance) {
	for index := instance.contextIndex; index < len(instance.contextSlots); index++ {
		releaseContextSubscription(instance.contextSlots[index])
		instance.contextSlots[index] = nil
	}
	instance.contextSlots = instance.contextSlots[:instance.contextIndex]
}

func releaseUnprovidedContexts(instance *componentInstance) {
	if len(instance.contextProviders) == 0 {
		return
	}
	for contextID, provider := range instance.contextProviders {
		if contextWasProvided(instance, contextID) {
			continue
		}
		removeContextProvider(provider)
		delete(instance.contextProviders, contextID)
	}
}

func contextWasProvided(instance *componentInstance, contextID int) bool {
	for _, providedID := range instance.providedContexts {
		if providedID == contextID {
			return true
		}
	}
	return false
}

func findContextProvider(instance *componentInstance, contextID int) *contextProvider {
	for current := instance; current != nil; current = current.parent {
		if current.contextProviders == nil {
			continue
		}
		if provider := current.contextProviders[contextID]; provider != nil {
			return provider
		}
	}
	return nil
}

func subscribeContext(
	instance *componentInstance,
	contextID int,
	kind string,
	provider *contextProvider,
	selected any,
	defaultValue any,
	update func(*contextSubscription, any) bool,
) {
	index := instance.contextIndex
	instance.contextIndex++
	if index == len(instance.contextSlots) {
		slot := &contextSubscription{
			owner:        instance,
			contextID:    contextID,
			kind:         kind,
			selected:     selected,
			defaultValue: defaultValue,
			update:       update,
		}
		instance.contextSlots = append(instance.contextSlots, slot)
		registerContextSubscription(slot)
		setContextSubscriptionProvider(slot, provider)
		return
	}

	slot := instance.contextSlots[index]
	if slot.kind != kind {
		panic("goframe: context hook at slot " + ToString(index) + " changed from " + slot.kind + " to " + kind)
	}
	if slot.contextID != contextID {
		panic("goframe: context hook at slot " + ToString(index) + " changed context")
	}
	slot.selected = selected
	slot.defaultValue = defaultValue
	slot.update = update
	setContextSubscriptionProvider(slot, provider)
}

func registerContextSubscription(slot *contextSubscription) {
	if slot == nil {
		return
	}
	if contextSubscriptionsByID == nil {
		contextSubscriptionsByID = make(map[int]map[*contextSubscription]bool)
	}
	slots := contextSubscriptionsByID[slot.contextID]
	if slots == nil {
		slots = make(map[*contextSubscription]bool)
		contextSubscriptionsByID[slot.contextID] = slots
	}
	slots[slot] = true
}

func releaseContextSubscription(slot *contextSubscription) {
	if slot == nil {
		return
	}
	unsubscribeContext(slot)
	if contextSubscriptionsByID == nil {
		return
	}
	slots := contextSubscriptionsByID[slot.contextID]
	delete(slots, slot)
	if len(slots) == 0 {
		delete(contextSubscriptionsByID, slot.contextID)
	}
}

func setContextSubscriptionProvider(slot *contextSubscription, provider *contextProvider) {
	if slot.provider == provider {
		return
	}
	unsubscribeContext(slot)
	slot.provider = provider
	if provider == nil {
		return
	}
	if provider.subscribers == nil {
		provider.subscribers = make(map[*contextSubscription]bool)
	}
	provider.subscribers[slot] = true
}

func unsubscribeContext(slot *contextSubscription) {
	if slot == nil || slot.provider == nil {
		return
	}
	delete(slot.provider.subscribers, slot)
	slot.provider = nil
}

func notifyContextSubscribers(provider *contextProvider, value any) {
	for slot := range provider.subscribers {
		if slot == nil || slot.owner == nil || !slot.owner.active {
			releaseContextSubscription(slot)
			continue
		}
		if slot.update(slot, value) {
			markComponentDirty(slot.owner)
		}
	}
}

func removeContextProvider(provider *contextProvider) {
	if provider == nil {
		return
	}
	if provider.owner != nil && provider.owner.contextProviders != nil {
		delete(provider.owner.contextProviders, provider.id)
	}
	refreshContextTopology(provider.id)
}

func refreshContextTopology(contextID int) {
	if contextSubscriptionsByID == nil {
		return
	}
	for slot := range contextSubscriptionsByID[contextID] {
		refreshContextSubscription(slot)
	}
}

func refreshContextSubscription(slot *contextSubscription) {
	if slot == nil || slot.owner == nil || !slot.owner.active {
		releaseContextSubscription(slot)
		return
	}
	provider := findContextProvider(slot.owner.parent, slot.contextID)
	if provider == slot.provider {
		return
	}

	value := slot.defaultValue
	if provider != nil {
		value = provider.value
	}
	slot.update(slot, value)
	setContextSubscriptionProvider(slot, provider)
	markComponentDirty(slot.owner)
}

func releaseContextSubscriptions(instance *componentInstance) {
	for _, slot := range instance.contextSlots {
		releaseContextSubscription(slot)
	}
}

func releaseContextProviders(instance *componentInstance) {
	for _, provider := range instance.contextProviders {
		removeContextProvider(provider)
	}
}

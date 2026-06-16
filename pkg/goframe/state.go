package goframe

type stateSlot struct {
	value any
	owner *componentInstance
}

// State is a handle to one persistent component-owned state slot.
type State[T any] struct {
	slot *stateSlot
}

type updateBatch struct {
	pending bool
}

func (batch *updateBatch) request(enqueue func(func()), update func()) {
	if batch.pending {
		return
	}
	batch.pending = true
	enqueue(func() {
		batch.pending = false
		update()
	})
}

func (batch *updateBatch) reset() {
	batch.pending = false
}

// UseState returns the state slot at the current component render position.
// Calls to UseState must stay in the same order between component renders.
func UseState[T any](initial T) *State[T] {
	instance := currentComponent
	if instance == nil {
		panic("goframe: UseState must be called during component render")
	}

	index := instance.stateIndex
	instance.stateIndex++
	if index == len(instance.stateSlots) {
		instance.stateSlots = append(instance.stateSlots, &stateSlot{
			value: initial,
			owner: instance,
		})
	}
	slot := instance.stateSlots[index]
	if _, ok := slotValue[T](slot); !ok {
		panic("goframe: UseState type changed between component renders")
	}
	return &State[T]{slot: slot}
}

// Get returns the current state value.
func (state *State[T]) Get() T {
	value, ok := slotValue[T](state.slot)
	if !ok {
		panic("goframe: state contains an unexpected value type")
	}
	return value
}

// Set updates the state, marks its owner component dirty, and requests a
// batched patch of that component subtree.
func (state *State[T]) Set(value T) {
	owner := state.slot.owner
	if owner == nil || !owner.active {
		reportStateSetAfterUnmount(ownerDebugName(owner))
		return
	}
	if currentComponent != nil {
		reportStateSetDuringRender(ownerDebugName(owner), ownerDebugName(currentComponent))
	}
	if stateValuesEqual(state.slot.value, value) {
		return
	}
	state.slot.value = value
	markComponentDirty(owner)
}

func stateValuesEqual(oldValue, newValue any) bool {
	switch oldValue := oldValue.(type) {
	case string:
		newValue, ok := newValue.(string)
		return ok && oldValue == newValue
	case bool:
		newValue, ok := newValue.(bool)
		return ok && oldValue == newValue
	case int:
		newValue, ok := newValue.(int)
		return ok && oldValue == newValue
	case int8:
		newValue, ok := newValue.(int8)
		return ok && oldValue == newValue
	case int16:
		newValue, ok := newValue.(int16)
		return ok && oldValue == newValue
	case int32:
		newValue, ok := newValue.(int32)
		return ok && oldValue == newValue
	case int64:
		newValue, ok := newValue.(int64)
		return ok && oldValue == newValue
	case uint:
		newValue, ok := newValue.(uint)
		return ok && oldValue == newValue
	case uint8:
		newValue, ok := newValue.(uint8)
		return ok && oldValue == newValue
	case uint16:
		newValue, ok := newValue.(uint16)
		return ok && oldValue == newValue
	case uint32:
		newValue, ok := newValue.(uint32)
		return ok && oldValue == newValue
	case uint64:
		newValue, ok := newValue.(uint64)
		return ok && oldValue == newValue
	case float32:
		newValue, ok := newValue.(float32)
		return ok && oldValue == newValue
	case float64:
		newValue, ok := newValue.(float64)
		return ok && oldValue == newValue
	default:
		return false
	}
}

func slotValue[T any](slot *stateSlot) (T, bool) {
	value, ok := slot.value.(T)
	return value, ok
}

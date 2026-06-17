package goframe

type stateSlot struct {
	value   any
	owner   *componentInstance
	reducer any
}

type stateHandle[T any] struct {
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

// UseState returns the state value at the current component render position
// and a setter that marks the owning component dirty. Calls to UseState must
// stay in the same order between component renders.
func UseState[T any](initial T) (T, func(T)) {
	state := useStateSlot(initial, "UseState")
	return state.get(), state.set
}

// Reducer computes the next state from the current state and an action.
type Reducer[S any, A any] func(state S, action A) S

// UseReducer returns the state value at the current component render position
// and a dispatch function. Dispatch reads the latest slot state and the latest
// reducer stored for this slot, so old dispatch closures still apply actions to
// current state.
func UseReducer[S any, A any](initial S, reducer Reducer[S, A]) (S, func(A)) {
	if reducer == nil {
		panic("goframe: UseReducer reducer must not be nil")
	}
	state := useStateSlot(initial, "UseReducer")
	setReducer(state.slot, reducer)
	return state.get(), func(action A) {
		dispatchReducer[S, A](state.slot, action)
	}
}

func useState[T any](initial T) stateHandle[T] {
	return useStateSlot(initial, "UseState")
}

func useStateSlot[T any](initial T, hookName string) stateHandle[T] {
	instance := currentComponent
	if instance == nil {
		panic("goframe: " + hookName + " must be called during component render")
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
		panic("goframe: " + hookName + " state type changed between component renders")
	}
	return stateHandle[T]{slot: slot}
}

func (state stateHandle[T]) get() T {
	value, ok := slotValue[T](state.slot)
	if !ok {
		panic("goframe: state contains an unexpected value type")
	}
	return value
}

func (state stateHandle[T]) set(value T) {
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

func setReducer[S any, A any](slot *stateSlot, reducer Reducer[S, A]) {
	if slot.reducer != nil {
		if _, ok := slot.reducer.(Reducer[S, A]); !ok {
			panic("goframe: UseReducer reducer type changed between component renders")
		}
	}
	slot.reducer = reducer
}

func dispatchReducer[S any, A any](slot *stateSlot, action A) {
	owner := slot.owner
	if owner == nil || !owner.active {
		reportStateSetAfterUnmount(ownerDebugName(owner))
		return
	}
	state := stateHandle[S]{slot: slot}
	reducer, ok := slot.reducer.(Reducer[S, A])
	if !ok {
		panic("goframe: UseReducer reducer type changed between component renders")
	}
	state.set(reducer(state.get(), action))
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

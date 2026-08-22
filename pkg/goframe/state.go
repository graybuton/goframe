package goframe

type stateSlot struct {
	value   any
	owner   *componentInstance
	reducer any
	kind    string
	pending *stateRenderParticipant
}

type stateHandle[T any] struct {
	slot *stateSlot
}

type updateBatch struct {
	pending bool
	epoch   int
}

func (batch *updateBatch) request(enqueue func(func()), update func()) {
	if batch.pending {
		return
	}
	batch.pending = true
	epoch := batch.epoch
	enqueue(func() {
		if epoch != batch.epoch {
			return
		}
		batch.pending = false
		update()
	})
}

func (batch *updateBatch) reset() {
	batch.pending = false
	batch.epoch++
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
		panicRuntimeInvariant("goframe: UseReducer reducer must not be nil")
	}
	state := useStateSlot(initial, "UseReducer")
	setReducer(state.slot, reducer)
	return state.get(), func(action A) {
		dispatchReducer[S, A](state.slot, action)
	}
}

func useStateSlot[T any](initial T, hookName string) stateHandle[T] {
	instance := currentComponent
	if instance == nil {
		panicRuntimeInvariant("goframe: " + hookName + " must be called during component render")
	}

	index := instance.stateIndex
	instance.stateIndex++
	state := requireStateRenderParticipant(instance)
	if index >= len(instance.stateSlots) {
		pendingIndex := index - len(instance.stateSlots)
		if pendingIndex == len(state.slots) {
			state.addSlot(&stateSlot{
				value: initial,
				owner: instance,
				kind:  hookName,
			})
		}
		slot := state.slots[pendingIndex]
		if slot.kind != hookName {
			panicRuntimeInvariant("goframe: hook at state slot " + ToString(index) + " changed from " + slot.kind + " to " + hookName)
		}
		if _, ok := slotValue[T](slot); !ok {
			panicRuntimeInvariant("goframe: " + hookName + " state type changed between component renders")
		}
		return stateHandle[T]{slot: slot}
	}
	slot := instance.stateSlots[index]
	if slot.kind != hookName {
		panicRuntimeInvariant("goframe: hook at state slot " + ToString(index) + " changed from " + slot.kind + " to " + hookName)
	}
	if _, ok := slotValue[T](slot); !ok {
		panicRuntimeInvariant("goframe: " + hookName + " state type changed between component renders")
	}
	return stateHandle[T]{slot: slot}
}

func (state stateHandle[T]) get() T {
	value, ok := slotValue[T](state.slot)
	if !ok {
		panicRuntimeInvariant("goframe: state contains an unexpected value type")
	}
	return value
}

func stageStateValueForRender[T any](state stateHandle[T], value T) T {
	participant := requireStateRenderParticipant(currentComponent)
	if state.slot.owner != participant.instance {
		panicRuntimeInvariant("goframe: render state normalization requires the current component state")
	}
	participant.stageValue(state.slot, value)
	return value
}

func recordStateValueWithoutRender[T any](state stateHandle[T], value T) bool {
	owner := state.slot.owner
	if owner == nil || !owner.active {
		return false
	}
	if currentComponent != nil || state.slot.pending != nil {
		panicRuntimeInvariant("goframe: private state recording requires committed state outside render")
	}
	state.slot.value = value
	return true
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
	if pending := state.slot.pending; pending != nil {
		pending.dirty = true
		return
	}
	markComponentDirty(owner)
}

func setReducer[S any, A any](slot *stateSlot, reducer Reducer[S, A]) {
	state := requireStateRenderParticipant(currentComponent)
	currentReducer := slot.reducer
	if staged, ok := state.reducerFor(slot); ok {
		currentReducer = staged
	}
	if currentReducer != nil {
		if _, ok := currentReducer.(Reducer[S, A]); !ok {
			panicRuntimeInvariant("goframe: UseReducer reducer type changed between component renders")
		}
	}
	state.stageReducer(slot, reducer)
}

func dispatchReducer[S any, A any](slot *stateSlot, action A) {
	owner := slot.owner
	if owner == nil || !owner.active {
		reportStateSetAfterUnmount(ownerDebugName(owner))
		return
	}
	state := stateHandle[S]{slot: slot}
	reducerValue := slot.reducer
	if currentComponent == owner && currentComponent.lifecycleAttempt.active {
		if pending := currentComponent.lifecycleAttempt.state; pending != nil {
			if staged, stagedOK := pending.reducerFor(slot); stagedOK {
				reducerValue = staged
			}
		}
	}
	reducer, ok := reducerValue.(Reducer[S, A])
	if !ok {
		panicRuntimeInvariant("goframe: UseReducer reducer type changed between component renders")
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

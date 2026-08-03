package goframe

type stateReducerRenderUpdate struct {
	slot    *stateSlot
	reducer any
}

type stateRenderParticipant struct {
	attempt  *renderLifecycleAttempt
	instance *componentInstance
	slots    []*stateSlot
	reducers []stateReducerRenderUpdate
	dirty    bool
}

func requireStateRenderParticipant(instance *componentInstance) *stateRenderParticipant {
	attempt := requireLifecycleRenderAttempt(instance)
	state := attempt.state
	if state == nil {
		state = &stateRenderParticipant{}
		attempt.state = state
	}
	if state.attempt == nil {
		state.attempt = attempt
		state.instance = instance
		attempt.participants = append(attempt.participants, state)
	}
	return state
}

func (state *stateRenderParticipant) addSlot(slot *stateSlot) {
	slot.pending = state
	state.slots = append(state.slots, slot)
}

func (state *stateRenderParticipant) stageReducer(slot *stateSlot, reducer any) {
	if slot.pending == state {
		slot.reducer = reducer
		return
	}
	for index := range state.reducers {
		if state.reducers[index].slot == slot {
			state.reducers[index].reducer = reducer
			return
		}
	}
	state.reducers = append(state.reducers, stateReducerRenderUpdate{
		slot:    slot,
		reducer: reducer,
	})
}

func (state *stateRenderParticipant) reducerFor(slot *stateSlot) (any, bool) {
	for index := range state.reducers {
		if state.reducers[index].slot == slot {
			return state.reducers[index].reducer, true
		}
	}
	return nil, false
}

func (state *stateRenderParticipant) finishRender(attempt *renderLifecycleAttempt, commit bool) {
	if state.attempt != attempt {
		return
	}
	instance := state.instance
	dirty := state.dirty
	if commit {
		for _, slot := range state.slots {
			slot.pending = nil
			instance.stateSlots = append(instance.stateSlots, slot)
		}
		for _, update := range state.reducers {
			update.slot.reducer = update.reducer
		}
	} else {
		for _, slot := range state.slots {
			slot.value = nil
			slot.owner = nil
			slot.reducer = nil
			slot.kind = ""
			slot.pending = nil
		}
	}

	clear(state.slots)
	clear(state.reducers)
	state.attempt = nil
	state.instance = nil
	state.slots = state.slots[:0]
	state.reducers = state.reducers[:0]
	state.dirty = false

	if commit && dirty {
		markComponentDirty(instance)
	}
}

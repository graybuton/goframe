package goframe

func beginProtectedSubtreeLifecycle(state *errorBoundaryState) *errorBoundaryState {
	if state.phase != errorBoundaryProtected || state.attempts != nil {
		panic("goframe: invalid protected subtree transaction")
	}
	previous := currentProtectedLifecycleBoundary
	state.attempts = make([]*componentInstance, 0)
	state.teardown.begin()
	currentProtectedLifecycleBoundary = state
	return previous
}

func finishProtectedSubtreeLifecycle(
	state *errorBoundaryState,
	previous *errorBoundaryState,
) {
	recovered := recover()
	if state.attempts == nil ||
		currentProtectedLifecycleBoundary != state {
		panic("goframe: invalid protected subtree transaction")
	}

	currentProtectedLifecycleBoundary = previous
	attempts := state.attempts
	state.attempts = nil
	if state.phase == errorBoundaryProtected &&
		state.info.Phase != 0 {
		state.phase = errorBoundaryFallback
	}

	if recovered != nil {
		rollbackProtectedSubtreeLifecycleAttempts(attempts)
		state.teardown.retain()
		panic(recovered)
	}
	if state.phase != errorBoundaryProtected &&
		state.phase != errorBoundaryFallback {
		rollbackProtectedSubtreeLifecycleAttempts(attempts)
		state.teardown.retain()
		return
	}
	if previous != nil {
		// The outer boundary still owns commit: a later outer failure must be
		// able to roll back work from this successful nested subtree.
		previous.attempts = append(previous.attempts, attempts...)
		state.teardown.merge(&previous.teardown)
		return
	}
	state.teardown.retain()
	commitProtectedSubtreeLifecycleAttempts(attempts)
	state.teardown.release()
}

func finalizePendingProtectedSubtreeTeardown(state *errorBoundaryState) {
	if state == nil {
		return
	}
	if currentProtectedLifecycleBoundary != nil {
		state.teardown.merge(&currentProtectedLifecycleBoundary.teardown)
		return
	}
	state.teardown.release()
}

func commitProtectedSubtreeLifecycleAttempts(attempts []*componentInstance) {
	defer recoverProtectedSubtreeLifecycleAttempts(attempts)
	for _, attempt := range attempts {
		commitLifecycleRenderAttempt(attempt)
	}
}

func recoverProtectedSubtreeLifecycleAttempts(attempts []*componentInstance) {
	if recovered := recover(); recovered != nil {
		rollbackProtectedSubtreeLifecycleAttempts(attempts)
		panic(recovered)
	}
}

func rollbackProtectedSubtreeLifecycleAttempts(attempts []*componentInstance) {
	for index := len(attempts); index > 0; index-- {
		rollbackLifecycleRenderAttempt(attempts[index-1])
	}
}

package goframe

func beginProtectedSubtreeLifecycle(state *errorBoundaryState) *errorBoundaryState {
	if state.phase != errorBoundaryProtected || state.attempts != nil {
		panic("goframe: invalid protected subtree transaction")
	}
	previous := currentProtectedLifecycleBoundary
	state.attempts = make([]*componentInstance, 0)
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

	if recovered != nil {
		rollbackProtectedSubtreeLifecycleAttempts(attempts)
		panic(recovered)
	}
	if state.phase != errorBoundaryProtected {
		rollbackProtectedSubtreeLifecycleAttempts(attempts)
		return
	}
	if previous != nil {
		// The outer boundary still owns commit: a later outer failure must be
		// able to roll back work from this successful nested subtree.
		previous.attempts = append(previous.attempts, attempts...)
		return
	}
	commitProtectedSubtreeLifecycleAttempts(attempts)
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

//go:build !js || !wasm

package goframe

func installProtectedMountedTeardown() {}

func prepareProtectedFallbackReconcile(*errorBoundaryState) {}

type protectedSubtreeTeardown struct {
	release func()
	owned   bool
}

type protectedTeardownState struct {
	entries  []*protectedSubtreeTeardown
	retained int
}

func (state *protectedTeardownState) begin() {
	if state.retained != len(state.entries) {
		panicRuntimeInvariant("goframe: invalid protected subtree transaction")
	}
}

func (state *protectedTeardownState) stage(
	teardown *protectedSubtreeTeardown,
) bool {
	if teardown == nil {
		return false
	}
	if teardown.owned {
		return true
	}
	teardown.owned = true
	state.entries = append(state.entries, teardown)
	return true
}

func (state *protectedTeardownState) retain() {
	state.retained = len(state.entries)
}

func (state *protectedTeardownState) merge(target *protectedTeardownState) {
	target.entries = append(target.entries, state.entries...)
	state.entries = nil
	state.retained = 0
}

func (state *protectedTeardownState) release() {
	entries := state.entries
	state.entries = nil
	state.retained = 0
	for _, entry := range entries {
		if entry == nil || entry.release == nil {
			continue
		}
		release := entry.release
		entry.release = nil
		entry.owned = false
		release()
	}
}

func stageProtectedSubtreeTeardownForTest(
	teardown *protectedSubtreeTeardown,
) bool {
	state := currentProtectedLifecycleBoundary
	if state == nil {
		return false
	}
	return state.teardown.stage(teardown)
}

func protectedTeardownForTestCallback(
	release func(),
) *protectedSubtreeTeardown {
	return &protectedSubtreeTeardown{release: release}
}

func protectedTeardownStateForTest(
	state *errorBoundaryState,
) (active int, pending int) {
	if state == nil {
		return 0, 0
	}
	return len(state.teardown.entries) - state.teardown.retained,
		state.teardown.retained
}

func finalizePendingProtectedSubtreeTeardownForTest(
	state *errorBoundaryState,
) {
	finalizePendingProtectedSubtreeTeardown(state)
}

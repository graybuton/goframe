//go:build js && wasm

package goframe

import "syscall/js"

var stageProtectedMountedTeardown func(*mountedNode) bool

func installProtectedMountedTeardown() {
	stageProtectedMountedTeardown = stageMountedTeardown
}

func prepareProtectedFallbackReconcile(state *errorBoundaryState) {
	state.phase = errorBoundaryProtected
}

func stageMountedTeardown(mounted *mountedNode) bool {
	state := currentProtectedLifecycleBoundary
	if state == nil {
		return false
	}
	return state.teardown.stage(mounted)
}

type protectedTeardownState []*mountedNode

func (state *protectedTeardownState) begin() {}

func (state *protectedTeardownState) stage(
	mounted *mountedNode,
) bool {
	if mounted == nil {
		return false
	}
	if mounted.pending.Type() == js.TypeNull {
		return true
	}
	mounted.pending = js.Null()
	*state = append(*state, mounted)
	return true
}

func (state *protectedTeardownState) retain() {}

func (state *protectedTeardownState) merge(target *protectedTeardownState) {
	*target = append(*target, *state...)
	*state = nil
}

func (state *protectedTeardownState) release() {
	entries := *state
	*state = nil
	for _, teardown := range entries {
		releaseMounted(teardown)
	}
}

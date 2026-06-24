package goframe

// ErrorBoundaryContext is passed to an ErrorBoundary fallback renderer.
type ErrorBoundaryContext struct {
	Info  ErrorInfo
	Reset func()
}

// ErrorBoundaryProps configures a scoped render error boundary.
type ErrorBoundaryProps struct {
	ResetKey string
	Fallback func(ErrorBoundaryContext) Node
	Children []Node
}

type errorBoundaryPhase uint8

const (
	errorBoundaryProtected errorBoundaryPhase = iota
	errorBoundaryCaptured
	errorBoundaryFallback
)

type errorBoundaryState struct {
	phase       errorBoundaryPhase
	info        ErrorInfo
	generation  int
	resetKey    string
	hasResetKey bool
}

var errorBoundaryComponentType = NewComponentType("goframe.ErrorBoundary", "ErrorBoundary")

// ErrorBoundary renders children until a descendant component render panics,
// then renders Fallback until Reset is called or ResetKey changes.
func ErrorBoundary(props ErrorBoundaryProps) Node {
	return ComponentT(errorBoundaryComponentType, props, renderErrorBoundary)
}

func renderErrorBoundary(props ErrorBoundaryProps) Node {
	if props.Fallback == nil {
		panic("goframe: ErrorBoundary requires Fallback")
	}
	instance := requireCurrentComponent("ErrorBoundary")
	state := ensureErrorBoundaryState(instance)
	updateErrorBoundaryResetKey(state, props.ResetKey)
	if state.phase == errorBoundaryCaptured {
		state.phase = errorBoundaryFallback
	}
	if state.phase == errorBoundaryFallback {
		context := ErrorBoundaryContext{
			Info: state.info,
			Reset: func() {
				resetErrorBoundary(instance, state)
			},
		}
		return Key(errorBoundaryFallbackKey(state.generation), Child(props.Fallback(context)))
	}
	return Key(errorBoundaryProtectedKey(state.generation), Fragment(props.Children...))
}

func ensureErrorBoundaryState(instance *componentInstance) *errorBoundaryState {
	if instance.errorBoundary == nil {
		instance.errorBoundary = &errorBoundaryState{}
	}
	return instance.errorBoundary
}

func updateErrorBoundaryResetKey(state *errorBoundaryState, resetKey string) {
	if !state.hasResetKey {
		state.resetKey = resetKey
		state.hasResetKey = true
		return
	}
	if state.resetKey == resetKey {
		return
	}
	state.resetKey = resetKey
	if state.phase != errorBoundaryProtected {
		clearErrorBoundary(state)
	}
}

func clearErrorBoundary(state *errorBoundaryState) {
	state.phase = errorBoundaryProtected
	state.info = ErrorInfo{}
	state.generation++
}

func resetErrorBoundary(instance *componentInstance, state *errorBoundaryState) {
	if instance == nil || !instance.active || instance.errorBoundary != state || state.phase == errorBoundaryProtected {
		return
	}
	clearErrorBoundary(state)
	markComponentDirty(instance)
}

func captureRenderErrorBoundary(failing *componentInstance, info ErrorInfo) {
	boundary := nearestErrorBoundary(failing)
	if boundary == nil || boundary.errorBoundary == nil {
		return
	}
	cancelPendingEffectsUnderBoundary(boundary)
	state := boundary.errorBoundary
	if state.phase == errorBoundaryProtected {
		state.phase = errorBoundaryCaptured
		state.info = info
		state.generation++
	}
	markComponentDirty(boundary)
}

func nearestErrorBoundary(instance *componentInstance) *componentInstance {
	for current := instance.parent; current != nil; current = current.parent {
		if current.active && current.errorBoundary != nil && current.errorBoundary.phase != errorBoundaryFallback {
			return current
		}
	}
	return nil
}

func cancelPendingEffectsForRenderFailure(instance *componentInstance) {
	if instance == nil {
		return
	}
	for _, slot := range instance.effectSlots {
		if slot == nil {
			continue
		}
		slot.pending = false
		slot.queued = false
	}
}

func cancelPendingEffectsUnderBoundary(boundary *componentInstance) {
	if boundary == nil {
		return
	}
	for _, slot := range pendingEffects {
		if slot == nil || slot.owner == nil || slot.owner == boundary {
			continue
		}
		if !componentIsDescendantOf(slot.owner, boundary) {
			continue
		}
		slot.pending = false
		slot.queued = false
	}
}

func componentIsDescendantOf(instance *componentInstance, ancestor *componentInstance) bool {
	for current := instance; current != nil; current = current.parent {
		if current == ancestor {
			return true
		}
	}
	return false
}

func errorBoundaryProtectedKey(generation int) string {
	return "goframe:error-boundary:protected:" + ToString(generation)
}

func errorBoundaryFallbackKey(generation int) string {
	return "goframe:error-boundary:fallback:" + ToString(generation)
}

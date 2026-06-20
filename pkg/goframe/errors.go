package goframe

// ErrorPhase identifies the runtime phase where user code panicked.
type ErrorPhase uint8

const (
	ErrorPhaseRender ErrorPhase = iota + 1
	ErrorPhaseEvent
	ErrorPhaseEffect
	ErrorPhaseEffectCleanup
	ErrorPhaseUnmountCleanup
	ErrorPhaseMemo
	ErrorPhaseContext
)

func (phase ErrorPhase) String() string {
	switch phase {
	case ErrorPhaseRender:
		return "render"
	case ErrorPhaseEvent:
		return "event"
	case ErrorPhaseEffect:
		return "effect"
	case ErrorPhaseEffectCleanup:
		return "effect-cleanup"
	case ErrorPhaseUnmountCleanup:
		return "unmount-cleanup"
	case ErrorPhaseMemo:
		return "memo"
	case ErrorPhaseContext:
		return "context"
	default:
		return ""
	}
}

// ErrorInfo describes a recovered runtime panic.
type ErrorInfo struct {
	Phase     ErrorPhase
	Component string
	Operation string
	Panic     any
}

// ErrorHandler receives runtime error reports.
type ErrorHandler func(ErrorInfo)

var runtimeErrorHandler ErrorHandler

// SetErrorHandler installs a runtime error handler and returns a restore
// function for tests and scoped application setup.
func SetErrorHandler(handler ErrorHandler) func() {
	previous := runtimeErrorHandler
	runtimeErrorHandler = handler
	return func() {
		runtimeErrorHandler = previous
	}
}

func reportRuntimeError(info ErrorInfo) {
	if runtimeErrorHandler != nil {
		runtimeErrorHandler(info)
	}
}

func reportRecoveredRuntimeError(info ErrorInfo, recovered any) {
	info.Panic = recovered
	reportRuntimeError(info)
}

func runtimeComponentName(instance *componentInstance) string {
	if instance == nil {
		return ""
	}
	return instance.name
}

func isRuntimeInvariantPanic(value any) bool {
	text, ok := value.(string)
	return ok && hasStringPrefix(text, "goframe:")
}

func hasStringPrefix(value string, prefix string) bool {
	if len(value) < len(prefix) {
		return false
	}
	return value[:len(prefix)] == prefix
}

package goframe

// Cleanup releases resources created by a lifecycle effect.
type Cleanup func()

type depsMode uint8

const (
	depsOnce depsMode = iota
	depsAlways
	depsValues
)

// EffectDeps is an explicit, lightweight dependency set for UseEffect.
type EffectDeps struct {
	mode   depsMode
	values []Dep
}

type depKind uint8

const (
	depNil depKind = iota + 1
	depString
	depBool
	depInt
	depInt8
	depInt16
	depInt32
	depInt64
	depUint
	depUint8
	depUint16
	depUint32
	depUint64
	depUintptr
	depFloat32
	depFloat64
)

// Dep is one comparable effect dependency value.
type Dep struct {
	kind     depKind
	text     string
	signed   int64
	unsigned uint64
	float    float64
	boolean  bool
}

// Once returns a dependency set that runs an effect only after mount.
func Once() EffectDeps {
	return EffectDeps{mode: depsOnce}
}

// EveryRender returns a dependency set that runs an effect after every render.
func EveryRender() EffectDeps {
	return EffectDeps{mode: depsAlways}
}

// Deps returns a dependency set from primitive comparable dependency values.
// Complex values should be reduced to a string, id, version, or counter first.
func Deps(values ...any) EffectDeps {
	if len(values) == 0 {
		return Once()
	}
	deps := make([]Dep, len(values))
	for index, value := range values {
		deps[index] = dependencyValue(value)
	}
	return EffectDeps{
		mode:   depsValues,
		values: deps,
	}
}

// NoDeps returns a dependency set that runs an effect only after mount.
//
// Deprecated: use Once or call UseEffect with no dependency argument.
func NoDeps() EffectDeps {
	return Once()
}

// AlwaysDeps returns a dependency set that runs an effect after every render.
//
// Deprecated: use EveryRender.
func AlwaysDeps() EffectDeps {
	return EveryRender()
}

// DepsOf returns a dependency set from explicit comparable dependency values.
//
// Deprecated: use Deps.
func DepsOf(values ...Dep) EffectDeps {
	if len(values) == 0 {
		return Once()
	}
	copied := make([]Dep, len(values))
	copy(copied, values)
	return EffectDeps{
		mode:   depsValues,
		values: copied,
	}
}

func dependencyValue(value any) Dep {
	switch value := value.(type) {
	case nil:
		return Dep{kind: depNil}
	case string:
		return DepString(value)
	case bool:
		return DepBool(value)
	case int:
		return Dep{kind: depInt, signed: int64(value)}
	case int8:
		return Dep{kind: depInt8, signed: int64(value)}
	case int16:
		return Dep{kind: depInt16, signed: int64(value)}
	case int32:
		return Dep{kind: depInt32, signed: int64(value)}
	case int64:
		return Dep{kind: depInt64, signed: value}
	case uint:
		return Dep{kind: depUint, unsigned: uint64(value)}
	case uint8:
		return Dep{kind: depUint8, unsigned: uint64(value)}
	case uint16:
		return Dep{kind: depUint16, unsigned: uint64(value)}
	case uint32:
		return Dep{kind: depUint32, unsigned: uint64(value)}
	case uint64:
		return Dep{kind: depUint64, unsigned: value}
	case uintptr:
		return Dep{kind: depUintptr, unsigned: uint64(value)}
	case float32:
		return Dep{kind: depFloat32, float: float64(value)}
	case float64:
		return Dep{kind: depFloat64, float: value}
	default:
		panic("goframe: unsupported effect dependency type; reduce complex values to string, id, version, or counter")
	}
}

func DepString(value string) Dep {
	return Dep{kind: depString, text: value}
}

func DepBool(value bool) Dep {
	return Dep{kind: depBool, boolean: value}
}

func DepInt(value int) Dep {
	return Dep{kind: depInt, signed: int64(value)}
}

func DepInt64(value int64) Dep {
	return Dep{kind: depInt64, signed: value}
}

func DepUint(value uint) Dep {
	return Dep{kind: depUint, unsigned: uint64(value)}
}

func DepUint64(value uint64) Dep {
	return Dep{kind: depUint64, unsigned: value}
}

func DepFloat64(value float64) Dep {
	return Dep{kind: depFloat64, float: value}
}

type effectKind uint8

const (
	effectMount effectKind = iota + 1
	effectRegular
)

type effectSlot struct {
	owner   *componentInstance
	kind    effectKind
	effect  func() Cleanup
	cleanup Cleanup
	deps    EffectDeps
	pending bool
	queued  bool
	running bool
	hasRun  bool
}

type effectRenderUpdate struct {
	effect func() Cleanup
	deps   EffectDeps
	slot   *effectSlot
	kind   effectKind
	queue  bool
}

type lifecycleRenderParticipant interface {
	commitLifecycleRender(*renderLifecycleAttempt)
	rollbackLifecycleRender(*renderLifecycleAttempt)
}

type renderLifecycleAttempt struct {
	active       bool
	effects      []effectRenderUpdate
	unmounts     []Cleanup
	participants []lifecycleRenderParticipant
	// Keep hook commit indirect so hook-free TinyGo apps discard effect machinery.
	commitHooks func(*componentInstance)
}

var (
	pendingEffects  []*effectSlot
	flushingEffects bool
)

// UseMount runs effect once after this component instance is first mounted.
// The returned cleanup, when non-nil, runs when the instance unmounts.
//
// Deprecated: use UseEffect with no dependency argument or Once.
func UseMount(effect func() Cleanup) {
	useEffect(effectMount, effect, Once())
}

// UseUnmount registers a cleanup for this component instance.
func UseUnmount(cleanup Cleanup) {
	instance := requireCurrentComponent("UseUnmount")
	index := instance.unmountIndex
	instance.unmountIndex++
	attempt := requireLifecycleRenderAttempt(instance)
	if index != len(attempt.unmounts) {
		panic("goframe: invalid unmount hook index")
	}
	attempt.commitHooks = commitLifecycleHooks
	attempt.unmounts = append(attempt.unmounts, cleanup)
}

// UseEffect runs effect after mount. With Deps it reruns after dependency
// changes; with EveryRender it reruns after each component render.
func UseEffect(effect func() Cleanup, deps ...EffectDeps) {
	useEffect(effectRegular, effect, effectDepsArg(deps))
}

func effectDepsArg(deps []EffectDeps) EffectDeps {
	if len(deps) == 0 {
		return Once()
	}
	if len(deps) > 1 {
		panic("goframe: UseEffect accepts at most one dependency set")
	}
	return deps[0]
}

func useEffect(kind effectKind, effect func() Cleanup, deps EffectDeps) {
	if effect == nil {
		panic("goframe: UseEffect requires an effect function")
	}
	instance := requireCurrentComponent("UseEffect")
	index := instance.effectIndex
	instance.effectIndex++
	attempt := requireLifecycleRenderAttempt(instance)
	if index != len(attempt.effects) {
		panic("goframe: invalid effect hook index")
	}
	attempt.commitHooks = commitLifecycleHooks

	update := effectRenderUpdate{
		kind:   kind,
		effect: effect,
	}
	if index >= len(instance.effectSlots) {
		update.deps = copyDeps(deps)
		update.queue = true
	} else {
		slot := instance.effectSlots[index]
		if slot.kind != kind {
			panic("goframe: lifecycle hook type changed between component renders")
		}
		update.slot = slot
		if shouldRunEffect(slot, deps) {
			update.deps = copyDeps(deps)
			update.queue = true
		}
	}
	attempt.effects = append(attempt.effects, update)
}

func requireCurrentComponent(hook string) *componentInstance {
	instance := currentComponent
	if instance == nil {
		panic("goframe: " + hook + " must be called during component render")
	}
	return instance
}

func beginLifecycleRenderAttempt(instance *componentInstance) {
	if instance == nil {
		panic("goframe: lifecycle render attempt requires a component")
	}
	attempt := &instance.lifecycleAttempt
	if attempt.active {
		panic("goframe: component lifecycle render attempt is already active")
	}
	attempt.active = true
}

func requireLifecycleRenderAttempt(instance *componentInstance) *renderLifecycleAttempt {
	if instance == nil || !instance.lifecycleAttempt.active {
		panic("goframe: lifecycle hook requires an active render attempt")
	}
	return &instance.lifecycleAttempt
}

func commitLifecycleRenderAttempt(instance *componentInstance) {
	attempt := requireLifecycleRenderAttempt(instance)
	for _, participant := range attempt.participants {
		participant.commitLifecycleRender(attempt)
	}
	if attempt.commitHooks != nil {
		attempt.commitHooks(instance)
	}
	finishLifecycleRenderAttempt(attempt)
}

func commitLifecycleHooks(instance *componentInstance) {
	attempt := &instance.lifecycleAttempt
	for index := range attempt.effects {
		update := &attempt.effects[index]
		slot := update.slot
		if slot == nil {
			slot = &effectSlot{
				owner: instance,
				kind:  update.kind,
			}
			instance.effectSlots = append(instance.effectSlots, slot)
		}
		slot.effect = update.effect
		if update.queue {
			slot.deps = update.deps
		}
		update.slot = slot
	}
	if len(attempt.unmounts) > len(instance.unmountSlots) {
		instance.unmountSlots = append(instance.unmountSlots, attempt.unmounts[len(instance.unmountSlots):]...)
	}
	copy(instance.unmountSlots, attempt.unmounts)
	for index := range attempt.effects {
		update := &attempt.effects[index]
		if update.queue {
			queueEffect(update.slot)
		}
	}
}

func rollbackLifecycleRenderAttempt(instance *componentInstance) {
	if instance == nil || !instance.lifecycleAttempt.active {
		return
	}
	attempt := &instance.lifecycleAttempt
	for _, participant := range attempt.participants {
		participant.rollbackLifecycleRender(attempt)
	}
	finishLifecycleRenderAttempt(attempt)
}

func finishLifecycleRenderAttempt(attempt *renderLifecycleAttempt) {
	clear(attempt.effects)
	clear(attempt.unmounts)
	clear(attempt.participants)
	attempt.commitHooks = nil
	attempt.effects = attempt.effects[:0]
	attempt.unmounts = attempt.unmounts[:0]
	attempt.participants = attempt.participants[:0]
	attempt.active = false
}

func releaseLifecycleRenderAttempt(instance *componentInstance) {
	if instance == nil {
		return
	}
	rollbackLifecycleRenderAttempt(instance)
	instance.lifecycleAttempt = renderLifecycleAttempt{}
}

func shouldRunEffect(slot *effectSlot, deps EffectDeps) bool {
	if !slot.hasRun {
		return true
	}
	if deps.mode == depsAlways {
		return true
	}
	return !depsEqual(slot.deps, deps)
}

func queueEffect(slot *effectSlot) {
	if slot == nil || slot.queued {
		return
	}
	slot.pending = true
	slot.queued = true
	pendingEffects = append(pendingEffects, slot)
}

func flushPendingEffects() {
	flushingEffects = true
	defer func() {
		flushingEffects = false
	}()
	for len(pendingEffects) > 0 {
		effects := pendingEffects
		pendingEffects = nil
		for _, slot := range effects {
			slot.queued = false
			if !slot.pending || slot.owner == nil || !slot.owner.active {
				continue
			}
			slot.pending = false
			if slot.cleanup != nil {
				runEffectCleanup(slot)
				slot.cleanup = nil
			}
			if cleanup, ok := runEffectSetup(slot); ok {
				slot.cleanup = cleanup
				slot.hasRun = true
			}
		}
	}
}

func runEffectSetup(slot *effectSlot) (cleanup Cleanup, ok bool) {
	slot.running = true
	defer func() {
		slot.running = false
		if recovered := recover(); recovered != nil {
			reportRecoveredRuntimeError(ErrorInfo{
				Phase:     ErrorPhaseEffect,
				Component: runtimeComponentName(slot.owner),
				Operation: "UseEffect",
			}, recovered)
			cleanup = nil
			ok = false
		}
	}()
	return slot.effect(), true
}

func runEffectCleanup(slot *effectSlot) {
	defer func() {
		if recovered := recover(); recovered != nil {
			reportRecoveredRuntimeError(ErrorInfo{
				Phase:     ErrorPhaseEffectCleanup,
				Component: runtimeComponentName(slot.owner),
				Operation: "UseEffect cleanup",
			}, recovered)
		}
	}()
	slot.cleanup()
}

func runUnmountCleanup(instance *componentInstance, cleanup Cleanup) {
	defer func() {
		if recovered := recover(); recovered != nil {
			reportRecoveredRuntimeError(ErrorInfo{
				Phase:     ErrorPhaseUnmountCleanup,
				Component: runtimeComponentName(instance),
				Operation: "UseUnmount cleanup",
			}, recovered)
		}
	}()
	cleanup()
}

func runUnmountCleanups(instance *componentInstance) {
	for _, slot := range instance.effectSlots {
		if slot == nil {
			continue
		}
		slot.pending = false
		slot.queued = false
		if slot.cleanup != nil {
			runEffectCleanup(slot)
			slot.cleanup = nil
		}
		slot.owner = nil
	}
	for index, cleanup := range instance.unmountSlots {
		if cleanup != nil {
			runUnmountCleanup(instance, cleanup)
			instance.unmountSlots[index] = nil
		}
	}
}

func copyDeps(deps EffectDeps) EffectDeps {
	if len(deps.values) == 0 {
		return EffectDeps{mode: deps.mode}
	}
	copied := make([]Dep, len(deps.values))
	copy(copied, deps.values)
	return EffectDeps{
		mode:   deps.mode,
		values: copied,
	}
}

func depsEqual(first, second EffectDeps) bool {
	if first.mode != second.mode || len(first.values) != len(second.values) {
		return false
	}
	for index := range first.values {
		if first.values[index] != second.values[index] {
			return false
		}
	}
	return true
}

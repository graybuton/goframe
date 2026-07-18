package goframe

// ResourceStatus is the current state of a component-scoped resource.
type ResourceStatus uint8

const (
	ResourceLoading ResourceStatus = iota
	ResourceReady
	ResourceFailed
)

// Resource is the public snapshot returned by UseResource.
type Resource[T any] struct {
	Status ResourceStatus
	Value  T
	Err    error
}

// Loading reports whether the resource is currently loading.
func (resource Resource[T]) Loading() bool {
	return resource.Status == ResourceLoading
}

// Ready reports whether the resource has a ready value.
func (resource Resource[T]) Ready() bool {
	return resource.Status == ResourceReady
}

// Failed reports whether the resource failed with an error.
func (resource Resource[T]) Failed() bool {
	return resource.Status == ResourceFailed
}

// ResourceLoader starts one resource generation for key.
//
// The first resolve or reject call wins for the generation. The optional
// cleanup runs when the generation is invalidated or the component unmounts.
type ResourceLoader[T any] func(key string, resolve func(T), reject func(error)) Cleanup

// UseResource owns one component-scoped asynchronous resource.
//
// The loader starts after patch through the normal effect lifecycle. The
// returned reload function starts a new generation for the current key.
func UseResource[T any](key string, loader ResourceLoader[T]) (Resource[T], func()) {
	if loader == nil {
		panic("goframe: UseResource requires a loader")
	}
	slot := useStateSlot[*resourceControl[T]](nil, "UseResource")
	control := slot.get()
	if control == nil {
		control = newResourceControl[T]()
		slot.slot.value = control
	}
	attempt := requireLifecycleRenderAttempt(currentComponent)
	snapshot, generation := control.prepareRender(attempt, currentComponent, key, loader)

	UseEffect(func() Cleanup {
		return control.start(key, generation)
	}, Deps(key, generation))

	return snapshot, control.reload
}

type resourceControl[T any] struct {
	committed  bool
	owner      *componentInstance
	key        string
	generation int
	loader     ResourceLoader[T]
	snapshot   Resource[T]
	current    *resourceRun[T]
	pending    resourceRenderState[T]
}

type resourceRenderState[T any] struct {
	attempt    *renderLifecycleAttempt
	owner      *componentInstance
	key        string
	generation int
	loader     ResourceLoader[T]
	snapshot   Resource[T]
	initial    bool
	keyChanged bool
}

type resourceRun[T any] struct {
	control       *resourceControl[T]
	generation    int
	active        bool
	completed     bool
	cleanup       Cleanup
	cleanupCalled bool
}

type resourceInternalError string

func (err resourceInternalError) Error() string {
	return string(err)
}

const (
	resourceNilRejectError   resourceInternalError = "goframe: resource rejected without error"
	resourceLoaderPanicError resourceInternalError = "goframe: resource loader panicked"
)

func newResourceControl[T any]() *resourceControl[T] {
	return &resourceControl[T]{
		snapshot: resourceLoading[T](),
	}
}

func (control *resourceControl[T]) prepareRender(
	attempt *renderLifecycleAttempt,
	owner *componentInstance,
	key string,
	loader ResourceLoader[T],
) (Resource[T], int) {
	if control.pending.attempt != nil {
		panic("goframe: resource already participated in this render attempt")
	}
	pending := resourceRenderState[T]{
		attempt:    attempt,
		owner:      owner,
		key:        control.key,
		generation: control.generation,
		loader:     loader,
		snapshot:   control.snapshot,
	}
	if !control.committed {
		pending.initial = true
		pending.key = key
		pending.snapshot = resourceLoading[T]()
	} else if control.key != key {
		pending.keyChanged = true
		pending.key = key
		pending.generation++
		pending.snapshot = resourceLoading[T]()
	}
	control.pending = pending
	attempt.participants = append(attempt.participants, control)
	return pending.snapshot, pending.generation
}

func (control *resourceControl[T]) commitLifecycleRender(attempt *renderLifecycleAttempt) {
	pending := control.pending
	if pending.attempt != attempt {
		return
	}
	control.owner = pending.owner
	control.loader = pending.loader
	if pending.initial {
		control.committed = true
		control.key = pending.key
		control.generation = pending.generation
		control.snapshot = pending.snapshot
	} else if pending.keyChanged {
		control.invalidateCurrent()
		control.key = pending.key
		control.generation = pending.generation
		control.snapshot = pending.snapshot
	}
	control.pending = resourceRenderState[T]{}
}

func (control *resourceControl[T]) rollbackLifecycleRender(attempt *renderLifecycleAttempt) {
	if control.pending.attempt == attempt {
		control.pending = resourceRenderState[T]{}
	}
}

func (control *resourceControl[T]) reload() {
	if control == nil || !control.committed || control.owner == nil || !control.owner.active {
		return
	}
	control.invalidateCurrent()
	control.generation++
	control.snapshot = resourceLoading[T]()
	markComponentDirty(control.owner)
}

func (control *resourceControl[T]) start(key string, generation int) (cleanup Cleanup) {
	if control == nil || control.owner == nil || !control.owner.active {
		return nil
	}
	if control.key != key || control.generation != generation {
		return nil
	}
	run := &resourceRun[T]{
		control:    control,
		generation: generation,
		active:     true,
	}
	control.current = run
	defer func() {
		if recovered := recover(); recovered != nil {
			control.fail(run, resourceLoaderPanicError)
			reportRecoveredRuntimeError(ErrorInfo{
				Phase:     ErrorPhaseEffect,
				Component: runtimeComponentName(control.owner),
				Operation: "UseEffect",
			}, recovered)
			cleanup = nil
		}
	}()
	run.cleanup = control.loader(key, func(value T) {
		control.resolve(run, value)
	}, func(err error) {
		control.reject(run, err)
	})
	cleanup = func() {
		control.cleanupRun(run)
	}
	return cleanup
}

func (control *resourceControl[T]) resolve(run *resourceRun[T], value T) {
	control.complete(run, Resource[T]{
		Status: ResourceReady,
		Value:  value,
	})
}

func (control *resourceControl[T]) reject(run *resourceRun[T], err error) {
	if err == nil {
		err = resourceNilRejectError
	}
	control.fail(run, err)
}

func (control *resourceControl[T]) fail(run *resourceRun[T], err error) {
	control.complete(run, Resource[T]{
		Status: ResourceFailed,
		Err:    err,
	})
}

func (control *resourceControl[T]) complete(run *resourceRun[T], snapshot Resource[T]) {
	if !control.accepts(run) {
		return
	}
	run.completed = true
	run.active = false
	control.snapshot = snapshot
	markComponentDirty(control.owner)
}

func (control *resourceControl[T]) accepts(run *resourceRun[T]) bool {
	return control != nil &&
		run != nil &&
		control.current == run &&
		control.owner != nil &&
		control.owner.active &&
		control.generation == run.generation &&
		run.active &&
		!run.completed
}

func (control *resourceControl[T]) invalidateCurrent() {
	if control == nil || control.current == nil {
		return
	}
	control.current.active = false
}

func (control *resourceControl[T]) cleanupRun(run *resourceRun[T]) {
	if run == nil || run.cleanupCalled {
		return
	}
	run.active = false
	run.cleanupCalled = true
	if control != nil && control.current == run {
		control.current = nil
	}
	if run.cleanup != nil {
		run.cleanup()
	}
}

func resourceLoading[T any]() Resource[T] {
	return Resource[T]{Status: ResourceLoading}
}

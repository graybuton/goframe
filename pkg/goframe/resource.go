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
		control = newResourceControl[T](currentComponent, key, loader)
		slot.slot.value = control
	}
	control.owner = currentComponent
	control.loader = loader
	control.prepareKey(key)

	generation := control.generation
	UseEffect(func() Cleanup {
		return control.start(key, generation)
	}, Deps(key, generation))

	return control.snapshot, control.reload
}

type resourceControl[T any] struct {
	owner      *componentInstance
	key        string
	generation int
	loader     ResourceLoader[T]
	snapshot   Resource[T]
	current    *resourceRun[T]
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

func newResourceControl[T any](owner *componentInstance, key string, loader ResourceLoader[T]) *resourceControl[T] {
	return &resourceControl[T]{
		owner:    owner,
		key:      key,
		loader:   loader,
		snapshot: resourceLoading[T](),
	}
}

func (control *resourceControl[T]) prepareKey(key string) {
	if control.key == key {
		return
	}
	control.invalidateCurrent()
	control.key = key
	control.generation++
	control.snapshot = resourceLoading[T]()
}

func (control *resourceControl[T]) reload() {
	if control == nil || control.owner == nil || !control.owner.active {
		return
	}
	control.invalidateCurrent()
	control.generation++
	control.snapshot = resourceLoading[T]()
	markComponentDirty(control.owner)
}

func (control *resourceControl[T]) start(key string, generation int) Cleanup {
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
			panic(recovered)
		}
	}()
	run.cleanup = control.loader(key, func(value T) {
		control.resolve(run, value)
	}, func(err error) {
		control.reject(run, err)
	})
	return func() {
		control.cleanupRun(run)
	}
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

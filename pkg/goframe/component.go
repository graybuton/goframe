package goframe

// ComponentFunc renders one typed component props value.
type ComponentFunc[P any] func(P) Node

type memoizedProps[P any] interface {
	MemoEqual(next P) bool
}

type componentIdentity struct {
	typed bool
	id    string
}

// ComponentType is an explicit component identity token.
//
// It separates runtime identity from the human-readable debug name. Use
// NewComponentType to create values; the zero value is not valid for
// ComponentT.
type ComponentType struct {
	identity  componentIdentity
	debugName string
}

// NewComponentType creates an explicit component identity token.
func NewComponentType(id string, debugName string) ComponentType {
	if id == "" {
		panic("goframe: empty component id")
	}
	if debugName == "" {
		debugName = id
	}
	return ComponentType{
		identity:  typedComponentIdentity(id),
		debugName: debugName,
	}
}

// ComponentNode preserves a function component boundary until the runtime
// creates or reuses its component instance.
type ComponentNode struct {
	Name      string
	Props     any
	identity  componentIdentity
	render    func() Node
	memoEqual func(any, any) bool
}

func (ComponentNode) isNode() {}

// Component creates a runtime-visible typed function component boundary.
func Component[P any](name string, props P, render ComponentFunc[P]) Node {
	return componentNode(name, legacyComponentIdentity(name), props, render)
}

// ComponentT creates a runtime-visible component boundary with an explicit
// component identity token.
func ComponentT[P any](componentType ComponentType, props P, render ComponentFunc[P]) Node {
	if componentType.identity.id == "" {
		panic("goframe: invalid component type")
	}
	return componentNode(componentType.debugName, componentType.identity, props, render)
}

func componentNode[P any](name string, identity componentIdentity, props P, render ComponentFunc[P]) Node {
	var memoEqual func(any, any) bool
	if _, ok := any(props).(memoizedProps[P]); ok {
		memoEqual = memoizeProps[P]
	}
	return ComponentNode{
		Name:      name,
		Props:     props,
		identity:  identity,
		memoEqual: memoEqual,
		render: func() Node {
			return render(props)
		},
	}
}

func memoizeProps[P any](oldProps, nextProps any) bool {
	oldValue, ok := oldProps.(P)
	if !ok {
		return false
	}
	nextValue, ok := nextProps.(P)
	if !ok {
		return false
	}
	memoizer, ok := any(oldValue).(memoizedProps[P])
	if !ok {
		return false
	}
	return memoizer.MemoEqual(nextValue)
}

// C is the short form of Component.
func C[P any](name string, props P, render ComponentFunc[P]) Node {
	return Component(name, props, render)
}

type componentInstance struct {
	name             string
	identity         componentIdentity
	key              string
	parent           *componentInstance
	node             ComponentNode
	memoEqual        func(any, any) bool
	stateSlots       []*stateSlot
	stateIndex       int
	effectSlots      []*effectSlot
	effectIndex      int
	unmountSlots     []*unmountSlot
	unmountIndex     int
	contextSlots     []*contextSubscription
	contextIndex     int
	contextProviders map[int]*contextProvider
	providedContexts []int
	errorBoundary    *errorBoundaryState
	dirty            bool
	dirtyCounted     bool
	dirtyDescendants int
	active           bool
	scheduleUpdate   func(*componentInstance)
	update           func()
}

var currentComponent *componentInstance

func newComponentInstance(node ComponentNode, key string, parent *componentInstance, schedule func(*componentInstance)) *componentInstance {
	return &componentInstance{
		name:           node.Name,
		identity:       nodeComponentIdentity(node),
		key:            key,
		parent:         parent,
		node:           node,
		memoEqual:      node.memoEqual,
		dirty:          true,
		active:         true,
		scheduleUpdate: schedule,
	}
}

func shouldSkipComponentRender(instance *componentInstance, nextNode ComponentNode, nextKey string) bool {
	if instance == nil || instance.identity != nodeComponentIdentity(nextNode) {
		return false
	}
	if instance.key != nextKey {
		return false
	}
	if instance.memoEqual == nil || instance.dirty || instance.dirtyDescendants > 0 || !instance.active {
		return false
	}
	return shouldSkipMemoizedProps(instance, nextNode)
}

func shouldSkipMemoizedProps(instance *componentInstance, nextNode ComponentNode) (skip bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			reportRecoveredRuntimeError(ErrorInfo{
				Phase:     ErrorPhaseMemo,
				Component: runtimeComponentName(instance),
				Operation: "MemoEqual",
			}, recovered)
			skip = false
		}
	}()
	return instance.memoEqual(instance.node.Props, nextNode.Props)
}

func renderComponentInstance(instance *componentInstance) (rendered Node) {
	previous := currentComponent
	currentComponent = instance
	instance.stateIndex = 0
	instance.effectIndex = 0
	instance.unmountIndex = 0
	instance.contextIndex = 0
	instance.providedContexts = instance.providedContexts[:0]
	clearComponentDirty(instance)
	defer func() {
		currentComponent = previous
		if recovered := recover(); recovered != nil {
			if isRuntimeInvariantPanic(recovered) {
				panic(recovered)
			}
			finishComponentContextRender(instance)
			cancelPendingEffectsForRenderFailure(instance)
			info := ErrorInfo{
				Phase:     ErrorPhaseRender,
				Component: runtimeComponentName(instance),
				Operation: "component render",
				Panic:     recovered,
			}
			reportRuntimeError(info)
			captureRenderErrorBoundary(instance, info)
			rendered = Empty()
		}
	}()

	reportComponentRender(instance.name)
	node := instance.node.render()
	finishComponentContextRender(instance)
	return Child(node)
}

func markComponentDirty(instance *componentInstance) {
	if instance == nil || !instance.active {
		return
	}
	if !instance.dirtyCounted {
		for ancestor := instance.parent; ancestor != nil; ancestor = ancestor.parent {
			ancestor.dirtyDescendants++
		}
		instance.dirtyCounted = true
	}
	instance.dirty = true
	if instance.scheduleUpdate != nil {
		instance.scheduleUpdate(instance)
	}
}

func clearComponentDirty(instance *componentInstance) {
	if instance == nil {
		return
	}
	if instance.dirtyCounted {
		for ancestor := instance.parent; ancestor != nil; ancestor = ancestor.parent {
			if ancestor.dirtyDescendants > 0 {
				ancestor.dirtyDescendants--
			}
		}
		instance.dirtyCounted = false
	}
	instance.dirty = false
}

func pruneDirtyComponents(dirty []*componentInstance) []*componentInstance {
	candidates := make(map[*componentInstance]bool, len(dirty))
	for _, instance := range dirty {
		if instance != nil && instance.active && instance.dirty {
			candidates[instance] = true
		}
	}

	pruned := make([]*componentInstance, 0, len(candidates))
	seen := make(map[*componentInstance]bool, len(candidates))
	for _, instance := range dirty {
		if !candidates[instance] || seen[instance] {
			continue
		}
		seen[instance] = true
		if hasDirtyAncestor(instance, candidates) {
			continue
		}
		pruned = append(pruned, instance)
	}
	return pruned
}

func hasDirtyAncestor(instance *componentInstance, dirty map[*componentInstance]bool) bool {
	for ancestor := instance.parent; ancestor != nil; ancestor = ancestor.parent {
		if dirty[ancestor] {
			return true
		}
	}
	return false
}

func ownerDebugName(owner *componentInstance) string {
	if owner == nil || owner.name == "" {
		return "<root>"
	}
	return "<" + owner.name + ">"
}

func legacyComponentIdentity(name string) componentIdentity {
	return componentIdentity{
		id: name,
	}
}

func typedComponentIdentity(id string) componentIdentity {
	return componentIdentity{
		typed: true,
		id:    id,
	}
}

func nodeComponentIdentity(node ComponentNode) componentIdentity {
	if node.identity.id != "" {
		return node.identity
	}
	return componentIdentity{
		id: node.Name,
	}
}

func deactivateComponent(instance *componentInstance) {
	if instance == nil {
		return
	}
	if instance.active {
		instance.active = false
		runUnmountCleanups(instance)
	} else {
		instance.active = false
	}
	releaseContextSubscriptions(instance)
	releaseContextProviders(instance)
	clearComponentDirty(instance)
	instance.dirtyDescendants = 0
	instance.parent = nil
	instance.update = nil
	instance.stateSlots = nil
	instance.effectSlots = nil
	instance.unmountSlots = nil
	instance.contextSlots = nil
	instance.contextProviders = nil
	instance.providedContexts = nil
	instance.errorBoundary = nil
}

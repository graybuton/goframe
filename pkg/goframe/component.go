package goframe

// ComponentFunc renders one typed component props value.
type ComponentFunc[P any] func(P) Node

type memoizedProps[P any] interface {
	MemoEqual(next P) bool
}

// ComponentNode preserves a function component boundary until the runtime
// creates or reuses its component instance.
type ComponentNode struct {
	Name      string
	Props     any
	render    func() Node
	memoEqual func(any, any) bool
}

func (ComponentNode) isNode() {}

// Component creates a runtime-visible typed function component boundary.
func Component[P any](name string, props P, render ComponentFunc[P]) Node {
	var memoEqual func(any, any) bool
	if _, ok := any(props).(memoizedProps[P]); ok {
		memoEqual = memoizeProps[P]
	}
	return ComponentNode{
		Name:      name,
		Props:     props,
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
	if instance == nil || instance.node.Name != nextNode.Name {
		return false
	}
	if instance.key != nextKey {
		return false
	}
	if instance.memoEqual == nil || instance.dirty || instance.dirtyDescendants > 0 || !instance.active {
		return false
	}
	return instance.memoEqual(instance.node.Props, nextNode.Props)
}

func renderComponentInstance(instance *componentInstance) Node {
	previous := currentComponent
	currentComponent = instance
	instance.stateIndex = 0
	instance.effectIndex = 0
	instance.unmountIndex = 0
	clearComponentDirty(instance)
	defer func() {
		currentComponent = previous
	}()

	reportComponentRender(instance.name)
	node := instance.node.render()
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
	clearComponentDirty(instance)
	instance.dirtyDescendants = 0
	instance.parent = nil
	instance.update = nil
	instance.stateSlots = nil
	instance.effectSlots = nil
	instance.unmountSlots = nil
}

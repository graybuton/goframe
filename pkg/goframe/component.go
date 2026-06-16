package goframe

// ComponentFunc renders one typed component props value.
type ComponentFunc[P any] func(P) Node

// ComponentNode preserves a function component boundary until the runtime
// creates or reuses its component instance.
type ComponentNode struct {
	Name   string
	Props  any
	render func() Node
}

func (ComponentNode) isNode() {}

// Component creates a runtime-visible typed function component boundary.
func Component[P any](name string, props P, render ComponentFunc[P]) Node {
	return ComponentNode{
		Name:  name,
		Props: props,
		render: func() Node {
			return render(props)
		},
	}
}

// C is the short form of Component.
func C[P any](name string, props P, render ComponentFunc[P]) Node {
	return Component(name, props, render)
}

type componentInstance struct {
	name           string
	key            string
	parent         *componentInstance
	node           ComponentNode
	stateSlots     []*stateSlot
	stateIndex     int
	dirty          bool
	active         bool
	scheduleUpdate func(*componentInstance)
	update         func()
}

var currentComponent *componentInstance

func newComponentInstance(node ComponentNode, key string, parent *componentInstance, schedule func(*componentInstance)) *componentInstance {
	return &componentInstance{
		name:           node.Name,
		key:            key,
		parent:         parent,
		node:           node,
		dirty:          true,
		active:         true,
		scheduleUpdate: schedule,
	}
}

func renderComponentInstance(instance *componentInstance) Node {
	previous := currentComponent
	currentComponent = instance
	instance.stateIndex = 0
	instance.dirty = false
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
	instance.dirty = true
	if instance.scheduleUpdate != nil {
		instance.scheduleUpdate(instance)
	}
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
	instance.active = false
	instance.dirty = false
	instance.parent = nil
	instance.update = nil
	instance.stateSlots = nil
}

//go:build js && wasm

package goframe

import "syscall/js"

type mountedEvent struct {
	handler   js.Func
	callback  any
	component string
	eventName string
}

// mountedNode retains the virtual node, its DOM range, children, and event
// resources between render cycles. first and last are equal for single-node
// values; fragments span their anchor comments.
type mountedNode struct {
	node           Node
	key            string
	first          js.Value
	last           js.Value
	pending        js.Value
	children       []*mountedNode
	events         map[string]*mountedEvent
	component      *componentInstance
	componentChild *mountedNode
}

func mountNode(document js.Value, node Node, owner *componentInstance) *mountedNode {
	key, node := unwrapNode(node)
	mounted := &mountedNode{
		node:    node,
		key:     key,
		pending: js.Undefined(),
	}

	switch node := node.(type) {
	case VNode:
		element := document.Call("createElement", node.Tag)
		mounted.first = element
		mounted.last = element
		mounted.pending = element
		patchProps(element, mounted, nil, node.Props, owner)
		mounted.children = mountChildren(document, element, node.Children, js.Null(), owner)
		applyPostMountProps(element, node)
	case TextNode:
		text := document.Call("createTextNode", node.Value)
		mounted.first = text
		mounted.last = text
		mounted.pending = text
	case FragmentNode:
		start := document.Call("createComment", "goframe-fragment")
		end := document.Call("createComment", "/goframe-fragment")
		fragment := document.Call("createDocumentFragment")
		fragment.Call("appendChild", start)
		mounted.children = mountChildren(document, fragment, node.Children, js.Null(), owner)
		fragment.Call("appendChild", end)
		mounted.first = start
		mounted.last = end
		mounted.pending = fragment
	case EmptyNode:
		comment := document.Call("createComment", "goframe-empty")
		mounted.first = comment
		mounted.last = comment
		mounted.pending = comment
	case ComponentNode:
		mountComponent(document, mounted, node, owner)
	default:
		panic("goframe: unsupported node type")
	}
	return mounted
}

func applyPostMountProps(element js.Value, node VNode) {
	if node.Tag != "select" || len(node.Props) == 0 {
		return
	}
	props, _ := splitProps(node.Props)
	if prop, ok := props.get("value"); ok {
		setDOMProp(element, "value", prop)
	}
}

func mountChildren(document, parent js.Value, nodes []Node, boundary js.Value, owner *componentInstance) []*mountedNode {
	reportDuplicateSiblingNodeKeys(nodes, ownerDebugName(owner))
	children := make([]*mountedNode, 0, len(nodes))
	for _, node := range nodes {
		child := mountNode(document, node, owner)
		placeMountedBefore(parent, child, boundary)
		children = append(children, child)
	}
	return children
}

func patchMounted(document, parent js.Value, mounted *mountedNode, newNode Node, owner *componentInstance) *mountedNode {
	newKey, newNode := unwrapNode(newNode)
	if mounted.key != newKey || !sameNodeIdentity(mounted.node, newNode) {
		replacement := mountNode(document, Key(newKey, newNode), owner)
		placeMountedBefore(parent, replacement, mounted.first)
		removeMounted(parent, mounted)
		return replacement
	}

	switch oldNode := mounted.node.(type) {
	case VNode:
		newNode := newNode.(VNode)
		patchProps(mounted.first, mounted, oldNode.Props, newNode.Props, owner)
		mounted.children = patchChildren(document, mounted.first, mounted.children, newNode.Children, js.Null(), owner)
	case TextNode:
		newNode := newNode.(TextNode)
		if oldNode.Value != newNode.Value {
			mounted.first.Set("nodeValue", newNode.Value)
		}
	case FragmentNode:
		newNode := newNode.(FragmentNode)
		mounted.children = patchChildren(document, parent, mounted.children, newNode.Children, mounted.last, owner)
	case EmptyNode:
	case ComponentNode:
		patchComponent(document, parent, mounted, newNode.(ComponentNode), owner)
	}
	mounted.node = newNode
	mounted.key = newKey
	return mounted
}

func mountComponent(document js.Value, mounted *mountedNode, node ComponentNode, owner *componentInstance) {
	instance := newComponentInstance(node, mounted.key, owner, queueDirtyComponent)
	mounted.component = instance

	start := document.Call("createComment", "goframe-component")
	end := document.Call("createComment", "/goframe-component")
	fragment := document.Call("createDocumentFragment")
	fragment.Call("appendChild", start)
	child := mountNode(document, renderComponentInstance(instance), instance)
	placeMountedBefore(fragment, child, js.Null())
	fragment.Call("appendChild", end)
	mounted.componentChild = child
	mounted.first = start
	mounted.last = end
	mounted.pending = fragment

	instance.update = func() {
		if !instance.active || !instance.dirty {
			return
		}
		parent := mounted.first.Get("parentNode")
		if parent.IsUndefined() || parent.IsNull() {
			return
		}
		patchComponent(document, parent, mounted, instance.node, instance.parent)
	}
}

func patchComponent(document, parent js.Value, mounted *mountedNode, newNode ComponentNode, owner *componentInstance) {
	instance := mounted.component
	reportComponentPatch(instance.name)
	instance.parent = owner
	if shouldSkipComponentRender(instance, newNode, mounted.key) {
		instance.node = newNode
		instance.dirty = false
		reportComponentMemoSkip(instance.name)
		return
	}
	instance.node = newNode
	child := patchMounted(document, parent, mounted.componentChild, renderComponentInstance(instance), instance)
	mounted.componentChild = child
}

func patchChildren(document, parent js.Value, oldChildren []*mountedNode, newNodes []Node, boundary js.Value, owner *componentInstance) []*mountedNode {
	oldKeys := make([]string, len(oldChildren))
	for index, child := range oldChildren {
		oldKeys[index] = child.key
	}
	newKeys := make([]string, len(newNodes))
	for index, node := range newNodes {
		newKeys[index], _ = unwrapNode(node)
	}
	reportDuplicateSiblingKeys(newKeys, ownerDebugName(owner))

	matches := matchChildIndices(oldKeys, newKeys)
	used := make([]bool, len(oldChildren))
	children := make([]*mountedNode, len(newNodes))
	for index, node := range newNodes {
		oldIndex := matches[index]
		if oldIndex == noChildMatch {
			children[index] = mountNode(document, node, owner)
			continue
		}
		used[oldIndex] = true
		children[index] = patchMounted(document, parent, oldChildren[oldIndex], node, owner)
	}
	for index, child := range oldChildren {
		if !used[index] {
			removeMounted(parent, child)
		}
	}

	stablePlacements := stableChildPlacements(matches, newKeys)
	reference := boundary
	for index := len(children) - 1; index >= 0; index-- {
		if stablePlacements != nil && stablePlacements[index] {
			reference = children[index].first
			continue
		}
		placeMountedBefore(parent, children[index], reference)
		reference = children[index].first
	}
	return children
}

func placeMountedBefore(parent js.Value, mounted *mountedNode, before js.Value) {
	if mounted.pending.Type() != js.TypeUndefined {
		parent.Call("insertBefore", mounted.pending, before)
		mounted.pending = js.Undefined()
		return
	}
	if mounted.last.Get("nextSibling").Equal(before) {
		return
	}

	current := mounted.first
	for {
		next := current.Get("nextSibling")
		parent.Call("insertBefore", current, before)
		if current.Equal(mounted.last) {
			return
		}
		current = next
	}
}

func removeMounted(parent js.Value, mounted *mountedNode) {
	releaseMounted(mounted)
	current := mounted.first
	for {
		next := current.Get("nextSibling")
		parent.Call("removeChild", current)
		if current.Equal(mounted.last) {
			return
		}
		current = next
	}
}

func releaseMounted(mounted *mountedNode) {
	if mounted.component != nil {
		releaseMounted(mounted.componentChild)
		deactivateComponent(mounted.component)
		mounted.component = nil
		mounted.componentChild = nil
		return
	}
	for eventName, event := range mounted.events {
		mounted.first.Call("removeEventListener", eventName, event.handler)
		event.handler.Release()
	}
	mounted.events = nil
	for _, child := range mounted.children {
		releaseMounted(child)
	}
}

func patchProps(element js.Value, mounted *mountedNode, oldProps, newProps Props, owner *componentInstance) {
	oldDOM, _ := splitProps(oldProps)
	newDOM, newEvents := splitProps(newProps)

	for _, oldProp := range oldDOM {
		if !newDOM.has(oldProp.name) {
			removeDOMProp(element, oldProp.name, oldProp.prop)
		}
	}
	for _, newProp := range newDOM {
		if oldProp, exists := oldDOM.get(newProp.name); exists && oldProp == newProp.prop {
			continue
		}
		setDOMProp(element, newProp.name, newProp.prop)
	}
	patchEvents(element, mounted, newEvents, owner)
}

func removeDOMProp(element js.Value, name string, prop domProp) {
	switch name {
	case "value":
		element.Set("value", "")
	case "checked", "selected", "disabled":
		element.Set(name, false)
	}
	element.Call("removeAttribute", name)
}

func setDOMProp(element js.Value, name string, prop domProp) {
	switch name {
	case "value":
		current := element.Get("value")
		if current.Type() != js.TypeString || current.String() != prop.value {
			element.Set("value", prop.value)
		}
		return
	case "checked", "selected", "disabled":
		element.Set(name, prop.boolean)
	}
	if prop.boolean {
		element.Call("setAttribute", name, "")
		return
	}
	element.Call("setAttribute", name, prop.value)
}

func patchEvents(element js.Value, mounted *mountedNode, callbacks splitEventProps, owner *componentInstance) {
	for eventName, event := range mounted.events {
		if _, exists := callbacks.get(eventName); exists {
			continue
		}
		element.Call("removeEventListener", eventName, event.handler)
		event.handler.Release()
		delete(mounted.events, eventName)
	}
	if len(callbacks) == 0 {
		mounted.events = nil
		return
	}

	if mounted.events == nil {
		mounted.events = make(map[string]*mountedEvent, len(callbacks))
	}
	for _, callback := range callbacks {
		eventName := callback.name
		if event, exists := mounted.events[eventName]; exists {
			event.callback = callback.callback
			event.component = runtimeComponentName(owner)
			event.eventName = eventName
			continue
		}
		event := &mountedEvent{
			callback:  callback.callback,
			component: runtimeComponentName(owner),
			eventName: eventName,
		}
		event.handler = eventHandler(event)
		element.Call("addEventListener", eventName, event.handler)
		mounted.events[eventName] = event
	}
}

func eventHandler(listener *mountedEvent) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		defer func() {
			if recovered := recover(); recovered != nil {
				reportRecoveredRuntimeError(ErrorInfo{
					Phase:     ErrorPhaseEvent,
					Component: listener.component,
					Operation: listener.eventName,
				}, recovered)
			}
		}()
		rawEvent := js.Undefined()
		if len(args) > 0 {
			rawEvent = args[0]
		}
		wrappedEvent := wrapEvent(rawEvent)
		inputEvent := wrapInputEvent(rawEvent, wrappedEvent)
		scrollEvent := wrapScrollEvent(rawEvent, wrappedEvent)
		switch callback := listener.callback.(type) {
		case func():
			callback()
		case func(Event):
			callback(wrappedEvent)
		case func(InputEvent):
			callback(inputEvent)
		case func(ScrollEvent):
			callback(scrollEvent)
		case func(js.Value):
			callback(rawEvent)
		default:
			panic("goframe: event handler must be func(), func(goframe.Event), func(goframe.InputEvent), func(goframe.ScrollEvent), or func(js.Value)")
		}
		return nil
	})
}

func wrapEvent(value js.Value) Event {
	if value.IsUndefined() || value.IsNull() {
		return Event{}
	}
	return Event{
		preventDefault: func() {
			value.Call("preventDefault")
		},
		stopPropagation: func() {
			value.Call("stopPropagation")
		},
	}
}

func wrapInputEvent(value js.Value, event Event) InputEvent {
	return InputEvent{
		Event: event,
		value: func() string {
			if value.IsUndefined() || value.IsNull() {
				return ""
			}
			target := value.Get("target")
			if target.IsUndefined() || target.IsNull() {
				return ""
			}
			current := target.Get("value")
			if current.IsUndefined() || current.IsNull() {
				return ""
			}
			return current.String()
		},
	}
}

func wrapScrollEvent(value js.Value, event Event) ScrollEvent {
	return ScrollEvent{
		Event: event,
		scrollTop: func() int {
			if value.IsUndefined() || value.IsNull() {
				return 0
			}
			target := value.Get("target")
			if target.IsUndefined() || target.IsNull() {
				return 0
			}
			current := target.Get("scrollTop")
			if current.IsUndefined() || current.IsNull() {
				return 0
			}
			return current.Int()
		},
	}
}

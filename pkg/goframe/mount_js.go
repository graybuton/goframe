//go:build js && wasm

package goframe

import "syscall/js"

type rootProps struct{}

const maxEffectUpdateLoop = 60

var mountedApp struct {
	root                  js.Value
	app                   func() Node
	tree                  *mountedNode
	rendering             bool
	batch                 updateBatch
	dirty                 []*componentInstance
	dirtySet              map[*componentInstance]bool
	effectScheduledUpdate bool
	effectLoopCount       int
}

// Mount creates the root component instance and connects component-owned state
// updates to batched dirty-subtree patches. The MVP supports one application.
func Mount(rootID string, app func() Node) {
	document := js.Global().Get("document")
	if document.IsUndefined() || document.IsNull() {
		panic("goframe: document is not available")
	}

	root := document.Call("getElementById", rootID)
	if root.IsUndefined() || root.IsNull() {
		panic("goframe: root element not found: " + rootID)
	}
	if mountedApp.tree != nil {
		releaseMounted(mountedApp.tree)
	}

	mountedApp.root = root
	mountedApp.app = app
	mountedApp.tree = nil
	mountedApp.rendering = false
	mountedApp.batch.reset()
	mountedApp.dirty = nil
	mountedApp.dirtySet = make(map[*componentInstance]bool)
	mountedApp.effectScheduledUpdate = false
	mountedApp.effectLoopCount = 0
	pendingEffects = nil

	mountApplication(document)
}

func mountApplication(document js.Value) {
	started := performanceNow()
	mountedApp.rendering = true

	rootNode := Component("App", rootProps{}, func(rootProps) Node {
		return mountedApp.app()
	})
	mountedApp.root.Set("textContent", "")
	mountedApp.tree = mountNode(document, rootNode, nil)
	placeMountedBefore(mountedApp.root, mountedApp.tree, js.Null())
	mountedApp.rendering = false
	reportRender("first-render", performanceNow()-started)
	flushPendingEffects()
	checkEffectUpdateLoop()
}

func queueDirtyComponent(instance *componentInstance) {
	if instance == nil || !instance.active || mountedApp.dirtySet[instance] {
		return
	}
	if flushingEffects {
		mountedApp.effectScheduledUpdate = true
	}
	mountedApp.dirtySet[instance] = true
	mountedApp.dirty = append(mountedApp.dirty, instance)
	scheduleRender()
}

func scheduleRender() {
	mountedApp.batch.request(enqueueBrowserUpdate, flushDirtyComponents)
}

func enqueueBrowserUpdate(update func()) {
	var callback js.Func
	callback = js.FuncOf(func(this js.Value, args []js.Value) any {
		callback.Release()
		update()
		return nil
	})
	requestAnimationFrame := js.Global().Get("requestAnimationFrame")
	if requestAnimationFrame.Type() == js.TypeFunction {
		requestAnimationFrame.Invoke(callback)
		return
	}
	js.Global().Call("queueMicrotask", callback)
}

func flushDirtyComponents() {
	if mountedApp.rendering {
		scheduleRender()
		return
	}

	started := performanceNow()
	mountedApp.rendering = true

	dirty := pruneDirtyComponents(mountedApp.dirty)
	mountedApp.dirty = nil
	mountedApp.dirtySet = make(map[*componentInstance]bool)

	document := js.Global().Get("document")
	focus := captureFocus(document)
	for _, instance := range dirty {
		if instance.active && instance.dirty && instance.update != nil {
			instance.update()
		}
	}
	restoreFocus(document, focus)
	mountedApp.rendering = false
	reportRender("update", performanceNow()-started)
	flushPendingEffects()
	checkEffectUpdateLoop()
}

func checkEffectUpdateLoop() {
	if !mountedApp.effectScheduledUpdate {
		mountedApp.effectLoopCount = 0
		return
	}
	mountedApp.effectScheduledUpdate = false
	mountedApp.effectLoopCount++
	if mountedApp.effectLoopCount <= maxEffectUpdateLoop {
		return
	}
	reportEffectUpdateLoopGuard()
	if !shouldStopEffectUpdateLoop() {
		return
	}
	mountedApp.dirty = nil
	mountedApp.dirtySet = make(map[*componentInstance]bool)
	mountedApp.batch.reset()
	mountedApp.effectLoopCount = 0
}

type focusSnapshot struct {
	id             string
	selectionStart int
	selectionEnd   int
	hasSelection   bool
}

func captureFocus(document js.Value) focusSnapshot {
	active := document.Get("activeElement")
	if active.IsUndefined() || active.IsNull() || !mountedApp.root.Call("contains", active).Bool() {
		return focusSnapshot{}
	}
	id := active.Get("id")
	if id.Type() != js.TypeString || id.String() == "" {
		return focusSnapshot{}
	}

	snapshot := focusSnapshot{id: id.String()}
	start := active.Get("selectionStart")
	end := active.Get("selectionEnd")
	if start.Type() == js.TypeNumber && end.Type() == js.TypeNumber {
		snapshot.selectionStart = start.Int()
		snapshot.selectionEnd = end.Int()
		snapshot.hasSelection = true
	}
	return snapshot
}

func restoreFocus(document js.Value, snapshot focusSnapshot) {
	if snapshot.id == "" {
		return
	}
	element := document.Call("getElementById", snapshot.id)
	if element.IsUndefined() || element.IsNull() || !mountedApp.root.Call("contains", element).Bool() {
		return
	}
	active := document.Get("activeElement")
	if !active.IsUndefined() && !active.IsNull() && active.Equal(element) {
		return
	}
	element.Call("focus")
	if snapshot.hasSelection && element.Get("setSelectionRange").Type() == js.TypeFunction {
		element.Call("setSelectionRange", snapshot.selectionStart, snapshot.selectionEnd)
	}
}

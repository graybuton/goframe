//go:build js && wasm && !goframe_document_state_experiment

package goframe

import "syscall/js"

func mountApplicationUpdate(
	root js.Value,
	app func() Node,
	document js.Value,
) {
	started := renderMountedApplication(root, app, document)
	finishBrowserRender("first-render", started)
}

func flushDirtyComponentsApplicationUpdate() {
	started := renderDirtyComponents()
	finishBrowserRender("update", started)
}

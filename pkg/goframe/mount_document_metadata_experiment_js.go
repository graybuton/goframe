//go:build js && wasm && goframe_document_state_experiment

package goframe

import "syscall/js"

func mountApplicationUpdate(
	root js.Value,
	app func() Node,
	document js.Value,
) {
	coordinator := activeDocumentMetadataCoordinator
	if coordinator != nil {
		coordinator.beginUpdate()
	}
	committed := false
	defer func() {
		if coordinator != nil && !committed {
			coordinator.rollbackUpdate()
		}
	}()

	started := renderMountedApplication(root, app, document)
	if coordinator != nil {
		coordinator.commitUpdate()
	}
	committed = true
	finishBrowserRender("first-render", started)
}

func flushDirtyComponentsApplicationUpdate() {
	coordinator := activeDocumentMetadataCoordinator
	if coordinator != nil {
		coordinator.beginUpdate()
	}
	committed := false
	defer func() {
		if coordinator != nil && !committed {
			coordinator.rollbackUpdate()
		}
	}()

	started := renderDirtyComponents()
	if coordinator != nil {
		coordinator.commitUpdate()
	}
	committed = true
	finishBrowserRender("update", started)
}

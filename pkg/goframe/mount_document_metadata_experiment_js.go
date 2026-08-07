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
	defer func() {
		if coordinator != nil && coordinator.batch.active {
			coordinator.rollbackUpdate()
		}
	}()

	started := renderMountedApplication(root, app, document)
	if coordinator != nil {
		commitDocumentMetadataApplicationUpdate(coordinator)
	}
	finishBrowserRender("first-render", started)
}

func flushDirtyComponentsApplicationUpdate() {
	coordinator := activeDocumentMetadataCoordinator
	if coordinator != nil {
		coordinator.beginUpdate()
	}
	defer func() {
		if coordinator != nil && coordinator.batch.active {
			coordinator.rollbackUpdate()
		}
	}()

	started := renderDirtyComponents()
	if coordinator != nil {
		commitDocumentMetadataApplicationUpdate(coordinator)
	}
	finishBrowserRender("update", started)
}

func commitDocumentMetadataApplicationUpdate(
	coordinator *documentMetadataCoordinator,
) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}
		documentError, ok := recovered.(*documentMetadataWrappedError)
		if !ok {
			panic(recovered)
		}
		reportRuntimeError(ErrorInfo{
			Phase:     ErrorPhaseRender,
			Operation: "document metadata transaction",
			Panic:     documentError.Error(),
		})
	}()
	coordinator.commitUpdate()
}

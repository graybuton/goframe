//go:build js && wasm && goframe_document_state_experiment

package sizefixture

import (
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

var value = gf.DocumentMetadata{
	Title:       "Candidate owner · GoFrame",
	Description: "Matched API-shape size fixture",
}

// Run installs the shared private foundation and mounts one matched UI.
func Run(render func() gf.Node) {
	title := js.Global().Get("document").Call("querySelector", "head title")
	description := js.Global().Get("document").Call(
		"querySelector",
		`head meta[name="description"]`,
	)
	baseline := gf.DocumentMetadataHandoffExperimentValue{
		Title:       title.Get("textContent").String(),
		Description: description.Call("getAttribute", "content").String(),
	}
	gf.InstallDocumentMetadataHandoffExperiment(
		baseline,
		func(next gf.DocumentMetadataHandoffExperimentValue) {
			title.Set("textContent", next.Title)
			description.Call("setAttribute", "content", next.Description)
		},
		nil,
	)
	gf.Mount("root", render)
	select {}
}

// Metadata returns the matched fixture-only candidate value.
func Metadata() gf.DocumentMetadata {
	return value
}

// HandoffMetadata returns the same value for the direct foundation control.
func HandoffMetadata() gf.DocumentMetadataHandoffExperimentValue {
	return gf.DocumentMetadataHandoffExperimentValue{
		Title:       value.Title,
		Description: value.Description,
	}
}

// Content is the identical host tree used by every size entry point.
func Content() gf.Node {
	return gf.El(
		"main",
		gf.Props{"data-testid": "api-shape-size"},
		gf.El("h1", nil, gf.Text("Document metadata API shape")),
		gf.El("p", nil, gf.Text("Matched candidate projection")),
	)
}

//go:build js && wasm && goframe_document_state_experiment

package main

import (
	gf "github.com/graybuton/goframe/pkg/goframe"
	"github.com/graybuton/goframe/scripts/fixtures/document-metadata-api-shape-v2/internal/sizefixture"
)

func main() {
	sizefixture.Run(func() gf.Node {
		gf.UseDocumentMetadataHandoffExperiment(sizefixture.HandoffMetadata())
		return sizefixture.Content()
	})
}

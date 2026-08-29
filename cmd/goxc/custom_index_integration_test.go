package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomIndexDevelopmentInjectionUsesCanonicalPackageHTML(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.html")
	packagePath := filepath.Join(root, "package", indexHTMLAssetName)
	source := `<!doctype html><html><head></head><body data-example="bundle.wasm">
<p>authored bundle.wasm</p>
<script src="wasm_exec.js"></script>
<script>
const go = new Go();
WebAssembly.instantiateStreaming(fetch("bundle.wasm"), go.importObject)
    .then((result) => go.run(result.instance));
</script>
<script>const documentation = "bundle.wasm";</script>
</body></html>`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(packagePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeRewrittenIndex(sourcePath, packagePath, htmlRewriteOptions{
		wasmPath:    "assets/bundle.12345678.wasm",
		runtimePath: "assets/wasm_exec.87654321.js",
	}); err != nil {
		t.Fatalf("writeRewrittenIndex() error: %v", err)
	}

	canonicalBytes, err := os.ReadFile(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	canonical := string(canonicalBytes)
	for _, want := range []string{
		`data-example="bundle.wasm"`,
		`<p>authored bundle.wasm</p>`,
		`const documentation = "bundle.wasm";`,
		`src="assets/wasm_exec.87654321.js"`,
		`fetch("assets/bundle.12345678.wasm")`,
	} {
		if !strings.Contains(canonical, want) {
			t.Errorf("canonical package index missing %q:\n%s", want, canonical)
		}
	}
	if strings.Contains(canonical, devReloadMarker) {
		t.Fatalf("canonical package index contains development marker:\n%s", canonical)
	}

	devPackage := t.TempDir()
	writeDevGenerationPackage(t, devPackage, canonical)
	manager := newTestDevGenerationManager(t)
	generation, err := manager.activatePackage(devPackage)
	if err != nil {
		t.Fatal(err)
	}
	broker := newDevReloadBroker(testDevReloadInstance)
	broker.activate(generation, false)
	response := httptest.NewRecorder()
	devReloadHandler(manager, broker).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/", nil),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("development index response status = %d, want 200", response.Code)
	}
	served := response.Body.String()
	if strings.Count(served, devReloadMarker) != 1 {
		t.Fatalf("served development index marker count = %d, want 1", strings.Count(served, devReloadMarker))
	}
	for _, sentinel := range []string{
		`data-example="bundle.wasm"`,
		`<p>authored bundle.wasm</p>`,
		`const documentation = "bundle.wasm";`,
	} {
		if !strings.Contains(served, sentinel) {
			t.Errorf("development injection changed authored sentinel %q:\n%s", sentinel, served)
		}
	}
	assertFileContent(t, packagePath, canonical)
	assertFileContent(t, filepath.Join(devPackage, indexHTMLAssetName), canonical)
}

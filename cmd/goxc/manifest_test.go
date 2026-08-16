package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestWASMExtensionUsesASCIIOnlyCaseFolding(t *testing.T) {
	for _, test := range []struct {
		name   string
		wasm   string
		accept bool
	}{
		{name: "lowercase", wasm: "bundle.wasm", accept: true},
		{name: "uppercase", wasm: "APP.WASM", accept: true},
		{name: "mixed ASCII case", wasm: "App.WaSm", accept: true},
		{name: "Unicode long s", wasm: "app.waſm", accept: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			appDir := t.TempDir()
			content := fmt.Sprintf(`{"wasm":%q}`, test.wasm)
			if err := os.WriteFile(filepath.Join(appDir, manifestName), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			manifest, err := loadManifest(appDir)
			if test.accept {
				if err != nil {
					t.Fatalf("loadManifest(%q) error: %v", test.wasm, err)
				}
				if manifest.WASM != test.wasm {
					t.Fatalf("WASM = %q, want %q", manifest.WASM, test.wasm)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "must end with .wasm") {
				t.Fatalf("loadManifest(%q) error = %v, want ASCII-only .wasm rejection", test.wasm, err)
			}
		})
	}
}

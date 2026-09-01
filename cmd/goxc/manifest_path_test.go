package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadManifestCanonicalizesAuthoredPathFields(t *testing.T) {
	for _, test := range []struct {
		name   string
		entry  string
		output string
		wasm   string
		assets any
		want   projectManifest
	}{
		{
			name:   "canonical slash",
			entry:  "cmd/app",
			output: "dist/output",
			wasm:   "nested/bundle.wasm",
			assets: []string{"assets/styles/app.css"},
			want: projectManifest{
				Entry:  "cmd/app",
				Output: "dist/output",
				WASM:   "nested/bundle.wasm",
				Assets: listManifestAssets([]string{"assets/styles/app.css"}),
			},
		},
		{
			name:   "leading dot slash",
			entry:  "./cmd/app",
			output: "./dist/output",
			wasm:   "./nested/bundle.wasm",
			assets: []string{"./assets/styles/app.css"},
			want: projectManifest{
				Entry:  "cmd/app",
				Output: "dist/output",
				WASM:   "nested/bundle.wasm",
				Assets: listManifestAssets([]string{"assets/styles/app.css"}),
			},
		},
		{
			name:   "backslash separators",
			entry:  `cmd\app`,
			output: `dist\output`,
			wasm:   `nested\bundle.wasm`,
			assets: []string{`assets\styles\app.css`},
			want: projectManifest{
				Entry:  "cmd/app",
				Output: "dist/output",
				WASM:   "nested/bundle.wasm",
				Assets: listManifestAssets([]string{"assets/styles/app.css"}),
			},
		},
		{
			name:   "leading dot backslash",
			entry:  `.\cmd\app`,
			output: `.\dist\output`,
			wasm:   `.\nested\bundle.wasm`,
			assets: `.\assets\styles`,
			want: projectManifest{
				Entry:  "cmd/app",
				Output: "dist/output",
				WASM:   "nested/bundle.wasm",
				Assets: directoryManifestAssets("assets/styles"),
			},
		},
		{
			name:   "mixed and repeated separators",
			entry:  `cmd\nested//./app`,
			output: `dist//nested\output`,
			wasm:   `nested\more//bundle.wasm`,
			assets: []string{`assets\styles//./app.css`},
			want: projectManifest{
				Entry:  "cmd/nested/app",
				Output: "dist/nested/output",
				WASM:   "nested/more/bundle.wasm",
				Assets: listManifestAssets([]string{"assets/styles/app.css"}),
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			appDir := t.TempDir()
			writeManifestPathFixture(t, appDir, map[string]any{
				"entry":  test.entry,
				"output": test.output,
				"wasm":   test.wasm,
				"assets": test.assets,
			})

			manifest, err := loadManifest(appDir)
			if err != nil {
				t.Fatalf("loadManifest() error: %v", err)
			}
			if manifest.Entry != test.want.Entry ||
				manifest.Output != test.want.Output ||
				manifest.WASM != test.want.WASM ||
				!reflect.DeepEqual(manifest.Assets, test.want.Assets) {
				t.Fatalf(
					"canonical paths = entry:%q output:%q wasm:%q assets:%+v; want entry:%q output:%q wasm:%q assets:%+v",
					manifest.Entry,
					manifest.Output,
					manifest.WASM,
					manifest.Assets,
					test.want.Entry,
					test.want.Output,
					test.want.WASM,
					test.want.Assets,
				)
			}
		})
	}
}

func TestLoadManifestCanonicalPathDefaults(t *testing.T) {
	appDir := t.TempDir()
	manifest, err := loadManifest(appDir)
	if err != nil {
		t.Fatalf("loadManifest() error: %v", err)
	}
	if manifest.Entry != "." || manifest.Output != "dist" || manifest.WASM != "bundle.wasm" {
		t.Fatalf("defaults = entry:%q output:%q wasm:%q", manifest.Entry, manifest.Output, manifest.WASM)
	}
	if manifest.Assets.Mode != manifestAssetsAuto {
		t.Fatalf("assets mode = %v, want auto", manifest.Assets.Mode)
	}
}

func TestLoadManifestRejectsPortableUnsafeAuthoredPaths(t *testing.T) {
	unsafe := []string{
		"..",
		"../a",
		"a/..",
		"a/../b",
		`a\..\b`,
		`a/..\b`,
		"/foo",
		`\foo`,
		"//server/share",
		`\\server\share`,
		"C:",
		"C:foo",
		"C:/foo",
		`C:\foo`,
		"c:",
		"c:foo",
		"c:/foo",
		`c:\foo`,
	}
	fields := []struct {
		name  string
		value func(string) map[string]any
	}{
		{name: "entry", value: func(value string) map[string]any { return map[string]any{"entry": value} }},
		{name: "output", value: func(value string) map[string]any { return map[string]any{"output": value} }},
		{name: "wasm", value: func(value string) map[string]any { return map[string]any{"wasm": value} }},
		{name: "assets directory", value: func(value string) map[string]any { return map[string]any{"assets": value} }},
		{name: "assets list", value: func(value string) map[string]any { return map[string]any{"assets": []string{value}} }},
	}

	for _, field := range fields {
		for _, value := range unsafe {
			t.Run(field.name+"/"+strings.ReplaceAll(value, "/", "_"), func(t *testing.T) {
				appDir := t.TempDir()
				writeManifestPathFixture(t, appDir, field.value(value))
				if _, err := loadManifest(appDir); err == nil {
					t.Fatalf("loadManifest() accepted unsafe %s path %q", field.name, value)
				}
			})
		}
	}
}

func writeManifestPathFixture(t *testing.T, appDir string, values map[string]any) {
	t.Helper()
	content, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, manifestName), content, 0o644); err != nil {
		t.Fatal(err)
	}
}

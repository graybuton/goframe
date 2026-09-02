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

func TestLoadManifestRejectsCanonicalizedDriveLikeAuthoredPaths(t *testing.T) {
	forms := []struct {
		name   string
		prefix string
	}{
		{name: "uppercase slash drive root", prefix: "./C:/"},
		{name: "uppercase slash drive relative", prefix: "./C:"},
		{name: "uppercase backslash drive root", prefix: ".\\C:\\"},
		{name: "uppercase backslash drive relative", prefix: ".\\C:"},
		{name: "lowercase slash drive root", prefix: "./c:/"},
		{name: "lowercase slash drive relative", prefix: "./c:"},
		{name: "lowercase backslash drive root", prefix: ".\\c:\\"},
		{name: "lowercase backslash drive relative", prefix: ".\\c:"},
	}
	fields := []struct {
		name  string
		value func(string) map[string]any
	}{
		{name: "entry", value: func(prefix string) map[string]any { return map[string]any{"entry": prefix + "child"} }},
		{name: "output", value: func(prefix string) map[string]any { return map[string]any{"output": prefix + "child"} }},
		{name: "wasm", value: func(prefix string) map[string]any { return map[string]any{"wasm": prefix + "bundle.wasm"} }},
		{name: "assets directory", value: func(prefix string) map[string]any { return map[string]any{"assets": prefix + "assets"} }},
		{name: "assets list", value: func(prefix string) map[string]any { return map[string]any{"assets": []string{prefix + "style.css"}} }},
	}

	for _, field := range fields {
		for _, form := range forms {
			t.Run(field.name+"/"+form.name, func(t *testing.T) {
				appDir := t.TempDir()
				writeManifestPathFixture(t, appDir, field.value(form.prefix))
				if _, err := loadManifest(appDir); err == nil {
					t.Fatalf("loadManifest() accepted canonicalized drive-like %s path with prefix %q", field.name, form.prefix)
				}
			})
		}
	}
}

func TestLoadManifestCanonicalPathValuesAreFixedPoints(t *testing.T) {
	fields := []struct {
		name       string
		input      string
		want       string
		value      func(string) map[string]any
		storedPath func(projectManifest) string
	}{
		{
			name:       "entry",
			input:      "./nested/file",
			want:       "nested/file",
			value:      func(value string) map[string]any { return map[string]any{"entry": value} },
			storedPath: func(manifest projectManifest) string { return manifest.Entry },
		},
		{
			name:       "output",
			input:      "nested\\file",
			want:       "nested/file",
			value:      func(value string) map[string]any { return map[string]any{"output": value} },
			storedPath: func(manifest projectManifest) string { return manifest.Output },
		},
		{
			name:       "wasm",
			input:      "nested//./bundle.wasm",
			want:       "nested/bundle.wasm",
			value:      func(value string) map[string]any { return map[string]any{"wasm": value} },
			storedPath: func(manifest projectManifest) string { return manifest.WASM },
		},
		{
			name:       "assets directory",
			input:      "nested/mixed\\file",
			want:       "nested/mixed/file",
			value:      func(value string) map[string]any { return map[string]any{"assets": value} },
			storedPath: func(manifest projectManifest) string { return manifest.Assets.Directory },
		},
		{
			name:       "assets list",
			input:      "./nested/style.css",
			want:       "nested/style.css",
			value:      func(value string) map[string]any { return map[string]any{"assets": []string{value}} },
			storedPath: func(manifest projectManifest) string { return manifest.Assets.List[0] },
		},
	}

	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			appDir := t.TempDir()
			writeManifestPathFixture(t, appDir, field.value(field.input))
			first, err := loadManifest(appDir)
			if err != nil {
				t.Fatalf("first loadManifest() error: %v", err)
			}
			stored := field.storedPath(first)
			if stored != field.want {
				t.Fatalf("first canonical value = %q, want %q", stored, field.want)
			}

			writeManifestPathFixture(t, appDir, field.value(stored))
			second, err := loadManifest(appDir)
			if err != nil {
				t.Fatalf("second loadManifest() error: %v", err)
			}
			if got := field.storedPath(second); got != stored {
				t.Fatalf("canonical value after revalidation = %q, want %q", got, stored)
			}
		})
	}
}

func TestCleanAuthoredManifestPathClassifiesOnlyDrivePrefixes(t *testing.T) {
	for _, test := range []struct {
		name      string
		value     string
		want      string
		wantError bool
	}{
		{name: "colon in relative name", value: "name:asset.css", want: "name:asset.css"},
		{name: "colon data in relative name", value: "foo:bar", want: "foo:bar"},
		{name: "uppercase drive relative", value: "C:asset.css", wantError: true},
		{name: "lowercase drive relative", value: "c:asset.css", wantError: true},
		{name: "rooted", value: "/asset.css", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := cleanAuthoredManifestPath(test.value, false)
			if test.wantError {
				if err == nil {
					t.Fatalf("cleanAuthoredManifestPath(%q) = %q, want error", test.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("cleanAuthoredManifestPath(%q) error: %v", test.value, err)
			}
			if got != test.want {
				t.Fatalf("cleanAuthoredManifestPath(%q) = %q, want %q", test.value, got, test.want)
			}
		})
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

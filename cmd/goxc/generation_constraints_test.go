package main

import (
	"encoding/json"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestAuthoredSourceConstraintsMatchBrowserTargets(t *testing.T) {
	t.Setenv("GOWASM", "")
	tests := []struct {
		name       string
		filename   string
		constraint string
		goWant     bool
		tinyWant   bool
	}{
		{name: "plain", filename: "source.go", goWant: true, tinyWant: true},
		{name: "windows suffix", filename: "source_windows.go"},
		{name: "linux suffix", filename: "source_linux.go"},
		{name: "js suffix", filename: "source_js.go", goWant: true, tinyWant: true},
		{name: "wasm suffix", filename: "source_wasm.go", goWant: true, tinyWant: true},
		{name: "js wasm suffix", filename: "source_js_wasm.go", goWant: true, tinyWant: true},
		{name: "test file", filename: "source_test.go"},
		{name: "js and wasm", filename: "source.go", constraint: "js && wasm", goWant: true, tinyWant: true},
		{name: "not js", filename: "source.go", constraint: "!js"},
		{name: "future release", filename: "source.go", constraint: "go1.999"},
		{name: "supported release", filename: "source.go", constraint: "go1.22", goWant: true, tinyWant: true},
		{name: "gc compiler", filename: "source.go", constraint: "gc", goWant: true, tinyWant: true},
		{name: "tinygo compiler", filename: "source.go", constraint: "tinygo", tinyWant: true},
		{name: "not tinygo compiler", filename: "source.go", constraint: "!tinygo", goWant: true},
		{name: "cgo", filename: "source.go", constraint: "cgo", tinyWant: true},
		{name: "tinygo wasm", filename: "source.go", constraint: "tinygo.wasm", tinyWant: true},
		{name: "tinygo gc", filename: "source.go", constraint: "gc.precise", tinyWant: true},
		{name: "tinygo scheduler", filename: "source.go", constraint: "scheduler.asyncify", tinyWant: true},
		{name: "host architecture feature", filename: "source.go", constraint: "amd64.v1"},
		{name: "inactive wasm feature", filename: "source.go", constraint: "wasm.satconv"},
		{name: "legacy windows", filename: "source.go", constraint: "+build windows"},
	}

	for _, compiler := range []struct {
		name string
		want func(test struct {
			name       string
			filename   string
			constraint string
			goWant     bool
			tinyWant   bool
		}) bool
	}{
		{name: "go", want: func(test struct {
			name       string
			filename   string
			constraint string
			goWant     bool
			tinyWant   bool
		}) bool {
			return test.goWant
		}},
		{name: "tinygo", want: func(test struct {
			name       string
			filename   string
			constraint string
			goWant     bool
			tinyWant   bool
		}) bool {
			return test.tinyWant
		}},
	} {
		t.Run(compiler.name, func(t *testing.T) {
			selection, err := browserGenerationSourceSelection(compiler.name)
			if err != nil {
				t.Fatal(err)
			}
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					source := "package app\n"
					if test.constraint != "" {
						prefix := "//go:build "
						if strings.HasPrefix(test.constraint, "+build ") {
							prefix = "// "
						}
						source = prefix + test.constraint + "\n\n" + source
					}
					got, err := selection.matchAuthoredGo(
						filepath.Join(t.TempDir(), "package"),
						test.filename,
						[]byte(source),
					)
					if err != nil {
						t.Fatal(err)
					}
					if got != compiler.want(test) {
						t.Fatalf(
							"match(%q, %q) = %t, want %t",
							test.filename,
							test.constraint,
							got,
							compiler.want(test),
						)
					}
				})
			}
		})
	}
}

func TestBrowserAuthoredSourceSelectionExcludesHostArchitectureFeature(t *testing.T) {
	t.Setenv("GOWASM", "")
	for _, compiler := range []string{"go", "tinygo"} {
		t.Run(compiler, func(t *testing.T) {
			selection, err := browserGenerationSourceSelection(compiler)
			if err != nil {
				t.Fatal(err)
			}
			matched, err := selection.matchAuthoredGo(
				filepath.Join(t.TempDir(), "package"),
				"inactive.go",
				[]byte("//go:build amd64.v1\n\npackage app\n"),
			)
			if err != nil {
				t.Fatal(err)
			}
			if matched {
				t.Fatalf(
					"amd64.v1 source matched the js/wasm browser target with ToolTags %v",
					selection.buildContext.ToolTags,
				)
			}
		})
	}
}

func TestBrowserBuildContextNormalizesTargetToolTags(t *testing.T) {
	firstBase := build.Context{
		GOOS:        "linux",
		GOARCH:      "amd64",
		Compiler:    "gc",
		CgoEnabled:  true,
		BuildTags:   []string{"ambient.second", "ambient.first"},
		ToolTags:    []string{"amd64.v1", "custom.tool", "goexperiment.coverageredesign", "wasm.signext", "custom.tool"},
		ReleaseTags: []string{"go1.1", "go1.22"},
	}
	secondBase := firstBase
	secondBase.BuildTags = []string{"ambient.first", "ambient.second", "ambient.first"}
	secondBase.ToolTags = []string{"wasm.satconv", "goexperiment.coverageredesign", "amd64.v4", "custom.tool"}
	firstBefore := cloneBuildContextTagSlices(firstBase)
	secondBefore := cloneBuildContextTagSlices(secondBase)
	firstEnvironment := []string{"SENTINEL=preserved", "GOWASM=signext,satconv"}
	secondEnvironment := []string{"GOWASM=satconv,signext", "SENTINEL=preserved"}
	firstEnvironmentBefore := append([]string(nil), firstEnvironment...)
	secondEnvironmentBefore := append([]string(nil), secondEnvironment...)

	for _, compiler := range []string{"go", "tinygo"} {
		t.Run(compiler, func(t *testing.T) {
			first, err := browserBuildContext(compiler, firstBase, firstEnvironment)
			if err != nil {
				t.Fatal(err)
			}
			second, err := browserBuildContext(compiler, secondBase, secondEnvironment)
			if err != nil {
				t.Fatal(err)
			}
			wantToolTags := []string{
				"custom.tool",
				"goexperiment.coverageredesign",
				"wasm.satconv",
				"wasm.signext",
			}
			if !reflect.DeepEqual(first.ToolTags, wantToolTags) {
				t.Fatalf("ToolTags = %v, want %v", first.ToolTags, wantToolTags)
			}
			if !reflect.DeepEqual(first.ToolTags, second.ToolTags) {
				t.Fatalf("ToolTags depend on input order:\nfirst:  %v\nsecond: %v", first.ToolTags, second.ToolTags)
			}
			if first.GOOS != "js" || first.GOARCH != "wasm" {
				t.Fatalf("target = %s/%s, want js/wasm", first.GOOS, first.GOARCH)
			}
			if first.Compiler != "gc" {
				t.Fatalf("Compiler = %q, want gc", first.Compiler)
			}
			if compiler == "go" {
				if first.CgoEnabled {
					t.Fatal("standard Go browser context enables cgo")
				}
				if len(first.BuildTags) != 0 {
					t.Fatalf("standard Go BuildTags = %v, want none", first.BuildTags)
				}
			} else {
				if !first.CgoEnabled {
					t.Fatal("TinyGo browser context disables cgo")
				}
				for _, tag := range []string{"tinygo", "tinygo.wasm", "gc.precise"} {
					if !containsString(first.BuildTags, tag) {
						t.Fatalf("TinyGo BuildTags %v do not contain %q", first.BuildTags, tag)
					}
				}
			}
			if !reflect.DeepEqual(first.ReleaseTags, firstBase.ReleaseTags) {
				t.Fatalf("ReleaseTags = %v, want %v", first.ReleaseTags, firstBase.ReleaseTags)
			}
		})
	}

	if !reflect.DeepEqual(firstBase, firstBefore) {
		t.Fatalf("first base context mutated:\nbefore: %#v\nafter:  %#v", firstBefore, firstBase)
	}
	if !reflect.DeepEqual(secondBase, secondBefore) {
		t.Fatalf("second base context mutated:\nbefore: %#v\nafter:  %#v", secondBefore, secondBase)
	}
	if !reflect.DeepEqual(firstEnvironment, firstEnvironmentBefore) {
		t.Fatalf("first environment mutated: %v", firstEnvironment)
	}
	if !reflect.DeepEqual(secondEnvironment, secondEnvironmentBefore) {
		t.Fatalf("second environment mutated: %v", secondEnvironment)
	}
}

func TestBrowserBuildContextRejectsInvalidGOWASM(t *testing.T) {
	_, err := browserBuildContext(
		"go",
		build.Default,
		[]string{"GOWASM=satconv,unknown"},
	)
	if err == nil {
		t.Fatal("browserBuildContext() accepted invalid GOWASM")
	}
	if !strings.Contains(err.Error(), `invalid GOWASM feature "unknown"`) {
		t.Fatalf("error %q does not identify invalid GOWASM feature", err)
	}
}

func TestPackageSourceSelectionUsesObservedCompilerTargets(t *testing.T) {
	t.Setenv("GOWASM", "")
	goSelection, err := browserGenerationSourceSelection("go")
	if err != nil {
		t.Fatal(err)
	}
	tinySelection, err := browserGenerationSourceSelection("tinygo")
	if err != nil {
		t.Fatal(err)
	}

	for name, test := range map[string]struct {
		selection generationSourceSelection
		cgo       bool
		tags      []string
	}{
		"go": {
			selection: goSelection,
		},
		"tinygo": {
			selection: tinySelection,
			cgo:       true,
			tags: []string{
				"tinygo.wasm",
				"tinygo",
				"purego",
				"osusergo",
				"math_big_pure_go",
				"gc.precise",
				"scheduler.asyncify",
				"serial.none",
				"tinygo.unicore",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			context := test.selection.buildContext
			if context == nil {
				t.Fatal("browser selection has no build context")
			}
			if context.GOOS != "js" || context.GOARCH != "wasm" {
				t.Fatalf("target = %s/%s, want js/wasm", context.GOOS, context.GOARCH)
			}
			if context.Compiler != "gc" {
				t.Fatalf("compiler tag = %q, want gc", context.Compiler)
			}
			if context.CgoEnabled != test.cgo {
				t.Fatalf("CgoEnabled = %t, want %t", context.CgoEnabled, test.cgo)
			}
			if !test.selection.excludeGoTests {
				t.Fatal("browser selection does not exclude test files")
			}
			for _, tag := range test.tags {
				if !containsString(context.BuildTags, tag) {
					t.Fatalf("BuildTags %v do not contain %q", context.BuildTags, tag)
				}
			}
			if !containsString(context.ReleaseTags, "go1.22") {
				t.Fatalf("ReleaseTags %v do not contain go1.22", context.ReleaseTags)
			}
		})
	}
}

func TestDefaultGenerationSourceSelectionUsesCurrentBuildContext(t *testing.T) {
	before := cloneBuildContextTagSlices(build.Default)
	selection := defaultGenerationSourceSelection()
	if selection.buildContext == nil {
		t.Fatal("default generation selection has no build context")
	}
	if !selection.excludeGoTests {
		t.Fatal("default generation selection includes authored test files")
	}
	context := selection.buildContext
	if context.GOOS != build.Default.GOOS ||
		context.GOARCH != build.Default.GOARCH ||
		context.Compiler != build.Default.Compiler ||
		context.CgoEnabled != build.Default.CgoEnabled {
		t.Fatalf(
			"default target = %s/%s compiler=%s cgo=%t, want %s/%s compiler=%s cgo=%t",
			context.GOOS,
			context.GOARCH,
			context.Compiler,
			context.CgoEnabled,
			build.Default.GOOS,
			build.Default.GOARCH,
			build.Default.Compiler,
			build.Default.CgoEnabled,
		)
	}
	for name, values := range map[string]struct {
		got  []string
		want []string
	}{
		"BuildTags":   {got: context.BuildTags, want: build.Default.BuildTags},
		"ToolTags":    {got: context.ToolTags, want: build.Default.ToolTags},
		"ReleaseTags": {got: context.ReleaseTags, want: build.Default.ReleaseTags},
	} {
		if !reflect.DeepEqual(values.got, values.want) {
			t.Fatalf("%s = %v, want %v", name, values.got, values.want)
		}
	}
	context.BuildTags = append(context.BuildTags, "test-only")
	context.ToolTags = append(context.ToolTags, "test-only")
	context.ReleaseTags = append(context.ReleaseTags, "test-only")
	if !reflect.DeepEqual(build.Default.BuildTags, before.BuildTags) ||
		!reflect.DeepEqual(build.Default.ToolTags, before.ToolTags) ||
		!reflect.DeepEqual(build.Default.ReleaseTags, before.ReleaseTags) {
		t.Fatalf(
			"default build tags mutated:\nbefore: %#v\nafter:  %#v",
			before,
			build.Default,
		)
	}
}

func TestGenerationSourceSelectionUsesProvidedTargetContext(t *testing.T) {
	tests := []struct {
		name     string
		context  build.Context
		active   string
		inactive string
	}{
		{
			name: "linux",
			context: build.Context{
				GOOS:        "linux",
				GOARCH:      "amd64",
				Compiler:    "gc",
				ToolTags:    []string{"amd64.v1"},
				ReleaseTags: []string{"go1.22"},
			},
			active:   "source_linux.go",
			inactive: "source_windows.go",
		},
		{
			name: "windows",
			context: build.Context{
				GOOS:        "windows",
				GOARCH:      "amd64",
				Compiler:    "gc",
				ToolTags:    []string{"amd64.v1"},
				ReleaseTags: []string{"go1.22"},
			},
			active:   "source_windows.go",
			inactive: "source_linux.go",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selection := generationSourceSelection{
				buildContext:   &test.context,
				excludeGoTests: true,
			}
			for _, source := range []struct {
				name string
				want bool
			}{
				{name: test.active, want: true},
				{name: test.inactive},
				{name: "source_test.go"},
			} {
				got, err := selection.matchAuthoredGo(
					t.TempDir(),
					source.name,
					[]byte("package app\n"),
				)
				if err != nil {
					t.Fatal(err)
				}
				if got != source.want {
					t.Fatalf("matchAuthoredGo(%q) = %t, want %t", source.name, got, source.want)
				}
			}
		})
	}
}

func TestGOXConstraintSelectionUsesGeneratedFilename(t *testing.T) {
	if got := generatedGOXSourceFilename("view_windows.gox"); got != "view_windows.gox.go" {
		t.Fatalf("generated GOX source filename = %q", got)
	}
	context := build.Context{
		GOOS:        "linux",
		GOARCH:      "amd64",
		Compiler:    "gc",
		ToolTags:    []string{"amd64.v1"},
		ReleaseTags: []string{"go1.22"},
	}
	selection := generationSourceSelection{buildContext: &context}
	packageDir := t.TempDir()
	content := []byte("package app\n")
	writeTestFile(
		t,
		packageDir,
		"view_windows.gox.go",
		string(content),
	)
	matched, err := selection.matchGOX(
		packageDir,
		"view_windows.gox",
		content,
	)
	if err != nil {
		t.Fatal(err)
	}
	want, err := context.MatchFile(packageDir, "view_windows.gox.go")
	if err != nil {
		t.Fatal(err)
	}
	if matched != want {
		t.Fatalf("GOX match = %t, actual generated filename match = %t", matched, want)
	}
}

func TestPrepareBuildWorkspaceFiltersInactiveAuthoredConstraints(t *testing.T) {
	t.Setenv("GOWASM", "")
	tests := []struct {
		name     string
		filename string
		source   string
	}{
		{
			name:     "inactive OS file",
			filename: "inactive_windows.go",
			source: `//go:build windows

package app

func Broken( {
`,
		},
		{
			name:     "inactive build expression",
			filename: "inactive.go",
			source: `//go:build !js

package app

func Broken( {
`,
		},
		{
			name:     "future release",
			filename: "future.go",
			source: `//go:build go1.999

package app

func Broken( {
`,
		},
		{
			name:     "inactive host architecture feature",
			filename: "inactive_amd64_feature.go",
			source: `//go:build amd64.v1

package app

func Broken( {
`,
		},
		{
			name:     "test file",
			filename: "collision_test.go",
			source: `package app

func Broken( {
`,
		},
	}

	for _, compiler := range []string{"go", "tinygo"} {
		t.Run(compiler, func(t *testing.T) {
			if compiler == "tinygo" {
				if _, err := exec.LookPath("tinygo"); err != nil {
					t.Skip("tinygo is not installed")
				}
			}
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					appDir := newAuthoredConstraintFixture(t)
					writeTestFile(t, appDir, test.filename, test.source)
					layout := newAuthoredConstraintLayout(t, appDir, compiler)
					if _, err := prepareBuildWorkspaceResult(
						layout,
						defaultEmbedManifest(compiler, "."),
					); err != nil {
						t.Fatalf("prepareBuildWorkspaceResult() error: %v", err)
					}
				})
			}
		})
	}
}

func TestGenerateIntoDirectoryFiltersInactiveHostFeatureSource(t *testing.T) {
	t.Setenv("GOWASM", "")
	for _, compiler := range []string{"go", "tinygo"} {
		t.Run(compiler, func(t *testing.T) {
			appDir := newAuthoredConstraintFixture(t)
			writeTestFile(t, appDir, "inactive_amd64_feature.go", `//go:build amd64.v1

package app

func Broken( {
`)
			if err := generateIntoDirectoryForCompiler(
				appDir,
				t.TempDir(),
				true,
				compiler,
			); err != nil {
				t.Fatalf("generateIntoDirectoryForCompiler() error: %v", err)
			}
		})
	}
}

func TestPrepareBuildWorkspaceActiveAuthoredConstraintFails(t *testing.T) {
	t.Setenv("GOWASM", "")
	for _, compiler := range []string{"go", "tinygo"} {
		t.Run(compiler, func(t *testing.T) {
			appDir := newAuthoredConstraintFixture(t)
			activePath := filepath.Join(appDir, "active.go")
			writeTestFile(t, appDir, "active.go", `//go:build js && wasm

package app

func Broken( {
`)
			layout := newAuthoredConstraintLayout(t, appDir, compiler)
			_, err := prepareBuildWorkspaceResult(
				layout,
				defaultEmbedManifest(compiler, "."),
			)
			if err == nil {
				t.Fatal("prepareBuildWorkspaceResult() succeeded")
			}
			for _, want := range []string{
				activePath,
				"parse transformed Go source before generated declarations",
			} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}

func TestGenerateIntoDirectoryCompilerIdentifierConstraints(t *testing.T) {
	t.Setenv("GOWASM", "")
	const candidate = "_goxComponent_view_Button"
	tests := []struct {
		name         string
		filename     string
		constraint   string
		goReserved   bool
		tinyReserved bool
	}{
		{
			name:       "inactive OS collision",
			filename:   "collision_windows.go",
			constraint: "windows",
		},
		{
			name:     "test collision",
			filename: "collision_test.go",
		},
		{
			name:         "active collision",
			filename:     "collision_js_wasm.go",
			goReserved:   true,
			tinyReserved: true,
		},
		{
			name:         "gc collision",
			filename:     "collision.go",
			constraint:   "gc",
			goReserved:   true,
			tinyReserved: true,
		},
		{
			name:         "tinygo collision",
			filename:     "collision.go",
			constraint:   "tinygo",
			tinyReserved: true,
		},
		{
			name:       "inactive host architecture collision",
			filename:   "collision.go",
			constraint: "amd64.v1",
		},
	}

	for _, compiler := range []string{"go", "tinygo"} {
		t.Run(compiler, func(t *testing.T) {
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					root := newPackageIdentifierFixture(t)
					source := "package app\n\nvar " + candidate + " = 1\n"
					if test.constraint != "" {
						source = "//go:build " + test.constraint + "\n\n" + source
					}
					writeTestFile(t, root, test.filename, source)
					writeTestFile(t, root, "view.gox", packageIdentifierGOXSource("View", "Button"))

					destination := t.TempDir()
					if err := generateIntoDirectoryForCompiler(
						root,
						destination,
						true,
						compiler,
					); err != nil {
						t.Fatalf("generateIntoDirectoryForCompiler() error: %v", err)
					}
					got := generatedIdentifierForComponent(
						t,
						filepath.Join(destination, "view.gox.go"),
						"Button",
					)
					wantReserved := test.goReserved
					if compiler == "tinygo" {
						wantReserved = test.tinyReserved
					}
					if wantReserved && got == candidate {
						t.Fatalf("active authored identifier %q was not reserved", candidate)
					}
					if !wantReserved && got != candidate {
						t.Fatalf("inactive authored identifier changed candidate to %q", got)
					}
				})
			}
		})
	}
}

func TestGeneratePathUsesCurrentAuthoredConstraintPolicy(t *testing.T) {
	inactiveOS := "windows"
	if build.Default.GOOS == "windows" {
		inactiveOS = "linux"
	}
	tests := []struct {
		name       string
		filename   string
		constraint string
	}{
		{
			name:       "inactive OS source",
			filename:   "inactive_" + inactiveOS + ".go",
			constraint: inactiveOS,
		},
		{
			name:     "test source",
			filename: "collision_test.go",
		},
		{
			name:       "future release source",
			filename:   "future.go",
			constraint: "go1.999",
		},
		{
			name:       "false build expression",
			filename:   "inactive.go",
			constraint: "windows && linux",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appDir := newAuthoredConstraintFixture(t)
			source := "package app\n\nfunc Broken( {\n"
			if test.constraint != "" {
				source = "//go:build " + test.constraint + "\n\n" + source
			}
			writeTestFile(t, appDir, test.filename, source)
			if err := generateIntoDirectory(appDir, t.TempDir(), true); err != nil {
				t.Fatalf("generateIntoDirectory() error: %v", err)
			}
		})
	}

	t.Run("active authored collision", func(t *testing.T) {
		const candidate = "_goxComponent_view_Button"
		appDir := newAuthoredConstraintFixture(t)
		writeTestFile(
			t,
			appDir,
			"collision.go",
			"package app\n\nvar "+candidate+" = 1\n",
		)
		destination := t.TempDir()
		if err := generateIntoDirectory(appDir, destination, true); err != nil {
			t.Fatalf("generateIntoDirectory() error: %v", err)
		}
		if got := generatedIdentifierForComponent(
			t,
			filepath.Join(destination, "view.gox.go"),
			"Button",
		); got == candidate {
			t.Fatalf("active authored identifier %q was not reserved", candidate)
		}
	})
}

func TestPrepareBuildWorkspaceFiltersInactiveGOXBeforePackageGeneration(t *testing.T) {
	t.Setenv("GOWASM", "")
	for _, compiler := range []string{"go", "tinygo"} {
		t.Run(compiler, func(t *testing.T) {
			if compiler == "tinygo" {
				if _, err := exec.LookPath("tinygo"); err != nil {
					t.Skip("tinygo is not installed")
				}
			}

			appDir := newAuthoredConstraintFixture(t)
			writeTestFile(t, appDir, "inactive.gox", `//go:build windows

package app_windows

import gf "github.com/graybuton/goframe/pkg/goframe"

func Inactive() gf.Node {
	return <main>inactive</main>
}
`)
			writeTestFile(t, appDir, "malformed.gox", `//go:build windows

this is not valid GOX
`)
			writeTestFile(t, appDir, "future.gox", `//go:build go1.999

this is not valid GOX
`)
			layout := newAuthoredConstraintLayout(t, appDir, compiler)
			if _, err := prepareBuildWorkspaceResult(
				layout,
				defaultEmbedManifest(compiler, "."),
			); err != nil {
				t.Fatalf("prepareBuildWorkspaceResult() error: %v", err)
			}

			config := workspaceModuleConfigForApp(appDir)
			appWorkDir := filepath.Join(layout.WorkDir, filepath.FromSlash(config.AppRel))
			if _, err := os.Stat(filepath.Join(appWorkDir, "view.gox.go")); err != nil {
				t.Fatalf("active generated output missing: %v", err)
			}
			if _, err := os.Stat(filepath.Join(appWorkDir, "inactive.gox.go")); !os.IsNotExist(err) {
				t.Fatalf("inactive generated output exists: %v", err)
			}
			if _, err := os.Stat(filepath.Join(appWorkDir, "malformed.gox.go")); !os.IsNotExist(err) {
				t.Fatalf("malformed inactive generated output exists: %v", err)
			}
			if _, err := os.Stat(filepath.Join(appWorkDir, "future.gox.go")); !os.IsNotExist(err) {
				t.Fatalf("future inactive generated output exists: %v", err)
			}
		})
	}
}

func TestGOXConstraintActivePackageMismatchStillFails(t *testing.T) {
	t.Setenv("GOWASM", "")
	for _, compiler := range []string{"go", "tinygo"} {
		t.Run(compiler, func(t *testing.T) {
			appDir := newAuthoredConstraintFixture(t)
			writeTestFile(t, appDir, "other.gox", `//go:build js && wasm

package other

import gf "github.com/graybuton/goframe/pkg/goframe"

func Other() gf.Node {
	return <main>other</main>
}
`)
			err := generateIntoDirectoryForCompiler(
				appDir,
				t.TempDir(),
				true,
				compiler,
			)
			if err == nil {
				t.Fatal("active package mismatch succeeded")
			}
			if !strings.Contains(err.Error(), "package app does not match package other") &&
				!strings.Contains(err.Error(), "package other does not match package app") {
				t.Fatalf("error %q does not identify active package mismatch", err)
			}
		})
	}
}

func TestInactiveGOXCollisionDoesNotPerturbActiveIdentifier(t *testing.T) {
	t.Setenv("GOWASM", "")
	for _, compiler := range []string{"go", "tinygo"} {
		t.Run(compiler, func(t *testing.T) {
			root := newPackageIdentifierFixture(t)
			writeTestFile(t, root, "view.gox", packageIdentifierGOXSource("View", "A_B"))
			writeTestFile(t, root, "view_A.gox", `//go:build windows

`+packageIdentifierGOXSource("Other", "B"))

			destination := t.TempDir()
			if err := generateIntoDirectoryForCompiler(
				root,
				destination,
				true,
				compiler,
			); err != nil {
				t.Fatalf("generateIntoDirectoryForCompiler() error: %v", err)
			}
			const candidate = "_goxComponent_view_A_B"
			if got := generatedIdentifierForComponent(
				t,
				filepath.Join(destination, "view.gox.go"),
				"A_B",
			); got != candidate {
				t.Fatalf("inactive GOX collision changed active identifier to %q", got)
			}
			if _, err := os.Stat(filepath.Join(destination, "view_A.gox.go")); !os.IsNotExist(err) {
				t.Fatalf("inactive collision output exists: %v", err)
			}
		})
	}
}

func TestPackageBrowserActiveGOXCollisionRemainsCoordinated(t *testing.T) {
	t.Setenv("GOWASM", "")
	for _, compiler := range []string{"go", "tinygo"} {
		t.Run(compiler, func(t *testing.T) {
			root := newPackageIdentifierFixture(t)
			writeTestFile(t, root, "view.gox", packageIdentifierGOXSource("View", "A_B"))
			writeTestFile(t, root, "view_A.gox", packageIdentifierGOXSource("Other", "B"))

			destination := t.TempDir()
			if err := generateIntoDirectoryForCompiler(
				root,
				destination,
				true,
				compiler,
			); err != nil {
				t.Fatalf("generateIntoDirectoryForCompiler() error: %v", err)
			}
			view := generatedIdentifierForComponent(
				t,
				filepath.Join(destination, "view.gox.go"),
				"A_B",
			)
			other := generatedIdentifierForComponent(
				t,
				filepath.Join(destination, "view_A.gox.go"),
				"B",
			)
			if view == other {
				t.Fatalf("active GOX components use duplicate identifier %q", view)
			}
		})
	}
}

func TestPrepareBuildWorkspaceAllowsNoActiveGOX(t *testing.T) {
	t.Setenv("GOWASM", "")
	for _, compiler := range []string{"go", "tinygo"} {
		t.Run(compiler, func(t *testing.T) {
			if compiler == "tinygo" {
				if _, err := exec.LookPath("tinygo"); err != nil {
					t.Skip("tinygo is not installed")
				}
			}
			appDir := t.TempDir()
			writeTestFile(t, appDir, "go.mod", "module example.com/app\n\ngo 1.22\n")
			writeTestFile(t, appDir, "main.go", "package app\n")
			writeTestFile(t, appDir, "inactive.gox", `//go:build windows

package other
`)
			layout := newAuthoredConstraintLayout(t, appDir, compiler)
			if _, err := prepareBuildWorkspaceResult(
				layout,
				defaultEmbedManifest(compiler, "."),
			); err != nil {
				t.Fatalf("prepareBuildWorkspaceResult() error: %v", err)
			}
			config := workspaceModuleConfigForApp(appDir)
			appWorkDir := filepath.Join(layout.WorkDir, filepath.FromSlash(config.AppRel))
			if _, err := os.Stat(filepath.Join(appWorkDir, "inactive.gox.go")); !os.IsNotExist(err) {
				t.Fatalf("inactive generated output exists: %v", err)
			}
		})
	}
}

func TestBrowserTargetActiveIdentifierCollisionCompiles(t *testing.T) {
	root := newPackageIdentifierFixture(t)
	writeTestFile(t, root, "collision_js_wasm.go", `package app

var _goxComponent_view_Button = 1
`)
	writeTestFile(t, root, "view.gox", packageIdentifierGOXSource("View", "Button"))
	if err := generateIntoDirectoryForCompiler(root, root, true, "go"); err != nil {
		t.Fatalf("generateIntoDirectoryForCompiler() error: %v", err)
	}
	if got := generatedIdentifierForComponent(
		t,
		filepath.Join(root, "view.gox.go"),
		"Button",
	); got == "_goxComponent_view_Button" {
		t.Fatal("active authored identifier was not disambiguated")
	}

	output := filepath.Join(t.TempDir(), "app.test.wasm")
	command := exec.Command("go", "test", "-c", "-o", output, ".")
	command.Dir = root
	command.Env = setEnvironmentValue(os.Environ(), "GOWORK", "off")
	command.Env = setEnvironmentValue(command.Env, "GOOS", "js")
	command.Env = setEnvironmentValue(command.Env, "GOARCH", "wasm")
	command.Env = setEnvironmentValue(command.Env, "CGO_ENABLED", "0")
	command.Env = setEnvironmentValue(
		command.Env,
		"GOFLAGS",
		"-mod=mod -buildvcs=false",
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile browser package: %v\n%s", err, output)
	}
}

func TestStandaloneGenerationSourceSelectionParity(t *testing.T) {
	root := newSourceSelectionFixture(t)
	selected, err := authoredPackageSources(
		root,
		defaultGenerationSourceSelection(),
	)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(selected))
	for _, source := range selected {
		got = append(got, filepath.Base(source.Filename))
	}
	sort.Strings(got)

	want := realCurrentGoSourceSelection(t, root)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf(
			"default selector differs from current go list:\nprivate: %v\ntoolchain: %v",
			got,
			want,
		)
	}
}

func TestBrowserCompilerSourceSelectionParity(t *testing.T) {
	root := newSourceSelectionFixture(t)
	for _, target := range []struct {
		name   string
		gowasm string
	}{
		{name: "default"},
		{name: "wasm features", gowasm: "satconv,signext"},
	} {
		t.Run(target.name, func(t *testing.T) {
			t.Setenv("GOWASM", target.gowasm)
			for _, compiler := range []string{"go", "tinygo"} {
				t.Run(compiler, func(t *testing.T) {
					if compiler == "tinygo" {
						if _, err := exec.LookPath("tinygo"); err != nil {
							t.Skip("tinygo is not installed")
						}
					}
					selection, err := browserGenerationSourceSelection(compiler)
					if err != nil {
						t.Fatal(err)
					}
					selected, err := authoredPackageSources(root, selection)
					if err != nil {
						t.Fatal(err)
					}
					got := make([]string, 0, len(selected))
					for _, source := range selected {
						got = append(got, filepath.Base(source.Filename))
					}
					sort.Strings(got)

					want := realToolchainSourceSelection(t, compiler, root)
					if !reflect.DeepEqual(got, want) {
						t.Fatalf(
							"private selector differs from %s list:\nprivate: %v\ntoolchain: %v",
							compiler,
							got,
							want,
						)
					}
				})
			}
		})
	}
}

func TestBrowserGOXSourceSelectionParity(t *testing.T) {
	root, sources := newGOXSourceSelectionFixture(t)
	for _, target := range []struct {
		name   string
		gowasm string
	}{
		{name: "default"},
		{name: "wasm features", gowasm: "satconv,signext"},
	} {
		t.Run(target.name, func(t *testing.T) {
			t.Setenv("GOWASM", target.gowasm)
			for _, compiler := range []string{"go", "tinygo"} {
				t.Run(compiler, func(t *testing.T) {
					if compiler == "tinygo" {
						if _, err := exec.LookPath("tinygo"); err != nil {
							t.Skip("tinygo is not installed")
						}
					}
					selection, err := browserGenerationSourceSelection(compiler)
					if err != nil {
						t.Fatal(err)
					}
					var got []string
					for _, source := range sources {
						content, err := readGenerationSource(source, "GOX source file")
						if err != nil {
							t.Fatal(err)
						}
						matched, err := selection.matchGOX(
							root,
							filepath.Base(source),
							content,
						)
						if err != nil {
							t.Fatal(err)
						}
						if matched {
							got = append(
								got,
								generatedGOXSourceFilename(source),
							)
						}
					}
					sort.Strings(got)

					var want []string
					for _, name := range realToolchainSourceSelection(t, compiler, root) {
						if strings.HasSuffix(name, ".gox.go") {
							want = append(want, name)
						}
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf(
							"private GOX selector differs from %s list:\nprivate: %v\ntoolchain: %v",
							compiler,
							got,
							want,
						)
					}
				})
			}
		})
	}
}

func newAuthoredConstraintFixture(t *testing.T) string {
	t.Helper()
	appDir := t.TempDir()
	writeTestFile(t, appDir, "go.mod", "module example.com/app\n\ngo 1.22\n")
	writeTestFile(t, appDir, "main.go", "package app\n")
	writeTestFile(t, appDir, "view.gox", packageIdentifierGOXSource("View", "Button"))
	writeTestFile(t, appDir, "components.go", `package app

import gf "github.com/graybuton/goframe/pkg/goframe"

type ButtonProps struct{}

func Button(ButtonProps) gf.Node {
	return gf.Text("button")
}
`)
	return appDir
}

func newAuthoredConstraintLayout(
	t *testing.T,
	appDir,
	compiler string,
) BuildLayout {
	t.Helper()
	layout, err := newBuildLayout(layoutOptions{
		appDir:    appDir,
		compiler:  compiler,
		workspace: filepath.Join(t.TempDir(), "workspace"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return layout
}

func newSourceSelectionFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/selection\n\ngo 1.22\n")
	writeTestFile(t, root, "base.go", "package selection\n")

	constraints := []struct {
		name       string
		constraint string
	}{
		{name: "tag_js.go", constraint: "js"},
		{name: "tag_wasm.go", constraint: "wasm"},
		{name: "tag_js_wasm.go", constraint: "js && wasm"},
		{name: "tag_tinygo.go", constraint: "tinygo"},
		{name: "tag_gc.go", constraint: "gc"},
		{name: "tag_not_tinygo.go", constraint: "!tinygo"},
		{name: "tag_cgo.go", constraint: "cgo"},
		{name: "tag_go122.go", constraint: "go1.22"},
		{name: "tag_go1999.go", constraint: "go1.999"},
		{name: "tag_tinygowasm.go", constraint: "tinygo.wasm"},
		{name: "tag_purego.go", constraint: "purego"},
		{name: "tag_osusergo.go", constraint: "osusergo"},
		{name: "tag_math_big.go", constraint: "math_big_pure_go"},
		{name: "tag_gc_precise.go", constraint: "gc.precise"},
		{name: "tag_scheduler.go", constraint: "scheduler.asyncify"},
		{name: "tag_serial.go", constraint: "serial.none"},
		{name: "tag_unicore.go", constraint: "tinygo.unicore"},
		{name: "tag_goexperiment.go", constraint: "goexperiment.coverageredesign"},
		{name: "tag_amd64_v1.go", constraint: "amd64.v1"},
		{name: "tag_wasm_satconv.go", constraint: "wasm.satconv"},
		{name: "tag_wasm_signext.go", constraint: "wasm.signext"},
	}
	for _, source := range constraints {
		writeTestFile(
			t,
			root,
			source.name,
			"//go:build "+source.constraint+"\n\npackage selection\n",
		)
	}
	for _, name := range []string{
		"suffix_js.go",
		"suffix_wasm.go",
		"suffix_js_wasm.go",
		"suffix_windows.go",
		"suffix_linux.go",
		"suffix_test.go",
	} {
		writeTestFile(t, root, name, "package selection\n")
	}
	return root
}

func newGOXSourceSelectionFixture(t *testing.T) (string, []string) {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/gox-selection\n\ngo 1.22\n")
	writeTestFile(t, root, "base.go", "package selection\n")
	tests := []struct {
		filename   string
		constraint string
	}{
		{filename: "plain.gox"},
		{filename: "tag_windows.gox", constraint: "windows"},
		{filename: "tag_js.gox", constraint: "js"},
		{filename: "tag_wasm.gox", constraint: "wasm"},
		{filename: "tag_browser.gox", constraint: "js && wasm"},
		{filename: "tag_gc.gox", constraint: "gc"},
		{filename: "tag_tinygo.gox", constraint: "tinygo"},
		{filename: "tag_cgo.gox", constraint: "cgo"},
		{filename: "tag_future.gox", constraint: "go1.999"},
		{filename: "tag_amd64.gox", constraint: "amd64.v1"},
		{filename: "tag_satconv.gox", constraint: "wasm.satconv"},
		{filename: "tag_signext.gox", constraint: "wasm.signext"},
		{filename: "suffix_windows.gox"},
		{filename: "suffix_js.gox"},
	}
	sources := make([]string, 0, len(tests))
	for _, test := range tests {
		content := "package selection\n"
		if test.constraint != "" {
			content = "//go:build " + test.constraint + "\n\n" + content
		}
		writeTestFile(t, root, test.filename, content)
		writeTestFile(
			t,
			root,
			generatedGOXSourceFilename(test.filename),
			generatedGOXFileHeader+content,
		)
		sources = append(sources, filepath.Join(root, test.filename))
	}
	return root, sources
}

func realToolchainSourceSelection(
	t *testing.T,
	compiler,
	root string,
) []string {
	t.Helper()
	var command *exec.Cmd
	switch compiler {
	case "go":
		command = exec.Command("go", "list", "-e", "-json", ".")
		if err := configureWorkspaceCompilerCommand(
			command,
			compilerEnvironmentOptions{
				Compiler:         "go",
				Invocation:       compilerInvocationEmbedDiscovery,
				WorkingDirectory: root,
				GoFlags:          workspaceCompilerBaseGoFlags,
				StandardGoTarget: true,
			},
		); err != nil {
			t.Fatal(err)
		}
	case "tinygo":
		command = exec.Command("tinygo", "list", "-target=wasm", "-json", ".")
		if err := configureWorkspaceCompilerCommand(
			command,
			compilerEnvironmentOptions{
				Compiler:         "tinygo",
				Invocation:       compilerInvocationEmbedDiscovery,
				WorkingDirectory: root,
				GoFlags:          workspaceCompilerBaseGoFlags,
				StandardGoTarget: false,
			},
		); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unsupported compiler %q", compiler)
	}

	output, err := command.Output()
	if err != nil {
		t.Fatalf("%s list: %v", compiler, err)
	}
	var packageInfo struct {
		GoFiles  []string
		CgoFiles []string
	}
	if err := json.Unmarshal(output, &packageInfo); err != nil {
		t.Fatal(err)
	}
	files := append(packageInfo.GoFiles, packageInfo.CgoFiles...)
	sort.Strings(files)
	return files
}

func realCurrentGoSourceSelection(t *testing.T, root string) []string {
	t.Helper()
	command := exec.Command("go", "list", "-e", "-json", ".")
	command.Dir = root
	command.Env = setEnvironmentValue(os.Environ(), "GOWORK", "off")
	command.Env = setEnvironmentValue(command.Env, "GOOS", build.Default.GOOS)
	command.Env = setEnvironmentValue(command.Env, "GOARCH", build.Default.GOARCH)
	cgoEnabled := "0"
	if build.Default.CgoEnabled {
		cgoEnabled = "1"
	}
	command.Env = setEnvironmentValue(command.Env, "CGO_ENABLED", cgoEnabled)
	goFlags := "-mod=mod -buildvcs=false"
	if len(build.Default.BuildTags) > 0 {
		goFlags += " -tags=" + strings.Join(build.Default.BuildTags, ",")
	}
	command.Env = setEnvironmentValue(
		command.Env,
		"GOFLAGS",
		goFlags,
	)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	var packageInfo struct {
		GoFiles  []string
		CgoFiles []string
	}
	if err := json.Unmarshal(output, &packageInfo); err != nil {
		t.Fatal(err)
	}
	files := append(packageInfo.GoFiles, packageInfo.CgoFiles...)
	sort.Strings(files)
	return files
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func cloneBuildContextTagSlices(context build.Context) build.Context {
	context.BuildTags = append([]string(nil), context.BuildTags...)
	context.ToolTags = append([]string(nil), context.ToolTags...)
	context.ReleaseTags = append([]string(nil), context.ReleaseTags...)
	return context
}

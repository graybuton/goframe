package main

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestEmbedMaterializationUsesGoToolSemantics(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		files   map[string]string
		want    []string
		absent  []string
		wantErr string
	}{
		{
			name:   "single file",
			source: embedStringSource("message.txt"),
			files:  map[string]string{"message.txt": "hello"},
			want:   []string{"message.txt"},
		},
		{
			name:   "multiple patterns",
			source: embedFSSource("templates/*.txt config/app.json"),
			files: map[string]string{
				"templates/first.txt":  "first",
				"templates/second.txt": "second",
				"templates/skip.html":  "skip",
				"config/app.json":      `{}`,
			},
			want:   []string{"config/app.json", "templates/first.txt", "templates/second.txt"},
			absent: []string{"templates/skip.html"},
		},
		{
			name: "repeated directives",
			source: `package main

import "embed"

//go:embed first.txt
//go:embed second.txt
var files embed.FS

func main() { _ = files }
`,
			files: map[string]string{"first.txt": "first", "second.txt": "second"},
			want:  []string{"first.txt", "second.txt"},
		},
		{
			name:   "quoted filename",
			source: embedStringSource(`"hello world.txt"`),
			files:  map[string]string{"hello world.txt": "hello"},
			want:   []string{"hello world.txt"},
		},
		{
			name:   "directory default exclusions",
			source: embedFSSource("templates"),
			files: map[string]string{
				"templates/visible.txt":        "visible",
				"templates/nested/data.txt":    "nested",
				"templates/.hidden.txt":        "hidden",
				"templates/_private.txt":       "private",
				"templates/nested/.hidden.txt": "nested hidden",
			},
			want: []string{"templates/nested/data.txt", "templates/visible.txt"},
			absent: []string{
				"templates/.hidden.txt",
				"templates/_private.txt",
				"templates/nested/.hidden.txt",
			},
		},
		{
			name:   "all directory includes hidden names",
			source: embedFSSource("all:templates"),
			files: map[string]string{
				"templates/visible.txt":  "visible",
				"templates/.hidden.txt":  "hidden",
				"templates/_private.txt": "private",
			},
			want: []string{"templates/.hidden.txt", "templates/_private.txt", "templates/visible.txt"},
		},
		{
			name:   "overlapping patterns deduplicate",
			source: embedFSSource("templates/*.txt templates/one.txt"),
			files:  map[string]string{"templates/one.txt": "one", "templates/two.txt": "two"},
			want:   []string{"templates/one.txt", "templates/two.txt"},
		},
		{
			name:    "no match",
			source:  embedStringSource("missing.txt"),
			wantErr: "pattern missing.txt: no matching files found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appDir := newEmbedTestApp(t, test.source, test.files)
			layout := newEmbedTestLayout(t, appDir, "go", "")
			result, err := prepareBuildWorkspaceResult(layout, defaultEmbedManifest("go", "."))
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("prepareBuildWorkspaceResult() error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("prepareBuildWorkspaceResult() error: %v", err)
			}
			got := embedPlanPaths(result.EmbedPlan)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("embed plan = %#v, want %#v", got, test.want)
			}
			for _, relative := range test.want {
				workspacePath := filepath.Join(layout.WorkDir, filepath.FromSlash(relative))
				gotContent, readErr := os.ReadFile(workspacePath)
				if readErr != nil {
					t.Fatalf("read materialized %s: %v", relative, readErr)
				}
				if string(gotContent) != test.files[relative] {
					t.Fatalf("materialized %s = %q, want %q", relative, gotContent, test.files[relative])
				}
			}
			for _, relative := range test.absent {
				if _, statErr := os.Stat(filepath.Join(layout.WorkDir, filepath.FromSlash(relative))); !os.IsNotExist(statErr) {
					t.Fatalf("unmatched input %s was materialized: %v", relative, statErr)
				}
			}
		})
	}
}

func TestEmbedDiscoveryUsesBrowserBuildConstraints(t *testing.T) {
	appDir := newEmbedTestApp(t, `package main

func main() {}
`, map[string]string{
		"active.txt": "active",
	})
	writeTestFile(t, appDir, "browser.go", `//go:build js && wasm

package main

import _ "embed"

//go:embed active.txt
var active string
`)
	writeTestFile(t, appDir, "host.go", `//go:build linux

package main

import _ "embed"

//go:embed missing-host.txt
var host string
`)

	layout := newEmbedTestLayout(t, appDir, "go", "")
	result, err := prepareBuildWorkspaceResult(layout, defaultEmbedManifest("go", "."))
	if err != nil {
		t.Fatalf("prepareBuildWorkspaceResult() error: %v", err)
	}
	if got, want := embedPlanPaths(result.EmbedPlan), []string{"active.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("embed plan = %#v, want %#v", got, want)
	}
}

func TestEmbedDiscoveryUsesTinyGoBuildConstraint(t *testing.T) {
	if _, err := exec.LookPath("tinygo"); err != nil {
		t.Skip("TinyGo is not available")
	}
	appDir := newEmbedTestApp(t, `package main

func main() {}
`, map[string]string{"tiny.txt": "tiny"})
	writeTestFile(t, appDir, "tiny.go", `//go:build tinygo

package main

import _ "embed"

//go:embed tiny.txt
var tiny string
`)
	layout := newEmbedTestLayout(t, appDir, "tinygo", "")
	result, err := prepareBuildWorkspaceResult(layout, defaultEmbedManifest("tinygo", "."))
	if err != nil {
		t.Fatalf("prepareBuildWorkspaceResult() error: %v", err)
	}
	if got, want := embedPlanPaths(result.EmbedPlan), []string{"tiny.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("embed plan = %#v, want %#v", got, want)
	}
}

func TestEmbedDiscoveryFiltersExternalDependencyInputs(t *testing.T) {
	root := t.TempDir()
	externalDir := filepath.Join(root, "external")
	writeTestFile(t, externalDir, "go.mod", "module example.com/external\n\ngo 1.22\n")
	writeTestFile(t, externalDir, "content.go", `package external

import _ "embed"

//go:embed external.txt
var Message string
`)
	writeTestFile(t, externalDir, "external.txt", "external-secret")
	appDir := filepath.Join(root, "app")
	writeTestFile(t, appDir, "go.mod", "module example.com/app\n\ngo 1.22\n\nrequire example.com/external v0.0.0\n\nreplace example.com/external => ../external\n")
	writeTestFile(t, appDir, "main.go", `package main

import "example.com/external"

func main() { _ = external.Message }
`)
	layout := newEmbedTestLayout(t, appDir, "go", "")
	result, err := prepareBuildWorkspaceResult(layout, defaultEmbedManifest("go", "."))
	if err != nil {
		t.Fatalf("prepareBuildWorkspaceResult() error: %v", err)
	}
	if len(result.EmbedPlan.Files) != 0 {
		t.Fatalf("external embed files entered local plan: %#v", result.EmbedPlan.Files)
	}
	if matches, err := filepath.Glob(filepath.Join(layout.WorkDir, "**", "external.txt")); err != nil || len(matches) != 0 {
		t.Fatalf("external embed file materialized: matches=%#v err=%v", matches, err)
	}
}

func TestEmbedInputPlanResolvesPhysicalWorkspaceAlias(t *testing.T) {
	requireSymlinkSupport(t)
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	workspaceParent := filepath.Join(root, "physical")
	workspaceRoot := filepath.Join(workspaceParent, "workspace")
	workspaceAliasParent := filepath.Join(root, "physical-alias")
	workspaceAlias := filepath.Join(workspaceAliasParent, "workspace")
	writeTestFile(t, appDir, "message.txt", "authored")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(workspaceParent, workspaceAliasParent); err != nil {
		t.Fatal(err)
	}

	discovery, err := createEmbedDiscoveryOverlay(appDir, workspaceAlias)
	if err != nil {
		t.Fatal(err)
	}
	defer discovery.Cleanup()
	_, plan, err := resolveEmbedInputPlan(appDir, workspaceAlias, discovery.Candidates, []embedListPackage{{
		Dir:        workspaceRoot,
		ImportPath: "example.com/alias",
		EmbedFiles: []string{"message.txt"},
	}})
	if err != nil {
		t.Fatalf("resolveEmbedInputPlan() error: %v", err)
	}
	if got, want := embedPlanPaths(plan), []string{"message.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("embed plan = %#v, want %#v", got, want)
	}
	if got := plan.Files[0].SourcePath; got != filepath.Join(appDir, "message.txt") {
		t.Fatalf("authored source = %q, want app-local message.txt", got)
	}
}

func TestEmbedDiscoveryOverlayPreservesLexicalWorkspacePath(t *testing.T) {
	requireSymlinkSupport(t)
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	workspaceParent := filepath.Join(root, "physical")
	workspaceRoot := filepath.Join(workspaceParent, "workspace")
	workspaceAliasParent := filepath.Join(root, "physical-alias")
	workspaceAlias := filepath.Join(workspaceAliasParent, "workspace")
	writeTestFile(t, appDir, "message.txt", "authored")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(workspaceParent, workspaceAliasParent); err != nil {
		t.Fatal(err)
	}

	discovery, err := createEmbedDiscoveryOverlay(appDir, workspaceAlias)
	if err != nil {
		t.Fatal(err)
	}
	defer discovery.Cleanup()
	content, err := os.ReadFile(discovery.OverlayPath)
	if err != nil {
		t.Fatal(err)
	}
	var overlay embedDiscoveryOverlay
	if err := json.Unmarshal(content, &overlay); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(filepath.Join(workspaceAlias, "message.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want = filepath.Clean(want)
	if got := overlay.Replace[want]; got != filepath.Join(appDir, "message.txt") {
		t.Fatalf("lexical overlay replacement = %q, want authored message.txt; replacements=%#v", got, overlay.Replace)
	}
	physical, err := canonicalPathForComparison(filepath.Join(workspaceRoot, "message.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if physical != want {
		if _, ok := overlay.Replace[physical]; ok {
			t.Fatalf("overlay retained physical replacement key %q: %#v", physical, overlay.Replace)
		}
	}
}

func TestEmbedDiscoveryOverlayUsesLexicalWorkspaceAliasIntegration(t *testing.T) {
	requireSymlinkSupport(t)
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	physicalParent := filepath.Join(root, "physical")
	workspaceRoot := filepath.Join(physicalParent, "workspace")
	aliasParent := filepath.Join(root, "physical-alias")
	workspaceAlias := filepath.Join(aliasParent, "workspace")
	source := `package main

import _ "embed"

//go:embed message.txt
var message string

func main() { _ = message }
`
	writeTestFile(t, appDir, "go.mod", "module example.com/lexical-overlay\n\ngo 1.22\n")
	writeTestFile(t, appDir, "main.go", source)
	writeTestFile(t, appDir, "message.txt", "authored")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(physicalParent, aliasParent); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, workspaceRoot, "go.mod", "module example.com/lexical-overlay\n\ngo 1.22\n")
	writeTestFile(t, workspaceRoot, "main.go", source)

	discovery, err := createEmbedDiscoveryOverlay(appDir, workspaceAlias)
	if err != nil {
		t.Fatal(err)
	}
	defer discovery.Cleanup()
	stalePWD := filepath.Join(root, "unrelated-pwd")
	if err := os.MkdirAll(stalePWD, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PWD", stalePWD)
	packages, err := listGoEmbedPackages(workspaceAlias, discovery.OverlayPath, false)
	if err != nil {
		t.Fatalf("listGoEmbedPackages() error: %v", err)
	}
	if err := embedPackageMetadataError(packages); err != nil {
		t.Fatalf("embedPackageMetadataError() error: %v", err)
	}
	var discovered *embedListPackage
	for index := range packages {
		if packages[index].ImportPath == "example.com/lexical-overlay" {
			discovered = &packages[index]
			break
		}
	}
	if discovered == nil {
		t.Fatalf("root package missing from go list output: %#v", packages)
	}
	if got, want := discovered.EmbedPatterns, []string{"message.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("EmbedPatterns = %#v, want %#v", got, want)
	}
	if got, want := discovered.EmbedFiles, []string{"message.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("EmbedFiles = %#v, want %#v", got, want)
	}
	_, plan, err := resolveEmbedInputPlan(appDir, workspaceAlias, discovery.Candidates, packages)
	if err != nil {
		t.Fatalf("resolveEmbedInputPlan() error: %v", err)
	}
	if got, want := embedPlanPaths(plan), []string{"message.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("embed plan = %#v, want %#v", got, want)
	}
}

func TestEmbedDiscoveryTinyGoUsesLexicalWorkspaceAliasIntegration(t *testing.T) {
	if _, err := exec.LookPath("tinygo"); err != nil {
		t.Skip("TinyGo is not available")
	}
	requireSymlinkSupport(t)
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	physicalParent := filepath.Join(root, "physical")
	workspaceRoot := filepath.Join(physicalParent, "workspace")
	aliasParent := filepath.Join(root, "physical-alias")
	workspaceAlias := filepath.Join(aliasParent, "workspace")
	source := `package main

import _ "embed"

//go:embed message.txt
var message string

func main() { _ = message }
`
	writeTestFile(t, appDir, "go.mod", "module example.com/lexical-tinygo-overlay\n\ngo 1.22\n")
	writeTestFile(t, appDir, "main.go", source)
	writeTestFile(t, appDir, "message.txt", "authored")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(physicalParent, aliasParent); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, workspaceRoot, "go.mod", "module example.com/lexical-tinygo-overlay\n\ngo 1.22\n")
	writeTestFile(t, workspaceRoot, "main.go", source)

	discovery, err := createEmbedDiscoveryOverlay(appDir, workspaceAlias)
	if err != nil {
		t.Fatal(err)
	}
	defer discovery.Cleanup()
	stalePWD := filepath.Join(root, "unrelated-pwd")
	if err := os.MkdirAll(stalePWD, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PWD", stalePWD)
	packages, err := listTinyGoEmbedPackages(workspaceAlias, discovery.OverlayPath)
	if err != nil {
		t.Fatalf("listTinyGoEmbedPackages() error: %v", err)
	}
	if err := embedPackageMetadataError(packages); err != nil {
		t.Fatalf("embedPackageMetadataError() error: %v", err)
	}
	var discovered *embedListPackage
	for index := range packages {
		if packages[index].ImportPath == "example.com/lexical-tinygo-overlay" {
			discovered = &packages[index]
			break
		}
	}
	if discovered == nil {
		t.Fatalf("root package missing from tinygo list output: %#v", packages)
	}
	if got, want := discovered.EmbedPatterns, []string{"message.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("EmbedPatterns = %#v, want %#v", got, want)
	}
	if got, want := discovered.EmbedFiles, []string{"message.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("EmbedFiles = %#v, want %#v", got, want)
	}
	_, plan, err := resolveEmbedInputPlan(appDir, workspaceAlias, discovery.Candidates, packages)
	if err != nil {
		t.Fatalf("resolveEmbedInputPlan() error: %v", err)
	}
	if got, want := embedPlanPaths(plan), []string{"message.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("embed plan = %#v, want %#v", got, want)
	}
}

func TestEmbedDiscoveryUsesOrdinaryWorkspaceWithStalePWD(t *testing.T) {
	appDir := newEmbedTestApp(t, embedStringSource("message.txt"), map[string]string{"message.txt": "ordinary workspace"})
	layout := newEmbedTestLayout(t, appDir, "go", "")
	stalePWD := t.TempDir()
	t.Setenv("PWD", stalePWD)
	result, err := prepareBuildWorkspaceResult(layout, defaultEmbedManifest("go", "."))
	if err != nil {
		t.Fatalf("prepareBuildWorkspaceResult() error: %v", err)
	}
	if got, want := embedPlanPaths(result.EmbedPlan), []string{"message.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("embed plan = %#v, want %#v", got, want)
	}
	assertEmbedFileContent(t, filepath.Join(layout.WorkDir, "message.txt"), "ordinary workspace")
}

func TestEmbedListCommandPreservesLexicalWorkingDirectory(t *testing.T) {
	requireSymlinkSupport(t)
	root := t.TempDir()
	physicalEntryPath := filepath.Join(root, "physical", "entry")
	aliasParent := filepath.Join(root, "physical-alias")
	lexicalEntryPath := filepath.Join(aliasParent, "entry")
	if err := os.MkdirAll(physicalEntryPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "physical"), aliasParent); err != nil {
		t.Fatal(err)
	}
	stalePWD := filepath.Join(root, "stale-pwd")
	parentPWD := filepath.Join(root, "parent-pwd")
	if err := os.MkdirAll(stalePWD, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(parentPWD, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PWD", parentPWD)
	want, err := filepath.Abs(lexicalEntryPath)
	if err != nil {
		t.Fatal(err)
	}
	want = filepath.Clean(want)
	physical, err := canonicalPathForComparison(lexicalEntryPath)
	if err != nil {
		t.Fatal(err)
	}
	if physical == want {
		t.Fatalf("test alias did not preserve a distinct lexical path: %q", want)
	}

	for _, test := range []struct {
		name        string
		environment func() []string
		wantValues  []string
	}{
		{
			name: "go",
			environment: func() []string {
				return append(compilerEnvironment("go"), "GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0", "PWD="+stalePWD)
			},
			wantValues: []string{"GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0"},
		},
		{
			name: "tinygo",
			environment: func() []string {
				environment := setEnvironmentValue(compilerEnvironment("tinygo"), "GOFLAGS", "-buildvcs=false -overlay=overlay.json")
				return append(environment, "PWD="+stalePWD)
			},
			wantValues: []string{"GOFLAGS=-buildvcs=false -overlay=overlay.json"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := &exec.Cmd{}
			if err := configureEmbedListCommand(command, lexicalEntryPath, test.environment()); err != nil {
				t.Fatalf("configureEmbedListCommand() error: %v", err)
			}
			if command.Dir != want {
				t.Fatalf("command.Dir = %q, want lexical path %q", command.Dir, want)
			}
			pwdCount := 0
			for _, item := range command.Env {
				if strings.HasPrefix(item, "PWD=") {
					pwdCount++
					if item != "PWD="+want {
						t.Fatalf("command PWD = %q, want %q", item, "PWD="+want)
					}
				}
				if item == "PWD="+stalePWD {
					t.Fatalf("stale PWD remained in command environment: %#v", command.Env)
				}
			}
			if pwdCount != 1 {
				t.Fatalf("command environment contains %d PWD entries, want 1: %#v", pwdCount, command.Env)
			}
			for _, wantValue := range test.wantValues {
				if !stringSliceContains(command.Env, wantValue) {
					t.Fatalf("command environment lost %q: %#v", wantValue, command.Env)
				}
			}
			if command.Dir == physical {
				t.Fatalf("command directory was physically canonicalized to %q", physical)
			}
			if got := os.Getenv("PWD"); got != parentPWD {
				t.Fatalf("parent PWD = %q, want unchanged %q", got, parentPWD)
			}
		})
	}
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestEmbedWorkspacePreparesThroughExternalWorkspaceAlias(t *testing.T) {
	requireSymlinkSupport(t)
	root := t.TempDir()
	appDir := newEmbedTestApp(t, embedStringSource("message.txt"), map[string]string{"message.txt": "external workspace"})
	physicalParent := filepath.Join(root, "physical")
	physicalWorkspace := filepath.Join(physicalParent, "workspace")
	aliasParent := filepath.Join(root, "physical-alias")
	workspaceAlias := filepath.Join(aliasParent, "workspace")
	if err := os.MkdirAll(physicalWorkspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(physicalParent, aliasParent); err != nil {
		t.Fatal(err)
	}
	layout := newEmbedTestLayout(t, appDir, "go", workspaceAlias)
	stalePWD := filepath.Join(root, "unrelated-pwd")
	if err := os.MkdirAll(stalePWD, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PWD", stalePWD)
	result, err := prepareBuildWorkspaceResult(layout, defaultEmbedManifest("go", "."))
	if err != nil {
		t.Fatalf("prepareBuildWorkspaceResult() error: %v", err)
	}
	if !result.EmbedPlan.Resolved {
		t.Fatal("embed plan was not resolved")
	}
	if got, want := embedPlanPaths(result.EmbedPlan), []string{"message.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("embed plan = %#v, want %#v", got, want)
	}
	assertEmbedFileContent(t, filepath.Join(layout.WorkDir, "message.txt"), "external workspace")
	if matches, err := filepath.Glob(filepath.Join(layout.WorkspaceRoot, "goxc-embed-*.json")); err != nil || len(matches) != 0 {
		t.Fatalf("temporary overlay leaked into workspace: matches=%#v err=%v", matches, err)
	}
}

func TestEmbedInputKeysPreservePhysicalApplicationAliasIdentity(t *testing.T) {
	requireSymlinkSupport(t)
	root := t.TempDir()
	physicalParent := filepath.Join(root, "physical")
	appDir := filepath.Join(physicalParent, "app")
	aliasParent := filepath.Join(root, "physical-alias")
	appAlias := filepath.Join(aliasParent, "app")
	source := filepath.Join(appDir, "internal", "content", "payload.txt")
	writeTestFile(t, appDir, "internal/content/payload.txt", "payload")
	if err := os.Symlink(physicalParent, aliasParent); err != nil {
		t.Fatal(err)
	}

	realRelative, inside, err := relativePathBelow(appDir, source)
	if err != nil || !inside {
		t.Fatalf("real source relation = %q, inside=%v, err=%v", realRelative, inside, err)
	}
	aliasRelative, inside, err := relativePathBelow(appAlias, source)
	if err != nil || !inside {
		t.Fatalf("aliased source relation = %q, inside=%v, err=%v", aliasRelative, inside, err)
	}
	realKey, err := newEmbedInputKey(realRelative)
	if err != nil {
		t.Fatal(err)
	}
	aliasKey, err := newEmbedInputKey(aliasRelative)
	if err != nil {
		t.Fatal(err)
	}
	if realKey != aliasKey || realKey != embedInputKey("internal/content/payload.txt") {
		t.Fatalf("physical alias keys = %q and %q", realKey, aliasKey)
	}
	if err := validatePathBelowRoot(appDir, source, "embedded source input", false); err != nil {
		t.Fatalf("validate physical authored source: %v", err)
	}
}

func TestEmbedInputPlanResolvesMissingWorkspaceTail(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	workspaceParent := filepath.Join(root, "workspace")
	appWorkDir := filepath.Join(workspaceParent, "missing-app")
	source := filepath.Join(appDir, "cmd", "app", "message.txt")
	writeTestFile(t, appDir, "cmd/app/message.txt", "payload")
	if err := os.MkdirAll(workspaceParent, 0o755); err != nil {
		t.Fatal(err)
	}

	inputs, plan, err := resolveEmbedInputPlan(appDir, appWorkDir, map[embedInputKey]embedCandidate{
		"cmd/app/message.txt": {SourcePath: source, Kind: embedCandidateRegular},
	}, []embedListPackage{{
		Dir:        filepath.Join(appWorkDir, "cmd", "app"),
		ImportPath: "example.com/missing/cmd/app",
		EmbedFiles: []string{"message.txt"},
	}})
	if err != nil {
		t.Fatalf("resolveEmbedInputPlan() error: %v", err)
	}
	if got, want := embedPlanPaths(plan), []string{"cmd/app/message.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("embed plan = %#v, want %#v", got, want)
	}
	if len(inputs) != 1 || !inputs[0].copy || inputs[0].destination != filepath.Join(appWorkDir, "cmd", "app", "message.txt") {
		t.Fatalf("missing-tail input = %+v", inputs)
	}
}

func TestEmbedDiscoveryCandidatesUseDistinctRelativeKeys(t *testing.T) {
	appDir := t.TempDir()
	appWorkDir := t.TempDir()
	writeTestFile(t, appDir, "first/payload.txt", "first")
	writeTestFile(t, appDir, "second/payload.txt", "second")
	discovery, err := createEmbedDiscoveryOverlay(appDir, appWorkDir)
	if err != nil {
		t.Fatal(err)
	}
	defer discovery.Cleanup()
	if len(discovery.Candidates) != 2 {
		t.Fatalf("candidate count = %d, want 2", len(discovery.Candidates))
	}
	for _, key := range []embedInputKey{"first/payload.txt", "second/payload.txt"} {
		if _, ok := discovery.Candidates[key]; !ok {
			t.Fatalf("candidate key %q missing: %#v", key, discovery.Candidates)
		}
	}
}

func TestEmbedInputPlanFiltersPhysicalPackageOutsideWorkspace(t *testing.T) {
	appDir := t.TempDir()
	appWorkDir := t.TempDir()
	externalPackage := filepath.Join(t.TempDir(), "external")
	if err := os.MkdirAll(externalPackage, 0o755); err != nil {
		t.Fatal(err)
	}
	inputs, plan, err := resolveEmbedInputPlan(appDir, appWorkDir, nil, []embedListPackage{{
		Dir:           externalPackage,
		ImportPath:    "example.com/external",
		EmbedPatterns: []string{"payload.txt"},
		EmbedFiles:    []string{"payload.txt"},
	}})
	if err != nil {
		t.Fatalf("resolveEmbedInputPlan() error: %v", err)
	}
	if len(inputs) != 0 || len(plan.Files) != 0 || len(plan.Watches) != 0 {
		t.Fatalf("external package entered embed plan: inputs=%#v plan=%#v", inputs, plan)
	}
}

func TestEmbedInputPlanFiltersPackageOnDifferentWindowsVolume(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows volume identity regression")
	}
	if relative, inside, err := relativeCanonicalPathBelow(
		`C:\workspace\app`,
		`D:\repository\pkg\goframe`,
		`C:\workspace\app`,
		`D:\repository\pkg\goframe`,
	); err != nil || inside || relative != "" {
		t.Fatalf("different-volume relation = %q, inside=%v, err=%v", relative, inside, err)
	}

	repositoryRoot, ok := findRepositoryRoot(".")
	if !ok {
		t.Fatal("repository root not found")
	}
	appDir := t.TempDir()
	appWorkDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(appWorkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	externalPackage := filepath.Join(repositoryRoot, "pkg", "goframe")
	if strings.EqualFold(filepath.VolumeName(appWorkDir), filepath.VolumeName(externalPackage)) {
		return
	}
	_, plan, err := resolveEmbedInputPlan(appDir, appWorkDir, nil, []embedListPackage{{
		Dir:        externalPackage,
		ImportPath: canonicalModulePath + "/pkg/goframe",
	}})
	if err != nil {
		t.Fatalf("different-volume external package error: %v", err)
	}
	if len(plan.Files) != 0 || len(plan.Watches) != 0 {
		t.Fatalf("different-volume external package entered plan: %#v", plan)
	}
}

func TestEmbedInputPlanRejectsReservedWorkspaceInputsThroughAlias(t *testing.T) {
	requireSymlinkSupport(t)
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	workspaceParent := filepath.Join(root, "physical")
	workspaceRoot := filepath.Join(workspaceParent, "workspace")
	aliasParent := filepath.Join(root, "physical-alias")
	workspaceAlias := filepath.Join(aliasParent, "workspace")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "cmd", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(workspaceParent, aliasParent); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name       string
		packageDir string
		embedFile  string
		want       string
	}{
		{name: "go.mod", packageDir: workspaceRoot, embedFile: "go.mod", want: "workspace go.mod"},
		{name: "generated GOX", packageDir: filepath.Join(workspaceRoot, "cmd", "app"), embedFile: "app.gox.go", want: "workspace-generated GOX source"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := resolveEmbedInputPlan(appDir, workspaceAlias, nil, []embedListPackage{{
				Dir:        test.packageDir,
				ImportPath: "example.com/reserved",
				EmbedFiles: []string{test.embedFile},
			}})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("resolveEmbedInputPlan() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestEmbedWatchIdentityDeduplicatesPhysicalAliases(t *testing.T) {
	requireSymlinkSupport(t)
	root := t.TempDir()
	physicalParent := filepath.Join(root, "physical")
	appDir := filepath.Join(physicalParent, "app")
	aliasParent := filepath.Join(root, "physical-alias")
	appAlias := filepath.Join(aliasParent, "app")
	writeTestFile(t, appDir, "message.txt", "payload")
	writeTestFile(t, appDir, "templates/page.html", "page")
	if err := os.Symlink(physicalParent, aliasParent); err != nil {
		t.Fatal(err)
	}

	realWatch, err := embedPatternWatchSpec(appDir, appDir, "message.txt")
	if err != nil {
		t.Fatal(err)
	}
	aliasWatch, err := embedPatternWatchSpec(appAlias, appAlias, "message.txt")
	if err != nil {
		t.Fatal(err)
	}
	if embedWatchKey(realWatch) != embedWatchKey(aliasWatch) {
		t.Fatalf("equivalent watch keys = %q and %q", embedWatchKey(realWatch), embedWatchKey(aliasWatch))
	}
	watches := map[string]embedWatchSpec{
		embedWatchKey(realWatch):  realWatch,
		embedWatchKey(aliasWatch): aliasWatch,
	}
	if len(watches) != 1 {
		t.Fatalf("deduplicated watch count = %d, want 1", len(watches))
	}
	realTreeWatch, err := embedPatternWatchSpec(appDir, appDir, "templates/*.html")
	if err != nil {
		t.Fatal(err)
	}
	aliasTreeWatch, err := embedPatternWatchSpec(appAlias, appAlias, "templates/*.html")
	if err != nil {
		t.Fatal(err)
	}
	if embedWatchKey(realTreeWatch) != embedWatchKey(aliasTreeWatch) {
		t.Fatalf("equivalent tree watch keys = %q and %q", embedWatchKey(realTreeWatch), embedWatchKey(aliasTreeWatch))
	}
	treeWatches := map[string]embedWatchSpec{
		embedWatchKey(realTreeWatch):  realTreeWatch,
		embedWatchKey(aliasTreeWatch): aliasTreeWatch,
	}
	if len(treeWatches) != 1 {
		t.Fatalf("deduplicated tree watch count = %d, want 1", len(treeWatches))
	}
	exactTreePath, err := newEmbedWatchSpec(appDir, realTreeWatch.Path, embedWatchExact)
	if err != nil {
		t.Fatal(err)
	}
	if embedWatchKey(exactTreePath) == embedWatchKey(realTreeWatch) {
		t.Fatal("exact and tree watches collapsed to one identity")
	}
}

func TestEmbedWorkspaceBuildsRootAndInternalPackageInputs(t *testing.T) {
	t.Run("root package", func(t *testing.T) {
		appDir := newEmbedTestApp(t, embedStringSource("message.txt"), map[string]string{"message.txt": "root payload"})
		output, err := buildApp(buildOptions{appDir: appDir, compiler: "go"})
		if err != nil {
			t.Fatalf("buildApp() error: %v", err)
		}
		if _, err := os.Stat(output); err != nil {
			t.Fatalf("built output missing: %v", err)
		}
		layout := newEmbedTestLayout(t, appDir, "go", "")
		assertEmbedFileContent(t, filepath.Join(layout.WorkDir, "message.txt"), "root payload")
	})

	t.Run("child entry with generated GOX dependency", func(t *testing.T) {
		appDir := t.TempDir()
		writeTestFile(t, appDir, "go.mod", "module example.com/multiembed\n\ngo 1.22\n")
		writeTestFile(t, appDir, "cmd/app/main.go", "package main\n\nfunc main() {}\n")
		writeTestFile(t, appDir, "cmd/app/app.gox", `package main

import (
	"example.com/multiembed/internal/content"
	gf "github.com/graybuton/goframe/pkg/goframe"
)

func App() gf.Node {
	return <p>{content.Message}</p>
}
`)
		writeTestFile(t, appDir, "internal/content/content.go", `package content

import _ "embed"

//go:embed payload.txt
var Message string
`)
		writeTestFile(t, appDir, "internal/content/payload.txt", "internal payload")
		writeTestFile(t, appDir, manifestName, `{"entry":"cmd/app","compiler":"go"}`)
		output, err := buildApp(buildOptions{appDir: appDir, compiler: "go"})
		if err != nil {
			t.Fatalf("buildApp() error: %v", err)
		}
		if _, err := os.Stat(output); err != nil {
			t.Fatalf("built output missing: %v", err)
		}
		layout := newEmbedTestLayout(t, appDir, "go", "")
		assertEmbedFileContent(t, filepath.Join(layout.WorkDir, "internal", "content", "payload.txt"), "internal payload")
	})
}

func TestEmbedWorkspaceSupportsExternalWorkspace(t *testing.T) {
	appDir := newEmbedTestApp(t, embedStringSource("message.txt"), map[string]string{"message.txt": "external workspace"})
	externalBase := t.TempDir()
	output, err := buildApp(buildOptions{appDir: appDir, compiler: "go", workspace: externalBase})
	if err != nil {
		t.Fatalf("buildApp() error: %v", err)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("built output missing: %v", err)
	}
	layout := newEmbedTestLayout(t, appDir, "go", externalBase)
	assertEmbedFileContent(t, filepath.Join(layout.WorkDir, "message.txt"), "external workspace")
	if matches, err := filepath.Glob(filepath.Join(layout.WorkspaceRoot, "**", "goxc-embed-overlay-*.json")); err != nil || len(matches) != 0 {
		t.Fatalf("temporary overlay leaked into workspace: matches=%#v err=%v", matches, err)
	}
}

func TestEmbedWorkspaceBuildsWithTinyGo(t *testing.T) {
	if _, err := exec.LookPath("tinygo"); err != nil {
		t.Skip("TinyGo is not available")
	}
	t.Setenv("GOFLAGS", strings.TrimSpace(os.Getenv("GOFLAGS")+" -buildvcs=false"))
	appDir := newEmbedTestApp(t, embedStringSource("message.txt"), map[string]string{"message.txt": "tinygo payload"})
	output, err := buildApp(buildOptions{appDir: appDir, compiler: "tinygo"})
	if err != nil {
		t.Fatalf("buildApp() error: %v", err)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("built output missing: %v", err)
	}
	layout := newEmbedTestLayout(t, appDir, "tinygo", "")
	assertEmbedFileContent(t, filepath.Join(layout.WorkDir, "message.txt"), "tinygo payload")
}

func TestEmbedMaterializationRejectsUnsafeInputs(t *testing.T) {
	t.Run("symlinked file", func(t *testing.T) {
		appDir := newEmbedTestApp(t, embedStringSource("message.txt"), nil)
		external := filepath.Join(t.TempDir(), "outside.txt")
		writeTestFile(t, filepath.Dir(external), filepath.Base(external), "outside-secret")
		if err := os.Symlink(external, filepath.Join(appDir, "message.txt")); err != nil {
			t.Skipf("symlink creation is unavailable: %v", err)
		}
		layout := newEmbedTestLayout(t, appDir, "go", "")
		_, err := prepareBuildWorkspaceResult(layout, defaultEmbedManifest("go", "."))
		if err == nil {
			t.Fatal("prepareBuildWorkspaceResult() succeeded for symlinked embedded input")
		}
		if content, readErr := os.ReadFile(filepath.Join(layout.WorkDir, "message.txt")); readErr == nil && string(content) == "outside-secret" {
			t.Fatal("external symlink target was copied")
		}
	})

	t.Run("symlinked directory", func(t *testing.T) {
		appDir := newEmbedTestApp(t, embedFSSource("templates"), nil)
		external := t.TempDir()
		writeTestFile(t, external, "outside.txt", "outside-secret")
		if err := os.Symlink(external, filepath.Join(appDir, "templates")); err != nil {
			t.Skipf("symlink creation is unavailable: %v", err)
		}
		layout := newEmbedTestLayout(t, appDir, "go", "")
		if _, err := prepareBuildWorkspaceResult(layout, defaultEmbedManifest("go", ".")); err == nil {
			t.Fatal("prepareBuildWorkspaceResult() succeeded for symlinked embedded directory")
		}
	})

	t.Run("destination traversal", func(t *testing.T) {
		appDir := t.TempDir()
		appWorkDir := t.TempDir()
		_, _, err := resolveEmbedInputPlan(appDir, appWorkDir, map[embedInputKey]embedCandidate{"escape.txt": {
			SourcePath: filepath.Join(appDir, "source.txt"),
			Kind:       embedCandidateRegular,
		}}, []embedListPackage{{
			Dir:        appWorkDir,
			ImportPath: "example.com/app",
			EmbedFiles: []string{"../escape.txt"},
		}})
		if err == nil || !strings.Contains(err.Error(), "must stay inside") {
			t.Fatalf("resolveEmbedInputPlan() error = %v, want workspace escape", err)
		}
	})

	t.Run("nested module and tool output", func(t *testing.T) {
		appDir := newEmbedTestApp(t, embedFSSource("nested .goframe"), nil)
		writeTestFile(t, appDir, "nested/go.mod", "module example.com/nested\n\ngo 1.22\n")
		writeTestFile(t, appDir, "nested/secret.txt", "nested-secret")
		writeTestFile(t, appDir, ".goframe/secret.txt", "tool-secret")
		layout := newEmbedTestLayout(t, appDir, "go", "")
		_, err := prepareBuildWorkspaceResult(layout, defaultEmbedManifest("go", "."))
		if err == nil {
			t.Fatal("prepareBuildWorkspaceResult() succeeded for excluded embed trees")
		}
		for _, secret := range []string{"nested-secret", "tool-secret"} {
			if workspaceContainsBytes(t, layout.WorkDir, secret) {
				t.Fatalf("workspace contains excluded bytes %q", secret)
			}
		}
	})
}

func TestEmbedDiscoveryPreservesUnsafeCandidateProvenance(t *testing.T) {
	requireSymlinkSupport(t)
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	physicalWorkspace := filepath.Join(root, "physical-workspace")
	workspaceAlias := filepath.Join(root, "workspace-alias")
	appWorkDir := filepath.Join(workspaceAlias, "work")
	writeTestFile(t, appDir, "main.go", embedFSSource("*"))
	if err := os.MkdirAll(physicalWorkspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(physicalWorkspace, workspaceAlias); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(root, "external.txt")
	writeTestFile(t, root, "external.txt", "external-secret")
	link := filepath.Join(appDir, "link.txt")
	if err := os.Symlink(external, link); err != nil {
		t.Fatal(err)
	}

	discovery, err := createEmbedDiscoveryOverlay(appDir, appWorkDir)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(appWorkDir, "link.txt")
	candidate, ok := discovery.Candidates["link.txt"]
	if !ok {
		discovery.Cleanup()
		t.Fatalf("symlink candidate missing for %s", destination)
	}
	if candidate.Kind != embedCandidateSymlink || candidate.SourcePath != link {
		discovery.Cleanup()
		t.Fatalf("symlink candidate = %+v", candidate)
	}
	overlayContent, err := os.ReadFile(discovery.OverlayPath)
	if err != nil {
		discovery.Cleanup()
		t.Fatal(err)
	}
	var overlay embedDiscoveryOverlay
	if err := json.Unmarshal(overlayContent, &overlay); err != nil {
		discovery.Cleanup()
		t.Fatal(err)
	}
	sentinel := overlay.Replace[destination]
	if sentinel == "" || sentinel == external || sentinel == link {
		discovery.Cleanup()
		t.Fatalf("unsafe overlay backing = %q", sentinel)
	}
	physicalDestination, err := canonicalPathForComparison(destination)
	if err != nil {
		discovery.Cleanup()
		t.Fatal(err)
	}
	if physicalDestination != destination {
		if _, ok := overlay.Replace[physicalDestination]; ok {
			discovery.Cleanup()
			t.Fatalf("unsafe overlay retained physical replacement key %q: %#v", physicalDestination, overlay.Replace)
		}
	}
	if _, inside, err := relativePathBelow(appDir, sentinel); err != nil || inside {
		discovery.Cleanup()
		t.Fatalf("sentinel %q is inside app: inside=%v err=%v", sentinel, inside, err)
	}
	if content, err := os.ReadFile(sentinel); err != nil || len(content) != 0 {
		discovery.Cleanup()
		t.Fatalf("sentinel content = %q, err=%v", content, err)
	}

	discovery.Cleanup()
	for _, temporary := range []string{discovery.OverlayPath, sentinel} {
		if _, err := os.Lstat(temporary); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("temporary discovery file %s remained: %v", temporary, err)
		}
	}
}

func TestEmbedMaterializationRejectsUnsafePatternCandidates(t *testing.T) {
	requireSymlinkSupport(t)
	tests := []struct {
		name       string
		pattern    string
		link       string
		additional map[string]string
	}{
		{
			name:       "broad root pattern",
			pattern:    "*",
			link:       "link.txt",
			additional: map[string]string{"regular.txt": "regular"},
		},
		{
			name:       "directory pattern with nested symlink",
			pattern:    "templates",
			link:       "templates/link.txt",
			additional: map[string]string{"templates/visible.txt": "visible"},
		},
		{
			name:       "all directory pattern",
			pattern:    "all:templates",
			link:       "templates/link.txt",
			additional: map[string]string{"templates/visible.txt": "visible"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			appDir := filepath.Join(root, "app")
			packageDir := filepath.Join(appDir, "cmd", "app")
			writeTestFile(t, appDir, "go.mod", "module example.com/unsafeembed\n\ngo 1.22\n")
			writeTestFile(t, packageDir, "main.go", embedFSSource(test.pattern))
			for relative, content := range test.additional {
				writeTestFile(t, packageDir, relative, content)
			}
			external := filepath.Join(root, "external.txt")
			writeTestFile(t, root, "external.txt", "external-secret")
			link := filepath.Join(packageDir, filepath.FromSlash(test.link))
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(external, link); err != nil {
				t.Fatal(err)
			}
			layout := newEmbedTestLayout(t, appDir, "go", "")
			_, err := prepareBuildWorkspaceResult(layout, defaultEmbedManifest("go", "cmd/app"))
			if err == nil || !strings.Contains(err.Error(), "symlinked authored input") || !strings.Contains(err.Error(), test.link) {
				t.Fatalf("prepareBuildWorkspaceResult() error = %v, want selected symlink rejection", err)
			}
			if workspaceContainsBytes(t, layout.WorkDir, "external-secret") {
				t.Fatal("external symlink target bytes entered the workspace")
			}
		})
	}
}

func TestEmbedMaterializationIgnoresUnselectedUnsafeCandidate(t *testing.T) {
	requireSymlinkSupport(t)
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	packageDir := filepath.Join(appDir, "cmd", "app")
	writeTestFile(t, appDir, "go.mod", "module example.com/unselectedembed\n\ngo 1.22\n")
	writeTestFile(t, packageDir, "main.go", embedStringSource("regular.txt"))
	writeTestFile(t, packageDir, "regular.txt", "regular")
	external := filepath.Join(root, "external.txt")
	writeTestFile(t, root, "external.txt", "external-secret")
	if err := os.Symlink(external, filepath.Join(packageDir, "unselected.txt")); err != nil {
		t.Fatal(err)
	}

	layout := newEmbedTestLayout(t, appDir, "go", "")
	result, err := prepareBuildWorkspaceResult(layout, defaultEmbedManifest("go", "cmd/app"))
	if err != nil {
		t.Fatalf("prepareBuildWorkspaceResult() error: %v", err)
	}
	if got, want := embedPlanPaths(result.EmbedPlan), []string{"cmd/app/regular.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("embed plan = %#v, want %#v", got, want)
	}
	if workspaceContainsBytes(t, layout.WorkDir, "external-secret") {
		t.Fatal("unselected symlink target bytes entered the workspace")
	}
}

func TestEmbedMaterializationRejectsIrregularPatternCandidate(t *testing.T) {
	t.Run("real socket", func(t *testing.T) {
		root, err := os.MkdirTemp("", "ge-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(root) })
		appDir := filepath.Join(root, "app")
		packageDir := filepath.Join(appDir, "cmd", "app")
		writeTestFile(t, appDir, "go.mod", "module example.com/irregularembed\n\ngo 1.22\n")
		writeTestFile(t, packageDir, "main.go", embedFSSource("*"))
		writeTestFile(t, packageDir, "regular.txt", "regular")
		socketPath := filepath.Join(packageDir, "payload.sock")
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			t.Skipf("Unix-domain socket creation is unavailable: %v", err)
		}
		defer listener.Close()

		layout := newEmbedTestLayout(t, appDir, "go", "")
		_, err = prepareBuildWorkspaceResult(layout, defaultEmbedManifest("go", "cmd/app"))
		if err == nil || !strings.Contains(err.Error(), "irregular authored input") || !strings.Contains(err.Error(), "payload.sock") {
			t.Fatalf("prepareBuildWorkspaceResult() error = %v, want irregular input rejection", err)
		}
	})

	t.Run("candidate validation", func(t *testing.T) {
		appDir := t.TempDir()
		appWorkDir := t.TempDir()
		_, _, err := resolveEmbedInputPlan(appDir, appWorkDir, map[embedInputKey]embedCandidate{
			"payload.irregular": {SourcePath: filepath.Join(appDir, "payload.irregular"), Kind: embedCandidateIrregular},
		}, []embedListPackage{{
			Dir:        appWorkDir,
			ImportPath: "example.com/irregular",
			EmbedFiles: []string{"payload.irregular"},
		}})
		if err == nil || !strings.Contains(err.Error(), "irregular authored input") {
			t.Fatalf("resolveEmbedInputPlan() error = %v, want irregular candidate rejection", err)
		}
	})
}

func TestEmbedMaterializationRejectsWorkspaceOnlyInputs(t *testing.T) {
	t.Run("missing authored go.mod", func(t *testing.T) {
		appDir := t.TempDir()
		writeTestFile(t, appDir, "main.go", embedStringSource("go.mod"))
		layout := newEmbedTestLayout(t, appDir, "go", "")
		_, err := prepareBuildWorkspaceResult(layout, defaultEmbedManifest("go", "."))
		if err == nil || !strings.Contains(err.Error(), "collides with generated workspace state") {
			t.Fatalf("prepareBuildWorkspaceResult() error = %v, want workspace go.mod rejection", err)
		}
	})

	t.Run("authored go.mod collision", func(t *testing.T) {
		appDir := newEmbedTestApp(t, embedStringSource("go.mod"), nil)
		layout := newEmbedTestLayout(t, appDir, "go", "")
		_, err := prepareBuildWorkspaceResult(layout, defaultEmbedManifest("go", "."))
		if err == nil || !strings.Contains(err.Error(), "workspace go.mod is not authored embed content") {
			t.Fatalf("prepareBuildWorkspaceResult() error = %v, want transformed go.mod rejection", err)
		}
	})

	t.Run("generated GOX output", func(t *testing.T) {
		repositoryRoot, ok := findRepositoryRoot(".")
		if !ok {
			t.Fatal("repository root not found")
		}
		appDir := t.TempDir()
		writeTestFile(t, appDir, "go.mod", "module example.com/generatedembed\n\ngo 1.22\n\nrequire "+canonicalModulePath+" v0.0.0\n\nreplace "+canonicalModulePath+" => "+filepath.ToSlash(repositoryRoot)+"\n")
		writeTestFile(t, appDir, "cmd/app/main.go", embedFSSource("*"))
		writeTestFile(t, appDir, "cmd/app/app.gox", `package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func App() gf.Node { return <p>generated</p> }
`)
		layout := newEmbedTestLayout(t, appDir, "go", "")
		_, err := prepareBuildWorkspaceResult(layout, defaultEmbedManifest("go", "cmd/app"))
		if err == nil || !strings.Contains(err.Error(), ".gox.go") || !strings.Contains(err.Error(), "workspace-generated GOX source") {
			t.Fatalf("prepareBuildWorkspaceResult() error = %v, want generated GOX rejection", err)
		}
	})

	t.Run("synthetic missing provenance", func(t *testing.T) {
		appDir := t.TempDir()
		appWorkDir := t.TempDir()
		writeTestFile(t, appWorkDir, "workspace-only.txt", "generated")
		_, _, err := resolveEmbedInputPlan(appDir, appWorkDir, nil, []embedListPackage{{
			Dir:        appWorkDir,
			ImportPath: "example.com/workspace-only",
			EmbedFiles: []string{"workspace-only.txt"},
		}})
		if err == nil || !strings.Contains(err.Error(), "has no valid authored backing file") {
			t.Fatalf("resolveEmbedInputPlan() error = %v, want missing provenance", err)
		}
	})
}

func TestEmbedMaterializationAcceptsAuthoredGoFileProvenance(t *testing.T) {
	appDir := newEmbedTestApp(t, embedStringSource("payload.go"), map[string]string{
		"payload.go": "package main\n\nconst authoredPayload = true\n",
	})
	layout := newEmbedTestLayout(t, appDir, "go", "")
	result, err := prepareBuildWorkspaceResult(layout, defaultEmbedManifest("go", "."))
	if err != nil {
		t.Fatalf("prepareBuildWorkspaceResult() error: %v", err)
	}
	if got, want := embedPlanPaths(result.EmbedPlan), []string{"payload.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("embed plan = %#v, want %#v", got, want)
	}
	assertEmbedFileContent(t, filepath.Join(layout.WorkDir, "payload.go"), "package main\n\nconst authoredPayload = true\n")
}

func TestEmbedPatternWatchBoundaries(t *testing.T) {
	appDir := t.TempDir()
	writeTestFile(t, appDir, "message.txt", "message")
	writeTestFile(t, appDir, "config/app.json", "{}")
	writeTestFile(t, appDir, "templates/one.txt", "one")
	tests := []struct {
		pattern string
		kind    embedWatchKind
		path    string
	}{
		{pattern: "message.txt", kind: embedWatchExact, path: "message.txt"},
		{pattern: "config/app.json", kind: embedWatchExact, path: "config/app.json"},
		{pattern: "missing.txt", kind: embedWatchExact, path: "missing.txt"},
		{pattern: "templates", kind: embedWatchTree, path: "templates"},
		{pattern: "templates/*.txt", kind: embedWatchTree, path: "templates"},
		{pattern: "*", kind: embedWatchTree, path: "."},
	}
	for _, test := range tests {
		t.Run(test.pattern, func(t *testing.T) {
			watch, err := embedPatternWatchSpec(appDir, appDir, test.pattern)
			if err != nil {
				t.Fatal(err)
			}
			if watch.Kind != test.kind || watch.Path != filepath.Join(appDir, filepath.FromSlash(test.path)) {
				t.Fatalf("watch = %+v, want kind=%d path=%s", watch, test.kind, test.path)
			}
		})
	}
}

func TestEmbedExactWatchFingerprintTracksOnlyPathType(t *testing.T) {
	requireSymlinkSupport(t)
	appDir := t.TempDir()
	target := filepath.Join(appDir, "message.txt")
	if got, err := embedWatchExactFingerprint(appDir, target); err != nil || got != "missing" {
		t.Fatalf("missing fingerprint = %q, err=%v", got, err)
	}
	writeTestFile(t, appDir, "message.txt", "first")
	first, err := embedWatchExactFingerprint(appDir, target)
	if err != nil || first != "regular" {
		t.Fatalf("regular fingerprint = %q, err=%v", first, err)
	}
	writeTestFile(t, appDir, "message.txt", "second")
	second, err := embedWatchExactFingerprint(appDir, target)
	if err != nil || second != first {
		t.Fatalf("content-only fingerprint = %q, want %q, err=%v", second, first, err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := embedWatchExactFingerprint(appDir, target); err != nil || got != "directory" {
		t.Fatalf("directory fingerprint = %q, err=%v", got, err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external.txt")
	writeTestFile(t, filepath.Dir(external), filepath.Base(external), "external")
	if err := os.Symlink(external, target); err != nil {
		t.Fatal(err)
	}
	if got, err := embedWatchExactFingerprint(appDir, target); err != nil || got != "symlink" {
		t.Fatalf("symlink fingerprint = %q, err=%v", got, err)
	}
}

func TestEmbedMaterializationDoesNotCopyUnrelatedFiles(t *testing.T) {
	appDir := newEmbedTestApp(t, embedStringSource("message.txt"), map[string]string{
		"message.txt": "included",
		"secret.txt":  "sensitive-unrelated-bytes",
	})
	layout := newEmbedTestLayout(t, appDir, "go", "")
	if _, err := prepareBuildWorkspaceResult(layout, defaultEmbedManifest("go", ".")); err != nil {
		t.Fatalf("prepareBuildWorkspaceResult() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(layout.WorkDir, "secret.txt")); !os.IsNotExist(err) {
		t.Fatalf("unrelated file was copied: %v", err)
	}
}

func newEmbedTestApp(t *testing.T, source string, files map[string]string) string {
	t.Helper()
	appDir := t.TempDir()
	writeTestFile(t, appDir, "go.mod", "module example.com/embedapp\n\ngo 1.22\n")
	writeTestFile(t, appDir, "main.go", source)
	for relative, content := range files {
		writeTestFile(t, appDir, relative, content)
	}
	return appDir
}

func newEmbedTestLayout(t *testing.T, appDir, compiler, workspace string) BuildLayout {
	t.Helper()
	layout, err := newBuildLayout(layoutOptions{appDir: appDir, compiler: compiler, workspace: workspace})
	if err != nil {
		t.Fatalf("newBuildLayout() error: %v", err)
	}
	return layout
}

func defaultEmbedManifest(compiler, entry string) projectManifest {
	return projectManifest{
		Name:     "embed-test",
		Entry:    entry,
		Compiler: compiler,
		WASM:     "bundle.wasm",
		Assets:   autoManifestAssets(),
	}
}

func embedStringSource(pattern string) string {
	return `package main

import _ "embed"

//go:embed ` + pattern + `
var payload string

func main() { _ = payload }
`
}

func embedFSSource(pattern string) string {
	return `package main

import "embed"

//go:embed ` + pattern + `
var payload embed.FS

func main() { _ = payload }
`
}

func embedPlanPaths(plan embedInputPlan) []string {
	paths := make([]string, 0, len(plan.Files))
	for _, file := range plan.Files {
		paths = append(paths, file.DisplayPath)
	}
	sort.Strings(paths)
	return paths
}

func assertEmbedFileContent(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(content) != want {
		t.Fatalf("%s = %q, want %q", path, content, want)
	}
}

func workspaceContainsBytes(t *testing.T, root, value string) bool {
	t.Helper()
	found := false
	err := filepath.WalkDir(root, func(current string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(current)
		if err != nil {
			return err
		}
		if strings.Contains(string(content), value) {
			found = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("inspect workspace: %v", err)
	}
	return found
}

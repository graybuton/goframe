package main

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestGeneratedPublicationPreflightsLaterActiveDestination(t *testing.T) {
	sourceRoot := t.TempDir()
	destinationRoot := t.TempDir()
	writeGeneratedPublicationSource(t, sourceRoot, "a.gox", "A", "first")
	writeGeneratedPublicationSource(t, sourceRoot, "z.gox", "Z", "last")

	blocked := filepath.Join(destinationRoot, "z.gox.go")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(destinationRoot, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := findGOXFiles(sourceRoot)
	if err != nil {
		t.Fatal(err)
	}
	err = generateFilesIntoDirectory(sourceRoot, destinationRoot, files)
	if err == nil {
		t.Fatal("generateFilesIntoDirectory() succeeded with a directory destination")
	}
	if !strings.Contains(err.Error(), blocked) || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("error %q does not identify blocked destination %s", err, blocked)
	}

	first := filepath.Join(destinationRoot, "a.gox.go")
	if content, readErr := os.ReadFile(first); readErr == nil {
		t.Errorf(
			"earlier output was published before later failure %v: size=%d sha256=%x",
			err,
			len(content),
			sha256.Sum256(content),
		)
	} else if !os.IsNotExist(readErr) {
		t.Fatalf("inspect earlier output: %v", readErr)
	}
	assertFileBytes(t, sentinel, []byte("keep"))
	if info, statErr := os.Stat(blocked); statErr != nil || !info.IsDir() {
		t.Fatalf("blocked destination changed: info=%v err=%v", info, statErr)
	}
}

func TestGeneratedPublicationPublishesCompleteActiveSet(t *testing.T) {
	sourceRoot := t.TempDir()
	destinationRoot := t.TempDir()
	writeGeneratedPublicationSource(t, sourceRoot, "a.gox", "A", "first")
	writeGeneratedPublicationSource(t, sourceRoot, "z.gox", "Z", "last")

	for _, name := range []string{"a.gox.go", "z.gox.go"} {
		if err := os.WriteFile(
			filepath.Join(destinationRoot, name),
			[]byte("old "+name),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
	}

	files, err := findGOXFiles(sourceRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := generateFilesIntoDirectory(sourceRoot, destinationRoot, files); err != nil {
		t.Fatalf("generateFilesIntoDirectory() error: %v", err)
	}
	for _, name := range []string{"a.gox.go", "z.gox.go"} {
		content, err := os.ReadFile(filepath.Join(destinationRoot, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(content), generatedGOXFileHeader) {
			t.Fatalf("generated output %s has no managed header:\n%s", name, content)
		}
	}
}

func TestGeneratedPublicationRollsBackLaterCommitFailure(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "a.gox.go")
	created := filepath.Join(root, "m.gox.go")
	inactive := filepath.Join(root, "z.gox.go")
	existingBefore := []byte(generatedGOXFileHeader + "package main\n\nvar previous = true\n")
	inactiveBefore := []byte(generatedGOXFileHeader + "package main\n\nvar inactive = true\n")
	if err := os.WriteFile(existing, existingBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inactive, inactiveBefore, 0o640); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(root, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	commitFailure := errors.New("forced later commit failure")
	err := publishGeneratedSourceSetWithHook(
		root,
		[]generatedGOXFile{
			{path: existing, content: []byte(generatedGOXFileHeader + "package main\n\nvar next = true\n")},
			{path: created, content: []byte(generatedGOXFileHeader + "package main\n\nvar created = true\n")},
		},
		[]string{inactive},
		func(step generatedPublicationStep) error {
			if step.phase == generatedPublicationCommit &&
				step.action == generatedPublicationReplaceOutput &&
				step.path == created {
				return commitFailure
			}
			return nil
		},
	)
	if !errors.Is(err, commitFailure) {
		t.Fatalf("publishGeneratedSourceSetWithHook() error = %v, want forced failure", err)
	}
	assertFileBytes(t, existing, existingBefore)
	if _, err := os.Lstat(created); !os.IsNotExist(err) {
		t.Fatalf("new output remains after rollback: %v", err)
	}
	assertFileBytes(t, inactive, inactiveBefore)
	assertFileBytes(t, sentinel, []byte("keep"))
	if runtime.GOOS != "windows" {
		assertFileMode(t, existing, 0o600)
		assertFileMode(t, inactive, 0o640)
	}
	assertNoGeneratedPublicationArtifacts(t, root)
}

func TestGeneratedPublicationPreflightsLaterActiveSymlink(t *testing.T) {
	requireSymlinkSupport(t)
	root := t.TempDir()
	first := filepath.Join(root, "a.gox.go")
	blocked := filepath.Join(root, "z.gox.go")
	external := filepath.Join(t.TempDir(), "external.go")
	externalContent := []byte("external")
	if err := os.WriteFile(external, externalContent, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, blocked); err != nil {
		t.Fatal(err)
	}

	err := publishGeneratedSourceSet(root, []generatedGOXFile{
		{path: first, content: []byte(generatedGOXFileHeader + "package main\n")},
		{path: blocked, content: []byte(generatedGOXFileHeader + "package main\n")},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), blocked) || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("publishGeneratedSourceSet() error = %v, want symlink refusal for %s", err, blocked)
	}
	if _, err := os.Lstat(first); !os.IsNotExist(err) {
		t.Fatalf("earlier output exists after symlink preflight failure: %v", err)
	}
	assertFileBytes(t, external, externalContent)
	assertNoGeneratedPublicationArtifacts(t, root)
}

func TestGeneratedPublicationStagingFailuresPreservePlan(t *testing.T) {
	actions := []generatedPublicationAction{
		generatedPublicationCreateStage,
		generatedPublicationWriteStage,
		generatedPublicationChmodStage,
		generatedPublicationCloseStage,
	}
	for _, action := range actions {
		t.Run(string(action), func(t *testing.T) {
			fixture := newGeneratedPublicationFixture(t)
			failure := errors.New("forced staging failure")
			err := publishGeneratedSourceSetWithHook(
				fixture.root,
				fixture.generated,
				fixture.removals,
				func(step generatedPublicationStep) error {
					if step.phase == generatedPublicationStage && step.action == action {
						return failure
					}
					return nil
				},
			)
			if !errors.Is(err, failure) {
				t.Fatalf("publishGeneratedSourceSetWithHook() error = %v, want staging failure", err)
			}
			fixture.assertPrior(t)
			assertNoGeneratedPublicationArtifacts(t, fixture.root)
		})
	}
}

func TestGeneratedPublicationDirectoryCreationFailureCleansCreatedDirectories(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "a.gox.go")
	existingBefore := []byte("existing")
	if err := os.WriteFile(existing, existingBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	inactive := filepath.Join(root, "z.gox.go")
	inactiveBefore := []byte(generatedGOXFileHeader + "package main\n")
	if err := os.WriteFile(inactive, inactiveBefore, 0o640); err != nil {
		t.Fatal(err)
	}
	created := filepath.Join(root, "package", "nested", "new.gox.go")
	failure := errors.New("forced directory creation failure")
	createdDirectories := 0
	err := publishGeneratedSourceSetWithHook(
		root,
		[]generatedGOXFile{
			{path: existing, content: []byte("next")},
			{path: created, content: []byte("new")},
		},
		[]string{inactive},
		func(step generatedPublicationStep) error {
			if step.phase == generatedPublicationStage &&
				step.action == generatedPublicationCreateDirectory {
				createdDirectories++
				if createdDirectories == 2 {
					return failure
				}
			}
			return nil
		},
	)
	if !errors.Is(err, failure) {
		t.Fatalf("publishGeneratedSourceSetWithHook() error = %v, want directory failure", err)
	}
	assertFileBytes(t, existing, existingBefore)
	assertFileBytes(t, inactive, inactiveBefore)
	if _, err := os.Lstat(created); !os.IsNotExist(err) {
		t.Fatalf("new output exists after directory failure: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "package")); !os.IsNotExist(err) {
		t.Fatalf("transaction-created directory remains: %v", err)
	}
	assertNoGeneratedPublicationArtifacts(t, root)
}

func TestGeneratedPublicationCommitFailuresRestoreCompletePriorSet(t *testing.T) {
	tests := []struct {
		name   string
		failAt int
	}{
		{name: "before first mutation", failAt: 1},
		{name: "after first replacement", failAt: 2},
		{name: "after multiple mutations", failAt: 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newGeneratedPublicationFixture(t)
			failure := errors.New("forced commit failure")
			var order []string
			err := publishGeneratedSourceSetWithHook(
				fixture.root,
				fixture.generated,
				fixture.removals,
				func(step generatedPublicationStep) error {
					if step.phase != generatedPublicationCommit {
						return nil
					}
					order = append(order, step.path)
					if len(order) == test.failAt {
						return failure
					}
					return nil
				},
			)
			if !errors.Is(err, failure) {
				t.Fatalf("publishGeneratedSourceSetWithHook() error = %v, want commit failure", err)
			}
			wantOrder := fixture.commitOrder()[:test.failAt]
			if !reflect.DeepEqual(order, wantOrder) {
				t.Fatalf("commit order = %v, want %v", order, wantOrder)
			}
			if !strings.Contains(err.Error(), wantOrder[len(wantOrder)-1]) {
				t.Fatalf("error %q does not identify failing path %s", err, wantOrder[len(wantOrder)-1])
			}
			fixture.assertPrior(t)
			assertNoGeneratedPublicationArtifacts(t, fixture.root)
		})
	}
}

func TestGeneratedPublicationMixedCommitAndRetry(t *testing.T) {
	fixture := newGeneratedPublicationFixture(t)
	var order []string
	err := publishGeneratedSourceSetWithHook(
		fixture.root,
		fixture.generated,
		fixture.removals,
		func(step generatedPublicationStep) error {
			if step.phase == generatedPublicationCommit {
				order = append(order, step.path)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("publishGeneratedSourceSetWithHook() error: %v", err)
	}
	if want := fixture.commitOrder(); !reflect.DeepEqual(order, want) {
		t.Fatalf("commit order = %v, want %v", order, want)
	}
	fixture.assertCommitted(t)
	assertNoGeneratedPublicationArtifacts(t, fixture.root)

	second := newGeneratedPublicationFixture(t)
	failure := errors.New("forced retry precursor")
	commitSteps := 0
	err = publishGeneratedSourceSetWithHook(
		second.root,
		second.generated,
		second.removals,
		func(step generatedPublicationStep) error {
			if step.phase == generatedPublicationCommit {
				commitSteps++
				if commitSteps == 3 {
					return failure
				}
			}
			return nil
		},
	)
	if !errors.Is(err, failure) {
		t.Fatalf("failed attempt error = %v, want forced failure", err)
	}
	second.assertPrior(t)
	if err := publishGeneratedSourceSet(second.root, second.generated, second.removals); err != nil {
		t.Fatalf("retry publication error: %v", err)
	}
	second.assertCommitted(t)
	assertNoGeneratedPublicationArtifacts(t, second.root)
}

func TestGeneratedPublicationRollsBackAcrossPackageDirectories(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first", "a.gox.go")
	later := filepath.Join(root, "second", "z.gox.go")
	sentinel := filepath.Join(root, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("forced second-package failure")
	commitSteps := 0
	generated := []generatedGOXFile{
		{path: later, content: []byte("second")},
		{path: first, content: []byte("first")},
	}
	err := publishGeneratedSourceSetWithHook(
		root,
		generated,
		nil,
		func(step generatedPublicationStep) error {
			if step.phase == generatedPublicationCommit {
				commitSteps++
				if commitSteps == 2 {
					return failure
				}
			}
			return nil
		},
	)
	if !errors.Is(err, failure) {
		t.Fatalf("publishGeneratedSourceSetWithHook() error = %v, want package failure", err)
	}
	for _, path := range []string{first, later, filepath.Dir(first), filepath.Dir(later)} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("transaction path remains after rollback %s: %v", path, err)
		}
	}
	assertFileBytes(t, sentinel, []byte("keep"))
	assertNoGeneratedPublicationArtifacts(t, root)

	if err := publishGeneratedSourceSet(root, generated, nil); err != nil {
		t.Fatalf("retry across package directories: %v", err)
	}
	assertFileBytes(t, first, []byte("first"))
	assertFileBytes(t, later, []byte("second"))
	assertNoGeneratedPublicationArtifacts(t, root)
}

func TestGeneratedPublicationReportsPrimaryAndRollbackFailures(t *testing.T) {
	fixture := newGeneratedPublicationFixture(t)
	primary := errors.New("forced inactive removal failure")
	rollback := errors.New("forced prior output restoration failure")
	err := publishGeneratedSourceSetWithHook(
		fixture.root,
		fixture.generated,
		fixture.removals,
		func(step generatedPublicationStep) error {
			switch {
			case step.phase == generatedPublicationCommit &&
				step.action == generatedPublicationRemoveOutput:
				return primary
			case step.phase == generatedPublicationRollback &&
				step.action == generatedPublicationRestoreOutput &&
				step.path == fixture.existing:
				return rollback
			default:
				return nil
			}
		},
	)
	if !errors.Is(err, primary) || !errors.Is(err, rollback) {
		t.Fatalf("joined error = %v, want primary and rollback failures", err)
	}
	for _, path := range []string{fixture.inactive, fixture.existing} {
		if !strings.Contains(err.Error(), path) {
			t.Fatalf("joined error %q does not identify %s", err, path)
		}
	}
	assertFileBytes(t, fixture.existing, fixture.existingNext)
	if _, err := os.Lstat(fixture.created); !os.IsNotExist(err) {
		t.Fatalf("created output remains despite successful rollback: %v", err)
	}
	assertFileBytes(t, fixture.inactive, fixture.inactiveBefore)
	assertFileBytes(t, fixture.sentinel, []byte("keep"))
	assertNoGeneratedPublicationArtifacts(t, fixture.root)
}

func TestGeneratedPublicationRejectsUnmanagedInactiveOutputBeforeMutation(t *testing.T) {
	root := t.TempDir()
	active := filepath.Join(root, "a.gox.go")
	activeBefore := []byte("active before")
	if err := os.WriteFile(active, activeBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	unmanaged := filepath.Join(root, "z.gox.go")
	unmanagedBefore := []byte("package main\n\nvar authored = true\n")
	if err := os.WriteFile(unmanaged, unmanagedBefore, 0o644); err != nil {
		t.Fatal(err)
	}

	err := publishGeneratedSourceSet(
		root,
		[]generatedGOXFile{{path: active, content: []byte("active after")}},
		[]string{unmanaged},
	)
	if err == nil || !strings.Contains(err.Error(), "not managed by goxc") {
		t.Fatalf("publishGeneratedSourceSet() error = %v, want unmanaged refusal", err)
	}
	assertFileBytes(t, active, activeBefore)
	assertFileBytes(t, unmanaged, unmanagedBefore)
	assertNoGeneratedPublicationArtifacts(t, root)
}

func TestGeneratedPublicationRejectsDuplicateDestination(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "same.gox.go")
	before := []byte("before")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	err := publishGeneratedSourceSet(root, []generatedGOXFile{
		{path: path, content: []byte("first")},
		{path: path, content: []byte("second")},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "same destination") {
		t.Fatalf("publishGeneratedSourceSet() error = %v, want duplicate destination refusal", err)
	}
	assertFileBytes(t, path, before)
}

func TestGeneratedPublicationMissingInactiveOutputIsNoop(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing.gox.go")
	if err := publishGeneratedSourceSet(root, nil, []string{missing}); err != nil {
		t.Fatalf("publishGeneratedSourceSet() missing removal error: %v", err)
	}
	if _, err := os.Lstat(missing); !os.IsNotExist(err) {
		t.Fatalf("missing inactive output appeared: %v", err)
	}
}

func TestGeneratedPublicationCommitsInactiveCleanupBeforeNoActiveResult(t *testing.T) {
	root := newPackageIdentifierFixture(t)
	source := filepath.Join(root, "view.gox")
	writeTestFile(t, root, "view.gox", packageIdentifierGOXSource("View", "Button"))
	outputRoot := t.TempDir()
	options := generateOptions{path: root, outDir: outputRoot}
	if _, err := runGeneratePathForTest(t, options); err != nil {
		t.Fatalf("initial generation error: %v", err)
	}
	output := filepath.Join(outputRoot, "view.gox.go")
	writeTestFile(
		t,
		root,
		"view.gox",
		"//go:build windows && linux\n\n"+packageIdentifierGOXSource("View", "Button"),
	)
	stdout, err := runGeneratePathForTest(t, options)
	if err == nil || !strings.Contains(err.Error(), "no active .gox files found below") {
		t.Fatalf("inactive generation error = %v", err)
	}
	if stdout != "" {
		t.Fatalf("inactive generation stdout = %q, want empty", stdout)
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatalf("managed inactive output remains: %v", err)
	}
	assertNoGeneratedPublicationArtifacts(t, outputRoot)
	assertFileBytes(t, source, []byte("//go:build windows && linux\n\n"+packageIdentifierGOXSource("View", "Button")))
}

func TestGeneratedPublicationSingleFileKeepsSiblingReadOnly(t *testing.T) {
	root := newPackageIdentifierFixture(t)
	writeCollisionGOXSources(t, root)
	outputRoot := t.TempDir()
	if _, err := runGeneratePathForTest(t, generateOptions{path: root, outDir: outputRoot}); err != nil {
		t.Fatalf("initial directory generation error: %v", err)
	}
	requestedSource := filepath.Join(root, "view_A.gox")
	requestedOutput := filepath.Join(outputRoot, "view_A.gox.go")
	siblingOutput := filepath.Join(outputRoot, "view.gox.go")
	requestedBefore, err := os.ReadFile(requestedOutput)
	if err != nil {
		t.Fatal(err)
	}
	siblingBefore, err := os.ReadFile(siblingOutput)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "view_A.gox", packageIdentifierGOXSource("OtherUpdated", "B"))
	if _, err := runGeneratePathForTest(t, generateOptions{
		path:   requestedSource,
		outDir: outputRoot,
	}); err != nil {
		t.Fatalf("single-file generation error: %v", err)
	}
	requestedAfter, err := os.ReadFile(requestedOutput)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(requestedAfter, requestedBefore) {
		t.Fatal("requested output did not change")
	}
	assertFileBytes(t, siblingOutput, siblingBefore)
	assertNoGeneratedPublicationArtifacts(t, outputRoot)
}

type generatedPublicationFixture struct {
	root           string
	existing       string
	created        string
	inactive       string
	sentinel       string
	existingBefore []byte
	existingNext   []byte
	createdNext    []byte
	inactiveBefore []byte
	generated      []generatedGOXFile
	removals       []string
}

func newGeneratedPublicationFixture(t *testing.T) generatedPublicationFixture {
	t.Helper()
	root := t.TempDir()
	fixture := generatedPublicationFixture{
		root:           root,
		existing:       filepath.Join(root, "a-existing.gox.go"),
		created:        filepath.Join(root, "m-created.gox.go"),
		inactive:       filepath.Join(root, "z-inactive.gox.go"),
		sentinel:       filepath.Join(root, "keep.txt"),
		existingBefore: []byte(generatedGOXFileHeader + "package main\n\nvar previous = true\n"),
		existingNext:   []byte(generatedGOXFileHeader + "package main\n\nvar next = true\n"),
		createdNext:    []byte(generatedGOXFileHeader + "package main\n\nvar created = true\n"),
		inactiveBefore: []byte(generatedGOXFileHeader + "package main\n\nvar inactive = true\n"),
	}
	if err := os.WriteFile(fixture.existing, fixture.existingBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.inactive, fixture.inactiveBefore, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.sentinel, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixture.generated = []generatedGOXFile{
		{path: fixture.created, content: fixture.createdNext},
		{path: fixture.existing, content: fixture.existingNext},
	}
	fixture.removals = []string{fixture.inactive}
	return fixture
}

func (fixture generatedPublicationFixture) commitOrder() []string {
	return []string{fixture.existing, fixture.created, fixture.inactive}
}

func (fixture generatedPublicationFixture) assertPrior(t *testing.T) {
	t.Helper()
	assertFileBytes(t, fixture.existing, fixture.existingBefore)
	if _, err := os.Lstat(fixture.created); !os.IsNotExist(err) {
		t.Fatalf("new output exists in prior state: %v", err)
	}
	assertFileBytes(t, fixture.inactive, fixture.inactiveBefore)
	assertFileBytes(t, fixture.sentinel, []byte("keep"))
	if runtime.GOOS != "windows" {
		assertFileMode(t, fixture.existing, 0o600)
		assertFileMode(t, fixture.inactive, 0o640)
	}
}

func (fixture generatedPublicationFixture) assertCommitted(t *testing.T) {
	t.Helper()
	assertFileBytes(t, fixture.existing, fixture.existingNext)
	assertFileBytes(t, fixture.created, fixture.createdNext)
	if _, err := os.Lstat(fixture.inactive); !os.IsNotExist(err) {
		t.Fatalf("inactive output remains after commit: %v", err)
	}
	assertFileBytes(t, fixture.sentinel, []byte("keep"))
	if runtime.GOOS != "windows" {
		assertFileMode(t, fixture.existing, 0o644)
		assertFileMode(t, fixture.created, 0o644)
	}
}

func writeGeneratedPublicationSource(
	t *testing.T,
	root,
	name,
	function,
	text string,
) {
	t.Helper()
	writeTestFile(t, root, name, `package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func `+function+`() gf.Node {
	return <main>`+text+`</main>
}
`)
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want.Perm() {
		t.Fatalf("%s mode = %s, want %s", path, got, want.Perm())
	}
}

func assertNoGeneratedPublicationArtifacts(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), ".goframe-publish-") ||
			strings.HasPrefix(entry.Name(), ".goframe-rollback-") {
			t.Errorf("transaction artifact remains: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

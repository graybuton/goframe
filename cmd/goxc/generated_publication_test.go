package main

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
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

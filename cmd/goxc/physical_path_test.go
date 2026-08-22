package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type modeledPhysicalFileInfo struct {
	os.FileInfo
	identity string
}

func TestPhysicalPathRelationDetectsModeledBindAliases(t *testing.T) {
	root := t.TempDir()
	packageRoot := filepath.Join(root, "workspace", "package")
	packageChild := filepath.Join(packageRoot, "nested")
	aliasRoot := filepath.Join(root, "mnt", "package")
	aliasChild := filepath.Join(aliasRoot, "nested")
	unrelated := filepath.Join(root, "mnt", "unrelated")
	commonFirst := filepath.Join(root, "common", "first")
	commonSecond := filepath.Join(root, "common", "second")
	for _, directory := range []string{
		packageChild,
		aliasChild,
		unrelated,
		commonFirst,
		commonSecond,
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	operations := modeledPhysicalPathOperations(t, map[string]string{
		packageRoot:  "package-root",
		aliasRoot:    "package-root",
		packageChild: "package-child",
		aliasChild:   "package-child",
	})
	tests := []struct {
		name   string
		first  string
		second string
		want   string
	}{
		{name: "same directory through alias", first: packageRoot, second: aliasRoot, want: "same"},
		{name: "candidate below bind alias", first: packageRoot, second: aliasChild, want: "contains"},
		{name: "root below bind alias", first: aliasChild, second: packageRoot, want: "inside"},
		{name: "missing tail below bind alias", first: packageRoot, second: filepath.Join(aliasChild, "missing", "tail"), want: "contains"},
		{name: "unrelated directories on same filesystem", first: packageRoot, second: unrelated, want: "separate"},
		{name: "ordinary common filesystem root only", first: commonFirst, second: commonSecond, want: "separate"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := physicalPathRelationWithOperations(test.first, test.second, operations)
			if err != nil {
				t.Fatalf("physicalPathRelationWithOperations() error: %v", err)
			}
			if got != test.want {
				t.Fatalf("physical path relation = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPhysicalPathRelationFailsClosedOnStatError(t *testing.T) {
	first := filepath.Join(t.TempDir(), "first")
	second := filepath.Join(t.TempDir(), "second")
	for _, directory := range []string{first, second} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	secondCanonical, err := canonicalPathForComparison(second)
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("identity unavailable")
	operations := defaultPhysicalPathOperations()
	operations.stat = func(path string) (os.FileInfo, error) {
		if filepath.Clean(path) == secondCanonical {
			return nil, want
		}
		return os.Stat(path)
	}
	if _, err := physicalPathRelationWithOperations(first, second, operations); !errors.Is(err, want) {
		t.Fatalf("physical path relation error = %v, want wrapped %v", err, want)
	}
}

func modeledPhysicalPathOperations(t *testing.T, identities map[string]string) physicalPathOperations {
	t.Helper()
	canonicalIdentities := make(map[string]string, len(identities))
	for path, identity := range identities {
		canonical, err := canonicalPathForComparison(path)
		if err != nil {
			t.Fatalf("canonicalize modeled path %s: %v", path, err)
		}
		canonicalIdentities[canonical] = identity
	}
	return physicalPathOperations{
		stat: func(path string) (os.FileInfo, error) {
			info, err := os.Stat(path)
			if err != nil {
				return nil, err
			}
			identity := filepath.Clean(path)
			if modeled, ok := canonicalIdentities[identity]; ok {
				identity = modeled
			}
			return modeledPhysicalFileInfo{FileInfo: info, identity: identity}, nil
		},
		sameFile: func(first, second os.FileInfo) bool {
			firstModeled, firstOK := first.(modeledPhysicalFileInfo)
			secondModeled, secondOK := second.(modeledPhysicalFileInfo)
			if !firstOK || !secondOK {
				t.Fatalf("modeled identity received unexpected file info types %T and %T", first, second)
			}
			return firstModeled.identity == secondModeled.identity
		},
	}
}

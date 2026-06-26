package gox

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "update GOX golden files")

func TestGolden(t *testing.T) {
	files, err := filepath.Glob("testdata/*.gox")
	if err != nil {
		t.Fatal(err)
	}
	for _, sourcePath := range files {
		name := strings.TrimSuffix(filepath.Base(sourcePath), ".gox")
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			displayPath := filepath.ToSlash(sourcePath)
			generated, err := GenerateNamed(displayPath, source)
			if err != nil {
				t.Fatalf("GenerateNamed() error: %v", err)
			}

			goldenPath := strings.TrimSuffix(sourcePath, ".gox") + ".golden.go"
			if *updateGolden {
				if err := os.WriteFile(goldenPath, generated, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			golden, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden file: %v; run go test ./pkg/gox -update", err)
			}
			got := normalizeGoldenText(generated)
			want := normalizeGoldenText(golden)
			if got != want {
				t.Fatalf("generated output differs from %s; run go test ./pkg/gox -update\n--- got ---\n%s\n--- want ---\n%s", goldenPath, got, want)
			}
		})
	}
}

func TestErrorGolden(t *testing.T) {
	files, err := filepath.Glob("testdata/errors/*.gox")
	if err != nil {
		t.Fatal(err)
	}
	for _, sourcePath := range files {
		name := strings.TrimSuffix(filepath.Base(sourcePath), ".gox")
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			displayPath := filepath.ToSlash(sourcePath)
			_, err = GenerateNamed(displayPath, source)
			if err == nil {
				t.Fatal("GenerateNamed() returned nil error")
			}
			goldenPath := strings.TrimSuffix(sourcePath, ".gox") + ".golden.txt"
			if *updateGolden {
				if writeErr := os.WriteFile(goldenPath, []byte(normalizeGoldenText([]byte(err.Error()+"\n"))), 0o644); writeErr != nil {
					t.Fatal(writeErr)
				}
			}
			golden, readErr := os.ReadFile(goldenPath)
			if readErr != nil {
				t.Fatalf("read error golden: %v; run go test ./pkg/gox -update", readErr)
			}
			got := normalizeGoldenText([]byte(err.Error() + "\n"))
			want := normalizeGoldenText(golden)
			if got != want {
				t.Fatalf("error differs from golden\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}

func normalizeGoldenText(content []byte) string {
	return strings.ReplaceAll(string(content), "\r\n", "\n")
}

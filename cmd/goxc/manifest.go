package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const manifestName = "goframe.json"

type projectManifest struct {
	Name     string   `json:"name"`
	Entry    string   `json:"entry"`
	Output   string   `json:"output"`
	Compiler string   `json:"compiler"`
	WASM     string   `json:"wasm"`
	Assets   []string `json:"assets"`
}

func loadManifest(appDir string) (projectManifest, error) {
	manifest := projectManifest{
		Name:     filepath.Base(filepath.Clean(appDir)),
		Entry:    ".",
		Output:   "dist",
		Compiler: "go",
		WASM:     "main.wasm",
		Assets:   []string{"index.html", "service-worker.js"},
	}

	content, err := os.ReadFile(filepath.Join(appDir, manifestName))
	if errors.Is(err, os.ErrNotExist) {
		return manifest, nil
	}
	if err != nil {
		return projectManifest{}, fmt.Errorf("read %s: %w", manifestName, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return projectManifest{}, fmt.Errorf("parse %s: %w", manifestName, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return projectManifest{}, fmt.Errorf("parse %s: unexpected trailing JSON data", manifestName)
	}

	if manifest.Name == "" {
		manifest.Name = filepath.Base(filepath.Clean(appDir))
	}
	if manifest.Entry == "" {
		manifest.Entry = "."
	}
	if manifest.Output == "" {
		manifest.Output = "dist"
	}
	if manifest.Compiler == "" {
		manifest.Compiler = "go"
	}
	if manifest.WASM == "" {
		manifest.WASM = "main.wasm"
	}
	if manifest.Assets == nil {
		manifest.Assets = []string{"index.html", "service-worker.js"}
	}

	if !safeRelativePath(manifest.Entry) {
		return projectManifest{}, fmt.Errorf("entry %q in %s must be a relative path inside the application", manifest.Entry, manifestName)
	}
	for name, value := range map[string]string{
		"output": manifest.Output,
		"wasm":   manifest.WASM,
	} {
		if !safeChildPath(value) {
			return projectManifest{}, fmt.Errorf("%s %q in %s must be a child path inside the application", name, value, manifestName)
		}
	}
	if manifest.Compiler != "go" && manifest.Compiler != "tinygo" {
		return projectManifest{}, fmt.Errorf("compiler %q in %s must be go or tinygo", manifest.Compiler, manifestName)
	}
	for _, asset := range manifest.Assets {
		if !safeChildPath(asset) {
			return projectManifest{}, fmt.Errorf("asset %q in %s must be a child path inside the application", asset, manifestName)
		}
	}
	return manifest, nil
}

func safeChildPath(path string) bool {
	return safeRelativePath(path) && filepath.Clean(path) != "."
}

func safeRelativePath(path string) bool {
	if path == "" || filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(path)
	return clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

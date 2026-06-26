package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
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
		WASM:     "bundle.wasm",
		Assets:   []string{"index.html"},
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
	if err := rejectExplicitEmptyEntry(content); err != nil {
		return projectManifest{}, err
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
		manifest.WASM = "bundle.wasm"
	}
	if manifest.Assets == nil {
		manifest.Assets = []string{"index.html"}
	}

	entry, err := cleanManifestEntry(manifest.Entry)
	if err != nil {
		return projectManifest{}, fmt.Errorf("entry %q in %s %s", manifest.Entry, manifestName, err)
	}
	manifest.Entry = entry
	for name, value := range map[string]string{
		"output": manifest.Output,
		"wasm":   manifest.WASM,
	} {
		if !safeChildPath(value) {
			return projectManifest{}, fmt.Errorf("%s %q in %s must be a child path inside the application", name, value, manifestName)
		}
	}
	if strings.ToLower(filepath.Ext(manifest.WASM)) != ".wasm" {
		return projectManifest{}, fmt.Errorf("wasm %q in %s must end with .wasm", manifest.WASM, manifestName)
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

func rejectExplicitEmptyEntry(content []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(content, &raw); err != nil {
		return err
	}
	value, ok := raw["entry"]
	if !ok {
		return nil
	}
	var entry string
	if err := json.Unmarshal(value, &entry); err != nil {
		return nil
	}
	if entry == "" {
		return fmt.Errorf("entry %q in %s must be a relative child package inside the application", entry, manifestName)
	}
	return nil
}

func cleanManifestEntry(entry string) (string, error) {
	logicalEntry := manifestPath(entry)
	if logicalEntry == "." {
		return ".", nil
	}
	if entry == "" || manifestPathIsAbs(entry) {
		return "", fmt.Errorf("must be a relative child package inside the application")
	}
	rawParts := strings.Split(logicalEntry, "/")
	for _, part := range rawParts {
		if part == ".." {
			return "", fmt.Errorf("must be a relative child package inside the application")
		}
	}
	entry = path.Clean(logicalEntry)
	if entry == "." {
		return ".", nil
	}
	parts := strings.Split(entry, "/")
	for _, part := range parts {
		if part == ".." {
			return "", fmt.Errorf("must be a relative child package inside the application")
		}
	}
	if strings.HasPrefix(entry, "../") || entry == ".." {
		return "", fmt.Errorf("must be a relative child package inside the application")
	}
	if isToolOwnedEntryRoot(parts[0]) {
		return "", fmt.Errorf("points to a GoFrame-owned or tool-owned directory")
	}
	return entry, nil
}

func isToolOwnedEntryRoot(root string) bool {
	switch root {
	case defaultWorkspaceName, "build", "dist", "node_modules", ".git", ".goxc-tmp":
		return true
	default:
		return false
	}
}

func safeChildPath(value string) bool {
	return safeRelativePath(value) && path.Clean(manifestPath(value)) != "."
}

func safeRelativePath(value string) bool {
	if value == "" || manifestPathIsAbs(value) {
		return false
	}
	for _, part := range strings.Split(manifestPath(value), "/") {
		if part == ".." {
			return false
		}
	}
	clean := path.Clean(manifestPath(value))
	return clean != ".." && !strings.HasPrefix(clean, "../")
}

func manifestPath(value string) string {
	return strings.ReplaceAll(filepath.ToSlash(value), "\\", "/")
}

func manifestPathIsAbs(value string) bool {
	logical := manifestPath(value)
	if strings.HasPrefix(logical, "/") || filepath.IsAbs(value) {
		return true
	}
	if len(logical) >= 2 && logical[1] == ':' {
		drive := logical[0]
		return (drive >= 'a' && drive <= 'z') || (drive >= 'A' && drive <= 'Z')
	}
	return false
}

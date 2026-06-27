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
	Name     string         `json:"name"`
	Entry    string         `json:"entry"`
	Output   string         `json:"output"`
	Compiler string         `json:"compiler"`
	WASM     string         `json:"wasm"`
	Assets   manifestAssets `json:"assets"`
}

type manifestAssetMode uint8

const (
	manifestAssetsAuto manifestAssetMode = iota
	manifestAssetsDirectory
	manifestAssetsList
)

type manifestAssets struct {
	Mode      manifestAssetMode
	Directory string
	List      []string
}

func autoManifestAssets() manifestAssets {
	return manifestAssets{Mode: manifestAssetsAuto}
}

func directoryManifestAssets(directory string) manifestAssets {
	return manifestAssets{Mode: manifestAssetsDirectory, Directory: directory}
}

func listManifestAssets(assets []string) manifestAssets {
	return manifestAssets{Mode: manifestAssetsList, List: assets}
}

func (assets *manifestAssets) UnmarshalJSON(content []byte) error {
	content = bytes.TrimSpace(content)
	if bytes.Equal(content, []byte("null")) {
		*assets = autoManifestAssets()
		return nil
	}
	var directory string
	if err := json.Unmarshal(content, &directory); err == nil {
		*assets = directoryManifestAssets(directory)
		return nil
	}
	var list []string
	if err := json.Unmarshal(content, &list); err == nil {
		*assets = listManifestAssets(list)
		return nil
	}
	return fmt.Errorf("assets must be a string directory, array of paths, null, or omitted")
}

func loadManifest(appDir string) (projectManifest, error) {
	manifest := projectManifest{
		Name:     filepath.Base(filepath.Clean(appDir)),
		Entry:    ".",
		Output:   "dist",
		Compiler: "go",
		WASM:     "bundle.wasm",
		Assets:   autoManifestAssets(),
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
	switch manifest.Assets.Mode {
	case manifestAssetsAuto:
	case manifestAssetsDirectory:
		directory, err := cleanManifestAssetDirectory(manifest.Assets.Directory)
		if err != nil {
			return projectManifest{}, fmt.Errorf("assets %q in %s %s", manifest.Assets.Directory, manifestName, err)
		}
		manifest.Assets.Directory = directory
	case manifestAssetsList:
		for index, asset := range manifest.Assets.List {
			cleaned, err := cleanManifestAssetPath(asset)
			if err != nil {
				return projectManifest{}, fmt.Errorf("asset %q in %s %s", asset, manifestName, err)
			}
			manifest.Assets.List[index] = cleaned
		}
	default:
		return projectManifest{}, fmt.Errorf("assets in %s has unsupported internal mode", manifestName)
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

func cleanManifestAssetDirectory(directory string) (string, error) {
	cleaned, err := cleanManifestAssetPath(directory)
	if err != nil {
		return "", err
	}
	if cleaned == indexHTMLAssetName {
		return "", fmt.Errorf("must be a relative child directory, not a file")
	}
	return cleaned, nil
}

func cleanManifestAssetPath(value string) (string, error) {
	if !safeChildPath(value) {
		return "", fmt.Errorf("must be a child path inside the application")
	}
	cleaned := path.Clean(manifestPath(value))
	parts := strings.Split(cleaned, "/")
	if len(parts) > 0 && isToolOwnedEntryRoot(parts[0]) {
		return "", fmt.Errorf("points to a GoFrame-owned or tool-owned directory")
	}
	return cleaned, nil
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

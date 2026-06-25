package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
)

const (
	defaultWorkspaceName = ".goframe"
	defaultProfileName   = "dev"
	releaseProfileName   = "release"
	standalonePackage    = "standalone"
	canonicalModulePath  = "github.com/graybuton/goframe"
)

type BuildLayout struct {
	AppDir string

	WorkspaceRoot     string
	WorkspaceBase     string
	ExternalWorkspace bool

	GenDir     string
	WorkDir    string
	BuildDir   string
	PackageDir string
	CacheDir   string
	LogsDir    string

	Compiler string
	Profile  string
}

type layoutOptions struct {
	appDir    string
	compiler  string
	profile   string
	workspace string
}

func newBuildLayout(options layoutOptions) (BuildLayout, error) {
	appDir, err := filepath.Abs(options.appDir)
	if err != nil {
		return BuildLayout{}, fmt.Errorf("resolve application directory: %w", err)
	}
	profile := options.profile
	if profile == "" {
		profile = defaultProfileName
	}
	compiler := options.compiler
	if compiler == "" {
		compiler = "go"
	}

	workspaceRoot := options.workspace
	externalWorkspace := false
	if workspaceRoot == "" {
		workspaceRoot = os.Getenv("GOFRAME_WORKSPACE")
	}
	if workspaceRoot == "" {
		workspaceRoot = filepath.Join(appDir, defaultWorkspaceName)
	} else {
		externalWorkspace = true
		workspaceRoot, err = filepath.Abs(workspaceRoot)
		if err != nil {
			return BuildLayout{}, fmt.Errorf("resolve workspace directory: %w", err)
		}
		if pathsOverlap(appDir, workspaceRoot) {
			return BuildLayout{}, fmt.Errorf("workspace directory %s must not overlap application directory %s", workspaceRoot, appDir)
		}
		workspaceRoot = filepath.Join(workspaceRoot, appWorkspaceSlug(appDir))
		if pathsOverlap(appDir, workspaceRoot) {
			return BuildLayout{}, fmt.Errorf("workspace root %s must not overlap application directory %s", workspaceRoot, appDir)
		}
	}

	return BuildLayout{
		AppDir:            appDir,
		WorkspaceRoot:     workspaceRoot,
		WorkspaceBase:     filepath.Dir(workspaceRoot),
		ExternalWorkspace: externalWorkspace,
		GenDir:            filepath.Join(workspaceRoot, "gen"),
		WorkDir:           filepath.Join(workspaceRoot, "work", profile),
		BuildDir:          filepath.Join(workspaceRoot, "build", compiler, profile),
		PackageDir:        filepath.Join(workspaceRoot, "package", standalonePackage),
		CacheDir:          filepath.Join(workspaceRoot, "cache"),
		LogsDir:           filepath.Join(workspaceRoot, "logs"),
		Compiler:          compiler,
		Profile:           profile,
	}, nil
}

func appWorkspaceSlug(appDir string) string {
	base := filepath.Base(filepath.Clean(appDir))
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = "app"
	}
	base = sanitizeSlug(base)
	sum := sha256.Sum256([]byte(appDir))
	return base + "-" + hex.EncodeToString(sum[:])[:8]
}

func sanitizeSlug(value string) string {
	value = strings.ToLower(value)
	replacer := regexp.MustCompile(`[^a-z0-9._-]+`)
	value = strings.Trim(replacer.ReplaceAllString(value, "-"), "-._")
	if value == "" {
		return "app"
	}
	return value
}

func packageProfile(assetHash, preload bool, compress map[string]bool) string {
	if assetHash || preload || len(compress) > 0 {
		return releaseProfileName
	}
	return defaultProfileName
}

func goframeModuleVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Path == canonicalModulePath && info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
		for _, module := range info.Deps {
			if module != nil && module.Path == canonicalModulePath && module.Version != "" && module.Version != "(devel)" {
				return module.Version
			}
		}
	}
	return "v0.0.0"
}

func findRepositoryRoot(start string) (string, bool) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		content, err := os.ReadFile(filepath.Join(current, "go.mod"))
		if err == nil && strings.Contains(string(content), "module "+canonicalModulePath) {
			return current, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

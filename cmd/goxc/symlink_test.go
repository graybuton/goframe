package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func requireSymlinkSupport(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on Windows")
	}
	root := t.TempDir()
	target := filepath.Join(root, "target")
	link := filepath.Join(root, "link")
	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not available: %v", err)
	}
}

func TestResolveEntryPackageDirRejectsSymlinkedEntry(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	externalEntry := filepath.Join(root, "external-entry")
	if err := os.MkdirAll(externalEntry, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalEntry, filepath.Join(appDir, "cmd")); err != nil {
		t.Fatal(err)
	}

	if _, err := resolveEntryPackageDir(appDir, "cmd"); err == nil {
		t.Fatal("resolveEntryPackageDir() accepted a symlinked entry")
	}
}

func TestFindGOXFilesRejectsSymlinkedGOXSource(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	externalSource := filepath.Join(root, "external.gox")
	if err := os.WriteFile(externalSource, []byte("package main\nfunc App() any { return <div></div> }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalSource, filepath.Join(appDir, "app.gox")); err != nil {
		t.Fatal(err)
	}

	if _, err := findGOXFiles(appDir); err == nil {
		t.Fatal("findGOXFiles() accepted a symlinked GOX source")
	}
}

func TestCopyAuthoredGoFilesRejectsSymlinkedGoSource(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	externalSource := filepath.Join(root, "external.go")
	if err := os.WriteFile(externalSource, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalSource, filepath.Join(appDir, "main.go")); err != nil {
		t.Fatal(err)
	}

	if err := copyAuthoredGoFiles(appDir, filepath.Join(t.TempDir(), "work")); err == nil {
		t.Fatal("copyAuthoredGoFiles() accepted a symlinked Go source")
	}
}

func TestResolvePackageAssetSourceRejectsSymlinkedAsset(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	externalAsset := filepath.Join(root, "secret.css")
	if err := os.WriteFile(externalAsset, []byte("body{color:red}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalAsset, filepath.Join(appDir, "styles.css")); err != nil {
		t.Fatal(err)
	}

	if _, _, _, err := resolvePackageAssetSource(appDir, "styles.css"); err == nil {
		t.Fatal("resolvePackageAssetSource() accepted a symlinked asset")
	}
}

func TestResolvePackageAssetSourceRejectsBrokenSymlink(t *testing.T) {
	requireSymlinkSupport(t)

	appDir := t.TempDir()
	if err := os.Symlink(filepath.Join(appDir, "missing.css"), filepath.Join(appDir, "styles.css")); err != nil {
		t.Fatal(err)
	}

	if _, _, _, err := resolvePackageAssetSource(appDir, "styles.css"); err == nil {
		t.Fatal("resolvePackageAssetSource() accepted a broken symlink")
	}
}

func TestValidatePackageDestinationRejectsSymlinkDirectory(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, packageMetadataName), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "package-link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := validatePackageDestination(link); err == nil {
		t.Fatal("validatePackageDestination() accepted a symlinked output directory")
	}
}

func TestValidateExportDestinationRejectsSymlinkDirectory(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, packageMetadataName), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "export-link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := validateExportDestination(link, true); err == nil {
		t.Fatal("validateExportDestination() accepted a symlinked output directory")
	}
}

func TestCleanDoesNotTraverseWorkspaceSymlinkTarget(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(filepath.Join(appDir, defaultWorkspaceName), 0o755); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(root, "external-package")
	if err := os.MkdirAll(external, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(external, "keep.txt")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(appDir, defaultWorkspaceName, "package")); err != nil {
		t.Fatal(err)
	}

	if err := cleanApp(cleanOptions{appDir: appDir}); err != nil {
		t.Fatalf("cleanApp() error: %v", err)
	}
	if content, err := os.ReadFile(keep); err != nil || string(content) != "keep" {
		t.Fatalf("clean traversed symlink target: content=%q err=%v", content, err)
	}
	if _, err := os.Lstat(filepath.Join(appDir, defaultWorkspaceName, "package")); !os.IsNotExist(err) {
		t.Fatalf("workspace package symlink still exists: %v", err)
	}
}

func TestExternalWorkspaceStillWorks(t *testing.T) {
	appDir := filepath.Join(t.TempDir(), "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	externalWorkspace := filepath.Join(t.TempDir(), "workspace")

	layout, err := newBuildLayout(layoutOptions{appDir: appDir, workspace: externalWorkspace})
	if err != nil {
		t.Fatalf("newBuildLayout(external workspace) error: %v", err)
	}
	if got := filepath.Dir(layout.WorkspaceRoot); got != externalWorkspace {
		t.Fatalf("workspace root = %q, want child of %q", layout.WorkspaceRoot, externalWorkspace)
	}
}

package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testIndex = `<!doctype html>
<html>
<head><title>fixture</title></head>
<body data-history-package-index="true">fixture index</body>
</html>
`

func TestPackageServerStrictAndFallbackModes(t *testing.T) {
	packageDir := writeTestPackage(t, testIndex)
	strict := newTestPackageServer(t, packageDir, serverModeStrict, "/")
	fallback := newTestPackageServer(t, packageDir, serverModeFallback, "/")

	tests := []struct {
		name        string
		handler     http.Handler
		method      string
		target      string
		accept      string
		status      int
		class       string
		contains    string
		notContains string
	}{
		{name: "strict root", handler: strict, method: http.MethodGet, target: "/", status: http.StatusOK, class: "index", contains: `<base href="/" />`},
		{name: "strict deep route", handler: strict, method: http.MethodGet, target: "/users/42", accept: "text/html", status: http.StatusNotFound, class: "not-found", notContains: "fixture index"},
		{name: "fallback deep route", handler: fallback, method: http.MethodGet, target: "/users/42", accept: "text/html", status: http.StatusOK, class: "fallback", contains: "fixture index"},
		{name: "fallback trailing slash", handler: fallback, method: http.MethodGet, target: "/settings/", accept: "text/html", status: http.StatusOK, class: "fallback", contains: "fixture index"},
		{name: "fallback requires html accept", handler: fallback, method: http.MethodGet, target: "/users/42", accept: "application/json", status: http.StatusNotFound, class: "not-found", notContains: "fixture index"},
		{name: "head fallback", handler: fallback, method: http.MethodHead, target: "/users/42", accept: "text/html", status: http.StatusOK, class: "fallback", notContains: "fixture index"},
		{name: "unsupported method", handler: fallback, method: http.MethodPost, target: "/users/42", accept: "text/html", status: http.StatusMethodNotAllowed},
		{name: "existing css", handler: fallback, method: http.MethodGet, target: "/assets/styles.css", status: http.StatusOK, class: "static", contains: "fixture-style"},
		{name: "existing wasm", handler: fallback, method: http.MethodGet, target: "/assets/bundle.wasm", status: http.StatusOK, class: "static", contains: "wasm"},
		{name: "missing asset", handler: fallback, method: http.MethodGet, target: "/assets/missing.js", accept: "text/html", status: http.StatusNotFound, class: "not-found", notContains: "fixture index"},
		{name: "missing root asset", handler: fallback, method: http.MethodGet, target: "/missing.wasm", accept: "text/html", status: http.StatusNotFound, class: "not-found", notContains: "fixture index"},
		{name: "api path", handler: fallback, method: http.MethodGet, target: "/api/missing", accept: "text/html", status: http.StatusNotFound, class: "not-found", notContains: "fixture index"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := requestServer(t, test.handler, test.method, test.target, test.accept)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body=%q", response.Code, test.status, response.Body.String())
			}
			if test.class != "" && response.Header().Get(responseClassHeader) != test.class {
				t.Fatalf("%s = %q, want %q", responseClassHeader, response.Header().Get(responseClassHeader), test.class)
			}
			if test.contains != "" && !strings.Contains(response.Body.String(), test.contains) {
				t.Fatalf("body %q does not contain %q", response.Body.String(), test.contains)
			}
			if test.notContains != "" && strings.Contains(response.Body.String(), test.notContains) {
				t.Fatalf("body unexpectedly contains %q", test.notContains)
			}
		})
	}

	css := requestServer(t, fallback, http.MethodGet, "/assets/styles.css", "")
	if got := css.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/css") {
		t.Fatalf("CSS Content-Type = %q", got)
	}
	wasm := requestServer(t, fallback, http.MethodGet, "/assets/bundle.wasm", "")
	if got := wasm.Header().Get("Content-Type"); got != "application/wasm" {
		t.Fatalf("WASM Content-Type = %q", got)
	}
}

func TestPackageServerSubpathAndTraversal(t *testing.T) {
	packageDir := writeTestPackage(t, testIndex)
	server := newTestPackageServer(t, packageDir, serverModeFallback, "app")

	tests := []struct {
		target string
		accept string
		status int
		class  string
	}{
		{target: "/app/", status: http.StatusOK, class: "index"},
		{target: "/app/users/42", accept: "text/html", status: http.StatusOK, class: "fallback"},
		{target: "/app/assets/styles.css", status: http.StatusOK, class: "static"},
		{target: "/app/assets/missing.js", accept: "text/html", status: http.StatusNotFound, class: "not-found"},
		{target: "/app/api/missing", accept: "text/html", status: http.StatusNotFound, class: "not-found"},
		{target: "/users/42", accept: "text/html", status: http.StatusNotFound, class: "not-found"},
		{target: "/application/users/42", accept: "text/html", status: http.StatusNotFound, class: "not-found"},
		{target: "/app/%2e%2e/secret", accept: "text/html", status: http.StatusNotFound, class: "not-found"},
	}
	for _, test := range tests {
		t.Run(test.target, func(t *testing.T) {
			response := requestServer(t, server, http.MethodGet, test.target, test.accept)
			if response.Code != test.status || response.Header().Get(responseClassHeader) != test.class {
				t.Fatalf("response = %d %q, want %d %q; body=%q", response.Code, response.Header().Get(responseClassHeader), test.status, test.class, response.Body.String())
			}
		})
	}

	response := requestServer(t, server, http.MethodGet, "/app/users/42", "text/html")
	if !strings.Contains(response.Body.String(), `<base href="/app/" />`) {
		t.Fatalf("subpath response missing document base: %q", response.Body.String())
	}
}

func TestPackageServerIndexTransformationDoesNotMutatePackage(t *testing.T) {
	packageDir := writeTestPackage(t, testIndex)
	before, err := os.ReadFile(filepath.Join(packageDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	server := newTestPackageServer(t, packageDir, serverModeFallback, "/app/")

	for _, target := range []string{"/app/", "/app/users/42"} {
		response := requestServer(t, server, http.MethodGet, target, "text/html")
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d", target, response.Code)
		}
		if count := strings.Count(response.Body.String(), "<base "); count != 1 {
			t.Fatalf("%s base element count = %d", target, count)
		}
	}

	after, err := os.ReadFile(filepath.Join(packageDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("package index changed on disk")
	}
}

func TestPackageServerRejectsInvalidIndexAndPackageInputs(t *testing.T) {
	t.Run("missing index", func(t *testing.T) {
		packageDir := t.TempDir()
		if _, err := newPackageServer(serverConfig{PackageDir: packageDir, Mode: serverModeStrict, Base: "/"}); err == nil || !strings.Contains(err.Error(), "package index") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("irregular index", func(t *testing.T) {
		packageDir := t.TempDir()
		if err := os.Mkdir(filepath.Join(packageDir, "index.html"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := newPackageServer(serverConfig{PackageDir: packageDir, Mode: serverModeStrict, Base: "/"}); err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("malformed head", func(t *testing.T) {
		packageDir := writeTestPackage(t, "<html><body>missing head</body></html>")
		if _, err := newPackageServer(serverConfig{PackageDir: packageDir, Mode: serverModeStrict, Base: "/"}); err == nil || !strings.Contains(err.Error(), "complete head") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("existing base", func(t *testing.T) {
		packageDir := writeTestPackage(t, `<html><head><base href="/already/"></head><body></body></html>`)
		if _, err := newPackageServer(serverConfig{PackageDir: packageDir, Mode: serverModeStrict, Base: "/"}); err == nil || !strings.Contains(err.Error(), "already contains a base") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("symlink package root", func(t *testing.T) {
		realRoot := writeTestPackage(t, testIndex)
		alias := filepath.Join(t.TempDir(), "package-alias")
		if err := os.Symlink(realRoot, alias); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := newPackageServer(serverConfig{PackageDir: alias, Mode: serverModeStrict, Base: "/"}); err == nil || !strings.Contains(err.Error(), "is a symlink") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestPackageServerDoesNotFollowStaticSymlinks(t *testing.T) {
	packageDir := writeTestPackage(t, testIndex)
	external := filepath.Join(t.TempDir(), "external.js")
	if err := os.WriteFile(external, []byte("external-secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(packageDir, "assets", "linked.js")
	if err := os.Symlink(external, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	server := newTestPackageServer(t, packageDir, serverModeFallback, "/")
	response := requestServer(t, server, http.MethodGet, "/assets/linked.js", "text/html")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
	if strings.Contains(response.Body.String(), "external-secret") {
		t.Fatal("external symlink target was served")
	}
}

func writeTestPackage(t *testing.T, index string) string {
	t.Helper()
	packageDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(packageDir, "index.html"), []byte(index), 0o644); err != nil {
		t.Fatal(err)
	}
	assets := filepath.Join(packageDir, "assets")
	if err := os.Mkdir(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "styles.css"), []byte("fixture-style"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "bundle.wasm"), []byte("wasm"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "wasm_exec.js"), []byte("runtime"), 0o644); err != nil {
		t.Fatal(err)
	}
	return packageDir
}

func newTestPackageServer(t *testing.T, packageDir string, mode serverMode, base string) *packageServer {
	t.Helper()
	server, err := newPackageServer(serverConfig{PackageDir: packageDir, Mode: mode, Base: base})
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func requestServer(t *testing.T, handler http.Handler, method, target, accept string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, "http://fixture.test"+target, nil)
	if accept != "" {
		request.Header.Set("Accept", accept)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code == http.StatusOK && method != http.MethodHead {
		result := response.Result()
		defer result.Body.Close()
		if _, err := io.ReadAll(result.Body); err != nil {
			t.Fatal(err)
		}
	}
	return response
}

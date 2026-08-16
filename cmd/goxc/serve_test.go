package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestStaticHandlerServesRootAndStaticFiles(t *testing.T) {
	directory := t.TempDir()
	writeServeFixture(t, directory, "index.html", "root")
	writeServeFixture(t, directory, "assets/app.js", "console.log('ok')")

	handler := staticHandler(directory)

	rootResponse := httptest.NewRecorder()
	handler.ServeHTTP(rootResponse, httptest.NewRequest("GET", "/", nil))
	if rootResponse.Code != http.StatusOK {
		t.Fatalf("root status = %d, want 200", rootResponse.Code)
	}
	if got := rootResponse.Body.String(); got != "root" {
		t.Fatalf("root body = %q, want root", got)
	}

	fileResponse := httptest.NewRecorder()
	handler.ServeHTTP(fileResponse, httptest.NewRequest("GET", "/assets/app.js?cache=ignored", nil))
	if fileResponse.Code != http.StatusOK {
		t.Fatalf("static file status = %d, want 200", fileResponse.Code)
	}
	if got := fileResponse.Body.String(); got != "console.log('ok')" {
		t.Fatalf("static file body = %q, want app fixture", got)
	}
	if got := fileResponse.Header().Get("Content-Type"); got != "text/javascript" {
		t.Fatalf("Content-Type = %q, want text/javascript", got)
	}
}

func TestStaticHandlerServesCompressedWASMHeaders(t *testing.T) {
	directory := t.TempDir()
	writeServeFixture(t, directory, "assets/bundle.wasm.gz", "gzip")
	writeServeFixture(t, directory, "assets/bundle.wasm.br", "brotli")

	for _, tt := range []struct {
		name     string
		path     string
		encoding string
		body     string
	}{
		{name: "gzip", path: "/assets/bundle.wasm.gz", encoding: "gzip", body: "gzip"},
		{name: "brotli", path: "/assets/bundle.wasm.br", encoding: "br", body: "brotli"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			staticHandler(directory).ServeHTTP(response, httptest.NewRequest("GET", tt.path, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", response.Code)
			}
			if got := response.Header().Get("Content-Encoding"); got != tt.encoding {
				t.Fatalf("Content-Encoding = %q, want %q", got, tt.encoding)
			}
			if got := response.Header().Get("Content-Type"); got != "application/wasm" {
				t.Fatalf("Content-Type = %q, want application/wasm", got)
			}
			if got := response.Body.String(); got != tt.body {
				t.Fatalf("body = %q, want %q", got, tt.body)
			}
		})
	}
}

func TestServeContentTypeUsesASCIIOnlyExtensions(t *testing.T) {
	directory := t.TempDir()
	for name, content := range map[string]string{
		"assets/APP.WASM":    "uppercase wasm",
		"assets/RUNTIME.JS":  "uppercase javascript",
		"assets/THEME.CSS":   "uppercase css",
		"assets/APP.WASM.gz": "compressed uppercase wasm",
		"assets/app.waſm":    "confusable wasm",
		"assets/runtime.jſ":  "confusable javascript",
		"assets/theme.csſ":   "confusable css",
	} {
		writeServeFixture(t, directory, name, content)
	}

	for _, test := range []struct {
		name         string
		requestPath  string
		wantType     string
		wantEncoding string
	}{
		{name: "uppercase WASM", requestPath: "/assets/APP.WASM", wantType: "application/wasm"},
		{name: "uppercase JavaScript", requestPath: "/assets/RUNTIME.JS", wantType: "text/javascript"},
		{name: "uppercase CSS", requestPath: "/assets/THEME.CSS", wantType: "text/css"},
		{name: "compressed uppercase WASM", requestPath: "/assets/APP.WASM.gz", wantType: "application/wasm", wantEncoding: "gzip"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			staticHandler(directory).ServeHTTP(response, httptest.NewRequest("GET", test.requestPath, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", response.Code)
			}
			if got := response.Header().Get("Content-Type"); got != test.wantType {
				t.Fatalf("Content-Type = %q, want %q", got, test.wantType)
			}
			if got := response.Header().Get("Content-Encoding"); got != test.wantEncoding {
				t.Fatalf("Content-Encoding = %q, want %q", got, test.wantEncoding)
			}
		})
	}

	for _, test := range []struct {
		name        string
		requestPath string
		specialType string
	}{
		{name: "WASM long s", requestPath: "/assets/app.waſm", specialType: "application/wasm"},
		{name: "JavaScript long s", requestPath: "/assets/runtime.jſ", specialType: "text/javascript"},
		{name: "CSS long s", requestPath: "/assets/theme.csſ", specialType: "text/css"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			staticHandler(directory).ServeHTTP(response, httptest.NewRequest("GET", test.requestPath, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", response.Code)
			}
			if got := response.Header().Get("Content-Type"); got == test.specialType {
				t.Fatalf("Unicode lookalike received special Content-Type %q", got)
			}
		})
	}
}

func TestStaticHandlerRejectsTraversalPaths(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "serve")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	writeServeFixture(t, directory, "public.txt", "public")
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, target := range []string{
		"/../secret.txt",
		"/%2e%2e/secret.txt",
		"/assets/../public.txt",
	} {
		t.Run(target, func(t *testing.T) {
			response := httptest.NewRecorder()
			staticHandler(directory).ServeHTTP(response, httptest.NewRequest("GET", target, nil))
			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", response.Code)
			}
			if got := response.Body.String(); got == "secret" || got == "public" {
				t.Fatalf("served body for rejected path: %q", got)
			}
		})
	}
}

func TestStaticHandlerRejectsBackslashPaths(t *testing.T) {
	directory := t.TempDir()
	writeServeFixture(t, directory, "public.txt", "public")

	// The "\\" escape in this Go string literal is one URL backslash rune.
	const oneBackslashPath = "/..\\secret.txt"
	assertSingleBackslashFixture(t, oneBackslashPath)

	for _, tt := range []struct {
		name   string
		mutate func(*http.Request)
	}{
		{
			name: "decoded path",
			mutate: func(request *http.Request) {
				request.URL.Path = oneBackslashPath
			},
		},
		{
			name: "raw path",
			mutate: func(request *http.Request) {
				request.URL.RawPath = oneBackslashPath
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "/public.txt", nil)
			tt.mutate(request)
			response := httptest.NewRecorder()
			staticHandler(directory).ServeHTTP(response, request)
			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", response.Code)
			}
		})
	}
}

func TestStaticHandlerRejectsSymlinkEntry(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "serve")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(root, "external.txt")
	if err := os.WriteFile(external, []byte("external"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(directory, "link.txt")); err != nil {
		t.Skipf("symlink creation is not available: %v", err)
	}

	response := httptest.NewRecorder()
	staticHandler(directory).ServeHTTP(response, httptest.NewRequest("GET", "/link.txt", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("symlink entry status = %d, want 404", response.Code)
	}
	if got := response.Body.String(); got == "external" {
		t.Fatalf("served symlink target body: %q", got)
	}
}

func assertSingleBackslashFixture(t *testing.T, value string) {
	t.Helper()
	count := 0
	for _, r := range value {
		if r == '\\' {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("test fixture must contain exactly one backslash rune, got %d in %q", count, value)
	}
}

func writeServeFixture(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

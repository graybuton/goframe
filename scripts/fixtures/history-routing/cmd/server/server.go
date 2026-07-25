package main

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"mime"
	"net/http"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/graybuton/goframe/scripts/fixtures/history-routing/internal/historyroute"
)

type serverMode string

const (
	serverModeStrict   serverMode = "strict"
	serverModeFallback serverMode = "fallback"

	responseClassHeader = "X-GoFrame-History-Response"
)

type serverConfig struct {
	PackageDir string
	Mode       serverMode
	Base       string
}

type packageServer struct {
	packageDir string
	mode       serverMode
	base       string
	index      []byte
}

func newPackageServer(config serverConfig) (*packageServer, error) {
	switch config.Mode {
	case serverModeStrict, serverModeFallback:
	default:
		return nil, fmt.Errorf("unsupported server mode %q", config.Mode)
	}
	base, err := historyroute.NormalizeBase(config.Base)
	if err != nil {
		return nil, fmt.Errorf("normalize deployment base: %w", err)
	}
	packageDir, err := filepath.Abs(config.PackageDir)
	if err != nil {
		return nil, fmt.Errorf("resolve package directory: %w", err)
	}
	info, err := os.Lstat(packageDir)
	if err != nil {
		return nil, fmt.Errorf("inspect package directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("package directory %s is a symlink", packageDir)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("package directory %s is not a directory", packageDir)
	}

	indexPath := filepath.Join(packageDir, "index.html")
	indexInfo, err := os.Lstat(indexPath)
	if err != nil {
		return nil, fmt.Errorf("inspect package index: %w", err)
	}
	if indexInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("package index %s is a symlink", indexPath)
	}
	if !indexInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("package index %s is not a regular file", indexPath)
	}
	index, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("read package index: %w", err)
	}
	index, err = injectDocumentBase(index, base)
	if err != nil {
		return nil, err
	}

	return &packageServer{
		packageDir: packageDir,
		mode:       config.Mode,
		base:       base,
		index:      index,
	}, nil
}

func (server *packageServer) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request == nil || request.URL == nil {
		server.notFound(response, request)
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	requestPath, err := cleanRequestPath(request.URL)
	if err != nil {
		server.notFound(response, request)
		return
	}
	relative, inside := relativeRequestPath(server.base, requestPath)
	if !inside {
		server.notFound(response, request)
		return
	}
	if relative == "" || relative == "index.html" {
		server.serveIndex(response, request, "index")
		return
	}

	filePath, exists := server.staticFile(relative)
	if exists {
		server.serveStatic(response, request, filePath, relative)
		return
	}

	if server.mode == serverModeFallback && allowsHTMLFallback(request, requestPath, relative) {
		server.serveIndex(response, request, "fallback")
		return
	}
	server.notFound(response, request)
}

func (server *packageServer) staticFile(relative string) (string, bool) {
	parts := strings.Split(filepath.ToSlash(relative), "/")
	current := server.packageDir
	for index, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", false
		}
		current = filepath.Join(current, filepath.FromSlash(part))
		info, err := os.Lstat(current)
		if err != nil {
			return "", false
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", false
		}
		if index < len(parts)-1 {
			if !info.IsDir() {
				return "", false
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return "", false
		}
	}
	return current, true
}

func (server *packageServer) serveIndex(response http.ResponseWriter, request *http.Request, class string) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set(responseClassHeader, class)
	writeResponse(response, request, server.index)
}

func (server *packageServer) serveStatic(response http.ResponseWriter, request *http.Request, filePath, relative string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		server.notFound(response, request)
		return
	}
	response.Header().Set("Content-Type", staticContentType(relative))
	response.Header().Set(responseClassHeader, "static")
	writeResponse(response, request, content)
}

func (server *packageServer) notFound(response http.ResponseWriter, request *http.Request) {
	response.Header().Set(responseClassHeader, "not-found")
	if request == nil {
		response.WriteHeader(http.StatusNotFound)
		return
	}
	http.NotFound(response, request)
}

func writeResponse(response http.ResponseWriter, request *http.Request, content []byte) {
	response.Header().Set("Content-Length", strconv.Itoa(len(content)))
	response.WriteHeader(http.StatusOK)
	if request.Method != http.MethodHead {
		_, _ = response.Write(content)
	}
}

func cleanRequestPath(requestURL *url.URL) (string, error) {
	escaped := requestURL.EscapedPath()
	decoded, err := url.PathUnescape(escaped)
	if err != nil {
		return "", errors.New("request path contains malformed escaping")
	}
	if decoded == "" {
		decoded = "/"
	}
	if !strings.HasPrefix(decoded, "/") {
		return "", errors.New("request path must start with slash")
	}
	if strings.ContainsRune(decoded, '\\') || strings.ContainsRune(requestURL.RawPath, '\\') {
		return "", errors.New("request path contains backslash")
	}
	for _, part := range strings.Split(decoded, "/") {
		if part == "." || part == ".." {
			return "", errors.New("request path contains traversal")
		}
	}
	cleaned := pathpkg.Clean(decoded)
	if cleaned == "." {
		cleaned = "/"
	}
	if strings.HasSuffix(decoded, "/") && cleaned != "/" {
		cleaned += "/"
	}
	return cleaned, nil
}

func relativeRequestPath(base, requestPath string) (string, bool) {
	if base == "/" {
		return strings.TrimPrefix(requestPath, "/"), true
	}
	if requestPath == base {
		return "", true
	}
	prefix := strings.TrimSuffix(base, "/")
	if !strings.HasPrefix(requestPath, prefix+"/") {
		return "", false
	}
	return strings.TrimPrefix(requestPath, prefix+"/"), true
}

func allowsHTMLFallback(request *http.Request, requestPath, relative string) bool {
	if !acceptsHTML(request.Header.Get("Accept")) {
		return false
	}
	if isAPIPath(relative) || looksLikeAssetPath(relative) {
		return false
	}
	return strings.HasSuffix(requestPath, "/") || pathpkg.Ext(relative) == ""
}

func acceptsHTML(value string) bool {
	for _, item := range strings.Split(value, ",") {
		mediaType := strings.TrimSpace(strings.SplitN(item, ";", 2)[0])
		if mediaType == "text/html" || mediaType == "application/xhtml+xml" {
			return true
		}
	}
	return false
}

func isAPIPath(relative string) bool {
	relative = strings.Trim(relative, "/")
	return relative == "api" || strings.HasPrefix(relative, "api/")
}

func looksLikeAssetPath(relative string) bool {
	relative = strings.Trim(relative, "/")
	return relative == "assets" ||
		strings.HasPrefix(relative, "assets/") ||
		pathpkg.Ext(relative) != ""
}

func staticContentType(relative string) string {
	switch strings.ToLower(pathpkg.Ext(relative)) {
	case ".wasm":
		return "application/wasm"
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	}
	if value := mime.TypeByExtension(pathpkg.Ext(relative)); value != "" {
		return value
	}
	return "application/octet-stream"
}

func injectDocumentBase(index []byte, base string) ([]byte, error) {
	if asciiTagIndex(index, "base") >= 0 {
		return nil, errors.New("package index already contains a base element")
	}
	head := asciiTagIndex(index, "head")
	closeHead := asciiClosingTagIndex(index, "head")
	if head < 0 || closeHead < 0 || closeHead <= head {
		return nil, errors.New("package index must contain one complete head element")
	}
	headEnd := bytes.IndexByte(index[head:], '>')
	if headEnd < 0 || head+headEnd >= closeHead {
		return nil, errors.New("package index contains a malformed head element")
	}
	headEnd += head + 1
	injection := []byte("\n    <base href=\"" + html.EscapeString(base) + "\" />")
	result := make([]byte, 0, len(index)+len(injection))
	result = append(result, index[:headEnd]...)
	result = append(result, injection...)
	result = append(result, index[headEnd:]...)
	return result, nil
}

func asciiTagIndex(content []byte, name string) int {
	return asciiElementIndex(content, "<"+name, false)
}

func asciiClosingTagIndex(content []byte, name string) int {
	return asciiElementIndex(content, "</"+name, true)
}

func asciiElementIndex(content []byte, prefix string, closing bool) int {
	for index := 0; index+len(prefix) <= len(content); index++ {
		if !asciiEqualFold(content[index:index+len(prefix)], prefix) {
			continue
		}
		next := index + len(prefix)
		if next >= len(content) {
			continue
		}
		if closing {
			if content[next] == '>' || isASCIISpace(content[next]) {
				return index
			}
			continue
		}
		if content[next] == '>' || content[next] == '/' || isASCIISpace(content[next]) {
			return index
		}
	}
	return -1
}

func asciiEqualFold(content []byte, value string) bool {
	if len(content) != len(value) {
		return false
	}
	for index := range content {
		left := content[index]
		right := value[index]
		if left >= 'A' && left <= 'Z' {
			left += 'a' - 'A'
		}
		if right >= 'A' && right <= 'Z' {
			right += 'a' - 'A'
		}
		if left != right {
			return false
		}
	}
	return true
}

func isASCIISpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}

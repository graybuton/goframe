package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
)

type serveOptions struct {
	appDir    string
	dir       string
	workspace string
	port      int
}

func serveCommand(args []string) error {
	options, err := parseServeOptions(args)
	if err != nil {
		return err
	}
	return serve(options)
}

func parseServeOptions(args []string) (serveOptions, error) {
	options := serveOptions{port: 8080}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case strings.HasPrefix(arg, "--dir="):
			options.dir = strings.TrimPrefix(arg, "--dir=")
		case arg == "--dir":
			index++
			if index >= len(args) {
				return serveOptions{}, errors.New("--dir requires a value")
			}
			options.dir = args[index]
		case strings.HasPrefix(arg, "--workspace="):
			options.workspace = strings.TrimPrefix(arg, "--workspace=")
		case arg == "--workspace":
			index++
			if index >= len(args) {
				return serveOptions{}, errors.New("--workspace requires a value")
			}
			options.workspace = args[index]
		case strings.HasPrefix(arg, "--port="):
			port, err := strconv.Atoi(strings.TrimPrefix(arg, "--port="))
			if err != nil {
				return serveOptions{}, fmt.Errorf("invalid port: %w", err)
			}
			options.port = port
		case arg == "--port":
			index++
			if index >= len(args) {
				return serveOptions{}, errors.New("--port requires a value")
			}
			port, err := strconv.Atoi(args[index])
			if err != nil {
				return serveOptions{}, fmt.Errorf("invalid port: %w", err)
			}
			options.port = port
		case strings.HasPrefix(arg, "-"):
			return serveOptions{}, fmt.Errorf("unknown serve flag %q", arg)
		case options.appDir == "":
			options.appDir = arg
		default:
			return serveOptions{}, fmt.Errorf("unexpected serve argument %q", arg)
		}
	}
	if options.port < 0 || options.port > 65535 {
		return serveOptions{}, fmt.Errorf("port must be between 0 and 65535")
	}
	if options.dir == "" {
		if options.appDir == "" {
			return serveOptions{}, errors.New("usage: goxc serve <app-directory> [--port=8080] [--workspace=directory] or goxc serve --dir=<directory>")
		}
		layout, err := newBuildLayout(layoutOptions{appDir: options.appDir, workspace: options.workspace})
		if err != nil {
			return serveOptions{}, err
		}
		options.dir = layout.PackageDir
	}
	return options, nil
}

func serve(options serveOptions) error {
	if err := directoryNoFollow(options.dir, "serve directory"); err != nil {
		if options.appDir != "" && errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("no standalone package found; run `goxc package %s` first", options.appDir)
		}
		return err
	}
	info, err := os.Stat(options.dir)
	if err != nil {
		if options.appDir != "" {
			return fmt.Errorf("no standalone package found; run `goxc package %s` first", options.appDir)
		}
		return fmt.Errorf("open serve directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", options.dir)
	}

	address := fmt.Sprintf("127.0.0.1:%d", options.port)
	fmt.Printf("serving %s at http://%s\n", options.dir, address)
	return http.ListenAndServe(address, staticHandler(options.dir))
}

func staticHandler(directory string) http.Handler {
	files := http.FileServer(http.Dir(directory))
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL == nil {
			http.NotFound(response, request)
			return
		}
		sanitizedPath, err := sanitizeServePath(request.URL.Path, request.URL.RawPath)
		if err != nil {
			http.NotFound(response, request)
			return
		}
		localPath := filepath.Join(directory, filepath.FromSlash(strings.TrimPrefix(sanitizedPath, "/")))
		if err := validatePathBelowRoot(directory, localPath, "serve path", false); err != nil {
			http.NotFound(response, request)
			return
		}
		if info, err := os.Lstat(localPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
			http.NotFound(response, request)
			return
		}
		path := sanitizedPath
		if strings.HasSuffix(path, ".br") {
			response.Header().Set("Content-Encoding", "br")
			path = strings.TrimSuffix(path, ".br")
		} else if strings.HasSuffix(path, ".gz") {
			response.Header().Set("Content-Encoding", "gzip")
			path = strings.TrimSuffix(path, ".gz")
		}
		switch {
		case strings.HasSuffix(path, ".wasm"):
			response.Header().Set("Content-Type", "application/wasm")
		case strings.HasSuffix(path, ".js"):
			response.Header().Set("Content-Type", "text/javascript")
		case strings.HasSuffix(path, ".css"):
			response.Header().Set("Content-Type", "text/css")
		}
		sanitizedRequest := request.Clone(request.Context())
		urlCopy := *request.URL
		urlCopy.Path = sanitizedPath
		urlCopy.RawPath = ""
		sanitizedRequest.URL = &urlCopy
		files.ServeHTTP(response, sanitizedRequest)
	})
}

func sanitizeServePath(requestPath, rawPath string) (string, error) {
	if requestPath == "" {
		requestPath = "/"
	}
	if strings.ContainsRune(requestPath, '\\') || strings.ContainsRune(rawPath, '\\') {
		return "", errors.New("serve path contains backslash")
	}
	if !strings.HasPrefix(requestPath, "/") {
		return "", errors.New("serve path must start with /")
	}
	for _, segment := range strings.Split(requestPath, "/") {
		if segment == ".." {
			return "", errors.New("serve path must not contain parent traversal")
		}
	}
	cleaned := pathpkg.Clean(requestPath)
	if cleaned == "." {
		return "/", nil
	}
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	return cleaned, nil
}

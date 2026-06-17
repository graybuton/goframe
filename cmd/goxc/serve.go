package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type serveOptions struct {
	appDir string
	dir    string
	port   int
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
			return serveOptions{}, errors.New("usage: goxc serve <app-directory> [--port=8080] or goxc serve --dir=<directory>")
		}
		manifest, err := loadManifest(options.appDir)
		if err != nil {
			return serveOptions{}, err
		}
		options.dir = filepath.Join(options.appDir, manifest.Output)
	}
	return options, nil
}

func serve(options serveOptions) error {
	info, err := os.Stat(options.dir)
	if err != nil {
		return fmt.Errorf("open serve directory: %w; run `goxc package` first", err)
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
		path := request.URL.Path
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
		files.ServeHTTP(response, request)
	})
}

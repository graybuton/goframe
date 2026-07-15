package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const slowGreetingDelay = 750 * time.Millisecond

func main() {
	packageDir := flag.String("package", "", "packaged GoFrame standalone directory")
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	flag.Parse()

	if *packageDir == "" {
		log.Fatal("--package is required")
	}
	info, err := os.Stat(*packageDir)
	if err != nil {
		log.Fatal(err)
	}
	if !info.IsDir() {
		log.Fatalf("%s is not a directory", *packageDir)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/greeting", greetingHandler)
	mux.Handle("/api/saved-greeting", savedGreetingHandler(
		newSavedGreetingStore("GoFrame"),
		slowGreetingDelay,
	))
	mux.Handle("/", staticPackageHandler(*packageDir))

	server := &http.Server{Addr: *addr, Handler: mux}
	log.Printf("serving %s at http://%s", *packageDir, *addr)
	log.Fatal(server.ListenAndServe())
}

func greetingHandler(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimSpace(request.URL.Query().Get("name"))
	if name == "" {
		name = "GoFrame"
	}
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	if name == "fail" {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(response, "controlled backend failure")
		return
	}
	if name == "slow" {
		select {
		case <-time.After(slowGreetingDelay):
		case <-request.Context().Done():
			return
		}
	}
	fmt.Fprintf(response, "Hello, %s, from Go backend!", name)
}

func staticPackageHandler(packageDir string) http.Handler {
	files := http.FileServer(http.Dir(packageDir))
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, ".wasm") {
			response.Header().Set("Content-Type", "application/wasm")
		}
		if request.URL.Path == "/" || filepath.Ext(request.URL.Path) == "" {
			response.Header().Set("Cache-Control", "no-store")
		}
		files.ServeHTTP(response, request)
	})
}

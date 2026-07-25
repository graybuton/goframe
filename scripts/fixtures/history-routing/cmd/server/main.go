package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	packageDir := flag.String("package", "", "packaged GoFrame standalone directory")
	address := flag.String("addr", "127.0.0.1:8080", "listen address")
	mode := flag.String("mode", string(serverModeStrict), "serving mode: strict or fallback")
	base := flag.String("base", "/", "deployment base path")
	flag.Parse()

	if *packageDir == "" {
		log.Fatal("--package is required")
	}
	handler, err := newPackageServer(serverConfig{
		PackageDir: *packageDir,
		Mode:       serverMode(*mode),
		Base:       *base,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("serving %s at http://%s in %s mode under %s", *packageDir, *address, *mode, handler.base)
	log.Fatal(http.ListenAndServe(*address, handler))
}

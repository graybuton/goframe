package main

import (
	"fmt"
	"io"
	"os"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage(os.Stdout)
		return
	}

	var err error
	switch os.Args[1] {
	case "help", "--help", "-h":
		usage(os.Stdout)
	case "version":
		err = versionCommand(os.Args[2:])
	case "doctor":
		err = doctorCommand(os.Args[2:])
	case "generate":
		err = generateCommand(os.Args[2:])
	case "build":
		err = buildCommand(os.Args[2:])
	case "package":
		err = packageCommand(os.Args[2:])
	case "size":
		err = sizeCommand(os.Args[2:])
	case "serve":
		err = serveCommand(os.Args[2:])
	case "clean":
		err = cleanCommand(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "goxc: unknown command %q\n\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "goxc:", err)
		os.Exit(1)
	}
}

func usage(output io.Writer) {
	fmt.Fprintln(output, "goxc - GoFrame compiler and application toolchain")
	fmt.Fprintln(output, "")
	fmt.Fprintln(output, "usage: goxc <command> [arguments]")
	fmt.Fprintln(output, "")
	fmt.Fprintln(output, "commands:")
	fmt.Fprintln(output, "  generate <path>       generate .go files from .gox files")
	fmt.Fprintln(output, "  build <app>           compile raw WASM into build/")
	fmt.Fprintln(output, "  package <app>         create a runnable dist/ bundle")
	fmt.Fprintln(output, "  size <app-or-dir>     report artifact sizes")
	fmt.Fprintln(output, "  serve [app]           serve a packaged application locally")
	fmt.Fprintln(output, "  clean <app>           remove build and package artifacts")
	fmt.Fprintln(output, "  doctor                inspect the local toolchain")
	fmt.Fprintln(output, "  version               print goxc and compiler versions")
	fmt.Fprintln(output, "  help                  print this help")
}

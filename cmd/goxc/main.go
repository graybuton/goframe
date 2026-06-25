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
		if commandHelpRequested(os.Args[2:]) {
			commandUsage(os.Stdout, "version")
			return
		}
		err = versionCommand(os.Args[2:])
	case "doctor":
		if commandHelpRequested(os.Args[2:]) {
			commandUsage(os.Stdout, "doctor")
			return
		}
		err = doctorCommand(os.Args[2:])
	case "generate":
		if commandHelpRequested(os.Args[2:]) {
			commandUsage(os.Stdout, "generate")
			return
		}
		err = generateCommand(os.Args[2:])
	case "build":
		if commandHelpRequested(os.Args[2:]) {
			commandUsage(os.Stdout, "build")
			return
		}
		err = buildCommand(os.Args[2:])
	case "package":
		if commandHelpRequested(os.Args[2:]) {
			commandUsage(os.Stdout, "package")
			return
		}
		err = packageCommand(os.Args[2:])
	case "export":
		if commandHelpRequested(os.Args[2:]) {
			commandUsage(os.Stdout, "export")
			return
		}
		err = exportCommand(os.Args[2:])
	case "size":
		if commandHelpRequested(os.Args[2:]) {
			commandUsage(os.Stdout, "size")
			return
		}
		err = sizeCommand(os.Args[2:])
	case "serve":
		if commandHelpRequested(os.Args[2:]) {
			commandUsage(os.Stdout, "serve")
			return
		}
		err = serveCommand(os.Args[2:])
	case "clean":
		if commandHelpRequested(os.Args[2:]) {
			commandUsage(os.Stdout, "clean")
			return
		}
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
	fmt.Fprintln(output, "  generate <path>       generate .go files from .gox files into .goframe/gen/")
	fmt.Fprintln(output, "  build <app>           compile raw WASM into .goframe/build/")
	fmt.Fprintln(output, "  package <app>         create a runnable bundle in .goframe/package/")
	fmt.Fprintln(output, "  export <app> --out    copy the latest standalone package to a deploy directory")
	fmt.Fprintln(output, "  size <app-or-dir>     report artifact sizes")
	fmt.Fprintln(output, "  serve [app]           serve a packaged application locally")
	fmt.Fprintln(output, "  clean <app>           remove build and package artifacts")
	fmt.Fprintln(output, "  doctor                inspect the local toolchain")
	fmt.Fprintln(output, "  version               print goxc and compiler versions")
	fmt.Fprintln(output, "  help                  print this help")
}

func commandHelpRequested(args []string) bool {
	if len(args) != 1 {
		return false
	}
	switch args[0] {
	case "help", "--help", "-h":
		return true
	default:
		return false
	}
}

func commandUsage(output io.Writer, command string) {
	switch command {
	case "generate":
		fmt.Fprintln(output, "usage: goxc generate <file-or-directory> [--out=directory] [--workspace=directory] [--in-place]")
		fmt.Fprintln(output, "generate .go compiler output from .gox files; default output is .goframe/gen/")
	case "build":
		fmt.Fprintln(output, "usage: goxc build <app-directory> [--compiler=go|tinygo] [--out=directory] [--workspace=directory]")
		fmt.Fprintln(output, "compile raw WASM into .goframe/build/<compiler>/dev/")
	case "package":
		fmt.Fprintln(output, "usage: goxc package <app-directory> [--compiler=go|tinygo] [--out=directory] [--workspace=directory] [--asset-hash] [--preload] [--compress=gzip,br]")
		fmt.Fprintln(output, "create a runnable standalone bundle in .goframe/package/standalone/")
	case "export":
		fmt.Fprintln(output, "usage: goxc export <app-directory> --out=directory [--workspace=directory] [--force]")
		fmt.Fprintln(output, "copy the latest standalone package to an explicit deploy directory")
	case "serve":
		fmt.Fprintln(output, "usage: goxc serve <app-directory> [--port=8080] [--workspace=directory]")
		fmt.Fprintln(output, "       goxc serve --dir=directory [--port=8080]")
		fmt.Fprintln(output, "serve a packaged application locally on 127.0.0.1")
	case "size":
		fmt.Fprintln(output, "usage: goxc size <app-directory-or-package-directory> [--workspace=directory]")
		fmt.Fprintln(output, "report raw and compressed package artifact sizes")
	case "doctor":
		fmt.Fprintln(output, "usage: goxc doctor")
		fmt.Fprintln(output, "inspect Go, TinyGo, compression tools, runtime shims, and local directories")
	case "clean":
		fmt.Fprintln(output, "usage: goxc clean <app-directory> [--generated] [--legacy] [--workspace=directory]")
		fmt.Fprintln(output, "remove tool-owned build/package workspace artifacts")
	case "version":
		fmt.Fprintln(output, "usage: goxc version")
		fmt.Fprintln(output, "print goxc, Go, and TinyGo versions")
	default:
		usage(output)
	}
}

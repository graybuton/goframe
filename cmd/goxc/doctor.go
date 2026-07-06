package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func doctorCommand(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: goxc doctor")
	}

	fmt.Println("GoFrame Toolchain Doctor")
	fmt.Println()
	warnings := 0
	failures := 0

	if path, err := exec.LookPath("go"); err == nil {
		output, versionErr := exec.Command(path, "version").CombinedOutput()
		if versionErr == nil {
			fmt.Printf("Go:           found, %s\n", strings.TrimSpace(string(output)))
		} else {
			fmt.Println("Go:           found, version unavailable")
			warnings++
		}
	} else {
		fmt.Println("Go:           not found")
		failures++
	}

	tinyGoFound := false
	if path, err := exec.LookPath("tinygo"); err == nil {
		tinyGoFound = true
		output, versionErr := exec.Command(path, "version").CombinedOutput()
		if versionErr == nil {
			fmt.Printf("TinyGo:       found, %s\n", strings.TrimSpace(string(output)))
		} else {
			fmt.Println("TinyGo:       found, version unavailable")
			warnings++
		}
	} else {
		fmt.Println("TinyGo:       not found (optional)")
		warnings++
	}

	warnings += printOptionalTool("gzip")
	warnings += printOptionalTool("brotli")

	if path, err := wasmExecPath("go"); err == nil {
		fmt.Printf("wasm_exec.js: found, %s\n", path)
	} else {
		fmt.Printf("wasm_exec.js: not found below %s\n", runtime.GOROOT())
		failures++
	}
	if tinyGoFound {
		if path, err := wasmExecPath("tinygo"); err == nil {
			fmt.Printf("TinyGo shim:  found, %s\n", path)
		} else {
			fmt.Printf("TinyGo shim:  error, %v\n", err)
			failures++
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		fmt.Printf("Working dir:  ok, %s\n", cwd)
	} else {
		fmt.Printf("Working dir:  error, %v\n", err)
		failures++
	}
	if directory, err := os.MkdirTemp("", "goxc-doctor-*"); err == nil {
		os.RemoveAll(directory)
		fmt.Println("Temp dir:     ok")
	} else {
		fmt.Printf("Temp dir:     error, %v\n", err)
		failures++
	}

	fmt.Println()
	switch {
	case failures > 0:
		fmt.Println("Status: errors found")
		return errors.New("doctor: required checks failed")
	case warnings > 0:
		fmt.Println("Status: ok with warnings")
	default:
		fmt.Println("Status: ok")
	}
	return nil
}

func printOptionalTool(name string) int {
	if _, err := exec.LookPath(name); err == nil {
		fmt.Printf("%-13s found\n", name+":")
		return 0
	}
	fmt.Printf("%-13s not found (optional)\n", name+":")
	return 1
}

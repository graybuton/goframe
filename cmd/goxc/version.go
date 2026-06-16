package main

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func versionCommand(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: goxc version")
	}
	fmt.Printf("goxc version %s\n", version)
	if path, err := exec.LookPath("go"); err == nil {
		output, versionErr := exec.Command(path, "version").CombinedOutput()
		if versionErr == nil {
			fmt.Println(strings.TrimSpace(string(output)))
		} else {
			fmt.Println(runtime.Version())
		}
	} else {
		fmt.Println(runtime.Version())
	}

	path, err := exec.LookPath("tinygo")
	if err != nil {
		fmt.Println("tinygo: not found")
		return nil
	}
	output, err := exec.Command(path, "version").CombinedOutput()
	if err != nil {
		fmt.Println("tinygo: found, version unavailable")
		return nil
	}
	fmt.Println(strings.TrimSpace(string(output)))
	return nil
}

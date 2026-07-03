package main

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
)

func versionCommand(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: goxc version")
	}
	fmt.Printf("goxc version %s\n", goxcVersion())
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

func goxcVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "devel"
	}
	return versionFromBuildInfo(info)
}

func versionFromBuildInfo(info *debug.BuildInfo) string {
	if info == nil {
		return "devel"
	}
	version := strings.TrimSpace(info.Main.Version)
	if version == "" || version == "(devel)" || buildInfoHasVCSSettings(info) {
		return "devel"
	}
	return version
}

func buildInfoHasVCSSettings(info *debug.BuildInfo) bool {
	for _, setting := range info.Settings {
		if setting.Key == "vcs" || strings.HasPrefix(setting.Key, "vcs.") {
			return true
		}
	}
	return false
}

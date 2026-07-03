package main

import (
	"runtime/debug"
	"testing"
)

func TestVersionFromBuildInfoTaggedModule(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{
			Version: "v0.2.0-preview.3",
		},
	}

	if got := versionFromBuildInfo(info); got != "v0.2.0-preview.3" {
		t.Fatalf("versionFromBuildInfo() = %q, want v0.2.0-preview.3", got)
	}
}

func TestVersionFromBuildInfoLocalVCSBuild(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{
			Version: "v0.2.0-preview.3",
		},
		Settings: []debug.BuildSetting{
			{Key: "vcs", Value: "git"},
			{Key: "vcs.revision", Value: "53b2a15e2255e586ad5ba550a514da141aa74514"},
			{Key: "vcs.modified", Value: "false"},
		},
	}

	if got := versionFromBuildInfo(info); got != "devel" {
		t.Fatalf("versionFromBuildInfo(local VCS build) = %q, want devel", got)
	}
}

func TestVersionFromBuildInfoDevelopmentBuild(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{
			Version: "(devel)",
		},
	}

	if got := versionFromBuildInfo(info); got != "devel" {
		t.Fatalf("versionFromBuildInfo() = %q, want devel", got)
	}
}

func TestVersionFromBuildInfoEmptyVersion(t *testing.T) {
	info := &debug.BuildInfo{}

	if got := versionFromBuildInfo(info); got != "devel" {
		t.Fatalf("versionFromBuildInfo() = %q, want devel", got)
	}
}

func TestVersionFromBuildInfoUnavailable(t *testing.T) {
	if got := versionFromBuildInfo(nil); got != "devel" {
		t.Fatalf("versionFromBuildInfo(nil) = %q, want devel", got)
	}
}

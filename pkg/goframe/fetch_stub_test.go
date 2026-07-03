package goframe

import (
	"strings"
	"testing"
)

func TestFetchTextStubRejectsOnHostBuilds(t *testing.T) {
	resolved := false
	rejected := false
	var message string

	cleanup := FetchText("/api/message", func(string) {
		resolved = true
	}, func(err error) {
		rejected = true
		if err != nil {
			message = err.Error()
		}
	})

	if resolved {
		t.Fatal("FetchText host stub called resolve, want reject only")
	}
	if !rejected {
		t.Fatal("FetchText host stub did not call reject")
	}
	if message == "" || !strings.Contains(message, "browser") || !strings.Contains(message, "js/wasm") {
		t.Fatalf("FetchText host stub error = %q, want browser/js/wasm availability", message)
	}
	if cleanup == nil {
		t.Fatal("FetchText host stub cleanup is nil, want no-op cleanup")
	}
	cleanup()
}

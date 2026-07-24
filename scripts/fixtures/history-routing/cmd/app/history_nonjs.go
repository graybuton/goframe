//go:build !js || !wasm

package main

import (
	gf "github.com/graybuton/goframe/pkg/goframe"
	"github.com/graybuton/goframe/scripts/fixtures/history-routing/internal/historyroute"
)

func browserHistoryBasePath() string {
	return "/"
}

func browserHistoryCurrentTarget(string) string {
	return "/"
}

func browserHistoryPush(_ string, target string) string {
	return historyroute.NormalizeTarget(target)
}

func browserHistoryReplace(_ string, target string) string {
	return historyroute.NormalizeTarget(target)
}

func browserHistorySubscribe(string, func(string)) gf.Cleanup {
	return nil
}

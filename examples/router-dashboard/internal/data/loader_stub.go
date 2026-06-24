//go:build !js || !wasm

package data

import gf "github.com/graybuton/goframe/pkg/goframe"

type LoadError string

func (err LoadError) Error() string {
	return string(err)
}

func LoadIssues(key string, resolve func([]Issue), reject func(error)) gf.Cleanup {
	reject(LoadError("router dashboard data fetch is available only in browser/WASM builds"))
	return nil
}

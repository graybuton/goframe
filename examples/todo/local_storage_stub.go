//go:build !js || !wasm

package main

func localStorageGet(key string) (string, bool) {
	return "", false
}

func localStorageSet(key string, value string) {}

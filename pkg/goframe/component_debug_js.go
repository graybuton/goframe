//go:build js && wasm && goframe_debug

package goframe

import "syscall/js"

func reportComponentRender(name string) {
	probe := js.Global().Get("goframeComponentRenderProbe")
	if probe.Type() == js.TypeFunction {
		probe.Invoke(name)
	}
}

func reportComponentPatch(name string) {
	probe := js.Global().Get("goframeComponentPatchProbe")
	if probe.Type() == js.TypeFunction {
		probe.Invoke(name)
	}
}

func reportDuplicateSiblingNodeKeys(nodes []Node, owner string) {
	keys := make([]string, 0, len(nodes))
	for _, node := range nodes {
		key, _ := unwrapNode(node)
		keys = append(keys, key)
	}
	reportDuplicateSiblingKeys(keys, owner)
}

func reportDuplicateSiblingKeys(keys []string, owner string) {
	seen := make(map[string]bool, len(keys))
	warned := make(map[string]bool)
	for _, key := range keys {
		if key == "" {
			continue
		}
		if seen[key] && !warned[key] {
			reportDuplicateKey(owner, key)
			warned[key] = true
			continue
		}
		seen[key] = true
	}
}

func reportDuplicateKey(owner, key string) {
	message := "goframe: duplicate key \"" + key + "\" among children of " + owner
	warnings := js.Global().Get("goframeDuplicateKeyWarnings")
	if warnings.IsUndefined() || warnings.IsNull() {
		warnings = js.Global().Get("Array").New()
		js.Global().Set("goframeDuplicateKeyWarnings", warnings)
	}
	warnings.Call("push", message)

	console := js.Global().Get("console")
	if !console.IsUndefined() && !console.IsNull() && console.Get("warn").Type() == js.TypeFunction {
		console.Call("warn", message)
	}
}

//go:build js && wasm

package main

import (
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

var (
	fixtureControls []js.Func

	nextAppAInstance  int
	nextAppBInstance  int
	nextAppCInstance  int
	activeApplication string

	appASetter              func(int)
	appAValue               int
	appARenderCount         int
	appAHandlerCount        int
	appAEffectSetups        = make(map[int]int)
	appAEffectCleanups      = make(map[int]int)
	appAUnmounts            = make(map[int]int)
	appAEffectCleanupSawDOM = make(map[int]int)
	appAUnmountSawDOM       = make(map[int]int)
	appAEffectVersions      []string

	appBValue                int
	appBRenderCount          int
	appBHandlerCount         int
	appBScheduledUpdateCount int
	appBCleanups             = make(map[int]int)

	appCRenderCount  int
	appCHandlerCount int
	appCCleanups     = make(map[int]int)

	missingRootPanicCount  int
	missingRootPanicText   string
	nestedTargetPanicCount int
	nestedTargetPanicText  string
	runtimeErrorCount      int
)

func fixturePanicText(value any) string {
	if err, ok := value.(error); ok {
		return err.Error()
	}
	return gf.ToString(value)
}

func main() {
	global := js.Global()
	global.Set("goframeRepeatedMountErrors", global.Get("Array").New())
	gf.SetErrorHandler(func(info gf.ErrorInfo) {
		runtimeErrorCount++
		report := global.Get("Object").New()
		report.Set("phase", info.Phase.String())
		report.Set("component", info.Component)
		report.Set("operation", info.Operation)
		report.Set("panic", fixturePanicText(info.Panic))
		global.Get("goframeRepeatedMountErrors").Call("push", report)
	})

	exportFixtureControls()
	mountAppA("root-a")

	done := make(chan struct{})
	<-done
}

func mountAppA(rootID string) {
	instance := nextAppAInstance + 1
	gf.Mount(rootID, func() gf.Node {
		return AppA(AppAProps{Instance: instance})
	})
	nextAppAInstance = instance
	activeApplication = "A:" + gf.ToString(instance)
}

func mountAppB(rootID string) {
	instance := nextAppBInstance + 1
	gf.Mount(rootID, func() gf.Node {
		return AppB(AppBProps{Instance: instance})
	})
	nextAppBInstance = instance
	activeApplication = "B:" + gf.ToString(instance)
}

func mountAppC(rootID string) {
	instance := nextAppCInstance + 1
	gf.Mount(rootID, func() gf.Node {
		return AppC(AppCProps{Instance: instance})
	})
	nextAppCInstance = instance
	activeApplication = "C:" + gf.ToString(instance)
}

func exportFixtureControls() {
	exportFixtureControl("goframeRepeatedMountMountB", func() {
		mountAppB("root-b")
	})
	exportFixtureControl("goframeRepeatedMountReplaceBWithC", func() {
		mountAppC("root-b")
	})
	exportFixtureControl("goframeRepeatedMountMountFreshA", func() {
		mountAppA("root-a")
	})
	exportFixtureControl("goframeRepeatedMountQueueAThenMountB", func() {
		appASetter(appAValue + 1)
		mountAppB("root-b")
	})
	exportFixtureControl("goframeRepeatedMountInvokeStaleASetter", func() {
		appASetter(appAValue + 1)
	})
	exportFixtureControl("goframeRepeatedMountAttemptMissingRoot", attemptMissingRoot)
	exportFixtureControl("goframeRepeatedMountAttemptOwnedNestedRoot", func() {
		attemptMount("owned-nested-root", &nestedTargetPanicCount, &nestedTargetPanicText)
	})
	exportFixtureControl("goframeRepeatedMountAttemptHostNestedRoot", func() {
		attemptMount("host-owned-subroot", &nestedTargetPanicCount, &nestedTargetPanicText)
	})

	read := js.FuncOf(func(js.Value, []js.Value) any {
		return repeatedMountEvidence()
	})
	fixtureControls = append(fixtureControls, read)
	js.Global().Set("goframeRepeatedMountRead", read)
}

func exportFixtureControl(name string, control func()) {
	callback := js.FuncOf(func(js.Value, []js.Value) any {
		control()
		return nil
	})
	fixtureControls = append(fixtureControls, callback)
	js.Global().Set(name, callback)
}

func attemptMissingRoot() {
	attemptMount("missing-root", &missingRootPanicCount, &missingRootPanicText)
}

func attemptMount(rootID string, panicCount *int, panicText *string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			*panicCount++
			*panicText = fixturePanicText(recovered)
		}
	}()
	mountAppB(rootID)
}

func repeatedMountEvidence() js.Value {
	result := js.Global().Get("Object").New()
	result.Set("activeApplication", activeApplication)
	result.Set("appARenders", appARenderCount)
	result.Set("appBRenders", appBRenderCount)
	result.Set("appCRenders", appCRenderCount)
	result.Set("appAHandlers", appAHandlerCount)
	result.Set("appBHandlers", appBHandlerCount)
	result.Set("appCHandlers", appCHandlerCount)
	result.Set("appAValue", appAValue)
	result.Set("appBValue", appBValue)
	result.Set("appBScheduledUpdates", appBScheduledUpdateCount)
	result.Set("appAEffectSetups", integerEvidence(appAEffectSetups, nextAppAInstance))
	result.Set("appAEffectCleanups", integerEvidence(appAEffectCleanups, nextAppAInstance))
	result.Set("appAUnmounts", integerEvidence(appAUnmounts, nextAppAInstance))
	result.Set("appAEffectCleanupSawDOM", integerEvidence(appAEffectCleanupSawDOM, nextAppAInstance))
	result.Set("appAUnmountSawDOM", integerEvidence(appAUnmountSawDOM, nextAppAInstance))
	result.Set("appAEffectVersions", stringEvidence(appAEffectVersions))
	result.Set("appBCleanups", integerEvidence(appBCleanups, nextAppBInstance))
	result.Set("appCCleanups", integerEvidence(appCCleanups, nextAppCInstance))
	result.Set("missingRootPanicCount", missingRootPanicCount)
	result.Set("missingRootPanicText", missingRootPanicText)
	result.Set("nestedTargetPanicCount", nestedTargetPanicCount)
	result.Set("nestedTargetPanicText", nestedTargetPanicText)
	result.Set("runtimeErrorCount", runtimeErrorCount)
	return result
}

func integerEvidence(values map[int]int, count int) js.Value {
	result := js.Global().Get("Array").New()
	for index := 1; index <= count; index++ {
		result.Call("push", values[index])
	}
	return result
}

func stringEvidence(values []string) js.Value {
	result := js.Global().Get("Array").New()
	for _, value := range values {
		result.Call("push", value)
	}
	return result
}

func recordAppAEffectCleanup(instance int) {
	appAEffectCleanups[instance]++
	if fixtureElementExists(appElementID("app-a", instance)) {
		appAEffectCleanupSawDOM[instance]++
	}
}

func recordAppAUnmount(instance int) {
	appAUnmounts[instance]++
	if fixtureElementExists(appElementID("app-a", instance)) {
		appAUnmountSawDOM[instance]++
	}
}

func fixtureElementExists(id string) bool {
	element := js.Global().Get("document").Call("getElementById", id)
	return !element.IsUndefined() && !element.IsNull()
}

func appElementID(name string, instance int) string {
	return name + "-" + gf.ToString(instance)
}

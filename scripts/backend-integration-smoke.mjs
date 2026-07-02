import { spawn } from "node:child_process";
import { mkdir, mkdtemp, rm, stat, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import net from "node:net";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const rootDir = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const chrome = process.env.CHROME ?? "google-chrome";
const debugPort = Number(process.env.GOFRAME_BACKEND_CHROME_DEBUG_PORT ?? await pickFreePort());
const backendPort = Number(process.env.GOFRAME_BACKEND_SMOKE_PORT ?? await pickFreePort());
const expectedMessage = "Hello from Go backend";
const tempRoot = await mkdtempCompat("goframe-backend-smoke-");
const appDir = join(tempRoot, "app");
const backendDir = join(tempRoot, "backend");
const profile = join(tempRoot, "chrome-profile");
const appURL = `http://127.0.0.1:${backendPort}/?smoke=${Date.now()}`;
const expectedApp = new URL(appURL);

let backend = null;
let browser = null;
let backendError = "";
let browserError = "";
let browserExit = null;

try {
    await createFixture();
    await runCommand("go", ["run", "./cmd/goxc", "package", appDir, "--compiler=go"], { cwd: rootDir });

    const packageDir = join(appDir, ".goframe", "package", "standalone");
    const packageInfo = await stat(packageDir);
    if (!packageInfo.isDirectory()) {
        throw new Error(`HARNESS FAILURE: package output is not a directory: ${packageDir}`);
    }

    backend = spawn("go", [
        "run",
        ".",
        `--package=${packageDir}`,
        `--addr=127.0.0.1:${backendPort}`,
    ], {
        cwd: backendDir,
        detached: true,
        stdio: ["ignore", "ignore", "pipe"],
    });
    backend.stderr.on("data", (chunk) => {
        backendError += chunk;
    });

    await waitForHTTP(`http://127.0.0.1:${backendPort}/`, () => backend.exitCode !== null, "backend");
    const apiResponse = await fetch(`http://127.0.0.1:${backendPort}/api/message`);
    if (!apiResponse.ok) {
        throw new Error(`HARNESS FAILURE: backend API returned HTTP ${apiResponse.status}`);
    }
    const apiText = await apiResponse.text();
    if (apiText !== expectedMessage) {
        throw new Error(`HARNESS FAILURE: backend API returned ${JSON.stringify(apiText)}, want ${JSON.stringify(expectedMessage)}`);
    }

    browser = spawn(chrome, [
        "--headless",
        "--no-sandbox",
        "--disable-gpu",
        `--remote-debugging-port=${debugPort}`,
        `--user-data-dir=${profile}`,
        "about:blank",
    ], {
        stdio: ["ignore", "ignore", "pipe"],
    });
    browser.stderr.on("data", (chunk) => {
        browserError += chunk;
    });
    browser.on("exit", (code, signal) => {
        browserExit = { code, signal };
    });

    const page = await waitForPage(debugPort);
    const client = await connect(page.webSocketDebuggerUrl);
    await client.call("Runtime.enable");
    await client.call("Page.enable");
    await navigateToApp(client, appURL);
    await waitForAppPage(client, expectedApp);
    await waitForCondition(async () => {
        const state = await appState(client, expectedMessage);
        return state.ready && state.messageMatches && state.origin === expectedApp.origin;
    }, "backend message render");

    const state = await appState(client, expectedMessage);
    assertState(state, {
        app: true,
        ready: true,
        failed: false,
        status: "ready",
        message: expectedMessage,
        messageMatches: true,
        origin: expectedApp.origin,
    }, "backend integration render");

    client.close();
    console.log("Backend integration browser smoke: ok");
} finally {
    await stopProcess(browser, { processGroup: false });
    await stopProcess(backend, { processGroup: true });
    await rm(tempRoot, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function createFixture() {
    await mkdir(appDir, { recursive: true });
    await mkdir(backendDir, { recursive: true });

    await writeFile(join(appDir, "go.mod"), `module example.com/backend-smoke

go 1.22

require github.com/graybuton/goframe v0.0.0

replace github.com/graybuton/goframe => ${goModPath(rootDir)}
`);
    await writeFile(join(appDir, "main.go"), `//go:build js && wasm

package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func main() {
\tdone := make(chan struct{})
\tgf.Mount("root", App)
\t<-done
}
`);
    await writeFile(join(appDir, "app.gox"), `package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func App() gf.Node {
\tresource, _ := gf.UseResource("/api/message", loadMessage)

\treturn (
\t\t<main data-testid="backend-app">
\t\t\t<h1>Go backend integration boundary</h1>
\t\t\t<p data-testid="backend-status">{messageStatus(resource)}</p>
\t\t\t{resource.Loading() && (
\t\t\t\t<p data-testid="backend-loading">Loading same-origin API...</p>
\t\t\t)}
\t\t\t{resource.Failed() && (
\t\t\t\t<p data-testid="backend-error">{messageError(resource)}</p>
\t\t\t)}
\t\t\t{resource.Ready() && (
\t\t\t\t<p data-testid="backend-message">{resource.Value}</p>
\t\t\t)}
\t\t</main>
\t)
}

func messageStatus(resource gf.Resource[string]) string {
\tif resource.Ready() {
\t\treturn "ready"
\t}
\tif resource.Failed() {
\t\treturn "failed"
\t}
\treturn "loading"
}

func messageError(resource gf.Resource[string]) string {
\tif resource.Err == nil {
\t\treturn ""
\t}
\treturn resource.Err.Error()
}
`);
    await writeFile(join(appDir, "fetch_js.go"), `//go:build js && wasm

package main

import (
\t"syscall/js"

\tgf "github.com/graybuton/goframe/pkg/goframe"
)

func loadMessage(key string, resolve func(string), reject func(error)) gf.Cleanup {
\tactive := true
\treleasedPromiseFuncs := false
\tvar responseThen js.Func
\tvar textThen js.Func
\tvar catchFunc js.Func

\treleasePromiseFuncs := func() {
\t\tif releasedPromiseFuncs {
\t\t\treturn
\t\t}
\t\treleasedPromiseFuncs = true
\t\tresponseThen.Release()
\t\ttextThen.Release()
\t\tcatchFunc.Release()
\t}
\tcomplete := func(text string) {
\t\tif !active {
\t\t\treturn
\t\t}
\t\tactive = false
\t\tresolve(text)
\t}

\ttextThen = js.FuncOf(func(this js.Value, args []js.Value) any {
\t\ttext := ""
\t\tif len(args) > 0 && args[0].Type() == js.TypeString {
\t\t\ttext = args[0].String()
\t\t}
\t\treleasePromiseFuncs()
\t\tcomplete(text)
\t\treturn nil
\t})
\tcatchFunc = js.FuncOf(func(this js.Value, args []js.Value) any {
\t\treleasePromiseFuncs()
\t\tif active {
\t\t\tactive = false
\t\t\treject(fetchError("fetch failed"))
\t\t}
\t\treturn nil
\t})
\tresponseThen = js.FuncOf(func(this js.Value, args []js.Value) any {
\t\tif !active {
\t\t\treleasePromiseFuncs()
\t\t\treturn nil
\t\t}
\t\tif len(args) == 0 {
\t\t\tactive = false
\t\t\treleasePromiseFuncs()
\t\t\treject(fetchError("fetch returned no response"))
\t\t\treturn nil
\t\t}
\t\tresponse := args[0]
\t\tif !response.Get("ok").Bool() {
\t\t\tactive = false
\t\t\treleasePromiseFuncs()
\t\t\treject(fetchError("fetch returned a non-ok response"))
\t\t\treturn nil
\t\t}
\t\tresponse.Call("text").Call("then", textThen).Call("catch", catchFunc)
\t\treturn nil
\t})

\tcontroller := js.Global().Get("AbortController").New()
\toptions := js.Global().Get("Object").New()
\toptions.Set("signal", controller.Get("signal"))
\tjs.Global().Call("fetch", key, options).Call("then", responseThen).Call("catch", catchFunc)

\treturn func() {
\t\tif !active {
\t\t\treturn
\t\t}
\t\tactive = false
\t\tcontroller.Call("abort")
\t}
}
`);
    await writeFile(join(appDir, "fetch_stub.go"), `//go:build !js || !wasm

package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func loadMessage(key string, resolve func(string), reject func(error)) gf.Cleanup {
\treject(fetchError("browser fetch is available only in browser/WASM builds"))
\treturn nil
}
`);
    await writeFile(join(appDir, "fetch_error.go"), `package main

type fetchError string

func (err fetchError) Error() string {
\treturn string(err)
}
`);

    await writeFile(join(backendDir, "go.mod"), `module example.com/backend-smoke-server

go 1.22
`);
    await writeFile(join(backendDir, "main.go"), `package main

import (
\t"flag"
\t"fmt"
\t"log"
\t"net/http"
\t"os"
\t"path/filepath"
\t"strings"
)

func main() {
\tpackageDir := flag.String("package", "", "packaged GoFrame standalone directory")
\taddr := flag.String("addr", "127.0.0.1:0", "listen address")
\tflag.Parse()
\tif *packageDir == "" {
\t\tlog.Fatal("--package is required")
\t}
\tinfo, err := os.Stat(*packageDir)
\tif err != nil {
\t\tlog.Fatal(err)
\t}
\tif !info.IsDir() {
\t\tlog.Fatalf("%s is not a directory", *packageDir)
\t}

\tmux := http.NewServeMux()
\tmux.HandleFunc("/api/message", func(response http.ResponseWriter, request *http.Request) {
\t\tif request.Method != http.MethodGet {
\t\t\tresponse.WriteHeader(http.StatusMethodNotAllowed)
\t\t\treturn
\t\t}
\t\tresponse.Header().Set("Content-Type", "text/plain; charset=utf-8")
\t\tresponse.Header().Set("Cache-Control", "no-store")
\t\tfmt.Fprint(response, "Hello from Go backend")
\t})
\tmux.Handle("/", staticPackageHandler(*packageDir))

\tserver := &http.Server{Addr: *addr, Handler: mux}
\tlog.Printf("serving %s at http://%s", *packageDir, *addr)
\tlog.Fatal(server.ListenAndServe())
}

func staticPackageHandler(packageDir string) http.Handler {
\tfiles := http.FileServer(http.Dir(packageDir))
\treturn http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
\t\tif strings.HasSuffix(request.URL.Path, ".wasm") {
\t\t\tresponse.Header().Set("Content-Type", "application/wasm")
\t\t}
\t\tif request.URL.Path == "/" || filepath.Ext(request.URL.Path) == "" {
\t\t\tresponse.Header().Set("Cache-Control", "no-store")
\t\t}
\t\tfiles.ServeHTTP(response, request)
\t})
}
`);
}

async function appState(client, expected) {
    return await client.callFunction(`function(expected) {
        const message = document.querySelector("[data-testid='backend-message']")?.textContent.trim() ?? "";
        return {
            app: Boolean(document.querySelector("[data-testid='backend-app']")),
            loading: Boolean(document.querySelector("[data-testid='backend-loading']")),
            ready: Boolean(document.querySelector("[data-testid='backend-message']")),
            failed: Boolean(document.querySelector("[data-testid='backend-error']")),
            status: document.querySelector("[data-testid='backend-status']")?.textContent.trim() ?? "",
            message,
            messageMatches: message === expected,
            origin: window.location.origin,
        };
    }`, expected);
}

function assertState(actual, expected, label) {
    for (const [key, value] of Object.entries(expected)) {
        if (actual[key] !== value) {
            throw new Error(`APP FAILURE: ${label}: ${key} got ${JSON.stringify(actual[key])}, want ${JSON.stringify(value)}; state=${JSON.stringify(actual)}`);
        }
    }
    console.log(`${label}: ok`);
}

async function waitForPage(port) {
    let lastError;
    for (let attempt = 0; attempt < 80; attempt++) {
        if (browserExit) {
            throw new Error(`HARNESS FAILURE: Chrome exited before CDP page was available: ${JSON.stringify(browserExit)}\n${browserError}`);
        }
        try {
            const pages = await fetchTargets(port);
            const page = pages.find((entry) => entry.type === "page" && entry.webSocketDebuggerUrl);
            if (page) {
                return page;
            }
        } catch (error) {
            lastError = error;
        }
        await wait(100);
    }
    throw new Error(`HARNESS FAILURE: Chrome DevTools did not become ready: ${lastError ?? browserError}`);
}

async function fetchTargets(port) {
    const response = await fetch(`http://127.0.0.1:${port}/json`);
    if (!response.ok) {
        throw new Error(`CDP /json returned HTTP ${response.status}`);
    }
    return await response.json();
}

async function navigateToApp(client, url) {
    await client.call("Page.navigate", { url });
}

async function waitForAppPage(client, expected) {
    let lastState = null;
    for (let attempt = 0; attempt < 120; attempt++) {
        lastState = await pageState(client);
        if (lastState.href.startsWith("chrome-error://")) {
            throw await harnessFailure(client, "Chrome loaded an error document", lastState);
        }
        if (isExpectedAppState(lastState, expected) && lastState.root && lastState.app) {
            return lastState;
        }
        await wait(100);
    }
    throw await harnessFailure(client, "app page did not become ready", lastState);
}

async function pageState(client) {
    return await client.evaluate(`(() => ({
        href: window.location.href,
        origin: window.location.origin,
        protocol: window.location.protocol,
        readyState: document.readyState,
        root: Boolean(document.querySelector("#root")),
        app: Boolean(document.querySelector("[data-testid='backend-app']")),
    }))()`);
}

function isExpectedAppState(state, expected) {
    if (!state || (state.protocol !== "http:" && state.protocol !== "https:")) {
        return false;
    }
    try {
        const actual = new URL(state.href);
        return actual.origin === expected.origin && actual.pathname === expected.pathname;
    } catch {
        return false;
    }
}

async function harnessFailure(client, message, detail) {
    const diagnostics = await collectDiagnostics(client);
    return new Error(`HARNESS FAILURE: ${message}\n${JSON.stringify({ appURL, debugPort, backendPort, detail, diagnostics }, null, 2)}`);
}

async function collectDiagnostics(client) {
    const diagnostics = { targets: [], page: null };
    try {
        diagnostics.targets = (await fetchTargets(debugPort)).map((target) => ({
            id: target.id,
            type: target.type,
            url: target.url,
            title: target.title,
        }));
    } catch (error) {
        diagnostics.targetsError = error.message;
    }
    if (client) {
        try {
            diagnostics.page = await pageState(client);
        } catch (error) {
            diagnostics.pageError = error.message;
        }
    }
    if (backend?.exitCode !== null) {
        diagnostics.backendExitCode = backend.exitCode;
    }
    if (backendError) {
        diagnostics.backendStderr = backendError.slice(-4000);
    }
    if (browserExit) {
        diagnostics.browserExit = browserExit;
    }
    if (browserError) {
        diagnostics.browserStderr = browserError.slice(-4000);
    }
    return diagnostics;
}

async function connect(url) {
    const socket = new WebSocket(url);
    await new Promise((resolve, reject) => {
        socket.addEventListener("open", resolve, { once: true });
        socket.addEventListener("error", reject, { once: true });
    });

    let nextID = 1;
    const pending = new Map();
    socket.addEventListener("message", (event) => {
        const message = JSON.parse(event.data);
        if (!message.id || !pending.has(message.id)) {
            return;
        }
        const request = pending.get(message.id);
        pending.delete(message.id);
        if (message.error) {
            request.reject(new Error(message.error.message));
        } else {
            request.resolve(message.result);
        }
    });

    const call = (method, params = {}) =>
        new Promise((resolve, reject) => {
            const id = nextID++;
            pending.set(id, { resolve, reject });
            socket.send(JSON.stringify({ id, method, params }));
        });
    let globalObjectID = "";
    const getGlobalObjectID = async () => {
        if (globalObjectID) {
            return globalObjectID;
        }
        const result = await call("Runtime.evaluate", {
            expression: "globalThis",
            returnByValue: false,
        });
        if (result.exceptionDetails) {
            throw new Error(`browser evaluation failed: ${JSON.stringify(result.exceptionDetails)}`);
        }
        globalObjectID = result.result.objectId;
        return globalObjectID;
    };

    return {
        call,
        close: () => socket.close(),
        evaluate: async (expression) => {
            const result = await call("Runtime.evaluate", {
                expression,
                returnByValue: true,
                awaitPromise: true,
            });
            if (result.exceptionDetails) {
                throw new Error(`browser evaluation failed: ${JSON.stringify(result.exceptionDetails)}`);
            }
            return result.result.value;
        },
        callFunction: async (functionDeclaration, ...args) => {
            const result = await call("Runtime.callFunctionOn", {
                objectId: await getGlobalObjectID(),
                functionDeclaration,
                arguments: args.map((value) => ({ value })),
                returnByValue: true,
                awaitPromise: true,
            });
            if (result.exceptionDetails) {
                throw new Error(`browser evaluation failed: ${JSON.stringify(result.exceptionDetails)}`);
            }
            return result.result.value;
        },
    };
}

async function waitForCondition(check, label) {
    let lastError = null;
    for (let attempt = 0; attempt < 120; attempt++) {
        try {
            if (await check()) {
                return;
            }
        } catch (error) {
            lastError = error;
        }
        await wait(100);
    }
    throw new Error(`HARNESS FAILURE: timed out waiting for ${label}${lastError ? `: ${lastError.message}` : ""}`);
}

async function waitForHTTP(url, exited, label) {
    let lastError;
    for (let attempt = 0; attempt < 120; attempt++) {
        if (exited()) {
            throw new Error(`HARNESS FAILURE: ${label} exited before HTTP was available\n${backendError}`);
        }
        try {
            const response = await fetch(url);
            if (response.ok) {
                return;
            }
            lastError = new Error(`HTTP ${response.status}`);
        } catch (error) {
            lastError = error;
        }
        await wait(100);
    }
    throw new Error(`HARNESS FAILURE: ${label} HTTP endpoint did not become ready: ${lastError?.message ?? ""}\n${backendError}`);
}

function runCommand(command, args, options = {}) {
    return new Promise((resolve, reject) => {
        const child = spawn(command, args, {
            cwd: options.cwd ?? rootDir,
            stdio: "inherit",
            env: {
                ...process.env,
                GOWORK: "off",
            },
        });
        child.on("error", reject);
        child.on("exit", (code, signal) => {
            if (code === 0) {
                resolve();
                return;
            }
            reject(new Error(`${command} ${args.join(" ")} failed with ${signal ?? code}`));
        });
    });
}

async function stopProcess(child, options) {
    if (!child || child.exitCode !== null) {
        return;
    }
    const exited = new Promise((resolve) => child.once("exit", resolve));
    terminate(child, options.processGroup, "SIGTERM");
    const stopped = await Promise.race([
        exited.then(() => true),
        wait(2000).then(() => false),
    ]);
    if (stopped) {
        return;
    }
    terminate(child, options.processGroup, "SIGKILL");
    await Promise.race([exited, wait(1000)]);
}

function terminate(child, processGroup, signal) {
    if (processGroup && child.pid) {
        try {
            process.kill(-child.pid, signal);
            return;
        } catch {
            // Fall back to the direct child when process-group signaling is unavailable.
        }
    }
    child.kill(signal);
}

function pickFreePort() {
    return new Promise((resolve, reject) => {
        const server = net.createServer();
        server.on("error", reject);
        server.listen(0, "127.0.0.1", () => {
            const address = server.address();
            server.close(() => resolve(address.port));
        });
    });
}

async function mkdtempCompat(prefix) {
    return await mkdtemp(join(tmpdir(), prefix));
}

function goModPath(path) {
    return path.split(sep).join("/");
}

function wait(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

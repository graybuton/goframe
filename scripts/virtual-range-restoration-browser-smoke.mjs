import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { copyFile, mkdtemp, readFile, rm, stat, writeFile } from "node:fs/promises";
import { createServer as createHTTPServer } from "node:http";
import { createServer as createPortServer } from "node:net";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const rootDir = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const fixturePackage = "./scripts/fixtures/virtual-range-restoration";
const compiler = process.env.GOFRAME_VIRTUAL_RANGE_COMPILER ?? "go";
const chrome = process.env.CHROME ?? "google-chrome";
const goToolchain = process.env.GOTOOLCHAIN ?? "go1.26.6";
const appPort = Number(process.env.GOFRAME_VIRTUAL_RANGE_PORT ?? await pickFreePort());
const debugPort = Number(
    process.env.GOFRAME_VIRTUAL_RANGE_CHROME_DEBUG_PORT ?? await pickFreePort(),
);
const origin = `http://127.0.0.1:${appPort}`;

const normal = { length: 200, height: 120, itemHeight: 20, overscan: 2 };
const short = { ...normal, length: 2 };
const empty = { ...normal, length: 0 };
const expanded = { length: 200, height: 4000, itemHeight: 10, overscan: 0 };

let tempRoot = null;
let profile = null;
let browser = null;
let browserError = "";
let server = null;
let client = null;
const cdpRuntimeErrors = [];

try {
    if (compiler !== "go" && compiler !== "tinygo") {
        throw new Error(`HARNESS FAILURE: unsupported compiler ${JSON.stringify(compiler)}`);
    }
    tempRoot = await mkdtemp(join(tmpdir(), `goframe-virtual-range-${compiler}-`));
    profile = await mkdtemp(join(tmpdir(), "goframe-virtual-range-chrome-"));
    const artifact = await buildFixture();
    server = await startStaticServer(tempRoot, appPort);
    browser = await startBrowser(chrome, [
        "--headless",
        "--no-sandbox",
        "--disable-gpu",
        `--remote-debugging-port=${debugPort}`,
        `--user-data-dir=${profile}`,
        "about:blank",
    ]);
    const page = await waitForPage(debugPort);
    client = await connect(page.webSocketDebuggerUrl);
    client.on("Runtime.exceptionThrown", (params) => {
        cdpRuntimeErrors.push(params.exceptionDetails?.text ?? "runtime exception");
    });
    await client.call("Runtime.enable");
    await client.call("Page.enable");
    await client.call("Page.addScriptToEvaluateOnNewDocument", {
        source: `(${installBrowserAudit.toString()})()`,
    });

    const scenarios = {
        short: await runCollectionScenario("short", "control-short", short),
        empty: await runCollectionScenario("empty", "control-empty", empty),
        window: await runWindowScenario(),
    };
    const behaviorJSON = JSON.stringify(scenarios);
    const report = {
        compiler,
        artifact,
        behaviorSha256: createHash("sha256").update(behaviorJSON).digest("hex"),
        scenarios,
    };
    console.log(`virtual range restoration evidence: ${JSON.stringify(report)}`);
    console.log(`virtual range restoration browser smoke (${compiler}): ok`);
} catch (error) {
    throw new Error(`${error.message}\n${JSON.stringify(await diagnostics(), null, 2)}`, {
        cause: error,
    });
} finally {
    client?.close();
    await stopProcess(browser);
    if (server) {
        await new Promise((resolveClose) => server.close(resolveClose));
    }
    if (profile) {
        await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
    }
    if (tempRoot) {
        await rm(tempRoot, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
    }
}

async function runCollectionScenario(label, shrinkControl, shrinkContract) {
    let state = await navigateScenario(label);
    assertRangeState(state, normal, 0, `${label} initial`);
    const initialRootToken = state.rootToken;

    await scrollDistant();
    state = await pageState();
    const distantStart = maxRangeStart(normal);
    assertRangeState(state, normal, distantStart, `${label} distant scroll`);
    const distant = {
        listIDs: state.listIDs,
        tableIDs: state.tableIDs,
        listScrollTop: state.listScrollTop,
        tableScrollTop: state.tableScrollTop,
    };

    await click(shrinkControl);
    state = await pageState();
    assertRangeState(state, shrinkContract, 0, `${label} shrink`);
    if (state.listScrollTop !== 0 || state.tableScrollTop !== 0) {
        throw new Error(`APP FAILURE: ${label} shrink retained scroll offsets: ${JSON.stringify(state)}`);
    }
    assertCleanupCount(state, "list", distantStart, 1, `${label} list distant cleanup`);
    assertCleanupCount(state, "table", distantStart, 1, `${label} table distant cleanup`);

    await click("control-large");
    state = await pageState();
    assertRangeState(state, normal, 0, `${label} restore`);
    assert(!state.listIDs.includes(distantStart), `${label} restored stale list range ${distantStart}`);
    assert(!state.tableIDs.includes(distantStart), `${label} restored stale table range ${distantStart}`);
    assert(state.rootToken === initialRootToken, `${label} replaced the application root`);
    const cleanupsBeforeInteraction = {
        list: mapCount(state.cleanups, `list:${distantStart}`),
        table: mapCount(state.cleanups, `table:${distantStart}`),
    };

    state = await exerciseRestoredInteractions();
    assertRangeState(state, normal, 0, `${label} interaction`);
    assert(
        mapCount(state.cleanups, `list:${distantStart}`) === cleanupsBeforeInteraction.list,
        `${label} replayed list cleanup for ${distantStart}`,
    );
    assert(
        mapCount(state.cleanups, `table:${distantStart}`) === cleanupsBeforeInteraction.table,
        `${label} replayed table cleanup for ${distantStart}`,
    );
    assertFinalAudit(state, `${label} final`);
    return summarizeScenario(distant, state);
}

async function runWindowScenario() {
    let state = await navigateScenario("window");
    assertRangeState(state, normal, 0, "window initial");
    const initialRootToken = state.rootToken;

    await scrollDistant();
    state = await pageState();
    const distantStart = maxRangeStart(normal);
    assertRangeState(state, normal, distantStart, "window distant scroll");
    const distant = {
        listIDs: state.listIDs,
        tableIDs: state.tableIDs,
        listScrollTop: state.listScrollTop,
        tableScrollTop: state.tableScrollTop,
    };

    await click("control-window-expand");
    state = await pageState();
    assertRangeState(state, expanded, 0, "window expanded");

    await click("control-window-reset");
    state = await pageState();
    assertRangeState(state, normal, 0, "window restored");
    assert(!state.listIDs.includes(distantStart), `window restored stale list range ${distantStart}`);
    assert(!state.tableIDs.includes(distantStart), `window restored stale table range ${distantStart}`);
    assertCleanupCount(state, "list", distantStart, 1, "window list distant cleanup");
    assertCleanupCount(state, "table", distantStart, 1, "window table distant cleanup");
    assert(state.rootToken === initialRootToken, "window transition replaced the application root");

    state = await exerciseRestoredInteractions();
    assertRangeState(state, normal, 0, "window interaction");
    assertFinalAudit(state, "window final");
    return summarizeScenario(distant, state);
}

async function navigateScenario(label) {
    cdpRuntimeErrors.length = 0;
    await client.call("Page.navigate", {
        url: `${origin}/?scenario=${encodeURIComponent(label)}&run=${Date.now()}`,
    });
    for (let attempt = 0; attempt < 150; attempt++) {
        const ready = await client.evaluate(`(() => ({
            href: location.href,
            ready: Boolean(document.querySelector("[data-testid='virtual-range-fixture']")),
            fixture: Boolean(window.__virtualRangeFixture),
        }))()`);
        if (ready.href.startsWith("chrome-error://")) {
            throw new Error(`HARNESS FAILURE: ${label} loaded a Chrome error document`);
        }
        if (ready.ready && ready.fixture) {
            await settle();
            await client.evaluate(`(() => {
                const audit = window.__virtualRangeBrowserAudit;
                audit.listenerBaseline = audit.listenerBalance;
                audit.appRoot = document.querySelector("[data-testid='virtual-range-fixture']");
                audit.rootToken = ${JSON.stringify(label)} + ":" + Date.now();
                audit.appRoot.setAttribute("data-root-token", audit.rootToken);
            })()`);
            return await pageState();
        }
        await wait(100);
    }
    throw new Error(`HARNESS FAILURE: ${label} fixture did not become ready`);
}

async function scrollDistant() {
    await client.evaluate(`(() => {
        for (const selector of ["[data-testid='fixture-virtual-list']", "[data-testid='fixture-virtual-table']"]) {
            const viewport = document.querySelector(selector);
            viewport.scrollTop = 100000;
            viewport.dispatchEvent(new Event("scroll", { bubbles: true }));
        }
    })()`);
    await settle();
}

async function exerciseRestoredInteractions() {
    await click("fixture-list-select-0");
    await click("fixture-list-toggle-1");
    await click("fixture-table-select-0");
    await click("fixture-table-toggle-1");
    const state = await pageState();
    assert(state.listSelection === "list-selected:0", `list selection targeted wrong item: ${JSON.stringify(state)}`);
    assert(state.listToggle === "list-toggled:1", `list toggle targeted wrong item: ${JSON.stringify(state)}`);
    assert(state.tableSelection === "table-selected:0", `table selection targeted wrong item: ${JSON.stringify(state)}`);
    assert(state.tableToggle === "table-toggled:1", `table toggle targeted wrong item: ${JSON.stringify(state)}`);
    assertDeepEqual(
        state.interactions.slice(-4),
        ["list:select:0", "list:toggle:1", "table:select:0", "table:toggle:1"],
        "restored interaction targets",
    );
    return state;
}

async function click(testID) {
    await client.evaluate(`(() => {
        const target = document.querySelector(${JSON.stringify(`[data-testid='${testID}']`)});
        if (!target) throw new Error(${JSON.stringify(`missing ${testID}`)});
        target.click();
    })()`);
    await settle();
}

async function settle() {
    await client.evaluate(`new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)))`);
    await wait(30);
}

async function pageState() {
    const state = await client.evaluate(`(() => {
        const fixture = window.__virtualRangeFixture;
        const browserAudit = window.__virtualRangeBrowserAudit;
        const listViewport = document.querySelector("[data-testid='fixture-virtual-list']");
        const tableViewport = document.querySelector("[data-testid='fixture-virtual-table']");
        const listNodes = [...document.querySelectorAll("[data-testid^='fixture-list-item-']")];
        const tableNodes = [...document.querySelectorAll("[data-testid^='fixture-table-row-']")];
        const testIDs = [...document.querySelectorAll("[data-testid]")].map((node) => node.getAttribute("data-testid"));
        const duplicateTestIDs = testIDs.filter((value, index) => testIDs.indexOf(value) !== index);
        const app = document.querySelector("[data-testid='virtual-range-fixture']");
        const listFirst = listNodes[0]?.closest(".gf-virtual-item");
        const topRow = document.querySelector(".gf-virtual-table-spacer-top");
        const bottomRow = document.querySelector(".gf-virtual-table-spacer-bottom");
        return {
            length: Number(app?.getAttribute("data-length") ?? -1),
            height: Number(app?.getAttribute("data-height") ?? -1),
            itemHeight: Number(app?.getAttribute("data-item-height") ?? -1),
            overscan: Number(app?.getAttribute("data-overscan") ?? -1),
            listIDs: listNodes.map((node) => Number(node.getAttribute("data-testid").replace("fixture-list-item-", ""))),
            tableIDs: tableNodes.map((node) => Number(node.getAttribute("data-testid").replace("fixture-table-row-", ""))),
            listScrollTop: listViewport?.scrollTop ?? -1,
            tableScrollTop: tableViewport?.scrollTop ?? -1,
            listTop: Number.parseInt(listFirst?.style.top || "0", 10),
            listTotalHeight: Number.parseInt(document.querySelector(".gf-virtual-list-spacer")?.style.height || "0", 10),
            tableTop: Number.parseInt(topRow?.style.height || "0", 10),
            tableBottom: Number.parseInt(bottomRow?.style.height || "0", 10),
            listSelection: document.querySelector("[data-testid='list-selection']")?.textContent ?? "",
            listToggle: document.querySelector("[data-testid='list-toggle']")?.textContent ?? "",
            tableSelection: document.querySelector("[data-testid='table-selection']")?.textContent ?? "",
            tableToggle: document.querySelector("[data-testid='table-toggle']")?.textContent ?? "",
            mounts: { ...fixture.mounts },
            cleanups: { ...fixture.cleanups },
            interactions: [...fixture.interactions],
            runtimeErrors: [...fixture.runtimeErrors],
            appMounts: fixture.appMounts,
            appCleanups: fixture.appCleanups,
            listenerDelta: browserAudit.listenerBalance - browserAudit.listenerBaseline,
            browserErrors: [...browserAudit.errors],
            rootStable: browserAudit.appRoot === app,
            rootToken: app?.getAttribute("data-root-token") ?? "",
            duplicateTestIDs,
            duplicateKeyWarnings: [...(globalThis.goframeDuplicateKeyWarnings || [])],
        };
    })()`);
    state.cdpRuntimeErrors = [...cdpRuntimeErrors];
    return state;
}

function assertRangeState(state, contract, start, label) {
    assertDeepEqual(
        {
            length: state.length,
            height: state.height,
            itemHeight: state.itemHeight,
            overscan: state.overscan,
        },
        contract,
        `${label} contract`,
    );
    const end = start + windowSize(contract);
    const expectedIDs = range(start, end);
    assertDeepEqual(state.listIDs, expectedIDs, `${label} list IDs`);
    assertDeepEqual(state.tableIDs, expectedIDs, `${label} table IDs`);
    assert(state.listTop === start * contract.itemHeight, `${label} list top ${state.listTop}`);
    assert(state.listTotalHeight === contract.length * contract.itemHeight, `${label} list total ${state.listTotalHeight}`);
    assert(state.tableTop === start * contract.itemHeight, `${label} table top ${state.tableTop}`);
    assert(
        state.tableBottom === (contract.length - end) * contract.itemHeight,
        `${label} table bottom ${state.tableBottom}`,
    );
    assert(state.listIDs.length <= windowSize(contract), `${label} list mount bound`);
    assert(state.tableIDs.length <= windowSize(contract), `${label} table mount bound`);
    assert(state.duplicateTestIDs.length === 0, `${label} duplicate test IDs ${JSON.stringify(state.duplicateTestIDs)}`);
    assert(state.duplicateKeyWarnings.length === 0, `${label} duplicate key warnings ${JSON.stringify(state.duplicateKeyWarnings)}`);
    assert(state.runtimeErrors.length === 0, `${label} runtime errors ${JSON.stringify(state.runtimeErrors)}`);
    assert(state.browserErrors.length === 0, `${label} browser errors ${JSON.stringify(state.browserErrors)}`);
    assert(state.cdpRuntimeErrors.length === 0, `${label} CDP runtime errors ${JSON.stringify(state.cdpRuntimeErrors)}`);
    assert(state.appMounts === 1 && state.appCleanups === 0, `${label} app lifetime ${state.appMounts}/${state.appCleanups}`);
    assert(state.rootStable, `${label} application root identity changed`);
}

function assertFinalAudit(state, label) {
    assert(state.listenerDelta === 0, `${label} listener delta ${state.listenerDelta}`);
    for (const [key, cleanups] of Object.entries(state.cleanups)) {
        assert(cleanups <= mapCount(state.mounts, key), `${label} cleanup exceeds mounts for ${key}`);
    }
}

function assertCleanupCount(state, kind, id, expected, label) {
    const key = `${kind}:${id}`;
    assert(mapCount(state.mounts, key) === 1, `${label} mounts ${mapCount(state.mounts, key)}, want 1`);
    assert(mapCount(state.cleanups, key) === expected, `${label} cleanups ${mapCount(state.cleanups, key)}, want ${expected}`);
}

function summarizeScenario(distant, finalState) {
    return {
        distant,
        final: {
            listIDs: finalState.listIDs,
            tableIDs: finalState.tableIDs,
            listScrollTop: finalState.listScrollTop,
            tableScrollTop: finalState.tableScrollTop,
            interactions: finalState.interactions,
            listenerDelta: finalState.listenerDelta,
            appMounts: finalState.appMounts,
            appCleanups: finalState.appCleanups,
            runtimeErrors: finalState.runtimeErrors.length + finalState.browserErrors.length + finalState.cdpRuntimeErrors.length,
        },
    };
}

function windowSize(contract) {
    if (contract.length <= 0) return 0;
    const overscan = Math.max(0, contract.overscan);
    return Math.min(
        contract.length,
        Math.ceil(contract.height / contract.itemHeight) + 2 * overscan,
    );
}

function maxRangeStart(contract) {
    return contract.length - windowSize(contract);
}

function range(start, end) {
    return Array.from({ length: Math.max(0, end - start) }, (_, index) => start + index);
}

function mapCount(map, key) {
    return Number(map[key] ?? 0);
}

async function buildFixture() {
    const wasm = join(tempRoot, "bundle.wasm");
    const commonEnvironment = {
        ...process.env,
        GOCACHE: join(tempRoot, "gocache"),
        GOWORK: "off",
        GOFLAGS: "-buildvcs=false",
        GOTOOLCHAIN: goToolchain,
        XDG_CACHE_HOME: join(tempRoot, "xdg-cache"),
    };
    let runtimeSource;
    if (compiler === "go") {
        await runCommand("go", [
            "build",
            "-tags=goframe_debug",
            "-o",
            wasm,
            fixturePackage,
        ], {
            ...commonEnvironment,
            GOOS: "js",
            GOARCH: "wasm",
            CGO_ENABLED: "0",
        });
        const goRoot = (await runCommand("go", ["env", "GOROOT"], commonEnvironment)).trim();
        runtimeSource = await firstExistingPath([
            join(goRoot, "lib", "wasm", "wasm_exec.js"),
            join(goRoot, "misc", "wasm", "wasm_exec.js"),
        ]);
    } else {
        await runCommand("tinygo", [
            "build",
            "-target=wasm",
            "-no-debug",
            "-panic=trap",
            "-tags=goframe_debug",
            "-o",
            wasm,
            fixturePackage,
        ], commonEnvironment);
        const tinyGoRoot = (await runCommand("tinygo", ["env", "TINYGOROOT"], commonEnvironment)).trim();
        runtimeSource = join(tinyGoRoot, "targets", "wasm_exec.js");
    }
    await copyFile(runtimeSource, join(tempRoot, "wasm_exec.js"));
    await writeFile(join(tempRoot, "index.html"), fixtureHTML(), "utf8");
    const bytes = await readFile(wasm);
    return {
        bytes: bytes.length,
        sha256: createHash("sha256").update(bytes).digest("hex"),
    };
}

async function firstExistingPath(paths) {
    for (const candidate of paths) {
        try {
            if ((await stat(candidate)).isFile()) return candidate;
        } catch {
            // Try the next supported Go distribution layout.
        }
    }
    throw new Error(`HARNESS FAILURE: wasm_exec.js not found in ${paths.join(", ")}`);
}

function startStaticServer(directory, port) {
    const server = createHTTPServer(async (request, response) => {
        try {
            const pathname = new URL(request.url, origin).pathname;
            const name = pathname === "/" ? "index.html" : pathname.slice(1);
            if (!new Set(["index.html", "wasm_exec.js", "bundle.wasm"]).has(name)) {
                response.writeHead(404).end("not found");
                return;
            }
            const content = await readFile(join(directory, name));
            const mime = name.endsWith(".wasm")
                ? "application/wasm"
                : name.endsWith(".js")
                    ? "text/javascript"
                    : "text/html";
            response.writeHead(200, { "content-type": mime });
            response.end(content);
        } catch (error) {
            response.writeHead(500).end(error.message);
        }
    });
    return new Promise((resolveServer, reject) => {
        server.once("error", reject);
        server.listen(port, "127.0.0.1", () => resolveServer(server));
    });
}

function installBrowserAudit() {
    let listenerBalance = 0;
    const errors = [];
    const originalAdd = EventTarget.prototype.addEventListener;
    const originalRemove = EventTarget.prototype.removeEventListener;
    EventTarget.prototype.addEventListener = function(...args) {
        listenerBalance++;
        return originalAdd.apply(this, args);
    };
    EventTarget.prototype.removeEventListener = function(...args) {
        listenerBalance--;
        return originalRemove.apply(this, args);
    };
    window.__virtualRangeBrowserAudit = {
        appRoot: null,
        rootToken: "",
        listenerBaseline: 0,
        errors,
        get listenerBalance() {
            return listenerBalance;
        },
    };
    window.addEventListener("error", (event) => {
        errors.push(event.message || "window error");
    });
    window.addEventListener("unhandledrejection", (event) => {
        errors.push(String(event.reason));
    });
}

async function diagnostics() {
    const result = {
        compiler,
        appPort,
        debugPort,
        browserStderr: browserError.slice(-6000),
        cdpRuntimeErrors,
    };
    if (client) {
        try {
            result.page = await pageState();
        } catch (error) {
            result.pageError = error.message;
        }
    }
    return result;
}

function startBrowser(command, args) {
    return new Promise((resolveBrowser, reject) => {
        const child = spawn(command, args, { stdio: ["ignore", "ignore", "pipe"] });
        child.stderr.on("data", (chunk) => {
            browserError += chunk;
        });
        child.once("error", reject);
        child.once("spawn", () => resolveBrowser(child));
    });
}

async function waitForPage(port) {
    let lastError = null;
    for (let attempt = 0; attempt < 100; attempt++) {
        if (browser?.exitCode !== null) {
            throw new Error(`HARNESS FAILURE: Chrome exited before CDP was ready\n${browserError}`);
        }
        try {
            const response = await fetch(`http://127.0.0.1:${port}/json`);
            const targets = await response.json();
            const page = targets.find((entry) => entry.type === "page" && entry.webSocketDebuggerUrl);
            if (page) return page;
        } catch (error) {
            lastError = error;
        }
        await wait(100);
    }
    throw new Error(`HARNESS FAILURE: Chrome DevTools did not become ready: ${lastError?.message ?? ""}`);
}

async function connect(url) {
    const socket = new WebSocket(url);
    await new Promise((resolveOpen, reject) => {
        socket.addEventListener("open", resolveOpen, { once: true });
        socket.addEventListener("error", reject, { once: true });
    });
    let nextID = 1;
    let closedError = null;
    const pending = new Map();
    const listeners = new Map();
    const terminate = (event) => {
        if (closedError) return;
        closedError = new Error(`HARNESS FAILURE: CDP socket terminated: ${event.type}`);
        for (const request of pending.values()) request.reject(closedError);
        pending.clear();
    };
    socket.addEventListener("close", terminate);
    socket.addEventListener("error", terminate);
    socket.addEventListener("message", (event) => {
        if (closedError) return;
        const message = JSON.parse(event.data);
        if (message.id && pending.has(message.id)) {
            const request = pending.get(message.id);
            pending.delete(message.id);
            if (message.error) request.reject(new Error(message.error.message));
            else request.resolve(message.result);
            return;
        }
        for (const listener of listeners.get(message.method) ?? []) {
            listener(message.params ?? {});
        }
    });
    return {
        call(method, params = {}) {
            if (closedError) return Promise.reject(closedError);
            return new Promise((resolveCall, reject) => {
                const id = nextID++;
                pending.set(id, { resolve: resolveCall, reject });
                try {
                    socket.send(JSON.stringify({ id, method, params }));
                } catch (error) {
                    pending.delete(id);
                    reject(error);
                    terminate({ type: "send" });
                }
            });
        },
        async evaluate(expression) {
            const result = await this.call("Runtime.evaluate", {
                expression,
                returnByValue: true,
                awaitPromise: true,
            });
            if (result.exceptionDetails) {
                throw new Error(`APP FAILURE: browser evaluation failed: ${JSON.stringify(result.exceptionDetails)}`);
            }
            return result.result.value;
        },
        on(method, listener) {
            const current = listeners.get(method) ?? [];
            current.push(listener);
            listeners.set(method, current);
        },
        close() {
            if (!closedError) socket.close();
        },
    };
}

function runCommand(command, args, environment) {
    return new Promise((resolveCommand, reject) => {
        const child = spawn(command, args, {
            cwd: rootDir,
            stdio: ["ignore", "pipe", "pipe"],
            env: environment,
        });
        let output = "";
        child.stdout.on("data", (chunk) => {
            output += chunk;
        });
        child.stderr.on("data", (chunk) => {
            output += chunk;
        });
        child.once("error", reject);
        child.once("exit", (code, signal) => {
            if (code === 0) {
                resolveCommand(output);
                return;
            }
            reject(new Error(
                `HARNESS FAILURE: ${command} ${args.join(" ")} failed with ${signal ?? code}\n${output}`,
            ));
        });
    });
}

async function stopProcess(child) {
    if (!child || child.exitCode !== null || child.signalCode !== null) return;
    const exited = new Promise((resolveExit) => child.once("exit", resolveExit));
    child.kill("SIGTERM");
    const stopped = await Promise.race([
        exited.then(() => true),
        wait(2000).then(() => false),
    ]);
    if (!stopped && child.exitCode === null && child.signalCode === null) {
        child.kill("SIGKILL");
        await Promise.race([exited, wait(1000)]);
    }
}

function pickFreePort() {
    return new Promise((resolvePort, reject) => {
        const server = createPortServer();
        server.once("error", reject);
        server.listen(0, "127.0.0.1", () => {
            const address = server.address();
            server.close(() => resolvePort(address.port));
        });
    });
}

function assert(condition, message) {
    if (!condition) throw new Error(`APP FAILURE: ${message}`);
}

function assertDeepEqual(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        throw new Error(`APP FAILURE: ${label}: got ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`);
    }
}

function wait(milliseconds) {
    return new Promise((resolveWait) => setTimeout(resolveWait, milliseconds));
}

function fixtureHTML() {
    return `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>Virtual range restoration fixture</title>
  <style>
    body { font-family: sans-serif; margin: 0; }
    main { padding: 12px; }
    #root { min-height: 100vh; }
    .gf-virtual-list, .gf-virtual-table-viewport { border: 1px solid #888; margin: 8px 0; }
    .fixture-list-item, .fixture-table-row { box-sizing: border-box; height: 100%; }
    table { border-collapse: collapse; width: 100%; }
  </style>
</head>
<body>
  <div id="root"></div>
  <script src="wasm_exec.js"></script>
  <script>
    const go = new Go();
    WebAssembly.instantiateStreaming(fetch("bundle.wasm"), go.importObject)
      .then((result) => go.run(result.instance))
      .catch((error) => { throw error; });
  </script>
</body>
</html>`;
}

import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const appURL = process.argv[2] ?? process.env.GOFRAME_CONTEXT_SELECTOR_TOPOLOGY_SMOKE_URL ?? "http://127.0.0.1:18080/";
const debugPort = Number(process.env.GOFRAME_CONTEXT_SELECTOR_TOPOLOGY_CHROME_DEBUG_PORT ?? "19248");
const chrome = process.env.CHROME ?? "google-chrome";
const profile = await mkdtemp(join(tmpdir(), "goframe-context-selector-topology-smoke-"));
const expectedApp = new URL(appURL);
const browser = spawn(chrome, [
    "--headless",
    "--no-sandbox",
    "--disable-gpu",
    `--remote-debugging-port=${debugPort}`,
    `--user-data-dir=${profile}`,
    "about:blank",
], {
    stdio: ["ignore", "ignore", "pipe"],
});

let browserError = "";
let browserExit = null;
browser.stderr.on("data", (chunk) => {
    browserError += chunk;
});
browser.on("exit", (code, signal) => {
    browserExit = { code, signal };
});

try {
    const page = await waitForPage(debugPort);
    const client = await connect(page.webSocketDebuggerUrl);
    await client.call("Runtime.enable");
    await client.call("Page.enable");
    await client.call("Page.navigate", { url: withSmokeParam(appURL, "topology") });
    await waitForAppPage(client, expectedApp);

    await client.evaluate(`(() => {
        window.__contextTopologyFixtureRoot = document.querySelector("#context-selector-topology-fixture");
        return Boolean(window.__contextTopologyFixtureRoot);
    })()`);

    let state = await topologyState(client);
    assertState(state, {
        activeProvider: "outer",
        outerPayload: "outer-safe",
        innerPayload: "inactive",
        selectedSource: "outer",
        selectedPayload: "outer-safe",
        appRenders: 1,
        consumerRenders: 1,
        selectorCalls: 1,
        errorCount: 0,
        sameRoot: true,
    }, "initial outer provider");

    await click(client, "#show-failing-inner");
    state = await waitForTopologyState(client, "failing inner provider", (next) =>
        next.activeProvider === "inner" && next.errorCount === 1);
    assertState(state, {
        activeProvider: "inner",
        innerPayload: "inner-failing",
        selectedSource: "outer",
        selectedPayload: "outer-safe",
        appRenders: 1,
        consumerRenders: 1,
        selectorCalls: 2,
        errorCount: 1,
        sameRoot: true,
    }, "failed outer-to-inner topology refresh");
    assertContextError(state.errors[0], "inner selector boom", "inner topology failure");

    await click(client, "#update-shadowed-outer");
    state = await waitForTopologyState(client, "shadowed outer update", (next) =>
        next.outerPayload === "outer-shadowed");
    assertState(state, {
        activeProvider: "inner",
        outerPayload: "outer-shadowed",
        selectedSource: "outer",
        selectedPayload: "outer-safe",
        appRenders: 1,
        consumerRenders: 1,
        selectorCalls: 2,
        errorCount: 1,
        sameRoot: true,
    }, "shadowed outer provider ignored after failed rebind");

    await click(client, "#make-inner-safe");
    state = await waitForTopologyState(client, "safe inner recovery", (next) =>
        next.selectedPayload === "inner-safe");
    assertState(state, {
        activeProvider: "inner",
        selectedSource: "inner",
        selectedPayload: "inner-safe",
        appRenders: 1,
        consumerRenders: 2,
        selectorCalls: 4,
        errorCount: 1,
        sameRoot: true,
    }, "safe inner provider recovery");

    await click(client, "#make-outer-failing");
    state = await waitForTopologyState(client, "shadowed outer failure prepared", (next) =>
        next.outerPayload === "outer-failing");
    assertState(state, {
        activeProvider: "inner",
        selectedSource: "inner",
        selectedPayload: "inner-safe",
        appRenders: 1,
        consumerRenders: 2,
        selectorCalls: 4,
        errorCount: 1,
        sameRoot: true,
    }, "failing outer remains shadowed");

    await click(client, "#remove-inner");
    state = await waitForTopologyState(client, "failing outer revealed", (next) =>
        next.activeProvider === "outer" && next.errorCount === 2);
    assertState(state, {
        activeProvider: "outer",
        innerPayload: "inactive",
        selectedSource: "inner",
        selectedPayload: "inner-safe",
        appRenders: 1,
        consumerRenders: 2,
        selectorCalls: 5,
        errorCount: 2,
        sameRoot: true,
    }, "failed inner-to-outer topology refresh");
    assertContextError(state.errors[1], "outer selector boom", "outer topology failure");

    await click(client, "#make-outer-safe");
    state = await waitForTopologyState(client, "safe outer recovery", (next) =>
        next.selectedPayload === "outer-safe-recovered");
    assertState(state, {
        activeProvider: "outer",
        selectedSource: "outer",
        selectedPayload: "outer-safe-recovered",
        appRenders: 1,
        consumerRenders: 3,
        selectorCalls: 7,
        errorCount: 2,
        sameRoot: true,
    }, "safe outer provider recovery");

    await click(client, "#make-inner-safe");
    state = await waitForTopologyState(client, "post-recovery interaction", (next) =>
        next.activeProvider === "inner" && next.selectedPayload === "inner-safe");
    assertState(state, {
        activeProvider: "inner",
        selectedSource: "inner",
        selectedPayload: "inner-safe",
        appRenders: 1,
        consumerRenders: 4,
        selectorCalls: 9,
        errorCount: 2,
        sameRoot: true,
    }, "fixture remains interactive after contained failures");

    client.close();
    console.log(`context selector topology evidence: ${JSON.stringify(state)}`);
    console.log("Context selector topology browser smoke: ok");
} finally {
    const exited = new Promise((resolve) => browser.once("exit", resolve));
    browser.kill("SIGTERM");
    await Promise.race([exited, wait(2000)]);
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function click(client, selector) {
    const clicked = await client.callFunction(`function(selector) {
        const element = document.querySelector(selector);
        if (!element) return false;
        element.click();
        return true;
    }`, selector);
    if (!clicked) {
        throw new Error(`APP FAILURE: missing element for click ${selector}`);
    }
}

async function topologyState(client) {
    return await client.evaluate(`(() => {
        const text = (selector) => document.querySelector(selector)?.textContent ?? "";
        const count = (selector) => Number(document.querySelector(selector)?.dataset.renderCount ?? "0");
        const errors = Array.from(globalThis.goframeContextTopologyErrors || []).map((error) => ({
            phase: error.phase,
            component: error.component,
            operation: error.operation,
            panic: error.panic,
        }));
        const root = document.querySelector("#context-selector-topology-fixture");
        return {
            activeProvider: text("#active-provider"),
            outerPayload: text("#outer-provider-payload"),
            innerPayload: text("#inner-provider-payload"),
            selectedSource: text("#selected-source"),
            selectedPayload: text("#selected-payload"),
            appRenders: Number(root?.dataset.appRenderCount ?? "0"),
            outerRenders: count("#outer-provider"),
            innerRenders: count("#inner-scope"),
            consumerRenders: count("#selector-consumer"),
            selectorCalls: Number(globalThis.goframeContextTopologySelectorCalls ?? 0),
            errorCount: errors.length,
            errors,
            sameRoot: Boolean(root && window.__contextTopologyFixtureRoot === root),
        };
    })()`);
}

async function waitForTopologyState(client, label, predicate) {
    const started = Date.now();
    let state = null;
    while (Date.now() - started < 5000) {
        state = await topologyState(client);
        if (predicate(state)) {
            return state;
        }
        await wait(50);
    }
    throw new Error(`APP FAILURE: timed out waiting for ${label}: ${JSON.stringify(state)}`);
}

function assertState(actual, expected, label) {
    for (const [key, value] of Object.entries(expected)) {
        if (actual[key] !== value) {
            throw new Error(`APP FAILURE: ${label} ${key} got ${JSON.stringify(actual[key])}, want ${JSON.stringify(value)}; state=${JSON.stringify(actual)}`);
        }
    }
    console.log(`${label}: ok`);
}

function assertContextError(actual, panicText, label) {
    const expected = {
        phase: "context",
        component: "SelectorConsumer",
        operation: "UseContextSelector",
        panic: panicText,
    };
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        throw new Error(`APP FAILURE: ${label} got ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`);
    }
}

async function waitForPage(port) {
    const started = Date.now();
    let lastError;
    while (Date.now() - started < 5000) {
        if (browserExit) {
            throw new Error(`HARNESS FAILURE: Chrome exited before CDP was ready: ${JSON.stringify(browserExit)}\n${browserError}`);
        }
        try {
            const response = await fetch(`http://127.0.0.1:${port}/json`);
            if (response.ok) {
                const targets = await response.json();
                const page = targets.find((entry) => entry.type === "page" && entry.webSocketDebuggerUrl);
                if (page) return page;
            }
        } catch (error) {
            lastError = error;
        }
        await wait(100);
    }
    throw new Error(`HARNESS FAILURE: Chrome DevTools page unavailable: ${lastError?.message ?? browserError}`);
}

async function waitForAppPage(client, expected) {
    const started = Date.now();
    let state = null;
    while (Date.now() - started < 8000) {
        state = await client.evaluate(`(() => ({
            href: window.location.href,
            ready: Boolean(document.querySelector("#context-selector-topology-fixture")),
        }))()`);
        if (state.href.startsWith("chrome-error://")) {
            throw new Error(`HARNESS FAILURE: Chrome loaded an error document: ${JSON.stringify(state)}`);
        }
        const actual = new URL(state.href);
        if (actual.origin === expected.origin && actual.pathname === expected.pathname && state.ready) {
            return;
        }
        await wait(100);
    }
    throw new Error(`HARNESS FAILURE: context selector topology app did not become ready: ${JSON.stringify(state)}`);
}

function withSmokeParam(url, label) {
    const next = new URL(url);
    next.searchParams.set("smoke", `${Date.now()}-${label}`);
    return next.toString();
}

function connect(url) {
    const socket = new WebSocket(url);
    let nextID = 1;
    const pending = new Map();

    return new Promise((resolve, reject) => {
        socket.addEventListener("open", () => {
            resolve({
                call(method, params = {}) {
                    const id = nextID++;
                    socket.send(JSON.stringify({ id, method, params }));
                    return new Promise((callResolve, callReject) => {
                        pending.set(id, { resolve: callResolve, reject: callReject });
                    });
                },
                async evaluate(expression) {
                    const response = await this.call("Runtime.evaluate", {
                        expression,
                        awaitPromise: true,
                        returnByValue: true,
                    });
                    if (response.exceptionDetails) {
                        throw new Error(`browser evaluation failed: ${JSON.stringify(response.exceptionDetails)}`);
                    }
                    return response.result.value;
                },
                async callFunction(functionDeclaration, ...args) {
                    if (!this.globalObjectID) {
                        const globalResponse = await this.call("Runtime.evaluate", {
                            expression: "globalThis",
                            returnByValue: false,
                        });
                        if (globalResponse.exceptionDetails) {
                            throw new Error(`browser evaluation failed: ${JSON.stringify(globalResponse.exceptionDetails)}`);
                        }
                        this.globalObjectID = globalResponse.result.objectId;
                    }
                    const response = await this.call("Runtime.callFunctionOn", {
                        objectId: this.globalObjectID,
                        functionDeclaration,
                        arguments: args.map((value) => ({ value })),
                        awaitPromise: true,
                        returnByValue: true,
                    });
                    if (response.exceptionDetails) {
                        throw new Error(`browser evaluation failed: ${JSON.stringify(response.exceptionDetails)}`);
                    }
                    return response.result.value;
                },
                close() {
                    socket.close();
                },
            });
        }, { once: true });
        socket.addEventListener("error", reject, { once: true });
        socket.addEventListener("message", (event) => {
            const message = JSON.parse(event.data);
            if (!message.id || !pending.has(message.id)) return;
            const request = pending.get(message.id);
            pending.delete(message.id);
            if (message.error) {
                request.reject(new Error(message.error.message));
                return;
            }
            request.resolve(message.result);
        });
    });
}

function wait(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const appURL = process.argv[2] ?? process.env.GOFRAME_RUNTIME_ERRORS_SMOKE_URL ?? "http://127.0.0.1:18080/";
const debugPort = Number(process.env.GOFRAME_RUNTIME_ERRORS_CHROME_DEBUG_PORT ?? "19240");
const chrome = process.env.CHROME ?? "google-chrome";
const profile = await mkdtemp(join(tmpdir(), "goframe-runtime-errors-smoke-"));
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
    await navigateToApp(client, withSmokeParam(appURL, "runtime-errors"));
    await waitForAppPage(client, expectedApp, "runtime-errors navigation");
    await waitForReport(client, "effect", "Effect setup report");

    await click(client, "#event-panic");
    await waitForReport(client, "event", "Event handler report");

    await click(client, "#increment");
    await waitForText(client, "#runtime-error-count", "1", "increment after event panic");

    await click(client, "#toggle-cleanup");
    await waitForAbsent(client, "#cleanup-panel", "cleanup panel unmount");
    await waitForReport(client, "effect-cleanup", "Effect cleanup report");
    await waitForReport(client, "unmount-cleanup", "Unmount cleanup report");

    await click(client, "#increment");
    await waitForText(client, "#runtime-error-count", "2", "increment after cleanup panic");

    const final = await client.evaluate(`(() => ({
        mounted: Boolean(document.querySelector("#runtime-error-fixture")),
        reports: globalThis.goframeRuntimeErrorReports || [],
    }))()`);
    if (!final.mounted) {
        throw new Error("APP FAILURE: runtime error fixture unmounted after recoverable errors");
    }
    const phases = final.reports.map((report) => report.phase);
    for (const phase of ["effect", "event", "effect-cleanup", "unmount-cleanup"]) {
        if (!phases.includes(phase)) {
            throw new Error(`APP FAILURE: missing runtime error phase ${phase}: ${JSON.stringify(final.reports)}`);
        }
    }

    client.close();
    console.log("Runtime errors browser smoke: ok");
} finally {
    const exited = new Promise((resolve) => browser.once("exit", resolve));
    browser.kill("SIGTERM");
    await Promise.race([exited, wait(2000)]);
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function click(client, selector) {
    const result = await client.callFunction(`function(selector) {
        const element = document.querySelector(selector);
        if (!element) return false;
        element.click();
        return true;
    }`, selector);
    if (!result) {
        throw new Error(`APP FAILURE: missing element for click ${selector}`);
    }
}

async function waitForReport(client, phase, label) {
    await waitUntil(client, label, () =>
        client.callFunction(`function(phase) {
            const reports = globalThis.goframeRuntimeErrorReports || [];
            return reports.some((report) => report.phase === phase);
        }`, phase));
}

async function waitForText(client, selector, expected, label) {
    await waitUntil(client, label, () =>
        client.callFunction(`function(selector, expected) {
            const element = document.querySelector(selector);
            return element ? element.textContent === expected : false;
        }`, selector, expected));
}

async function waitForAbsent(client, selector, label) {
    await waitUntil(client, label, () =>
        client.callFunction(`function(selector) {
            return !document.querySelector(selector);
        }`, selector));
}

async function waitUntil(client, label, predicate) {
    const started = Date.now();
    let lastValue = null;
    while (Date.now() - started < 5000) {
        lastValue = await predicate();
        if (lastValue === true) {
            return;
        }
        await wait(100);
    }
    throw new Error(`APP FAILURE: timed out waiting for ${label}; last=${JSON.stringify(lastValue)}`);
}

async function waitForPage(port) {
    const started = Date.now();
    let lastError;
    while (Date.now() - started < 5000) {
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
    throw new Error(`HARNESS FAILURE: Chrome DevTools page unavailable: ${lastError?.message ?? browserError}`);
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

async function waitForAppPage(client, expected, label) {
    let lastState = null;
    const started = Date.now();
    while (Date.now() - started < 8000) {
        lastState = await pageState(client);
        if (lastState.href.startsWith("chrome-error://")) {
            throw await harnessFailure(client, `${label}: Chrome loaded an error document`, lastState);
        }
        if (isExpectedAppState(lastState, expected) && lastState.fixtureReady && lastState.storage === "available") {
            return lastState;
        }
        await wait(100);
    }
    throw await harnessFailure(client, `${label}: app page did not become ready`, lastState);
}

async function pageState(client) {
    return await client.evaluate(`(() => {
        let storage = "available";
        try {
            window.localStorage.length;
        } catch (error) {
            storage = error.name + ": " + error.message;
        }
        return {
            href: window.location.href,
            origin: window.location.origin,
            protocol: window.location.protocol,
            readyState: document.readyState,
            fixtureReady: Boolean(document.querySelector("#runtime-error-fixture")),
            storage,
        };
    })()`);
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
    return new Error(`HARNESS FAILURE: ${message}\n${JSON.stringify({ appURL, debugPort, detail, diagnostics }, null, 2)}`);
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
    if (browserExit) {
        diagnostics.browserExit = browserExit;
    }
    if (browserError) {
        diagnostics.browserStderr = browserError.slice(-4000);
    }
    return diagnostics;
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
            if (!message.id || !pending.has(message.id)) {
                return;
            }
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

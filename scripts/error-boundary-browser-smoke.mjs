import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const appURL = process.argv[2] ?? process.env.GOFRAME_ERROR_BOUNDARY_SMOKE_URL ?? "http://127.0.0.1:18080/";
const debugPort = Number(process.env.GOFRAME_ERROR_BOUNDARY_CHROME_DEBUG_PORT ?? "19241");
const chrome = process.env.CHROME ?? "google-chrome";
const profile = await mkdtemp(join(tmpdir(), "goframe-error-boundary-smoke-"));
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
    await navigateToApp(client, withSmokeParam(appURL, "error-boundary"));
    await waitForAppPage(client, expectedApp, "error-boundary navigation");
    await waitForText(client, "[data-testid='eb-protected-state']", "0", "initial protected state");
    await waitForProbe(client, (probe) => probe.effectCount === 1, "initial effect setup");
    await captureShell(client);
    await installListenerAudit(client);

    await click(client, "[data-testid='eb-protected-increment']");
    await waitForText(client, "[data-testid='eb-protected-state']", "1", "protected state before failure");
    await waitForProbe(client, (probe) => probe.effectCount === 2 && probe.cleanupCount === 1, "effect rerun before failure");
    const beforeFailure = await probe(client);

    await click(client, "[data-testid='eb-trigger-render-error']");
    await waitForSelector(client, "[data-testid='eb-fallback']", "boundary fallback");
    await waitForAbsent(client, "[data-testid='eb-protected']", "failed protected subtree removed");
    await waitForText(client, "[data-testid='eb-error-component']", "RiskyPanel", "fallback component name");
    await waitForText(client, "[data-testid='eb-error-operation']", "component render", "fallback operation");
    await waitForProbe(client, (current) =>
        current.reports.length === beforeFailure.reports.length + 1 &&
        current.reports.at(-1).phase === "render" &&
        current.reports.at(-1).component === "RiskyPanel" &&
        current.effectCount === beforeFailure.effectCount &&
        current.cleanupCount === beforeFailure.cleanupCount + 1,
    "render failure report and cleanup");
    await assertShellSame(client, "shell after fallback");

    const afterFallback = await probe(client);
    await click(client, "[data-testid='eb-retry']");
    await waitForSelector(client, "[data-testid='eb-protected']", "protected subtree after retry");
    await waitForAbsent(client, "[data-testid='eb-fallback']", "fallback cleared after retry");
    await waitForText(client, "[data-testid='eb-protected-state']", "0", "retry remounts fresh protected state");
    await waitForProbe(client, (current) =>
        current.reports.length === afterFallback.reports.length &&
        current.listenerAudit.add === current.listenerAudit.remove,
    "retry does not report or leak listeners");

    const beforeSecondFailure = await probe(client);
    await click(client, "[data-testid='eb-trigger-render-error']");
    await waitForSelector(client, "[data-testid='eb-fallback']", "second fallback");
    await waitForProbe(client, (current) => current.reports.length === beforeSecondFailure.reports.length + 1, "second incident report");
    await click(client, "[data-testid='eb-reset-key']");
    await waitForSelector(client, "[data-testid='eb-protected']", "ResetKey clears fallback");
    await waitForAbsent(client, "[data-testid='eb-fallback']", "fallback cleared by ResetKey");

    const beforeNested = await probe(client);
    await click(client, "[data-testid='eb-trigger-nested-error']");
    await waitForSelector(client, "[data-testid='eb-nested-inner-fallback']", "nested inner fallback");
    await waitForAbsent(client, "[data-testid='eb-nested-outer-fallback']", "outer fallback stays inactive");
    await waitForProbe(client, (current) => current.reports.length === beforeNested.reports.length + 1, "nested inner report");

    const beforeFallbackPanic = await probe(client);
    await click(client, "[data-testid='eb-trigger-inner-fallback-panic']");
    await waitForSelector(client, "[data-testid='eb-nested-outer-fallback']", "inner fallback panic bubbles to outer");
    await waitForProbe(client, (current) =>
        current.reports.length === beforeFallbackPanic.reports.length + 1 &&
        current.reports.at(-1).component === "ErrorBoundary",
    "fallback panic report");

    const beforeNoBoundary = await probe(client);
    await click(client, "[data-testid='eb-trigger-no-boundary-error']");
    await waitForAbsent(client, "[data-testid='eb-no-boundary-healthy']", "no-boundary subtree default Empty fallback");
    await waitForProbe(client, (current) =>
        current.reports.length === beforeNoBoundary.reports.length + 1 &&
        current.reports.at(-1).component === "NoBoundaryRisky",
    "no-boundary render report");
    await assertShellSame(client, "shell after no-boundary failure");

    client.close();
    console.log("Error boundary browser smoke: ok");
} finally {
    const exited = new Promise((resolve) => browser.once("exit", resolve));
    browser.kill("SIGTERM");
    await Promise.race([exited, wait(2000)]);
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function installListenerAudit(client) {
    await client.evaluate(`(() => {
        if (window.__errorBoundaryListenerAudit) return true;
        const originalAdd = EventTarget.prototype.addEventListener;
        const originalRemove = EventTarget.prototype.removeEventListener;
        const audit = { add: 0, remove: 0 };
        EventTarget.prototype.addEventListener = function(...args) {
            audit.add++;
            return originalAdd.apply(this, args);
        };
        EventTarget.prototype.removeEventListener = function(...args) {
            audit.remove++;
            return originalRemove.apply(this, args);
        };
        window.__errorBoundaryListenerAudit = audit;
        return true;
    })()`);
}

async function captureShell(client) {
    const ok = await client.evaluate(`(() => {
        window.__errorBoundaryShell = document.querySelector("[data-testid='eb-shell']");
        return Boolean(window.__errorBoundaryShell);
    })()`);
    if (!ok) {
        throw new Error("APP FAILURE: missing shell for identity capture");
    }
}

async function assertShellSame(client, label) {
    const same = await client.evaluate(`(() => window.__errorBoundaryShell === document.querySelector("[data-testid='eb-shell']"))()`);
    if (!same) {
        throw new Error(`APP FAILURE: shell identity changed during ${label}`);
    }
}

async function probe(client) {
    return await client.evaluate(`(() => ({
        effectCount: globalThis.goframeErrorBoundaryEffectCount ?? 0,
        cleanupCount: globalThis.goframeErrorBoundaryCleanupCount ?? 0,
        reports: Array.from(globalThis.goframeErrorBoundaryReports || []),
        listenerAudit: globalThis.__errorBoundaryListenerAudit || { add: 0, remove: 0 },
    }))()`);
}

async function click(client, selector) {
    const result = await client.evaluate(`(() => {
        const element = document.querySelector(${JSON.stringify(selector)});
        if (!element) return false;
        element.click();
        return true;
    })()`);
    if (!result) {
        throw new Error(`APP FAILURE: missing element for click ${selector}`);
    }
}

async function waitForProbe(client, predicate, label) {
    const started = Date.now();
    let last = null;
    while (Date.now() - started < 5000) {
        last = await probe(client);
        if (predicate(last)) {
            return;
        }
        await wait(100);
    }
    throw new Error(`APP FAILURE: timed out waiting for ${label}; last=${JSON.stringify(last)}`);
}

async function waitForSelector(client, selector, label) {
    await waitUntil(client, label, `(() => Boolean(document.querySelector(${JSON.stringify(selector)})))()`);
}

async function waitForText(client, selector, expected, label) {
    await waitUntil(client, label, `(() => {
        const element = document.querySelector(${JSON.stringify(selector)});
        return element ? element.textContent === ${JSON.stringify(expected)} : false;
    })()`);
}

async function waitForAbsent(client, selector, label) {
    await waitUntil(client, label, `(() => !document.querySelector(${JSON.stringify(selector)}))()`);
}

async function waitUntil(client, label, expression) {
    const started = Date.now();
    let lastValue = null;
    while (Date.now() - started < 5000) {
        lastValue = await client.evaluate(expression);
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
            fixtureReady: Boolean(document.querySelector("[data-testid='eb-shell']")),
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
                        throw new Error(response.exceptionDetails.text);
                    }
                    return response.result.result.value;
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
            request.resolve(message);
        });
    });
}

function wait(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

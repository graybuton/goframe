import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const appURL = process.argv[2] ?? process.env.GOFRAME_CONTEXT_SMOKE_URL ?? "http://127.0.0.1:18080/";
const debugPort = Number(process.env.GOFRAME_CONTEXT_CHROME_DEBUG_PORT ?? "19226");
const chrome = process.env.CHROME ?? "google-chrome";
const profile = await mkdtemp(join(tmpdir(), "goframe-context-smoke-"));
const expectedApp = new URL(appURL);
const componentNames = [
    "App",
    "PreferencesProvider",
    "PreferencesControls",
    "ConsumerGrid",
    "DensityConsumer",
    "AccentConsumer",
    "CounterConsumer",
    "BroadConsumer",
    "StaticPanel",
    "NestedAccentProvider",
    "InnerAccentConsumer",
];

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

    await navigateToApp(client, withSmokeParam(appURL, "initial"));
    await waitForAppPage(client, expectedApp, "initial navigation");

    assertDeepEqual(
        await contextState(client),
        {
            density: "comfortable",
            accent: "blue",
            counter: "0",
            broad: "comfortable / blue / 0",
            innerAccent: "purple",
            staticText: "unchanged",
            appRenders: 1,
        },
        "context initial render",
    );

    assertDeepEqual(
        await client.evaluate(installContextAuditExpression(componentNames)),
        { ready: true },
        "context performance audit installed",
    );

    await startScenario(client, "density-compact");
    await setSelectValue(client, "#density-select", "compact");
    const densityReport = await finishScenario(client, "density-compact");
    assertSelectorReport(densityReport, "DensityConsumer", "density change");
    assertDeepEqual(
        await contextState(client),
        {
            density: "compact",
            accent: "blue",
            counter: "0",
            broad: "compact / blue / 0",
            innerAccent: "purple",
            staticText: "unchanged",
            appRenders: 1,
        },
        "context density update",
    );

    await startScenario(client, "accent-green");
    await setSelectValue(client, "#accent-select", "green");
    const accentReport = await finishScenario(client, "accent-green");
    assertSelectorReport(accentReport, "AccentConsumer", "accent change");
    assertDeepEqual(
        await contextState(client),
        {
            density: "compact",
            accent: "green",
            counter: "0",
            broad: "compact / green / 0",
            innerAccent: "purple",
            staticText: "unchanged",
            appRenders: 1,
        },
        "context accent update",
    );

    await startScenario(client, "counter-increment");
    await client.evaluate(`document.querySelector("#increment-counter").click()`);
    await wait(120);
    const counterReport = await finishScenario(client, "counter-increment");
    assertSelectorReport(counterReport, "CounterConsumer", "counter increment");
    assertDeepEqual(
        await contextState(client),
        {
            density: "compact",
            accent: "green",
            counter: "1",
            broad: "compact / green / 1",
            innerAccent: "purple",
            staticText: "unchanged",
            appRenders: 1,
        },
        "context counter update",
    );

    await startScenario(client, "reset");
    await client.evaluate(`document.querySelector("#reset-preferences").click()`);
    await wait(120);
    const resetReport = await finishScenario(client, "reset");
    assertResetReport(resetReport);
    assertDeepEqual(
        await contextState(client),
        {
            density: "comfortable",
            accent: "blue",
            counter: "0",
            broad: "comfortable / blue / 0",
            innerAccent: "purple",
            staticText: "unchanged",
            appRenders: 1,
        },
        "context reset",
    );

    client.close();
    console.log("Context browser smoke: ok");
} finally {
    const exited = new Promise((resolve) => browser.once("exit", resolve));
    browser.kill("SIGTERM");
    await Promise.race([exited, wait(2000)]);
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function contextState(client) {
    return await client.evaluate(`(() => ({
        density: document.querySelector("[data-testid='density-consumer'] strong")?.textContent.trim(),
        accent: document.querySelector("[data-testid='accent-consumer'] strong")?.textContent.trim(),
        counter: document.querySelector("[data-testid='counter-consumer'] strong")?.textContent.trim(),
        broad: document.querySelector("[data-testid='broad-consumer'] strong")?.textContent.trim(),
        innerAccent: document.querySelector("[data-testid='inner-accent-consumer']")?.textContent.trim(),
        staticText: document.querySelector("[data-testid='static-panel'] strong")?.textContent.trim(),
        appRenders: window.goframeComponentRenderCounts.App,
    }))()`);
}

async function setSelectValue(client, selector, value) {
    await client.evaluate(`(() => {
        const select = document.querySelector(${JSON.stringify(selector)});
        select.value = ${JSON.stringify(value)};
        select.dispatchEvent(new Event("change", { bubbles: true }));
        return true;
    })()`);
    await wait(140);
}

async function startScenario(client, label) {
    await client.evaluate(`(() => {
        window.__contextAudit.start(${JSON.stringify(label)});
        return true;
    })()`);
}

async function finishScenario(client, label) {
    const report = await client.evaluate(`(() => window.__contextAudit.finish(${JSON.stringify(label)}))()`);
    console.log(`context perf ${label}: ${JSON.stringify(report)}`);
    return report;
}

function assertSelectorReport(report, changedComponent, label) {
    const selectorComponents = ["DensityConsumer", "AccentConsumer", "CounterConsumer"];
    for (const name of selectorComponents) {
        const renders = report.renderDeltas[name];
        if (name === changedComponent) {
            if (renders <= 0) {
                throw new Error(`APP FAILURE: ${label} did not render ${changedComponent}: ${JSON.stringify(report)}`);
            }
            continue;
        }
        if (renders !== 0) {
            throw new Error(`APP FAILURE: ${label} rendered unrelated ${name}: ${JSON.stringify(report.renderDeltas)}`);
        }
        if (report.memoDeltas[name] <= 0) {
            throw new Error(`APP FAILURE: ${label} did not memo-skip clean ${name}: ${JSON.stringify(report.memoDeltas)}`);
        }
    }
    if (report.renderDeltas.StaticPanel !== 0 || report.memoDeltas.StaticPanel <= 0) {
        throw new Error(`APP FAILURE: ${label} touched static panel: ${JSON.stringify(report)}`);
    }
    if (report.renderDeltas.BroadConsumer <= 0) {
        throw new Error(`APP FAILURE: ${label} did not rerender broad UseContext consumer: ${JSON.stringify(report.renderDeltas)}`);
    }
    if (report.renderDeltas.InnerAccentConsumer !== 0) {
        throw new Error(`APP FAILURE: ${label} rendered inner accent selector despite unchanged nested value: ${JSON.stringify(report.renderDeltas)}`);
    }
    assertNoStructuralOrListenerChurn(report, label);
}

function assertResetReport(report) {
    if (report.renderDeltas.DensityConsumer <= 0 || report.renderDeltas.AccentConsumer <= 0 || report.renderDeltas.CounterConsumer <= 0) {
        throw new Error(`APP FAILURE: reset should render changed selector consumers: ${JSON.stringify(report.renderDeltas)}`);
    }
    if (report.renderDeltas.StaticPanel !== 0 || report.memoDeltas.StaticPanel <= 0) {
        throw new Error(`APP FAILURE: reset touched static panel: ${JSON.stringify(report)}`);
    }
    if (report.renderDeltas.BroadConsumer <= 0) {
        throw new Error(`APP FAILURE: reset did not rerender broad UseContext consumer: ${JSON.stringify(report.renderDeltas)}`);
    }
    assertNoStructuralOrListenerChurn(report, "reset");
}

function assertNoStructuralOrListenerChurn(report, label) {
    const structural = ["createElement", "createTextNode", "appendChild", "removeChild", "replaceChild", "insertBefore"];
    for (const key of structural) {
        if (report.operations[key] !== 0) {
            throw new Error(`APP FAILURE: structural DOM op during ${label}: ${JSON.stringify(report.operations)}`);
        }
    }
    if (report.operations.addEventListener !== 0 || report.operations.removeEventListener !== 0) {
        throw new Error(`APP FAILURE: listener churn during ${label}: ${JSON.stringify(report.operations)}`);
    }
}

async function waitForPage(port) {
    let lastError;
    for (let attempt = 0; attempt < 50; attempt++) {
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

async function waitForAppPage(client, expected, label) {
    let lastState = null;
    for (let attempt = 0; attempt < 100; attempt++) {
        lastState = await pageState(client);
        if (lastState.href.startsWith("chrome-error://")) {
            throw await harnessFailure(client, `${label}: Chrome loaded an error document`, lastState);
        }
        if (isExpectedAppState(lastState, expected) && lastState.root && lastState.appReady) {
            return lastState;
        }
        await wait(100);
    }
    throw await harnessFailure(client, `${label}: app page did not become ready`, lastState);
}

async function pageState(client) {
    return await client.evaluate(`(() => ({
        href: window.location.href,
        origin: window.location.origin,
        protocol: window.location.protocol,
        readyState: document.readyState,
        root: Boolean(document.querySelector("#root")),
        appReady: Boolean(document.querySelector("#density-select") && window.goframeComponentRenderCounts?.App),
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
    };
}

function installContextAuditExpression(names) {
    return `(() => {
        if (window.__contextAudit) return { ready: true };
        const operations = ${JSON.stringify(emptyOperations())};
        const componentNames = ${JSON.stringify(names)};
        const audit = {
            operations,
            baseline: null,
            startedAt: 0,
            start(label) {
                for (const key of Object.keys(this.operations)) this.operations[key] = 0;
                this.baseline = snapshotCounts(componentNames);
                this.startedAt = performance.now();
            },
            finish(label) {
                const next = snapshotCounts(componentNames);
                const renderDeltas = {};
                const patchDeltas = {};
                const memoDeltas = {};
                for (const name of componentNames) {
                    renderDeltas[name] = next.renders[name] - this.baseline.renders[name];
                    patchDeltas[name] = next.patches[name] - this.baseline.patches[name];
                    memoDeltas[name] = next.memoSkips[name] - this.baseline.memoSkips[name];
                }
                return {
                    label,
                    durationMs: Math.round((performance.now() - this.startedAt) * 100) / 100,
                    renderDeltas,
                    patchDeltas,
                    memoDeltas,
                    operations: { ...this.operations },
                };
            },
        };
        const wrap = (owner, name, counter) => {
            const original = owner[name];
            owner[name] = function(...args) {
                operations[counter]++;
                return original.apply(this, args);
            };
        };
        wrap(Document.prototype, "createElement", "createElement");
        wrap(Document.prototype, "createTextNode", "createTextNode");
        wrap(Node.prototype, "appendChild", "appendChild");
        wrap(Node.prototype, "removeChild", "removeChild");
        wrap(Node.prototype, "replaceChild", "replaceChild");
        wrap(Node.prototype, "insertBefore", "insertBefore");
        wrap(Element.prototype, "setAttribute", "setAttribute");
        wrap(Element.prototype, "removeAttribute", "removeAttribute");
        wrap(EventTarget.prototype, "addEventListener", "addEventListener");
        wrap(EventTarget.prototype, "removeEventListener", "removeEventListener");
        const nodeValue = Object.getOwnPropertyDescriptor(Node.prototype, "nodeValue");
        Object.defineProperty(Node.prototype, "nodeValue", {
            configurable: nodeValue.configurable,
            enumerable: nodeValue.enumerable,
            get: nodeValue.get,
            set(value) {
                operations.setTextNodeValue++;
                return nodeValue.set.call(this, value);
            },
        });
        const selectValue = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, "value");
        Object.defineProperty(HTMLSelectElement.prototype, "value", {
            configurable: selectValue.configurable,
            enumerable: selectValue.enumerable,
            get: selectValue.get,
            set(value) {
                operations.setProperty++;
                return selectValue.set.call(this, value);
            },
        });
        window.__contextAudit = audit;
        return { ready: true };

        function snapshotCounts(names) {
            const renderCounts = window.goframeComponentRenderCounts || {};
            const patchCounts = window.goframeComponentPatchCounts || {};
            const memoSkips = window.goframeComponentMemoSkips || {};
            const renders = {};
            const patches = {};
            const skips = {};
            for (const name of names) {
                renders[name] = renderCounts[name] || 0;
                patches[name] = patchCounts[name] || 0;
                skips[name] = memoSkips[name] || 0;
            }
            return { renders, patches, memoSkips: skips };
        }
    })()`;
}

function emptyOperations() {
    return {
        createElement: 0,
        createTextNode: 0,
        appendChild: 0,
        removeChild: 0,
        replaceChild: 0,
        insertBefore: 0,
        setTextNodeValue: 0,
        setAttribute: 0,
        removeAttribute: 0,
        setProperty: 0,
        addEventListener: 0,
        removeEventListener: 0,
    };
}

function assertDeepEqual(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        throw new Error(`APP FAILURE: ${label}: got ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`);
    }
    console.log(`${label}: ok`);
}

function wait(duration) {
    return new Promise((resolve) => setTimeout(resolve, duration));
}

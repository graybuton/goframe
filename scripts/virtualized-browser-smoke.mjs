import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const appURL = process.argv[2] ?? process.env.GOFRAME_VIRTUALIZED_SMOKE_URL ?? "http://127.0.0.1:18080/";
const debugPort = Number(process.env.GOFRAME_VIRTUALIZED_CHROME_DEBUG_PORT ?? "19227");
const chrome = process.env.CHROME ?? "google-chrome";
const profile = await mkdtemp(join(tmpdir(), "goframe-virtualized-smoke-"));
const expectedApp = new URL(appURL);
const logicalItems = 10000;
const maxMountedListItems = 40;
const maxMountedTableRows = 40;
const componentNames = [
    "App",
    "VirtualizedApp",
    "VirtualListPanel",
    "VirtualList",
    "VirtualListItem",
    "VirtualTablePanel",
    "VirtualTable",
    "VirtualTableHeader",
    "VirtualTableRow",
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
    assertDeepEqual(await client.evaluate(installVirtualizedAuditExpression(componentNames)), { ready: true }, "virtualized audit installed");

    const initial = await virtualizedState(client);
    assertVirtualizedState(initial, "initial");
    assertDeepEqual(
        {
            logical: initial.logical,
            appRenders: initial.appRenders,
            header: initial.header,
        },
        { logical: "10000 logical items", appRenders: 1, header: true },
        "virtualized initial render",
    );

    await startScenario(client, "scroll-list");
    await scrollTo(client, "[data-testid='virtual-list']", 44 * 500);
    const listReport = await finishScenario(client, "scroll-list");
    const listScrolled = await virtualizedState(client);
    assertVirtualizedState(listScrolled, "list scroll");
    if (JSON.stringify(initial.listIDs) === JSON.stringify(listScrolled.listIDs)) {
        throw new Error(`APP FAILURE: VirtualList scroll did not change visible IDs: ${JSON.stringify(listScrolled.listIDs)}`);
    }
    assertMountedCountsInReport(listReport, "VirtualList scroll");
    console.log("virtualized list scroll: ok");

    await startScenario(client, "scroll-table");
    await scrollTo(client, "[data-testid='virtual-table']", 40 * 800);
    const tableReport = await finishScenario(client, "scroll-table");
    const tableScrolled = await virtualizedState(client);
    assertVirtualizedState(tableScrolled, "table scroll");
    if (JSON.stringify(initial.tableIDs) === JSON.stringify(tableScrolled.tableIDs)) {
        throw new Error(`APP FAILURE: VirtualTable scroll did not change visible IDs: ${JSON.stringify(tableScrolled.tableIDs)}`);
    }
    assertMountedCountsInReport(tableReport, "VirtualTable scroll");
    console.log("virtualized table scroll: ok");

    const targetID = tableScrolled.tableIDs[2] ?? tableScrolled.tableIDs[0];
    if (!targetID) {
        throw new Error("APP FAILURE: no virtual table row target after scroll");
    }
    await client.evaluate(`document.querySelector("[data-testid='virtual-row-${targetID}'] .row-link").click()`);
    await wait(120);
    const selected = await virtualizedState(client);
    if (!selected.selection.includes(`#${targetID}`)) {
        throw new Error(`APP FAILURE: selecting row ${targetID} after scroll failed: ${JSON.stringify(selected)}`);
    }
    console.log("virtualized table selection after scroll: ok");

    const beforeToggle = selected.selection.includes("enabled") ? "enabled" : "disabled";
    await startScenario(client, "toggle-after-scroll");
    await client.evaluate(`document.querySelector("[data-testid='virtual-row-${targetID}'] .tiny").click()`);
    await wait(120);
    const toggleReport = await finishScenario(client, "toggle-after-scroll");
    const afterToggle = await virtualizedState(client);
    const afterLabel = afterToggle.selection.includes("enabled") ? "enabled" : "disabled";
    if (beforeToggle === afterLabel) {
        throw new Error(`APP FAILURE: toggling row ${targetID} after scroll did not update selection summary: ${JSON.stringify(afterToggle)}`);
    }
    assertVirtualizedState(afterToggle, "toggle after scroll");
    assertNoListenerNetGrowth(toggleReport, "toggle after scroll");
    console.log("virtualized toggle after scroll: ok");

    await startScenario(client, "scroll-stability");
    for (let index = 0; index < 6; index++) {
        await scrollTo(client, "[data-testid='virtual-table']", 40 * (index % 2 === 0 ? 1200 : 300));
    }
    const stabilityReport = await finishScenario(client, "scroll-stability");
    const stable = await virtualizedState(client);
    assertVirtualizedState(stable, "scroll stability");
    assertNoListenerNetGrowth(stabilityReport, "repeated virtual table scroll");
    console.log("virtualized repeated scroll listener stability: ok");

    client.close();
    console.log("Virtualized browser smoke: ok");
} finally {
    const exited = new Promise((resolve) => browser.once("exit", resolve));
    browser.kill("SIGTERM");
    await Promise.race([exited, wait(2000)]);
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function virtualizedState(client) {
    return await client.evaluate(`(() => ({
        header: Boolean(document.querySelector("[data-testid='virtualized-header']")),
        logical: document.querySelector("[data-testid='logical-count']")?.textContent.trim() || "",
        selection: document.querySelector("[data-testid='selection-summary']")?.textContent.replace(/\\s+/g, " ").trim() || "",
        listItems: document.querySelectorAll("[data-testid^='virtual-list-item-']").length,
        tableRows: document.querySelectorAll("[data-testid^='virtual-row-']").length,
        listIDs: [...document.querySelectorAll("[data-testid^='virtual-list-item-']")].map((node) => node.getAttribute("data-testid").replace("virtual-list-item-", "")),
        tableIDs: [...document.querySelectorAll("[data-testid^='virtual-row-']")].map((node) => node.getAttribute("data-testid").replace("virtual-row-", "")),
        liveDOM: document.querySelectorAll("*").length,
        appRenders: window.goframeComponentRenderCounts.App,
    }))()`);
}

function assertVirtualizedState(state, label) {
    if (state.logical !== `${logicalItems} logical items`) {
        throw new Error(`APP FAILURE: ${label}: logical count mismatch: ${JSON.stringify(state)}`);
    }
    if (state.listItems <= 0 || state.listItems > maxMountedListItems) {
        throw new Error(`APP FAILURE: ${label}: mounted list items ${state.listItems}, limit ${maxMountedListItems}`);
    }
    if (state.tableRows <= 0 || state.tableRows > maxMountedTableRows) {
        throw new Error(`APP FAILURE: ${label}: mounted table rows ${state.tableRows}, limit ${maxMountedTableRows}`);
    }
}

function assertNoListenerNetGrowth(report, label) {
    if (report.netListeners !== 0) {
        throw new Error(`APP FAILURE: ${label}: listener net changed: ${JSON.stringify(report)}`);
    }
}

function assertMountedCountsInReport(report, label) {
    if (report.listItems <= 0 || report.listItems > maxMountedListItems ||
        report.tableRows <= 0 || report.tableRows > maxMountedTableRows) {
        throw new Error(`APP FAILURE: ${label}: mounted count exceeded bounds: ${JSON.stringify(report)}`);
    }
}

async function scrollTo(client, selector, scrollTop) {
    await client.evaluate(`(() => {
        const viewport = document.querySelector(${JSON.stringify(selector)});
        viewport.scrollTop = ${scrollTop};
        viewport.dispatchEvent(new Event("scroll", { bubbles: true }));
    })()`);
    await wait(160);
}

async function startScenario(client, label) {
    await client.evaluate(`window.__virtualizedAudit.start(${JSON.stringify(label)})`);
}

async function finishScenario(client, label) {
    const report = await client.evaluate(`window.__virtualizedAudit.finish(${JSON.stringify(label)})`);
    console.log(`virtualized perf ${label}: ${JSON.stringify(report)}`);
    return report;
}

function installVirtualizedAuditExpression(names) {
    return `(() => {
        if (window.__virtualizedAudit) return { ready: true };
        const componentNames = ${JSON.stringify(names)};
        const operations = ${JSON.stringify(emptyOperations())};
        let netListeners = 0;
        const audit = {
            baseline: null,
            operationBaseline: null,
            netBaseline: 0,
            start(label) {
                this.baseline = snapshotCounts(componentNames);
                this.operationBaseline = { ...operations };
                this.netBaseline = netListeners;
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
                const opDeltas = {};
                for (const key of Object.keys(operations)) {
                    opDeltas[key] = operations[key] - this.operationBaseline[key];
                }
                return {
                    label,
                    renderDeltas,
                    patchDeltas,
                    memoDeltas,
                    operations: opDeltas,
                    netListeners: netListeners - this.netBaseline,
                    listItems: document.querySelectorAll("[data-testid^='virtual-list-item-']").length,
                    tableRows: document.querySelectorAll("[data-testid^='virtual-row-']").length,
                    liveDOM: document.querySelectorAll("*").length,
                };
            },
        };
        const wrap = (owner, name, counter, after) => {
            const original = owner[name];
            owner[name] = function(...args) {
                operations[counter]++;
                if (after) after();
                return original.apply(this, args);
            };
        };
        wrap(Document.prototype, "createElement", "createElement");
        wrap(Document.prototype, "createTextNode", "createTextNode");
        wrap(Node.prototype, "appendChild", "appendChild");
        wrap(Node.prototype, "removeChild", "removeChild");
        wrap(Node.prototype, "insertBefore", "insertBefore");
        wrap(EventTarget.prototype, "addEventListener", "addEventListener", () => { netListeners++; });
        wrap(EventTarget.prototype, "removeEventListener", "removeEventListener", () => { netListeners--; });
        window.__virtualizedAudit = audit;
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
        insertBefore: 0,
        addEventListener: 0,
        removeEventListener: 0,
    };
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
            if (page) return page;
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
        appReady: Boolean(document.querySelector("[data-testid='virtualized-header']") && window.goframeComponentRenderCounts?.App),
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
    if (browserExit) diagnostics.browserExit = browserExit;
    if (browserError) diagnostics.browserStderr = browserError.slice(-4000);
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
        if (!message.id || !pending.has(message.id)) return;
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

function assertDeepEqual(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        throw new Error(`APP FAILURE: ${label}: got ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`);
    }
    console.log(`${label}: ok`);
}

function wait(duration) {
    return new Promise((resolve) => setTimeout(resolve, duration));
}

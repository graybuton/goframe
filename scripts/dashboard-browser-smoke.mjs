import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const appURL = process.argv[2] ?? process.env.GOFRAME_DASHBOARD_SMOKE_URL ?? "http://127.0.0.1:18080/";
const debugPort = Number(process.env.GOFRAME_DASHBOARD_CHROME_DEBUG_PORT ?? "19224");
const chrome = process.env.CHROME ?? "google-chrome";
const profile = await mkdtemp(join(tmpdir(), "goframe-dashboard-smoke-"));
const expectedApp = new URL(appURL);
const timings = [];
const performanceReports = [];
const dashboardLogicalRows = 300;
const maxMountedRows = 70;
const componentNames = [
    "App",
    "Header",
    "DashboardApp",
    "DashboardShell",
    "MetricsGrid",
    "MetricCard",
    "Card",
    "FilterBar",
    "SearchBox",
    "IssueWorkspace",
    "IssueTable",
    "VirtualTable",
    "IssueTableHeader",
    "IssueRow",
    "DetailPanel",
    "EmptyDetail",
    "Button",
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

    const start = Date.now();
    await navigateToApp(client, withSmokeParam(appURL, "initial"));
    await waitForAppPage(client, expectedApp, "initial navigation");
    timings.push({ step: "initial-ready", ms: Date.now() - start });
    await clearClientStorage(client, expectedApp);
    await navigateToApp(client, withSmokeParam(appURL, "clean"));
    await waitForAppPage(client, expectedApp, "post-clean navigation");

    const initialState = await client.evaluate(`(() => {
            window.__dashboardHeader = document.querySelector("[data-testid='dashboard-header']");
            window.__dashboardSearch = document.querySelector("#dashboard-search");
            window.__dashboardMetrics = document.querySelector("[data-testid='metrics-grid']");
            window.__dashboardTable = document.querySelector("[data-testid='issue-table']");
            window.__dashboardDetail = document.querySelector("[data-testid='detail-panel']");
            window.__dashboardRow1 = document.querySelector("#issue-row-1");
            return {
                header: Boolean(window.__dashboardHeader),
                search: Boolean(window.__dashboardSearch),
                metrics: Boolean(window.__dashboardMetrics),
                table: Boolean(window.__dashboardTable),
                detail: Boolean(window.__dashboardDetail),
                rows: document.querySelectorAll(".issue-row").length,
                summary: document.querySelector("[data-testid='visible-summary']")?.textContent.trim(),
                appRenders: window.goframeComponentRenderCounts.App,
                headerRenders: window.goframeComponentRenderCounts.Header,
            };
        })()`);
    assertMountedRowsBounded(initialState.rows, dashboardLogicalRows, "dashboard initial mounted rows");
    assertDeepEqual(
        {
            header: initialState.header,
            search: initialState.search,
            metrics: initialState.metrics,
            table: initialState.table,
            detail: initialState.detail,
            summary: initialState.summary,
            appRenders: initialState.appRenders,
            headerRenders: initialState.headerRenders,
        },
        {
            header: true,
            search: true,
            metrics: true,
            table: true,
            detail: true,
            summary: "Showing 300 of 300 issues",
            appRenders: 1,
            headerRenders: 1,
        },
        "dashboard initial render",
    );

    assertDeepEqual(
        await client.evaluate(installDashboardAuditExpression(componentNames)),
        { ready: true },
        "dashboard performance audit installed",
    );

    await startScenario(client, "focus-search");
    await client.evaluate(`document.querySelector("#dashboard-search").focus()`);
    await wait(120);
    const focusReport = await finishScenario(client, "focus-search");
    assertFocusOnlyReport(focusReport);

    await startScenario(client, "search-billing");
    const searchStart = Date.now();
    await setInputValue(client, "#dashboard-search", "billing");
    timings.push({ step: "search-update", ms: Date.now() - searchStart });
    const searchReport = await finishScenario(client, "search-billing");
    const searchState = await client.evaluate(`(() => {
        const rows = document.querySelectorAll(".issue-row").length;
        const summary = document.querySelector("[data-testid='visible-summary']")?.textContent.trim();
        return {
            rows,
            summary,
            headerSame: window.__dashboardHeader === document.querySelector("[data-testid='dashboard-header']"),
            searchSame: window.__dashboardSearch === document.querySelector("#dashboard-search"),
            metricsVisible: Number(document.querySelectorAll(".metric-card strong")[1]?.textContent),
            headerRenders: window.goframeComponentRenderCounts.Header,
        };
    })()`);
    const searchVisible = parseSummary(searchState.summary).visible;
    assertMountedRowsBounded(searchState.rows, searchVisible, "dashboard search mounted rows");
    if (!(searchVisible > 0 && searchVisible < 300)) {
        throw new Error(`APP FAILURE: search should narrow rows: ${JSON.stringify(searchState)}`);
    }
    assertDeepEqual(
        {
            headerSame: searchState.headerSame,
            searchSame: searchState.searchSame,
            metricsVisible: searchState.metricsVisible,
            headerRenders: searchState.headerRenders,
        },
        {
            headerSame: true,
            searchSame: true,
            metricsVisible: searchVisible,
            headerRenders: 1,
        },
        "dashboard search updates visible rows without replacing header/search",
    );

    await client.evaluate(`document.querySelector(".issue-row .row-link").click()`);
    await wait(120);
    await startScenario(client, "row-select");
    const selectStart = Date.now();
    await client.evaluate(`(() => {
        const rows = document.querySelectorAll(".issue-row");
        const links = document.querySelectorAll(".issue-row .row-link");
        window.__dashboardSelectionTarget = rows[1].id.replace("issue-row-", "");
        links[1].click();
    })()`);
    await wait(120);
    timings.push({ step: "selection-update", ms: Date.now() - selectStart });
    const selectReport = await finishScenario(client, "row-select");
    assertRowSelectionReport(selectReport);
    const selected = await client.evaluate(`(() => {
        const target = window.__dashboardSelectionTarget;
        return {
            selected: Boolean(document.querySelector(".issue-row-selected")),
            detail: document.querySelector("[data-testid='detail-panel']")?.textContent.includes(target),
            headerSame: window.__dashboardHeader === document.querySelector("[data-testid='dashboard-header']"),
            searchSame: window.__dashboardSearch === document.querySelector("#dashboard-search"),
        };
    })()`);
    assertDeepEqual(
        selected,
        { selected: true, detail: true, headerSame: true, searchSame: true },
        "dashboard row selection updates detail only",
    );

    await startScenario(client, "clear-search");
    await setInputValue(client, "#dashboard-search", "");
    await wait(120);
    const clearSearchReport = await finishScenario(client, "clear-search");
    windowlessAssert(
        await client.evaluate(`(() => {
            window.__dashboardRow1 = document.querySelector("#issue-row-1");
            const before = [...document.querySelectorAll(".issue-row")].slice(0, 8).map((node) => node.id);
            window.__dashboardBeforeSort = before;
            window.__dashboardBeforeSortNodes = {};
            for (const node of document.querySelectorAll(".issue-row")) {
                window.__dashboardBeforeSortNodes[node.id] = node;
            }
            return { rows: document.querySelectorAll(".issue-row").length, row1: Boolean(window.__dashboardRow1) };
        })()`),
        (value) => value.rows > 0 && value.rows <= maxMountedRows && value.row1,
        "APP FAILURE: dashboard reset before sort did not restore bounded mounted rows",
    );

    await startScenario(client, "table-scroll");
    await client.evaluate(`(() => {
        const viewport = document.querySelector("[data-testid='issue-table']");
        window.__dashboardBeforeScroll = [...document.querySelectorAll(".issue-row")].map((node) => node.id);
        viewport.scrollTop = 48 * 80;
        viewport.dispatchEvent(new Event("scroll", { bubbles: true }));
    })()`);
    await wait(160);
    const scrollReport = await finishScenario(client, "table-scroll");
    const scrolled = await client.evaluate(`(() => {
        const after = [...document.querySelectorAll(".issue-row")].map((node) => node.id);
        const target = after[2] || after[0] || "";
        window.__dashboardScrolledTarget = target.replace("issue-row-", "");
        return {
            rows: after.length,
            changed: JSON.stringify(after) !== JSON.stringify(window.__dashboardBeforeScroll),
            target: window.__dashboardScrolledTarget,
        };
    })()`);
    assertMountedRowsBounded(scrolled.rows, dashboardLogicalRows, "dashboard scrolled mounted rows");
    if (!scrolled.changed || !scrolled.target) {
        throw new Error(`APP FAILURE: dashboard scroll should change visible row window: ${JSON.stringify(scrolled)}`);
    }
    await client.evaluate(`(() => {
        const target = document.querySelector("#issue-row-" + window.__dashboardScrolledTarget + " .row-link");
        target.click();
    })()`);
    await wait(120);
    const scrolledSelection = await client.evaluate(`(() => ({
        detail: document.querySelector("[data-testid='detail-panel']")?.textContent.includes(window.__dashboardScrolledTarget),
        rows: document.querySelectorAll(".issue-row").length,
    }))()`);
    assertMountedRowsBounded(scrolledSelection.rows, dashboardLogicalRows, "dashboard scrolled selection mounted rows");
    if (!scrolledSelection.detail) {
        throw new Error(`APP FAILURE: selecting after scroll did not update detail: ${JSON.stringify(scrolledSelection)}`);
    }
    console.log("dashboard virtual scroll selection updates detail: ok");

    await client.evaluate(`(() => {
        const viewport = document.querySelector("[data-testid='issue-table']");
        viewport.scrollTop = 0;
        viewport.dispatchEvent(new Event("scroll", { bubbles: true }));
    })()`);
    await wait(160);
    await client.evaluate(`(() => {
        const before = [...document.querySelectorAll(".issue-row")].slice(0, 8).map((node) => node.id);
        window.__dashboardBeforeSort = before;
        window.__dashboardBeforeSortNodes = {};
        for (const node of document.querySelectorAll(".issue-row")) {
            window.__dashboardBeforeSortNodes[node.id] = node;
        }
    })()`);

    await startScenario(client, "sort-priority");
    const sortStart = Date.now();
    await setSelectValue(client, "#sort-mode", "priority");
    timings.push({ step: "sort-update", ms: Date.now() - sortStart });
    const sortReport = await finishScenario(client, "sort-priority");
    const sorted = await client.evaluate(`(() => {
        const after = [...document.querySelectorAll(".issue-row")].slice(0, 8).map((node) => node.id);
        const overlap = after.find((id) => window.__dashboardBeforeSort.includes(id));
        return {
            rows: document.querySelectorAll(".issue-row").length,
            changed: JSON.stringify(after) !== JSON.stringify(window.__dashboardBeforeSort),
            headerSame: window.__dashboardHeader === document.querySelector("[data-testid='dashboard-header']"),
            overlapPreserved: overlap ? window.__dashboardBeforeSortNodes[overlap] === document.querySelector("#" + overlap) : true,
        };
    })()`);
    assertMountedRowsBounded(sorted.rows, dashboardLogicalRows, "dashboard sorted mounted rows");
    assertDeepEqual(
        { changed: sorted.changed, headerSame: sorted.headerSame, overlapPreserved: sorted.overlapPreserved },
        { changed: true, headerSame: true, overlapPreserved: true },
        "dashboard sort updates virtual row window without replacing visible survivors",
    );

    await startScenario(client, "status-blocked");
    await setSelectValue(client, "#status-filter", "blocked");
    const statusReport = await finishScenario(client, "status-blocked");
    const filtered = await client.evaluate(`(() => {
        const rows = document.querySelectorAll(".issue-row").length;
        return {
            rows,
            summary: document.querySelector("[data-testid='visible-summary']")?.textContent.trim(),
            headerSame: window.__dashboardHeader === document.querySelector("[data-testid='dashboard-header']"),
        };
    })()`);
    const filteredVisible = parseSummary(filtered.summary).visible;
    assertMountedRowsBounded(filtered.rows, filteredVisible, "dashboard status mounted rows");
    if (!(filteredVisible > 0 && filteredVisible < 300)) {
        throw new Error(`APP FAILURE: status filter should narrow rows: ${JSON.stringify(filtered)}`);
    }
    assertDeepEqual(
        { summary: filtered.summary, headerSame: filtered.headerSame },
        { summary: `Showing ${filteredVisible} of 300 issues`, headerSame: true },
        "dashboard status filter updates table predictably",
    );

    await startScenario(client, "reset-before-simulate");
    await clickButtonByText(client, "Reset");
    await wait(120);
    const resetBeforeSimulateReport = await finishScenario(client, "reset-before-simulate");

    await startScenario(client, "row-toggle");
    const toggleBefore = await firstRowStatus(client);
    await client.evaluate(`document.querySelector(".issue-row .button").click()`);
    await wait(120);
    const toggleReport = await finishScenario(client, "row-toggle");
    assertRowDataChangeReport(toggleReport, "row toggle");
    const toggleAfter = await firstRowStatus(client);
    if (toggleBefore === toggleAfter) {
        throw new Error(`APP FAILURE: row toggle did not change first row status: ${toggleBefore}`);
    }
    console.log("dashboard row toggle changes status: ok");

    const beforeEvents = await metricValue(client, "Events");
    await startScenario(client, "simulate-update");
    await clickButtonByText(client, "Simulate update");
    await wait(120);
    const simulateReport = await finishScenario(client, "simulate-update");
    assertRowDataChangeReport(simulateReport, "simulate update");
    const afterEvents = await metricValue(client, "Events");
    if (!(afterEvents > beforeEvents)) {
        throw new Error(`APP FAILURE: simulate update did not increase events: before ${beforeEvents}, after ${afterEvents}`);
    }
    console.log("dashboard simulate update changes metrics: ok");

    await startScenario(client, "post-simulate-row-toggle");
    const postSimulateToggleBefore = await firstRowStatus(client);
    await client.evaluate(`document.querySelector(".issue-row .button").click()`);
    await wait(120);
    const postSimulateToggleReport = await finishScenario(client, "post-simulate-row-toggle");
    assertRowDataChangeReport(postSimulateToggleReport, "post-simulate row toggle");
    const postSimulateToggleAfter = await firstRowStatus(client);
    const eventsAfterPostSimulateToggle = await metricValue(client, "Events");
    if (postSimulateToggleBefore === postSimulateToggleAfter) {
        throw new Error(`APP FAILURE: post-simulate row toggle did not change first row status: ${postSimulateToggleBefore}`);
    }
    if (eventsAfterPostSimulateToggle !== afterEvents) {
        throw new Error(`APP FAILURE: post-simulate row toggle used stale issue data: events after simulate ${afterEvents}, after toggle ${eventsAfterPostSimulateToggle}`);
    }
    console.log("dashboard post-simulate row toggle preserves simulated data: ok");

    await startScenario(client, "reset-final");
    await clickButtonByText(client, "Reset");
    await wait(120);
    const resetReport = await finishScenario(client, "reset-final");
    windowlessAssert(
        await client.evaluate(`(() => ({
            rows: document.querySelectorAll(".issue-row").length,
            summary: document.querySelector("[data-testid='visible-summary']")?.textContent.trim(),
            title: document.title,
            headerSame: window.__dashboardHeader === document.querySelector("[data-testid='dashboard-header']"),
            searchSame: window.__dashboardSearch === document.querySelector("#dashboard-search"),
        }))()`),
        (value) => {
            assertMountedRowsBounded(value.rows, dashboardLogicalRows, "dashboard reset mounted rows");
            return value.summary === "Showing 300 of 300 issues" &&
                value.title === "GoFrame Dashboard · 300 visible" &&
                value.headerSame &&
                value.searchSame;
        },
        "dashboard reset restores deterministic view",
    );

    client.close();
    console.log(`Dashboard timing report: ${JSON.stringify(timings)}`);
    console.log(`Dashboard performance report: ${JSON.stringify([
        focusReport,
        searchReport,
        selectReport,
        clearSearchReport,
        scrollReport,
        sortReport,
        statusReport,
        resetBeforeSimulateReport,
        toggleReport,
        simulateReport,
        postSimulateToggleReport,
        resetReport,
    ])}`);
    console.log("Dashboard browser smoke: ok");
} finally {
    const exited = new Promise((resolve) => browser.once("exit", resolve));
    browser.kill("SIGTERM");
    await Promise.race([exited, wait(2000)]);
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function clearClientStorage(client) {
    await ensureAppPage(client, expectedApp, "before storage cleanup");
    const result = await client.evaluate(`(async () => {
        const result = { href: window.location.href, origin: window.location.origin, protocol: window.location.protocol, ok: false, error: "" };
        try {
            window.localStorage.clear();
            window.sessionStorage.clear();
            if (window.caches) {
                const keys = await window.caches.keys();
                await Promise.all(keys.map((key) => window.caches.delete(key)));
            }
            if (window.navigator?.serviceWorker?.getRegistrations) {
                const registrations = await window.navigator.serviceWorker.getRegistrations();
                await Promise.all(registrations.map((registration) => registration.unregister()));
            }
            result.ok = true;
            return result;
        } catch (error) {
            result.error = error.name + ": " + error.message;
            return result;
        }
    })()`);
    if (!result.ok) {
        throw await harnessFailure(client, "storage cleanup failed", result);
    }
}

async function setInputValue(client, selector, value) {
    await client.evaluate(`(() => {
        const input = document.querySelector(${JSON.stringify(selector)});
        input.focus();
        input.value = ${JSON.stringify(value)};
        input.dispatchEvent(new Event("input", { bubbles: true }));
        return true;
    })()`);
    await wait(140);
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

async function clickButtonByText(client, text) {
    await client.evaluate(`(() => {
        const button = [...document.querySelectorAll("button")].find((node) => node.textContent.trim() === ${JSON.stringify(text)});
        if (!button) return false;
        button.click();
        return true;
    })()`);
}

async function metricValue(client, label) {
    return await client.evaluate(`(() => {
        for (const card of document.querySelectorAll(".metric-card")) {
            if (card.querySelector("span")?.textContent === ${JSON.stringify(label)}) {
                return Number(card.querySelector("strong")?.textContent);
            }
        }
        return -1;
    })()`);
}

async function firstRowStatus(client) {
    return await client.evaluate(`document.querySelector(".issue-row .status")?.textContent || ""`);
}

async function startScenario(client, label) {
    await client.evaluate(`(() => {
        const audit = window.__dashboardAudit;
        if (!audit) return false;
        audit.start(${JSON.stringify(label)});
        return true;
    })()`);
}

async function finishScenario(client, label) {
    const report = await client.evaluate(`(() => window.__dashboardAudit.finish(${JSON.stringify(label)}))()`);
    performanceReports.push(report);
    console.log(`dashboard perf ${label}: ${JSON.stringify(report)}`);
    return report;
}

function assertFocusOnlyReport(report) {
    const nonZeroRenders = Object.entries(report.renderDeltas).filter(([, value]) => value !== 0);
    const nonZeroPatches = Object.entries(report.patchDeltas).filter(([, value]) => value !== 0);
    const nonZeroOperations = Object.entries(report.operations).filter(([, value]) => value !== 0);
    const nonZeroMutations = Object.entries(report.mutations).filter(([, value]) => value !== 0);
    if (nonZeroRenders.length || nonZeroPatches.length || nonZeroOperations.length || nonZeroMutations.length) {
        throw new Error(`APP FAILURE: focus-only should not trigger runtime work: ${JSON.stringify(report)}`);
    }
    console.log("dashboard focus-only does not trigger runtime work: ok");
}

function parseSummary(summary) {
    const match = /^Showing (\d+) of (\d+) issues$/.exec(summary || "");
    if (!match) {
        throw new Error(`APP FAILURE: unexpected dashboard summary: ${summary}`);
    }
    return { visible: Number(match[1]), total: Number(match[2]) };
}

function assertMountedRowsBounded(rows, logicalRows, label) {
    const expectedLimit = Math.min(maxMountedRows, Math.max(0, logicalRows));
    if (rows < 0 || rows > expectedLimit || (logicalRows > 0 && rows === 0)) {
        throw new Error(`APP FAILURE: ${label}: mounted rows ${rows}, logical rows ${logicalRows}, limit ${expectedLimit}`);
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
        if (isExpectedAppState(lastState, expected) && lastState.root && lastState.appReady && lastState.storage === "available") {
            return lastState;
        }
        await wait(100);
    }
    throw await harnessFailure(client, `${label}: app page did not become ready`, lastState);
}

async function ensureAppPage(client, expected, label) {
    const state = await pageState(client);
    if (!isExpectedAppState(state, expected) || !state.root || !state.appReady || state.storage !== "available") {
        throw await harnessFailure(client, `${label}: wrong page or unavailable origin storage`, state);
    }
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
            root: Boolean(document.querySelector("#root")),
            appReady: Boolean(document.querySelector("#dashboard-search") && document.querySelector("[data-testid='metrics-grid']") && window.goframeComponentRenderCounts?.App),
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

function assertDeepEqual(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        throw new Error(`APP FAILURE: ${label}: got ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`);
    }
    console.log(`${label}: ok`);
}

function windowlessAssert(value, predicate, message) {
    if (!predicate(value)) {
        throw new Error(`${message}: ${JSON.stringify(value)}`);
    }
}

function wait(duration) {
    return new Promise((resolve) => setTimeout(resolve, duration));
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

function emptyMutations() {
    return {
        rootChildList: 0,
        header: 0,
        metrics: 0,
        filters: 0,
        table: 0,
        detail: 0,
    };
}

function installDashboardAuditExpression(names) {
    return `(() => {
        if (window.__dashboardAudit) return { ready: true };

        const operations = ${JSON.stringify(emptyOperations())};
        const mutations = ${JSON.stringify(emptyMutations())};
        const componentNames = ${JSON.stringify(names)};
        const audit = {
            operations,
            mutations,
            baseline: null,
            startedAt: 0,
            label: "",
            start(label) {
                this.label = label;
                this.startedAt = performance.now();
                for (const key of Object.keys(this.operations)) this.operations[key] = 0;
                for (const key of Object.keys(this.mutations)) this.mutations[key] = 0;
                this.baseline = snapshotCounts(componentNames);
            },
            finish(label) {
            const next = snapshotCounts(componentNames);
            const renderDeltas = {};
            const patchDeltas = {};
            for (const name of componentNames) {
                renderDeltas[name] = next.renders[name] - this.baseline.renders[name];
                patchDeltas[name] = next.patches[name] - this.baseline.patches[name];
            }
            const memoDeltas = {};
            for (const name of componentNames) {
                memoDeltas[name] = next.memoSkips[name] - this.baseline.memoSkips[name];
            }
            return {
                label,
                durationMs: Math.round((performance.now() - this.startedAt) * 100) / 100,
                renderDeltas,
                patchDeltas,
                memoDeltas,
                operations: { ...this.operations },
                mutations: { ...this.mutations },
                rows: document.querySelectorAll(".issue-row").length,
                summary: document.querySelector("[data-testid='visible-summary']")?.textContent.trim() || "",
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
        const inputValue = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value");
        Object.defineProperty(HTMLInputElement.prototype, "value", {
            configurable: inputValue.configurable,
            enumerable: inputValue.enumerable,
            get: inputValue.get,
            set(value) {
                operations.setProperty++;
                return inputValue.set.call(this, value);
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

        const observe = (selector, key, options) => {
            const node = document.querySelector(selector);
            if (!node) return;
            new MutationObserver((records) => {
                if (key === "rootChildList") {
                    mutations[key] += records.filter((record) => record.type === "childList").length;
                    return;
                }
                mutations[key] += records.length;
            }).observe(node, options);
        };
        observe("#root", "rootChildList", { childList: true });
        observe("[data-testid='dashboard-header']", "header", { childList: true, subtree: true, attributes: true, characterData: true });
        observe("[data-testid='metrics-grid']", "metrics", { childList: true, subtree: true, attributes: true, characterData: true });
        observe("[data-testid='filter-bar']", "filters", { childList: true, subtree: true, attributes: true, characterData: true });
        observe("[data-testid='issue-table']", "table", { childList: true, subtree: true, attributes: true, characterData: true });
        observe("[data-testid='detail-panel']", "detail", { childList: true, subtree: true, attributes: true, characterData: true });

        window.__dashboardAudit = audit;
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

function assertRowSelectionReport(report) {
    const allowed = 2;
    if (report.renderDeltas.IssueRow > allowed) {
        throw new Error(`APP FAILURE: IssueRow renders ${report.renderDeltas.IssueRow} for row selection (limit ${allowed})`);
    }
    if (report.memoDeltas.IssueRow <= 0) {
        throw new Error(`APP FAILURE: IssueRow memo skips not observed on row selection: ${JSON.stringify(report.memoDeltas)}`);
    }
    if (report.renderDeltas.Header !== 0 || report.renderDeltas.FilterBar !== 0 || report.renderDeltas.MetricsGrid !== 0) {
        throw new Error(`APP FAILURE: unrelated renders triggered by row selection: ${JSON.stringify(report.renderDeltas)}`);
    }
    if (report.patchDeltas.Header !== 0 || report.patchDeltas.FilterBar !== 0 || report.patchDeltas.MetricsGrid !== 0) {
        throw new Error(`APP FAILURE: unrelated patches triggered by row selection: ${JSON.stringify(report.patchDeltas)}`);
    }
    if (report.operations.addEventListener !== 0 || report.operations.removeEventListener !== 0) {
        throw new Error(`APP FAILURE: listener churn during row selection: ${JSON.stringify(report.operations)}`);
    }
    console.log(`dashboard row-selection memo summary: ${JSON.stringify({ renderDeltas: report.renderDeltas, memoDeltas: report.memoDeltas })}`);
}

function assertRowDataChangeReport(report, label) {
    const allowed = 2;
    if (report.renderDeltas.IssueRow > allowed) {
        throw new Error(`APP FAILURE: IssueRow renders ${report.renderDeltas.IssueRow} for ${label} (limit ${allowed})`);
    }
    if (report.memoDeltas.IssueRow <= 0) {
        throw new Error(`APP FAILURE: IssueRow memo skips not observed for ${label}: ${JSON.stringify(report.memoDeltas)}`);
    }
    if (report.operations.createElement !== 0 || report.operations.createTextNode !== 0 ||
        report.operations.appendChild !== 0 || report.operations.removeChild !== 0 ||
        report.operations.replaceChild !== 0 || report.operations.insertBefore !== 0) {
        throw new Error(`APP FAILURE: structural DOM operations during ${label}: ${JSON.stringify(report.operations)}`);
    }
    if (report.operations.addEventListener !== 0 || report.operations.removeEventListener !== 0) {
        throw new Error(`APP FAILURE: listener churn during ${label}: ${JSON.stringify(report.operations)}`);
    }
    console.log(`dashboard ${label} memo summary: ${JSON.stringify({ renderDeltas: report.renderDeltas, memoDeltas: report.memoDeltas })}`);
}

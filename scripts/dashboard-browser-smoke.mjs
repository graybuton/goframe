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
    await assertDashboardTableLayout(client, "initial");
    await assertVirtualTableSpacerStability(client, "initial");

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

    await captureDashboardRowWindow(client, "__dashboardSearchBillingBefore");
    await prepareControlValue(client, "#dashboard-search", "billing");
    await startScenario(client, "search-billing");
    const searchStart = Date.now();
    await dispatchControlEvent(client, "#dashboard-search", "input");
    timings.push({ step: "search-update", ms: Date.now() - searchStart });
    const searchReport = await finishScenario(client, "search-billing");
    const searchAttribution = await readDashboardRowAttribution(client, "__dashboardSearchBillingBefore");
    assertDashboardRowAttribution(searchReport, searchAttribution, "search-billing");
    console.log(`dashboard DOM bridge search-billing attribution: ${JSON.stringify(buildDashboardBridgeAttribution(searchReport, searchAttribution))}`);
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

    const withinRowScrollBefore = await client.evaluate(`(() => {
        const viewport = document.querySelector("[data-testid='issue-table']");
        viewport.scrollTop = 0;
        viewport.dispatchEvent(new Event("scroll", { bubbles: true }));
        window.__dashboardBeforeWithinRowScroll = [...document.querySelectorAll(".issue-row")].map((node) => node.id);
        return window.__dashboardBeforeWithinRowScroll;
    })()`);
    await wait(120);
    await assertVirtualTableSpacerStability(client, "before within-row scroll");
    await startScenario(client, "table-scroll-within-row");
    await client.evaluate(`(() => {
        const viewport = document.querySelector("[data-testid='issue-table']");
        viewport.scrollTop = 10;
        viewport.dispatchEvent(new Event("scroll", { bubbles: true }));
    })()`);
    await wait(120);
    const withinRowScrollReport = await finishScenario(client, "table-scroll-within-row");
    const withinRowScroll = await client.evaluate(`(() => ({
        rowIDs: [...document.querySelectorAll(".issue-row")].map((node) => node.id),
    }))()`);
    assertWithinRowScrollReport(withinRowScrollReport, withinRowScrollBefore, withinRowScroll);
    await assertVirtualTableSpacerStability(client, "within-row scroll");

    const insideBufferBefore = await captureVirtualTableDom(client, "__dashboardInsideBufferBefore");
    await startScenario(client, "table-scroll-inside-buffer");
    await client.evaluate(`(() => {
        const viewport = document.querySelector("[data-testid='issue-table']");
        viewport.scrollTop = 48 * 4;
        viewport.dispatchEvent(new Event("scroll", { bubbles: true }));
    })()`);
    await wait(120);
    const insideBufferReport = await finishScenario(client, "table-scroll-inside-buffer");
    const insideBufferAfter = await readVirtualTableDom(client, "__dashboardInsideBufferBefore");
    assertInsideBufferScrollReport(insideBufferReport, insideBufferBefore, insideBufferAfter);
    await assertVirtualTableSpacerStability(client, "inside-buffer scroll");

    const beyondBufferBefore = await captureVirtualTableDom(client, "__dashboardBeyondBufferBefore");
    await startScenario(client, "table-scroll-beyond-buffer");
    await client.evaluate(`(() => {
        const viewport = document.querySelector("[data-testid='issue-table']");
        viewport.scrollTop = 48 * 20;
        viewport.dispatchEvent(new Event("scroll", { bubbles: true }));
    })()`);
    await wait(160);
    const beyondBufferReport = await finishScenario(client, "table-scroll-beyond-buffer");
    const beyondBufferAfter = await readVirtualTableDom(client, "__dashboardBeyondBufferBefore");
    assertBeyondBufferScrollReport(beyondBufferReport, beyondBufferBefore, beyondBufferAfter);
    await assertVirtualTableSpacerStability(client, "beyond-buffer scroll");

    await client.evaluate(`(() => {
        const viewport = document.querySelector("[data-testid='issue-table']");
        viewport.scrollTop = 0;
        viewport.dispatchEvent(new Event("scroll", { bubbles: true }));
    })()`);
    await wait(160);
    const continuousBefore = await captureVirtualTableDom(client, "__dashboardContinuousBefore");
    await startScenario(client, "table-continuous-scroll-buffered");
    const continuousScrollSteps = await client.evaluate(`(async () => {
        const viewport = document.querySelector("[data-testid='issue-table']");
        let steps = 0;
        for (let top = 0; top <= 48 * 80; top += 12) {
            viewport.scrollTop = top;
            viewport.dispatchEvent(new Event("scroll", { bubbles: true }));
            steps++;
            await new Promise((resolve) => requestAnimationFrame(resolve));
        }
        return steps;
    })()`);
    await wait(240);
    const continuousReport = await finishScenario(client, "table-continuous-scroll-buffered");
    const continuousAfter = await readVirtualTableDom(client, "__dashboardContinuousBefore");
    assertContinuousScrollReport(continuousReport, continuousScrollSteps, continuousBefore, continuousAfter);
    await assertVirtualTableSpacerStability(client, "continuous buffered scroll");

    await client.evaluate(`(() => {
        const viewport = document.querySelector("[data-testid='issue-table']");
        viewport.scrollTop = 0;
        viewport.dispatchEvent(new Event("scroll", { bubbles: true }));
    })()`);
    await wait(160);
    await assertVirtualTableSpacerStability(client, "before cross-row scroll");

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
    await assertVirtualTableSpacerStability(client, "cross-row scroll");
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
    await assertVirtualTableSpacerStability(client, "scroll back to top");
    await client.evaluate(`(() => {
        const before = [...document.querySelectorAll(".issue-row")].slice(0, 8).map((node) => node.id);
        window.__dashboardBeforeSort = before;
        window.__dashboardBeforeSortNodes = {};
        for (const node of document.querySelectorAll(".issue-row")) {
            window.__dashboardBeforeSortNodes[node.id] = node;
        }
    })()`);
    await captureDashboardRowWindow(client, "__dashboardSortAttributionBefore");

    await prepareControlValue(client, "#sort-mode", "priority");
    await startScenario(client, "sort-priority");
    const sortStart = Date.now();
    await dispatchControlEvent(client, "#sort-mode", "change");
    timings.push({ step: "sort-update", ms: Date.now() - sortStart });
    const sortReport = await finishScenario(client, "sort-priority");
    const sortAttribution = await readDashboardRowAttribution(client, "__dashboardSortAttributionBefore");
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
    if (searchAttribution.retainedCount === 0) {
        assertDashboardRowAttribution(sortReport, sortAttribution, "sort-priority fallback");
        if (sortAttribution.retainedCount === 0) {
            throw new Error(`APP FAILURE: dashboard search and sort scenarios have no retained visible rows: ${JSON.stringify({ searchAttribution, sortAttribution })}`);
        }
        console.log(`dashboard DOM bridge sort-priority fallback attribution: ${JSON.stringify(buildDashboardBridgeAttribution(sortReport, sortAttribution))}`);
    }
    await assertVirtualTableSpacerStability(client, "sort");

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
    await assertDashboardTableLayout(client, "status filter");
    await assertVirtualTableSpacerStability(client, "status filter");

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
    await assertDashboardTableLayout(client, "reset");
    await assertVirtualTableSpacerStability(client, "reset");

    client.close();
    console.log(`Dashboard timing report: ${JSON.stringify(timings)}`);
    console.log(`Dashboard performance report: ${JSON.stringify([
        focusReport,
        searchReport,
        selectReport,
        clearSearchReport,
        withinRowScrollReport,
        insideBufferReport,
        beyondBufferReport,
        continuousReport,
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

async function setInputValue(client, selector, value, focus = true) {
    await client.callFunction(`function(selector, value, focus) {
        const input = document.querySelector(selector);
        if (focus) input.focus();
        input.value = value;
        input.dispatchEvent(new Event("input", { bubbles: true }));
        return true;
    }`, selector, value, focus);
    await wait(140);
}

async function setSelectValue(client, selector, value) {
    await client.callFunction(`function(selector, value) {
        const select = document.querySelector(selector);
        select.value = value;
        select.dispatchEvent(new Event("change", { bubbles: true }));
        return true;
    }`, selector, value);
    await wait(140);
}

async function prepareControlValue(client, selector, value) {
    await client.callFunction(`function(selector, value) {
        const control = document.querySelector(selector);
        control.value = value;
        return true;
    }`, selector, value);
}

async function dispatchControlEvent(client, selector, eventType, waitDuration = 140) {
    await client.callFunction(`function(selector, eventType) {
        const control = document.querySelector(selector);
        control.dispatchEvent(new Event(eventType, { bubbles: true }));
        return true;
    }`, selector, eventType);
    await wait(waitDuration);
}

async function clickButtonByText(client, text) {
    await client.callFunction(`function(text) {
        const button = [...document.querySelectorAll("button")].find((node) => node.textContent.trim() === text);
        if (!button) return false;
        button.click();
        return true;
    }`, text);
}

async function metricValue(client, label) {
    return await client.callFunction(`function(label) {
        for (const card of document.querySelectorAll(".metric-card")) {
            if (card.querySelector("span")?.textContent === label) {
                return Number(card.querySelector("strong")?.textContent);
            }
        }
        return -1;
    }`, label);
}

async function firstRowStatus(client) {
    return await client.evaluate(`document.querySelector(".issue-row .status")?.textContent || ""`);
}

async function startScenario(client, label) {
    await client.callFunction(`function(label) {
        const audit = window.__dashboardAudit;
        if (!audit) return false;
        audit.start(label);
        return true;
    }`, label);
}

async function finishScenario(client, label) {
    const report = await client.callFunction(`function(label) {
        return window.__dashboardAudit.finish(label);
    }`, label);
    performanceReports.push(report);
    console.log(`dashboard perf ${label}: ${JSON.stringify(report)}`);
    return report;
}

function assertFocusOnlyReport(report) {
    const nonZeroRenders = Object.entries(report.renderDeltas).filter(([, value]) => value !== 0);
    const nonZeroPatches = Object.entries(report.patchDeltas).filter(([, value]) => value !== 0);
    const nonZeroOperations = Object.entries(report.operations).filter(([name, value]) => name !== "focus" && value !== 0);
    const nonZeroMutations = Object.entries(report.mutations).filter(([, value]) => value !== 0);
    if (nonZeroRenders.length || nonZeroPatches.length || nonZeroOperations.length || nonZeroMutations.length || report.operations.focus !== 1) {
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

async function captureDashboardRowWindow(client, key) {
    return await client.callFunction(`function(key) {
        const rows = [...document.querySelectorAll(".issue-row")];
        const nodes = {};
        for (const row of rows) nodes[row.id] = row;
        const snapshot = {
            ids: rows.map((row) => row.id),
            nodes,
            header: document.querySelector("[data-testid='dashboard-header']"),
            search: document.querySelector("#dashboard-search"),
            table: document.querySelector("[data-testid='issue-table']"),
            detail: document.querySelector("[data-testid='detail-panel']"),
            summary: document.querySelector("[data-testid='visible-summary']")?.textContent.trim() || "",
        };
        window[key] = snapshot;
        return { ids: snapshot.ids, count: snapshot.ids.length, summary: snapshot.summary };
    }`, key);
}

async function readDashboardRowAttribution(client, key) {
    return await client.callFunction(`function(key) {
        const before = window[key];
        if (!before) return null;
        const rows = [...document.querySelectorAll(".issue-row")];
        const afterIDs = rows.map((row) => row.id);
        const afterNodes = {};
        for (const row of rows) afterNodes[row.id] = row;

        const beforeIDs = before.ids || [];
        const beforeSet = new Set(beforeIDs);
        const afterSet = new Set(afterIDs);
        const retainedIDs = afterIDs.filter((id) => beforeSet.has(id));
        const newIDs = afterIDs.filter((id) => !beforeSet.has(id));
        const removedIDs = beforeIDs.filter((id) => !afterSet.has(id));
        const retainedSameNodeIDs = retainedIDs.filter((id) => before.nodes[id] === afterNodes[id]);
        const retainedRecreatedIDs = retainedIDs.filter((id) => before.nodes[id] !== afterNodes[id]);
        const rowOperations = window.__dashboardAudit?.lastRowOperations || {};
        const unique = (ids) => [...new Set((ids || []).filter(Boolean))];
        const countStrings = (values) => {
            const counts = {};
            for (const value of values || []) {
                if (!value) continue;
                counts[value] = (counts[value] || 0) + 1;
            }
            return Object.fromEntries(
                Object.entries(counts).sort(([left], [right]) => left.localeCompare(right)),
            );
        };

        return {
            beforeIDs,
            afterIDs,
            beforeCount: beforeIDs.length,
            afterCount: afterIDs.length,
            retainedIDs,
            newIDs,
            removedIDs,
            retainedCount: retainedIDs.length,
            newCount: newIDs.length,
            removedCount: removedIDs.length,
            retainedSameNodeIDs,
            retainedSameNodeCount: retainedSameNodeIDs.length,
            retainedRecreatedIDs,
            orderChanged: JSON.stringify(beforeIDs) !== JSON.stringify(afterIDs),
            headerSame: before.header === document.querySelector("[data-testid='dashboard-header']"),
            searchSame: before.search === document.querySelector("#dashboard-search"),
            tableSame: before.table === document.querySelector("[data-testid='issue-table']"),
            detailSame: before.detail === document.querySelector("[data-testid='detail-panel']"),
            beforeSummary: before.summary,
            afterSummary: document.querySelector("[data-testid='visible-summary']")?.textContent.trim() || "",
            activeElementID: document.activeElement?.id || "",
            rowOperationIDs: {
                placementIDs: unique(rowOperations.placements),
                removalIDs: unique(rowOperations.removals),
                listenerAddIDs: unique(rowOperations.listenerAdds),
                listenerRemoveIDs: unique(rowOperations.listenerRemoves),
            },
            rowOperationCounts: {
                placements: (rowOperations.placements || []).length,
                removals: (rowOperations.removals || []).length,
                listenerAdds: (rowOperations.listenerAdds || []).length,
                listenerRemoves: (rowOperations.listenerRemoves || []).length,
            },
            placementCountsByRow: countStrings(rowOperations.placements),
            listenerAddCountsByRow: countStrings(rowOperations.listenerAdds),
            listenerRemoveCountsByRow: countStrings(rowOperations.listenerRemoves),
        };
    }`, key);
}

function sameStringSet(left, right) {
    const normalize = (values) => [...new Set(values || [])].sort();
    return JSON.stringify(normalize(left)) === JSON.stringify(normalize(right));
}

function sumCounts(counts) {
    return Object.values(counts || {}).reduce((total, value) => total + value, 0);
}

function everyIDHasCount(ids, counts, expected) {
    return (ids || []).every((id) => counts?.[id] === expected);
}

function assertSingleRAFScheduledUpdate(report, label) {
    const scheduling = report.scheduling;
    if (report.flushes !== 1 || scheduling.requestAnimationFrame !== 1 ||
        scheduling.requestAnimationFrameCallbacks !== 1 || scheduling.queueMicrotask !== 0 ||
        scheduling.queueMicrotaskCallbacks !== 0) {
        throw new Error(`APP FAILURE: ${label} should use one rAF dirty flush with no microtask fallback: ${JSON.stringify(report)}`);
    }
}

function assertDashboardRowAttribution(report, attribution, label) {
    if (!attribution) {
        throw new Error(`APP FAILURE: missing dashboard row attribution for ${label}`);
    }
    const visible = parseSummary(attribution.afterSummary).visible;
    assertMountedRowsBounded(attribution.afterCount, visible, `dashboard ${label} mounted rows`);
    if (attribution.retainedCount + attribution.newCount !== attribution.afterCount ||
        attribution.retainedCount + attribution.removedCount !== attribution.beforeCount) {
        throw new Error(`APP FAILURE: inconsistent dashboard row attribution for ${label}: ${JSON.stringify(attribution)}`);
    }
    if (attribution.retainedSameNodeCount !== attribution.retainedCount || attribution.retainedRecreatedIDs.length !== 0) {
        throw new Error(`APP FAILURE: retained dashboard rows were recreated during ${label}: ${JSON.stringify(attribution)}`);
    }
    if (!attribution.headerSame || !attribution.searchSame || !attribution.tableSame || !attribution.detailSame) {
        throw new Error(`APP FAILURE: dashboard shell node was replaced during ${label}: ${JSON.stringify(attribution)}`);
    }
    const placementIDs = attribution.rowOperationIDs.placementIDs;
    const newRowPlacementIDs = placementIDs.filter((id) => attribution.newIDs.includes(id));
    const missingNewRowPlacementIDs = attribution.newIDs.filter((id) => !placementIDs.includes(id));
    const retainedRowPlacementIDs = placementIDs.filter((id) => attribution.retainedIDs.includes(id));
    if ((attribution.newCount > 0 && missingNewRowPlacementIDs.length !== 0) || retainedRowPlacementIDs.length !== 0) {
        throw new Error(`APP FAILURE: row placement attribution does not match ${label} row replacement: ${JSON.stringify({
            newIDs: attribution.newIDs,
            retainedIDs: attribution.retainedIDs,
            placementIDs,
            newRowPlacementIDs,
            missingNewRowPlacementIDs,
            retainedRowPlacementIDs,
            placementCountsByRow: attribution.placementCountsByRow,
            rawPlacements: report.rowOperations.placements,
            appendChild: report.operations.appendChild,
            insertBefore: report.operations.insertBefore,
            replaceChild: report.operations.replaceChild,
        })}`);
    }
    const listenerAddTargetsMatchNewRows = sameStringSet(attribution.rowOperationIDs.listenerAddIDs, attribution.newIDs);
    const listenerRemoveTargetsMatchRemovedRows = sameStringSet(attribution.rowOperationIDs.listenerRemoveIDs, attribution.removedIDs);
    const listenerAddHistogramKeysMatchNewRows = sameStringSet(Object.keys(attribution.listenerAddCountsByRow), attribution.newIDs);
    const listenerRemoveHistogramKeysMatchRemovedRows = sameStringSet(Object.keys(attribution.listenerRemoveCountsByRow), attribution.removedIDs);
    const everyNewRowHasTwoAdds = everyIDHasCount(attribution.newIDs, attribution.listenerAddCountsByRow, 2);
    const everyRemovedRowHasTwoRemoves = everyIDHasCount(attribution.removedIDs, attribution.listenerRemoveCountsByRow, 2);
    const listenerAddHistogramTotal = sumCounts(attribution.listenerAddCountsByRow);
    const listenerRemoveHistogramTotal = sumCounts(attribution.listenerRemoveCountsByRow);
    // Each mounted IssueRow currently registers its row link and toggle listeners.
    if (!listenerAddTargetsMatchNewRows || !listenerRemoveTargetsMatchRemovedRows ||
        !listenerAddHistogramKeysMatchNewRows || !listenerRemoveHistogramKeysMatchRemovedRows ||
        !everyNewRowHasTwoAdds || !everyRemovedRowHasTwoRemoves ||
        listenerAddHistogramTotal !== attribution.rowOperationCounts.listenerAdds ||
        listenerRemoveHistogramTotal !== attribution.rowOperationCounts.listenerRemoves ||
        attribution.rowOperationCounts.listenerAdds !== report.operations.addEventListener ||
        attribution.rowOperationCounts.listenerRemoves !== report.operations.removeEventListener ||
        report.operations.addEventListener !== attribution.newCount * 2 ||
        report.operations.removeEventListener !== attribution.removedCount * 2) {
        throw new Error(`APP FAILURE: listener churn does not match ${label} row replacement: ${JSON.stringify({
            newIDs: attribution.newIDs,
            removedIDs: attribution.removedIDs,
            listenerAddIDs: attribution.rowOperationIDs.listenerAddIDs,
            listenerRemoveIDs: attribution.rowOperationIDs.listenerRemoveIDs,
            listenerAddCountsByRow: attribution.listenerAddCountsByRow,
            listenerRemoveCountsByRow: attribution.listenerRemoveCountsByRow,
            listenerAdds: report.operations.addEventListener,
            listenerRemoves: report.operations.removeEventListener,
            rowOperationCounts: attribution.rowOperationCounts,
        })}`);
    }
    for (const [name, value] of Object.entries(report.operations)) {
        if (!Number.isFinite(value) || value < 0) {
            throw new Error(`APP FAILURE: invalid ${name} operation count during ${label}: ${JSON.stringify(report)}`);
        }
    }
    for (const [name, value] of Object.entries(report.scheduling)) {
        if (!Number.isFinite(value) || value < 0) {
            throw new Error(`APP FAILURE: invalid ${name} scheduling count during ${label}: ${JSON.stringify(report)}`);
        }
    }
    assertSingleRAFScheduledUpdate(report, label);
}

function buildDashboardBridgeAttribution(report, rows) {
    const operations = report.operations;
    const structuralOperationTotal = operations.createElement + operations.createTextNode + operations.createComment +
        operations.appendChild + operations.insertBefore + operations.removeChild + operations.replaceChild;
    const creationOperationTotal = operations.createElement + operations.createTextNode + operations.createComment;
    const placementOperationTotal = operations.appendChild + operations.insertBefore;
    const removalOperationTotal = operations.removeChild + operations.replaceChild;
    const textPropertyAttributeTotal = operations.setTextNodeValue + operations.setAttribute +
        operations.removeAttribute + operations.setProperty;
    const listenerAddsPerNewVisibleRow = rows.newCount > 0 ? operations.addEventListener / rows.newCount : null;
    const listenerRemovesPerRemovedVisibleRow = rows.removedCount > 0 ? operations.removeEventListener / rows.removedCount : null;
    const placementIDs = rows.rowOperationIDs.placementIDs;
    const newRowPlacementIDs = placementIDs.filter((id) => rows.newIDs.includes(id));
    const missingNewRowPlacementIDs = rows.newIDs.filter((id) => !placementIDs.includes(id));
    const retainedPlacementIDs = placementIDs.filter((id) => rows.retainedIDs.includes(id));
    const listenerAddTargetsMatchNewRows = sameStringSet(rows.rowOperationIDs.listenerAddIDs, rows.newIDs);
    const listenerRemoveTargetsMatchRemovedRows = sameStringSet(rows.rowOperationIDs.listenerRemoveIDs, rows.removedIDs);
    const listenerAddHistogramKeysMatchNewRows = sameStringSet(Object.keys(rows.listenerAddCountsByRow), rows.newIDs);
    const listenerRemoveHistogramKeysMatchRemovedRows = sameStringSet(Object.keys(rows.listenerRemoveCountsByRow), rows.removedIDs);
    const everyNewRowHasTwoAdds = everyIDHasCount(rows.newIDs, rows.listenerAddCountsByRow, 2);
    const everyRemovedRowHasTwoRemoves = everyIDHasCount(rows.removedIDs, rows.listenerRemoveCountsByRow, 2);
    const listenerAddHistogramTotal = sumCounts(rows.listenerAddCountsByRow);
    const listenerRemoveHistogramTotal = sumCounts(rows.listenerRemoveCountsByRow);
    const listenerChurnTracksRowReplacement = listenerAddTargetsMatchNewRows && listenerRemoveTargetsMatchRemovedRows &&
        listenerAddHistogramKeysMatchNewRows && listenerRemoveHistogramKeysMatchRemovedRows &&
        everyNewRowHasTwoAdds && everyRemovedRowHasTwoRemoves &&
        listenerAddHistogramTotal === rows.rowOperationCounts.listenerAdds &&
        listenerRemoveHistogramTotal === rows.rowOperationCounts.listenerRemoves &&
        operations.addEventListener === rows.rowOperationCounts.listenerAdds &&
        operations.removeEventListener === rows.rowOperationCounts.listenerRemoves &&
        operations.addEventListener === rows.newCount * 2 &&
        operations.removeEventListener === rows.removedCount * 2
        ? true
        : "unknown";
    const retainedRowsReinsertedSuspected = retainedPlacementIDs.length > 0;
    const immediateBroadCommitBufferJustified = listenerChurnTracksRowReplacement === true &&
        !retainedRowsReinsertedSuspected && rows.retainedRecreatedIDs.length === 0
        ? false
        : "unknown";

    return {
        scenario: report.label,
        flushes: report.flushes,
        scheduling: report.scheduling,
        operations,
        rows: {
            beforeCount: rows.beforeCount,
            afterCount: rows.afterCount,
            beforeIDs: rows.beforeIDs,
            afterIDs: rows.afterIDs,
            retainedIDs: rows.retainedIDs,
            newIDs: rows.newIDs,
            removedIDs: rows.removedIDs,
            retainedSameNodeCount: rows.retainedSameNodeCount,
            retainedRecreatedIDs: rows.retainedRecreatedIDs,
            orderChanged: rows.orderChanged,
        },
        retention: {
            headerSame: rows.headerSame,
            searchSame: rows.searchSame,
            tableSame: rows.tableSame,
            detailSame: rows.detailSame,
            activeElementID: rows.activeElementID,
        },
        components: {
            renderDeltas: report.renderDeltas,
            patchDeltas: report.patchDeltas,
        },
        listeners: {
            addedRowIDs: rows.rowOperationIDs.listenerAddIDs,
            removedRowIDs: rows.rowOperationIDs.listenerRemoveIDs,
            addCountsByRow: rows.listenerAddCountsByRow,
            removeCountsByRow: rows.listenerRemoveCountsByRow,
            addCalls: rows.rowOperationCounts.listenerAdds,
            removeCalls: rows.rowOperationCounts.listenerRemoves,
            uniqueAddedRowsMatchNewRows: listenerAddTargetsMatchNewRows,
            uniqueRemovedRowsMatchRemovedRows: listenerRemoveTargetsMatchRemovedRows,
            addHistogramKeysMatchNewRows: listenerAddHistogramKeysMatchNewRows,
            removeHistogramKeysMatchRemovedRows: listenerRemoveHistogramKeysMatchRemovedRows,
            everyNewRowHasTwoAdds,
            everyRemovedRowHasTwoRemoves,
            addsPerNewVisibleRow: listenerAddsPerNewVisibleRow,
            removesPerRemovedVisibleRow: listenerRemovesPerRemovedVisibleRow,
        },
        placements: {
            rowIDs: placementIDs,
            countsByRow: rows.placementCountsByRow,
            attributedCalls: rows.rowOperationCounts.placements,
            newRowIDsObserved: newRowPlacementIDs,
            missingNewRowIDs: missingNewRowPlacementIDs,
            retainedRowIDsObserved: retainedPlacementIDs,
        },
        derived: {
            structuralOperationTotal,
            creationOperationTotal,
            placementOperationTotal,
            removalOperationTotal,
            textPropertyAttributeTotal,
            listenerAdds: operations.addEventListener,
            listenerRemoves: operations.removeEventListener,
            listenerNet: operations.addEventListener - operations.removeEventListener,
            retainedVisibleRows: rows.retainedCount,
            newVisibleRows: rows.newCount,
            removedVisibleRows: rows.removedCount,
            retainedRowsRecreated: false,
            retainedRowPlacementIDs: retainedPlacementIDs,
            listenerAddsPerNewVisibleRow,
            listenerRemovesPerRemovedVisibleRow,
            listenerChurnTracksRowReplacement,
            retainedRowsReinsertedSuspected,
            immediateBroadCommitBufferJustified,
            bufferedAlternativeMeasured: false,
            broadCommitBufferBenefit: "unknown",
            narrowPrototypeCandidate: retainedRowsReinsertedSuspected
                ? "measure retained row-range placement separately"
                : null,
        },
    };
}

async function assertDashboardTableLayout(client, label) {
    const layout = await client.evaluate(`(() => {
        const viewport = document.querySelector("[data-testid='issue-table']");
        const table = viewport?.querySelector("table");
        const headerCells = table ? [...table.querySelectorAll("thead th")] : [];
        const firstRow = table?.querySelector("tbody tr.issue-row");
        const firstRowCells = firstRow ? [...firstRow.querySelectorAll("td")] : [];
        const rowLink = firstRow?.querySelector(".row-link");
        const bounds = (node) => {
            const rect = node.getBoundingClientRect();
            return { width: Math.round(rect.width), height: Math.round(rect.height) };
        };
        return {
            viewportWidth: viewport ? Math.round(viewport.getBoundingClientRect().width) : 0,
            tableWidth: table ? Math.round(table.getBoundingClientRect().width) : 0,
            headerCellCount: headerCells.length,
            firstRowCellCount: firstRowCells.length,
            headerWidths: headerCells.map((cell) => bounds(cell).width),
            firstRowWidths: firstRowCells.map((cell) => bounds(cell).width),
            rowLinkWidth: rowLink ? bounds(rowLink).width : 0,
        };
    })()`);
    const failures = [];
    if (layout.headerCellCount !== 7) failures.push(`header cells=${layout.headerCellCount}`);
    if (layout.firstRowCellCount !== 7) failures.push(`row cells=${layout.firstRowCellCount}`);
    if (!(layout.viewportWidth > 0 && layout.tableWidth >= layout.viewportWidth * 0.95)) {
        failures.push(`table width ${layout.tableWidth}, viewport width ${layout.viewportWidth}`);
    }
    for (const [index, width] of layout.headerWidths.entries()) {
        if (width <= 30) failures.push(`header width[${index}]=${width}`);
    }
    if ((layout.firstRowWidths[0] ?? 0) <= 100) {
        failures.push(`issue column width=${layout.firstRowWidths[0] ?? 0}`);
    }
    if ((layout.firstRowWidths[6] ?? 0) <= 50) {
        failures.push(`action column width=${layout.firstRowWidths[6] ?? 0}`);
    }
    if (layout.rowLinkWidth <= 100) {
        failures.push(`row link width=${layout.rowLinkWidth}`);
    }
    if (failures.length > 0) {
        throw new Error(`APP FAILURE: dashboard table layout collapsed during ${label}: ${failures.join("; ")} ${JSON.stringify(layout)}`);
    }
    console.log(`dashboard table layout ${label}: ok`);
}

async function assertVirtualTableSpacerStability(client, label) {
    const state = await client.evaluate(`(() => {
        const tbody = document.querySelector("[data-testid='issue-table'] tbody");
        const top = tbody?.querySelector(".gf-virtual-table-spacer-top") || null;
        const bottom = tbody?.querySelector(".gf-virtual-table-spacer-bottom") || null;
        const spacers = tbody ? [...tbody.querySelectorAll(".gf-virtual-table-spacer")] : [];
        if (top && !window.__dashboardSpacerTop) window.__dashboardSpacerTop = top;
        if (bottom && !window.__dashboardSpacerBottom) window.__dashboardSpacerBottom = bottom;
        return {
            spacerCount: spacers.length,
            topExists: Boolean(top),
            bottomExists: Boolean(bottom),
            topSame: Boolean(top && window.__dashboardSpacerTop === top),
            bottomSame: Boolean(bottom && window.__dashboardSpacerBottom === bottom),
            childCount: tbody?.children.length ?? 0,
            rowCount: document.querySelectorAll(".issue-row").length,
        };
    })()`);
    const failures = [];
    if (state.spacerCount !== 2) failures.push(`spacer count=${state.spacerCount}`);
    if (!state.topExists) failures.push("top spacer missing");
    if (!state.bottomExists) failures.push("bottom spacer missing");
    if (!state.topSame) failures.push("top spacer node identity changed");
    if (!state.bottomSame) failures.push("bottom spacer node identity changed");
    assertMountedRowsBounded(state.rowCount, dashboardLogicalRows, `dashboard spacer stability ${label} mounted rows`);
    if (failures.length > 0) {
        throw new Error(`APP FAILURE: virtual table spacers unstable during ${label}: ${failures.join("; ")} ${JSON.stringify(state)}`);
    }
    console.log(`dashboard virtual table spacers ${label}: ok`);
}

function assertWithinRowScrollReport(report, beforeState, state) {
    const before = JSON.stringify(beforeState);
    const current = JSON.stringify(state.rowIDs);
    if (before !== current) {
        throw new Error(`APP FAILURE: within-row scroll changed mounted row IDs: before ${before}, after ${current}`);
    }
    if (report.renderDeltas.VirtualTable !== 0 || report.renderDeltas.IssueRow !== 0) {
        throw new Error(`APP FAILURE: within-row scroll should not render VirtualTable/IssueRow: ${JSON.stringify(report.renderDeltas)}`);
    }
    if (report.operations.createElement !== 0 || report.operations.createComment !== 0 ||
        report.operations.removeChild !== 0 ||
        report.operations.insertBefore !== 0 || report.operations.appendChild !== 0) {
        throw new Error(`APP FAILURE: within-row scroll should not churn DOM: ${JSON.stringify(report.operations)}`);
    }
    console.log(`dashboard within-row scroll no-op summary: ${JSON.stringify({ renderDeltas: report.renderDeltas, operations: report.operations })}`);
}

async function captureVirtualTableDom(client, key) {
    return await client.callFunction(`function(key) {
        const tbody = document.querySelector("[data-testid='issue-table'] tbody");
        const top = tbody?.querySelector(".gf-virtual-table-spacer-top") || null;
        const bottom = tbody?.querySelector(".gf-virtual-table-spacer-bottom") || null;
        const rows = [...document.querySelectorAll(".issue-row")];
        window[key] = {
            top,
            bottom,
            rowIDs: rows.map((node) => node.id),
            childCount: tbody?.children.length ?? 0,
        };
        return {
            rowIDs: rows.map((node) => node.id),
            rowCount: rows.length,
            childCount: tbody?.children.length ?? 0,
            spacerCount: tbody ? tbody.querySelectorAll(".gf-virtual-table-spacer").length : 0,
            topExists: Boolean(top),
            bottomExists: Boolean(bottom),
        };
    }`, key);
}

async function readVirtualTableDom(client, key) {
    return await client.callFunction(`function(key) {
        const before = window[key] || {};
        const tbody = document.querySelector("[data-testid='issue-table'] tbody");
        const top = tbody?.querySelector(".gf-virtual-table-spacer-top") || null;
        const bottom = tbody?.querySelector(".gf-virtual-table-spacer-bottom") || null;
        const rows = [...document.querySelectorAll(".issue-row")];
        return {
            rowIDs: rows.map((node) => node.id),
            rowCount: rows.length,
            childCount: tbody?.children.length ?? 0,
            spacerCount: tbody ? tbody.querySelectorAll(".gf-virtual-table-spacer").length : 0,
            topSame: Boolean(top && before.top === top),
            bottomSame: Boolean(bottom && before.bottom === bottom),
            rowIDsSame: JSON.stringify(before.rowIDs || []) === JSON.stringify(rows.map((node) => node.id)),
            childCountSame: before.childCount === (tbody?.children.length ?? 0),
        };
    }`, key);
}

function assertInsideBufferScrollReport(report, beforeState, state) {
    assertMountedRowsBounded(state.rowCount, dashboardLogicalRows, "dashboard inside-buffer mounted rows");
    if (!state.topSame || !state.bottomSame || state.spacerCount !== 2) {
        throw new Error(`APP FAILURE: inside-buffer scroll changed spacer identity: ${JSON.stringify(state)}`);
    }
    if (!state.rowIDsSame || !state.childCountSame || beforeState.childCount !== state.childCount) {
        throw new Error(`APP FAILURE: inside-buffer scroll changed mounted rows: before ${JSON.stringify(beforeState)}, after ${JSON.stringify(state)}`);
    }
    if (report.renderDeltas.VirtualTable !== 0 || report.renderDeltas.IssueRow !== 0) {
        throw new Error(`APP FAILURE: inside-buffer scroll should not render VirtualTable/IssueRow: ${JSON.stringify(report.renderDeltas)}`);
    }
    if (report.operations.createElement !== 0 || report.operations.createComment !== 0 ||
        report.operations.removeChild !== 0 ||
        report.operations.insertBefore !== 0 || report.operations.appendChild !== 0 ||
        report.operations.addEventListener !== 0 || report.operations.removeEventListener !== 0) {
        throw new Error(`APP FAILURE: inside-buffer scroll should not churn DOM/listeners: ${JSON.stringify(report.operations)}`);
    }
    console.log(`dashboard inside-buffer scroll no-op summary: ${JSON.stringify({ renderDeltas: report.renderDeltas, operations: report.operations })}`);
}

function assertBeyondBufferScrollReport(report, beforeState, state) {
    assertMountedRowsBounded(state.rowCount, dashboardLogicalRows, "dashboard beyond-buffer mounted rows");
    if (!state.topSame || !state.bottomSame || state.spacerCount !== 2) {
        throw new Error(`APP FAILURE: beyond-buffer scroll changed spacer identity: ${JSON.stringify(state)}`);
    }
    if (state.rowIDsSame) {
        throw new Error(`APP FAILURE: beyond-buffer scroll should update mounted row window: before ${JSON.stringify(beforeState)}, after ${JSON.stringify(state)}`);
    }
    if (report.renderDeltas.VirtualTable <= 0) {
        throw new Error(`APP FAILURE: beyond-buffer scroll should render VirtualTable: ${JSON.stringify(report.renderDeltas)}`);
    }
    if (report.operations.addEventListener !== report.operations.removeEventListener) {
        throw new Error(`APP FAILURE: beyond-buffer scroll listener net changed: ${JSON.stringify(report.operations)}`);
    }
    const rowCount = Math.max(beforeState.rowCount, state.rowCount, 1);
    const changedRows = Math.max(1, countRowWindowChanges(beforeState.rowIDs, state.rowIDs));
    const created = report.operations.createElement + report.operations.createTextNode + report.operations.createComment;
    const inserted = report.operations.insertBefore + report.operations.appendChild;
    const createdLimit = rowCount * 16;
    const insertedLimit = rowCount * 16;
    const removedLimit = rowCount * 4;
    const listenerLimit = rowCount * 2;
    if (created > createdLimit || inserted > insertedLimit || report.operations.removeChild > removedLimit ||
        report.operations.addEventListener > listenerLimit || report.operations.removeEventListener > listenerLimit) {
        throw new Error(`APP FAILURE: beyond-buffer scroll DOM churn is unbounded: ${JSON.stringify({
            operations: report.operations,
            rowCount,
            changedRows,
            limits: { createdLimit, insertedLimit, removedLimit, listenerLimit },
        })}`);
    }
    console.log(`dashboard beyond-buffer scroll bounded summary: ${JSON.stringify({ renderDeltas: report.renderDeltas, operations: report.operations, rowCount, changedRows, limits: { createdLimit, insertedLimit, removedLimit, listenerLimit }, beforeRows: beforeState.rowCount, afterRows: state.rowCount })}`);
}

function countRowWindowChanges(beforeIDs, afterIDs) {
    const before = new Set(beforeIDs || []);
    let changed = 0;
    for (const id of afterIDs || []) {
        if (!before.has(id)) {
            changed++;
        }
    }
    return changed;
}

function assertContinuousScrollReport(report, scrollSteps, beforeState, state) {
    assertMountedRowsBounded(state.rowCount, dashboardLogicalRows, "dashboard continuous-scroll mounted rows");
    if (!state.topSame || !state.bottomSame || state.spacerCount !== 2) {
        throw new Error(`APP FAILURE: continuous scroll changed spacer identity: ${JSON.stringify(state)}`);
    }
    const maxTableRenders = Math.ceil(scrollSteps / 4);
    if (report.renderDeltas.VirtualTable > maxTableRenders) {
        throw new Error(`APP FAILURE: continuous scroll rendered VirtualTable too often: renders=${report.renderDeltas.VirtualTable}, steps=${scrollSteps}, report=${JSON.stringify(report)}`);
    }
    if (report.renderDeltas.IssueRow > maxMountedRows * maxTableRenders) {
        throw new Error(`APP FAILURE: continuous scroll rendered IssueRow too often: ${JSON.stringify(report.renderDeltas)}`);
    }
    const created = report.operations.createElement + report.operations.createTextNode + report.operations.createComment;
    const inserted = report.operations.insertBefore + report.operations.appendChild;
    if (created > maxMountedRows * maxTableRenders || inserted > maxMountedRows * maxTableRenders ||
        report.operations.removeChild > maxMountedRows * maxTableRenders) {
        throw new Error(`APP FAILURE: continuous scroll DOM churn is unbounded: ${JSON.stringify(report.operations)}`);
    }
    if (report.operations.addEventListener !== report.operations.removeEventListener) {
        throw new Error(`APP FAILURE: continuous scroll listener net changed: ${JSON.stringify(report.operations)}`);
    }
    console.log(`dashboard continuous buffered scroll summary: ${JSON.stringify({ scrollSteps, tableRenders: report.renderDeltas.VirtualTable, rowRenders: report.renderDeltas.IssueRow, operations: report.operations, beforeRows: beforeState.rowCount, afterRows: state.rowCount })}`);
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
        createComment: 0,
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
        focus: 0,
        setSelectionRange: 0,
    };
}

function emptyScheduling() {
    return {
        requestAnimationFrame: 0,
        requestAnimationFrameCallbacks: 0,
        queueMicrotask: 0,
        queueMicrotaskCallbacks: 0,
    };
}

function emptyRowOperations() {
    return {
        placements: [],
        removals: [],
        listenerAdds: [],
        listenerRemoves: [],
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
        const scheduling = ${JSON.stringify(emptyScheduling())};
        const rowOperations = ${JSON.stringify(emptyRowOperations())};
        const componentNames = ${JSON.stringify(names)};
        const renderReports = [];
        window.goframeRenderProbe = (phase, duration) => {
            renderReports.push({ phase, duration });
        };
        const audit = {
            operations,
            mutations,
            scheduling,
            rowOperations,
            lastRowOperations: null,
            baseline: null,
            renderBaseline: 0,
            startedAt: 0,
            label: "",
            start(label) {
                this.label = label;
                this.startedAt = performance.now();
                for (const key of Object.keys(this.operations)) this.operations[key] = 0;
                for (const key of Object.keys(this.mutations)) this.mutations[key] = 0;
                for (const key of Object.keys(this.scheduling)) this.scheduling[key] = 0;
                clearRowOperations(this.rowOperations);
                this.lastRowOperations = null;
                this.baseline = snapshotCounts(componentNames);
                this.renderBaseline = renderReports.length;
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
            const updates = renderReports.slice(this.renderBaseline).filter((report) => report.phase === "update");
            const rowOperationSnapshot = snapshotRowOperations(this.rowOperations);
            this.lastRowOperations = rowOperationSnapshot;
            return {
                label,
                durationMs: Math.round((performance.now() - this.startedAt) * 100) / 100,
                flushes: updates.length,
                scheduling: { ...this.scheduling },
                renderDeltas,
                patchDeltas,
                memoDeltas,
                operations: { ...this.operations },
                mutations: { ...this.mutations },
                rowOperations: rowOperationSnapshot,
                rows: document.querySelectorAll(".issue-row").length,
                summary: document.querySelector("[data-testid='visible-summary']")?.textContent.trim() || "",
            };
            },
        };

        const rowIDsForNode = (node) => {
            const ids = [];
            const seen = new Set();
            const addRow = (row) => {
                const id = row?.id || "";
                if (!id || seen.has(id)) return;
                seen.add(id);
                ids.push(id);
            };
            const visit = (candidate) => {
                if (!candidate) return;
                if (candidate.nodeType === Node.DOCUMENT_FRAGMENT_NODE) {
                    for (const child of candidate.childNodes) visit(child);
                    return;
                }
                if (candidate.nodeType === Node.ELEMENT_NODE) {
                    const enclosing = candidate.matches(".issue-row")
                        ? candidate
                        : candidate.closest(".issue-row");
                    if (enclosing) {
                        addRow(enclosing);
                        return;
                    }
                    for (const row of candidate.querySelectorAll(".issue-row")) addRow(row);
                    return;
                }
                if (candidate.parentElement) addRow(candidate.parentElement.closest(".issue-row"));
            };
            visit(node);
            return ids;
        };
        const rowIDForTarget = (node) => rowIDsForNode(node)[0] || "";
        const recordRowOperation = (bucket, node) => {
            for (const id of rowIDsForNode(node)) rowOperations[bucket].push(id);
        };
        const recordRowTarget = (bucket, node) => {
            rowOperations[bucket].push(node);
        };
        const wrap = (owner, name, counter, onCall) => {
            const original = owner[name];
            owner[name] = function(...args) {
                operations[counter]++;
                if (onCall) onCall.call(this, args);
                return original.apply(this, args);
            };
        };
        wrap(Document.prototype, "createElement", "createElement");
        wrap(Document.prototype, "createTextNode", "createTextNode");
        wrap(Document.prototype, "createComment", "createComment");
        wrap(Node.prototype, "appendChild", "appendChild", function(args) {
            recordRowOperation("placements", args[0]);
        });
        wrap(Node.prototype, "removeChild", "removeChild", function(args) {
            recordRowOperation("removals", args[0]);
        });
        wrap(Node.prototype, "replaceChild", "replaceChild", function(args) {
            recordRowOperation("placements", args[0]);
            recordRowOperation("removals", args[1]);
        });
        wrap(Node.prototype, "insertBefore", "insertBefore", function(args) {
            recordRowOperation("placements", args[0]);
        });
        wrap(Element.prototype, "setAttribute", "setAttribute");
        wrap(Element.prototype, "removeAttribute", "removeAttribute");
        wrap(EventTarget.prototype, "addEventListener", "addEventListener", function() {
            recordRowTarget("listenerAdds", this);
        });
        wrap(EventTarget.prototype, "removeEventListener", "removeEventListener", function() {
            recordRowTarget("listenerRemoves", this);
        });
        wrap(HTMLElement.prototype, "focus", "focus");

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
        if (typeof HTMLInputElement.prototype.setSelectionRange === "function") {
            const setSelectionRange = HTMLInputElement.prototype.setSelectionRange;
            HTMLInputElement.prototype.setSelectionRange = function(...args) {
                operations.setSelectionRange++;
                return setSelectionRange.apply(this, args);
            };
        }

        const requestAnimationFrame = window.requestAnimationFrame;
        if (typeof requestAnimationFrame === "function") {
            window.requestAnimationFrame = function(callback) {
                scheduling.requestAnimationFrame++;
                return requestAnimationFrame.call(window, (...args) => {
                    scheduling.requestAnimationFrameCallbacks++;
                    return callback(...args);
                });
            };
        }
        const queueMicrotask = window.queueMicrotask;
        if (typeof queueMicrotask === "function") {
            window.queueMicrotask = function(callback) {
                scheduling.queueMicrotask++;
                return queueMicrotask.call(window, () => {
                    scheduling.queueMicrotaskCallbacks++;
                    return callback();
                });
            };
        }

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

        function clearRowOperations(next) {
            for (const key of Object.keys(next)) next[key].length = 0;
        }

        function snapshotRowOperations(next) {
            const rowIDs = (values) => values.map((value) => typeof value === "string" ? value : rowIDForTarget(value)).filter(Boolean);
            return {
                placements: [...next.placements],
                removals: [...next.removals],
                listenerAdds: rowIDs(next.listenerAdds),
                listenerRemoves: rowIDs(next.listenerRemoves),
            };
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
        report.operations.createComment !== 0 ||
        report.operations.appendChild !== 0 || report.operations.removeChild !== 0 ||
        report.operations.replaceChild !== 0 || report.operations.insertBefore !== 0) {
        throw new Error(`APP FAILURE: structural DOM operations during ${label}: ${JSON.stringify(report.operations)}`);
    }
    if (report.operations.addEventListener !== 0 || report.operations.removeEventListener !== 0) {
        throw new Error(`APP FAILURE: listener churn during ${label}: ${JSON.stringify(report.operations)}`);
    }
    console.log(`dashboard ${label} memo summary: ${JSON.stringify({ renderDeltas: report.renderDeltas, memoDeltas: report.memoDeltas })}`);
}

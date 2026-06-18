import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const scriptDir = dirname(fileURLToPath(import.meta.url));
const rootDir = resolve(scriptDir, "..");
const appDir = join(rootDir, "examples/dashboard");
const appURLBase = process.env.GOFRAME_DASHBOARD_PRESSURE_URL ?? "http://127.0.0.1";
const chrome = process.env.CHROME ?? "google-chrome";
const cycles = Number(process.env.GOFRAME_DASHBOARD_PRESSURE_CYCLES ?? "20");
const settleFrames = Number(process.env.GOFRAME_DASHBOARD_PRESSURE_SETTLE_FRAMES ?? "2");
const postIdleMs = Number(process.env.GOFRAME_DASHBOARD_PRESSURE_POST_IDLE_MS ?? "3000");
const profile = await mkdtemp(join(tmpdir(), "goframe-dashboard-pressure-"));
const expectedAllRows = 300;
const expectedTableColumns = 7;
const maxMountedRows = 70;
const maxAllCreateNodes = 2500;
const maxAllEventAdds = 160;
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

let browser = null;
let server = null;
let browserError = "";
let browserExit = null;

try {
    await prepareDashboardDebugPackage();

    const serverPort = await pickFreePort();
    const debugPort = await pickFreePort();
    const appURL = `${appURLBase}:${serverPort}/?pressure=${Date.now()}`;
    const expectedApp = new URL(appURL);

    server = startServer(serverPort);
    await waitForServer(serverPort, server);

    browser = startChrome(debugPort);
    const page = await waitForPage(debugPort);
    const client = await connect(page.webSocketDebuggerUrl);
    await client.call("Runtime.enable");
    await client.call("Page.enable");
    await client.call("Performance.enable");
    await client.call("HeapProfiler.enable").catch(() => {});

    await navigateToApp(client, appURL);
    await waitForAppPage(client, expectedApp, "initial navigation");
    await collectGarbage(client);
    await client.evaluate(installPressureAuditExpression(componentNames));

    const initial = await samplePage(client, "initial", 0, "all");
    if (initial.logicalRows !== expectedAllRows || initial.rows <= 0 || initial.rows > maxMountedRows) {
        throw new Error(`APP FAILURE: dashboard should start with ${expectedAllRows} logical rows and bounded mounted rows: ${JSON.stringify(initial)}`);
    }
    await assertDashboardTableLayout(client, "initial");
    await assertVirtualTableSpacerStability(client, "initial");
    const scrollRecords = [
        await runScrollStability(client, "scroll-within-row", 10),
        await runScrollStability(client, "scroll-cross-row", 48 * 8 + 10),
        await runScrollStability(client, "scroll-back-top", 0),
    ];

    const records = [];
    for (let cycle = 1; cycle <= cycles; cycle++) {
        records.push(await runTransition(client, cycle, "open", "Open"));
        records.push(await runTransition(client, cycle, "all", "All"));
    }

    const continuousScrollRecord = await runContinuousScrollAudit(client);

    await wait(postIdleMs);
    await collectGarbage(client);
    const finalIdle = await samplePage(client, "post-idle-gc", cycles, "all");

    printRecords(records);
    printScrollRecords(scrollRecords);
    printContinuousScrollRecord(continuousScrollRecord);
    const analysis = analyzeRecords(records, finalIdle, continuousScrollRecord);
    printAnalysis(initial, finalIdle, analysis);
    if (analysis.failures.length > 0) {
        throw new Error(`APP FAILURE: dashboard DOM pressure leak guard failed:\n${analysis.failures.join("\n")}`);
    }

    client.close();
    console.log("Dashboard DOM pressure audit: ok");
} finally {
    if (browser) {
        const exited = new Promise((resolveExit) => browser.once("exit", resolveExit));
        killProcessGroup(browser);
        await Promise.race([exited, wait(2000)]);
    }
    if (server) {
        const exited = new Promise((resolveExit) => server.once("exit", resolveExit));
        killProcessGroup(server);
        await Promise.race([exited, wait(2000)]);
    }
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function prepareDashboardDebugPackage() {
    console.log("== Dashboard pressure package ==");
    await runGoxc(["package", "./examples/dashboard", "--compiler=tinygo"]);
    await runCommand("tinygo", [
        "build",
        "-target=wasm",
        "-no-debug",
        "-panic=trap",
        "-tags=goframe_debug",
        "-o",
        join(appDir, ".goframe/package/standalone/assets/bundle.wasm"),
        ".",
    ], {
        cwd: join(appDir, ".goframe/work/dev"),
    });
}

async function runGoxc(args) {
    const configured = process.env.GOXC;
    if (configured) {
        await runCommand(configured, args, { cwd: rootDir });
        return;
    }
    await runCommand("go", ["run", "./cmd/goxc", ...args], { cwd: rootDir });
}

async function runCommand(command, args, options = {}) {
    await new Promise((resolveRun, rejectRun) => {
        const child = spawn(command, args, {
            cwd: options.cwd ?? rootDir,
            stdio: "inherit",
            env: process.env,
        });
        child.on("error", rejectRun);
        child.on("exit", (code) => {
            if (code === 0) {
                resolveRun();
                return;
            }
            rejectRun(new Error(`${command} ${args.join(" ")} exited with ${code}`));
        });
    });
}

async function pickFreePort() {
    return await new Promise((resolvePort, rejectPort) => {
        const probe = createServer();
        probe.on("error", rejectPort);
        probe.listen(0, "127.0.0.1", () => {
            const address = probe.address();
            probe.close(() => resolvePort(address.port));
        });
    });
}

function startServer(port) {
    const child = spawnGoxc(["serve", "./examples/dashboard", `--port=${port}`], {
        stdio: ["ignore", "pipe", "pipe"],
        detached: true,
    });
    child.stdout.on("data", (chunk) => process.stdout.write(chunk));
    child.stderr.on("data", (chunk) => process.stderr.write(chunk));
    return child;
}

function spawnGoxc(args, options) {
    const configured = process.env.GOXC;
    if (configured) {
        return spawn(configured, args, { cwd: rootDir, env: process.env, ...options });
    }
    return spawn("go", ["run", "./cmd/goxc", ...args], { cwd: rootDir, env: process.env, ...options });
}

async function waitForServer(port, child) {
    for (let attempt = 0; attempt < 160; attempt++) {
        if (child.exitCode !== null) {
            throw new Error(`HARNESS FAILURE: goxc serve exited before becoming ready with code ${child.exitCode}`);
        }
        try {
            const response = await fetch(`http://127.0.0.1:${port}/`);
            if (response.ok) {
                return;
            }
        } catch {}
        await wait(100);
    }
    throw new Error(`HARNESS FAILURE: goxc serve did not become ready on port ${port}`);
}

function startChrome(debugPort) {
    const child = spawn(chrome, [
        "--headless",
        "--no-sandbox",
        "--disable-gpu",
        "--enable-precise-memory-info",
        `--remote-debugging-port=${debugPort}`,
        `--user-data-dir=${profile}`,
        "about:blank",
    ], {
        stdio: ["ignore", "ignore", "pipe"],
        detached: true,
    });
    child.stderr.on("data", (chunk) => {
        browserError += chunk;
    });
    child.on("exit", (code, signal) => {
        browserExit = { code, signal };
    });
    return child;
}

async function waitForPage(port) {
    let lastError;
    for (let attempt = 0; attempt < 60; attempt++) {
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
    for (let attempt = 0; attempt < 120; attempt++) {
        lastState = await pageState(client);
        if (lastState.href.startsWith("chrome-error://")) {
            throw new Error(`HARNESS FAILURE: ${label}: Chrome loaded an error document: ${JSON.stringify(lastState)}`);
        }
        if (isExpectedAppState(lastState, expected) && lastState.root && lastState.appReady) {
            return;
        }
        await wait(100);
    }
    throw new Error(`HARNESS FAILURE: ${label}: app page did not become ready: ${JSON.stringify(lastState)}`);
}

async function pageState(client) {
    return await client.evaluate(`(() => ({
        href: window.location.href,
        origin: window.location.origin,
        protocol: window.location.protocol,
        readyState: document.readyState,
        root: Boolean(document.querySelector("#root")),
        appReady: Boolean(document.querySelector("#dashboard-search") && window.goframeComponentRenderCounts?.App),
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

async function runTransition(client, cycle, status, label) {
    await collectGarbage(client);
    const metricsBefore = await performanceMetrics(client);
    await client.evaluate(`window.__dashboardPressure.start(${JSON.stringify(label)})`);
    const tracePromise = startTrace(client);
    await client.evaluate(`(() => {
        const select = document.querySelector("#status-filter");
        select.value = ${JSON.stringify(status)};
        select.dispatchEvent(new Event("change", { bubbles: true }));
    })()`);
    await settle(client);
    const audit = await client.evaluate(`window.__dashboardPressure.finish(${JSON.stringify(label)})`);
    const trace = await stopTrace(client, tracePromise);
    const metricsAfter = await performanceMetrics(client);
    await collectGarbage(client);
    const afterGC = await samplePage(client, label, cycle, status);
    const spacerState = await assertVirtualTableSpacerStability(client, `${label} cycle ${cycle}`);
    if (status === "all") {
        await assertDashboardTableLayout(client, `${label} cycle ${cycle}`);
    }

    return {
        cycle,
        transition: label,
        status,
        durationMs: audit.durationMs,
        rows: audit.rows,
        logicalRows: audit.logicalRows,
        totalRows: audit.totalRows,
        summary: audit.summary,
        liveDOMNodes: audit.liveDOMNodes,
        allNodesAfterGC: afterGC.liveDOMNodes,
        cdpNodes: metricsAfter.Nodes ?? 0,
        cdpNodesAfterGC: afterGC.metrics.Nodes ?? 0,
        jsEventListeners: metricsAfter.JSEventListeners ?? 0,
        jsEventListenersAfterGC: afterGC.metrics.JSEventListeners ?? 0,
        jsHeapUsed: metricsAfter.JSHeapUsedSize ?? 0,
        jsHeapUsedAfterGC: afterGC.metrics.JSHeapUsedSize ?? 0,
        netListeners: audit.netListeners,
        operations: audit.operations,
        renderDeltas: audit.renderDeltas,
        patchDeltas: audit.patchDeltas,
        memoDeltas: audit.memoDeltas,
        renderReports: audit.renderReports,
        runtimeRenderMs: round(sumBy(audit.renderReports, (entry) => entry.duration || 0)),
        performanceDeltas: diffMetrics(metricsBefore, metricsAfter),
        spacerCount: spacerState.spacerCount,
        spacerTopStable: spacerState.topSame,
        spacerBottomStable: spacerState.bottomSame,
        trace,
    };
}

async function runScrollStability(client, label, scrollTop) {
    await client.evaluate(`window.__dashboardPressure.start(${JSON.stringify(label)})`);
    await client.evaluate(`(() => {
        const viewport = document.querySelector("[data-testid='issue-table']");
        viewport.scrollTop = ${scrollTop};
        viewport.dispatchEvent(new Event("scroll", { bubbles: true }));
    })()`);
    await settle(client);
    const audit = await client.evaluate(`window.__dashboardPressure.finish(${JSON.stringify(label)})`);
    const spacerState = await assertVirtualTableSpacerStability(client, label);
    return {
        label,
        scrollTop,
        rows: audit.rows,
        operations: audit.operations,
        renderDeltas: audit.renderDeltas,
        patchDeltas: audit.patchDeltas,
        memoDeltas: audit.memoDeltas,
        spacerCount: spacerState.spacerCount,
        spacerTopStable: spacerState.topSame,
        spacerBottomStable: spacerState.bottomSame,
    };
}

async function runContinuousScrollAudit(client) {
    const label = "continuous-scroll-buffered";
    await client.evaluate(`(() => {
        const viewport = document.querySelector("[data-testid='issue-table']");
        viewport.scrollTop = 0;
        viewport.dispatchEvent(new Event("scroll", { bubbles: true }));
    })()`);
    await settle(client);
    const before = await client.evaluate(`(() => {
        const tbody = document.querySelector("[data-testid='issue-table'] tbody");
        const top = tbody?.querySelector(".gf-virtual-table-spacer-top") || null;
        const bottom = tbody?.querySelector(".gf-virtual-table-spacer-bottom") || null;
        window.__dashboardPressureContinuousTop = top;
        window.__dashboardPressureContinuousBottom = bottom;
        return {
            rows: document.querySelectorAll(".issue-row").length,
            childCount: tbody?.children.length ?? 0,
            spacerCount: tbody ? tbody.querySelectorAll(".gf-virtual-table-spacer").length : 0,
        };
    })()`);
    await client.evaluate(`window.__dashboardPressure.start(${JSON.stringify(label)})`);
    const scrollSummary = await client.evaluate(`(async () => {
        const viewport = document.querySelector("[data-testid='issue-table']");
        let steps = 0;
        let maxRows = document.querySelectorAll(".issue-row").length;
        for (let top = 0; top <= 48 * 80; top += 12) {
            viewport.scrollTop = top;
            viewport.dispatchEvent(new Event("scroll", { bubbles: true }));
            steps++;
            maxRows = Math.max(maxRows, document.querySelectorAll(".issue-row").length);
            await new Promise((resolve) => requestAnimationFrame(resolve));
            maxRows = Math.max(maxRows, document.querySelectorAll(".issue-row").length);
        }
        return { steps, maxRows };
    })()`);
    await settle(client);
    const audit = await client.evaluate(`window.__dashboardPressure.finish(${JSON.stringify(label)})`);
    const spacerState = await assertVirtualTableSpacerStability(client, label);
    const after = await client.evaluate(`(() => {
        const tbody = document.querySelector("[data-testid='issue-table'] tbody");
        const top = tbody?.querySelector(".gf-virtual-table-spacer-top") || null;
        const bottom = tbody?.querySelector(".gf-virtual-table-spacer-bottom") || null;
        return {
            rows: document.querySelectorAll(".issue-row").length,
            childCount: tbody?.children.length ?? 0,
            spacerCount: tbody ? tbody.querySelectorAll(".gf-virtual-table-spacer").length : 0,
            topSame: Boolean(top && window.__dashboardPressureContinuousTop === top),
            bottomSame: Boolean(bottom && window.__dashboardPressureContinuousBottom === bottom),
        };
    })()`);
    const mountedRowsMax = Math.max(before.rows, scrollSummary.maxRows, after.rows);
    return {
        label,
        scrollSteps: scrollSummary.steps,
        durationMs: audit.durationMs,
        rows: audit.rows,
        mountedRowsMax,
        operations: audit.operations,
        renderDeltas: audit.renderDeltas,
        patchDeltas: audit.patchDeltas,
        memoDeltas: audit.memoDeltas,
        netListeners: audit.netListeners,
        listenerNetDelta: audit.operations.addEventListener - audit.operations.removeEventListener,
        renderScrollRatio: round(audit.renderDeltas.VirtualTable / scrollSummary.steps),
        spacerCount: spacerState.spacerCount,
        spacerTopStable: spacerState.topSame && after.topSame,
        spacerBottomStable: spacerState.bottomSame && after.bottomSame,
        childCountBefore: before.childCount,
        childCountAfter: after.childCount,
    };
}

async function settle(client) {
    await client.evaluate(`new Promise((resolve) => {
        let frames = ${settleFrames};
        const tick = () => {
            if (frames-- <= 0) {
                setTimeout(resolve, 0);
                return;
            }
            requestAnimationFrame(tick);
        };
        requestAnimationFrame(tick);
    })`);
}

async function samplePage(client, label, cycle, status) {
    const page = await client.evaluate(`(() => ({
        label: ${JSON.stringify(label)},
        cycle: ${cycle},
        status: ${JSON.stringify(status)},
        rows: document.querySelectorAll(".issue-row").length,
        liveDOMNodes: document.querySelectorAll("*").length,
        summary: document.querySelector("[data-testid='visible-summary']")?.textContent.trim() || "",
    }))()`);
    const parsed = parseSummary(page.summary);
    page.logicalRows = parsed.visible;
    page.totalRows = parsed.total;
    return { ...page, metrics: await performanceMetrics(client) };
}

async function assertDashboardTableLayout(client, label) {
    const layout = await client.evaluate(`(() => {
        const viewport = document.querySelector("[data-testid='issue-table']");
        const table = viewport?.querySelector("table");
        const headerCells = table ? [...table.querySelectorAll("thead th")] : [];
        const firstRow = table?.querySelector("tbody tr.issue-row");
        const firstRowCells = firstRow ? [...firstRow.querySelectorAll("td")] : [];
        const rowLink = firstRow?.querySelector(".row-link");
        return {
            viewportWidth: viewport ? Math.round(viewport.getBoundingClientRect().width) : 0,
            tableWidth: table ? Math.round(table.getBoundingClientRect().width) : 0,
            headerCellCount: headerCells.length,
            firstRowCellCount: firstRowCells.length,
            headerWidths: headerCells.map((cell) => Math.round(cell.getBoundingClientRect().width)),
            firstRowWidths: firstRowCells.map((cell) => Math.round(cell.getBoundingClientRect().width)),
            rowLinkWidth: rowLink ? Math.round(rowLink.getBoundingClientRect().width) : 0,
        };
    })()`);
    const failures = [];
    if (layout.headerCellCount !== expectedTableColumns) failures.push(`header cells=${layout.headerCellCount}`);
    if (layout.firstRowCellCount !== expectedTableColumns) failures.push(`row cells=${layout.firstRowCellCount}`);
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
}

async function assertVirtualTableSpacerStability(client, label) {
    const state = await client.evaluate(`(() => {
        const tbody = document.querySelector("[data-testid='issue-table'] tbody");
        const top = tbody?.querySelector(".gf-virtual-table-spacer-top") || null;
        const bottom = tbody?.querySelector(".gf-virtual-table-spacer-bottom") || null;
        const spacers = tbody ? [...tbody.querySelectorAll(".gf-virtual-table-spacer")] : [];
        if (top && !window.__dashboardPressureSpacerTop) window.__dashboardPressureSpacerTop = top;
        if (bottom && !window.__dashboardPressureSpacerBottom) window.__dashboardPressureSpacerBottom = bottom;
        return {
            spacerCount: spacers.length,
            topExists: Boolean(top),
            bottomExists: Boolean(bottom),
            topSame: Boolean(top && window.__dashboardPressureSpacerTop === top),
            bottomSame: Boolean(bottom && window.__dashboardPressureSpacerBottom === bottom),
            rowCount: document.querySelectorAll(".issue-row").length,
        };
    })()`);
    const failures = [];
    if (state.spacerCount !== 2) failures.push(`spacer count=${state.spacerCount}`);
    if (!state.topExists) failures.push("top spacer missing");
    if (!state.bottomExists) failures.push("bottom spacer missing");
    if (!state.topSame) failures.push("top spacer identity changed");
    if (!state.bottomSame) failures.push("bottom spacer identity changed");
    if (state.rowCount <= 0 || state.rowCount > maxMountedRows) {
        failures.push(`mounted rows=${state.rowCount}, limit ${maxMountedRows}`);
    }
    if (failures.length > 0) {
        throw new Error(`APP FAILURE: virtual table spacers unstable during ${label}: ${failures.join("; ")} ${JSON.stringify(state)}`);
    }
    return state;
}

async function performanceMetrics(client) {
    try {
        const result = await client.call("Performance.getMetrics");
        const metrics = {};
        for (const entry of result.metrics || []) {
            metrics[entry.name] = entry.value;
        }
        return metrics;
    } catch {
        return {};
    }
}

function diffMetrics(before, after) {
    const durationNames = [
        "TaskDuration",
        "ScriptDuration",
        "LayoutDuration",
        "RecalcStyleDuration",
    ];
    const countNames = [
        "LayoutCount",
        "RecalcStyleCount",
    ];
    const deltas = {};
    for (const name of durationNames) {
        deltas[name] = round(((after[name] ?? 0) - (before[name] ?? 0)) * 1000);
    }
    for (const name of countNames) {
        deltas[name] = round((after[name] ?? 0) - (before[name] ?? 0));
    }
    return deltas;
}

async function collectGarbage(client) {
    try {
        await client.call("HeapProfiler.collectGarbage");
        return true;
    } catch {
        return false;
    }
}

async function startTrace(client) {
    const events = [];
    let complete;
    const completePromise = new Promise((resolveComplete) => {
        complete = resolveComplete;
    });
    const offData = client.on("Tracing.dataCollected", (params) => {
        events.push(...(params.value || []));
    });
    const offComplete = client.on("Tracing.tracingComplete", () => {
        offData();
        offComplete();
        complete(events);
    });
    try {
        await client.call("Tracing.start", {
            categories: "devtools.timeline,disabled-by-default-devtools.timeline",
            transferMode: "ReportEvents",
        });
        return { completePromise, available: true };
    } catch {
        offData();
        offComplete();
        complete([]);
        return { completePromise, available: false };
    }
}

async function stopTrace(client, tracePromise) {
    if (!tracePromise.available) {
        return { traceAvailable: false };
    }
    try {
        await client.call("Tracing.end");
        const events = await Promise.race([tracePromise.completePromise, wait(3000).then(() => [])]);
        return summarizeTrace(events);
    } catch {
        return { traceAvailable: false };
    }
}

function summarizeTrace(events) {
    const totals = {
        traceAvailable: events.length > 0,
        functionCallMs: 0,
        evaluateScriptMs: 0,
        updateLayoutTreeMs: 0,
        layoutMs: 0,
        paintMs: 0,
        compositeLayersMs: 0,
    };
    for (const event of events) {
        if (event.ph !== "X" || typeof event.dur !== "number") continue;
        const ms = event.dur / 1000;
        switch (event.name) {
        case "FunctionCall":
            totals.functionCallMs += ms;
            break;
        case "EvaluateScript":
            totals.evaluateScriptMs += ms;
            break;
        case "UpdateLayoutTree":
        case "RecalculateStyles":
            totals.updateLayoutTreeMs += ms;
            break;
        case "Layout":
            totals.layoutMs += ms;
            break;
        case "Paint":
            totals.paintMs += ms;
            break;
        case "CompositeLayers":
            totals.compositeLayersMs += ms;
            break;
        }
    }
    for (const key of Object.keys(totals)) {
        if (typeof totals[key] === "number") {
            totals[key] = round(totals[key]);
        }
    }
    return totals;
}

function installPressureAuditExpression(names) {
    return `(() => {
        if (window.__dashboardPressure) return true;

        const componentNames = ${JSON.stringify(names)};
        const operations = ${JSON.stringify(emptyOperations())};
        let netListeners = 0;
        const renderReports = [];
        window.goframeRenderProbe = (phase, duration) => {
            renderReports.push({ phase, duration });
        };

        const audit = {
            baseline: null,
            operationBaseline: null,
            reportBaseline: 0,
            startedAt: 0,
            start(label) {
                this.startedAt = performance.now();
                this.baseline = snapshotCounts(componentNames);
                this.operationBaseline = { ...operations, netListeners };
                this.reportBaseline = renderReports.length;
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
                const summary = document.querySelector("[data-testid='visible-summary']")?.textContent.trim() || "";
                const match = /^Showing (\\d+) of (\\d+) issues$/.exec(summary);
                return {
                    label,
                    durationMs: Math.round((performance.now() - this.startedAt) * 100) / 100,
                    rows: document.querySelectorAll(".issue-row").length,
                    logicalRows: match ? Number(match[1]) : -1,
                    totalRows: match ? Number(match[2]) : -1,
                    liveDOMNodes: document.querySelectorAll("*").length,
                    summary,
                    netListeners,
                    operations: opDeltas,
                    renderDeltas,
                    patchDeltas,
                    memoDeltas,
                    renderReports: renderReports.slice(this.reportBaseline),
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
        wrap(Document.prototype, "createComment", "createComment");
        wrap(Document.prototype, "createDocumentFragment", "createDocumentFragment");
        wrap(Node.prototype, "appendChild", "appendChild");
        wrap(Node.prototype, "removeChild", "removeChild");
        wrap(Node.prototype, "replaceChild", "replaceChild");
        wrap(Node.prototype, "insertBefore", "insertBefore");
        wrap(Element.prototype, "setAttribute", "setAttribute");
        wrap(Element.prototype, "removeAttribute", "removeAttribute");
        wrap(EventTarget.prototype, "addEventListener", "addEventListener", () => { netListeners++; });
        wrap(EventTarget.prototype, "removeEventListener", "removeEventListener", () => { netListeners--; });

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

        window.__dashboardPressure = audit;
        return true;

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
        createComment: 0,
        createDocumentFragment: 0,
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

function printRecords(records) {
    const rows = records.map((record) => ({
        cycle: record.cycle,
        to: record.transition,
        ms: record.durationMs,
        rows: record.rows,
        logical: record.logicalRows,
        liveDOM: record.liveDOMNodes,
        cdpNodesGC: record.cdpNodesAfterGC,
        jsListenersGC: record.jsEventListenersAfterGC,
        netListeners: record.netListeners,
        create: record.operations.createElement + record.operations.createTextNode + record.operations.createComment,
        insert: record.operations.insertBefore + record.operations.appendChild,
        remove: record.operations.removeChild,
        addEvt: record.operations.addEventListener,
        remEvt: record.operations.removeEventListener,
        rowRender: record.renderDeltas.IssueRow,
        rowPatch: record.patchDeltas.IssueRow,
        rowMemo: record.memoDeltas.IssueRow,
        spacers: record.spacerCount,
        topStable: record.spacerTopStable,
        bottomStable: record.spacerBottomStable,
        rtRenderMs: record.runtimeRenderMs,
        scriptMs: record.performanceDeltas.ScriptDuration,
        layoutMs: record.performanceDeltas.LayoutDuration,
        recalcMs: record.performanceDeltas.RecalcStyleDuration,
        fnMs: record.trace.functionCallMs ?? 0,
        paintMs: (record.trace.paintMs ?? 0) + (record.trace.compositeLayersMs ?? 0),
        heapGC: record.jsHeapUsedAfterGC,
    }));
    console.table(rows);
}

function printScrollRecords(records) {
    console.table(records.map((record) => ({
        label: record.label,
        scrollTop: record.scrollTop,
        rows: record.rows,
        create: record.operations.createElement + record.operations.createTextNode + record.operations.createComment,
        insert: record.operations.insertBefore + record.operations.appendChild,
        remove: record.operations.removeChild,
        addEvt: record.operations.addEventListener,
        remEvt: record.operations.removeEventListener,
        tableRender: record.renderDeltas.VirtualTable,
        rowRender: record.renderDeltas.IssueRow,
        rowPatch: record.patchDeltas.IssueRow,
        rowMemo: record.memoDeltas.IssueRow,
        spacers: record.spacerCount,
        topStable: record.spacerTopStable,
        bottomStable: record.spacerBottomStable,
    })));
}

function printContinuousScrollRecord(record) {
    console.log(`Dashboard DOM pressure continuous scroll: ${JSON.stringify({
        scrollSteps: record.scrollSteps,
        virtualTableRenders: record.renderDeltas.VirtualTable,
        issueRowRenders: record.renderDeltas.IssueRow,
        renderScrollRatio: record.renderScrollRatio,
        createElement: record.operations.createElement,
        removeChild: record.operations.removeChild,
        insertBefore: record.operations.insertBefore,
        appendChild: record.operations.appendChild,
        listenerNetDelta: record.listenerNetDelta,
        spacersStable: record.spacerCount === 2 && record.spacerTopStable && record.spacerBottomStable,
        mountedRowsMax: record.mountedRowsMax,
    })}`);
}

function analyzeRecords(records, finalIdle, continuousScrollRecord) {
    const failures = [];
    const warnings = [];
    const allRecords = records.filter((record) => record.status === "all");
    const openRecords = records.filter((record) => record.status === "open");

    for (const record of allRecords) {
        if (record.logicalRows !== expectedAllRows) {
            failures.push(`cycle ${record.cycle} All logical rows=${record.logicalRows}, want ${expectedAllRows}`);
        }
        if (record.rows <= 0 || record.rows > maxMountedRows) {
            failures.push(`cycle ${record.cycle} All mounted rows=${record.rows}, limit ${maxMountedRows}`);
        }
        const createdNodes = record.operations.createElement + record.operations.createTextNode + record.operations.createComment;
        if (createdNodes > maxAllCreateNodes) {
            failures.push(`cycle ${record.cycle} All created DOM nodes=${createdNodes}, limit ${maxAllCreateNodes}`);
        }
        if (record.operations.addEventListener > maxAllEventAdds) {
            failures.push(`cycle ${record.cycle} All addEventListener delta=${record.operations.addEventListener}, limit ${maxAllEventAdds}`);
        }
        if (record.spacerCount !== 2 || !record.spacerTopStable || !record.spacerBottomStable) {
            failures.push(`cycle ${record.cycle} All spacer instability: count=${record.spacerCount}, top=${record.spacerTopStable}, bottom=${record.spacerBottomStable}`);
        }
    }
    for (const record of openRecords) {
        if (record.logicalRows <= 0 || record.logicalRows >= expectedAllRows) {
            failures.push(`cycle ${record.cycle} Open logical rows=${record.logicalRows}, expected filtered subset`);
        }
        if (record.rows <= 0 || record.rows > maxMountedRows) {
            failures.push(`cycle ${record.cycle} Open mounted rows=${record.rows}, limit ${maxMountedRows}`);
        }
        if (record.spacerCount !== 2 || !record.spacerTopStable || !record.spacerBottomStable) {
            failures.push(`cycle ${record.cycle} Open spacer instability: count=${record.spacerCount}, top=${record.spacerTopStable}, bottom=${record.spacerBottomStable}`);
        }
    }

    checkStable(allRecords, "liveDOMNodes", "All live DOM nodes", 0, failures);
    checkStable(allRecords, "netListeners", "All net listener delta", 0, failures);
    checkStable(allRecords, "jsEventListenersAfterGC", "All CDP JS event listeners after GC", 2, failures);
    checkStable(openRecords, "liveDOMNodes", "Open live DOM nodes", 0, failures);
    checkStable(openRecords, "netListeners", "Open net listener delta", 0, failures);

    const cdpNodeDrift = drift(allRecords.map((record) => record.cdpNodesAfterGC));
    if (cdpNodeDrift > 5) {
        warnings.push(`CDP Nodes after GC drifted by ${cdpNodeDrift}; live DOM stability decides leak failure.`);
    }

    const averageAllCreate = average(allRecords.map((record) =>
        record.operations.createElement + record.operations.createTextNode + record.operations.createComment));
    const averageAllRemove = average(allRecords.map((record) => record.operations.removeChild));
    const averageAllDuration = average(allRecords.map((record) => record.durationMs));
    const averageAllLayout = average(allRecords.map((record) => record.performanceDeltas.LayoutDuration));
    const averageAllScript = average(allRecords.map((record) => record.performanceDeltas.ScriptDuration));
    const averageAllRender = average(allRecords.map((record) => record.runtimeRenderMs));
    if (averageAllDuration > 150) {
        warnings.push(`average All transition duration ${round(averageAllDuration)}ms remains high despite bounded DOM.`);
    }

    if (continuousScrollRecord) {
        const maxTableRenders = Math.ceil(continuousScrollRecord.scrollSteps / 4);
        const createdNodes = continuousScrollRecord.operations.createElement +
            continuousScrollRecord.operations.createTextNode +
            continuousScrollRecord.operations.createComment;
        const insertedNodes = continuousScrollRecord.operations.insertBefore +
            continuousScrollRecord.operations.appendChild;
        const maxChurn = maxMountedRows * maxTableRenders;
        if (continuousScrollRecord.renderDeltas.VirtualTable > maxTableRenders) {
            failures.push(`continuous scroll VirtualTable renders=${continuousScrollRecord.renderDeltas.VirtualTable}, scroll steps=${continuousScrollRecord.scrollSteps}, limit=${maxTableRenders}`);
        }
        if (continuousScrollRecord.mountedRowsMax <= 0 || continuousScrollRecord.mountedRowsMax > maxMountedRows) {
            failures.push(`continuous scroll mounted rows max=${continuousScrollRecord.mountedRowsMax}, limit=${maxMountedRows}`);
        }
        if (continuousScrollRecord.spacerCount !== 2 || !continuousScrollRecord.spacerTopStable || !continuousScrollRecord.spacerBottomStable) {
            failures.push(`continuous scroll spacer instability: count=${continuousScrollRecord.spacerCount}, top=${continuousScrollRecord.spacerTopStable}, bottom=${continuousScrollRecord.spacerBottomStable}`);
        }
        if (continuousScrollRecord.listenerNetDelta !== 0) {
            failures.push(`continuous scroll listener net delta=${continuousScrollRecord.listenerNetDelta}`);
        }
        if (createdNodes > maxChurn || insertedNodes > maxChurn || continuousScrollRecord.operations.removeChild > maxChurn) {
            failures.push(`continuous scroll DOM churn unbounded: create=${createdNodes}, insert=${insertedNodes}, remove=${continuousScrollRecord.operations.removeChild}, limit=${maxChurn}`);
        }
    }

    return {
        failures,
        warnings,
        summary: {
            cycles,
            allRowsLogical: expectedAllRows,
            allRowsMountedMax: Math.max(...allRecords.map((record) => record.rows)),
            openRowsLogical: openRecords[0]?.logicalRows ?? 0,
            openRowsMountedMax: Math.max(...openRecords.map((record) => record.rows)),
            averageAllDuration: round(averageAllDuration),
            averageAllCreateNodes: round(averageAllCreate),
            averageAllRemoveNodes: round(averageAllRemove),
            averageAllRuntimeRenderMs: round(averageAllRender),
            averageAllScriptMs: round(averageAllScript),
            averageAllLayoutMs: round(averageAllLayout),
            maxAllDuration: Math.max(...allRecords.map((record) => record.durationMs)),
            liveDOMAllStart: allRecords[0]?.liveDOMNodes ?? 0,
            liveDOMAllEnd: allRecords.at(-1)?.liveDOMNodes ?? 0,
            netListenersAllStart: allRecords[0]?.netListeners ?? 0,
            netListenersAllEnd: allRecords.at(-1)?.netListeners ?? 0,
            cdpNodesAllStart: allRecords[0]?.cdpNodesAfterGC ?? 0,
            cdpNodesAllEnd: allRecords.at(-1)?.cdpNodesAfterGC ?? 0,
            postIdleLiveDOMNodes: finalIdle.liveDOMNodes,
            postIdleCDPNodes: finalIdle.metrics.Nodes ?? 0,
            postIdleJSEventListeners: finalIdle.metrics.JSEventListeners ?? 0,
            postIdleJSHeapUsed: finalIdle.metrics.JSHeapUsedSize ?? 0,
            spacerTopStable: allRecords.every((record) => record.spacerTopStable),
            spacerBottomStable: allRecords.every((record) => record.spacerBottomStable),
            continuousScrollSteps: continuousScrollRecord?.scrollSteps ?? 0,
            continuousVirtualTableRenders: continuousScrollRecord?.renderDeltas.VirtualTable ?? 0,
            continuousIssueRowRenders: continuousScrollRecord?.renderDeltas.IssueRow ?? 0,
            continuousRenderScrollRatio: continuousScrollRecord?.renderScrollRatio ?? 0,
            continuousSpacersStable: continuousScrollRecord ?
                continuousScrollRecord.spacerCount === 2 && continuousScrollRecord.spacerTopStable && continuousScrollRecord.spacerBottomStable :
                false,
            continuousMountedRowsMax: continuousScrollRecord?.mountedRowsMax ?? 0,
            continuousListenerNetDelta: continuousScrollRecord?.listenerNetDelta ?? 0,
        },
    };
}

function printAnalysis(initial, finalIdle, analysis) {
    console.log(`Dashboard DOM pressure initial: ${JSON.stringify(initial)}`);
    console.log(`Dashboard DOM pressure post-idle GC: ${JSON.stringify(finalIdle)}`);
    console.log(`Dashboard DOM pressure summary: ${JSON.stringify(analysis.summary)}`);
    for (const warning of analysis.warnings) {
        console.warn(`WARNING: ${warning}`);
    }
}

function checkStable(records, field, label, tolerance, failures) {
    if (records.length === 0) return;
    const values = records.map((record) => record[field]);
    const min = Math.min(...values);
    const max = Math.max(...values);
    const first = values[0];
    const last = values.at(-1);
    if (max - min > tolerance || last - first > tolerance) {
        failures.push(`${label} grew or failed to stabilize: first=${first}, last=${last}, min=${min}, max=${max}, tolerance=${tolerance}`);
    }
}

function average(values) {
    if (values.length === 0) return 0;
    return values.reduce((sum, value) => sum + value, 0) / values.length;
}

function drift(values) {
    if (values.length === 0) return 0;
    return Math.max(...values) - Math.min(...values);
}

function parseSummary(summary) {
    const match = /^Showing (\d+) of (\d+) issues$/.exec(summary || "");
    if (!match) {
        return { visible: -1, total: -1 };
    }
    return { visible: Number(match[1]), total: Number(match[2]) };
}

function sumBy(values, pick) {
    return values.reduce((sum, value) => sum + pick(value), 0);
}

function round(value) {
    return Math.round(value * 100) / 100;
}

function killProcessGroup(child) {
    try {
        process.kill(-child.pid, "SIGTERM");
        return;
    } catch {}
    try {
        child.kill("SIGTERM");
    } catch {}
}

async function connect(url) {
    const socket = new WebSocket(url);
    await new Promise((resolveSocket, rejectSocket) => {
        socket.addEventListener("open", resolveSocket, { once: true });
        socket.addEventListener("error", rejectSocket, { once: true });
    });

    let nextID = 1;
    const pending = new Map();
    const listeners = new Map();
    socket.addEventListener("message", (event) => {
        const message = JSON.parse(event.data);
        if (message.id && pending.has(message.id)) {
            const request = pending.get(message.id);
            pending.delete(message.id);
            if (message.error) {
                request.reject(new Error(message.error.message));
            } else {
                request.resolve(message.result || {});
            }
            return;
        }
        const callbacks = listeners.get(message.method);
        if (!callbacks) return;
        for (const callback of callbacks) callback(message.params || {});
    });

    const call = (method, params = {}) =>
        new Promise((resolveCall, rejectCall) => {
            const id = nextID++;
            pending.set(id, { resolve: resolveCall, reject: rejectCall });
            socket.send(JSON.stringify({ id, method, params }));
        });

    return {
        call,
        close: () => socket.close(),
        on: (method, callback) => {
            if (!listeners.has(method)) listeners.set(method, new Set());
            listeners.get(method).add(callback);
            return () => listeners.get(method)?.delete(callback);
        },
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

function wait(duration) {
    return new Promise((resolveWait) => setTimeout(resolveWait, duration));
}

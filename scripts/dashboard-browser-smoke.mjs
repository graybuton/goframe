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

    assertDeepEqual(
        await client.evaluate(`(() => {
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
        })()`),
        {
            header: true,
            search: true,
            metrics: true,
            table: true,
            detail: true,
            rows: 300,
            summary: "Showing 300 of 300 issues",
            appRenders: 1,
            headerRenders: 1,
        },
        "dashboard initial render",
    );

    const searchStart = Date.now();
    await setInputValue(client, "#dashboard-search", "billing");
    timings.push({ step: "search-update", ms: Date.now() - searchStart });
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
    if (!(searchState.rows > 0 && searchState.rows < 300)) {
        throw new Error(`APP FAILURE: search should narrow rows: ${JSON.stringify(searchState)}`);
    }
    assertDeepEqual(
        {
            summary: searchState.summary,
            headerSame: searchState.headerSame,
            searchSame: searchState.searchSame,
            metricsVisible: searchState.metricsVisible,
            headerRenders: searchState.headerRenders,
        },
        {
            summary: `Showing ${searchState.rows} of 300 issues`,
            headerSame: true,
            searchSame: true,
            metricsVisible: searchState.rows,
            headerRenders: 1,
        },
        "dashboard search updates visible rows without replacing header/search",
    );

    const selectStart = Date.now();
    await client.evaluate(`document.querySelector(".issue-row .row-link").click()`);
    await wait(120);
    timings.push({ step: "selection-update", ms: Date.now() - selectStart });
    const selected = await client.evaluate(`(() => {
        const first = document.querySelector(".issue-row");
        return {
            selected: Boolean(document.querySelector(".issue-row-selected")),
            detail: document.querySelector("[data-testid='detail-panel']")?.textContent.includes(first.id.replace("issue-row-", "")),
            headerSame: window.__dashboardHeader === document.querySelector("[data-testid='dashboard-header']"),
            searchSame: window.__dashboardSearch === document.querySelector("#dashboard-search"),
        };
    })()`);
    assertDeepEqual(
        selected,
        { selected: true, detail: true, headerSame: true, searchSame: true },
        "dashboard row selection updates detail only",
    );

    await setInputValue(client, "#dashboard-search", "");
    await wait(120);
    windowlessAssert(
        await client.evaluate(`(() => {
            window.__dashboardRow1 = document.querySelector("#issue-row-1");
            const before = [...document.querySelectorAll(".issue-row")].slice(0, 8).map((node) => node.id);
            window.__dashboardBeforeSort = before;
            return { rows: document.querySelectorAll(".issue-row").length, row1: Boolean(window.__dashboardRow1) };
        })()`),
        (value) => value.rows === 300 && value.row1,
        "APP FAILURE: dashboard reset before sort did not restore 300 visible rows",
    );

    const sortStart = Date.now();
    await setSelectValue(client, "#sort-mode", "priority");
    timings.push({ step: "sort-update", ms: Date.now() - sortStart });
    const sorted = await client.evaluate(`(() => {
        const after = [...document.querySelectorAll(".issue-row")].slice(0, 8).map((node) => node.id);
        return {
            rows: document.querySelectorAll(".issue-row").length,
            row1Same: window.__dashboardRow1 === document.querySelector("#issue-row-1"),
            changed: JSON.stringify(after) !== JSON.stringify(window.__dashboardBeforeSort),
            headerSame: window.__dashboardHeader === document.querySelector("[data-testid='dashboard-header']"),
        };
    })()`);
    assertDeepEqual(
        sorted,
        { rows: 300, row1Same: true, changed: true, headerSame: true },
        "dashboard sort reorders keyed rows without replacing survivors",
    );

    await setSelectValue(client, "#status-filter", "blocked");
    const filtered = await client.evaluate(`(() => {
        const rows = document.querySelectorAll(".issue-row").length;
        return {
            rows,
            summary: document.querySelector("[data-testid='visible-summary']")?.textContent.trim(),
            headerSame: window.__dashboardHeader === document.querySelector("[data-testid='dashboard-header']"),
        };
    })()`);
    if (!(filtered.rows > 0 && filtered.rows < 300)) {
        throw new Error(`APP FAILURE: status filter should narrow rows: ${JSON.stringify(filtered)}`);
    }
    assertDeepEqual(
        { summary: filtered.summary, headerSame: filtered.headerSame },
        { summary: `Showing ${filtered.rows} of 300 issues`, headerSame: true },
        "dashboard status filter updates table predictably",
    );

    await clickButtonByText(client, "Reset");
    await wait(120);
    const beforeEvents = await metricValue(client, "Events");
    await clickButtonByText(client, "Simulate update");
    await wait(120);
    const afterEvents = await metricValue(client, "Events");
    if (!(afterEvents > beforeEvents)) {
        throw new Error(`APP FAILURE: simulate update did not increase events: before ${beforeEvents}, after ${afterEvents}`);
    }
    console.log("dashboard simulate update changes metrics: ok");

    await clickButtonByText(client, "Reset");
    await wait(120);
    assertDeepEqual(
        await client.evaluate(`(() => ({
            rows: document.querySelectorAll(".issue-row").length,
            summary: document.querySelector("[data-testid='visible-summary']")?.textContent.trim(),
            title: document.title,
            headerSame: window.__dashboardHeader === document.querySelector("[data-testid='dashboard-header']"),
            searchSame: window.__dashboardSearch === document.querySelector("#dashboard-search"),
        }))()`),
        {
            rows: 300,
            summary: "Showing 300 of 300 issues",
            title: "GoFrame Dashboard · 300 visible",
            headerSame: true,
            searchSame: true,
        },
        "dashboard reset restores deterministic view",
    );

    client.close();
    console.log(`Dashboard timing report: ${JSON.stringify(timings)}`);
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

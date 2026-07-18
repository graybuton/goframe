import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const appURL = process.argv[2] ?? process.env.GOFRAME_ROUTER_DASHBOARD_SMOKE_URL ?? "http://127.0.0.1:18080/";
const debugPort = Number(process.env.GOFRAME_ROUTER_DASHBOARD_CHROME_DEBUG_PORT ?? "19250");
const chrome = process.env.CHROME ?? "google-chrome";
const profile = await mkdtemp(join(tmpdir(), "goframe-router-dashboard-smoke-"));
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
browser.stderr.on("data", (chunk) => {
    browserError += chunk;
});

try {
    const page = await waitForPage(debugPort);
    const client = await connect(page.webSocketDebuggerUrl);
    await client.call("Runtime.enable");
    await client.call("Page.enable");

    await navigateToApp(client, withSmokeParam(appURL));
    await waitForAppPage(client, expectedApp);
    await client.evaluate(`window.__routerDashboardSmokeShell = document.querySelector("[data-testid='rd-shell']")`);
    await waitForResourceStatus(client, "ready");

    assertState(await appState(client), {
        route: "home",
        hash: "",
        resourceStatus: "ready",
        resourceAttempt: "1",
        boundaryFallback: false,
        shellSame: true,
    }, "router-dashboard initial home");

    await client.evaluate(`document.querySelector("[data-testid='rd-nav-issues']").click()`);
    await waitForRoute(client, "issues");
    assertState(await appState(client), {
        route: "issues",
        hash: "#/issues",
        rowCount: 12,
        queryValue: "",
        statusValue: "all",
        resourceStatus: "ready",
        resourceAttempt: "1",
        boundaryFallback: false,
        shellSame: true,
    }, "router-dashboard issues route");

    await setInput(client, "[data-testid='rd-filter-query']", "auth");
    await setSelect(client, "[data-testid='rd-filter-status']", "open");
    await settleAnimationFrame(client);
    await client.evaluate(`document.querySelector("[data-testid='rd-filter-apply']").click()`);
    await waitForCondition(async () => {
        const state = await appState(client);
        return state.route === "issues" && state.rowCount === 1 && state.hash === "#/issues?q=auth&status=open";
    }, "filtered issues route");
    assertState(await appState(client), {
        route: "issues",
        hash: "#/issues?q=auth&status=open",
        rowCount: 1,
        queryValue: "auth",
        statusValue: "open",
        resourceStatus: "ready",
        resourceAttempt: "1",
        boundaryFallback: false,
        shellSame: true,
    }, "router-dashboard query filter");

    const filteredHash = (await appState(client)).hash;
    await client.evaluate(`document.querySelector("[data-testid='rd-resource-reload']").click()`);
    await waitForCondition(async () => {
        const state = await appState(client);
        return state.resourceStatus === "loading" || state.resourceAttempt === "2";
    }, "resource reload starts");
    await waitForResourceStatus(client, "ready");
    assertState(await appState(client), {
        route: "issues",
        hash: filteredHash,
        rowCount: 1,
        queryValue: "auth",
        statusValue: "open",
        resourceStatus: "ready",
        resourceAttempt: "2",
        boundaryFallback: false,
        shellSame: true,
    }, "router-dashboard resource reload preserves query");

    await client.evaluate(`history.back()`);
    await waitForCondition(async () => {
        const state = await appState(client);
        return state.route === "issues" && state.hash === "#/issues" && state.rowCount === 12;
    }, "browser back restores query");
    assertState(await appState(client), {
        route: "issues",
        hash: "#/issues",
        rowCount: 12,
        queryValue: "",
        statusValue: "all",
        resourceStatus: "ready",
        resourceAttempt: "2",
        boundaryFallback: false,
        shellSame: true,
    }, "router-dashboard browser back");

    await client.evaluate(`document.querySelector("[data-testid='rd-issue-link-RD-2']").click()`);
    await waitForRoute(client, "details");
    assertState(await appState(client), {
        route: "details",
        hash: "#/issues/RD-2",
        detailTitle: "Billing dashboard needs clearer empty state",
        resourceStatus: "ready",
        resourceAttempt: "2",
        boundaryFallback: false,
        shellSame: true,
    }, "router-dashboard detail route");

    await client.evaluate(`document.querySelector("[data-testid='rd-edit-link']").click()`);
    await waitForRoute(client, "edit");
    assertState(await appState(client), {
        route: "edit",
        hash: "#/issues/RD-2/edit",
        formTitle: "Billing dashboard needs clearer empty state",
        formDirty: "No local changes",
        titleAriaInvalid: "false",
        resourceStatus: "ready",
        resourceAttempt: "2",
        boundaryFallback: false,
        shellSame: true,
    }, "router-dashboard edit route");

    await client.evaluate(`history.back()`);
    await waitForRoute(client, "details");
    assertState(await appState(client), {
        route: "details",
        hash: "#/issues/RD-2",
        resourceStatus: "ready",
        resourceAttempt: "2",
        boundaryFallback: false,
        shellSame: true,
    }, "router-dashboard detail route after edit back");

    await client.evaluate(`location.hash = "#/issues/RD-2?panic=render"`);
    await waitForRoute(client, "boundary");
    assertState(await appState(client), {
        route: "boundary",
        hash: "#/issues/RD-2?panic=render",
        resourceStatus: "ready",
        resourceAttempt: "2",
        resourceLoading: false,
        resourceFailed: false,
        boundaryFallback: true,
        boundaryResetPresent: true,
        boundaryBackPresent: true,
        shellSame: true,
    }, "router-dashboard route error fallback");

    await client.evaluate(`document.querySelector("[data-testid='rd-boundary-back-to-issues']").click()`);
    await waitForRoute(client, "issues");
    assertState(await appState(client), {
        route: "issues",
        hash: "#/issues",
        rowCount: 12,
        queryValue: "",
        statusValue: "all",
        resourceStatus: "ready",
        resourceAttempt: "2",
        boundaryFallback: false,
        shellSame: true,
    }, "router-dashboard route error safe recovery");

    await client.evaluate(`document.querySelector("[data-testid='rd-issue-link-RD-2']").click()`);
    await waitForRoute(client, "details");
    assertState(await appState(client), {
        route: "details",
        hash: "#/issues/RD-2",
        detailTitle: "Billing dashboard needs clearer empty state",
        resourceStatus: "ready",
        resourceAttempt: "2",
        boundaryFallback: false,
        shellSame: true,
    }, "router-dashboard route error post-recovery interaction");

    await client.evaluate(`document.querySelector("[data-testid='rd-resource-fail']").click()`);
    await waitForResourceStatus(client, "failed");
    const failed = await appState(client);
    if (!failed.resourceError.includes("fetch")) {
        throw new Error(`APP FAILURE: router-dashboard resource failure did not mention fetch: ${JSON.stringify(failed)}`);
    }
    assertState(failed, {
        route: "resource",
        hash: "#/issues/RD-2",
        resourceStatus: "failed",
        resourceAttempt: "3",
        resourceFailed: true,
        boundaryFallback: false,
        shellSame: true,
    }, "router-dashboard resource failed state");

    await client.evaluate(`document.querySelector("[data-testid='rd-resource-retry']").click()`);
    await waitForCondition(async () => {
        const state = await appState(client);
        return state.resourceStatus === "loading" || state.resourceAttempt === "4";
    }, "resource retry starts");
    await waitForResourceStatus(client, "ready");
    assertState(await appState(client), {
        route: "details",
        hash: "#/issues/RD-2",
        detailTitle: "Billing dashboard needs clearer empty state",
        resourceStatus: "ready",
        resourceAttempt: "4",
        boundaryFallback: false,
        shellSame: true,
    }, "router-dashboard resource retry restores data");

    await client.evaluate(`document.querySelector("[data-testid='rd-edit-link']").click()`);
    await waitForRoute(client, "edit");
    assertState(await appState(client), {
        route: "edit",
        hash: "#/issues/RD-2/edit",
        formTitle: "Billing dashboard needs clearer empty state",
        formDirty: "No local changes",
        titleAriaInvalid: "false",
        resourceStatus: "ready",
        resourceAttempt: "4",
        boundaryFallback: false,
        shellSame: true,
    }, "router-dashboard edit route after resource retry");

    await setInput(client, "[data-testid='rd-field-title']", "");
    await client.evaluate(`document.querySelector("[data-testid='rd-form-submit']").click()`);
    await waitForCondition(async () => {
        return (await appState(client)).titleError === "Title is required.";
    }, "title validation error");
    assertState(await appState(client), {
        route: "edit",
        titleError: "Title is required.",
        titleAriaInvalid: "true",
        formDirty: "Unsaved local changes",
        saved: false,
        resourceStatus: "ready",
        resourceAttempt: "4",
        boundaryFallback: false,
        shellSame: true,
    }, "router-dashboard validation error");

    await setInput(client, "[data-testid='rd-field-title']", "Updated billing title");
    await client.evaluate(`document.querySelector("[data-testid='rd-form-submit']").click()`);
    await waitForCondition(async () => {
        return (await appState(client)).saved;
    }, "valid form submit");
    assertState(await appState(client), {
        route: "edit",
        titleError: "",
        titleAriaInvalid: "false",
        saved: true,
        resourceStatus: "ready",
        resourceAttempt: "4",
        boundaryFallback: false,
        shellSame: true,
    }, "router-dashboard valid submit");

    await client.evaluate(`document.querySelector("[data-testid='rd-form-reset']").click()`);
    await waitForCondition(async () => {
        const state = await appState(client);
        return state.formTitle === "Billing dashboard needs clearer empty state" &&
            state.formDirty === "No local changes" &&
            !state.saved &&
            state.titleError === "" &&
            state.titleAriaInvalid === "false";
    }, "form reset");
    assertState(await appState(client), {
        route: "edit",
        formTitle: "Billing dashboard needs clearer empty state",
        formDirty: "No local changes",
        saved: false,
        titleError: "",
        titleAriaInvalid: "false",
        resourceStatus: "ready",
        resourceAttempt: "4",
        boundaryFallback: false,
        shellSame: true,
    }, "router-dashboard form reset");

    await client.evaluate(`location.hash = "#/missing"`);
    await waitForRoute(client, "notFound");
    assertState(await appState(client), {
        route: "notFound",
        hash: "#/missing",
        notFoundPath: "/missing",
        resourceStatus: "ready",
        resourceAttempt: "4",
        boundaryFallback: false,
        shellSame: true,
    }, "router-dashboard not-found");

    const debug = await appState(client);
    if (debug.routerViewRenders < 2 || debug.routerRouteRenders < 2) {
        throw new Error(`APP FAILURE: router-dashboard debug render counters are not readable: ${JSON.stringify(debug)}`);
    }

    client.close();
    console.log("Router dashboard browser smoke: ok");
} finally {
    const exited = new Promise((resolve) => browser.once("exit", resolve));
    browser.kill("SIGTERM");
    await Promise.race([exited, wait(2000)]);
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function appState(client) {
    return await client.evaluate(`(() => {
        const home = document.querySelector("[data-testid='rd-home']");
        const issues = document.querySelector("[data-testid='rd-issue-list']");
        const details = document.querySelector("[data-testid='rd-issue-detail']");
        const edit = document.querySelector("[data-testid='rd-issue-edit']");
        const notFound = document.querySelector("[data-testid='rd-not-found']");
        const resourcePanel = document.querySelector("[data-testid='rd-resource-loading'], [data-testid='rd-resource-failed']");
        const boundary = document.querySelector("[data-testid='rd-boundary-fallback']");
        return {
            route: home ? "home" : issues ? "issues" : details ? "details" : edit ? "edit" : notFound ? "notFound" : resourcePanel ? "resource" : boundary ? "boundary" : "missing",
            hash: location.hash,
            rowCount: document.querySelectorAll("[data-testid='rd-issue-row']").length,
            queryValue: document.querySelector("[data-testid='rd-filter-query']")?.value ?? "",
            statusValue: document.querySelector("[data-testid='rd-filter-status']")?.value ?? "",
            summary: document.querySelector("[data-testid='rd-filter-summary']")?.textContent.trim() ?? "",
            detailTitle: document.querySelector("[data-testid='rd-issue-detail'] h2")?.textContent.trim() ?? "",
            formTitle: document.querySelector("[data-testid='rd-field-title']")?.value ?? "",
            formDirty: document.querySelector("[data-testid='rd-form-dirty']")?.textContent.trim() ?? "",
            titleError: document.querySelector("[data-testid='rd-field-error-title']")?.textContent.trim() ?? "",
            titleAriaInvalid: document.querySelector("[data-testid='rd-field-title']")?.getAttribute("aria-invalid") ?? "<missing>",
            saved: Boolean(document.querySelector("[data-testid='rd-form-saved']")),
            notFoundPath: document.querySelector("[data-testid='rd-not-found-path']")?.textContent.trim() ?? "",
            resourceStatus: document.querySelector("[data-testid='rd-resource-status']")?.textContent.trim() ?? "",
            resourceAttempt: document.querySelector("[data-testid='rd-resource-attempt']")?.textContent.trim().match(/\\d+/)?.[0] ?? "",
            resourceLoading: Boolean(document.querySelector("[data-testid='rd-resource-loading']")),
            resourceFailed: Boolean(document.querySelector("[data-testid='rd-resource-failed']")),
            resourceError: document.querySelector("[data-testid='rd-resource-error']")?.textContent.trim() ?? "",
            boundaryFallback: Boolean(boundary),
            boundaryResetPresent: Boolean(document.querySelector("[data-testid='rd-boundary-reset']")),
            boundaryBackPresent: Boolean(document.querySelector("[data-testid='rd-boundary-back-to-issues']")),
            shellSame: window.__routerDashboardSmokeShell === document.querySelector("[data-testid='rd-shell']"),
            routerViewRenders: window.goframeComponentRenderCounts?.RouterView ?? 0,
            routerRouteRenders: window.goframeComponentRenderCounts?.RouterRoute ?? 0,
        };
    })()`);
}

async function setInput(client, selector, value) {
    await client.evaluate(`(() => {
        const element = document.querySelector(${JSON.stringify(selector)});
        if (!element) throw new Error("missing input " + ${JSON.stringify(selector)});
        element.value = ${JSON.stringify(value)};
        element.dispatchEvent(new Event("input", { bubbles: true }));
    })()`);
}

async function setSelect(client, selector, value) {
    await client.evaluate(`(() => {
        const element = document.querySelector(${JSON.stringify(selector)});
        if (!element) throw new Error("missing select " + ${JSON.stringify(selector)});
        element.value = ${JSON.stringify(value)};
        element.dispatchEvent(new Event("change", { bubbles: true }));
    })()`);
}

async function settleAnimationFrame(client) {
    await client.evaluate(`new Promise((resolve) => {
        requestAnimationFrame(() => requestAnimationFrame(resolve));
    })`);
}

async function waitForRoute(client, route) {
    await waitForCondition(async () => {
        return (await appState(client)).route === route;
    }, `route ${route}`);
}

async function waitForResourceStatus(client, status) {
    await waitForCondition(async () => {
        return (await appState(client)).resourceStatus === status;
    }, `resource status ${status}`);
}

function assertState(actual, expected, label) {
    for (const [key, value] of Object.entries(expected)) {
        if (actual[key] !== value) {
            throw new Error(`APP FAILURE: ${label}: ${key} got ${JSON.stringify(actual[key])}, want ${JSON.stringify(value)}; state=${JSON.stringify(actual)}`);
        }
    }
    console.log(`${label}: ok`);
}

function withSmokeParam(url) {
    const next = new URL(url);
    next.searchParams.set("smoke", String(Date.now()));
    next.hash = "";
    return String(next);
}

async function waitForPage(port) {
    const endpoint = `http://127.0.0.1:${port}/json/version`;
    const started = Date.now();
    while (Date.now() - started < 10_000) {
        try {
            const version = await fetch(endpoint).then((response) => response.json());
            if (version.webSocketDebuggerUrl) {
                const pages = await fetch(`http://127.0.0.1:${port}/json`).then((response) => response.json());
                const page = pages.find((entry) => entry.type === "page" && entry.webSocketDebuggerUrl);
                if (page) {
                    return page;
                }
            }
        } catch {
            if (browser.exitCode !== null) {
                throw new Error(`HARNESS FAILURE: Chrome exited early. stderr:\n${browserError}`);
            }
        }
        await wait(100);
    }
    throw new Error(`HARNESS FAILURE: Chrome DevTools endpoint did not start. stderr:\n${browserError}`);
}

async function connect(url) {
    const socket = new WebSocket(url);
    let nextID = 1;
    const pending = new Map();

    socket.addEventListener("message", (event) => {
        const message = JSON.parse(event.data);
        if (!message.id || !pending.has(message.id)) {
            return;
        }
        const { resolve, reject } = pending.get(message.id);
        pending.delete(message.id);
        if (message.error) {
            reject(new Error(`${message.error.message}: ${message.error.data ?? ""}`));
        } else {
            resolve(message.result ?? {});
        }
    });

    await new Promise((resolve, reject) => {
        socket.addEventListener("open", resolve, { once: true });
        socket.addEventListener("error", reject, { once: true });
    });

    return {
        call(method, params = {}) {
            const id = nextID++;
            socket.send(JSON.stringify({ id, method, params }));
            return new Promise((resolve, reject) => {
                pending.set(id, { resolve, reject });
            });
        },
        async evaluate(expression) {
            const result = await this.call("Runtime.evaluate", {
                expression,
                awaitPromise: true,
                returnByValue: true,
            });
            if (result.exceptionDetails) {
                throw new Error(`APP FAILURE: evaluation failed: ${JSON.stringify(result.exceptionDetails)}`);
            }
            return result.result.value;
        },
        close() {
            socket.close();
        },
    };
}

async function navigateToApp(client, url) {
    await client.call("Page.navigate", { url });
    await waitForCondition(async () => {
        const readyState = await client.evaluate("document.readyState");
        return readyState === "complete" || readyState === "interactive";
    }, "page navigation");
}

async function waitForAppPage(client, expectedApp) {
    await waitForCondition(async () => {
        const state = await client.evaluate(`(() => ({
            href: location.href,
            origin: location.origin,
            rootReady: Boolean(document.querySelector("#root")),
            appReady: Boolean(document.querySelector("[data-testid='rd-shell']")),
        }))()`);
        return state.origin === expectedApp.origin && state.rootReady && state.appReady;
    }, "router-dashboard app page");
}

async function waitForCondition(predicate, label) {
    const started = Date.now();
    let lastError;
    while (Date.now() - started < 10_000) {
        try {
            if (await predicate()) {
                return;
            }
        } catch (error) {
            lastError = error;
        }
        await wait(100);
    }
    throw new Error(`APP FAILURE: timed out waiting for ${label}${lastError ? `: ${lastError.message}` : ""}`);
}

function wait(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

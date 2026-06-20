import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const appURL = process.argv[2] ?? process.env.GOFRAME_ROUTER_SMOKE_URL ?? "http://127.0.0.1:18080/";
const debugPort = Number(process.env.GOFRAME_ROUTER_CHROME_DEBUG_PORT ?? "19249");
const chrome = process.env.CHROME ?? "google-chrome";
const profile = await mkdtemp(join(tmpdir(), "goframe-router-smoke-"));
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
    await client.evaluate(`window.__routerSmokeShell = document.querySelector("[data-testid='router-shell']")`);

    assertRoute(await appState(client), {
        route: "home",
        hash: "",
        shellSame: true,
    }, "router initial home route");

    await client.evaluate(`document.querySelector("[data-testid='router-link-issues']").click()`);
    await waitForRoute(client, "issues");
    assertRoute(await appState(client), {
        route: "issues",
        hash: "#/issues",
        issueCount: 3,
        shellSame: true,
    }, "router issues link route");

    await client.evaluate(`document.querySelector("[data-testid='router-link-first-issue']").click()`);
    await waitForRoute(client, "details");
    assertRoute(await appState(client), {
        route: "details",
        hash: "#/issues/1",
        issueID: "1",
        shellSame: true,
    }, "router param route");

    await client.evaluate(`document.querySelector("[data-testid='router-programmatic-nav']").click()`);
    await waitForIssueID(client, "2");
    assertRoute(await appState(client), {
        route: "details",
        hash: "#/issues/2",
        issueID: "2",
        shellSame: true,
    }, "router programmatic navigate");

    await client.evaluate(`history.back()`);
    await waitForIssueID(client, "1");
    assertRoute(await appState(client), {
        route: "details",
        hash: "#/issues/1",
        issueID: "1",
        shellSame: true,
    }, "router browser back");

    await client.evaluate(`location.hash = "#/missing"`);
    await waitForRoute(client, "notFound");
    assertRoute(await appState(client), {
        route: "notFound",
        hash: "#/missing",
        notFoundPath: "/missing",
        shellSame: true,
    }, "router not-found route");

    const debug = await appState(client);
    if (debug.routerViewRenders < 2 || debug.routerRouteRenders < 2) {
        throw new Error(`APP FAILURE: router debug render counters are not readable: ${JSON.stringify(debug)}`);
    }

    client.close();
    console.log("Router browser smoke: ok");
} finally {
    const exited = new Promise((resolve) => browser.once("exit", resolve));
    browser.kill("SIGTERM");
    await Promise.race([exited, wait(2000)]);
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function appState(client) {
    return await client.evaluate(`(() => {
        const home = document.querySelector("[data-testid='router-home']");
        const issues = document.querySelector("[data-testid='router-issues']");
        const details = document.querySelector("[data-testid='router-issue-details']");
        const notFound = document.querySelector("[data-testid='router-not-found']");
        return {
            route: home ? "home" : issues ? "issues" : details ? "details" : notFound ? "notFound" : "missing",
            href: location.href,
            hash: location.hash,
            issueID: document.querySelector("[data-testid='router-issue-id']")?.textContent.trim() ?? "",
            issueCount: document.querySelectorAll("[data-testid='router-issue-item']").length,
            notFoundPath: document.querySelector("[data-testid='router-not-found-path']")?.textContent.trim() ?? "",
            shellSame: window.__routerSmokeShell === document.querySelector("[data-testid='router-shell']"),
            shellText: document.querySelector("[data-testid='router-shell']")?.textContent.trim() ?? "",
            routerViewRenders: window.goframeComponentRenderCounts?.RouterView ?? 0,
            routerRouteRenders: window.goframeComponentRenderCounts?.RouterRoute ?? 0,
        };
    })()`);
}

async function waitForRoute(client, route) {
    await waitForCondition(async () => {
        return (await appState(client)).route === route;
    }, `route ${route}`);
}

async function waitForIssueID(client, id) {
    await waitForCondition(async () => {
        const state = await appState(client);
        return state.route === "details" && state.issueID === id;
    }, `issue ${id}`);
}

function assertRoute(actual, expected, label) {
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
            ready: Boolean(document.querySelector("[data-testid='router-shell']") && document.querySelector("[data-testid='router-home']")),
        }))()`);
        const actual = new URL(state.href);
        if (actual.origin !== expectedApp.origin) {
            throw new Error(`HARNESS FAILURE: expected app origin ${expectedApp.origin}, got ${actual.origin}`);
        }
        return state.ready;
    }, "router app ready");
}

async function waitForCondition(check, label) {
    const started = Date.now();
    while (Date.now() - started < 10_000) {
        if (await check()) {
            return;
        }
        await wait(50);
    }
    throw new Error(`APP FAILURE: timed out waiting for ${label}`);
}

function wait(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

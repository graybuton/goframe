import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const appURL = process.argv[2] ?? process.env.GOFRAME_RESOURCE_SMOKE_URL ?? "http://127.0.0.1:18080/";
const debugPort = Number(process.env.GOFRAME_RESOURCE_CHROME_DEBUG_PORT ?? "19260");
const chrome = process.env.CHROME ?? "google-chrome";
const profile = await mkdtemp(join(tmpdir(), "goframe-resource-smoke-"));
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
    assertState(await resourceState(client), {
        panel: true,
        status: "loading",
        key: "slow:assets/data/issues-open.txt",
        boundaryFallback: false,
    }, "resource initial loading");

    await waitForStatus(client, "ready");
    assertState(await resourceState(client), {
        status: "ready",
        itemCount: 2,
        firstItem: "GF-101Audit render error fallback bubblingopen",
        boundaryFallback: false,
    }, "resource initial ready");

    await client.evaluate(`document.querySelector("[data-testid='resource-reload']").click()`);
    await waitForStatus(client, "loading");
    assertState(await resourceState(client), {
        status: "loading",
        attempt: "2",
        boundaryFallback: false,
    }, "resource reload loading");
    await waitForStatus(client, "ready");
    assertState(await resourceState(client), {
        status: "ready",
        itemCount: 2,
        attempt: "2",
        boundaryFallback: false,
    }, "resource reload ready");

    await client.evaluate(`document.querySelector("[data-testid='resource-load-missing']").click()`);
    await waitForStatus(client, "failed");
    const failed = await resourceState(client);
    if (!failed.errorText.includes("fetch")) {
        throw new Error(`APP FAILURE: missing asset error did not mention fetch: ${JSON.stringify(failed)}`);
    }
    assertState(failed, {
        status: "failed",
        boundaryFallback: false,
    }, "resource missing failed state");

    await client.evaluate(`document.querySelector("[data-testid='resource-load-open']").click()`);
    await waitForStatus(client, "ready");
    assertState(await resourceState(client), {
        status: "ready",
        itemCount: 2,
        key: "assets/data/issues-open.txt",
        boundaryFallback: false,
    }, "resource recovers after failed state");

    await client.evaluate(`document.querySelector("[data-testid='resource-load-slow']").click()`);
    await waitForStatus(client, "loading");
    await client.evaluate(`document.querySelector("[data-testid='resource-load-fast']").click()`);
    await waitForCondition(async () => {
        const state = await resourceState(client);
        return state.status === "ready" && state.key === "assets/data/issues-all.txt" && state.itemCount === 4;
    }, "fast all wins over slow open");
    await wait(500);
    assertState(await resourceState(client), {
        status: "ready",
        key: "assets/data/issues-all.txt",
        itemCount: 4,
        lastItem: "GF-301Ship public preview polish notesclosed",
        boundaryFallback: false,
    }, "resource stale slow result ignored");

    await client.evaluate(`document.querySelector("[data-testid='resource-load-slow']").click()`);
    await waitForStatus(client, "loading");
    await client.evaluate(`document.querySelector("[data-testid='resource-toggle-panel']").click()`);
    await waitForCondition(async () => {
        return (await resourceState(client)).unmounted === true;
    }, "resource panel unmounted");
    await wait(500);
    assertState(await resourceState(client), {
        panel: false,
        unmounted: true,
        boundaryFallback: false,
    }, "resource late completion after unmount ignored");

    await client.evaluate(`document.querySelector("[data-testid='resource-toggle-panel']").click()`);
    await waitForStatus(client, "loading");
    await waitForStatus(client, "ready");
    assertState(await resourceState(client), {
        panel: true,
        status: "ready",
        itemCount: 2,
        boundaryFallback: false,
    }, "resource app remains interactive");

    client.close();
    console.log("Resource browser smoke: ok");
} finally {
    const exited = new Promise((resolve) => browser.once("exit", resolve));
    browser.kill("SIGTERM");
    await Promise.race([exited, wait(2000)]);
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function resourceState(client) {
    return await client.evaluate(`(() => {
        const itemTexts = [...document.querySelectorAll("[data-testid='resource-item']")]
            .map((item) => item.textContent.trim().replace(/\\s+/g, " "));
        return {
            app: Boolean(document.querySelector("[data-testid='resource-app']")),
            panel: Boolean(document.querySelector("[data-testid='resource-panel']")),
            unmounted: Boolean(document.querySelector("[data-testid='resource-panel-unmounted']")),
            boundaryFallback: Boolean(document.querySelector("[data-testid='resource-boundary-fallback']")),
            key: document.querySelector("[data-testid='resource-key']")?.textContent.trim() ?? "",
            attempt: document.querySelector("[data-testid='resource-attempt']")?.textContent.trim() ?? "",
            status: document.querySelector("[data-testid='resource-status']")?.textContent.trim() ?? "",
            loading: Boolean(document.querySelector("[data-testid='resource-loading']")),
            ready: Boolean(document.querySelector("[data-testid='resource-ready']")),
            failed: Boolean(document.querySelector("[data-testid='resource-failed']")),
            errorText: document.querySelector("[data-testid='resource-error']")?.textContent.trim() ?? "",
            itemCount: itemTexts.length,
            firstItem: itemTexts[0] ?? "",
            lastItem: itemTexts[itemTexts.length - 1] ?? "",
            errors: window.goframeResourceErrors ?? [],
        };
    })()`);
}

async function waitForStatus(client, status) {
    await waitForCondition(async () => {
        return (await resourceState(client)).status === status;
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
    let lastState = null;
    try {
        await waitForCondition(async () => {
            const state = await client.evaluate(`(() => ({
            href: location.href,
            rootText: document.querySelector("#root")?.textContent.trim() ?? "",
            ready: Boolean(document.querySelector("[data-testid='resource-app']")),
            errors: window.goframeResourceErrors ?? [],
        }))()`);
            lastState = state;
            const actual = new URL(state.href);
            if (actual.origin !== expectedApp.origin) {
                throw new Error(`HARNESS FAILURE: expected app origin ${expectedApp.origin}, got ${actual.origin}`);
            }
            return state.ready;
        }, "resource app ready");
    } catch (error) {
        throw new Error(`${error.message}; state=${JSON.stringify(lastState)}`);
    }
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

import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const appURL = process.argv[2] ?? process.env.GOFRAME_CMDAPP_SMOKE_URL ?? "http://127.0.0.1:18080/";
const debugPort = Number(process.env.GOFRAME_CMDAPP_CHROME_DEBUG_PORT ?? "19239");
const chrome = process.env.CHROME ?? "google-chrome";
const profile = await mkdtemp(join(tmpdir(), "goframe-cmdapp-smoke-"));
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

    assertDeepEqual(
        await appState(client),
        {
            entryHeader: "Entry HeaderChild entry package calling internal GOX packages",
            uiHeader: "UI HeaderChild entry GOX",
            increment: "Increment0",
            taskCount: 3,
            taskSummary: "3 child-entry tasks",
            headerRenders: 2,
            layoutRenders: 0,
            taskListRenders: 0,
            taskRowRenders: 3,
        },
        "cmdapp initial render",
    );

    await client.evaluate(`document.querySelector("[data-testid='cmdapp-increment']").click()`);
    await wait(140);
    const afterClick = await appState(client);
    if (afterClick.increment !== "Increment1") {
        throw new Error(`APP FAILURE: cmdapp increment did not update: ${JSON.stringify(afterClick)}`);
    }
    if (afterClick.taskCount !== 3 || afterClick.taskSummary !== "3 child-entry tasks") {
        throw new Error(`APP FAILURE: cmdapp task list changed unexpectedly: ${JSON.stringify(afterClick)}`);
    }
    if (afterClick.headerRenders < 4 || afterClick.taskRowRenders < 3) {
        throw new Error(`APP FAILURE: cmdapp debug render counters are not readable: ${JSON.stringify(afterClick)}`);
    }
    console.log("cmdapp interaction: ok");

    client.close();
    console.log("Cmdapp browser smoke: ok");
} finally {
    const exited = new Promise((resolve) => browser.once("exit", resolve));
    browser.kill("SIGTERM");
    await Promise.race([exited, wait(2000)]);
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function appState(client) {
    return await client.evaluate(`(() => ({
        entryHeader: document.querySelector("[data-testid='entry-header']")?.textContent.trim(),
        uiHeader: document.querySelector("[data-testid='cmdapp-ui-header']")?.textContent.trim(),
        increment: document.querySelector("[data-testid='cmdapp-increment']")?.textContent.trim(),
        taskCount: document.querySelectorAll("[data-testid='cmdapp-task-row']").length,
        taskSummary: document.querySelector("[data-testid='cmdapp-task-list'] h2")?.textContent.trim(),
        headerRenders: window.goframeComponentRenderCounts?.Header ?? 0,
        layoutRenders: window.goframeComponentRenderCounts?.Layout ?? 0,
        taskListRenders: window.goframeComponentRenderCounts?.TaskList ?? 0,
        taskRowRenders: window.goframeComponentRenderCounts?.TaskRow ?? 0,
    }))()`);
}

function withSmokeParam(url) {
    const next = new URL(url);
    next.searchParams.set("smoke", String(Date.now()));
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
            ready: Boolean(document.querySelector("[data-testid='entry-header']") && document.querySelector("[data-testid='cmdapp-task-list']")),
        }))()`);
        const actual = new URL(state.href);
        if (actual.origin !== expectedApp.origin) {
            throw new Error(`HARNESS FAILURE: expected app origin ${expectedApp.origin}, got ${actual.origin}`);
        }
        return state.ready;
    }, "cmdapp app ready");
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

function assertDeepEqual(actual, expected, label) {
    const actualJSON = JSON.stringify(actual);
    const expectedJSON = JSON.stringify(expected);
    if (actualJSON !== expectedJSON) {
        throw new Error(`APP FAILURE: ${label}: got ${actualJSON}, want ${expectedJSON}`);
    }
    console.log(`${label}: ok`);
}

function wait(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

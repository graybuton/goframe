import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const appURL = process.argv[2] ?? "http://127.0.0.1:18081/";
const debugPort = Number(process.env.GOFRAME_DUPLICATE_KEY_CHROME_DEBUG_PORT ?? "19223");
const chrome = process.env.CHROME ?? "google-chrome";
const profile = await mkdtemp(join(tmpdir(), "goframe-duplicate-key-smoke-"));
const browser = spawn(chrome, [
    "--headless",
    "--no-sandbox",
    "--disable-gpu",
    `--remote-debugging-port=${debugPort}`,
    `--user-data-dir=${profile}`,
    appURL,
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
    await waitForWarning(client);
    const result = await client.evaluate(`(() => ({
        mounted: Boolean(document.querySelector("#duplicate-key-fixture")),
        warnings: globalThis.goframeDuplicateKeyWarnings || [],
    }))()`);
    if (!result.mounted) {
        throw new Error("duplicate key fixture did not mount");
    }
    if (!result.warnings.some((warning) => warning.includes('duplicate key "same"'))) {
        throw new Error(`duplicate key warning missing: ${JSON.stringify(result.warnings)}`);
    }
    client.close();
    console.log("Duplicate key browser smoke: ok");
} finally {
    const exited = new Promise((resolve) => browser.once("exit", resolve));
    browser.kill("SIGTERM");
    await Promise.race([exited, wait(2000)]);
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function waitForWarning(client) {
    const started = Date.now();
    while (Date.now() - started < 5000) {
        const result = await client.evaluate(`(() => {
            const warnings = globalThis.goframeDuplicateKeyWarnings || [];
            return warnings.some((warning) => warning.includes('duplicate key "same"'));
        })()`);
        if (result === true) {
            return;
        }
        await wait(100);
    }
    throw new Error("timed out waiting for duplicate key warning");
}

async function waitForPage(port) {
    const started = Date.now();
    let lastError;
    while (Date.now() - started < 5000) {
        try {
            const response = await fetch(`http://127.0.0.1:${port}/json`);
            const pages = await response.json();
            const page = pages.find((entry) => entry.type === "page" && entry.webSocketDebuggerUrl);
            if (page) {
                return page;
            }
        } catch (error) {
            lastError = error;
        }
        await wait(100);
    }
    throw new Error(`Chrome DevTools page unavailable: ${lastError?.message ?? browserError}`);
}

function connect(url) {
    const socket = new WebSocket(url);
    let nextID = 1;
    const pending = new Map();

    return new Promise((resolve, reject) => {
        socket.addEventListener("open", () => {
            resolve({
                call(method, params = {}) {
                    const id = nextID++;
                    socket.send(JSON.stringify({ id, method, params }));
                    return new Promise((callResolve, callReject) => {
                        pending.set(id, { resolve: callResolve, reject: callReject });
                    });
                },
                async evaluate(expression) {
                    const response = await this.call("Runtime.evaluate", {
                        expression,
                        awaitPromise: true,
                        returnByValue: true,
                    });
                    if (response.exceptionDetails) {
                        throw new Error(response.exceptionDetails.text);
                    }
                    return response.result.result.value;
                },
                close() {
                    socket.close();
                },
            });
        }, { once: true });
        socket.addEventListener("error", reject, { once: true });
        socket.addEventListener("message", (event) => {
            const message = JSON.parse(event.data);
            if (!message.id || !pending.has(message.id)) {
                return;
            }
            const request = pending.get(message.id);
            pending.delete(message.id);
            if (message.error) {
                request.reject(new Error(message.error.message));
                return;
            }
            request.resolve(message);
        });
    });
}

function wait(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

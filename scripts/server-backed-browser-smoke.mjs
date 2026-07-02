import { spawn } from "node:child_process";
import { mkdtemp, rm, stat } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import net from "node:net";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const rootDir = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const appDir = join(rootDir, "examples", "server-backed");
const packageDir = join(appDir, ".goframe", "package", "standalone");
const chrome = process.env.CHROME ?? "google-chrome";
const debugPort = Number(process.env.GOFRAME_SERVER_BACKED_CHROME_DEBUG_PORT ?? await pickFreePort());
const backendPort = Number(process.env.GOFRAME_SERVER_BACKED_SMOKE_PORT ?? await pickFreePort());
const profile = await mkdtempCompat("goframe-server-backed-smoke-");
const appURL = `http://127.0.0.1:${backendPort}/?smoke=${Date.now()}`;
const expectedApp = new URL(appURL);
const initialMessage = "Hello, GoFrame, from Go backend!";
const updatedName = "Ada";
const updatedMessage = "Hello, Ada, from Go backend!";
const failureName = "fail";
const failureStatus = "failed";
const failureMessage = "backend returned HTTP 500";

let backend = null;
let browser = null;
let backendError = "";
let browserError = "";
let browserExit = null;

try {
    await runCommand("go", ["run", "./cmd/goxc", "package", "./examples/server-backed", "--compiler=go"], { cwd: rootDir });

    const packageInfo = await stat(packageDir);
    if (!packageInfo.isDirectory()) {
        throw new Error(`HARNESS FAILURE: package output is not a directory: ${packageDir}`);
    }

    backend = spawn("go", [
        "run",
        "./examples/server-backed/cmd/server",
        `--package=${packageDir}`,
        `--addr=127.0.0.1:${backendPort}`,
    ], {
        cwd: rootDir,
        detached: true,
        stdio: ["ignore", "ignore", "pipe"],
    });
    backend.stderr.on("data", (chunk) => {
        backendError += chunk;
    });

    await waitForHTTP(`http://127.0.0.1:${backendPort}/`, () => backend.exitCode !== null, "server-backed backend");
    await assertBackendAPI("GoFrame", initialMessage);
    await assertBackendAPI(updatedName, updatedMessage);
    await assertBackendAPIFailure(failureName, 500);

    browser = spawn(chrome, [
        "--headless",
        "--no-sandbox",
        "--disable-gpu",
        `--remote-debugging-port=${debugPort}`,
        `--user-data-dir=${profile}`,
        "about:blank",
    ], {
        stdio: ["ignore", "ignore", "pipe"],
    });
    browser.stderr.on("data", (chunk) => {
        browserError += chunk;
    });
    browser.on("exit", (code, signal) => {
        browserExit = { code, signal };
    });

    const page = await waitForPage(debugPort);
    const client = await connect(page.webSocketDebuggerUrl);
    await client.call("Runtime.enable");
    await client.call("Page.enable");
    await navigateToApp(client, appURL);
    await waitForAppPage(client, expectedApp);
    await waitForGreeting(client, initialMessage, "initial backend greeting");
    assertState(await appState(client, initialMessage), {
        app: true,
        ready: true,
        failed: false,
        status: "ready",
        message: initialMessage,
        messageMatches: true,
        origin: expectedApp.origin,
        input: "GoFrame",
    }, "server-backed initial render");

    await setGreetingName(client, updatedName);
    await waitForCondition(async () => {
        return (await appState(client, initialMessage)).input === updatedName;
    }, "controlled input update");
    await wait(100);
    await submitGreeting(client);
    await waitForGreeting(client, updatedMessage, "updated backend greeting");
    assertState(await appState(client, updatedMessage), {
        app: true,
        ready: true,
        failed: false,
        status: "ready",
        message: updatedMessage,
        messageMatches: true,
        origin: expectedApp.origin,
        input: updatedName,
    }, "server-backed form submit render");

    await setGreetingName(client, failureName);
    await waitForCondition(async () => {
        return (await appState(client, updatedMessage)).input === failureName;
    }, "controlled failure input update");
    await wait(100);
    await submitGreeting(client);
    await waitForFailure(client, failureMessage, "controlled backend failure");
    assertState(await appState(client, updatedMessage), {
        app: true,
        ready: false,
        failed: true,
        status: failureStatus,
        input: failureName,
        error: failureMessage,
        errorNonEmpty: true,
    }, "server-backed controlled failure render");

    await setGreetingName(client, updatedName);
    await waitForCondition(async () => {
        return (await appState(client, updatedMessage)).input === updatedName;
    }, "recovery input update");
    await wait(100);
    await submitGreeting(client);
    await waitForGreeting(client, updatedMessage, "recovered backend greeting");
    assertState(await appState(client, updatedMessage), {
        app: true,
        ready: true,
        failed: false,
        status: "ready",
        message: updatedMessage,
        messageMatches: true,
        origin: expectedApp.origin,
        input: updatedName,
        error: "",
        errorNonEmpty: false,
    }, "server-backed recovery render");

    client.close();
    console.log("Server-backed browser smoke: ok");
} finally {
    await stopProcess(browser, { processGroup: false });
    await stopProcess(backend, { processGroup: true });
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function assertBackendAPI(name, expected) {
    const url = new URL(`http://127.0.0.1:${backendPort}/api/greeting`);
    url.searchParams.set("name", name);
    const response = await fetch(url);
    if (!response.ok) {
        throw new Error(`HARNESS FAILURE: backend API returned HTTP ${response.status}`);
    }
    const text = await response.text();
    if (text !== expected) {
        throw new Error(`HARNESS FAILURE: backend API returned ${JSON.stringify(text)}, want ${JSON.stringify(expected)}`);
    }
}

async function assertBackendAPIFailure(name, status) {
    const url = new URL(`http://127.0.0.1:${backendPort}/api/greeting`);
    url.searchParams.set("name", name);
    const response = await fetch(url);
    if (response.status !== status) {
        throw new Error(`HARNESS FAILURE: backend API returned HTTP ${response.status}, want ${status}`);
    }
    const text = await response.text();
    if (!text.trim()) {
        throw new Error("HARNESS FAILURE: backend API failure response body was empty");
    }
}

async function setGreetingName(client, name) {
    return await client.callFunction(`function(name) {
        const input = document.querySelector("[data-testid='greeting-name']");
        if (!input) {
            return { ok: false, reason: "missing input" };
        }
        input.value = name;
        input.dispatchEvent(new Event("input", { bubbles: true }));
        return { ok: true, value: input.value };
    }`, name);
}

async function submitGreeting(client) {
    return await client.callFunction(`function() {
        const form = document.querySelector("[data-testid='greeting-form']");
        if (!form) {
            return { ok: false, reason: "missing form" };
        }
        if (typeof form.requestSubmit === "function") {
            form.requestSubmit();
        } else {
            form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
        }
        return { ok: true };
    }`);
}

async function waitForGreeting(client, expected, label) {
    await waitForCondition(async () => {
        const state = await appState(client, expected);
        return state.ready && state.messageMatches && state.origin === expectedApp.origin;
    }, label);
}

async function waitForFailure(client, expectedError, label) {
    await waitForCondition(async () => {
        const state = await appState(client, updatedMessage);
        return state.failed && state.status === failureStatus && state.error === expectedError;
    }, label);
}

async function appState(client, expected) {
    return await client.callFunction(`function(expected) {
        const message = document.querySelector("[data-testid='greeting-message']")?.textContent.trim() ?? "";
        const error = document.querySelector("[data-testid='greeting-error']")?.textContent.trim() ?? "";
        return {
            app: Boolean(document.querySelector("[data-testid='server-backed-app']")),
            loading: Boolean(document.querySelector("[data-testid='greeting-loading']")),
            ready: Boolean(document.querySelector("[data-testid='greeting-message']")),
            failed: Boolean(document.querySelector("[data-testid='greeting-error']")),
            status: document.querySelector("[data-testid='greeting-status']")?.textContent.trim() ?? "",
            input: document.querySelector("[data-testid='greeting-name']")?.value ?? "",
            key: document.querySelector("[data-testid='greeting-resource-key']")?.textContent.trim() ?? "",
            message,
            messageMatches: message === expected,
            error,
            errorNonEmpty: error.length > 0,
            origin: window.location.origin,
        };
    }`, expected);
}

function assertState(actual, expected, label) {
    for (const [key, value] of Object.entries(expected)) {
        if (actual[key] !== value) {
            throw new Error(`APP FAILURE: ${label}: ${key} got ${JSON.stringify(actual[key])}, want ${JSON.stringify(value)}; state=${JSON.stringify(actual)}`);
        }
    }
    console.log(`${label}: ok`);
}

async function waitForPage(port) {
    let lastError;
    for (let attempt = 0; attempt < 80; attempt++) {
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

async function waitForAppPage(client, expected) {
    let lastState = null;
    for (let attempt = 0; attempt < 120; attempt++) {
        lastState = await pageState(client);
        if (lastState.href.startsWith("chrome-error://")) {
            throw await harnessFailure(client, "Chrome loaded an error document", lastState);
        }
        if (isExpectedAppState(lastState, expected) && lastState.root && lastState.app) {
            return lastState;
        }
        await wait(100);
    }
    throw await harnessFailure(client, "app page did not become ready", lastState);
}

async function pageState(client) {
    return await client.evaluate(`(() => ({
        href: window.location.href,
        origin: window.location.origin,
        protocol: window.location.protocol,
        readyState: document.readyState,
        root: Boolean(document.querySelector("#root")),
        app: Boolean(document.querySelector("[data-testid='server-backed-app']")),
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
    return new Error(`HARNESS FAILURE: ${message}\n${JSON.stringify({ appURL, debugPort, backendPort, detail, diagnostics }, null, 2)}`);
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
    if (backend?.exitCode !== null) {
        diagnostics.backendExitCode = backend.exitCode;
    }
    if (backendError) {
        diagnostics.backendStderr = backendError.slice(-4000);
    }
    if (browserExit) {
        diagnostics.browserExit = browserExit;
    }
    if (browserError) {
        diagnostics.browserStderr = browserError.slice(-4000);
    }
    return diagnostics;
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

async function waitForCondition(check, label) {
    let lastError = null;
    for (let attempt = 0; attempt < 120; attempt++) {
        try {
            if (await check()) {
                return;
            }
        } catch (error) {
            lastError = error;
        }
        await wait(100);
    }
    throw new Error(`HARNESS FAILURE: timed out waiting for ${label}${lastError ? `: ${lastError.message}` : ""}`);
}

async function waitForHTTP(url, exited, label) {
    let lastError;
    for (let attempt = 0; attempt < 120; attempt++) {
        if (exited()) {
            throw new Error(`HARNESS FAILURE: ${label} exited before HTTP was available\n${backendError}`);
        }
        try {
            const response = await fetch(url);
            if (response.ok) {
                return;
            }
            lastError = new Error(`HTTP ${response.status}`);
        } catch (error) {
            lastError = error;
        }
        await wait(100);
    }
    throw new Error(`HARNESS FAILURE: ${label} HTTP endpoint did not become ready: ${lastError?.message ?? ""}\n${backendError}`);
}

function runCommand(command, args, options = {}) {
    return new Promise((resolve, reject) => {
        const child = spawn(command, args, {
            cwd: options.cwd ?? rootDir,
            stdio: "inherit",
            env: {
                ...process.env,
                GOWORK: "off",
            },
        });
        child.on("error", reject);
        child.on("exit", (code, signal) => {
            if (code === 0) {
                resolve();
                return;
            }
            reject(new Error(`${command} ${args.join(" ")} failed with ${signal ?? code}`));
        });
    });
}

async function stopProcess(child, options) {
    if (!child || child.exitCode !== null) {
        return;
    }
    const exited = new Promise((resolve) => child.once("exit", resolve));
    terminate(child, options.processGroup, "SIGTERM");
    const stopped = await Promise.race([
        exited.then(() => true),
        wait(2000).then(() => false),
    ]);
    if (!stopped && child.exitCode === null) {
        terminate(child, options.processGroup, "SIGKILL");
        await Promise.race([exited, wait(1000)]);
    }
}

function terminate(child, processGroup, signal) {
    try {
        if (processGroup) {
            process.kill(-child.pid, signal);
            return;
        }
    } catch {
        // Fall back to direct process termination below.
    }
    try {
        child.kill(signal);
    } catch {
        // The process may have exited between checks.
    }
}

function wait(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

function mkdtempCompat(prefix) {
    return mkdtemp(join(tmpdir(), prefix));
}

function pickFreePort() {
    return new Promise((resolve, reject) => {
        const server = net.createServer();
        server.on("error", reject);
        server.listen(0, "127.0.0.1", () => {
            const address = server.address();
            server.close(() => resolve(address.port));
        });
    });
}

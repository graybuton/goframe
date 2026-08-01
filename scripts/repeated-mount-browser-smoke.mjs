import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const appURL = process.argv[2] ?? process.env.GOFRAME_REPEATED_MOUNT_SMOKE_URL ?? "http://127.0.0.1:18080/";
const compiler = process.argv[3] ?? "unknown";
const debugPort = Number(process.env.GOFRAME_REPEATED_MOUNT_CHROME_DEBUG_PORT ?? "19273");
const chrome = process.env.CHROME ?? "google-chrome";
const profile = await mkdtemp(join(tmpdir(), "goframe-repeated-mount-smoke-"));
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

const browserSpawn = new Promise((resolve, reject) => {
    browser.once("spawn", resolve);
    browser.once("error", reject);
});
let browserError = "";
let browserExit = null;
browser.stderr.on("data", (chunk) => {
    browserError += chunk;
});
browser.on("exit", (code, signal) => {
    browserExit = { code, signal };
});

let client;
try {
    try {
        await browserSpawn;
    } catch (error) {
        throw new Error(`HARNESS FAILURE: Chrome failed to spawn (${compiler}): ${error.message}`);
    }
    const page = await waitForPage(debugPort);
    client = await connect(page.webSocketDebuggerUrl);
    await client.call("Runtime.enable");
    await client.call("Page.enable");
    await client.call("Page.navigate", { url: withSmokeParam(appURL, compiler) });
    await waitForAppPage(client, expectedApp);

    let state = await repeatedMountState(client);
    assertSubset(state, {
        activeApplication: "A:1",
        appARenders: 1,
        appAHandlers: 0,
        appAEffectSetups: [1],
        appAEffectCleanups: [0],
        appAUnmounts: [0],
        appAEffectVersions: ["1:0"],
        runtimeErrorCount: 0,
    }, "initial App A evidence");
    assertRoot(state.rootA, { appA: 1, appB: 0, appC: 0, goframeComments: 2 }, "initial root A");
    assertRoot(state.rootB, { appA: 0, appB: 0, appC: 0, goframeComments: 0 }, "initial root B");

    await client.evaluate(`(() => {
        window.__repeatedMountRootA = document.querySelector("#root-a");
        window.__repeatedMountRootB = document.querySelector("#root-b");
        window.__repeatedMountFirstAMarker = document.querySelector("[data-testid='app-a']");
        window.__repeatedMountOldAButton = document.querySelector("[data-testid='app-a-event']");
        const sentinel = document.createElement("span");
        sentinel.id = "host-owned-sentinel";
        sentinel.textContent = "host-owned";
        window.__repeatedMountRootA.appendChild(sentinel);
        return true;
    })()`);

    await click(client, "[data-testid='app-a-event']");
    state = await waitForState(client, "initial App A interaction", (next) =>
        next.appAHandlers === 1 && next.appAValue === 1);
    assertSubset(state, {
        appARenders: 2,
        appAHandlers: 1,
        appAEffectVersions: ["1:0", "1:1"],
        hostSentinelInRootA: true,
    }, "initial App A interaction");

    await runControl(client, "goframeRepeatedMountMountB");
    state = await waitForState(client, "different-root App B mount", (next) =>
        next.activeApplication === "B:1" && next.appBValue === 1);
    console.log(`different-root replacement before old-range assertion (${compiler}): ${JSON.stringify(state)}`);
    assertSubset(state, {
        activeApplication: "B:1",
        appAEffectCleanups: [1],
        appAUnmounts: [1],
        appAEffectCleanupSawDOM: [1],
        appAUnmountSawDOM: [1],
        appBRenders: 2,
        appBScheduledUpdates: 1,
        appBCleanups: [0],
        hostSentinelInRootA: true,
        runtimeErrorCount: 0,
    }, "different-root lifecycle release");
    assertRoot(state.rootB, { appA: 0, appB: 1, appC: 0, goframeComments: 2 }, "different-root active root B");

    const oldAHandlers = state.appAHandlers;
    await client.evaluate(`(() => {
        window.__repeatedMountOldAButton.click();
        return true;
    })()`);
    await waitForAnimationFrames(client, 2);
    state = await repeatedMountState(client);
    assertSubset(state, {
        appAHandlers: oldAHandlers,
        appBValue: 1,
        runtimeErrorCount: 0,
    }, "released App A listener");

    const appBRendersBeforeStaleSetter = state.appBRenders;
    await runControl(client, "goframeRepeatedMountInvokeStaleASetter");
    await waitForAnimationFrames(client, 2);
    state = await repeatedMountState(client);
    assertSubset(state, {
        appARenders: 2,
        appBRenders: appBRendersBeforeStaleSetter,
        appBValue: 1,
        runtimeErrorCount: 0,
    }, "stale App A setter isolation");

    if (state.rootA.appA !== 0) {
        throw new Error(
            `APP FAILURE: root A still contains the App A mounted range after ` +
            `Mount transfers ownership to root B (${compiler}); state=${JSON.stringify(state)}`,
        );
    }
    assertRoot(state.rootA, { appA: 0, appB: 0, appC: 0, goframeComments: 0 }, "different-root old root A");
    assertSubset(state, {
        oldAButtonConnected: false,
        hostSentinelInRootA: true,
    }, "different-root physical removal");

    await click(client, "[data-testid='app-b-event']");
    state = await waitForState(client, "first App B interaction", (next) =>
        next.appBHandlers === 1 && next.appBValue === 2);

    await runControl(client, "goframeRepeatedMountMountFreshA");
    state = await waitForState(client, "fresh App A mount", (next) =>
        next.activeApplication === "A:2" && next.rootA.appA === 1);
    assertSubset(state, {
        appARenders: 3,
        appAEffectSetups: [1, 1],
        appAEffectVersions: ["1:0", "1:1", "2:0"],
        appBCleanups: [1],
        hostSentinelInRootA: false,
        firstAMarkerIsCurrent: false,
    }, "fresh App A ownership");
    assertRoot(state.rootB, { appA: 0, appB: 0, appC: 0, goframeComments: 0 }, "root B after transfer to fresh App A");

    await runControl(client, "goframeRepeatedMountQueueAThenMountB");
    state = await waitForState(client, "queued App A transfer", (next) =>
        next.activeApplication === "B:2" && next.appBValue === 1);
    await waitForAnimationFrames(client, 2);
    state = await repeatedMountState(client);
    assertSubset(state, {
        activeApplication: "B:2",
        appARenders: 3,
        appAEffectSetups: [1, 1],
        appAEffectCleanups: [1, 1],
        appAUnmounts: [1, 1],
        appAEffectCleanupSawDOM: [1, 1],
        appAUnmountSawDOM: [1, 1],
        appAEffectVersions: ["1:0", "1:1", "2:0"],
        appBRenders: 5,
        appBScheduledUpdates: 2,
        appBCleanups: [1, 0],
        runtimeErrorCount: 0,
    }, "queued old update isolation");
    assertRoot(state.rootA, { appA: 0, appB: 0, appC: 0, goframeComments: 0 }, "root A after queued transfer");
    assertRoot(state.rootB, { appA: 0, appB: 1, appC: 0, goframeComments: 2 }, "second App B active root");

    await click(client, "[data-testid='app-b-event']");
    state = await waitForState(client, "second App B interaction", (next) =>
        next.appBHandlers === 2 && next.appBValue === 2);
    await client.evaluate(`(() => {
        window.__repeatedMountOldBButton = document.querySelector("[data-testid='app-b-event']");
        return true;
    })()`);

    await runControl(client, "goframeRepeatedMountReplaceBWithC");
    state = await waitForState(client, "same-root App C replacement", (next) =>
        next.activeApplication === "C:1" && next.rootB.appC === 1);
    assertSubset(state, {
        appBCleanups: [1, 1],
        appCCleanups: [0],
        oldBButtonConnected: false,
        duplicateIDs: [],
        runtimeErrorCount: 0,
    }, "same-root replacement");
    assertRoot(state.rootB, { appA: 0, appB: 0, appC: 1, goframeComments: 2 }, "same-root active App C");

    const oldBHandlers = state.appBHandlers;
    await client.evaluate(`(() => {
        window.__repeatedMountOldBButton.click();
        return true;
    })()`);
    await waitForAnimationFrames(client, 2);
    state = await repeatedMountState(client);
    assertSubset(state, { appBHandlers: oldBHandlers, runtimeErrorCount: 0 }, "released App B listener");

    await click(client, "[data-testid='app-c-event']");
    state = await waitForState(client, "App C interaction", (next) =>
        next.appCHandlers === 1 && next.currentAppState === 1);

    await runControl(client, "goframeRepeatedMountMountFreshA");
    state = await waitForState(client, "transfer back to root A", (next) =>
        next.activeApplication === "A:3" && next.rootA.appA === 1);
    assertSubset(state, {
        appARenders: 4,
        appAEffectSetups: [1, 1, 1],
        appAEffectVersions: ["1:0", "1:1", "2:0", "3:0"],
        appCCleanups: [1],
        hostSentinelInRootA: false,
        firstAMarkerIsCurrent: false,
        duplicateIDs: [],
        runtimeErrorCount: 0,
    }, "A to B to C to A transfer");
    assertRoot(state.rootA, { appA: 1, appB: 0, appC: 0, goframeComments: 2 }, "fresh App A active root");
    assertRoot(state.rootB, { appA: 0, appB: 0, appC: 0, goframeComments: 0 }, "root B after transfer back");

    await click(client, "[data-testid='app-a-event']");
    state = await waitForState(client, "fresh App A interaction", (next) =>
        next.appAHandlers === 2 && next.appAValue === 1);

    if (compiler === "go") {
        await client.evaluate(`(() => {
            window.__repeatedMountBeforeMissing = document.querySelector("[data-testid='app-a']");
            return true;
        })()`);
        const cleanupBeforeMissing = JSON.stringify({
            effects: state.appAEffectCleanups,
            unmounts: state.appAUnmounts,
        });
        await runControl(client, "goframeRepeatedMountAttemptMissingRoot");
        state = await repeatedMountState(client);
        assertSubset(state, {
            activeApplication: "A:3",
            missingRootPanicCount: 1,
            missingRootPanicText: "goframe: root element not found: missing-root",
            currentAMarkerSameAfterMissing: true,
            runtimeErrorCount: 0,
        }, "missing-root preservation");
        const cleanupAfterMissing = JSON.stringify({
            effects: state.appAEffectCleanups,
            unmounts: state.appAUnmounts,
        });
        if (cleanupAfterMissing !== cleanupBeforeMissing) {
            throw new Error(`APP FAILURE: missing-root mount changed cleanup counts: before=${cleanupBeforeMissing}, after=${cleanupAfterMissing}`);
        }
        await click(client, "[data-testid='app-a-event']");
        state = await waitForState(client, "post-panic App A interaction", (next) =>
            next.appAHandlers === 3 && next.appAValue === 2);
        console.log("missing-root preservation (go): ok");
    } else if (compiler === "tinygo") {
        console.log(
            "missing-root preservation (tinygo): NOT APPLICABLE - normal TinyGo " +
            "trap-mode build does not provide recover-based panic containment",
        );
    } else {
        throw new Error(`HARNESS FAILURE: unsupported compiler label ${JSON.stringify(compiler)}`);
    }

    assertSubset(state, { runtimeErrorCount: 0, duplicateIDs: [] }, "final runtime evidence");
    console.log(`repeated mount evidence (${compiler}): ${JSON.stringify(state)}`);
    console.log(`Repeated mount browser smoke (${compiler}): ok`);
} finally {
    client?.close();
    if (browser.exitCode === null && !browser.killed) {
        const exited = new Promise((resolve) => browser.once("exit", resolve));
        browser.kill("SIGTERM");
        await Promise.race([exited, wait(2000)]);
    }
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function click(client, selector) {
    const clicked = await client.callFunction(`function(selector) {
        const element = document.querySelector(selector);
        if (!element) return false;
        element.click();
        return true;
    }`, selector);
    if (!clicked) {
        throw new Error(`APP FAILURE: missing element for click ${selector} (${compiler})`);
    }
}

async function runControl(client, name) {
    const result = await client.callFunction(`function(name) {
        const control = globalThis[name];
        if (typeof control !== "function") return { ok: false };
        control();
        return { ok: true };
    }`, name);
    if (!result?.ok) {
        throw new Error(`HARNESS FAILURE: missing fixture control ${name} (${compiler})`);
    }
}

async function repeatedMountState(client) {
    return await client.evaluate(`(() => {
        const rootState = (root) => {
            const comments = [];
            const walker = document.createTreeWalker(root, NodeFilter.SHOW_COMMENT);
            while (walker.nextNode()) comments.push(walker.currentNode.nodeValue ?? "");
            return {
                appA: root.querySelectorAll("[data-testid='app-a']").length,
                appB: root.querySelectorAll("[data-testid='app-b']").length,
                appC: root.querySelectorAll("[data-testid='app-c']").length,
                goframeComments: comments.filter((value) => value.includes("goframe-")).length,
                text: root.textContent,
            };
        };
        const ids = Array.from(document.querySelectorAll("[id]"), (element) => element.id);
        const duplicateIDs = ids.filter((id, index) => ids.indexOf(id) !== index);
        const currentMarker = document.querySelector(
            "[data-testid='app-a'], [data-testid='app-b'], [data-testid='app-c']",
        );
        return {
            ...globalThis.goframeRepeatedMountRead(),
            rootA: rootState(document.querySelector("#root-a")),
            rootB: rootState(document.querySelector("#root-b")),
            hostSentinelInRootA:
                document.querySelector("#host-owned-sentinel")?.parentElement?.id === "root-a",
            oldAButtonConnected: Boolean(window.__repeatedMountOldAButton?.isConnected),
            oldBButtonConnected: Boolean(window.__repeatedMountOldBButton?.isConnected),
            firstAMarkerIsCurrent:
                window.__repeatedMountFirstAMarker ===
                document.querySelector("[data-testid='app-a']"),
            currentAMarkerSameAfterMissing:
                window.__repeatedMountBeforeMissing ===
                document.querySelector("[data-testid='app-a']"),
            currentAppState: Number(currentMarker?.dataset.state ?? "-1"),
            duplicateIDs,
        };
    })()`);
}

async function waitForState(client, label, predicate) {
    const started = Date.now();
    let state;
    while (Date.now() - started < 10_000) {
        state = await repeatedMountState(client);
        if (predicate(state)) {
            return state;
        }
        await wait(50);
    }
    throw new Error(`APP FAILURE: timed out waiting for ${label} (${compiler}): ${JSON.stringify(state)}`);
}

function assertSubset(actual, expected, label) {
    for (const [key, value] of Object.entries(expected)) {
        if (JSON.stringify(actual[key]) !== JSON.stringify(value)) {
            throw new Error(
                `APP FAILURE: ${label} (${compiler}) ${key} got ` +
                `${JSON.stringify(actual[key])}, want ${JSON.stringify(value)}; ` +
                `state=${JSON.stringify(actual)}`,
            );
        }
    }
    console.log(`${label} (${compiler}): ok`);
}

function assertRoot(actual, expected, label) {
    for (const [key, value] of Object.entries(expected)) {
        if (actual[key] !== value) {
            throw new Error(
                `APP FAILURE: ${label} (${compiler}) ${key} got ` +
                `${JSON.stringify(actual[key])}, want ${JSON.stringify(value)}; ` +
                `root=${JSON.stringify(actual)}`,
            );
        }
    }
    console.log(`${label} (${compiler}): ok`);
}

async function waitForAnimationFrames(client, count) {
    await client.callFunction(`async function(count) {
        for (let index = 0; index < count; index++) {
            await new Promise((resolve) => requestAnimationFrame(resolve));
        }
        return true;
    }`, count);
}

async function waitForPage(port) {
    const started = Date.now();
    let lastError;
    while (Date.now() - started < 5000) {
        if (browserExit) {
            throw new Error(
                `HARNESS FAILURE: Chrome exited before CDP was ready (${compiler}): ` +
                `${JSON.stringify(browserExit)}\n${browserError}`,
            );
        }
        try {
            const response = await fetch(`http://127.0.0.1:${port}/json`);
            if (response.ok) {
                const targets = await response.json();
                const page = targets.find(
                    (entry) => entry.type === "page" && entry.webSocketDebuggerUrl,
                );
                if (page) return page;
            }
        } catch (error) {
            lastError = error;
        }
        await wait(50);
    }
    throw new Error(
        `HARNESS FAILURE: Chrome DevTools page unavailable (${compiler}): ` +
        `${lastError?.message ?? browserError}`,
    );
}

async function waitForAppPage(client, expected) {
    const started = Date.now();
    let state;
    while (Date.now() - started < 10_000) {
        state = await client.evaluate(`(() => ({
            href: location.href,
            origin: location.origin,
            protocol: location.protocol,
            readyState: document.readyState,
            rootA: Boolean(document.querySelector("#root-a")),
            rootB: Boolean(document.querySelector("#root-b")),
            appReady: Boolean(document.querySelector("[data-testid='app-a']")),
            controlsReady: typeof globalThis.goframeRepeatedMountRead === "function",
        }))()`);
        if (state.href.startsWith("chrome-error://")) {
            throw new Error(
                `HARNESS FAILURE: Chrome loaded an error document (${compiler}): ` +
                `${JSON.stringify(state)}`,
            );
        }
        if (
            (state.protocol === "http:" || state.protocol === "https:") &&
            state.origin === expected.origin &&
            new URL(state.href).pathname === expected.pathname &&
            state.rootA &&
            state.rootB &&
            state.appReady &&
            state.controlsReady
        ) {
            return;
        }
        await wait(50);
    }
    throw new Error(
        `HARNESS FAILURE: repeated mount app did not become ready at expected origin ` +
        `(${compiler}): ${JSON.stringify(state)}`,
    );
}

function withSmokeParam(url, label) {
    const next = new URL(url);
    next.searchParams.set("smoke", `${Date.now()}-${label}`);
    return next.toString();
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
                        throw new Error(
                            `APP FAILURE: browser evaluation failed (${compiler}): ` +
                            `${JSON.stringify(response.exceptionDetails)}`,
                        );
                    }
                    return response.result.value;
                },
                async callFunction(functionDeclaration, ...args) {
                    if (!this.globalObjectID) {
                        const globalResponse = await this.call("Runtime.evaluate", {
                            expression: "globalThis",
                            returnByValue: false,
                        });
                        if (globalResponse.exceptionDetails) {
                            throw new Error(
                                `HARNESS FAILURE: global object unavailable (${compiler}): ` +
                                `${JSON.stringify(globalResponse.exceptionDetails)}`,
                            );
                        }
                        this.globalObjectID = globalResponse.result.objectId;
                    }
                    const response = await this.call("Runtime.callFunctionOn", {
                        objectId: this.globalObjectID,
                        functionDeclaration,
                        arguments: args.map((value) => ({ value })),
                        awaitPromise: true,
                        returnByValue: true,
                    });
                    if (response.exceptionDetails) {
                        throw new Error(
                            `APP FAILURE: browser function failed (${compiler}): ` +
                            `${JSON.stringify(response.exceptionDetails)}`,
                        );
                    }
                    return response.result.value;
                },
                close() {
                    socket.close();
                },
            });
        }, { once: true });
        socket.addEventListener("error", reject, { once: true });
        socket.addEventListener("message", (event) => {
            const message = JSON.parse(event.data);
            if (!message.id || !pending.has(message.id)) return;
            const request = pending.get(message.id);
            pending.delete(message.id);
            if (message.error) {
                request.reject(new Error(message.error.message));
                return;
            }
            request.resolve(message.result);
        });
    });
}

function wait(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

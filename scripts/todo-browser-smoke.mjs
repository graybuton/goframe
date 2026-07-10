import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const appURL = process.argv[2] ?? process.env.GOFRAME_TODO_SMOKE_URL ?? "http://127.0.0.1:18080/";
const debugPort = Number(process.env.GOFRAME_CHROME_DEBUG_PORT ?? "19222");
const chrome = process.env.CHROME ?? "google-chrome";
const profile = await mkdtemp(join(tmpdir(), "goframe-todo-smoke-"));
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

    await navigateToApp(client, withSmokeParam(appURL, "initial"));
    await waitForAppPage(client, expectedApp, "initial navigation");
    await clearClientStorage(client, expectedApp);
    await navigateToApp(client, withSmokeParam(appURL, "clean"));
    await waitForAppPage(client, expectedApp, "post-clean navigation");

    assertDeepEqual(
        await client.evaluate(`(() => {
            window.__root = document.querySelector("#root");
            window.__input = document.querySelector("#todo-input");
            window.__header = document.querySelector(".site-header");
            return {
                root: Boolean(window.__root),
                input: Boolean(window.__input),
                empty: document.querySelectorAll(".empty-state").length,
                appRenders: window.goframeComponentRenderCounts.App,
                headerRenders: window.goframeComponentRenderCounts.Header,
                formRenders: window.goframeComponentRenderCounts.TodoForm,
            };
        })()`),
        { root: true, input: true, empty: 1, appRenders: 1, headerRenders: 1, formRenders: 1 },
        "initial render",
    );

    await addTodo(client, "A");
    assertDeepEqual(
        await client.evaluate(installTypingAuditExpression()),
        { ready: true, listItems: 1 },
        "typing audit installed",
    );

    await startTodoScenario(client, "controlled-input");
    await typeTodoWithRetainedSelection(client, "BC", "BXC", 1, 1);
    assertDeepEqual(
        await client.evaluate(`(() => {
            const debug = window.__GOFRAME_DEBUG__;
            const baseline = window.__typingBaseline;
            return {
                rootSame: window.__root === document.querySelector("#root"),
                rootChildrenSame: window.__rootChildren.length === window.__root.childNodes.length &&
                    window.__rootChildren.every((node, index) => node === window.__root.childNodes[index]),
                inputSame: window.__input === document.querySelector("#todo-input"),
                headerSame: window.__header === document.querySelector(".site-header"),
                formSame: window.__form === document.querySelector(".todo-form"),
                listSame: window.__list === document.querySelector("#todo-list"),
                active: document.activeElement.id,
                value: document.querySelector("#todo-input").value,
                selectionStart: document.querySelector("#todo-input").selectionStart,
                selectionEnd: document.querySelector("#todo-input").selectionEnd,
                appRenderDelta: (window.goframeComponentRenderCounts.App || 0) - baseline.appRenders,
                headerRenderDelta: (window.goframeComponentRenderCounts.Header || 0) - baseline.headerRenders,
                formRenderDelta: (window.goframeComponentRenderCounts.TodoForm || 0) - baseline.formRenders,
                listRenderDelta: (window.goframeComponentRenderCounts.TodoList || 0) - baseline.listRenders,
                appPatchDelta: (window.goframeComponentPatchCounts.App || 0) - baseline.appPatches,
                headerPatchDelta: (window.goframeComponentPatchCounts.Header || 0) - baseline.headerPatches,
                formPatchDelta: (window.goframeComponentPatchCounts.TodoForm || 0) - baseline.formPatches,
                listPatchDelta: (window.goframeComponentPatchCounts.TodoList || 0) - baseline.listPatches,
                mutations: debug.mutations,
                operations: debug.operations,
            };
        })()`),
        {
            rootSame: true,
            rootChildrenSame: true,
            inputSame: true,
            headerSame: true,
            formSame: true,
            listSame: true,
            active: "todo-input",
            value: "BC",
            selectionStart: 1,
            selectionEnd: 1,
            appRenderDelta: 0,
            headerRenderDelta: 0,
            formRenderDelta: 1,
            listRenderDelta: 0,
            appPatchDelta: 0,
            headerPatchDelta: 0,
            formPatchDelta: 1,
            listPatchDelta: 0,
            mutations: {
                rootChildList: 0,
                header: 0,
                formChildList: 0,
                list: 0,
            },
            operations: { ...emptyOperations(), setProperty: 1, setSelectionRange: 1 },
        },
        "typing preserves unrelated DOM and performs no structural operations",
    );
    assertControlledInputCharacterization(await finishTodoScenario(client, "controlled-input"));

    await submitTodo(client);
    assertDeepEqual(
        await client.evaluate(`(() => {
            window.__todo1 = document.querySelector("#todo-1");
            window.__todo2 = document.querySelector("#todo-2");
            window.__summaryNode = document.querySelector(".summary");
            return {
                inputSame: window.__input === document.querySelector("#todo-input"),
                headerSame: window.__header === document.querySelector(".site-header"),
                text: document.querySelector("#todo-1 .todo-text")?.textContent,
                order: [...document.querySelectorAll(".todo-item")].map((node) => node.id),
                summary: window.__summaryNode?.textContent,
                headerRenders: window.goframeComponentRenderCounts.Header,
            };
        })()`),
        {
            inputSame: true,
            headerSame: true,
            text: "A",
            order: ["todo-1", "todo-2"],
            summary: "2 task(s)",
            headerRenders: 1,
        },
        "second add preserves Header and controlled input",
    );

    await client.evaluate(`document.querySelector("#todo-1 .todo-toggle").click()`);
    await wait(120);
    assertDeepEqual(
        await client.evaluate(`({
            done: document.querySelector("#todo-1").classList.contains("todo-item-done"),
            todo1Same: window.__todo1 === document.querySelector("#todo-1"),
            headerRenders: window.goframeComponentRenderCounts.Header,
        })`),
        { done: true, todo1Same: true, headerRenders: 1 },
        "single click fires once and patches props",
    );

    await client.evaluate(`document.querySelector("#todo-1 .todo-toggle").click()`);
    await wait(120);
    assertDeepEqual(
        await client.evaluate(`({
            done: document.querySelector("#todo-1").classList.contains("todo-item-done"),
            todo1Same: window.__todo1 === document.querySelector("#todo-1"),
        })`),
        { done: false, todo1Same: true },
        "second single click removes changed prop",
    );

    await startTodoScenario(client, "keyed-reorder");
    await client.evaluate(
        `[...document.querySelectorAll("button")].find((node) => node.textContent.trim() === "Reverse tasks").click()`,
    );
    await wait(120);
    assertDeepEqual(
        await client.evaluate(`({
            order: [...document.querySelectorAll(".todo-item")].map((node) => node.id),
            todo1Same: window.__todo1 === document.querySelector("#todo-1"),
            todo2Same: window.__todo2 === document.querySelector("#todo-2"),
            headerRenders: window.goframeComponentRenderCounts.Header,
        })`),
        { order: ["todo-2", "todo-1"], todo1Same: true, todo2Same: true, headerRenders: 1 },
        "keyed reorder moves existing DOM nodes",
    );
    assertKeyedReorderCharacterization(await finishTodoScenario(client, "keyed-reorder"));

    await client.evaluate(`document.querySelector("#todo-1 .button").click()`);
    await wait(120);
    assertDeepEqual(
        await client.evaluate(`({
            order: [...document.querySelectorAll(".todo-item")].map((node) => node.id),
            todo2Same: window.__todo2 === document.querySelector("#todo-2"),
            summaryNodeSame: window.__summaryNode === document.querySelector(".summary"),
            summary: document.querySelector(".summary").textContent,
            inputSame: window.__input === document.querySelector("#todo-input"),
            headerSame: window.__header === document.querySelector(".site-header"),
            headerRenders: window.goframeComponentRenderCounts.Header,
        })`),
        {
            order: ["todo-2"],
            todo2Same: true,
            summaryNodeSame: true,
            summary: "1 task(s)",
            inputSame: true,
            headerSame: true,
            headerRenders: 1,
        },
        "keyed removal and text patch preserve survivors",
    );

    await startTodoScenario(client, "burst-state-updates");
    await queueTodoInputBurst(client, ["burst-one", "burst-two", "burst-three"], "transient", 1, 4);
    assertDeepEqual(
        await client.evaluate(`(() => {
            const input = document.querySelector("#todo-input");
            return {
                inputSame: window.__input === input,
                active: document.activeElement.id,
                value: input.value,
                selectionStart: input.selectionStart,
                selectionEnd: input.selectionEnd,
            };
        })()`),
        {
            inputSame: true,
            active: "todo-input",
            value: "burst-three",
            selectionStart: 1,
            selectionEnd: 4,
        },
        "bursty controlled updates retain input focus and selection",
    );
    assertBurstUpdateCharacterization(await finishTodoScenario(client, "burst-state-updates"));

    const persisted = await client.evaluate(`localStorage.getItem("goframe.todo.items")`);
    if (typeof persisted !== "string" || !persisted.includes("BC")) {
        throw new Error(`APP FAILURE: todo persistence: got ${JSON.stringify(persisted)}, want stored todo text`);
    }
    console.log("todo persistence write: ok");

    await client.call("Page.reload", { ignoreCache: true });
    await wait(800);
    assertDeepEqual(
        await client.evaluate(`({
            order: [...document.querySelectorAll(".todo-item")].map((node) => node.id),
            text: document.querySelector("#todo-2 .todo-text")?.textContent,
            headerRenders: window.goframeComponentRenderCounts.Header,
        })`),
        { order: ["todo-2"], text: "BC", headerRenders: 1 },
        "todo persistence reload",
    );

    client.close();
    console.log("Todo browser smoke: ok");
} finally {
    const exited = new Promise((resolve) => browser.once("exit", resolve));
    browser.kill("SIGTERM");
    await Promise.race([exited, wait(2000)]);
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function clearClientStorage(client) {
    await ensureAppPage(client, expectedApp, "before storage cleanup");
    const result = await client.evaluate(`(async () => {
        const result = {
            href: window.location.href,
            origin: window.location.origin,
            protocol: window.location.protocol,
            ok: false,
            error: "",
        };
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

async function addTodo(client, text) {
    await typeTodo(client, text);
    await submitTodo(client);
}

async function typeTodo(client, text) {
    await client.evaluate(`(() => {
        const input = document.querySelector("#todo-input");
        input.focus();
        input.value = ${JSON.stringify(text)};
        input.setSelectionRange(input.value.length, input.value.length);
        if (window.__GOFRAME_DEBUG__) {
            for (const key of Object.keys(window.__GOFRAME_DEBUG__.operations)) {
                window.__GOFRAME_DEBUG__.operations[key] = 0;
            }
        }
        input.dispatchEvent(new Event("input", { bubbles: true }));
        return true;
    })()`);
    await wait(100);
}

async function typeTodoWithRetainedSelection(client, text, transientText, selectionStart, selectionEnd) {
    await client.evaluate(`(() => {
        const input = document.querySelector("#todo-input");
        input.focus();
        input.value = ${JSON.stringify(text)};
        input.setSelectionRange(${selectionStart}, ${selectionEnd});
        window.__domBridgeAudit.discardDOMWork();
        input.dispatchEvent(new Event("input", { bubbles: true }));

        // Force the dirty patch to restore the controlled value while retaining
        // the active input node, so selection restoration is observable.
        input.value = ${JSON.stringify(transientText)};
        input.setSelectionRange(${selectionStart}, ${selectionEnd});
        window.__domBridgeAudit.discardDOMWork();
        return true;
    })()`);
    await wait(100);
}

async function queueTodoInputBurst(client, values, transientText, selectionStart, selectionEnd) {
    await client.evaluate(`(() => {
        const input = document.querySelector("#todo-input");
        input.focus();
        for (const value of ${JSON.stringify(values)}) {
            input.value = value;
            input.dispatchEvent(new Event("input", { bubbles: true }));
        }

        // Keep the last state update queued while making the controlled patch
        // restore both the value and a non-collapsed selection range.
        input.value = ${JSON.stringify(transientText)};
        input.setSelectionRange(${selectionStart}, ${selectionEnd});
        window.__domBridgeAudit.discardDOMWork();
        return true;
    })()`);
    await wait(100);
}

async function submitTodo(client) {
    await client.evaluate(`document.querySelector("button[type=submit]").click()`);
    await wait(120);
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
    for (let attempt = 0; attempt < 80; attempt++) {
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
            appReady: Boolean(document.querySelector("#todo-input") && window.goframeComponentRenderCounts?.App),
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

async function startTodoScenario(client, label) {
    const started = await client.evaluate(`window.__domBridgeAudit?.start(${JSON.stringify(label)}) ?? false`);
    if (!started) {
        throw new Error(`APP FAILURE: DOM bridge audit did not start ${label}`);
    }
}

async function finishTodoScenario(client, label) {
    const report = await client.evaluate(`window.__domBridgeAudit?.finish(${JSON.stringify(label)}) ?? null`);
    if (!report) {
        throw new Error(`APP FAILURE: DOM bridge audit did not finish ${label}`);
    }
    console.log(`todo DOM bridge ${label}: ${JSON.stringify(report)}`);
    return report;
}

function assertSingleScheduledUpdate(report, label) {
    if (report.flushes !== 1 || report.scheduling.requestAnimationFrame !== 1 ||
        report.scheduling.requestAnimationFrameCallbacks !== 1 ||
        report.scheduling.queueMicrotask !== 0 || report.scheduling.queueMicrotaskCallbacks !== 0) {
        throw new Error(`APP FAILURE: ${label} should use one rAF-batched dirty flush: ${JSON.stringify(report)}`);
    }
}

function assertNoStructuralOrListenerChurn(report, label) {
    const operations = report.operations;
    const structural = [
        "createElement",
        "createTextNode",
        "createComment",
        "appendChild",
        "removeChild",
        "replaceChild",
        "insertBefore",
    ];
    for (const operation of structural) {
        if (operations[operation] !== 0) {
            throw new Error(`APP FAILURE: ${label} unexpectedly used ${operation}: ${JSON.stringify(report)}`);
        }
    }
    if (operations.addEventListener !== 0 || operations.removeEventListener !== 0) {
        throw new Error(`APP FAILURE: ${label} churned listeners: ${JSON.stringify(report)}`);
    }
}

function assertControlledInputCharacterization(report) {
    assertSingleScheduledUpdate(report, "controlled input");
    assertNoStructuralOrListenerChurn(report, "controlled input");
    if (report.operations.setProperty !== 1 || report.operations.focus !== 0 ||
        report.operations.setSelectionRange !== 1 || report.flushComponents.TodoForm !== 1 ||
        report.flushPatches.TodoForm !== 1 || report.mutations.rootChildList !== 0 ||
        report.mutations.header !== 0 || report.mutations.formChildList !== 0 || report.mutations.list !== 0) {
        throw new Error(`APP FAILURE: controlled-input bridge characterization changed: ${JSON.stringify(report)}`);
    }
}

function assertBurstUpdateCharacterization(report) {
    assertSingleScheduledUpdate(report, "burst state updates");
    assertNoStructuralOrListenerChurn(report, "burst state updates");
    if (report.operations.setProperty !== 1 || report.operations.focus !== 0 ||
        report.operations.setSelectionRange !== 1 || report.flushComponents.TodoForm !== 1 ||
        report.flushPatches.TodoForm !== 1) {
        throw new Error(`APP FAILURE: burst updates did not coalesce into one retained-input patch: ${JSON.stringify(report)}`);
    }
}

function assertKeyedReorderCharacterization(report) {
    assertSingleScheduledUpdate(report, "keyed reorder");
    if (report.operations.createElement !== 0 || report.operations.createTextNode !== 0 ||
        report.operations.createComment !== 0 || report.operations.removeChild !== 0 ||
        report.operations.addEventListener !== 0 || report.operations.removeEventListener !== 0 ||
        // Moving a retained TodoItem component range places its start anchor,
        // element, and end anchor without recreating the range.
        report.operations.insertBefore !== 3 || report.flushComponents.TodoList !== 1 ||
        report.flushPatches.TodoList !== 1) {
        throw new Error(`APP FAILURE: keyed reorder should retain nodes and use bounded placement: ${JSON.stringify(report)}`);
    }
}

function installTypingAuditExpression() {
    return `(() => {
    window.__root = document.querySelector("#root");
    window.__header = document.querySelector(".site-header");
    window.__form = document.querySelector(".todo-form");
    window.__input = document.querySelector("#todo-input");
    window.__list = document.querySelector("#todo-list");
    window.__rootChildren = [...window.__root.childNodes];

    const operations = ${JSON.stringify(emptyOperations())};
    const mutations = {
        rootChildList: 0,
        header: 0,
        formChildList: 0,
        list: 0,
    };
    const scheduling = {
        requestAnimationFrame: 0,
        requestAnimationFrameCallbacks: 0,
        queueMicrotask: 0,
        queueMicrotaskCallbacks: 0,
    };
    const componentNames = ["App", "Header", "TodoForm", "TodoList"];
    const renderReports = [];
    window.goframeRenderProbe = (phase, duration) => {
        renderReports.push({ phase, duration });
    };
    const audit = {
        baseline: null,
        renderBaseline: 0,
        start() {
            for (const key of Object.keys(operations)) operations[key] = 0;
            for (const key of Object.keys(mutations)) mutations[key] = 0;
            for (const key of Object.keys(scheduling)) scheduling[key] = 0;
            this.baseline = snapshotComponentCounts(componentNames);
            this.renderBaseline = renderReports.length;
            window.__typingBaseline = {
                appRenders: this.baseline.renders.App,
                headerRenders: this.baseline.renders.Header,
                formRenders: this.baseline.renders.TodoForm,
                listRenders: this.baseline.renders.TodoList,
                appPatches: this.baseline.patches.App,
                headerPatches: this.baseline.patches.Header,
                formPatches: this.baseline.patches.TodoForm,
                listPatches: this.baseline.patches.TodoList,
            };
            return true;
        },
        discardDOMWork() {
            for (const key of Object.keys(operations)) operations[key] = 0;
            for (const key of Object.keys(mutations)) mutations[key] = 0;
        },
        finish(label) {
            const next = snapshotComponentCounts(componentNames);
            const flushComponents = {};
            const flushPatches = {};
            for (const name of componentNames) {
                flushComponents[name] = next.renders[name] - this.baseline.renders[name];
                flushPatches[name] = next.patches[name] - this.baseline.patches[name];
            }
            const updates = renderReports.slice(this.renderBaseline).filter((report) => report.phase === "update");
            return {
                label,
                flushes: updates.length,
                flushComponents,
                flushPatches,
                scheduling: { ...scheduling },
                operations: { ...operations },
                mutations: { ...mutations },
            };
        },
    };
    window.__domBridgeAudit = audit;
    window.__GOFRAME_DEBUG__ = { operations, mutations };

    const wrap = (owner, name, counter) => {
        const original = owner[name];
        owner[name] = function(...args) {
            operations[counter]++;
            return original.apply(this, args);
        };
    };
    wrap(Document.prototype, "createElement", "createElement");
    wrap(Document.prototype, "createTextNode", "createTextNode");
    wrap(Document.prototype, "createComment", "createComment");
    wrap(Node.prototype, "appendChild", "appendChild");
    wrap(Node.prototype, "removeChild", "removeChild");
    wrap(Node.prototype, "replaceChild", "replaceChild");
    wrap(Node.prototype, "insertBefore", "insertBefore");
    wrap(Element.prototype, "setAttribute", "setAttribute");
    wrap(Element.prototype, "removeAttribute", "removeAttribute");
    wrap(EventTarget.prototype, "addEventListener", "addEventListener");
    wrap(EventTarget.prototype, "removeEventListener", "removeEventListener");
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
    const setSelectionRange = HTMLInputElement.prototype.setSelectionRange;
    HTMLInputElement.prototype.setSelectionRange = function(...args) {
        operations.setSelectionRange++;
        return setSelectionRange.apply(this, args);
    };

    const requestAnimationFrame = window.requestAnimationFrame;
    window.requestAnimationFrame = function(callback) {
        scheduling.requestAnimationFrame++;
        return requestAnimationFrame.call(window, (...args) => {
            scheduling.requestAnimationFrameCallbacks++;
            return callback(...args);
        });
    };
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

    new MutationObserver((records) => {
        mutations.rootChildList += records.filter((record) => record.type === "childList").length;
    }).observe(window.__root, { childList: true });
    new MutationObserver((records) => {
        mutations.header += records.length;
    }).observe(window.__header, { childList: true, subtree: true, attributes: true, characterData: true });
    new MutationObserver((records) => {
        mutations.formChildList += records.filter((record) => record.type === "childList").length;
    }).observe(window.__form, { childList: true, subtree: true });
    new MutationObserver((records) => {
        mutations.list += records.length;
    }).observe(window.__list, { childList: true, subtree: true, attributes: true, characterData: true });

    window.__typingBaseline = {
        appRenders: 0,
        headerRenders: 0,
        formRenders: 0,
        listRenders: 0,
        appPatches: 0,
        headerPatches: 0,
        formPatches: 0,
        listPatches: 0,
    };
    return { ready: true, listItems: document.querySelectorAll(".todo-item").length };

    function snapshotComponentCounts(names) {
        const renderCounts = window.goframeComponentRenderCounts || {};
        const patchCounts = window.goframeComponentPatchCounts || {};
        const renders = {};
        const patches = {};
        for (const name of names) {
            renders[name] = renderCounts[name] || 0;
            patches[name] = patchCounts[name] || 0;
        }
        return { renders, patches };
    }
})()`;
}

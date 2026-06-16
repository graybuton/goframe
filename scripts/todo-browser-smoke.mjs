import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const appURL = process.argv[2] ?? "http://127.0.0.1:18080/";
const debugPort = Number(process.env.GOFRAME_CHROME_DEBUG_PORT ?? "19222");
const chrome = process.env.CHROME ?? "google-chrome";
const profile = await mkdtemp(join(tmpdir(), "goframe-todo-smoke-"));
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
    await wait(800);

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

    await typeTodo(client, "B");
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
            value: "B",
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
            operations: emptyOperations(),
        },
        "typing preserves unrelated DOM and performs no structural operations",
    );

    await submitTodo(client);
    assertDeepEqual(
        await client.evaluate(`(() => {
            window.__todo1 = document.querySelector("#todo-1");
            window.__todo2 = document.querySelector("#todo-2");
            window.__summaryText = document.querySelector(".summary").firstChild;
            return {
                inputSame: window.__input === document.querySelector("#todo-input"),
                headerSame: window.__header === document.querySelector(".site-header"),
                text: document.querySelector("#todo-1 .todo-text")?.textContent,
                order: [...document.querySelectorAll(".todo-item")].map((node) => node.id),
                summary: window.__summaryText.nodeValue,
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

    await client.evaluate(`document.querySelector("#todo-1 .button").click()`);
    await wait(120);
    assertDeepEqual(
        await client.evaluate(`({
            order: [...document.querySelectorAll(".todo-item")].map((node) => node.id),
            todo2Same: window.__todo2 === document.querySelector("#todo-2"),
            summaryNodeSame: window.__summaryText === document.querySelector(".summary").firstChild,
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

    client.close();
    console.log("Todo browser smoke: ok");
} finally {
    const exited = new Promise((resolve) => browser.once("exit", resolve));
    browser.kill("SIGTERM");
    await Promise.race([exited, wait(2000)]);
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
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

async function submitTodo(client) {
    await client.evaluate(`document.querySelector("button[type=submit]").click()`);
    await wait(120);
}

async function waitForPage(port) {
    let lastError;
    for (let attempt = 0; attempt < 50; attempt++) {
        try {
            const pages = await fetch(`http://127.0.0.1:${port}/json`).then((response) => response.json());
            const page = pages.find((entry) => entry.type === "page");
            if (page) {
                return page;
            }
        } catch (error) {
            lastError = error;
        }
        await wait(100);
    }
    throw new Error(`Chrome DevTools did not become ready: ${lastError ?? browserError}`);
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
        throw new Error(`${label}: got ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`);
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
    wrap(Node.prototype, "appendChild", "appendChild");
    wrap(Node.prototype, "removeChild", "removeChild");
    wrap(Node.prototype, "replaceChild", "replaceChild");
    wrap(Node.prototype, "insertBefore", "insertBefore");
    wrap(Element.prototype, "setAttribute", "setAttribute");
    wrap(Element.prototype, "removeAttribute", "removeAttribute");
    wrap(EventTarget.prototype, "addEventListener", "addEventListener");
    wrap(EventTarget.prototype, "removeEventListener", "removeEventListener");

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

    const renderCounts = window.goframeComponentRenderCounts;
    const patchCounts = window.goframeComponentPatchCounts;
    window.__typingBaseline = {
        appRenders: renderCounts.App || 0,
        headerRenders: renderCounts.Header || 0,
        formRenders: renderCounts.TodoForm || 0,
        listRenders: renderCounts.TodoList || 0,
        appPatches: patchCounts.App || 0,
        headerPatches: patchCounts.Header || 0,
        formPatches: patchCounts.TodoForm || 0,
        listPatches: patchCounts.TodoList || 0,
    };
    for (const key of Object.keys(operations)) operations[key] = 0;
    return { ready: true, listItems: document.querySelectorAll(".todo-item").length };
})()`;
}

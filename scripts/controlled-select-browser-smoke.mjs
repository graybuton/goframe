import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const appURL = process.argv[2] ?? process.env.GOFRAME_CONTROLLED_SELECT_SMOKE_URL ?? "http://127.0.0.1:18080/";
const compiler = process.argv[3] ?? "unknown";
const debugPort = Number(process.env.GOFRAME_CONTROLLED_SELECT_CHROME_DEBUG_PORT ?? "19261");
const chrome = process.env.CHROME ?? "google-chrome";
const profile = await mkdtemp(join(tmpdir(), "goframe-controlled-select-smoke-"));
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
    await client.call("Page.navigate", { url: withSmokeParam(appURL, compiler) });
    await waitForAppPage(client, expectedApp);

    await client.evaluate(`(() => {
        window.__controlledSelectNode =
            document.querySelector("[data-testid='controlled-select']");
        window.__uncontrolledSelectNode =
            document.querySelector("[data-testid='uncontrolled-select']");
        window.__descendantControlledSelectNode =
            document.querySelector("[data-testid='descendant-controlled-select']");
        return true;
    })()`);

    const evidence = [];
    evidence.push(await assertSnapshot(client, "initial no-match mount", {
        phase: "initial-no-match",
        authoredValue: "b",
        optionValues: ["a"],
        actualValue: "",
        selectedIndex: -1,
        controlledSame: true,
        inputCount: 0,
        changeCount: 0,
    }));

    await clickAndWait(
        client,
        "[data-testid='select-add-match']",
        (state) => state.phase === "matching-option-added",
        "matching option insertion",
    );
    evidence.push(await assertSnapshot(client, "add matching option", {
        phase: "matching-option-added",
        authoredValue: "b",
        optionValues: ["a", "b"],
        actualValue: "b",
        selectedIndex: 1,
        controlledSame: true,
        inputCount: 0,
        changeCount: 0,
    }));

    await clickAndWait(
        client,
        "[data-testid='select-change-value']",
        (state) => state.phase === "value-and-options-changed",
        "value and option change",
    );
    evidence.push(await assertSnapshot(client, "change value and options", {
        phase: "value-and-options-changed",
        authoredValue: "c",
        optionValues: ["a", "c"],
        actualValue: "c",
        selectedIndex: 1,
        controlledSame: true,
        inputCount: 0,
        changeCount: 0,
    }));

    await clickAndWait(
        client,
        "[data-testid='select-remove-match']",
        (state) => state.phase === "matching-option-removed",
        "matching option removal",
    );
    evidence.push(await assertSnapshot(client, "remove matching option", {
        phase: "matching-option-removed",
        authoredValue: "c",
        optionValues: ["a", "b"],
        actualValue: "",
        selectedIndex: -1,
        controlledSame: true,
        inputCount: 0,
        changeCount: 0,
    }));

    await clickAndWait(
        client,
        "[data-testid='select-restore-match']",
        (state) => state.phase === "matching-option-restored",
        "matching option restoration",
    );
    evidence.push(await assertSnapshot(client, "restore matching option", {
        phase: "matching-option-restored",
        authoredValue: "c",
        optionValues: ["a", "b", "c"],
        actualValue: "c",
        selectedIndex: 2,
        controlledSame: true,
        inputCount: 0,
        changeCount: 0,
    }));

    await clickAndWait(
        client,
        "[data-testid='select-reorder']",
        (state) => state.phase === "options-reordered",
        "option reorder",
    );
    evidence.push(await assertSnapshot(client, "reorder options", {
        phase: "options-reordered",
        authoredValue: "c",
        optionValues: ["c", "a", "b"],
        actualValue: "c",
        selectedIndex: 0,
        controlledSame: true,
        inputCount: 0,
        changeCount: 0,
    }));

    await client.evaluate(`(() => {
        const select = document.querySelector("[data-testid='uncontrolled-select']");
        select.value = "b";
        return select.value;
    })()`);
    evidence.push(await assertSnapshot(client, "set uncontrolled selection", {
        uncontrolledValue: "b",
        uncontrolledIndex: 1,
        uncontrolledOptions: ["a", "b"],
        uncontrolledSame: true,
    }));

    await clickAndWait(
        client,
        "[data-testid='select-rerender-uncontrolled']",
        (state) => state.uncontrolledRenderCount === 1,
        "uncontrolled unrelated rerender",
    );
    evidence.push(await assertSnapshot(client, "uncontrolled rerender", {
        uncontrolledValue: "b",
        uncontrolledIndex: 1,
        uncontrolledOptions: ["a", "b"],
        uncontrolledSame: true,
    }));

    await clickAndWait(
        client,
        "[data-testid='select-extend-uncontrolled']",
        (state) => state.uncontrolledOptions.length === 3,
        "uncontrolled option insertion",
    );
    evidence.push(await assertSnapshot(client, "uncontrolled option insertion", {
        uncontrolledValue: "b",
        uncontrolledIndex: 1,
        uncontrolledOptions: ["a", "b", "c"],
        uncontrolledSame: true,
    }));

    evidence.push(await assertSnapshot(client, "programmatic event counts", {
        actualValue: "c",
        inputCount: 0,
        changeCount: 0,
    }));
    await client.evaluate(`(() => {
        document.querySelector("[data-testid='controlled-select']")
            .dispatchEvent(new Event("change", { bubbles: true }));
        return true;
    })()`);
    await waitForState(
        client,
        (state) => state.changeCount === 1,
        "explicit controlled change callback",
    );
    evidence.push(await assertSnapshot(client, "explicit event liveness", {
        actualValue: "c",
        controlledSame: true,
        inputCount: 0,
        changeCount: 1,
    }));

    evidence.push(await assertSnapshot(client, "descendant initial no-match", {
        descendantPhase: "only-a",
        descendantAuthoredValue: "b",
        descendantOptionValues: ["a"],
        descendantActualValue: "",
        descendantSelectedIndex: -1,
        descendantSame: true,
        descendantParentRenderCount: 9,
        descendantChildRenderCount: 9,
        descendantInputCount: 0,
        descendantChangeCount: 0,
    }));

    await clickAndWait(
        client,
        "[data-testid='descendant-add-match']",
        (state) => state.descendantPhase === "a-and-b",
        "descendant matching option insertion",
    );
    evidence.push(await assertSnapshot(client, "descendant add matching option", {
        descendantPhase: "a-and-b",
        descendantAuthoredValue: "b",
        descendantOptionValues: ["a", "b"],
        descendantActualValue: "b",
        descendantSelectedIndex: 1,
        descendantSame: true,
        descendantParentRenderCount: 9,
        descendantChildRenderCount: 10,
        descendantInputCount: 0,
        descendantChangeCount: 0,
    }));

    await clickAndWait(
        client,
        "[data-testid='descendant-remove-match']",
        (state) => state.descendantPhase === "only-a",
        "descendant matching option removal",
    );
    evidence.push(await assertSnapshot(client, "descendant remove matching option", {
        descendantPhase: "only-a",
        descendantAuthoredValue: "b",
        descendantOptionValues: ["a"],
        descendantActualValue: "",
        descendantSelectedIndex: -1,
        descendantSame: true,
        descendantParentRenderCount: 9,
        descendantChildRenderCount: 11,
        descendantInputCount: 0,
        descendantChangeCount: 0,
    }));

    await clickAndWait(
        client,
        "[data-testid='descendant-restore-match']",
        (state) => state.descendantPhase === "a-and-b",
        "descendant matching option restoration",
    );
    evidence.push(await assertSnapshot(client, "descendant restore matching option", {
        descendantPhase: "a-and-b",
        descendantAuthoredValue: "b",
        descendantOptionValues: ["a", "b"],
        descendantActualValue: "b",
        descendantSelectedIndex: 1,
        descendantSame: true,
        descendantParentRenderCount: 9,
        descendantChildRenderCount: 12,
        descendantInputCount: 0,
        descendantChangeCount: 0,
    }));

    await clickAndWait(
        client,
        "[data-testid='descendant-change-value']",
        (state) => state.descendantAuthoredValue === "c",
        "descendant parent value change",
    );
    evidence.push(await assertSnapshot(client, "descendant change current value", {
        descendantPhase: "a-and-b",
        descendantAuthoredValue: "c",
        descendantOptionValues: ["a", "b"],
        descendantActualValue: "",
        descendantSelectedIndex: -1,
        descendantSame: true,
        descendantParentRenderCount: 10,
        descendantChildRenderCount: 13,
        descendantInputCount: 0,
        descendantChangeCount: 0,
    }));

    await clickAndWait(
        client,
        "[data-testid='descendant-add-current']",
        (state) => state.descendantPhase === "a-b-and-c",
        "descendant current option insertion",
    );
    evidence.push(await assertSnapshot(client, "descendant add current option", {
        descendantPhase: "a-b-and-c",
        descendantAuthoredValue: "c",
        descendantOptionValues: ["a", "b", "c"],
        descendantActualValue: "c",
        descendantSelectedIndex: 2,
        descendantSame: true,
        descendantParentRenderCount: 10,
        descendantChildRenderCount: 14,
        descendantInputCount: 0,
        descendantChangeCount: 0,
    }));

    await clickAndWait(
        client,
        "[data-testid='descendant-reorder']",
        (state) => state.descendantPhase === "c-a-and-b",
        "descendant option reorder",
    );
    evidence.push(await assertSnapshot(client, "descendant reorder options", {
        descendantPhase: "c-a-and-b",
        descendantAuthoredValue: "c",
        descendantOptionValues: ["c", "a", "b"],
        descendantActualValue: "c",
        descendantSelectedIndex: 0,
        descendantSame: true,
        descendantParentRenderCount: 10,
        descendantChildRenderCount: 15,
        descendantInputCount: 0,
        descendantChangeCount: 0,
    }));

    await client.evaluate(`(() => {
        document.querySelector("[data-testid='descendant-controlled-select']")
            .dispatchEvent(new Event("change", { bubbles: true }));
        return true;
    })()`);
    await waitForState(
        client,
        (state) => state.descendantChangeCount === 1,
        "explicit descendant change callback",
    );
    evidence.push(await assertSnapshot(client, "descendant explicit event liveness", {
        descendantActualValue: "c",
        descendantSelectedIndex: 0,
        descendantSame: true,
        descendantParentRenderCount: 11,
        descendantChildRenderCount: 16,
        descendantInputCount: 0,
        descendantChangeCount: 1,
    }));

    console.log(`controlled select evidence (${compiler}): ${JSON.stringify(evidence)}`);
    client.close();
    console.log(`Controlled select browser smoke (${compiler}): ok`);
} finally {
    const exited = new Promise((resolve) => browser.once("exit", resolve));
    browser.kill("SIGTERM");
    await Promise.race([exited, wait(2000)]);
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function assertSnapshot(client, label, expected) {
    const actual = await selectState(client);
    console.log(`controlled select ${label} (${compiler}): ${JSON.stringify(actual)}`);
    for (const [key, value] of Object.entries(expected)) {
        if (JSON.stringify(actual[key]) !== JSON.stringify(value)) {
            throw new Error(
                `APP FAILURE: ${label} (${compiler}): ${key} got ` +
                `${JSON.stringify(actual[key])}, want ${JSON.stringify(value)}; ` +
                `state=${JSON.stringify(actual)}`,
            );
        }
    }
    return { label, ...actual };
}

async function clickAndWait(client, selector, predicate, label) {
    await client.evaluate(`(() => {
        const button = document.querySelector(${JSON.stringify(selector)});
        if (!button) throw new Error("missing button " + ${JSON.stringify(selector)});
        button.click();
        return true;
    })()`);
    await waitForState(client, predicate, label);
}

async function waitForState(client, predicate, label) {
    const started = Date.now();
    let lastState;
    while (Date.now() - started < 10_000) {
        lastState = await selectState(client);
        if (predicate(lastState)) {
            return lastState;
        }
        await wait(50);
    }
    throw new Error(
        `APP FAILURE: timed out waiting for ${label} (${compiler}); ` +
        `state=${JSON.stringify(lastState)}`,
    );
}

async function selectState(client) {
    return await client.evaluate(`(() => {
        const controlled =
            document.querySelector("[data-testid='controlled-select']");
        const uncontrolled =
            document.querySelector("[data-testid='uncontrolled-select']");
        const descendant =
            document.querySelector("[data-testid='descendant-controlled-select']");
        const statefulOptions =
            document.querySelector("[data-testid='stateful-options']");
        const number = (selector) =>
            Number(document.querySelector(selector)?.textContent.trim() ?? "0");
        return {
            phase:
                document.querySelector("[data-testid='controlled-phase']")
                    ?.textContent.trim() ?? "",
            authoredValue: controlled?.dataset.authoredValue ?? "",
            optionValues: controlled
                ? Array.from(controlled.options, (option) => option.value)
                : [],
            actualValue: controlled?.value ?? "",
            selectedIndex: controlled?.selectedIndex ?? -2,
            controlledSame: controlled === window.__controlledSelectNode,
            inputCount: number("[data-testid='controlled-input-count']"),
            changeCount: number("[data-testid='controlled-change-count']"),
            uncontrolledValue: uncontrolled?.value ?? "",
            uncontrolledIndex: uncontrolled?.selectedIndex ?? -2,
            uncontrolledOptions: uncontrolled
                ? Array.from(uncontrolled.options, (option) => option.value)
                : [],
            uncontrolledSame: uncontrolled === window.__uncontrolledSelectNode,
            uncontrolledRenderCount:
                number("[data-testid='uncontrolled-rerender-count']"),
            descendantPhase: statefulOptions?.dataset.phase ?? "",
            descendantAuthoredValue: descendant?.dataset.authoredValue ?? "",
            descendantOptionValues: descendant
                ? Array.from(descendant.options, (option) => option.value)
                : [],
            descendantActualValue: descendant?.value ?? "",
            descendantSelectedIndex: descendant?.selectedIndex ?? -2,
            descendantSame:
                descendant === window.__descendantControlledSelectNode,
            descendantParentRenderCount:
                number("[data-testid='descendant-parent-render-count']"),
            descendantChildRenderCount:
                Number(statefulOptions?.dataset.renderCount ?? "0"),
            descendantInputCount:
                number("[data-testid='descendant-input-count']"),
            descendantChangeCount:
                number("[data-testid='descendant-change-count']"),
        };
    })()`);
}

async function waitForPage(port) {
    let lastError;
    for (let attempt = 0; attempt < 100; attempt++) {
        if (browserExit) {
            throw new Error(
                `HARNESS FAILURE: Chrome exited before CDP was ready ` +
                `(${compiler}): ${JSON.stringify(browserExit)}\n${browserError}`,
            );
        }
        try {
            const response = await fetch(`http://127.0.0.1:${port}/json`);
            if (response.ok) {
                const targets = await response.json();
                const page = targets.find(
                    (target) => target.type === "page" && target.webSocketDebuggerUrl,
                );
                if (page) {
                    return page;
                }
            }
        } catch (error) {
            lastError = error;
        }
        await wait(50);
    }
    throw new Error(
        `HARNESS FAILURE: Chrome DevTools did not become ready (${compiler}): ` +
        `${lastError?.message ?? browserError}`,
    );
}

async function waitForAppPage(client, expected) {
    const started = Date.now();
    let lastState;
    while (Date.now() - started < 10_000) {
        lastState = await client.evaluate(`(() => ({
            href: location.href,
            origin: location.origin,
            protocol: location.protocol,
            readyState: document.readyState,
            root: Boolean(document.querySelector("#root")),
            appReady: Boolean(
                document.querySelector("[data-testid='controlled-select-fixture']"),
            ),
        }))()`);
        if (lastState.href.startsWith("chrome-error://")) {
            throw new Error(
                `HARNESS FAILURE: Chrome loaded an error document (${compiler}): ` +
                `${JSON.stringify(lastState)}`,
            );
        }
        if (
            (lastState.protocol === "http:" || lastState.protocol === "https:") &&
            lastState.origin === expected.origin &&
            lastState.root &&
            lastState.appReady
        ) {
            return;
        }
        await wait(50);
    }
    throw new Error(
        `HARNESS FAILURE: app did not become ready at expected origin ` +
        `(${compiler}): ${JSON.stringify(lastState)}`,
    );
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
            request.resolve(message.result ?? {});
        }
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
                throw new Error(
                    `APP FAILURE: browser evaluation failed (${compiler}): ` +
                    `${JSON.stringify(result.exceptionDetails)}`,
                );
            }
            return result.result.value;
        },
        close() {
            socket.close();
        },
    };
}

function wait(duration) {
    return new Promise((resolve) => setTimeout(resolve, duration));
}

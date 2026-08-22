import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { copyFile, mkdtemp, readFile, rm, stat, writeFile } from "node:fs/promises";
import { createServer as createHTTPServer } from "node:http";
import { createServer as createPortServer } from "node:net";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const rootDir = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const fixturePackage = "./scripts/fixtures/virtual-range-restoration";
const compiler = process.env.GOFRAME_VIRTUAL_RANGE_COMPILER ?? "go";
const chrome = process.env.CHROME ?? "google-chrome";
const goToolchain = process.env.GOTOOLCHAIN ?? "go1.26.6";
const appPort = Number(process.env.GOFRAME_VIRTUAL_RANGE_PORT ?? await pickFreePort());
const debugPort = Number(
    process.env.GOFRAME_VIRTUAL_RANGE_CHROME_DEBUG_PORT ?? await pickFreePort(),
);
const origin = `http://127.0.0.1:${appPort}`;

const normal = { length: 200, height: 192, itemHeight: 48, overscan: 2 };
const short = { ...normal, length: 2 };
const empty = { ...normal, length: 0 };
const expandedHeight = { ...normal, height: 384 };
const expandedOverscan = { ...normal, overscan: 20 };
const changedItemHeight = { ...normal, itemHeight: 40 };
const geometryTolerance = 0.75;

let tempRoot = null;
let profile = null;
let browser = null;
let browserError = "";
let server = null;
let client = null;
const cdpRuntimeErrors = [];

try {
    if (compiler !== "go" && compiler !== "tinygo") {
        throw new Error(`HARNESS FAILURE: unsupported compiler ${JSON.stringify(compiler)}`);
    }
    tempRoot = await mkdtemp(join(tmpdir(), `goframe-virtual-range-${compiler}-`));
    profile = await mkdtemp(join(tmpdir(), "goframe-virtual-range-chrome-"));
    const artifact = await buildFixture();
    server = await startStaticServer(tempRoot, appPort);
    browser = await startBrowser(chrome, [
        "--headless",
        "--no-sandbox",
        "--disable-gpu",
        `--remote-debugging-port=${debugPort}`,
        `--user-data-dir=${profile}`,
        "about:blank",
    ]);
    const page = await waitForPage(debugPort);
    client = await connect(page.webSocketDebuggerUrl);
    client.on("Runtime.exceptionThrown", (params) => {
        cdpRuntimeErrors.push(params.exceptionDetails?.text ?? "runtime exception");
    });
    await client.call("Runtime.enable");
    await client.call("Page.enable");
    await client.call("Page.addScriptToEvaluateOnNewDocument", {
        source: `(${installBrowserAudit.toString()})()`,
    });

    const scenarios = {
        short: await runCollectionScenario("short", "control-short", short),
        empty: await runCollectionScenario("empty", "control-empty", empty),
        height: await runWindowScenario(
            "height",
            "control-height-expand",
            "control-height-reset",
            expandedHeight,
        ),
        overscan: await runWindowScenario(
            "overscan",
            "control-overscan-expand",
            "control-overscan-reset",
            expandedOverscan,
        ),
        itemHeight: await runWindowScenario(
            "item-height",
            "control-item-height-change",
            "control-item-height-reset",
            changedItemHeight,
        ),
    };
    const behaviorJSON = JSON.stringify(scenarios);
    const report = {
        compiler,
        artifact,
        behaviorSha256: createHash("sha256").update(behaviorJSON).digest("hex"),
        scenarios,
    };
    console.log(`virtual range restoration evidence: ${JSON.stringify(report)}`);
    console.log(`virtual range restoration browser smoke (${compiler}): ok`);
} catch (error) {
    throw new Error(`${error.message}\n${JSON.stringify(await diagnostics(), null, 2)}`, {
        cause: error,
    });
} finally {
    client?.close();
    await stopProcess(browser);
    if (server) {
        await new Promise((resolveClose) => server.close(resolveClose));
    }
    if (profile) {
        await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
    }
    if (tempRoot) {
        await rm(tempRoot, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
    }
}

async function runCollectionScenario(label, shrinkControl, shrinkContract) {
    let state = await navigateScenario(label);
    assertViewportState(state, normal, `${label} initial`);
    const initialRootToken = state.rootToken;

    await scrollDistant();
    state = await settledPageState(`${label} distant scroll`);
    assertViewportState(state, normal, `${label} distant scroll`);
    const distant = summarizeViewportState(state);
    const distantListStart = state.listIDs[0];
    const distantTableStart = state.tableIDs[0];

    await clickControl(shrinkControl);
    state = await settledPageState(`${label} shrink`);
    assertViewportState(state, shrinkContract, `${label} shrink`);
    if (state.listScrollTop !== 0 || state.tableScrollTop !== 0) {
        throw new Error(`APP FAILURE: ${label} shrink retained scroll offsets: ${JSON.stringify(state)}`);
    }
    assertCleanupCount(state, "list", distantListStart, 1, `${label} list distant cleanup`);
    assertCleanupCount(state, "table", distantTableStart, 1, `${label} table distant cleanup`);

    await clickControl("control-large");
    state = await settledPageState(`${label} restore`);
    assertViewportState(state, normal, `${label} restore`);
    assert(!state.listIDs.includes(distantListStart), `${label} restored stale list range ${distantListStart}`);
    assert(!state.tableIDs.includes(distantTableStart), `${label} restored stale table range ${distantTableStart}`);
    assert(state.rootToken === initialRootToken, `${label} replaced the application root`);
    const cleanupsBeforeInteraction = {
        list: mapCount(state.cleanups, `list:${distantListStart}`),
        table: mapCount(state.cleanups, `table:${distantTableStart}`),
    };

    const interaction = await exerciseVisibleInteractions(state, label);
    state = interaction.state;
    assertViewportState(state, normal, `${label} interaction`);
    assert(
        mapCount(state.cleanups, `list:${distantListStart}`) === cleanupsBeforeInteraction.list,
        `${label} replayed list cleanup for ${distantListStart}`,
    );
    assert(
        mapCount(state.cleanups, `table:${distantTableStart}`) === cleanupsBeforeInteraction.table,
        `${label} replayed table cleanup for ${distantTableStart}`,
    );
    assertFinalAudit(state, `${label} final`);
    return summarizeScenario(distant, state, interaction.targets);
}

async function runWindowScenario(label, changeControl, resetControl, changedContract) {
    let state = await navigateScenario(label);
    assertViewportState(state, normal, `${label} initial`);
    const initialRootToken = state.rootToken;

    await scrollDistant();
    state = await settledPageState(`${label} distant scroll`);
    assertViewportState(state, normal, `${label} distant scroll`);
    const distant = summarizeViewportState(state);

    await clickControl(changeControl);
    state = await settledPageState(`${label} changed`);
    assertViewportState(state, changedContract, `${label} changed`);
    const expectedChangedScrollTop = Math.min(distant.listScrollTop, maxScrollTop(changedContract));
    assert(
        state.listScrollTop === expectedChangedScrollTop && state.tableScrollTop === expectedChangedScrollTop,
        `${label} changed scrollTop ${state.listScrollTop}/${state.tableScrollTop}, want ${expectedChangedScrollTop}`,
    );
    const changed = summarizeViewportState(state);

    await clickControl(resetControl);
    state = await settledPageState(`${label} restored`);
    assertViewportState(state, normal, `${label} restored`);
    const expectedRestoredScrollTop = Math.min(expectedChangedScrollTop, maxScrollTop(normal));
    assert(
        state.listScrollTop === expectedRestoredScrollTop && state.tableScrollTop === expectedRestoredScrollTop,
        `${label} restored scrollTop ${state.listScrollTop}/${state.tableScrollTop}, want ${expectedRestoredScrollTop}`,
    );
    assert(state.rootToken === initialRootToken, `${label} transition replaced the application root`);

    const interaction = await exerciseVisibleInteractions(state, label);
    state = interaction.state;
    assertViewportState(state, normal, `${label} interaction`);
    assertFinalAudit(state, `${label} final`);
    return summarizeScenario(distant, state, interaction.targets, changed);
}

async function navigateScenario(label) {
    cdpRuntimeErrors.length = 0;
    await client.call("Page.navigate", {
        url: `${origin}/?scenario=${encodeURIComponent(label)}&run=${Date.now()}`,
    });
    for (let attempt = 0; attempt < 150; attempt++) {
        const ready = await client.evaluate(`(() => ({
            href: location.href,
            ready: Boolean(document.querySelector("[data-testid='virtual-range-fixture']")),
            fixture: Boolean(window.__virtualRangeFixture),
        }))()`);
        if (ready.href.startsWith("chrome-error://")) {
            throw new Error(`HARNESS FAILURE: ${label} loaded a Chrome error document`);
        }
        if (ready.ready && ready.fixture) {
            await settle();
            await client.evaluate(`(() => {
                const audit = window.__virtualRangeBrowserAudit;
                audit.listenerBaseline = audit.listenerBalance;
                audit.appRoot = document.querySelector("[data-testid='virtual-range-fixture']");
                audit.rootToken = ${JSON.stringify(label)} + ":" + Date.now();
                audit.appRoot.setAttribute("data-root-token", audit.rootToken);
            })()`);
            return await settledPageState(`${label} initial`);
        }
        await wait(100);
    }
    throw new Error(`HARNESS FAILURE: ${label} fixture did not become ready`);
}

async function scrollDistant() {
    await client.evaluate(`(() => {
        for (const selector of ["[data-testid='fixture-virtual-list']", "[data-testid='fixture-virtual-table']"]) {
            const viewport = document.querySelector(selector);
            viewport.scrollTop = 100000;
            viewport.dispatchEvent(new Event("scroll", { bubbles: true }));
        }
    })()`);
    await settle();
}

async function exerciseVisibleInteractions(state, label) {
    const listSelect = visibleInteractionID(state.geometry.list.visibleIDs, 0, `${label} list select`);
    const listToggle = visibleInteractionID(state.geometry.list.visibleIDs, 1, `${label} list toggle`);
    const tableSelect = visibleInteractionID(state.geometry.table.visibleIDs, 0, `${label} table select`);
    const tableToggle = visibleInteractionID(state.geometry.table.visibleIDs, 1, `${label} table toggle`);
    const targets = [];
    targets.push(await pointerClickVisible(`fixture-list-select-${listSelect}`, "fixture-virtual-list"));
    targets.push(await pointerClickVisible(`fixture-list-toggle-${listToggle}`, "fixture-virtual-list"));
    targets.push(await pointerClickVisible(`fixture-table-select-${tableSelect}`, "fixture-virtual-table"));
    targets.push(await pointerClickVisible(`fixture-table-toggle-${tableToggle}`, "fixture-virtual-table"));
    state = await settledPageState(`${label} interactions`);
    assert(state.listSelection === `list-selected:${listSelect}`, `list selection targeted wrong item: ${JSON.stringify(state)}`);
    assert(state.listToggle === `list-toggled:${listToggle}`, `list toggle targeted wrong item: ${JSON.stringify(state)}`);
    assert(state.tableSelection === `table-selected:${tableSelect}`, `table selection targeted wrong item: ${JSON.stringify(state)}`);
    assert(state.tableToggle === `table-toggled:${tableToggle}`, `table toggle targeted wrong item: ${JSON.stringify(state)}`);
    assertDeepEqual(
        state.interactions.slice(-4),
        [
            `list:select:${listSelect}`,
            `list:toggle:${listToggle}`,
            `table:select:${tableSelect}`,
            `table:toggle:${tableToggle}`,
        ],
        "restored interaction targets",
    );
    return { state, targets };
}

function visibleInteractionID(visibleIDs, offset, label) {
    if (visibleIDs.length <= offset) {
        throw new Error(`APP FAILURE: ${label} has no visible target: ${JSON.stringify(visibleIDs)}`);
    }
    return visibleIDs[offset];
}

async function clickControl(testID) {
    await client.evaluate(`(() => {
        const target = document.querySelector(${JSON.stringify(`[data-testid='${testID}']`)});
        if (!target) throw new Error(${JSON.stringify(`missing ${testID}`)});
        target.click();
    })()`);
    await settle();
}

async function pointerClickVisible(testID, viewportTestID) {
    const target = await client.evaluate(`(() => {
        const element = document.querySelector(${JSON.stringify(`[data-testid='${testID}']`)});
        const viewport = document.querySelector(${JSON.stringify(`[data-testid='${viewportTestID}']`)});
        if (!element || !viewport) throw new Error(${JSON.stringify(`missing visible target ${testID}`)});
        let rect = element.getBoundingClientRect();
        let viewportRect = viewport.getBoundingClientRect();
        let top = Math.max(rect.top, viewportRect.top);
        let bottom = Math.min(rect.bottom, viewportRect.bottom);
        const initialY = (top + bottom) / 2;
        if (initialY < 0 || initialY > window.innerHeight) {
            window.scrollBy(0, initialY - window.innerHeight / 2);
            rect = element.getBoundingClientRect();
            viewportRect = viewport.getBoundingClientRect();
            top = Math.max(rect.top, viewportRect.top);
            bottom = Math.min(rect.bottom, viewportRect.bottom);
        }
        const left = Math.max(rect.left, viewportRect.left);
        const right = Math.min(rect.right, viewportRect.right);
        if (rect.width <= 0 || rect.height <= 0 || right <= left || bottom <= top) {
            throw new Error(${JSON.stringify(`${testID} does not intersect ${viewportTestID}`)});
        }
        const x = (left + right) / 2;
        const y = (top + bottom) / 2;
        const hit = document.elementFromPoint(x, y);
        if (!hit || (hit !== element && !element.contains(hit))) {
            throw new Error(${JSON.stringify(`${testID} is not hit-testable at its visible center`)} +
                ": target=" + element.tagName +
                " rect=" + JSON.stringify({ left: rect.left, right: rect.right, top: rect.top, bottom: rect.bottom }) +
                " point=" + JSON.stringify({ x, y }) +
                " hit=" + (hit?.tagName ?? "none") +
                " hitTestID=" + (hit?.getAttribute?.("data-testid") ?? ""));
        }
        return {
            testID: ${JSON.stringify(testID)},
            viewportTestID: ${JSON.stringify(viewportTestID)},
            x,
            y,
            width: rect.width,
            height: rect.height,
        };
    })()`);
    await client.call("Input.dispatchMouseEvent", {
        type: "mousePressed",
        x: target.x,
        y: target.y,
        button: "left",
        clickCount: 1,
    });
    await client.call("Input.dispatchMouseEvent", {
        type: "mouseReleased",
        x: target.x,
        y: target.y,
        button: "left",
        clickCount: 1,
    });
    await settle();
    return {
        testID: target.testID,
        viewportTestID: target.viewportTestID,
        width: round(target.width),
        height: round(target.height),
    };
}

async function settle() {
    await client.evaluate(`new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)))`);
    await wait(30);
}

async function settledPageState(label) {
    let previous = "";
    for (let attempt = 0; attempt < 20; attempt++) {
        await settle();
        const state = await pageState();
        const signature = JSON.stringify({
            contract: [state.length, state.height, state.itemHeight, state.overscan],
            list: [state.listScrollTop, state.listIDs, state.geometry.list],
            table: [state.tableScrollTop, state.tableIDs, state.geometry.table],
            interactions: state.interactions,
        });
        if (signature === previous) return state;
        previous = signature;
    }
    throw new Error(`HARNESS FAILURE: ${label} did not reach stable viewport geometry`);
}

async function pageState() {
    const state = await client.evaluate(`(() => {
        const fixture = window.__virtualRangeFixture;
        const browserAudit = window.__virtualRangeBrowserAudit;
        const listViewport = document.querySelector("[data-testid='fixture-virtual-list']");
        const tableViewport = document.querySelector("[data-testid='fixture-virtual-table']");
        const listNodes = [...document.querySelectorAll("[data-testid^='fixture-list-item-']")];
        const tableNodes = [...document.querySelectorAll("[data-testid^='fixture-table-row-']")];
        const testIDs = [...document.querySelectorAll("[data-testid]")].map((node) => node.getAttribute("data-testid"));
        const duplicateTestIDs = testIDs.filter((value, index) => testIDs.indexOf(value) !== index);
        const app = document.querySelector("[data-testid='virtual-range-fixture']");
        const listFirst = listNodes[0]?.closest(".gf-virtual-item");
        const listLast = listNodes.at(-1)?.closest(".gf-virtual-item");
        const topRow = document.querySelector(".gf-virtual-table-spacer-top");
        const bottomRow = document.querySelector(".gf-virtual-table-spacer-bottom");
        const table = tableViewport?.querySelector("table");
        const roundedRect = (node) => {
            if (!node) return null;
            const rect = node.getBoundingClientRect();
            return {
                top: Math.round(rect.top * 1000) / 1000,
                bottom: Math.round(rect.bottom * 1000) / 1000,
                height: Math.round(rect.height * 1000) / 1000,
            };
        };
        const listRecords = listNodes.map((node) => {
            const item = node.closest(".gf-virtual-item");
            return {
                id: Number(node.getAttribute("data-testid").replace("fixture-list-item-", "")),
                ...roundedRect(item),
            };
        });
        const tableRecords = tableNodes.map((node) => ({
            id: Number(node.getAttribute("data-testid").replace("fixture-table-row-", "")),
            ...roundedRect(node),
        }));
        const listViewportRect = roundedRect(listViewport);
        const tableViewportRect = roundedRect(tableViewport);
        const visibleIDs = (records, viewportRect) => records
            .filter((record) => record.bottom > viewportRect.top && record.top < viewportRect.bottom)
            .map((record) => record.id);
        return {
            length: Number(app?.getAttribute("data-length") ?? -1),
            height: Number(app?.getAttribute("data-height") ?? -1),
            itemHeight: Number(app?.getAttribute("data-item-height") ?? -1),
            overscan: Number(app?.getAttribute("data-overscan") ?? -1),
            listIDs: listNodes.map((node) => Number(node.getAttribute("data-testid").replace("fixture-list-item-", ""))),
            tableIDs: tableNodes.map((node) => Number(node.getAttribute("data-testid").replace("fixture-table-row-", ""))),
            listScrollTop: listViewport?.scrollTop ?? -1,
            tableScrollTop: tableViewport?.scrollTop ?? -1,
            listTop: Number.parseInt(listFirst?.style.top || "0", 10),
            listTotalHeight: Number.parseInt(document.querySelector(".gf-virtual-list-spacer")?.style.height || "0", 10),
            tableTop: Number.parseInt(topRow?.style.height || "0", 10),
            tableBottom: Number.parseInt(bottomRow?.style.height || "0", 10),
            geometry: {
                list: {
                    viewport: listViewportRect,
                    clientHeight: listViewport?.clientHeight ?? -1,
                    scrollHeight: listViewport?.scrollHeight ?? -1,
                    first: roundedRect(listFirst),
                    last: roundedRect(listLast),
                    spacer: roundedRect(document.querySelector(".gf-virtual-list-spacer")),
                    itemHeights: listRecords.map((record) => record.height),
                    visibleIDs: visibleIDs(listRecords, listViewportRect),
                },
                table: {
                    viewport: tableViewportRect,
                    clientHeight: tableViewport?.clientHeight ?? -1,
                    scrollHeight: tableViewport?.scrollHeight ?? -1,
                    table: roundedRect(table),
                    first: roundedRect(tableNodes[0]),
                    last: roundedRect(tableNodes.at(-1)),
                    rowHeights: tableRecords.map((record) => record.height),
                    visibleIDs: visibleIDs(tableRecords, tableViewportRect),
                    topSpacer: roundedRect(topRow),
                    bottomSpacer: roundedRect(bottomRow),
                },
            },
            listSelection: document.querySelector("[data-testid='list-selection']")?.textContent ?? "",
            listToggle: document.querySelector("[data-testid='list-toggle']")?.textContent ?? "",
            tableSelection: document.querySelector("[data-testid='table-selection']")?.textContent ?? "",
            tableToggle: document.querySelector("[data-testid='table-toggle']")?.textContent ?? "",
            mounts: { ...fixture.mounts },
            cleanups: { ...fixture.cleanups },
            interactions: [...fixture.interactions],
            runtimeErrors: [...fixture.runtimeErrors],
            appMounts: fixture.appMounts,
            appCleanups: fixture.appCleanups,
            listenerDelta: browserAudit.listenerBalance - browserAudit.listenerBaseline,
            browserErrors: [...browserAudit.errors],
            rootStable: browserAudit.appRoot === app,
            rootToken: app?.getAttribute("data-root-token") ?? "",
            duplicateTestIDs,
            duplicateKeyWarnings: [...(globalThis.goframeDuplicateKeyWarnings || [])],
        };
    })()`);
    state.cdpRuntimeErrors = [...cdpRuntimeErrors];
    return state;
}

function assertViewportState(state, contract, label) {
    assertDeepEqual(
        {
            length: state.length,
            height: state.height,
            itemHeight: state.itemHeight,
            overscan: state.overscan,
        },
        contract,
        `${label} contract`,
    );
    assertContiguousIDs(state.listIDs, `${label} list IDs`);
    assertContiguousIDs(state.tableIDs, `${label} table IDs`);
    assertDeepEqual(state.listIDs, state.tableIDs, `${label} list/table parity`);
    const start = state.listIDs[0] ?? 0;
    const end = (state.listIDs.at(-1) ?? -1) + 1;
    assert(state.listTop === start * contract.itemHeight, `${label} list top ${state.listTop}`);
    assert(state.listTotalHeight === contract.length * contract.itemHeight, `${label} list total ${state.listTotalHeight}`);
    assert(state.tableTop === start * contract.itemHeight, `${label} table top ${state.tableTop}`);
    assert(
        state.tableBottom === (contract.length - end) * contract.itemHeight,
        `${label} table bottom ${state.tableBottom}`,
    );
    assert(state.listIDs.length <= maximumWindowSize(contract), `${label} list mount bound`);
    assert(state.tableIDs.length <= maximumWindowSize(contract), `${label} table mount bound`);
    assertGeometry(state, contract, start, end, label);
    assert(state.duplicateTestIDs.length === 0, `${label} duplicate test IDs ${JSON.stringify(state.duplicateTestIDs)}`);
    assert(state.duplicateKeyWarnings.length === 0, `${label} duplicate key warnings ${JSON.stringify(state.duplicateKeyWarnings)}`);
    assert(state.runtimeErrors.length === 0, `${label} runtime errors ${JSON.stringify(state.runtimeErrors)}`);
    assert(state.browserErrors.length === 0, `${label} browser errors ${JSON.stringify(state.browserErrors)}`);
    assert(state.cdpRuntimeErrors.length === 0, `${label} CDP runtime errors ${JSON.stringify(state.cdpRuntimeErrors)}`);
    assert(state.appMounts === 1 && state.appCleanups === 0, `${label} app lifetime ${state.appMounts}/${state.appCleanups}`);
    assert(state.rootStable, `${label} application root identity changed`);
}

function assertGeometry(state, contract, renderedStart, renderedEnd, label) {
    const expectedScrollHeight = Math.max(contract.height, contract.length * contract.itemHeight);
    for (const [kind, geometry] of Object.entries(state.geometry)) {
        assertApprox(geometry.clientHeight, contract.height, `${label} ${kind} clientHeight`);
        if (contract.length === 0) {
            assert((kind === "list" ? state.listScrollTop : state.tableScrollTop) === 0, `${label} ${kind} empty scrollTop`);
            assert((kind === "list" ? state.listIDs : state.tableIDs).length === 0, `${label} ${kind} empty rendered IDs`);
            continue;
        }
        assertApprox(geometry.scrollHeight, expectedScrollHeight, `${label} ${kind} scrollHeight`);
        const heights = kind === "list" ? geometry.itemHeights : geometry.rowHeights;
        for (const height of heights) {
            assertApprox(height, contract.itemHeight, `${label} ${kind} rendered height`);
        }
        const scrollTop = kind === "list" ? state.listScrollTop : state.tableScrollTop;
        const visibleStart = Math.min(contract.length, Math.floor(scrollTop / contract.itemHeight));
        const visibleEnd = Math.min(
            contract.length,
            Math.ceil((scrollTop + geometry.clientHeight) / contract.itemHeight),
        );
        assert(
            renderedStart <= visibleStart && renderedEnd >= visibleEnd,
            `${label} ${kind} rendered [${renderedStart},${renderedEnd}) does not cover viewport [${visibleStart},${visibleEnd}) at scrollTop ${scrollTop}`,
        );
        assert(geometry.visibleIDs.length > 0, `${label} ${kind} viewport contains no rendered item`);
        const contentBottom = Math.min(
            geometry.viewport.bottom,
            geometry.viewport.top + Math.max(0, contract.length * contract.itemHeight - scrollTop),
        );
        assert(
            geometry.first.top <= geometry.viewport.top + geometryTolerance,
            `${label} ${kind} first rendered top ${geometry.first.top} leaves blank viewport at ${geometry.viewport.top}`,
        );
        assert(
            geometry.last.bottom >= contentBottom - geometryTolerance,
            `${label} ${kind} last rendered bottom ${geometry.last.bottom} leaves blank viewport before ${contentBottom}`,
        );
    }
    if (contract.length === 0) return;
    assertApprox(state.geometry.list.spacer.height, contract.length * contract.itemHeight, `${label} list spacer height`);
    assertApprox(state.geometry.table.table.height, contract.length * contract.itemHeight, `${label} table logical height`);
    assertApprox(state.geometry.table.topSpacer.height, renderedStart * contract.itemHeight, `${label} table top spacer height`);
    assertApprox(
        state.geometry.table.bottomSpacer.height,
        (contract.length - renderedEnd) * contract.itemHeight,
        `${label} table bottom spacer height`,
    );
}

function assertContiguousIDs(ids, label) {
    for (let index = 1; index < ids.length; index++) {
        assert(ids[index] === ids[index - 1] + 1, `${label} are not contiguous: ${JSON.stringify(ids)}`);
    }
}

function assertApprox(actual, expected, label) {
    assert(
        Math.abs(actual - expected) <= geometryTolerance,
        `${label} ${actual}, want ${expected} within ${geometryTolerance}px`,
    );
}

function assertFinalAudit(state, label) {
    assert(state.listenerDelta === 0, `${label} listener delta ${state.listenerDelta}`);
    for (const [key, cleanups] of Object.entries(state.cleanups)) {
        assert(cleanups <= mapCount(state.mounts, key), `${label} cleanup exceeds mounts for ${key}`);
    }
}

function assertCleanupCount(state, kind, id, expected, label) {
    const key = `${kind}:${id}`;
    assert(mapCount(state.mounts, key) === 1, `${label} mounts ${mapCount(state.mounts, key)}, want 1`);
    assert(mapCount(state.cleanups, key) === expected, `${label} cleanups ${mapCount(state.cleanups, key)}, want ${expected}`);
}

function summarizeScenario(distant, finalState, interactionTargets, changed = null) {
    return {
        distant,
        ...(changed ? { changed } : {}),
        final: summarizeViewportState(finalState),
        interactionTargets,
    };
}

function summarizeViewportState(state) {
    return {
        listIDs: state.listIDs,
        tableIDs: state.tableIDs,
        listScrollTop: state.listScrollTop,
        tableScrollTop: state.tableScrollTop,
        listVisibleIDs: state.geometry.list.visibleIDs,
        tableVisibleIDs: state.geometry.table.visibleIDs,
        listClientHeight: state.geometry.list.clientHeight,
        tableClientHeight: state.geometry.table.clientHeight,
        listScrollHeight: state.geometry.list.scrollHeight,
        tableScrollHeight: state.geometry.table.scrollHeight,
        listItemHeights: [...new Set(state.geometry.list.itemHeights)],
        tableRowHeights: [...new Set(state.geometry.table.rowHeights)],
        tableTopSpacer: state.geometry.table.topSpacer?.height ?? 0,
        tableBottomSpacer: state.geometry.table.bottomSpacer?.height ?? 0,
        interactions: state.interactions,
        listenerDelta: state.listenerDelta,
        appMounts: state.appMounts,
        appCleanups: state.appCleanups,
        runtimeErrors: state.runtimeErrors.length + state.browserErrors.length + state.cdpRuntimeErrors.length,
    };
}

function maximumWindowSize(contract) {
    if (contract.length <= 0) return 0;
    const visible = Math.ceil((contract.height + contract.itemHeight - 1) / contract.itemHeight);
    return Math.min(contract.length, visible + 2 * Math.max(0, contract.overscan));
}

function maxScrollTop(contract) {
    return Math.max(0, contract.length * contract.itemHeight - contract.height);
}

function mapCount(map, key) {
    return Number(map[key] ?? 0);
}

function round(value) {
    return Math.round(value * 1000) / 1000;
}

async function buildFixture() {
    const wasm = join(tempRoot, "bundle.wasm");
    const commonEnvironment = {
        ...process.env,
        GOCACHE: join(tempRoot, "gocache"),
        GOWORK: "off",
        GOFLAGS: "-buildvcs=false",
        GOTOOLCHAIN: goToolchain,
        XDG_CACHE_HOME: join(tempRoot, "xdg-cache"),
    };
    let runtimeSource;
    if (compiler === "go") {
        await runCommand("go", [
            "build",
            "-tags=goframe_debug",
            "-o",
            wasm,
            fixturePackage,
        ], {
            ...commonEnvironment,
            GOOS: "js",
            GOARCH: "wasm",
            CGO_ENABLED: "0",
        });
        const goRoot = (await runCommand("go", ["env", "GOROOT"], commonEnvironment)).trim();
        runtimeSource = await firstExistingPath([
            join(goRoot, "lib", "wasm", "wasm_exec.js"),
            join(goRoot, "misc", "wasm", "wasm_exec.js"),
        ]);
    } else {
        await runCommand("tinygo", [
            "build",
            "-target=wasm",
            "-no-debug",
            "-panic=trap",
            "-tags=goframe_debug",
            "-o",
            wasm,
            fixturePackage,
        ], commonEnvironment);
        const tinyGoRoot = (await runCommand("tinygo", ["env", "TINYGOROOT"], commonEnvironment)).trim();
        runtimeSource = join(tinyGoRoot, "targets", "wasm_exec.js");
    }
    await copyFile(runtimeSource, join(tempRoot, "wasm_exec.js"));
    await writeFile(join(tempRoot, "index.html"), fixtureHTML(), "utf8");
    const bytes = await readFile(wasm);
    return {
        bytes: bytes.length,
        sha256: createHash("sha256").update(bytes).digest("hex"),
    };
}

async function firstExistingPath(paths) {
    for (const candidate of paths) {
        try {
            if ((await stat(candidate)).isFile()) return candidate;
        } catch {
            // Try the next supported Go distribution layout.
        }
    }
    throw new Error(`HARNESS FAILURE: wasm_exec.js not found in ${paths.join(", ")}`);
}

function startStaticServer(directory, port) {
    const server = createHTTPServer(async (request, response) => {
        try {
            const pathname = new URL(request.url, origin).pathname;
            const name = pathname === "/" ? "index.html" : pathname.slice(1);
            if (!new Set(["index.html", "wasm_exec.js", "bundle.wasm"]).has(name)) {
                response.writeHead(404).end("not found");
                return;
            }
            const content = await readFile(join(directory, name));
            const mime = name.endsWith(".wasm")
                ? "application/wasm"
                : name.endsWith(".js")
                    ? "text/javascript"
                    : "text/html";
            response.writeHead(200, { "content-type": mime });
            response.end(content);
        } catch (error) {
            response.writeHead(500).end(error.message);
        }
    });
    return new Promise((resolveServer, reject) => {
        server.once("error", reject);
        server.listen(port, "127.0.0.1", () => resolveServer(server));
    });
}

function installBrowserAudit() {
    let listenerBalance = 0;
    const errors = [];
    const originalAdd = EventTarget.prototype.addEventListener;
    const originalRemove = EventTarget.prototype.removeEventListener;
    EventTarget.prototype.addEventListener = function(...args) {
        listenerBalance++;
        return originalAdd.apply(this, args);
    };
    EventTarget.prototype.removeEventListener = function(...args) {
        listenerBalance--;
        return originalRemove.apply(this, args);
    };
    window.__virtualRangeBrowserAudit = {
        appRoot: null,
        rootToken: "",
        listenerBaseline: 0,
        errors,
        get listenerBalance() {
            return listenerBalance;
        },
    };
    window.addEventListener("error", (event) => {
        errors.push(event.message || "window error");
    });
    window.addEventListener("unhandledrejection", (event) => {
        errors.push(String(event.reason));
    });
}

async function diagnostics() {
    const result = {
        compiler,
        appPort,
        debugPort,
        browserStderr: browserError.slice(-6000),
        cdpRuntimeErrors,
    };
    if (client) {
        try {
            result.page = await pageState();
        } catch (error) {
            result.pageError = error.message;
        }
    }
    return result;
}

function startBrowser(command, args) {
    return new Promise((resolveBrowser, reject) => {
        const child = spawn(command, args, { stdio: ["ignore", "ignore", "pipe"] });
        child.stderr.on("data", (chunk) => {
            browserError += chunk;
        });
        child.once("error", reject);
        child.once("spawn", () => resolveBrowser(child));
    });
}

async function waitForPage(port) {
    let lastError = null;
    for (let attempt = 0; attempt < 100; attempt++) {
        if (browser?.exitCode !== null) {
            throw new Error(`HARNESS FAILURE: Chrome exited before CDP was ready\n${browserError}`);
        }
        try {
            const response = await fetch(`http://127.0.0.1:${port}/json`);
            const targets = await response.json();
            const page = targets.find((entry) => entry.type === "page" && entry.webSocketDebuggerUrl);
            if (page) return page;
        } catch (error) {
            lastError = error;
        }
        await wait(100);
    }
    throw new Error(`HARNESS FAILURE: Chrome DevTools did not become ready: ${lastError?.message ?? ""}`);
}

async function connect(url) {
    const socket = new WebSocket(url);
    await new Promise((resolveOpen, reject) => {
        socket.addEventListener("open", resolveOpen, { once: true });
        socket.addEventListener("error", reject, { once: true });
    });
    let nextID = 1;
    let closedError = null;
    const pending = new Map();
    const listeners = new Map();
    const terminate = (event) => {
        if (closedError) return;
        closedError = new Error(`HARNESS FAILURE: CDP socket terminated: ${event.type}`);
        for (const request of pending.values()) request.reject(closedError);
        pending.clear();
    };
    socket.addEventListener("close", terminate);
    socket.addEventListener("error", terminate);
    socket.addEventListener("message", (event) => {
        if (closedError) return;
        const message = JSON.parse(event.data);
        if (message.id && pending.has(message.id)) {
            const request = pending.get(message.id);
            pending.delete(message.id);
            if (message.error) request.reject(new Error(message.error.message));
            else request.resolve(message.result);
            return;
        }
        for (const listener of listeners.get(message.method) ?? []) {
            listener(message.params ?? {});
        }
    });
    return {
        call(method, params = {}) {
            if (closedError) return Promise.reject(closedError);
            return new Promise((resolveCall, reject) => {
                const id = nextID++;
                pending.set(id, { resolve: resolveCall, reject });
                try {
                    socket.send(JSON.stringify({ id, method, params }));
                } catch (error) {
                    pending.delete(id);
                    reject(error);
                    terminate({ type: "send" });
                }
            });
        },
        async evaluate(expression) {
            const result = await this.call("Runtime.evaluate", {
                expression,
                returnByValue: true,
                awaitPromise: true,
            });
            if (result.exceptionDetails) {
                throw new Error(`APP FAILURE: browser evaluation failed: ${JSON.stringify(result.exceptionDetails)}`);
            }
            return result.result.value;
        },
        on(method, listener) {
            const current = listeners.get(method) ?? [];
            current.push(listener);
            listeners.set(method, current);
        },
        close() {
            if (!closedError) socket.close();
        },
    };
}

function runCommand(command, args, environment) {
    return new Promise((resolveCommand, reject) => {
        const child = spawn(command, args, {
            cwd: rootDir,
            stdio: ["ignore", "pipe", "pipe"],
            env: environment,
        });
        let output = "";
        child.stdout.on("data", (chunk) => {
            output += chunk;
        });
        child.stderr.on("data", (chunk) => {
            output += chunk;
        });
        child.once("error", reject);
        child.once("exit", (code, signal) => {
            if (code === 0) {
                resolveCommand(output);
                return;
            }
            reject(new Error(
                `HARNESS FAILURE: ${command} ${args.join(" ")} failed with ${signal ?? code}\n${output}`,
            ));
        });
    });
}

async function stopProcess(child) {
    if (!child || child.exitCode !== null || child.signalCode !== null) return;
    const exited = new Promise((resolveExit) => child.once("exit", resolveExit));
    child.kill("SIGTERM");
    const stopped = await Promise.race([
        exited.then(() => true),
        wait(2000).then(() => false),
    ]);
    if (!stopped && child.exitCode === null && child.signalCode === null) {
        child.kill("SIGKILL");
        await Promise.race([exited, wait(1000)]);
    }
}

function pickFreePort() {
    return new Promise((resolvePort, reject) => {
        const server = createPortServer();
        server.once("error", reject);
        server.listen(0, "127.0.0.1", () => {
            const address = server.address();
            server.close(() => resolvePort(address.port));
        });
    });
}

function assert(condition, message) {
    if (!condition) throw new Error(`APP FAILURE: ${message}`);
}

function assertDeepEqual(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        throw new Error(`APP FAILURE: ${label}: got ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`);
    }
}

function wait(milliseconds) {
    return new Promise((resolveWait) => setTimeout(resolveWait, milliseconds));
}

function fixtureHTML() {
    return `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>Virtual range restoration fixture</title>
  <style>
    body { font-family: sans-serif; margin: 0; }
    main { padding: 12px; }
    #root { min-height: 100vh; }
    button { box-sizing: border-box; height: 24px; line-height: 20px; padding: 1px 6px; }
    .gf-virtual-list, .gf-virtual-table-viewport { margin: 8px 0; outline: 1px solid #888; }
    .fixture-list-item {
      align-items: center;
      box-sizing: border-box;
      display: flex;
      gap: 4px;
      height: 100%;
    }
    .gf-virtual-table { border-collapse: collapse; border-spacing: 0; table-layout: fixed; width: 100%; }
    .fixture-table-row { box-sizing: border-box; }
    .fixture-table-row > td { border: 0; box-sizing: border-box; line-height: 1; padding: 0; }
  </style>
</head>
<body>
  <div id="root"></div>
  <script src="wasm_exec.js"></script>
  <script>
    const go = new Go();
    WebAssembly.instantiateStreaming(fetch("bundle.wasm"), go.importObject)
      .then((result) => go.run(result.instance))
      .catch((error) => { throw error; });
  </script>
</body>
</html>`;
}

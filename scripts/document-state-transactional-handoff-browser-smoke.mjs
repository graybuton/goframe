import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { copyFile, mkdtemp, readFile, rm, stat } from "node:fs/promises";
import { createServer as createHTTPServer } from "node:http";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { createServer as createPortServer } from "node:net";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const rootDir = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const fixtureDir = join(rootDir, "scripts", "fixtures", "document-state-api-design");
const fixturePackage = "./scripts/fixtures/document-state-api-design/cmd/handoff";
const compiler = process.env.GOFRAME_DOCUMENT_HANDOFF_COMPILER ?? "go";
const chrome = process.env.CHROME ?? "google-chrome";
const goToolchain = process.env.GOTOOLCHAIN ?? "go1.26.5";
const appPort = Number(
    process.env.GOFRAME_DOCUMENT_HANDOFF_PORT ?? await pickFreePort(),
);
const debugPort = Number(
    process.env.GOFRAME_DOCUMENT_HANDOFF_CHROME_DEBUG_PORT ?? await pickFreePort(),
);
const origin = `http://127.0.0.1:${appPort}`;

const baseline = pair(
    "GoFrame document API design fixture",
    "Authored document API design baseline",
);
const metadataA = pair("Owner A · GoFrame", "Description A");
const metadataA2 = pair("Owner A2 · GoFrame", "Description A2");
const metadataB = pair("Owner B · GoFrame", "Description B");
const metadataC = pair("Owner C · GoFrame", "Description C");
const failureMetadata = pair(
    "Failed owner · GoFrame",
    "Failed owner description",
);
const validPairs = [baseline, metadataA, metadataA2, metadataB, metadataC, failureMetadata];

let tempRoot = null;
let profile = null;
let browser = null;
let browserError = "";
let server = null;
let client = null;

try {
    if (compiler !== "go" && compiler !== "tinygo") {
        throw new Error(`HARNESS FAILURE: unsupported compiler ${JSON.stringify(compiler)}`);
    }
    tempRoot = await mkdtemp(join(tmpdir(), `goframe-document-handoff-${compiler}-`));
    profile = await mkdtemp(join(tmpdir(), "goframe-document-handoff-chrome-"));
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
    await client.call("Runtime.enable");
    await client.call("Page.enable");
    await client.call("Page.addScriptToEvaluateOnNewDocument", {
        source: `(${installHeadObserver.toString()})()`,
    });

    const results = {};
    results.direct = await runDirectReplacement();
    results.nested = await runNestedPriority();
    results.nonselected = await runNonselectedRelease();
    results.sameValue = await runSameValueUpdate();
    results.multiple = await runMultipleOperations();
    results.lifetime = await runLifetime();
    results.repeatedMount = await runRepeatedMount();
    results.teardown = await runApplicationTeardown();
    if (compiler === "go") {
        results.failedInitial = await runFailedInitial();
        results.failedReplacement = await runFailedReplacementAndRetry();
    }

    const report = {
        compiler,
        artifact,
        results,
    };
    console.log(`document state transactional handoff evidence: ${JSON.stringify(report)}`);
    console.log(`document state transactional handoff browser smoke (${compiler}): ok`);
} catch (error) {
    throw new Error(`${error.message}\n${JSON.stringify(await diagnostics(), null, 2)}`, {
        cause: error,
    });
} finally {
    client?.close();
    await stopProcess(browser, false);
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

async function runDirectReplacement() {
    let state = await navigateScenario("direct");
    assertCommitted(state, metadataA, 1, "direct initial owner");
    assert(state.runtime.statistics.committedIDAssignments === 1, "direct initial owner id");
    assert(state.runtime.ownerRenders.length === 1, "direct initial owner required one render");
    const before = boundary(state);
    const frames = await clickAndCapture("replace-owner");
    state = await pageState();
    assertCommitted(state, metadataB, 1, "direct replacement");
    assertNoInvalidPairs(state, "direct replacement");
    assertSequence(frames, [metadataA, metadataB], "direct replacement frames");
    assertSequence(observerSequenceSince(state, before), [metadataA, metadataB], "direct replacement observer");
    assertDeltas(state, before, { title: 1, description: 1, applies: 1, baseline: 0 }, "direct replacement");
    assert(state.runtime.statistics.committedIDAssignments === 2, "direct replacement owner id assignment");
    assertIdentity(state, "direct replacement");
    return resultSummary(state, frames);
}

async function runNestedPriority() {
    let state = await navigateScenario("nested");
    assertCommitted(state, metadataB, 2, "nested initial override");
    assert(state.runtime.documentApplies === 1, "nested initial owners must coalesce to one publication");

    let before = boundary(state);
    let frames = await clickAndCapture("update-parent");
    state = await pageState();
    assertCommitted(state, metadataB, 2, "parent update beneath override");
    assertSequence(frames, [metadataB], "parent update frames");
    assertDeltas(state, before, { title: 0, description: 0, applies: 0, baseline: 0 }, "parent update");

    before = boundary(state);
    frames = await clickAndCapture("release-nested");
    state = await pageState();
    assertCommitted(state, metadataA2, 1, "nested release reveals parent");
    assertSequence(frames, [metadataB, metadataA2], "nested release frames");
    assertSequence(observerSequenceSince(state, before), [metadataB, metadataA2], "nested release observer");
    assertDeltas(state, before, { title: 1, description: 1, applies: 1, baseline: 0 }, "nested release");

    before = boundary(state);
    frames = await clickAndCapture("release-parent");
    state = await pageState();
    assertCommitted(state, baseline, 0, "final owner release");
    assertSequence(frames, [metadataA2, baseline], "final release frames");
    assertDeltas(state, before, { title: 1, description: 1, applies: 1, baseline: 1 }, "final release");
    assertIdentity(state, "nested lifecycle");
    return resultSummary(state, frames);
}

async function runNonselectedRelease() {
    let state = await navigateScenario("nonselected");
    assertCommitted(state, metadataB, 2, "non-selected initial owners");
    const before = boundary(state);
    const frames = await clickAndCapture("release-nonselected");
    state = await pageState();
    assertCommitted(state, metadataB, 1, "non-selected release");
    assertSequence(frames, [metadataB], "non-selected release frames");
    assertDeltas(state, before, { title: 0, description: 0, applies: 0, baseline: 0 }, "non-selected release");
    return resultSummary(state, frames);
}

async function runSameValueUpdate() {
    let state = await navigateScenario("same-value");
    assertCommitted(state, metadataA, 1, "same-value initial owner");
    const before = boundary(state);
    const frames = await clickAndCapture("publish-same");
    state = await pageState();
    assertCommitted(state, metadataA, 1, "same-value publication");
    assertSequence(frames, [metadataA], "same-value frames");
    assertDeltas(state, before, { title: 0, description: 0, applies: 0, baseline: 0 }, "same-value publication");
    assert(
        state.runtime.statistics.duplicatePublications ===
            before.statistics.duplicatePublications + 1,
        "same-value publication was not classified as duplicate",
    );
    assert(state.runtime.statistics.committedIDAssignments === 1, "same-value update changed owner identity");
    return resultSummary(state, frames);
}

async function runMultipleOperations() {
    let state = await navigateScenario("multiple");
    assertCommitted(state, metadataB, 2, "multiple initial owners");
    const before = boundary(state);
    const frames = await clickAndCapture("run-multiple");
    state = await pageState();
    assertCommitted(state, metadataC, 2, "mixed operation result");
    assertSequence(frames, [metadataB, metadataC], "mixed operation frames");
    assertSequence(observerSequenceSince(state, before), [metadataB, metadataC], "mixed operation observer");
    assertDeltas(state, before, { title: 1, description: 1, applies: 1, baseline: 0 }, "mixed operation");
    assert(state.runtime.statistics.committedIDAssignments === 3, "mixed operation owner identity order");
    return resultSummary(state, frames);
}

async function runLifetime() {
    let state = await navigateScenario("lifetime");
    assertCommitted(state, metadataA, 1, "lifetime initial owner");
    const firstID = state.runtime.snapshot.activeOwnerID;

    let before = boundary(state);
    let frames = await clickAndCapture("unmount-owner");
    state = await pageState();
    assertCommitted(state, baseline, 0, "lifetime unmount");
    assertSequence(frames, [metadataA, baseline], "lifetime unmount frames");
    assertDeltas(state, before, { title: 1, description: 1, applies: 1, baseline: 1 }, "lifetime unmount");

    before = boundary(state);
    frames = await clickAndCapture("remount-owner");
    state = await pageState();
    assertCommitted(state, metadataA, 1, "lifetime remount");
    assert(state.runtime.snapshot.activeOwnerID > firstID, "remount reused released owner identity");
    assertDeltas(state, before, { title: 1, description: 1, applies: 1, baseline: 0 }, "lifetime remount");
    return resultSummary(state, frames);
}

async function runRepeatedMount() {
    let state = await navigateScenario("repeated-mount");
    assertCommitted(state, metadataA, 1, "repeated Mount initial owner");
    const before = boundary(state);
    const frames = await callAndCapture("goframeDocumentHandoffRepeatedMount");
    state = await pageState();
    assertCommitted(state, metadataB, 1, "repeated Mount replacement");
    assertSequence(frames, [metadataB], "repeated Mount frames");
    assertSequence(observerSequenceSince(state, before), [metadataA, metadataB], "repeated Mount observer");
    assertDeltas(state, before, { title: 1, description: 1, applies: 1, baseline: 0 }, "repeated Mount");
    assertIdentity(state, "repeated Mount");
    return resultSummary(state, frames);
}

async function runApplicationTeardown() {
    let state = await navigateScenario("teardown");
    assertCommitted(state, metadataA, 1, "teardown initial owner");
    const before = boundary(state);
    const frames = await callAndCapture("goframeDocumentHandoffTeardown");
    state = await pageState();
    assertCommitted(state, baseline, 0, "application teardown");
    assert(state.ownerlessApp, "application teardown did not mount ownerless replacement");
    assertSequence(frames, [baseline], "application teardown frames");
    assertDeltas(state, before, { title: 1, description: 1, applies: 1, baseline: 1 }, "application teardown");
    return resultSummary(state, frames);
}

async function runFailedInitial() {
    let state = await navigateScenario("failed-initial");
    await waitForAnimationFrames(4);
    state = await pageState();
    assertCommitted(state, baseline, 0, "failed initial owner");
    assert(state.runtime.statistics.tokenCreations === 1, "failed initial owner allocation evidence");
    assert(state.runtime.statistics.committedIDAssignments === 0, "failed initial owner received committed id");
    assert(state.runtime.documentApplies === 0, "failed initial owner mutated document");
    assert(state.runtime.runtimeErrors === 1, "failed initial render did not report once");
    assert(
        state.runtime.ownershipEvents.some((event) => event.kind === "publish-rolled-back"),
        "failed initial publication was not rolled back",
    );
    assertNoInvalidPairs(state, "failed initial owner");
    return resultSummary(state, [baseline]);
}

async function runFailedReplacementAndRetry() {
    let state = await navigateScenario("failed-replacement");
    assertCommitted(state, metadataA, 1, "failed replacement initial owner");
    const ownerAID = Number(state.runtime.snapshot.activeOwnerID);
    let before = boundary(state);
    let frames = await clickAndCapture("activate-failed-owner", 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "failed replacement rollback");
    assertSequence(frames, [metadataA], "failed replacement frames");
    assertSequence(observerSequenceSince(state, before), [metadataA], "failed replacement observer");
    assertDeltas(state, before, { title: 0, description: 0, applies: 0, baseline: 0 }, "failed replacement");
    assert(state.runtime.statistics.committedIDAssignments === 1, "failed replacement assigned speculative id");
    assert(state.runtime.statistics.tokenCreations === 2, "failed replacement token count");
    assert(state.runtime.runtimeErrors === 1, "failed replacement runtime report count");
    assert(state.retryVisible, "failed replacement fallback retry is unavailable");
    assert(Number(state.runtime.snapshot.activeOwnerID) === ownerAID, "failed replacement changed owner A identity");
    assert(Number(state.runtime.snapshot.failedBoundaryCount) === 1, "failed replacement boundary was not retained");
    assert(Number(state.runtime.snapshot.retainedReleaseCount) === 1, "failed replacement release was not retained");
    assert(state.runtime.statistics.releases === 0, "failed replacement released owner A");
    assert(state.runtime.statistics.baselineRestorations === 0, "failed replacement restored authored baseline");
    const failedEvents = state.runtime.ownershipEvents.slice(before.ownershipEventIndex);
    assert(
        failedEvents.some((event) =>
            event.kind === "publish-rolled-back" &&
            Number(event.ownerID) === 0 &&
            pairEqual(pairFromState(event), metadataB)
        ),
        "failed replacement did not retain B as an uncommitted owner",
    );
    assert(
        failedEvents.filter((event) => event.kind === "owner-committed").length === 0,
        "failed replacement committed owner B",
    );

    before = boundary(state);
    frames = await clickAndCapture("retry-owner", 5);
    state = await pageState();
    assertCommitted(state, metadataB, 1, "ErrorBoundary retry");
    assertSequence(frames, [metadataA, metadataB], "ErrorBoundary retry frames");
    assertSequence(observerSequenceSince(state, before), [metadataA, metadataB], "ErrorBoundary retry observer");
    assertDeltas(state, before, { title: 1, description: 1, applies: 1, baseline: 0 }, "ErrorBoundary retry");
    assert(state.runtime.statistics.committedIDAssignments === 2, "retry owner id assignment");
    assert(state.runtime.statistics.tokenCreations === 3, "retry did not create one fresh B lifetime");
    assert(Number(state.runtime.snapshot.activeOwnerID) > ownerAID, "retry did not commit a distinct owner B identity");
    assert(state.runtime.statistics.releases === 1, "retry did not release owner A exactly once");
    assert(Number(state.runtime.snapshot.failedBoundaryCount) === 0, "retry retained failed boundary state");
    assert(Number(state.runtime.snapshot.retainedReleaseCount) === 0, "retry retained stale owner release");
    assert(state.runtime.statistics.baselineRestorations === 0, "retry exposed authored baseline");
    assert(state.runtime.runtimeErrors === 1, "retry changed runtime error count");
    assert(
        JSON.stringify(state.runtime.ownerRenders.map((entry) => entry.role)) ===
            JSON.stringify(["replacement-a", "replacement-b", "replacement-b"]),
        `failed replacement owner renders = ${JSON.stringify(state.runtime.ownerRenders)}`,
    );
    assertIdentity(state, "failed replacement retry");
    return resultSummary(state, frames);
}

async function navigateScenario(scenario) {
    await client.call("Page.navigate", {
        url: `${origin}/?scenario=${encodeURIComponent(scenario)}&run=${Date.now()}`,
    });
    await waitForBoot(scenario);
    await waitForAnimationFrames(2);
    const state = await pageState();
    assertIdentity(state, `${scenario} boot`);
    assertNoInvalidPairs(state, `${scenario} boot`);
    return state;
}

async function clickAndCapture(testID, count = 4) {
    return client.evaluate(`new Promise((resolve, reject) => {
        const element = document.querySelector('[data-testid="${testID}"]');
        if (!element) {
            reject(new Error('missing control ${testID}'));
            return;
        }
        const frames = [];
        let remaining = ${count};
        const sample = () => {
            frames.push({
                title: document.querySelector('head title')?.textContent ?? '',
                description: document.querySelector('head meta[name="description"]')?.getAttribute('content') ?? '',
            });
            remaining--;
            if (remaining === 0) {
                resolve(frames);
                return;
            }
            requestAnimationFrame(sample);
        };
        requestAnimationFrame(sample);
        element.click();
    })`);
}

async function callAndCapture(functionName, count = 4) {
    return client.evaluate(`new Promise((resolve, reject) => {
        const action = window[${JSON.stringify(functionName)}];
        if (typeof action !== 'function') {
            reject(new Error('missing action ${functionName}'));
            return;
        }
        const frames = [];
        let remaining = ${count};
        const sample = () => {
            frames.push({
                title: document.querySelector('head title')?.textContent ?? '',
                description: document.querySelector('head meta[name="description"]')?.getAttribute('content') ?? '',
            });
            remaining--;
            if (remaining === 0) {
                resolve(frames);
                return;
            }
            requestAnimationFrame(sample);
        };
        requestAnimationFrame(sample);
        action();
    })`);
}

async function waitForAnimationFrames(count) {
    assert(Number.isInteger(count) && count > 0, `invalid animation frame count ${count}`);
    await client.evaluate(`new Promise((resolve) => {
        let remaining = ${count};
        const next = () => {
            remaining--;
            if (remaining === 0) {
                resolve(true);
                return;
            }
            requestAnimationFrame(next);
        };
        requestAnimationFrame(next);
    })`);
}

async function waitForBoot(scenario) {
    let last = null;
    for (let attempt = 0; attempt < 200; attempt++) {
        try {
            last = await pageState();
            if (last.app && last.scenario === scenario &&
                Number(last.runtime?.statistics?.updateBatches ?? 0) > 0) {
                return;
            }
        } catch {
            // Navigation can replace the execution context between probes.
        }
        await wait(50);
    }
    throw new Error(`HARNESS FAILURE: ${scenario} did not boot: ${JSON.stringify(last)}`);
}

async function pageState() {
    return client.evaluate(`(() => {
        const title = document.querySelector('head title');
        const description = document.querySelector('head meta[name="description"]');
        const head = window.__goframeDocumentHandoffHead;
        const runtime = window.goframeDocumentHandoffEvidence;
        return {
            app: Boolean(document.querySelector('[data-testid="handoff-app"]')),
            ownerlessApp: Boolean(document.querySelector('[data-testid="ownerless-app"]')),
            retryVisible: Boolean(document.querySelector('[data-testid="retry-owner"]')),
            scenario: document.querySelector('[data-testid="handoff-app"]')?.getAttribute('data-scenario') ?? '',
            title: title?.textContent ?? '',
            description: description?.getAttribute('content') ?? '',
            runtime: runtime ? JSON.parse(JSON.stringify(runtime)) : null,
            head: head ? {
                titleMutations: head.titleMutations,
                descriptionMutations: head.descriptionMutations,
                snapshots: JSON.parse(JSON.stringify(head.snapshots)),
                titleNodeSame: head.titleNode === title,
                descriptionNodeSame: head.descriptionNode === description,
                unrelatedNodeSame: head.unrelatedNode === document.querySelector('head meta[name="fixture-unrelated"]'),
                unrelatedValue: head.unrelatedNode?.getAttribute('content') ?? '',
                titleConnected: Boolean(head.titleNode?.isConnected),
                descriptionConnected: Boolean(head.descriptionNode?.isConnected),
            } : null,
        };
    })()`);
}

function boundary(state) {
    return {
        titleMutations: state.head.titleMutations,
        descriptionMutations: state.head.descriptionMutations,
        snapshotIndex: state.head.snapshots.length,
        documentApplies: state.runtime.documentApplies,
        baselineRestorations: state.runtime.baselineRestorations,
        ownershipEventIndex: state.runtime.ownershipEvents.length,
        statistics: { ...state.runtime.statistics },
    };
}

function observerSequenceSince(state, before) {
    return [pairFromState({
        title: before.snapshotIndex > 0
            ? state.head.snapshots[before.snapshotIndex - 1].title
            : baseline.title,
        description: before.snapshotIndex > 0
            ? state.head.snapshots[before.snapshotIndex - 1].description
            : baseline.description,
    }), ...state.head.snapshots.slice(before.snapshotIndex).map(pairFromState)];
}

function assertCommitted(state, expected, ownerCount, label) {
    assert(pairEqual(pairFromState(state), expected), `${label} document pair = ${JSON.stringify(pairFromState(state))}`);
    assert(state.runtime, `${label} runtime evidence is missing`);
    assert(pairEqual(pairFromState(state.runtime.snapshot), expected), `${label} committed snapshot mismatch`);
    assert(Number(state.runtime.snapshot.ownerCount) === ownerCount, `${label} owner count = ${state.runtime.snapshot.ownerCount}`);
    assert((ownerCount === 0) === (Number(state.runtime.snapshot.activeOwnerID) === 0), `${label} active owner identity mismatch`);
}

function assertDeltas(state, before, expected, label) {
    const actual = {
        title: state.head.titleMutations - before.titleMutations,
        description: state.head.descriptionMutations - before.descriptionMutations,
        applies: state.runtime.documentApplies - before.documentApplies,
        baseline: state.runtime.baselineRestorations - before.baselineRestorations,
    };
    assert(
        JSON.stringify(actual) === JSON.stringify(expected),
        `${label} deltas = ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`,
    );
}

function assertSequence(values, expected, label) {
    const actual = normalizePairs(values.map(pairFromState));
    assert(
        JSON.stringify(actual) === JSON.stringify(expected),
        `${label} = ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`,
    );
}

function assertNoInvalidPairs(state, label) {
    const observed = [pairFromState(state), ...state.head.snapshots.map(pairFromState)];
    const invalid = observed.filter((value) => !validPairs.some((candidate) => pairEqual(value, candidate)));
    assert(invalid.length === 0, `${label} observed invalid pairs ${JSON.stringify(invalid)}`);
}

function assertIdentity(state, label) {
    assert(state.head, `${label} head evidence is missing`);
    assert(state.head.titleNodeSame && state.head.descriptionNodeSame && state.head.unrelatedNodeSame, `${label} replaced authored head nodes`);
    assert(state.head.titleConnected && state.head.descriptionConnected, `${label} disconnected authored head nodes`);
    assert(state.head.unrelatedValue === "preserve-me", `${label} changed unrelated head metadata`);
}

function resultSummary(state, frames) {
    assertNoInvalidPairs(state, state.scenario || "ownerless result");
    assertIdentity(state, state.scenario || "ownerless result");
    return {
        selected: pairFromState(state),
        ownerID: Number(state.runtime.snapshot.activeOwnerID),
        ownerCount: Number(state.runtime.snapshot.ownerCount),
        frames: normalizePairs(frames.map(pairFromState)),
        titleMutations: state.head.titleMutations,
        descriptionMutations: state.head.descriptionMutations,
        documentApplies: state.runtime.documentApplies,
        baselineRestorations: state.runtime.baselineRestorations,
        statistics: state.runtime.statistics,
        renderReports: state.runtime.renderReports,
        componentRenders: state.runtime.componentRenders,
        componentPatches: state.runtime.componentPatches,
        effectFlushes: state.runtime.effectFlushes,
        runtimeErrors: state.runtime.runtimeErrors,
    };
}

function normalizePairs(values) {
    const result = [];
    for (const value of values) {
        if (result.length === 0 || !pairEqual(result[result.length - 1], value)) {
            result.push(value);
        }
    }
    return result;
}

function pairFromState(value) {
    return pair(String(value?.title ?? ""), String(value?.description ?? ""));
}

function pair(title, description) {
    return { title, description };
}

function pairEqual(first, second) {
    return first.title === second.title && first.description === second.description;
}

async function buildFixture() {
    const wasm = join(tempRoot, "bundle.wasm");
    const cache = join(tempRoot, "gocache");
    const xdgCache = join(tempRoot, "xdg-cache");
    const commonEnvironment = {
        ...process.env,
        GOCACHE: cache,
        GOWORK: "off",
        GOFLAGS: "-buildvcs=false",
        GOTOOLCHAIN: goToolchain,
        XDG_CACHE_HOME: xdgCache,
    };
    let runtimeSource;
    if (compiler === "go") {
        await runCommand("go", [
            "build",
            "-tags=goframe_document_state_experiment,goframe_debug",
            "-o",
            wasm,
            fixturePackage,
        ], {
            ...commonEnvironment,
            GOOS: "js",
            GOARCH: "wasm",
            CGO_ENABLED: "0",
        });
        const goRoot = (await runCapture("go", ["env", "GOROOT"], commonEnvironment)).trim();
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
            "-tags=goframe_document_state_experiment,goframe_debug",
            "-o",
            wasm,
            fixturePackage,
        ], commonEnvironment);
        const tinyGoRoot = (await runCapture("tinygo", ["env", "TINYGOROOT"], commonEnvironment)).trim();
        runtimeSource = join(tinyGoRoot, "targets", "wasm_exec.js");
    }
    await copyFile(runtimeSource, join(tempRoot, "wasm_exec.js"));
    await copyFile(join(fixtureDir, "assets", "index.html"), join(tempRoot, "index.html"));
    await copyFile(join(fixtureDir, "assets", "styles.css"), join(tempRoot, "styles.css"));
    const bytes = await readFile(wasm);
    return {
        bytes: bytes.length,
        sha256: createHash("sha256").update(bytes).digest("hex"),
    };
}

async function firstExistingPath(paths) {
    for (const path of paths) {
        try {
            if ((await stat(path)).isFile()) return path;
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
            if (!new Set(["index.html", "styles.css", "wasm_exec.js", "bundle.wasm"]).has(name)) {
                response.writeHead(404).end("not found");
                return;
            }
            const content = await readFile(join(directory, name));
            const mime = name.endsWith(".wasm")
                ? "application/wasm"
                : name.endsWith(".js")
                    ? "text/javascript"
                    : name.endsWith(".css")
                        ? "text/css"
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

function installHeadObserver() {
    const state = {
        baselineCaptured: false,
        titleNode: null,
        descriptionNode: null,
        unrelatedNode: null,
        titleMutations: 0,
        descriptionMutations: 0,
        snapshots: [],
    };
    window.__goframeDocumentHandoffHead = state;

    const captureBaseline = () => {
        if (state.baselineCaptured) return true;
        const titles = document.querySelectorAll("head title");
        const descriptions = document.querySelectorAll(
            'head meta[name="description"]',
        );
        const unrelated = document.querySelector(
            'head meta[name="fixture-unrelated"]',
        );
        if (titles.length !== 1 || descriptions.length !== 1 || !unrelated) {
            return false;
        }
        state.titleNode = titles[0];
        state.descriptionNode = descriptions[0];
        state.unrelatedNode = unrelated;
        state.baselineCaptured = true;
        state.snapshots.push({
            title: state.titleNode.textContent ?? "",
            description: state.descriptionNode.getAttribute("content") ?? "",
        });
        return true;
    };

    const containsNode = (record, node) => {
        if (!node) return false;
        if (record.target === node || node.contains(record.target)) return true;
        return [...record.addedNodes, ...record.removedNodes].some(
            (changed) => changed === node ||
                (changed.nodeType === Node.ELEMENT_NODE && changed.contains(node)),
        );
    };

    const observer = new MutationObserver((records) => {
        const capturedBeforeBatch = state.baselineCaptured;
        if (!captureBaseline() || !capturedBeforeBatch) return;
        const titleTouched = records.some((record) =>
            containsNode(record, state.titleNode)
        );
        const descriptionTouched = records.some((record) =>
            containsNode(record, state.descriptionNode)
        );
        if (titleTouched) state.titleMutations++;
        if (descriptionTouched) state.descriptionMutations++;
        if (!titleTouched && !descriptionTouched) return;
        state.snapshots.push({
            title: state.titleNode.textContent ?? "",
            description: state.descriptionNode.getAttribute("content") ?? "",
        });
    });
    observer.observe(document, {
        subtree: true,
        childList: true,
        attributes: true,
        characterData: true,
    });
    queueMicrotask(captureBaseline);
    document.addEventListener("DOMContentLoaded", captureBaseline, { once: true });
}

async function diagnostics() {
    const result = {
        compiler,
        appPort,
        debugPort,
        browserStderr: browserError.slice(-6000),
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
    const pending = new Map();
    socket.addEventListener("message", (event) => {
        const message = JSON.parse(event.data);
        if (!message.id || !pending.has(message.id)) return;
        const request = pending.get(message.id);
        pending.delete(message.id);
        if (message.error) request.reject(new Error(message.error.message));
        else request.resolve(message.result);
    });
    return {
        call(method, params = {}) {
            return new Promise((resolveCall, reject) => {
                const id = nextID++;
                pending.set(id, { resolve: resolveCall, reject });
                socket.send(JSON.stringify({ id, method, params }));
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
        close() {
            socket.close();
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

function runCapture(command, args, environment) {
    return runCommand(command, args, environment);
}

async function stopProcess(child, processGroup) {
    if (!child || child.exitCode !== null || child.signalCode !== null) return;
    const exited = new Promise((resolveExit) => child.once("exit", resolveExit));
    terminate(child, processGroup, "SIGTERM");
    const stopped = await Promise.race([
        exited.then(() => true),
        wait(2000).then(() => false),
    ]);
    if (!stopped && child.exitCode === null && child.signalCode === null) {
        terminate(child, processGroup, "SIGKILL");
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
        // Fall through to direct child termination.
    }
    try {
        child.kill(signal);
    } catch {
        // The process may have exited between checks.
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

function wait(milliseconds) {
    return new Promise((resolveWait) => setTimeout(resolveWait, milliseconds));
}

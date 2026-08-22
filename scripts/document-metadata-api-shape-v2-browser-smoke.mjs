import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { copyFile, cp, mkdir, mkdtemp, readFile, rm, stat } from "node:fs/promises";
import { createServer as createHTTPServer } from "node:http";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { createServer as createPortServer } from "node:net";

import { createCDPClient } from "./document-metadata-api-shape-v2-cdp.mjs";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const rootDir = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const sourceFixtureDir = join(
    rootDir,
    "scripts",
    "fixtures",
    "document-metadata-api-shape-v2",
);
const fixturePackage = "./scripts/fixtures/document-metadata-api-shape-v2/cmd/app";
const compiler = process.env.GOFRAME_DOCUMENT_API_SHAPE_COMPILER ?? "go";
const chrome = process.env.CHROME ?? "google-chrome";
const goToolchain = process.env.GOTOOLCHAIN ?? "go1.26.6";
const appPort = Number(
    process.env.GOFRAME_DOCUMENT_API_SHAPE_PORT ?? await pickFreePort(),
);
const debugPort = Number(
    process.env.GOFRAME_DOCUMENT_API_SHAPE_CHROME_DEBUG_PORT ?? await pickFreePort(),
);
const origin = `http://127.0.0.1:${appPort}`;
const modes = ["control", "hook", "component", "handle"];

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
let activeMode = "control";

try {
    if (compiler !== "go" && compiler !== "tinygo") {
        throw new Error(`HARNESS FAILURE: unsupported compiler ${JSON.stringify(compiler)}`);
    }
    tempRoot = await mkdtemp(join(tmpdir(), `goframe-document-api-shape-v2-${compiler}-`));
    profile = await mkdtemp(join(tmpdir(), "goframe-document-api-shape-v2-chrome-"));
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
    const behaviorHashes = {};
    for (const mode of modes) {
        activeMode = mode;
        results[mode] = await runCandidateMode();
        behaviorHashes[mode] = hashJSON(results[mode]);
    }

    const report = {
        compiler,
        artifact,
        behaviorHashes,
        combinedBehaviorHash: hashJSON(results),
        results,
    };
    console.log(`document metadata API shape v2 evidence: ${JSON.stringify(report)}`);
    console.log(`document metadata API shape v2 browser smoke (${compiler}): ok`);
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

async function runCandidateMode() {
    const results = {};
    results.direct = await runDirectReplacement();
    results.nested = await runNestedPriority();
    results.nonselected = await runNonselectedRelease();
    results.sameValue = await runSameValueUpdate();
    results.multiple = await runMultipleOperations();
    results.lifetime = await runLifetime();
    results.repeatedMount = await runRepeatedMount();
    results.teardown = await runApplicationTeardown();
    if (activeMode === "hook") {
        results.candidateContract = await runHookContract();
    } else if (activeMode === "component") {
        results.candidateContract = await runComponentContract();
    } else if (activeMode === "handle") {
        results.candidateContract = await runHandleContract();
    }
    if (compiler === "go") {
        results.failedInitial = await runFailedInitial();
        results.failedReplacement = await runFailedReplacementAndRetry();
        results.panicBeforeMetadata = await runPanicBeforeMetadataAndRetry();
        results.siblingFailure = await runSiblingFailureAndRetry();
        results.ownerlessRecovery = await runOwnerlessRecovery();
        results.nestedOuterFailure = await runNestedOuterFailure();
        results.publicationFailure = await runPublicationFailureAndRetry();
        results.reversedPlan = await runReversedOwnershipPlan();
        results.partialPlan = await runPartialOwnershipPlan();
        results.additivePlan = await runAdditiveOwnershipPlan();
        results.initialPlan = await runInitialOwnershipPlan();
        results.externalFinalization = await runFinalizationAbsorption(
            "plan-finalization-external",
            "external",
        );
        results.crossBoundaryFinalization = await runFinalizationAbsorption(
            "plan-finalization-cross-boundary",
            "cross",
        );
        results.newerBoundaryFailure = await runNewerBoundaryFailure();
    }
    return results;
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
    const expectedDuplicates = before.statistics.duplicatePublications + 1;
    assert(
        state.runtime.statistics.duplicatePublications === expectedDuplicates,
        "same-value publication classification",
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

async function runHookContract() {
    let state = await navigateScenario("hook-contract");
    assertCommitted(state, metadataB, 2, "hook slots initial owners");
    assertOwnerOrder(state, [1, 2], [metadataA.title, metadataB.title], "hook slots initial order");
    assert(state.runtime.statistics.committedIDAssignments === 2, "hook slots IDs");

    let before = boundary(state);
    let frames = await clickAndCapture("update-first-hook-slot");
    state = await pageState();
    assertCommitted(state, metadataB, 2, "hook first-slot update");
    assertOwnerOrder(state, [1, 2], [metadataA2.title, metadataB.title], "hook updated order");
    assertSequence(frames, [metadataB], "hook first-slot update frames");
    assertDeltas(state, before, { title: 0, description: 0, applies: 0, baseline: 0 }, "hook first-slot update");

    before = boundary(state);
    frames = await clickAndCapture("release-hook-slots");
    state = await pageState();
    assertCommitted(state, baseline, 0, "hook slot component boundary release");
    assertSequence(frames, [metadataB, baseline], "hook slot release frames");
    assertDeltas(state, before, { title: 1, description: 1, applies: 1, baseline: 1 }, "hook slot release");
    assert(state.runtime.statistics.releases === 2, "hook slots did not release once each");
    return resultSummary(state, frames);
}

async function runComponentContract() {
    let state = await navigateScenario("component-contract");
    assertCommitted(state, metadataA, 1, "component initial owner");
    assert(state.componentChildParentTestID === "handoff-scenario", "component emitted a wrapper DOM node");
    const firstID = Number(state.runtime.snapshot.activeOwnerID);

    let before = boundary(state);
    const identity = await clickAndCaptureElementIdentity(
        "update-component-metadata",
        '[data-testid="component-contract-child"]',
    );
    state = await pageState();
    assertCommitted(state, metadataA2, 1, "component prop update");
    assert(Number(state.runtime.snapshot.activeOwnerID) === firstID, "component prop update changed owner identity");
    assert(identity.same, "component prop update replaced child DOM identity");
    assertSequence(identity.frames, [metadataA, metadataA2], "component prop update frames");
    assertDeltas(state, before, { title: 1, description: 1, applies: 1, baseline: 0 }, "component prop update");

    before = boundary(state);
    const frames = await clickAndCapture("release-component-owner");
    state = await pageState();
    assertCommitted(state, baseline, 0, "component conditional unmount");
    assertSequence(frames, [metadataA2, baseline], "component release frames");
    assertDeltas(state, before, { title: 1, description: 1, applies: 1, baseline: 1 }, "component release");
    return resultSummary(state, frames);
}

async function runHandleContract() {
    let state = await navigateScenario("handle-contract");
    assertCommitted(state, metadataA, 1, "handle primary publication");
    assert(Number(state.runtime.candidate.handleID) === 1, "handle committed identity");
    assert(Number(state.runtime.candidate.activePublications) === 1, "handle primary publication count");
    const firstID = Number(state.runtime.snapshot.activeOwnerID);

    let before = boundary(state);
    let frames = await clickAndCapture("add-handle-duplicate");
    state = await pageState();
    assertCommitted(state, metadataA, 1, "handle identical duplicate");
    assert(Number(state.runtime.candidate.activePublications) === 2, "handle duplicate did not coalesce");
    assertSequence(frames, [metadataA], "handle duplicate frames");
    assertDeltas(state, before, { title: 0, description: 0, applies: 0, baseline: 0 }, "handle duplicate");

    if (compiler === "go") {
        before = boundary(state);
        frames = await clickAndCapture("conflict-handle-duplicate", 5);
        state = await pageState();
        assertCommitted(state, metadataA, 1, "handle conflict rejection");
        assert(state.handleConflictFallback, "handle conflict did not enter its boundary fallback");
        assert(state.runtime.runtimeErrors === 1, "handle conflict runtime error count");
        assert(
            Number(state.runtime.candidate.activePublications) === 1,
            "handle conflict fallback did not release only the rejected duplicate",
        );
        assertSequence(frames, [metadataA], "handle conflict frames");
        assertNoDocumentPublication(state, before, "handle conflict");
    }

    before = boundary(state);
    frames = await clickAndCapture("release-handle-duplicate", 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "handle duplicate release");
    assert(Number(state.runtime.candidate.activePublications) === 1, "handle duplicate release count");
    assert(Number(state.runtime.snapshot.activeOwnerID) === firstID, "handle duplicate release changed owner");
    assertSequence(frames, [metadataA], "handle duplicate release frames");
    assertNoDocumentPublication(state, before, "handle duplicate release");

    before = boundary(state);
    frames = await clickAndCapture("update-handle-primary");
    state = await pageState();
    assertCommitted(state, metadataB, 1, "handle sole-primary update");
    assert(Number(state.runtime.snapshot.activeOwnerID) === firstID, "handle update changed owner identity");
    assertSequence(frames, [metadataA, metadataB], "handle update frames");
    assertDeltas(state, before, { title: 1, description: 1, applies: 1, baseline: 0 }, "handle update");

    before = boundary(state);
    frames = await clickAndCapture("release-handle-primary");
    state = await pageState();
    assertCommitted(state, baseline, 0, "handle final publication release");
    assert(Number(state.runtime.candidate.activePublications) === 0, "handle final publication remained active");
    assert(state.runtime.statistics.releases === 1, "handle owner release count");
    assertSequence(frames, [metadataB, baseline], "handle final release frames");
    assertDeltas(state, before, { title: 1, description: 1, applies: 1, baseline: 1 }, "handle final release");
    return resultSummary(state, frames);
}

async function runFailedInitial() {
    let state = await navigateScenario("failed-initial");
    await waitForAnimationFrames(4);
    state = await pageState();
    assertCommitted(state, baseline, 0, "failed initial owner");
    assert(
        state.runtime.statistics.tokenCreations === (activeMode === "control" ? 1 : 0),
        "failed initial owner allocation evidence",
    );
    assert(state.runtime.statistics.committedIDAssignments === 0, "failed initial owner received committed id");
    assert(state.runtime.documentApplies === 0, "failed initial owner mutated document");
    assert(state.runtime.runtimeErrors === 1, "failed initial render did not report once");
    const rolledBack = state.runtime.ownershipEvents.some(
        (event) => event.kind === "publish-rolled-back",
    );
    assert(rolledBack, "failed initial candidate participation evidence");
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
    assert(
        state.runtime.statistics.tokenCreations === (activeMode === "control" ? 2 : 1),
        "failed replacement token count",
    );
    assert(state.runtime.runtimeErrors === 1, "failed replacement runtime report count");
    assert(state.retryVisible, "failed replacement fallback retry is unavailable");
    assert(Number(state.runtime.snapshot.activeOwnerID) === ownerAID, "failed replacement changed owner A identity");
    assert(Number(state.runtime.snapshot.failedBoundaryCount) === 1, "failed replacement boundary was not retained");
    assert(Number(state.runtime.snapshot.retainedReleaseCount) === 1, "failed replacement release was not retained");
    assert(state.runtime.statistics.releases === 0, "failed replacement released owner A");
    assert(state.runtime.statistics.baselineRestorations === 0, "failed replacement restored authored baseline");
    const failedEvents = state.runtime.ownershipEvents.slice(before.ownershipEventIndex);
    const failedPublication = failedEvents.some((event) =>
        event.kind === "publish-rolled-back" &&
        Number(event.ownerID) === 0 &&
        pairEqual(pairFromState(event), metadataB)
    );
    assert(failedPublication, "failed replacement candidate participation evidence");
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
    assert(
        state.runtime.statistics.tokenCreations === (activeMode === "control" ? 3 : 2),
        "retry did not create one fresh B lifetime",
    );
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

async function runPanicBeforeMetadataAndRetry() {
    let state = await navigateScenario("panic-before-metadata");
    assertCommitted(state, metadataA, 1, "pre-metadata initial owner");
    const ownerAID = Number(state.runtime.snapshot.activeOwnerID);
    let before = boundary(state);
    let frames = await clickAndCapture("activate-pre-metadata-failure", 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "pre-metadata failure retention");
    assertSequence(frames, [metadataA], "pre-metadata failure frames");
    assertSequence(observerSequenceSince(state, before), [metadataA], "pre-metadata failure observer");
    assertDeltas(state, before, { title: 0, description: 0, applies: 0, baseline: 0 }, "pre-metadata failure");
    assert(state.retryPreMetadataVisible, "pre-metadata retry is unavailable");
    assert(Number(state.runtime.snapshot.activeOwnerID) === ownerAID, "pre-metadata failure changed owner A");
    assert(Number(state.runtime.snapshot.failedBoundaryCount) === 1, "pre-metadata failure boundary missing");
    assert(Number(state.runtime.snapshot.retainedReleaseCount) === 1, "pre-metadata release missing");
    assert(!state.runtime.snapshot.batchActive, "pre-metadata failure left a batch active");
    assert(state.runtime.statistics.tokenCreations === 1, "pre-metadata panic created owner B token");
    assert(cleanupCount(state, "pre-metadata-a") === 1, "pre-metadata owner A cleanup count");

    before = boundary(state);
    frames = await clickAndCapture("retry-pre-metadata", 5);
    state = await pageState();
    assertCommitted(state, metadataB, 1, "pre-metadata retry");
    assertSequence(frames, [metadataA, metadataB], "pre-metadata retry frames");
    assertSequence(observerSequenceSince(state, before), [metadataA, metadataB], "pre-metadata retry observer");
    assert(Number(state.runtime.snapshot.failedBoundaryCount) === 0, "pre-metadata retry retained failed boundary");
    assert(Number(state.runtime.snapshot.retainedReleaseCount) === 0, "pre-metadata retry retained release");
    assert(!state.runtime.snapshot.batchActive, "pre-metadata retry left a batch active");
    assert(state.runtime.statistics.releases === 1, "pre-metadata retry release count");
    assert(cleanupCount(state, "pre-metadata-a") === 1, "pre-metadata retry replayed owner A cleanup");
    return resultSummary(state, frames);
}

async function runSiblingFailureAndRetry() {
    let state = await navigateScenario("sibling-failure");
    assertCommitted(state, metadataA, 1, "sibling initial owner");
    let before = boundary(state);
    let frames = await clickAndCapture("activate-sibling-failure", 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "sibling failure retention");
    assertSequence(frames, [metadataA], "sibling failure frames");
    assertSequence(observerSequenceSince(state, before), [metadataA], "sibling failure observer");
    assert(state.retrySiblingVisible, "sibling retry is unavailable");
    assert(Number(state.runtime.snapshot.failedBoundaryCount) === 1, "sibling failed boundary missing");
    assert(Number(state.runtime.snapshot.retainedReleaseCount) === 1, "sibling retained release missing");
    assert(!state.runtime.snapshot.batchActive, "sibling failure left a batch active");
    assert(state.runtime.statistics.committedIDAssignments === 1, "sibling failure committed owner B");
    assert(cleanupCount(state, "sibling-owner-a") === 1, "sibling failure owner A cleanup count");

    before = boundary(state);
    frames = await clickAndCapture("retry-sibling-failure", 5);
    state = await pageState();
    assertCommitted(state, metadataB, 1, "sibling retry");
    assertSequence(frames, [metadataA, metadataB], "sibling retry frames");
    assertSequence(observerSequenceSince(state, before), [metadataA, metadataB], "sibling retry observer");
    assert(Number(state.runtime.snapshot.failedBoundaryCount) === 0, "sibling retry retained failed boundary");
    assert(Number(state.runtime.snapshot.retainedReleaseCount) === 0, "sibling retry retained release");
    assert(!state.runtime.snapshot.batchActive, "sibling retry left a batch active");
    assert(state.runtime.statistics.committedIDAssignments === 2, "sibling retry did not commit B once");
    assert(state.runtime.statistics.releases === 1, "sibling retry release count");
    return resultSummary(state, frames);
}

async function runOwnerlessRecovery() {
    let state = await navigateScenario("ownerless-recovery");
    assertCommitted(state, metadataA, 1, "ownerless initial owner");
    let frames = await clickAndCapture("activate-ownerless-failure", 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "ownerless failure retention");
    assertSequence(frames, [metadataA], "ownerless failure frames");
    assert(state.recoverOwnerlessVisible, "ownerless recovery is unavailable");
    const before = boundary(state);

    frames = await clickAndCapture("recover-ownerless", 5);
    state = await pageState();
    assertCommitted(state, baseline, 0, "ownerless recovery");
    assert(state.ownerlessRecoveryContent, "ownerless recovery content is absent");
    assertSequence(frames, [metadataA, baseline], "ownerless recovery frames");
    assertSequence(observerSequenceSince(state, before), [metadataA, baseline], "ownerless recovery observer");
    assert(Number(state.runtime.snapshot.failedBoundaryCount) === 0, "ownerless recovery retained failed boundary");
    assert(Number(state.runtime.snapshot.retainedReleaseCount) === 0, "ownerless recovery retained release");
    assert(!state.runtime.snapshot.batchActive, "ownerless recovery left a batch active");
    assert(state.runtime.statistics.releases === 1, "ownerless recovery release count");
    assert(state.runtime.statistics.baselineRestorations === 1, "ownerless recovery baseline count");
    assert(cleanupCount(state, "ownerless-a") === 1, "ownerless recovery replayed owner A cleanup");
    return resultSummary(state, frames);
}

async function runNestedOuterFailure() {
    let state = await navigateScenario("nested-outer-failure");
    assertCommitted(state, metadataA, 1, "nested outer initial owner");
    let before = boundary(state);
    let frames = await clickAndCapture("activate-nested-outer-failure", 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "nested outer failure retention");
    assert(!state.unexpectedInnerFallback, "nested inner boundary captured the outer sibling failure");
    assert(state.retryNestedOuterVisible, "nested outer retry is unavailable");
    assertSequence(frames, [metadataA], "nested outer failure frames");
    assertSequence(observerSequenceSince(state, before), [metadataA], "nested outer failure observer");
    assert(Number(state.runtime.snapshot.failedBoundaryCount) === 1, "nested outer failed boundary missing");
    assert(Number(state.runtime.snapshot.retainedReleaseCount) === 1, "nested outer retained release missing");
    assert(!state.runtime.snapshot.batchActive, "nested outer failure left a batch active");

    before = boundary(state);
    frames = await clickAndCapture("retry-nested-outer", 5);
    state = await pageState();
    assertCommitted(state, metadataB, 1, "nested outer retry");
    assertSequence(frames, [metadataA, metadataB], "nested outer retry frames");
    assertSequence(observerSequenceSince(state, before), [metadataA, metadataB], "nested outer retry observer");
    assert(Number(state.runtime.snapshot.failedBoundaryCount) === 0, "nested outer retry retained boundary");
    assert(Number(state.runtime.snapshot.retainedReleaseCount) === 0, "nested outer retry retained release");
    assert(!state.runtime.snapshot.batchActive, "nested outer retry left a batch active");

    state = await navigateScenario("nested-outer-failure");
    assertCommitted(state, metadataA, 1, "nested ownerless initial owner");
    frames = await clickAndCapture("activate-nested-outer-failure", 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "nested ownerless failure retention");
    before = boundary(state);
    frames = await clickAndCapture("recover-nested-ownerless", 5);
    state = await pageState();
    assertCommitted(state, baseline, 0, "nested ownerless recovery");
    assert(state.nestedOwnerlessContent, "nested ownerless content is absent");
    assertSequence(frames, [metadataA, baseline], "nested ownerless recovery frames");
    assertSequence(observerSequenceSince(state, before), [metadataA, baseline], "nested ownerless recovery observer");
    assert(Number(state.runtime.snapshot.failedBoundaryCount) === 0, "nested ownerless recovery retained boundary");
    assert(Number(state.runtime.snapshot.retainedReleaseCount) === 0, "nested ownerless recovery retained release");
    assert(!state.runtime.snapshot.batchActive, "nested ownerless recovery left a batch active");
    assert(cleanupCount(state, "nested-outer-a") === 1, "nested ownerless recovery replayed owner A cleanup");
    return resultSummary(state, frames);
}

async function runPublicationFailureAndRetry() {
    let state = await navigateScenario("publication-failure");
    assertCommitted(state, metadataA, 1, "publication failure initial owner");
    const ownerAID = Number(state.runtime.snapshot.activeOwnerID);
    let before = boundary(state);
    let frames = await clickAndCapture("activate-publication-failure", 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "publication failure retention");
    assertSequence(frames, [metadataA], "publication failure frames");
    assertSequence(observerSequenceSince(state, before), [metadataA], "publication failure observer");
    assert(Number(state.runtime.snapshot.activeOwnerID) === ownerAID, "publication failure changed owner A");
    assert(Number(state.runtime.snapshot.retainedReleaseCount) === 1, "publication failure lost owner A detach");
    assert(!state.runtime.snapshot.batchActive, "publication failure left a batch active");
    assert(state.publicationNonce === "0", "publication failure did not retain mounted B");
    assert(state.runtime.publicationFailures === 1, "publication failure was not injected once");
    assert(state.runtime.statistics.tokenCreations === 2, "publication failure did not render owner B");
    assert(state.runtime.statistics.committedIDAssignments === 1, "publication failure committed owner B");
    assert(state.runtime.statistics.releases === 0, "publication failure committed owner A release");
    assert(state.runtime.runtimeErrors === 1, "publication failure runtime report count");
    assert(state.runtime.runtimeReports.length === 1, "publication failure report evidence count");
    assert(state.runtime.runtimeReports[0].phase === "render", "publication failure report phase");
    assert(state.runtime.runtimeReports[0].operation === "document metadata transaction", "publication failure report operation");
    assert(state.runtime.runtimeReports[0].panic.includes("document metadata publication failed"), "publication failure report diagnostic");
    assert(cleanupCount(state, "publication-a") === 1, "publication failure owner A cleanup count");
    assertPendingPlan(state, [metadataB.title], 1, 0, "publication failure plan");

    const rendersBeforeShell = ownerRenderCount(state, "publication-b");
    before = boundary(state);
    frames = await clickAndCapture("update-publication-shell", 3);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "publication failure unrelated update");
    assertSequence(frames, [metadataA], "publication failure unrelated frames");
    assertSequence(observerSequenceSince(state, before), [metadataA], "publication failure unrelated observer");
    assertDeltas(state, before, { title: 0, description: 0, applies: 0, baseline: 0 }, "publication failure unrelated update");
    assert(state.shell["publication-shell"] === "1", "publication shell did not update");
    assert(ownerRenderCount(state, "publication-b") === rendersBeforeShell, "unrelated shell update rerendered B");
    assertPendingPlan(state, [metadataB.title], 1, 0, "publication failure plan after unrelated update");

    before = boundary(state);
    frames = await clickAndCapture("retry-publication", 5);
    state = await pageState();
    assertCommitted(state, metadataB, 1, "publication retry");
    assertSequence(frames, [metadataA, metadataB], "publication retry frames");
    assertSequence(observerSequenceSince(state, before), [metadataA, metadataB], "publication retry observer");
    assert(state.publicationNonce === "1", "publication retry did not rerender mounted B");
    assert(state.runtime.statistics.committedIDAssignments === 2, "publication retry did not commit B once");
    assert(state.runtime.statistics.releases === 1, "publication retry owner A release count");
    assert(Number(state.runtime.snapshot.retainedReleaseCount) === 0, "publication retry retained detach");
    assert(!state.runtime.snapshot.batchActive, "publication retry left a batch active");
    assert(state.runtime.runtimeErrors === 1, "publication retry added a runtime report");
    assert(cleanupCount(state, "publication-a") === 1, "publication retry replayed owner A cleanup");

    before = boundary(state);
    frames = await clickAndCapture("unmount-publication-owner", 5);
    state = await pageState();
    assertCommitted(state, baseline, 0, "publication owner unmount");
    assertSequence(frames, [metadataB, baseline], "publication owner unmount frames");
    assertSequence(observerSequenceSince(state, before), [metadataB, baseline], "publication owner unmount observer");
    assert(Number(state.runtime.snapshot.retainedReleaseCount) === 0, "publication owner unmount retained detach");
    assert(!state.runtime.snapshot.batchActive, "publication owner unmount left a batch active");
    assert(state.runtime.statistics.releases === 2, "publication owner unmount release count");
    assert(cleanupCount(state, "publication-a") === 1, "publication owner unmount replayed owner A cleanup");
    assert(cleanupCount(state, "publication-b") === 1, "publication owner B cleanup count");
    return resultSummary(state, frames);
}

async function runReversedOwnershipPlan() {
    let state = await navigateScenario("plan-reversed");
    assertCommitted(state, metadataA, 1, "reversed plan initial owner");
    let before = boundary(state);
    let frames = await clickAndCapture("activate-plan-reversed", 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "reversed plan failed publication");
    assertSequence(frames, [metadataA], "reversed plan failure frames");
    assertSequence(observerSequenceSince(state, before), [metadataA], "reversed plan failure observer");
    assertNoDocumentPublication(state, before, "reversed plan failure");
    assertPendingPlan(state, [metadataB.title, metadataC.title], 1, 0, "reversed plan pending order");
    assert(ownerRenderCount(state, "reversed-b") === 1, "reversed plan B initial render count");
    assert(ownerRenderCount(state, "reversed-c") === 1, "reversed plan C initial render count");
    assert(cleanupCount(state, "reversed-a") === 1, "reversed plan A cleanup count");

    before = boundary(state);
    frames = await clickAndCapture("retry-plan-reversed", 5);
    state = await pageState();
    assertCommitted(state, metadataC, 2, "reversed plan retry");
    assertSequence(frames, [metadataA, metadataC], "reversed plan retry frames");
    assertSequence(observerSequenceSince(state, before), [metadataA, metadataC], "reversed plan retry observer");
    assertOwnerOrder(state, [2, 3], [metadataB.title, metadataC.title], "reversed plan retry");
    assert(Number(state.runtime.snapshot.activeOwnerID) === 3, "reversed plan selected the retry encounter order");
    assert(ownerRenderCount(state, "reversed-b") === 2, "reversed plan B retry render count");
    assert(ownerRenderCount(state, "reversed-c") === 2, "reversed plan C retry render count");
    assertNoPendingState(state, "reversed plan retry");
    return resultSummary(state, frames);
}

async function runPartialOwnershipPlan() {
    let state = await navigateScenario("plan-partial");
    assertCommitted(state, metadataA, 1, "partial plan initial owner");
    let before = boundary(state);
    let frames = await clickAndCapture("activate-plan-partial", 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "partial plan failed publication");
    assertSequence(frames, [metadataA], "partial plan failure frames");
    assertNoDocumentPublication(state, before, "partial plan failure");
    assertPendingPlan(state, [metadataB.title, metadataC.title], 1, 0, "partial plan initial pending state");

    before = boundary(state);
    frames = await clickAndCapture("retry-plan-partial-b", 3);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "partial plan B readiness");
    assertSequence(frames, [metadataA], "partial plan B readiness frames");
    assertNoDocumentPublication(state, before, "partial plan B readiness");
    assertPendingPlan(state, [metadataB.title, metadataC.title], 1, 0, "partial plan B readiness state");
    assert(ownerRenderCount(state, "partial-b") === 2, "partial plan B readiness render count");
    assert(ownerRenderCount(state, "partial-c") === 1, "partial plan C rendered during B readiness");

    const rendersBeforeShell = ownerRenderCount(state, "partial-b");
    before = boundary(state);
    frames = await clickAndCapture("update-partial-shell", 3);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "partial plan unrelated update");
    assertSequence(frames, [metadataA], "partial plan unrelated frames");
    assertDeltas(state, before, { title: 0, description: 0, applies: 0, baseline: 0 }, "partial plan unrelated update");
    assert(state.shell["partial-shell"] === "1", "partial plan shell did not update");
    assert(ownerRenderCount(state, "partial-b") === rendersBeforeShell, "partial plan unrelated update rerendered B");

    before = boundary(state);
    frames = await clickAndCapture("retry-plan-partial-c", 5);
    state = await pageState();
    assertCommitted(state, metadataC, 2, "partial plan final retry");
    assertSequence(frames, [metadataA, metadataC], "partial plan final frames");
    assertSequence(observerSequenceSince(state, before), [metadataA, metadataC], "partial plan final observer");
    assertOwnerOrder(state, [2, 3], [metadataB.title, metadataC.title], "partial plan final order");
    assert(ownerRenderCount(state, "partial-b") === 2, "partial plan rerendered ready B a third time");
    assert(ownerRenderCount(state, "partial-c") === 2, "partial plan C final render count");
    assertNoPendingState(state, "partial plan final retry");
    return resultSummary(state, frames);
}

async function runAdditiveOwnershipPlan() {
    let state = await navigateScenario("plan-additive");
    assertCommitted(state, metadataA, 1, "additive plan initial owner");
    let before = boundary(state);
    let frames = await clickAndCapture("activate-plan-additive", 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "additive plan failed publication");
    assertSequence(frames, [metadataA], "additive plan failure frames");
    assertNoDocumentPublication(state, before, "additive plan failure");
    assertOwnerOrder(state, [1], [metadataA.title], "additive plan committed owner");
    assertPendingPlan(state, [metadataB.title], 0, 0, "additive plan pending owner");
    assert(cleanupCount(state, "additive-a") === 0, "additive plan cleaned up A");

    const rendersBeforeShell = ownerRenderCount(state, "additive-b");
    before = boundary(state);
    frames = await clickAndCapture("update-additive-shell", 3);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "additive plan unrelated update");
    assertSequence(frames, [metadataA], "additive plan unrelated frames");
    assertDeltas(state, before, { title: 0, description: 0, applies: 0, baseline: 0 }, "additive plan unrelated update");
    assert(ownerRenderCount(state, "additive-b") === rendersBeforeShell, "additive shell rerendered B");

    before = boundary(state);
    frames = await clickAndCapture("abandon-plan-additive", 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "additive plan abandonment");
    assertSequence(frames, [metadataA], "additive plan abandonment frames");
    assertNoDocumentPublication(state, before, "additive plan abandonment");
    assert(cleanupCount(state, "additive-b") === 1, "additive plan B cleanup count");
    assert(cleanupCount(state, "additive-a") === 0, "additive plan A cleanup count");
    assert(state.runtime.statistics.baselineRestorations === 0, "additive plan restored baseline");
    assertNoPendingState(state, "additive plan abandonment");
    const abandonment = resultSummary(state, frames);

    state = await navigateScenario("plan-additive");
    await clickAndCapture("activate-plan-additive", 5);
    state = await pageState();
    assertPendingPlan(state, [metadataB.title], 0, 0, "additive retry pending owner");
    before = boundary(state);
    frames = await clickAndCapture("retry-plan-additive", 5);
    state = await pageState();
    assertCommitted(state, metadataB, 2, "additive plan retry");
    assertSequence(frames, [metadataA, metadataB], "additive plan retry frames");
    assertSequence(observerSequenceSince(state, before), [metadataA, metadataB], "additive plan retry observer");
    assertOwnerOrder(state, [1, 2], [metadataA.title, metadataB.title], "additive plan retry order");
    assert(cleanupCount(state, "additive-a") === 0, "additive retry cleaned up A");
    assertNoPendingState(state, "additive plan retry");
    return { abandonment, retry: resultSummary(state, frames) };
}

async function runInitialOwnershipPlan() {
    let state = await navigateScenario("plan-initial");
    assertCommitted(state, baseline, 0, "initial plan authored baseline");
    let before = boundary(state);
    let frames = await clickAndCapture("activate-plan-initial", 5);
    state = await pageState();
    assertCommitted(state, baseline, 0, "initial plan failed publication");
    assertSequence(frames, [baseline], "initial plan failure frames");
    assertNoDocumentPublication(state, before, "initial plan failure");
    assertPendingPlan(state, [metadataB.title], 0, 0, "initial plan pending owner");
    assert(state.runtime.statistics.committedIDAssignments === 0, "initial plan assigned B an ID");

    const rendersBeforeShell = ownerRenderCount(state, "initial-b");
    before = boundary(state);
    frames = await clickAndCapture("update-initial-shell", 3);
    state = await pageState();
    assertCommitted(state, baseline, 0, "initial plan unrelated update");
    assertSequence(frames, [baseline], "initial plan unrelated frames");
    assertDeltas(state, before, { title: 0, description: 0, applies: 0, baseline: 0 }, "initial plan unrelated update");
    assert(ownerRenderCount(state, "initial-b") === rendersBeforeShell, "initial shell rerendered B");

    before = boundary(state);
    frames = await clickAndCapture("abandon-plan-initial", 5);
    state = await pageState();
    assertCommitted(state, baseline, 0, "initial plan abandonment");
    assertSequence(frames, [baseline], "initial plan abandonment frames");
    assertNoDocumentPublication(state, before, "initial plan abandonment");
    assert(cleanupCount(state, "initial-b") === 1, "initial plan B cleanup count");
    assert(state.runtime.statistics.documentPublications === 0, "initial plan published metadata");
    assert(state.runtime.statistics.baselineRestorations === 0, "initial plan counted a baseline restoration");
    assertNoPendingState(state, "initial plan abandonment");
    const abandonment = resultSummary(state, frames);

    state = await navigateScenario("plan-initial");
    await clickAndCapture("activate-plan-initial", 5);
    state = await pageState();
    assertPendingPlan(state, [metadataB.title], 0, 0, "initial retry pending owner");
    before = boundary(state);
    frames = await clickAndCapture("retry-plan-initial", 5);
    state = await pageState();
    assertCommitted(state, metadataB, 1, "initial plan retry");
    assertSequence(frames, [baseline, metadataB], "initial plan retry frames");
    assertSequence(observerSequenceSince(state, before), [baseline, metadataB], "initial plan retry observer");
    assertOwnerOrder(state, [1], [metadataB.title], "initial plan retry order");
    assertNoPendingState(state, "initial plan retry");
    return { abandonment, retry: resultSummary(state, frames) };
}

async function runFinalizationAbsorption(scenario, prefix) {
    let state = await navigateScenario(scenario);
    assertCommitted(state, metadataA, 1, `${prefix} finalization initial owner`);
    let before = boundary(state);
    let frames = await clickAndCapture(`${prefix}-fail-boundary`, 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, `${prefix} finalization retained owner`);
    assertSequence(frames, [metadataA], `${prefix} finalization failure frames`);
    assertNoDocumentPublication(state, before, `${prefix} finalization render failure`);
    assert(Number(state.runtime.snapshot.failedBoundaryCount) === 1, `${prefix} failed boundary count`);
    assert(Number(state.runtime.snapshot.retainedReleaseCount) === 1, `${prefix} retained release count`);
    assert(cleanupCount(state, `${prefix}-a`) === 1, `${prefix} owner A cleanup count`);

    before = boundary(state);
    frames = await clickAndCapture(`${prefix}-recover-ownerless`, 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, `${prefix} ownerless publication failure`);
    assertSequence(frames, [metadataA], `${prefix} ownerless failure frames`);
    assertNoDocumentPublication(state, before, `${prefix} ownerless publication failure`);
    assert(Number(state.runtime.snapshot.pendingPlanCount) === 0, `${prefix} ownerless failure created a plan`);
    assert(Number(state.runtime.snapshot.pendingFinalizations) === 1, `${prefix} ownerless finalization was not retained`);
    assert(state.runtime.runtimeErrors === 2, `${prefix} ownerless runtime error count`);

    before = boundary(state);
    frames = await clickAndCapture(`${prefix}-mount-successor`, 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, `${prefix} successor publication failure`);
    assertSequence(frames, [metadataA], `${prefix} successor failure frames`);
    assertNoDocumentPublication(state, before, `${prefix} successor publication failure`);
    assertPendingPlan(state, [metadataB.title], 1, 1, `${prefix} absorbed finalization`);
    assert(state.runtime.runtimeErrors === 3, `${prefix} successor runtime error count`);
    assert(!state.unexpectedYFallback, `${prefix} boundary Y entered fallback`);

    const rendersBeforeShell = ownerRenderCount(state, `${prefix}-b`);
    before = boundary(state);
    frames = await clickAndCapture(`update-${prefix}-shell`, 3);
    state = await pageState();
    assertCommitted(state, metadataA, 1, `${prefix} unrelated update`);
    assertSequence(frames, [metadataA], `${prefix} unrelated frames`);
    assertDeltas(state, before, { title: 0, description: 0, applies: 0, baseline: 0 }, `${prefix} unrelated update`);
    assert(ownerRenderCount(state, `${prefix}-b`) === rendersBeforeShell, `${prefix} unrelated update rerendered B`);
    assertPendingPlan(state, [metadataB.title], 1, 1, `${prefix} plan after unrelated update`);

    before = boundary(state);
    frames = await clickAndCapture(`${prefix}-retry-successor`, 5);
    state = await pageState();
    assertCommitted(state, metadataB, 1, `${prefix} successor retry`);
    assertSequence(frames, [metadataA, metadataB], `${prefix} successor retry frames`);
    assertSequence(observerSequenceSince(state, before), [metadataA, metadataB], `${prefix} successor retry observer`);
    assertOwnerOrder(state, [2], [metadataB.title], `${prefix} successor retry order`);
    assert(Number(state.runtime.snapshot.failedBoundaryCount) === 0, `${prefix} retained failed boundary`);
    assert(Number(state.runtime.snapshot.retainedReleaseCount) === 0, `${prefix} retained release after retry`);
    assert(cleanupCount(state, `${prefix}-a`) === 1, `${prefix} replayed A cleanup`);
    assert(ownerRenderCount(state, `${prefix}-b`) === 2, `${prefix} B retry render count`);
    assert(state.runtime.statistics.baselineRestorations === 0, `${prefix} exposed authored baseline`);
    assertNoPendingState(state, `${prefix} successor retry`);
    return resultSummary(state, frames);
}

async function runNewerBoundaryFailure() {
    let state = await navigateScenario("plan-newer-boundary-failure");
    assertCommitted(state, metadataA, 1, "newer boundary initial owner");
    let before = boundary(state);
    let frames = await clickAndCapture("supersede-fail-initial", 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "newer boundary first failure");
    assertSequence(frames, [metadataA], "newer boundary first failure frames");
    assertNoDocumentPublication(state, before, "newer boundary first failure");
    assert(Number(state.runtime.snapshot.failedBoundaryCount) === 1, "newer boundary first failed state");

    before = boundary(state);
    frames = await clickAndCapture("supersede-recover-ownerless", 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "newer boundary ownerless publication failure");
    assertSequence(frames, [metadataA], "newer boundary ownerless failure frames");
    assertNoDocumentPublication(state, before, "newer boundary ownerless publication failure");
    assert(Number(state.runtime.snapshot.pendingFinalizations) === 1, "newer boundary finalization missing");

    before = boundary(state);
    frames = await clickAndCapture("supersede-mount-b", 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "newer boundary pending B failure");
    assertSequence(frames, [metadataA], "newer boundary pending B frames");
    assertNoDocumentPublication(state, before, "newer boundary pending B failure");
    assertPendingPlan(state, [metadataB.title], 1, 1, "newer boundary pending B plan");
    assert(ownerRenderCount(state, "supersede-b") === 1, "newer boundary B initial render count");

    before = boundary(state);
    frames = await clickAndCapture("supersede-fail-newer", 5);
    state = await pageState();
    assertCommitted(state, metadataA, 1, "newer boundary superseding failure");
    assertSequence(frames, [metadataA], "newer boundary superseding failure frames");
    assertNoDocumentPublication(state, before, "newer boundary superseding failure");
    assert(Number(state.runtime.snapshot.failedBoundaryCount) === 1, "newer boundary did not remain failed");
    assert(Number(state.runtime.snapshot.retainedReleaseCount) === 1, "newer boundary lost A release");
    assert(cleanupCount(state, "supersede-b") === 1, "newer boundary B cleanup count");
    assertNoPendingState(state, "newer boundary superseding failure");
    assert(state.runtime.runtimeErrors === 4, "newer boundary runtime error count");

    before = boundary(state);
    frames = await clickAndCapture("supersede-final-recover", 5);
    state = await pageState();
    assertCommitted(state, baseline, 0, "newer boundary final recovery");
    assertSequence(frames, [metadataA, baseline], "newer boundary final recovery frames");
    assertSequence(observerSequenceSince(state, before), [metadataA, baseline], "newer boundary final recovery observer");
    assert(cleanupCount(state, "supersede-a") === 1, "newer boundary replayed A cleanup");
    assert(cleanupCount(state, "supersede-b") === 1, "newer boundary replayed B cleanup");
    assert(state.runtime.statistics.baselineRestorations === 1, "newer boundary baseline restoration count");
    assertNoPendingState(state, "newer boundary final recovery");
    return resultSummary(state, frames);
}

async function navigateScenario(scenario) {
    await client.call("Page.navigate", {
        url: `${origin}/?mode=${encodeURIComponent(activeMode)}&scenario=${encodeURIComponent(scenario)}&run=${Date.now()}`,
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

async function clickAndCaptureElementIdentity(testID, selector, count = 4) {
    return client.evaluate(`new Promise((resolve, reject) => {
        const element = document.querySelector('[data-testid="${testID}"]');
        const tracked = document.querySelector(${JSON.stringify(selector)});
        if (!element || !tracked) {
            reject(new Error('missing identity control ${testID} or ${selector}'));
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
                resolve({
                    frames,
                    same: tracked === document.querySelector(${JSON.stringify(selector)}),
                });
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
            if (last.app && last.mode === activeMode && last.scenario === scenario &&
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
        window.goframeDocumentHandoffRefreshEvidence?.();
        const title = document.querySelector('head title');
        const description = document.querySelector('head meta[name="description"]');
        const head = window.__goframeDocumentHandoffHead;
        const runtime = window.goframeDocumentHandoffEvidence;
        return {
            app: Boolean(document.querySelector('[data-testid="handoff-app"]')),
            ownerlessApp: Boolean(document.querySelector('[data-testid="ownerless-app"]')),
            retryVisible: Boolean(document.querySelector('[data-testid="retry-owner"]')),
            retryPreMetadataVisible: Boolean(document.querySelector('[data-testid="retry-pre-metadata"]')),
            retrySiblingVisible: Boolean(document.querySelector('[data-testid="retry-sibling-failure"]')),
            recoverOwnerlessVisible: Boolean(document.querySelector('[data-testid="recover-ownerless"]')),
            retryNestedOuterVisible: Boolean(document.querySelector('[data-testid="retry-nested-outer"]')),
            ownerlessRecoveryContent: Boolean(document.querySelector('[data-testid="ownerless-recovery-content"]')),
            nestedOwnerlessContent: Boolean(document.querySelector('[data-testid="nested-ownerless-content"]')),
            unexpectedInnerFallback: Boolean(document.querySelector('[data-testid="unexpected-inner-fallback"]')),
            unexpectedYFallback: Boolean(document.querySelector('[data-testid$="-unexpected-y-fallback"]')),
            handleConflictFallback: Boolean(document.querySelector('[data-testid="handle-conflict-fallback"]')),
            componentChildParentTestID: document.querySelector('[data-testid="component-contract-child"]')?.parentElement?.getAttribute('data-testid') ?? '',
            publicationNonce: document.querySelector('[data-testid="publication-nonce"]')?.textContent ?? '',
            shell: Object.fromEntries(
                [...document.querySelectorAll('[data-shell-role]')].map((element) => [
                    element.getAttribute('data-shell-role') ?? '',
                    element.textContent ?? '',
                ]),
            ),
            mode: document.querySelector('[data-testid="handoff-app"]')?.getAttribute('data-mode') ?? '',
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

function assertNoDocumentPublication(state, before, label) {
    assert(
        state.head.titleMutations === before.titleMutations &&
            state.head.descriptionMutations === before.descriptionMutations,
        `${label} mutated authored metadata nodes`,
    );
    assert(
        Number(state.runtime.statistics.documentPublications) ===
            Number(before.statistics.documentPublications),
        `${label} committed a document publication`,
    );
    assert(
        Number(state.runtime.statistics.baselineRestorations) ===
            Number(before.statistics.baselineRestorations),
        `${label} restored the authored baseline`,
    );
}

function assertOwnerOrder(state, ids, titles, label) {
    assert(
        JSON.stringify(state.runtime.snapshot.ownerIDs) === JSON.stringify(ids),
        `${label} owner IDs = ${JSON.stringify(state.runtime.snapshot.ownerIDs)}, want ${JSON.stringify(ids)}`,
    );
    assert(
        JSON.stringify(state.runtime.snapshot.ownerTitles) === JSON.stringify(titles),
        `${label} owner titles = ${JSON.stringify(state.runtime.snapshot.ownerTitles)}, want ${JSON.stringify(titles)}`,
    );
}

function assertPendingPlan(state, titles, retainedReleases, finalizations, label) {
    const snapshot = state.runtime.snapshot;
    assert(Number(snapshot.pendingPlanCount) === 1, `${label} plan count = ${snapshot.pendingPlanCount}`);
    assert(Number(snapshot.pendingOwnerCount) === titles.length, `${label} owner count = ${snapshot.pendingOwnerCount}`);
    assert(
        JSON.stringify(snapshot.pendingOwnerTitles) === JSON.stringify(titles),
        `${label} owner titles = ${JSON.stringify(snapshot.pendingOwnerTitles)}, want ${JSON.stringify(titles)}`,
    );
    assert(
        JSON.stringify(snapshot.pendingOwnerIDs) === JSON.stringify(titles.map(() => 0)),
        `${label} owner IDs = ${JSON.stringify(snapshot.pendingOwnerIDs)}, want pending ID zero`,
    );
    assert(
        Number(snapshot.retainedReleaseCount) === retainedReleases,
        `${label} retained releases = ${snapshot.retainedReleaseCount}, want ${retainedReleases}`,
    );
    assert(
        Number(snapshot.pendingFinalizations) === finalizations,
        `${label} pending finalizations = ${snapshot.pendingFinalizations}, want ${finalizations}`,
    );
    assert(!snapshot.batchActive, `${label} left the batch active`);
}

function assertNoPendingState(state, label) {
    const snapshot = state.runtime.snapshot;
    assert(Number(snapshot.pendingPlanCount) === 0, `${label} retained a pending plan`);
    assert(Number(snapshot.pendingOwnerCount) === 0, `${label} retained pending owners`);
    assert(Number(snapshot.pendingFinalizations) === 0, `${label} retained finalization authority`);
    assert(!snapshot.batchActive, `${label} left the batch active`);
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

function cleanupCount(state, role) {
    return state.runtime.ownerCleanups.filter((entry) => entry.role === role).length;
}

function ownerRenderCount(state, role) {
    return state.runtime.ownerRenders.filter((entry) => entry.role === role).length;
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
        publicationFailures: state.runtime.publicationFailures,
        snapshot: state.runtime.snapshot,
        ownerCleanups: state.runtime.ownerCleanups,
        runtimeReports: state.runtime.runtimeReports,
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

function hashJSON(value) {
    return createHash("sha256").update(JSON.stringify(value)).digest("hex");
}

async function buildFixture() {
    const workspaceRoot = join(tempRoot, "workspace");
    await mkdir(workspaceRoot, { recursive: true });
    for (const name of ["go.mod", "cmd", "pkg"]) {
        await cp(join(rootDir, name), join(workspaceRoot, name), { recursive: true });
    }
    const fixtureDir = join(
        workspaceRoot,
        "scripts",
        "fixtures",
        "document-metadata-api-shape-v2",
    );
    await mkdir(dirname(fixtureDir), { recursive: true });
    await cp(sourceFixtureDir, fixtureDir, { recursive: true });

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
    await runCommand("go", [
        "run",
        "./cmd/goxc",
        "generate",
        "scripts/fixtures/document-metadata-api-shape-v2/cmd/app",
        "--in-place",
    ], commonEnvironment, workspaceRoot);
    const generatedGOX = await readFile(join(fixtureDir, "cmd", "app", "projections.gox.go"));
    let runtimeSource;
    if (compiler === "go") {
        await runCommand("go", [
            "build",
            "-buildvcs=false",
            "-trimpath",
            "-ldflags=-buildid=",
            "-tags=goframe_document_state_experiment,goframe_debug",
            "-o",
            wasm,
            fixturePackage,
        ], {
            ...commonEnvironment,
            GOOS: "js",
            GOARCH: "wasm",
            CGO_ENABLED: "0",
        }, workspaceRoot);
        const goRoot = (await runCapture("go", ["env", "GOROOT"], commonEnvironment, workspaceRoot)).trim();
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
        ], commonEnvironment, workspaceRoot);
        const tinyGoRoot = (await runCapture("tinygo", ["env", "TINYGOROOT"], commonEnvironment, workspaceRoot)).trim();
        runtimeSource = join(tinyGoRoot, "targets", "wasm_exec.js");
    }
    await copyFile(runtimeSource, join(tempRoot, "wasm_exec.js"));
    await copyFile(join(fixtureDir, "assets", "index.html"), join(tempRoot, "index.html"));
    await copyFile(join(fixtureDir, "assets", "styles.css"), join(tempRoot, "styles.css"));
    const bytes = await readFile(wasm);
    return {
        bytes: bytes.length,
        sha256: createHash("sha256").update(bytes).digest("hex"),
        generatedGOXSHA256: createHash("sha256").update(generatedGOX).digest("hex"),
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
    return createCDPClient(socket);
}

function runCommand(command, args, environment, workingDirectory = rootDir) {
    return new Promise((resolveCommand, reject) => {
        const child = spawn(command, args, {
            cwd: workingDirectory,
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

function runCapture(command, args, environment, workingDirectory = rootDir) {
    return runCommand(command, args, environment, workingDirectory);
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

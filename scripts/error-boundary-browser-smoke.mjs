import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const appURL = process.argv[2] ?? process.env.GOFRAME_ERROR_BOUNDARY_SMOKE_URL ?? "http://127.0.0.1:18080/";
const successfulStateOnly = process.env.GOFRAME_ERROR_BOUNDARY_SUCCESS_ONLY === "1";
const debugPort = Number(process.env.GOFRAME_ERROR_BOUNDARY_CHROME_DEBUG_PORT ?? "19241");
const chrome = process.env.CHROME ?? "google-chrome";
const profile = await mkdtemp(join(tmpdir(), "goframe-error-boundary-smoke-"));
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
    await navigateToApp(client, withSmokeParam(appURL, "error-boundary"));
    await waitForAppPage(client, expectedApp, "error-boundary navigation");
    await waitForText(client, "[data-testid='eb-protected-state']", "0", "initial protected state");
    await waitForProbe(client, (probe) => probe.effectCount === 1, "initial effect setup");
    await captureShell(client);
    await installListenerAudit(client);

    const stateSuccessEvidence = await exerciseSuccessfulStateTransactions(client);
    if (successfulStateOnly) {
        console.log(`State transaction successful-path counters: ${JSON.stringify(stateSuccessEvidence)}`);
    } else {
        const stateFailureEvidence = await exerciseFailedStateTransactions(client);

    await waitForProbe(client, (current) =>
        current.local.ownerRenders >= 1 &&
        current.local.siblingRenders >= 1 &&
        current.local.siblingEffectSetups >= 1 &&
        current.local.nestedInnerOwnerRenders >= 1 &&
        current.local.nestedInnerSiblingRenders >= 1 &&
        current.local.nestedInnerSiblingSetups >= 1 &&
        current.local.nestedOuterSiblingRenders >= 1 &&
        current.local.nestedOuterSiblingSetups >= 1,
    "initial local update probes");

    await waitForText(
        client,
        "[data-testid='eb-dirty-batch-later-version']",
        "A",
        "initial captured dirty batch later owner",
    );
    await waitForText(
        client,
        "[data-testid='eb-dirty-batch-independent']",
        "0",
        "initial captured dirty batch independent owner",
    );
    await waitForProbe(client, (current) =>
        current.dirtyBatch.failingOwnerRenders === 1 &&
        current.dirtyBatch.laterARenders === 1 &&
        current.dirtyBatch.aEffectSetups === 1 &&
        current.dirtyBatch.aResourceStarts === 1 &&
        current.dirtyBatch.independentOwnerRenders === 1,
    "initial captured dirty batch lifecycle");

    await waitForSelector(
        client,
        "[data-testid='eb-nested-fallback-protected']",
        "initial nested fallback protected child",
    );
    await waitForAbsent(
        client,
        "[data-testid='eb-nested-fallback-owner']",
        "initial nested fallback owner absent",
    );
    await waitForAbsent(
        client,
        "[data-testid='eb-nested-fallback-outer']",
        "initial nested fallback outer absent",
    );
    await waitForProbe(client, (current) =>
        current.nestedFallback.ownerRenders === 0 &&
        current.nestedFallback.effectSetups === 0 &&
        current.nestedFallback.effectCleanups === 0 &&
        current.nestedFallback.unmountCallbacks === 0 &&
        current.nestedFallback.resourceStarts === 0 &&
        current.nestedFallback.resourceCleanups === 0 &&
        current.nestedFallback.laterSiblingSetups === 0,
    "initial nested fallback transaction counters");

    await waitForSelector(
        client,
        "[data-testid='eb-teardown-removed']",
        "initial removable lifecycle child",
    );
    await waitForText(
        client,
        "[data-testid='eb-teardown-replaced']",
        "replaced A",
        "initial replaceable lifecycle child",
    );
    await waitForAbsent(
        client,
        "[data-testid='eb-teardown-fallback']",
        "initial teardown fallback absent",
    );
    await waitForProbe(client, (current) =>
        current.teardown.ownerRenders === 1 &&
        current.teardown.removedEffectSetups === 1 &&
        current.teardown.removedEffectCleanups === 0 &&
        current.teardown.removedUnmountCallbacks === 0 &&
        current.teardown.removedResourceStarts === 1 &&
        current.teardown.removedResourceCleanups === 0 &&
        current.teardown.replacedEffectSetups === 1 &&
        current.teardown.replacedEffectCleanups === 0 &&
        current.teardown.replacedUnmountCallbacks === 0 &&
        current.teardown.replacedResourceStarts === 1 &&
        current.teardown.replacedResourceCleanups === 0 &&
        current.teardown.fallbackRenders === 0 &&
        current.teardown.boundaryReports === 0 &&
        current.teardown.order === "",
    "initial protected teardown lifecycle");

    const beforeTeardownFailure = await probe(client);
    await click(client, "[data-testid='eb-trigger-teardown-error']");
    await waitForSelector(
        client,
        "[data-testid='eb-teardown-fallback']",
        "protected teardown fallback",
    );
    await waitForAbsent(
        client,
        "[data-testid='eb-teardown-owner']",
        "failed teardown owner removed",
    );
    await waitForText(
        client,
        "[data-testid='eb-teardown-error-component']",
        "TeardownRiskyDescendant",
        "protected teardown report component",
    );
    await waitForProbe(client, (current) =>
        current.reports.length === beforeTeardownFailure.reports.length + 1 &&
        current.reports.at(-1).component === "TeardownRiskyDescendant" &&
        current.teardown.ownerRenders === 2 &&
        current.teardown.removedEffectSetups === 1 &&
        current.teardown.removedEffectCleanups === 1 &&
        current.teardown.removedUnmountCallbacks === 1 &&
        current.teardown.removedResourceStarts === 1 &&
        current.teardown.removedResourceCleanups === 1 &&
        current.teardown.replacedEffectSetups === 1 &&
        current.teardown.replacedEffectCleanups === 1 &&
        current.teardown.replacedUnmountCallbacks === 1 &&
        current.teardown.replacedResourceStarts === 1 &&
        current.teardown.replacedResourceCleanups === 1 &&
        current.teardown.fallbackRenders === 1 &&
        current.teardown.boundaryReports === 1,
    "protected teardown cleanup");
    const afterTeardownFailure = await probe(client);
    const teardownOrdering = assertProtectedTeardownOrdering(
        afterTeardownFailure.teardown.order,
        "protected teardown fallback",
    );
    await assertShellSame(client, "protected teardown fallback");

    await click(client, "[data-testid='eb-teardown-retry']");
    await waitForText(
        client,
        "[data-testid='eb-teardown-replaced']",
        "replaced B",
        "healthy teardown retry",
    );
    await waitForAbsent(
        client,
        "[data-testid='eb-teardown-fallback']",
        "teardown fallback cleared",
    );
    await waitForProbe(client, (current) =>
        current.reports.length === afterTeardownFailure.reports.length &&
        current.teardown.ownerRenders === 3 &&
        current.teardown.removedEffectSetups === 1 &&
        current.teardown.removedEffectCleanups === 1 &&
        current.teardown.removedUnmountCallbacks === 1 &&
        current.teardown.removedResourceStarts === 1 &&
        current.teardown.removedResourceCleanups === 1 &&
        current.teardown.replacedEffectSetups === 2 &&
        current.teardown.replacedEffectCleanups === 1 &&
        current.teardown.replacedUnmountCallbacks === 1 &&
        current.teardown.replacedResourceStarts === 2 &&
        current.teardown.replacedResourceCleanups === 1 &&
        current.teardown.fallbackRenders === 1 &&
        current.teardown.boundaryReports === 1 &&
        current.teardown.order === afterTeardownFailure.teardown.order &&
        current.listenerAudit.add === current.listenerAudit.remove,
    "healthy protected teardown retry");
    await assertShellSame(client, "protected teardown retry");
    const afterTeardownRetry = await probe(client);
    const protectedTeardownEvidence = {
        ...afterTeardownRetry.teardown,
        boundaryReportDelta:
            afterTeardownRetry.reports.length -
            beforeTeardownFailure.reports.length,
        fallbackIndex: teardownOrdering.fallbackIndex,
        earliestTeardownIndex: teardownOrdering.earliestTeardownIndex,
    };

    const beforeDirtyBatch = await probe(client);
    await click(client, "[data-testid='eb-trigger-dirty-batch']");
    await waitForSelector(
        client,
        "[data-testid='eb-dirty-batch-fallback']",
        "captured dirty batch fallback",
    );
    await waitForText(
        client,
        "[data-testid='eb-dirty-batch-error-component']",
        "DirtyBatchRiskyDescendant",
        "captured dirty batch report component",
    );
    await waitForText(
        client,
        "[data-testid='eb-dirty-batch-independent']",
        "1",
        "captured dirty batch independent update",
    );
    await waitForProbe(client, (current) =>
        current.reports.length === beforeDirtyBatch.reports.length + 1 &&
        current.reports.at(-1).component === "DirtyBatchRiskyDescendant" &&
        current.dirtyBatch.setterOrder === "failing B,later B,independent 1" &&
        current.dirtyBatch.renderOrder === "failing B,risky B,independent 1" &&
        current.dirtyBatch.attemptedBRenders === 0 &&
        current.dirtyBatch.attemptedBEffectSetups === 0 &&
        current.dirtyBatch.attemptedBEffectCleanups === 0 &&
        current.dirtyBatch.attemptedBUnmounts === 0 &&
        current.dirtyBatch.attemptedBResourceStarts === 0 &&
        current.dirtyBatch.attemptedBResourceCleanups === 0 &&
        current.dirtyBatch.aEffectCleanups === 1 &&
        current.dirtyBatch.aUnmountCallbacks === 1 &&
        current.dirtyBatch.aResourceCleanups === 1 &&
        current.dirtyBatch.independentOwnerRenders ===
            beforeDirtyBatch.dirtyBatch.independentOwnerRenders + 1,
    "captured dirty batch discard");
    await assertShellSame(client, "captured dirty batch fallback");
    const afterDirtyBatch = await probe(client);
    const dirtyBatchEvidence = {
        ...afterDirtyBatch.dirtyBatch,
        boundaryReports:
            afterDirtyBatch.reports.length - beforeDirtyBatch.reports.length,
        independentRenderDelta:
            afterDirtyBatch.dirtyBatch.independentOwnerRenders -
            beforeDirtyBatch.dirtyBatch.independentOwnerRenders,
    };

    const localEvidence = {};
    const beforeLocalUpdate = await probe(client);
    await click(client, "[data-testid='eb-local-owner-update']");
    await waitForText(client, "[data-testid='eb-local-owner-state']", "1", "local owner state update");
    const afterLocalUpdate = await probe(client);
    localEvidence.ownerRenderDelta = counterDelta(
        afterLocalUpdate.local.ownerRenders,
        beforeLocalUpdate.local.ownerRenders,
        1,
        "local owner render",
    );
    localEvidence.siblingRenderDelta = counterDelta(
        afterLocalUpdate.local.siblingRenders,
        beforeLocalUpdate.local.siblingRenders,
        0,
        "unrelated sibling render",
    );
    localEvidence.siblingEveryRenderDelta = counterDelta(
        afterLocalUpdate.local.siblingEffectSetups,
        beforeLocalUpdate.local.siblingEffectSetups,
        0,
        "unrelated sibling EveryRender setup",
    );
    counterDelta(
        afterLocalUpdate.reports.length,
        beforeLocalUpdate.reports.length,
        0,
        "local update boundary report",
    );
    counterDelta(
        afterLocalUpdate.listenerAudit.add,
        beforeLocalUpdate.listenerAudit.add,
        0,
        "local update listener addition",
    );
    counterDelta(
        afterLocalUpdate.listenerAudit.remove,
        beforeLocalUpdate.listenerAudit.remove,
        0,
        "local update listener removal",
    );
    await waitForAbsent(client, "[data-testid='eb-local-unexpected-fallback']", "local update fallback stays inactive");
    await assertShellSame(client, "successful local owner update");

    const beforeNestedLocalUpdate = await probe(client);
    await click(client, "[data-testid='eb-nested-local-owner-update']");
    await waitForText(client, "[data-testid='eb-nested-local-owner-state']", "1", "nested local owner state update");
    const afterNestedLocalUpdate = await probe(client);
    localEvidence.nestedInnerOwnerRenderDelta = counterDelta(
        afterNestedLocalUpdate.local.nestedInnerOwnerRenders,
        beforeNestedLocalUpdate.local.nestedInnerOwnerRenders,
        1,
        "nested inner owner render",
    );
    localEvidence.nestedInnerSiblingRenderDelta = counterDelta(
        afterNestedLocalUpdate.local.nestedInnerSiblingRenders,
        beforeNestedLocalUpdate.local.nestedInnerSiblingRenders,
        0,
        "nested inner sibling render",
    );
    localEvidence.nestedInnerSiblingEveryRenderDelta = counterDelta(
        afterNestedLocalUpdate.local.nestedInnerSiblingSetups,
        beforeNestedLocalUpdate.local.nestedInnerSiblingSetups,
        0,
        "nested inner sibling EveryRender setup",
    );
    localEvidence.nestedOuterSiblingRenderDelta = counterDelta(
        afterNestedLocalUpdate.local.nestedOuterSiblingRenders,
        beforeNestedLocalUpdate.local.nestedOuterSiblingRenders,
        0,
        "nested outer sibling render",
    );
    counterDelta(
        afterNestedLocalUpdate.reports.length,
        beforeNestedLocalUpdate.reports.length,
        0,
        "nested local update boundary report",
    );
    await waitForAbsent(
        client,
        "[data-testid='eb-nested-local-unexpected-fallback']",
        "nested local update fallback stays inactive",
    );
    await assertShellSame(client, "successful nested local owner update");

    await waitForText(client, "[data-testid='eb-local-transaction-version']", "A", "initial local transaction version");
    await waitForProbe(client, (current) =>
        current.localTransaction.aEffectSetups === 1 &&
        current.localTransaction.aResourceStarts === 1 &&
        current.localTransaction.aLaterSiblingSetups === 1 &&
        current.localTransaction.aEffectCleanups === 0 &&
        current.localTransaction.aUnmountCallbacks === 0 &&
        current.localTransaction.aResourceCleanups === 0,
    "initial local protected transaction lifecycle");
    const beforeLocalTransactionFailure = await probe(client);

    await click(client, "[data-testid='eb-local-transaction-trigger']");
    await waitForSelector(client, "[data-testid='eb-local-transaction-fallback']", "local transaction boundary fallback");
    await waitForAbsent(client, "[data-testid='eb-local-transaction-owner']", "failed local transaction owner removed");
    await waitForText(
        client,
        "[data-testid='eb-local-transaction-error-component']",
        "LocalTransactionRiskyDescendant",
        "local transaction descendant component",
    );
    await waitForProbe(client, (current) =>
        current.reports.length === beforeLocalTransactionFailure.reports.length + 1 &&
        current.reports.at(-1).phase === "render" &&
        current.reports.at(-1).component === "LocalTransactionRiskyDescendant" &&
        current.localTransaction.attemptedBEffectSetups === 0 &&
        current.localTransaction.attemptedBUnmountCallbacks === 0 &&
        current.localTransaction.attemptedBResourceStarts === 0 &&
        current.localTransaction.attemptedBResourceCleanups === 0 &&
        current.localTransaction.attemptedBLaterSetups === 0 &&
        current.localTransaction.aEffectCleanups === 1 &&
        current.localTransaction.aUnmountCallbacks === 1 &&
        current.localTransaction.aResourceCleanups === 1,
    "failed local protected transaction rollback");
    await assertShellSame(client, "local protected transaction fallback");

    const afterLocalTransactionFailure = await probe(client);
    localEvidence.localTransactionBoundaryReports =
        afterLocalTransactionFailure.reports.length - beforeLocalTransactionFailure.reports.length;
    await click(client, "[data-testid='eb-local-transaction-retry']");
    await waitForText(client, "[data-testid='eb-local-transaction-version']", "B", "local transaction retry version");
    await waitForAbsent(client, "[data-testid='eb-local-transaction-fallback']", "local transaction fallback cleared");
    await waitForProbe(client, (current) =>
        current.reports.length === afterLocalTransactionFailure.reports.length &&
        current.localTransaction.retryBEffectSetups === 1 &&
        current.localTransaction.retryBResourceStarts === 1 &&
        current.localTransaction.retryBLaterSetups === 1 &&
        current.localTransaction.attemptedBEffectSetups === 0 &&
        current.localTransaction.attemptedBUnmountCallbacks === 0 &&
        current.localTransaction.attemptedBResourceStarts === 0 &&
        current.localTransaction.attemptedBLaterSetups === 0,
    "healthy local protected transaction retry");
    await assertShellSame(client, "local protected transaction retry");

    await click(client, "[data-testid='eb-protected-increment']");
    await waitForText(client, "[data-testid='eb-protected-state']", "1", "protected state before failure");
    await waitForProbe(client, (probe) => probe.effectCount === 2 && probe.cleanupCount === 1, "effect rerun before failure");
    const beforeFailure = await probe(client);

    await click(client, "[data-testid='eb-trigger-render-error']");
    await waitForSelector(client, "[data-testid='eb-fallback']", "boundary fallback");
    await waitForAbsent(client, "[data-testid='eb-protected']", "failed protected subtree removed");
    await waitForText(client, "[data-testid='eb-error-component']", "RiskyPanel", "fallback component name");
    await waitForText(client, "[data-testid='eb-error-operation']", "component render", "fallback operation");
    await waitForProbe(client, (current) =>
        current.reports.length === beforeFailure.reports.length + 1 &&
        current.reports.at(-1).phase === "render" &&
        current.reports.at(-1).component === "RiskyPanel" &&
        current.effectCount === beforeFailure.effectCount &&
        current.cleanupCount === beforeFailure.cleanupCount + 1,
    "render failure report and cleanup");
    await assertShellSame(client, "shell after fallback");

    const afterFallback = await probe(client);
    await click(client, "[data-testid='eb-retry']");
    await waitForSelector(client, "[data-testid='eb-protected']", "protected subtree after retry");
    await waitForAbsent(client, "[data-testid='eb-fallback']", "fallback cleared after retry");
    await waitForText(client, "[data-testid='eb-protected-state']", "0", "retry remounts fresh protected state");
    await waitForProbe(client, (current) =>
        current.reports.length === afterFallback.reports.length &&
        current.listenerAudit.add === current.listenerAudit.remove,
    "retry does not report or leak listeners");

    const beforeSecondFailure = await probe(client);
    await click(client, "[data-testid='eb-trigger-render-error']");
    await waitForSelector(client, "[data-testid='eb-fallback']", "second fallback");
    await waitForProbe(client, (current) => current.reports.length === beforeSecondFailure.reports.length + 1, "second incident report");
    await click(client, "[data-testid='eb-reset-key']");
    await waitForSelector(client, "[data-testid='eb-protected']", "ResetKey clears fallback");
    await waitForAbsent(client, "[data-testid='eb-fallback']", "fallback cleared by ResetKey");

    const beforeNested = await probe(client);
    await click(client, "[data-testid='eb-trigger-nested-error']");
    await waitForSelector(client, "[data-testid='eb-nested-inner-fallback']", "nested inner fallback");
    await waitForAbsent(client, "[data-testid='eb-nested-outer-fallback']", "outer fallback stays inactive");
    await waitForProbe(client, (current) => current.reports.length === beforeNested.reports.length + 1, "nested inner report");

    const beforeFallbackPanic = await probe(client);
    await click(client, "[data-testid='eb-trigger-inner-fallback-panic']");
    await waitForSelector(client, "[data-testid='eb-nested-outer-fallback']", "inner fallback panic bubbles to outer");
    await waitForAbsent(client, "[data-testid='eb-nested-inner-fallback']", "inner fallback removed after outer capture");
    await waitForProbe(client, (current) =>
        current.reports.length === beforeFallbackPanic.reports.length + 1 &&
        current.reports.at(-1).component === "InnerFallback",
    "fallback component panic report");
    await waitForStableReportCount(client, beforeFallbackPanic.reports.length + 1, "fallback component panic");
    await assertShellSame(client, "shell after nested fallback panic");
    await assertListenerNetStable(client, "nested fallback panic");

    const beforeNestedFallbackTransaction = await probe(client);
    await click(client, "[data-testid='eb-trigger-nested-fallback-transaction']");
    await waitForSelector(
        client,
        "[data-testid='eb-nested-fallback-outer']",
        "nested fallback descendant bubbles to outer",
    );
    await waitForAbsent(
        client,
        "[data-testid='eb-nested-fallback-owner']",
        "failed nested fallback lifecycle owner removed",
    );
    await waitForProbe(client, (current) =>
        current.reports.length === beforeNestedFallbackTransaction.reports.length + 2 &&
        current.reports.at(-2).component === "InitialRiskyChild" &&
        current.reports.at(-1).component === "FallbackRiskyDescendant" &&
        current.nestedFallback.ownerRenders === 1 &&
        current.nestedFallback.effectSetups === 0 &&
        current.nestedFallback.effectCleanups === 0 &&
        current.nestedFallback.unmountCallbacks === 0 &&
        current.nestedFallback.resourceStarts === 0 &&
        current.nestedFallback.resourceCleanups === 0 &&
        current.nestedFallback.laterSiblingSetups === 0,
    "nested fallback transaction rollback");
    await waitForStableReportCount(
        client,
        beforeNestedFallbackTransaction.reports.length + 2,
        "nested fallback transaction",
    );
    await assertShellSame(client, "nested fallback transaction");
    await assertListenerNetStable(client, "nested fallback transaction");
    const afterNestedFallbackTransaction = await probe(client);
    const nestedFallbackReports = afterNestedFallbackTransaction.reports.slice(
        beforeNestedFallbackTransaction.reports.length,
    );
    const nestedFallbackEvidence = {
        ...afterNestedFallbackTransaction.nestedFallback,
        innerReports: nestedFallbackReports.filter(
            (report) => report.component === "InitialRiskyChild",
        ).length,
        outerReports: nestedFallbackReports.filter(
            (report) => report.component === "FallbackRiskyDescendant",
        ).length,
    };

    await waitForText(client, "[data-testid='eb-transaction-version']", "A", "initial transaction version");
    await waitForProbe(client, (current) =>
        current.transaction.aEffectSetups === 1 &&
        current.transaction.aResourceStarts === 1 &&
        current.transaction.aLaterSiblingSetups === 1 &&
        current.transaction.aEffectCleanups === 0 &&
        current.transaction.aUnmountCallbacks === 0 &&
        current.transaction.aResourceCleanups === 0,
    "initial protected transaction lifecycle");
    const beforeTransactionFailure = await probe(client);

    await click(client, "[data-testid='eb-trigger-transaction-error']");
    await waitForSelector(client, "[data-testid='eb-transaction-fallback']", "transaction boundary fallback");
    await waitForAbsent(client, "[data-testid='eb-transaction-owner']", "failed transaction owner removed");
    await waitForText(
        client,
        "[data-testid='eb-transaction-error-component']",
        "TransactionRiskyDescendant",
        "transaction descendant component",
    );
    await waitForProbe(client, (current) =>
        current.reports.length === beforeTransactionFailure.reports.length + 1 &&
        current.reports.at(-1).phase === "render" &&
        current.reports.at(-1).component === "TransactionRiskyDescendant" &&
        current.transaction.attemptedBEffectSetups === 0 &&
        current.transaction.attemptedBUnmountCallbacks === 0 &&
        current.transaction.attemptedBResourceStarts === 0 &&
        current.transaction.attemptedBResourceCleanups === 0 &&
        current.transaction.attemptedBLaterSetups === 0 &&
        current.transaction.aEffectCleanups === 1 &&
        current.transaction.aUnmountCallbacks === 1 &&
        current.transaction.aResourceCleanups === 1,
    "failed protected transaction rollback");
    await assertShellSame(client, "protected transaction fallback");

    const afterTransactionFailure = await probe(client);
    const transactionBoundaryReports =
        afterTransactionFailure.reports.length - beforeTransactionFailure.reports.length;
    await click(client, "[data-testid='eb-transaction-retry']");
    await waitForText(client, "[data-testid='eb-transaction-version']", "B", "transaction retry version");
    await waitForAbsent(client, "[data-testid='eb-transaction-fallback']", "transaction fallback cleared");
    await waitForProbe(client, (current) =>
        current.reports.length === afterTransactionFailure.reports.length &&
        current.transaction.retryBEffectSetups === 1 &&
        current.transaction.retryBResourceStarts === 1 &&
        current.transaction.retryBLaterSetups === 1 &&
        current.transaction.attemptedBEffectSetups === 0 &&
        current.transaction.attemptedBUnmountCallbacks === 0 &&
        current.transaction.attemptedBResourceStarts === 0 &&
        current.transaction.attemptedBLaterSetups === 0 &&
        current.listenerAudit.add === current.listenerAudit.remove,
    "healthy protected transaction retry");
    await assertShellSame(client, "protected transaction retry");

    const beforeNoBoundary = await probe(client);
    await click(client, "[data-testid='eb-trigger-no-boundary-error']");
    await waitForAbsent(client, "[data-testid='eb-no-boundary-healthy']", "no-boundary subtree default Empty fallback");
    await waitForProbe(client, (current) =>
        current.reports.length === beforeNoBoundary.reports.length + 1 &&
        current.reports.at(-1).component === "NoBoundaryRisky",
    "no-boundary render report");
    await assertShellSame(client, "shell after no-boundary failure");

    const finalProbe = await probe(client);
    console.log(`Error boundary protected transaction counters: ${JSON.stringify({
        ...finalProbe.transaction,
        localEvidence,
        localUpdate: finalProbe.local,
        localTransaction: finalProbe.localTransaction,
        nestedFallback: nestedFallbackEvidence,
        capturedDirtyBatch: dirtyBatchEvidence,
        protectedTeardown: protectedTeardownEvidence,
        stateTransactions: stateFailureEvidence,
        stateSuccess: stateSuccessEvidence,
        boundaryReports: transactionBoundaryReports,
        shellIdentityChanges: finalProbe.shellIdentityChanges,
        listenerAdditions: finalProbe.listenerAudit.add,
        listenerRemovals: finalProbe.listenerAudit.remove,
    })}`);
    }
    client.close();
    console.log(successfulStateOnly
        ? "State transaction TinyGo successful-path browser smoke: ok"
        : "Error boundary browser smoke: ok");
} finally {
    const exited = new Promise((resolve) => browser.once("exit", resolve));
    browser.kill("SIGTERM");
    await Promise.race([exited, wait(2000)]);
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function exerciseSuccessfulStateTransactions(client) {
    await waitForText(client, "[data-testid='eb-state-success-first']", "2", "successful render-time state update");
    await waitForText(client, "[data-testid='eb-state-success-second']", "second", "successful second state slot");
    await waitForText(client, "[data-testid='eb-state-success-reducer']", "11", "successful render-time reducer dispatch");
    await waitForText(
        client,
        "[data-testid='eb-state-success-resource']",
        "ready:ready-aligned",
        "successful state resource alignment",
    );
    await waitForProbe(client, (current) =>
        current.stateSuccess.renderTimeSetters === 1 &&
        current.stateSuccess.renderTimeDispatches === 1 &&
        current.stateSuccess.resourceStarts === 1 &&
        current.stateSuccess.resourceCleanups === 0 &&
        current.stateSuccess.firstValue === 2 &&
        current.stateSuccess.secondValue === "second" &&
        current.stateSuccess.reducerValue === 11 &&
        current.stateSuccess.resourceStatus === "ready" &&
        current.stateSuccess.resourceValue === "ready-aligned",
    "initial successful state transaction");
    await captureIdentity(client, "stateSuccessHost", "[data-testid='eb-state-success-host']");
    await captureIdentity(client, "stateSuccessOwner", "[data-testid='eb-state-success-owner']");

    const initial = await probe(client);
    await click(client, "[data-testid='eb-state-success-update']");
    await waitForText(client, "[data-testid='eb-state-success-first']", "3", "successful ordinary state update");
    await waitForProbe(client, (current) =>
        current.stateSuccess.stateClicks === initial.stateSuccess.stateClicks + 1 &&
        current.stateSuccess.ownerRenders === initial.stateSuccess.ownerRenders + 1 &&
        current.stateSuccess.resourceStarts === initial.stateSuccess.resourceStarts,
    "single successful state update");
    await assertIdentitySame(client, "stateSuccessHost", "[data-testid='eb-state-success-host']", "successful host state update");
    await assertIdentitySame(client, "stateSuccessOwner", "[data-testid='eb-state-success-owner']", "successful owner state update");

    const beforeReplacement = await probe(client);
    await click(client, "[data-testid='eb-state-success-replace-reducer']");
    await waitForText(client, "[data-testid='eb-state-success-reducer-mode']", "candidate", "successful reducer replacement");
    await waitForProbe(client, (current) =>
        current.stateSuccess.reducerReplaceClicks === beforeReplacement.stateSuccess.reducerReplaceClicks + 1 &&
        current.stateSuccess.ownerRenders === beforeReplacement.stateSuccess.ownerRenders + 1 &&
        current.stateSuccess.reducerValue === 11 &&
        current.stateSuccess.resourceStarts === beforeReplacement.stateSuccess.resourceStarts,
    "successful reducer replacement commit");
    await assertIdentitySame(client, "stateSuccessOwner", "[data-testid='eb-state-success-owner']", "successful reducer replacement");

    const beforeDispatch = await probe(client);
    await click(client, "[data-testid='eb-state-success-dispatch']");
    await waitForText(client, "[data-testid='eb-state-success-reducer']", "111", "old dispatch uses successful reducer replacement");
    await waitForProbe(client, (current) =>
        current.stateSuccess.reducerDispatchClicks === beforeDispatch.stateSuccess.reducerDispatchClicks + 1 &&
        current.stateSuccess.ownerRenders === beforeDispatch.stateSuccess.ownerRenders + 1 &&
        current.stateSuccess.reducerValue === 111,
    "old dispatch after successful reducer replacement");
    await assertIdentitySame(client, "stateSuccessOwner", "[data-testid='eb-state-success-owner']", "successful reducer dispatch");

    const beforeUnmount = await probe(client);
    await click(client, "[data-testid='eb-state-success-toggle']");
    await waitForAbsent(client, "[data-testid='eb-state-success-owner']", "successful state owner unmount");
    await waitForText(client, "[data-testid='eb-state-success-unmounted']", "unmounted", "successful state owner unmounted marker");
    await waitForProbe(client, (current) =>
        current.stateSuccess.resourceCleanups === beforeUnmount.stateSuccess.resourceCleanups + 1,
    "successful state resource cleanup");
    await assertIdentitySame(client, "stateSuccessHost", "[data-testid='eb-state-success-host']", "successful state owner unmount");

    await click(client, "[data-testid='eb-state-success-toggle']");
    await waitForText(client, "[data-testid='eb-state-success-first']", "2", "successful state owner replacement state");
    await waitForText(client, "[data-testid='eb-state-success-reducer']", "11", "successful state owner replacement reducer");
    await waitForText(
        client,
        "[data-testid='eb-state-success-resource']",
        "ready:ready-aligned",
        "successful state owner replacement resource",
    );
    await waitForProbe(client, (current) =>
        current.stateSuccess.resourceStarts === beforeUnmount.stateSuccess.resourceStarts + 1 &&
        current.stateSuccess.resourceCleanups === beforeUnmount.stateSuccess.resourceCleanups + 1 &&
        current.stateSuccess.renderTimeSetters === beforeUnmount.stateSuccess.renderTimeSetters + 1 &&
        current.stateSuccess.renderTimeDispatches === beforeUnmount.stateSuccess.renderTimeDispatches + 1,
    "successful state owner replacement lifecycle");
    await assertIdentitySame(client, "stateSuccessHost", "[data-testid='eb-state-success-host']", "successful state owner replacement host");
    await assertIdentityChanged(client, "stateSuccessOwner", "[data-testid='eb-state-success-owner']", "successful state owner replacement");

    const beforeReplacementUpdate = await probe(client);
    await click(client, "[data-testid='eb-state-success-update']");
    await waitForText(client, "[data-testid='eb-state-success-first']", "3", "replacement state owner update");
    await waitForProbe(client, (current) =>
        current.stateSuccess.stateClicks === beforeReplacementUpdate.stateSuccess.stateClicks + 1 &&
        current.stateSuccess.ownerRenders === beforeReplacementUpdate.stateSuccess.ownerRenders + 1,
    "single replacement state owner update");
    await assertIdentitySame(client, "stateSuccessOwner", "[data-testid='eb-state-success-owner']", "replacement state owner update");

    const final = await probe(client);
    const listenerAdditions = final.listenerAudit.add - initial.listenerAudit.add;
    const listenerRemovals = final.listenerAudit.remove - initial.listenerAudit.remove;
    if (listenerAdditions !== listenerRemovals) {
        throw new Error(
            `APP FAILURE: successful state listener delta add=${listenerAdditions} remove=${listenerRemovals}`,
        );
    }
    return {
        ...final.stateSuccess,
        listenerAdditions,
        listenerRemovals,
        hostIdentityChanges: 0,
        ownerReplacements: 1,
    };
}

async function exerciseFailedStateTransactions(client) {
    await captureIdentity(client, "stateInitialScenario", "[data-testid='eb-state-initial-scenario']");
    await captureIdentity(client, "stateReducerScenario", "[data-testid='eb-state-reducer-scenario']");
    await waitForText(client, "[data-testid='eb-state-reducer-value']", "1", "initial committed reducer value");
    const initial = await probe(client);

    await click(client, "[data-testid='eb-state-initial-start']");
    await waitForSelector(client, "[data-testid='eb-state-initial-fallback']", "failed initial state fallback");
    await waitForAbsent(client, "[data-testid='eb-state-initial-owner']", "failed initial state owner absent");
    await waitForProbe(client, (current) =>
        current.reports.length === initial.reports.length + 1 &&
        current.reports.at(-1).component === "StateTransactionInitialOwner" &&
        current.stateTransactions.initialOwnerRenders === initial.stateTransactions.initialOwnerRenders + 1 &&
        current.stateTransactions.initialFailedRenders === initial.stateTransactions.initialFailedRenders + 1 &&
        current.stateTransactions.initialFallbackRenders === initial.stateTransactions.initialFallbackRenders + 1,
    "failed initial state rollback");
    await assertIdentitySame(client, "stateInitialScenario", "[data-testid='eb-state-initial-scenario']", "failed initial state fallback");

    const afterInitialFailure = await probe(client);
    await click(client, "[data-testid='eb-state-initial-retry']");
    await waitForText(client, "[data-testid='eb-state-initial-value']", "99", "failed initial state retry value");
    await waitForText(client, "[data-testid='eb-state-initial-reducer']", "99", "failed initial reducer retry value");
    await waitForProbe(client, (current) =>
        current.reports.length === afterInitialFailure.reports.length &&
        current.stateTransactions.initialRecoveredRenders === afterInitialFailure.stateTransactions.initialRecoveredRenders + 1,
    "failed initial state retry commit");
    await captureIdentity(client, "stateInitialOwner", "[data-testid='eb-state-initial-owner']");

    const beforeFailedClosures = await probe(client);
    await click(client, "[data-testid='eb-state-invoke-failed-closures']");
    await waitForProbe(client, (current) =>
        current.stateTransactions.failedClosureInvocations ===
            beforeFailedClosures.stateTransactions.failedClosureInvocations + 1,
    "discarded state closure invocation");
    await wait(250);
    const afterFailedClosures = await probe(client);
    counterDelta(
        afterFailedClosures.stateTransactions.initialRecoveredRenders,
        beforeFailedClosures.stateTransactions.initialRecoveredRenders,
        0,
        "discarded state closure render",
    );
    counterDelta(
        afterFailedClosures.reports.length,
        beforeFailedClosures.reports.length,
        0,
        "discarded state closure report",
    );
    await waitForText(client, "[data-testid='eb-state-initial-value']", "99", "discarded setter remains inert");
    await waitForText(client, "[data-testid='eb-state-initial-reducer']", "99", "discarded dispatch remains inert");
    await assertIdentitySame(client, "stateInitialOwner", "[data-testid='eb-state-initial-owner']", "discarded state closures");

    const beforeCurrentUpdate = await probe(client);
    await click(client, "[data-testid='eb-state-update-current']");
    await waitForText(client, "[data-testid='eb-state-initial-value']", "100", "committed state setter after retry");
    await waitForText(client, "[data-testid='eb-state-initial-reducer']", "100", "committed reducer dispatch after retry");
    await waitForProbe(client, (current) =>
        current.stateTransactions.currentUpdateInvocations ===
            beforeCurrentUpdate.stateTransactions.currentUpdateInvocations + 1 &&
        current.stateTransactions.initialRecoveredRenders ===
            beforeCurrentUpdate.stateTransactions.initialRecoveredRenders + 1,
    "committed state closures after retry");
    await assertIdentitySame(client, "stateInitialOwner", "[data-testid='eb-state-initial-owner']", "committed state closures");

    await click(client, "[data-testid='eb-state-initial-finish']");
    await waitForSelector(client, "[data-testid='eb-state-initial-start']", "failed initial state scenario reset");
    await waitForAbsent(client, "[data-testid='eb-state-initial-owner']", "recovered initial state owner unmounted");
    await waitForAbsent(client, "[data-testid='eb-state-initial-fallback']", "initial state fallback remains cleared");
    await assertIdentitySame(client, "stateInitialScenario", "[data-testid='eb-state-initial-scenario']", "failed initial state scenario finish");

    const beforeReducerFailure = await probe(client);
    await click(client, "[data-testid='eb-state-reducer-fail']");
    await waitForText(client, "[data-testid='eb-state-reducer-mode']", "failed", "failed reducer candidate mode");
    await waitForAbsent(client, "[data-testid='eb-state-reducer-value']", "failed reducer candidate output");
    await waitForProbe(client, (current) =>
        current.reports.length === beforeReducerFailure.reports.length + 1 &&
        current.reports.at(-1).component === "StateTransactionReducerOwner" &&
        current.stateTransactions.reducerFailedRenders ===
            beforeReducerFailure.stateTransactions.reducerFailedRenders + 1,
    "failed reducer replacement rollback");
    await assertIdentitySame(client, "stateReducerScenario", "[data-testid='eb-state-reducer-scenario']", "failed reducer replacement");

    const afterReducerFailure = await probe(client);
    await click(client, "[data-testid='eb-state-reducer-recover']");
    await waitForText(client, "[data-testid='eb-state-reducer-mode']", "base", "reducer recovery mode");
    await waitForText(client, "[data-testid='eb-state-reducer-value']", "1", "committed reducer retained after failure");
    await waitForProbe(client, (current) => current.reports.length === afterReducerFailure.reports.length, "reducer recovery without report");
    await captureIdentity(client, "stateReducerOwner", "[data-testid='eb-state-reducer-value']");

    await click(client, "[data-testid='eb-state-reducer-dispatch']");
    await waitForText(client, "[data-testid='eb-state-reducer-value']", "2", "old dispatch uses committed reducer after failure");
    await assertIdentitySame(client, "stateReducerOwner", "[data-testid='eb-state-reducer-value']", "committed reducer dispatch after failure");

    await click(client, "[data-testid='eb-state-reducer-commit']");
    await waitForText(client, "[data-testid='eb-state-reducer-mode']", "candidate", "successful reducer candidate mode");
    await waitForText(client, "[data-testid='eb-state-reducer-value']", "2", "successful reducer replacement retains state");
    await assertIdentitySame(client, "stateReducerOwner", "[data-testid='eb-state-reducer-value']", "successful reducer replacement");

    await click(client, "[data-testid='eb-state-reducer-dispatch']");
    await waitForText(client, "[data-testid='eb-state-reducer-value']", "102", "old dispatch uses latest committed reducer");
    await waitForProbe(client, (current) =>
        current.stateTransactions.committedDispatchInvokes ===
            beforeReducerFailure.stateTransactions.committedDispatchInvokes + 2 &&
        current.stateTransactions.lastCommittedReducerValue === 102,
    "latest committed reducer dispatch");
    await assertIdentitySame(client, "stateReducerOwner", "[data-testid='eb-state-reducer-value']", "latest committed reducer dispatch");
    await assertIdentitySame(client, "stateReducerScenario", "[data-testid='eb-state-reducer-scenario']", "reducer transaction scenario");

    const final = await probe(client);
    const listenerAdditions = final.listenerAudit.add - initial.listenerAudit.add;
    const listenerRemovals = final.listenerAudit.remove - initial.listenerAudit.remove;
    if (listenerAdditions !== listenerRemovals) {
        throw new Error(
            `APP FAILURE: failed state listener delta add=${listenerAdditions} remove=${listenerRemovals}`,
        );
    }
    return {
        ...final.stateTransactions,
        initialBoundaryReports: afterInitialFailure.reports.length - initial.reports.length,
        reducerReports: final.reports.length - beforeReducerFailure.reports.length,
        listenerAdditions,
        listenerRemovals,
        scenarioIdentityChanges: 0,
    };
}

async function captureIdentity(client, key, selector) {
    const captured = await client.callFunction(`function(key, selector) {
        const element = document.querySelector(selector);
        if (!element) return false;
        window.__errorBoundaryIdentities ||= {};
        window.__errorBoundaryIdentities[key] = element;
        return true;
    }`, key, selector);
    if (!captured) {
        throw new Error(`APP FAILURE: missing ${selector} for ${key} identity capture`);
    }
}

async function assertIdentitySame(client, key, selector, label) {
    const same = await client.callFunction(`function(key, selector) {
        return window.__errorBoundaryIdentities?.[key] === document.querySelector(selector);
    }`, key, selector);
    if (!same) {
        throw new Error(`APP FAILURE: ${key} identity changed during ${label}`);
    }
}

async function assertIdentityChanged(client, key, selector, label) {
    const changed = await client.callFunction(`function(key, selector) {
        const element = document.querySelector(selector);
        if (!element || window.__errorBoundaryIdentities?.[key] === element) return false;
        window.__errorBoundaryIdentities[key] = element;
        return true;
    }`, key, selector);
    if (!changed) {
        throw new Error(`APP FAILURE: ${key} identity did not change during ${label}`);
    }
}

async function installListenerAudit(client) {
    await client.evaluate(`(() => {
        if (window.__errorBoundaryListenerAudit) return true;
        const originalAdd = EventTarget.prototype.addEventListener;
        const originalRemove = EventTarget.prototype.removeEventListener;
        const audit = { add: 0, remove: 0 };
        EventTarget.prototype.addEventListener = function(...args) {
            audit.add++;
            return originalAdd.apply(this, args);
        };
        EventTarget.prototype.removeEventListener = function(...args) {
            audit.remove++;
            return originalRemove.apply(this, args);
        };
        window.__errorBoundaryListenerAudit = audit;
        return true;
    })()`);
}

async function captureShell(client) {
    const ok = await client.evaluate(`(() => {
        window.__errorBoundaryShell = document.querySelector("[data-testid='eb-shell']");
        window.__errorBoundaryShellIdentityChanges = 0;
        return Boolean(window.__errorBoundaryShell);
    })()`);
    if (!ok) {
        throw new Error("APP FAILURE: missing shell for identity capture");
    }
}

async function assertShellSame(client, label) {
    const same = await client.evaluate(`(() => {
        const same = window.__errorBoundaryShell === document.querySelector("[data-testid='eb-shell']");
        if (!same) window.__errorBoundaryShellIdentityChanges++;
        return same;
    })()`);
    if (!same) {
        throw new Error(`APP FAILURE: shell identity changed during ${label}`);
    }
}

async function probe(client) {
    return await client.evaluate(`(() => ({
        effectCount: globalThis.goframeErrorBoundaryEffectCount ?? 0,
        cleanupCount: globalThis.goframeErrorBoundaryCleanupCount ?? 0,
        reports: Array.from(globalThis.goframeErrorBoundaryReports || []),
        transaction: globalThis.goframeProtectedTransactionProbe || {},
        local: globalThis.goframeLocalUpdateProbe || {},
        localTransaction: globalThis.goframeLocalTransactionProbe || {},
        nestedFallback: globalThis.goframeNestedFallbackTransactionProbe || {},
        dirtyBatch: globalThis.goframeCapturedDirtyBatchProbe || {},
        teardown: globalThis.goframeProtectedTeardownProbe || {},
        stateTransactions: globalThis.goframeStateTransactionProbe || {},
        stateSuccess: globalThis.goframeStateTransactionSuccessProbe || {},
        shellIdentityChanges: globalThis.__errorBoundaryShellIdentityChanges || 0,
        listenerAudit: globalThis.__errorBoundaryListenerAudit || { add: 0, remove: 0 },
    }))()`);
}

function counterDelta(after, before, expected, label) {
    const delta = after - before;
    if (delta !== expected) {
        throw new Error(`APP FAILURE: ${label} delta = ${delta}, want ${expected}`);
    }
    return delta;
}

function assertProtectedTeardownOrdering(order, label) {
    const events = order.split(",").filter(Boolean);
    const fallbackIndex = events.indexOf("fallback-render");
    const teardownMarkers = [
        "removed-effect-cleanup",
        "removed-resource-cleanup",
        "removed-unmount",
        "replaced-effect-cleanup",
        "replaced-resource-cleanup",
        "replaced-unmount",
    ];
    const teardownIndices = teardownMarkers.map((marker) => {
        const index = events.indexOf(marker);
        if (index < 0) {
            throw new Error(
                `APP FAILURE: missing ${marker} during ${label}; order=${JSON.stringify(events)}`,
            );
        }
        if (events.indexOf(marker, index + 1) >= 0) {
            throw new Error(
                `APP FAILURE: duplicate ${marker} during ${label}; order=${JSON.stringify(events)}`,
            );
        }
        return index;
    });
    const earliestTeardownIndex = Math.min(...teardownIndices);
    if (fallbackIndex < 0 || teardownIndices.some((index) => index <= fallbackIndex)) {
        throw new Error(
            `APP FAILURE: protected teardown ran before fallback render during ${label}; ` +
            `fallbackIndex=${fallbackIndex}; earliestTeardownIndex=${earliestTeardownIndex}; ` +
            `order=${JSON.stringify(events)}`,
        );
    }
    return { fallbackIndex, earliestTeardownIndex };
}

async function click(client, selector) {
    const result = await client.callFunction(`function(selector) {
        const element = document.querySelector(selector);
        if (!element) return false;
        element.click();
        return true;
    }`, selector);
    if (!result) {
        throw new Error(`APP FAILURE: missing element for click ${selector}`);
    }
}

async function waitForProbe(client, predicate, label) {
    const started = Date.now();
    let last = null;
    while (Date.now() - started < 5000) {
        last = await probe(client);
        if (predicate(last)) {
            return;
        }
        await wait(100);
    }
    throw new Error(`APP FAILURE: timed out waiting for ${label}; last=${JSON.stringify(last)}`);
}

async function waitForStableReportCount(client, expected, label) {
    await waitForProbe(client, (current) => current.reports.length === expected, `${label} report count`);
    for (let i = 0; i < 5; i++) {
        await wait(100);
        const current = await probe(client);
        if (current.reports.length !== expected) {
            throw new Error(`APP FAILURE: unstable report count after ${label}; expected=${expected}; current=${JSON.stringify(current)}`);
        }
    }
}

async function assertListenerNetStable(client, label) {
    const current = await probe(client);
    if (current.listenerAudit.add !== current.listenerAudit.remove) {
        throw new Error(`APP FAILURE: listener net changed during ${label}; audit=${JSON.stringify(current.listenerAudit)}`);
    }
}

async function waitForSelector(client, selector, label) {
    await waitUntil(client, label, () =>
        client.callFunction(`function(selector) {
            return Boolean(document.querySelector(selector));
        }`, selector));
}

async function waitForText(client, selector, expected, label) {
    await waitUntil(client, label, () =>
        client.callFunction(`function(selector, expected) {
            const element = document.querySelector(selector);
            return element ? element.textContent === expected : false;
        }`, selector, expected));
}

async function waitForAbsent(client, selector, label) {
    await waitUntil(client, label, () =>
        client.callFunction(`function(selector) {
            return !document.querySelector(selector);
        }`, selector));
}

async function waitUntil(client, label, predicate) {
    const started = Date.now();
    let lastValue = null;
    while (Date.now() - started < 5000) {
        lastValue = await predicate();
        if (lastValue === true) {
            return;
        }
        await wait(100);
    }
    throw new Error(`APP FAILURE: timed out waiting for ${label}; last=${JSON.stringify(lastValue)}`);
}

async function waitForPage(port) {
    const started = Date.now();
    let lastError;
    while (Date.now() - started < 5000) {
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
    throw new Error(`HARNESS FAILURE: Chrome DevTools page unavailable: ${lastError?.message ?? browserError}`);
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
    const started = Date.now();
    while (Date.now() - started < 8000) {
        lastState = await pageState(client);
        if (lastState.href.startsWith("chrome-error://")) {
            throw await harnessFailure(client, `${label}: Chrome loaded an error document`, lastState);
        }
        if (isExpectedAppState(lastState, expected) && lastState.fixtureReady && lastState.storage === "available") {
            return lastState;
        }
        await wait(100);
    }
    throw await harnessFailure(client, `${label}: app page did not become ready`, lastState);
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
            fixtureReady: Boolean(document.querySelector("[data-testid='eb-shell']")),
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
                        throw new Error(`browser evaluation failed: ${JSON.stringify(response.exceptionDetails)}`);
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
                            throw new Error(`browser evaluation failed: ${JSON.stringify(globalResponse.exceptionDetails)}`);
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
                        throw new Error(`browser evaluation failed: ${JSON.stringify(response.exceptionDetails)}`);
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
            if (!message.id || !pending.has(message.id)) {
                return;
            }
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

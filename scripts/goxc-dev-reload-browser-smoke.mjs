import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdtemp, mkdir, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const repositoryRoot = process.cwd();
const chrome = process.env.CHROME ?? "google-chrome";
const root = await mkdtemp(join(tmpdir(), "goframe-dev-reload-smoke-"));
const appDir = join(root, "app");
const workspace = join(root, "workspace");
const profile = join(root, "chrome-profile");
const localGoxc = join(root, "bin", "goxc");
const goxc = process.env.GOXC || localGoxc;
const debugPort = Number(process.env.GOFRAME_DEV_RELOAD_CHROME_DEBUG_PORT || await freePort());
const generationRootsBefore = await listGenerationRoots();

let dev = null;
let browser = null;
let client1 = null;
let client2 = null;
let activationGapProbe = null;
let devOutput = "";
let browserError = "";
let buildErrorEvidence = null;
let bodyRootEvidence = null;
const lines = [];
const counters = {
    successfulPackageAttempts: 0,
    failedPackageAttempts: 0,
    completedGenerations: 0,
    generationActivations: 0,
    reloadEventsPublished: 0,
    catchUpReloads: 0,
    previousProcessCatchUpReloads: 0,
    embedReloads: 0,
    embedFailedAttempts: 0,
    publicationProbeBatches: 0,
    completeResponsesDuringBuild: 0,
    oldGenerationIndexResponses: 0,
    newGenerationIndexResponses: 0,
    responses404DuringBuild: 0,
    partialResponsesDuringBuild: 0,
};

try {
    await writeApplication("gox-initial", "go-initial", "index-initial");
    if (!process.env.GOXC) {
        await mkdir(dirname(localGoxc), { recursive: true });
        await runProcess("go", ["build", "-o", localGoxc, "./cmd/goxc"], { cwd: repositoryRoot });
    }

    dev = spawn(goxc, ["dev", appDir, "--compiler=go", "--port=0", `--workspace=${workspace}`], {
        cwd: repositoryRoot,
        env: {
            ...process.env,
            GOWORK: "off",
            GOPROXY: "off",
            GOSUMDB: "off",
            GOFLAGS: "-mod=mod",
        },
        stdio: ["ignore", "pipe", "pipe"],
    });
    captureLines(dev.stdout, "stdout");
    captureLines(dev.stderr, "stderr");
    const serverLine = await waitForLine((line) => line.text.includes("development server ready at "), 30_000);
    const serverURL = serverLine.text.match(/http:\/\/\S+/)?.[0];
    if (!serverURL) throw new Error(`HARNESS FAILURE: development URL missing from ${serverLine.text}`);
    await waitForBuild(1, "succeeded");
    counters.successfulPackageAttempts++;
    counters.completedGenerations++;
    counters.generationActivations++;

    const packageDir = await findCanonicalPackageDirectory(workspace);
    const canonicalIndex = await readFile(join(packageDir, "index.html"), "utf8");
    assert(!canonicalIndex.includes("data-goframe-dev-reload"), "canonical package index contains the development reload tag");
    const manifest = JSON.parse(await readFile(join(packageDir, "asset-manifest.json"), "utf8"));
    const wasmPath = manifest.entrypoints.wasm;

    browser = spawn(chrome, [
        "--headless",
        "--no-sandbox",
        "--disable-gpu",
        `--remote-debugging-port=${debugPort}`,
        `--user-data-dir=${profile}`,
        "about:blank",
    ], { stdio: ["ignore", "ignore", "pipe"] });
    browser.stderr.on("data", (chunk) => { browserError += chunk; });

    const initialTarget = await waitForPage(debugPort);
    client1 = await connect(initialTarget.webSocketDebuggerUrl);
    await prepareClient(client1);
    await navigate(client1, `${serverURL}/?smoke=initial`);
    const initial = await waitForPageState(client1, { gox: "gox-initial", go: "go-initial", index: "index-initial", embedded: "embed-alpha" });
    assert(initial.pageLoads === 1, `initial page loads = ${initial.pageLoads}, want 1`);
    assert(initial.generation === 1, `initial generation = ${initial.generation}, want 1`);
    assert(/^[0-9a-f]{32}$/.test(initial.instance), `initial process instance = ${JSON.stringify(initial.instance)}, want a 32-character hex token`);
    const activeInstance = initial.instance;
    await waitForEventSourceOpen(client1, 1);
    await wait(250);
    const stableInitial = await pageState(client1);
    assert(stableInitial.pageLoads === 1 && stableInitial.reloadEvents === 0, `initial page reloaded spontaneously: ${JSON.stringify(stableInitial)}`);
    console.log("initial load: ok");

    let nextBuild = 2;
    let activeGeneration = 1;

    const goxBaseline = await pageState(client1);
    const goxStart = lines.length;
    await writeGOX("gox-rebuild");
    await waitForBuildStarted(nextBuild, goxStart);
    const [, publicationEvidence] = await Promise.all([
        waitForBuild(nextBuild, "succeeded", goxStart),
        probeCompletedGenerationUntilBuildCompletes(serverURL, wasmPath, activeGeneration, nextBuild),
    ]);
    counters.publicationProbeBatches += publicationEvidence.batches;
    counters.completeResponsesDuringBuild += publicationEvidence.completeResponses;
    counters.oldGenerationIndexResponses += publicationEvidence.oldGenerationIndexResponses;
    counters.newGenerationIndexResponses += publicationEvidence.newGenerationIndexResponses;
    counters.responses404DuringBuild += publicationEvidence.responses404;
    counters.partialResponsesDuringBuild += publicationEvidence.partialResponses;
    assert(publicationEvidence.batches > 0, "publication probe issued no request batches");
    assert(publicationEvidence.completeResponses > 0, "publication probe observed no complete responses");
    assert(publicationEvidence.responses404 === 0, `publication probe observed ${publicationEvidence.responses404} HTTP 404 responses`);
    assert(publicationEvidence.partialResponses === 0, `publication probe observed ${publicationEvidence.partialResponses} partial responses`);
    await assertSuccessfulReload(client1, goxBaseline, { gox: "gox-rebuild", go: "go-initial", index: "index-initial" }, ++activeGeneration);
    recordSuccessfulReload();
    nextBuild++;
    console.log("GOX rebuild reload: ok");

    const goBaseline = await pageState(client1);
    await writeGoMessage("go-rebuild");
    await waitForBuild(nextBuild, "succeeded");
    await assertSuccessfulReload(client1, goBaseline, { gox: "gox-rebuild", go: "go-rebuild", index: "index-initial" }, ++activeGeneration);
    recordSuccessfulReload();
    nextBuild++;
    console.log("Go rebuild reload: ok");

    const indexBaseline = await pageState(client1);
    await writeIndex("index-rebuild");
    await waitForBuild(nextBuild, "succeeded");
    await assertSuccessfulReload(client1, indexBaseline, { gox: "gox-rebuild", go: "go-rebuild", index: "index-rebuild" }, ++activeGeneration);
    recordSuccessfulReload();
    nextBuild++;
    console.log("asset/index rebuild reload: ok");

    const burstBaseline = await pageState(client1);
    const burstStart = lines.length;
    for (const value of ["burst-one", "burst-two", "burst-final"]) {
        await writeGOX(value);
    }
    await waitForBuild(nextBuild, "succeeded", burstStart);
    await assertSuccessfulReload(client1, burstBaseline, { gox: "burst-final", go: "go-rebuild", index: "index-rebuild" }, ++activeGeneration);
    recordSuccessfulReload();
    nextBuild++;
    await wait(500);
    assert(countBuildStartsSince(burstStart) === 1, `burst saves produced ${countBuildStartsSince(burstStart)} package attempts, want 1`);
    console.log("burst rebuild collapse: ok");

    const failureBaseline = await pageState(client1);
    const failureStart = lines.length;
    const firstFailureBuild = nextBuild;
    await writeBrokenGOX();
    await waitForBuild(firstFailureBuild, "failed", failureStart);
    counters.failedPackageAttempts++;
    nextBuild++;
    const failureRoot = await fetch(`${serverURL}/`, { cache: "no-store" });
    const failureMetadata = await fetch(`${serverURL}/goframe-package.json`, { cache: "no-store" });
    assert(failureRoot.ok && failureMetadata.ok, `last successful package unavailable after failure: ${failureRoot.status}/${failureMetadata.status}`);
    const failureRootText = await failureRoot.text();
    const failureMetadataJSON = await failureMetadata.json();
    assert(!failureRootText.includes("data-goframe-dev-build-error"), "development error presentation entered the served package index");
    assert(failureMetadataJSON.version === 1, `last successful package metadata changed after failure: ${JSON.stringify(failureMetadataJSON)}`);
    assert(!(await readFile(join(packageDir, "index.html"), "utf8")).includes("data-goframe-dev-build-error"),
        "development error presentation entered the canonical package index");

    const afterFailure = await waitForBuildError(client1, firstFailureBuild, "empty expression for attribute");
    assertFailedBuildPreserved(failureBaseline, afterFailure, activeGeneration, "first failed build");
    assert(afterFailure.buildErrorEvents === failureBaseline.buildErrorEvents + 1,
        `first failed build events = ${afterFailure.buildErrorEvents}, want ${failureBaseline.buildErrorEvents + 1}`);
    assert(afterFailure.buildErrorMessage.includes("<img data-goframe-diagnostic-injected"),
        `first failure omitted markup-like source text: ${JSON.stringify(afterFailure.buildErrorMessage)}`);
    assert(afterFailure.injectedDiagnosticElements === 0 && !afterFailure.diagnosticExecuted,
        `first failure executed markup-like diagnostic text: ${JSON.stringify(afterFailure)}`);
    const interactiveAfterFailure = await exerciseInteraction(client1, afterFailure);
    assert(interactiveAfterFailure.appRootIdentity === failureBaseline.appRootIdentity,
        `first failed build replaced the application root: ${JSON.stringify({ failureBaseline, interactiveAfterFailure })}`);
    assertAuthoredMarker(interactiveAfterFailure, "first failed build interaction");
    assert(interactiveAfterFailure.authoredMarkerIdentity === failureBaseline.authoredMarkerIdentity,
        `first failed build interaction replaced the authored marker: ${JSON.stringify({ failureBaseline, interactiveAfterFailure })}`);
    console.log("first failed build presentation and interaction: ok");

    const failureSecondTarget = await client1.call("Target.createTarget", { url: "about:blank" });
    const failureSecondPage = await waitForTarget(debugPort, failureSecondTarget.targetId);
    client2 = await connect(failureSecondPage.webSocketDebuggerUrl);
    await prepareClient(client2);
    await navigate(client2, `${serverURL}/?smoke=build-error-second-client`);
    const failureSecondInitial = await waitForPageState(client2, {
        gox: "burst-final",
        go: "go-rebuild",
        index: "index-rebuild",
        generation: activeGeneration,
    });
    const failureSecondRetained = await waitForBuildError(client2, firstFailureBuild, "empty expression for attribute");
    assert(failureSecondRetained.pageLoads === failureSecondInitial.pageLoads
        && failureSecondRetained.reloadEvents === failureSecondInitial.reloadEvents,
        `retained failure reloaded the second page: ${JSON.stringify(failureSecondRetained)}`);
    assert(failureSecondRetained.buildErrorEvents === 1,
        `second page retained failure events = ${failureSecondRetained.buildErrorEvents}, want 1`);

    await removeDevBuildErrorPanel(client1);
    const replacementBaseline1 = await pageState(client1);
    assert(replacementBaseline1.buildErrorPresentations === 0 && replacementBaseline1.buildErrorMarkers === 1,
        `external removal did not leave only the authored marker: ${JSON.stringify(replacementBaseline1)}`);
    assertAuthoredMarker(replacementBaseline1, "external development-panel removal");
    assert(replacementBaseline1.authoredMarkerIdentity === afterFailure.authoredMarkerIdentity,
        `external development-panel removal replaced the authored marker: ${JSON.stringify({ afterFailure, replacementBaseline1 })}`);
    const replacementBaseline2 = await pageState(client2);
    const replacementFailureBuild = nextBuild;
    await writeReplacementBrokenGOX();
    await waitForBuild(replacementFailureBuild, "failed");
    counters.failedPackageAttempts++;
    nextBuild++;
    const [replacementFailure1, replacementFailure2] = await Promise.all([
        waitForBuildError(client1, replacementFailureBuild, "data-replacement"),
        waitForBuildError(client2, replacementFailureBuild, "data-replacement"),
    ]);
    assertFailedBuildPreserved(replacementBaseline1, replacementFailure1, activeGeneration, "replacement failed build on page 1");
    assertFailedBuildPreserved(replacementBaseline2, replacementFailure2, activeGeneration, "replacement failed build on page 2");
    for (const [label, state] of [["page 1", replacementFailure1], ["page 2", replacementFailure2]]) {
        assert(state.buildErrorMessage !== afterFailure.buildErrorMessage,
            `${label} retained the first failure message`);
        assert(!state.buildErrorMessage.includes("data-goframe-diagnostic-injected"),
            `${label} retained markup from the first failure`);
    }
    assert(replacementFailure1.devPanelIdentity !== afterFailure.devPanelIdentity,
        `page 1 reused its disconnected development panel: ${JSON.stringify({ afterFailure, replacementFailure1 })}`);
    assert(replacementFailure2.devPanelIdentity === replacementBaseline2.devPanelIdentity,
        `page 2 replaced its connected development panel: ${JSON.stringify({ replacementBaseline2, replacementFailure2 })}`);
    const interactiveSecondPage = await exerciseInteraction(client2, replacementFailure2);
    assert(interactiveSecondPage.appRootIdentity === replacementFailure2.appRootIdentity,
        `replacement failure interaction replaced page 2 root: ${JSON.stringify(interactiveSecondPage)}`);
    assertAuthoredMarker(interactiveSecondPage, "replacement failure interaction");
    assert(interactiveSecondPage.authoredMarkerIdentity === replacementFailure2.authoredMarkerIdentity,
        `replacement failure interaction replaced page 2 authored marker: ${JSON.stringify({ replacementFailure2, interactiveSecondPage })}`);
    console.log("consecutive failure replacement on two pages: ok");

    const reconnectBaseline = await pageState(client1);
    await reconnectReloadClient(client1, activeInstance, activeGeneration, true);
    await waitForEventSourceOpen(client1, reconnectBaseline.eventSourceOpens + 1);
    const afterFailureReconnect = await waitForBuildError(client1, replacementFailureBuild, "data-replacement");
    assertFailedBuildPreserved(reconnectBaseline, afterFailureReconnect, activeGeneration, "current-generation failure reconnect");
    assert(afterFailureReconnect.buildErrorEvents === reconnectBaseline.buildErrorEvents + 1,
        `current-generation reconnect events = ${afterFailureReconnect.buildErrorEvents}, want ${reconnectBaseline.buildErrorEvents + 1}`);
    assert(afterFailureReconnect.devPanelIdentity !== reconnectBaseline.devPanelIdentity,
        `current-generation reconnect adopted the removed development panel: ${JSON.stringify({ reconnectBaseline, afterFailureReconnect })}`);
    console.log("current-generation failure reconnect: ok");

    const malformedBaseline = await pageState(client1);
    await dispatchMalformedBuildError(client1);
    const afterMalformed = await pageState(client1);
    assert(afterMalformed.buildErrorEvents === malformedBaseline.buildErrorEvents
        && afterMalformed.buildErrorBuild === malformedBaseline.buildErrorBuild
        && afterMalformed.buildErrorMessage === malformedBaseline.buildErrorMessage,
        `malformed private event changed the presentation: ${JSON.stringify({ malformedBaseline, afterMalformed })}`);
    assert(client1.runtimeErrors.length === 0, `malformed private event caused runtime errors: ${JSON.stringify(client1.runtimeErrors)}`);

    const staleFailureBaseline = afterMalformed;
    await reconnectReloadClient(client1, activeInstance, activeGeneration - 1, true);
    const staleFailureCaughtUp = await waitForPageState(client1, {
        gox: "burst-final",
        go: "go-rebuild",
        index: "index-rebuild",
        pageLoads: staleFailureBaseline.pageLoads + 1,
        reloadEvents: staleFailureBaseline.reloadEvents + 1,
        generation: activeGeneration,
    });
    counters.catchUpReloads++;
    const failureAfterCatchUp = await waitForBuildError(client1, replacementFailureBuild, "data-replacement");
    assert(failureAfterCatchUp.buildErrorEvents === staleFailureBaseline.buildErrorEvents + 1,
        `stale catch-up failure events = ${failureAfterCatchUp.buildErrorEvents}, want ${staleFailureBaseline.buildErrorEvents + 1}`);
    assert(failureAfterCatchUp.pageLoads === staleFailureCaughtUp.pageLoads
        && failureAfterCatchUp.reloadEvents === staleFailureCaughtUp.reloadEvents,
        `current failure caused a second reload after stale catch-up: ${JSON.stringify(failureAfterCatchUp)}`);
    console.log("stale-generation reload precedes retained failure: ok");

    const recoveryBaseline1 = failureAfterCatchUp;
    const recoveryBaseline2 = interactiveSecondPage;
    await writeGOX("recovered");
    await waitForBuild(nextBuild, "succeeded");
    const recoveryGeneration = ++activeGeneration;
    const [recovered1, recovered2] = await Promise.all([
        assertSuccessfulReload(client1, recoveryBaseline1, { gox: "recovered", go: "go-rebuild", index: "index-rebuild" }, recoveryGeneration),
        assertSuccessfulReload(client2, recoveryBaseline2, { gox: "recovered", go: "go-rebuild", index: "index-rebuild" }, recoveryGeneration),
    ]);
    recordSuccessfulReload();
    nextBuild++;
    await Promise.all([
        waitForEventSourceOpen(client1, recoveryBaseline1.eventSourceOpens + 1),
        waitForEventSourceOpen(client2, recoveryBaseline2.eventSourceOpens + 1),
    ]);
    const stableRecovery1 = await pageState(client1);
    const stableRecovery2 = await pageState(client2);
    for (const [label, baseline, state] of [
        ["page 1", recoveryBaseline1, stableRecovery1],
        ["page 2", recoveryBaseline2, stableRecovery2],
    ]) {
        assert(state.buildErrorPresentations === 0, `${label} retained an error presentation after recovery: ${JSON.stringify(state)}`);
        assert(state.buildErrorEvents === baseline.buildErrorEvents,
            `${label} replayed a cleared failure after recovery: ${JSON.stringify({ baseline, state })}`);
        assert(state.panelCountBeforeUnload === 0,
            `${label} still had an error presentation when recovery reload was initiated: ${JSON.stringify(state)}`);
        assert(state.authoredMarkerCountBeforeUnload === 1
            && state.authoredMarkerIdentityBeforeUnload === baseline.authoredMarkerIdentity
            && state.authoredHeadingBeforeUnload === "authored heading"
            && state.authoredMessageBeforeUnload === "authored message",
        `${label} changed the authored marker before recovery reload: ${JSON.stringify({ baseline, state })}`);
        assertAuthoredMarker(state, `${label} recovery`);
    }
    assert(recovered1.generation === recoveryGeneration && recovered2.generation === recoveryGeneration,
        `recovery generations = ${recovered1.generation}/${recovered2.generation}, want ${recoveryGeneration}`);

    const clearedReconnectBaseline = stableRecovery1;
    await reconnectReloadClient(client1, activeInstance, activeGeneration, true);
    await waitForEventSourceOpen(client1, clearedReconnectBaseline.eventSourceOpens + 1);
    await wait(100);
    const afterClearedReconnect = await pageState(client1);
    assert(afterClearedReconnect.pageLoads === clearedReconnectBaseline.pageLoads
        && afterClearedReconnect.reloadEvents === clearedReconnectBaseline.reloadEvents
        && afterClearedReconnect.buildErrorEvents === clearedReconnectBaseline.buildErrorEvents
        && afterClearedReconnect.buildErrorPresentations === 0,
        `cleared failure replayed on current-generation reconnect: ${JSON.stringify(afterClearedReconnect)}`);
    assert(client1.runtimeErrors.length === 0 && client2.runtimeErrors.length === 0,
        `application runtime errors during failed-build recovery: ${JSON.stringify({ page1: client1.runtimeErrors, page2: client2.runtimeErrors })}`);
    buildErrorEvidence = {
        builds: [firstFailureBuild, replacementFailureBuild],
        firstPageEvents: afterClearedReconnect.buildErrorEvents,
        secondPageEvents: stableRecovery2.buildErrorEvents,
        recoveryGeneration,
        firstPageLoads: stableRecovery1.pageLoads,
        secondPageLoads: stableRecovery2.pageLoads,
        firstPageReloads: stableRecovery1.reloadEvents,
        secondPageReloads: stableRecovery2.reloadEvents,
        interactionCounts: [interactiveAfterFailure.interactionCount, interactiveSecondPage.interactionCount],
        runtimeErrors: 0,
    };
    client2.close();
    await client1.call("Target.closeTarget", { targetId: failureSecondTarget.targetId });
    client2 = null;
    console.log("failed build recovery reload on two pages: ok");

    const embedBaseline = await pageState(client1);
    await writeEmbeddedMessage("embed-beta");
    await waitForBuild(nextBuild, "succeeded");
    await assertSuccessfulReload(client1, embedBaseline, {
        gox: "recovered",
        go: "go-rebuild",
        index: "index-rebuild",
        embedded: "embed-beta",
    }, ++activeGeneration);
    counters.embedReloads++;
    recordSuccessfulReload();
    nextBuild++;
    console.log("embedded payload reload: ok");

    const removedEmbedBaseline = await pageState(client1);
    await rm(join(appDir, "embedded-message.txt"));
    await waitForBuild(nextBuild, "failed");
    counters.failedPackageAttempts++;
    counters.embedFailedAttempts++;
    nextBuild++;
    await wait(250);
    const afterEmbedRemoval = await pageState(client1);
    assert(afterEmbedRemoval.pageLoads === removedEmbedBaseline.pageLoads
        && afterEmbedRemoval.reloadEvents === removedEmbedBaseline.reloadEvents,
        `removed embedded payload reloaded the browser: ${JSON.stringify({ removedEmbedBaseline, afterEmbedRemoval })}`);
    assert(afterEmbedRemoval.embedded === "embed-beta", `removed embedded payload changed the active page: ${JSON.stringify(afterEmbedRemoval)}`);
    console.log("embedded payload removal preservation: ok");

    await writeEmbeddedMessage("embed-gamma");
    await waitForBuild(nextBuild, "succeeded");
    await assertSuccessfulReload(client1, afterEmbedRemoval, {
        gox: "recovered",
        go: "go-rebuild",
        index: "index-rebuild",
        embedded: "embed-gamma",
    }, ++activeGeneration);
    counters.embedReloads++;
    recordSuccessfulReload();
    nextBuild++;
    console.log("embedded payload recovery reload: ok");

    const secondTarget = await client1.call("Target.createTarget", { url: "about:blank" });
    const target = await waitForTarget(debugPort, secondTarget.targetId);
    client2 = await connect(target.webSocketDebuggerUrl);
    await prepareClient(client2);
    await navigate(client2, `${serverURL}/?smoke=second-client`);
    const secondInitial = await waitForPageState(client2, { gox: "recovered", go: "go-rebuild", index: "index-rebuild" });
    assert(secondInitial.instance === activeInstance, `second client process instance = ${secondInitial.instance}, want ${activeInstance}`);
    await waitForEventSourceOpen(client2, 1);
    await wait(250);
    assert((await pageState(client2)).pageLoads === 1, "second client entered an initial reload loop");

    const firstMultiBaseline = await pageState(client1);
    const secondMultiBaseline = await pageState(client2);
    activationGapProbe = await openReloadProbe(serverURL, activeInstance, activeGeneration + 1);
    await activationGapProbe.waitConnected();
    await writeGoMessage("two-clients");
    await waitForBuild(nextBuild, "succeeded");
    const nextGeneration = ++activeGeneration;
    await Promise.all([
        assertSuccessfulReload(client1, firstMultiBaseline, { gox: "recovered", go: "two-clients", index: "index-rebuild" }, nextGeneration),
        assertSuccessfulReload(client2, secondMultiBaseline, { gox: "recovered", go: "two-clients", index: "index-rebuild" }, nextGeneration),
    ]);
    recordSuccessfulReload();
    nextBuild++;
    await wait(100);
    activationGapProbe.assertHealthy();
    assert(activationGapProbe.events.length === 0,
        `activation-gap subscriber received its declared generation: ${JSON.stringify(activationGapProbe.events)}`);
    console.log("two-client reload: ok");

    const firstGapFollowUpBaseline = await pageState(client1);
    const secondGapFollowUpBaseline = await pageState(client2);
    await writeGoMessage("activation-gap-follow-up");
    await waitForBuild(nextBuild, "succeeded");
    const gapFollowUpGeneration = ++activeGeneration;
    await Promise.all([
        assertSuccessfulReload(client1, firstGapFollowUpBaseline, { gox: "recovered", go: "activation-gap-follow-up", index: "index-rebuild" }, gapFollowUpGeneration),
        assertSuccessfulReload(client2, secondGapFollowUpBaseline, { gox: "recovered", go: "activation-gap-follow-up", index: "index-rebuild" }, gapFollowUpGeneration),
    ]);
    recordSuccessfulReload();
    nextBuild++;
    await activationGapProbe.waitForEvents(1);
    assert(JSON.stringify(activationGapProbe.events) === JSON.stringify([gapFollowUpGeneration]),
        `activation-gap subscriber events = ${JSON.stringify(activationGapProbe.events)}, want [${gapFollowUpGeneration}]`);
    await activationGapProbe.close();
    activationGapProbe = null;
    console.log("same-generation activation gap: ok");

    const currentReconnect = await pageState(client1);
    await reconnectReloadClient(client1, activeInstance, activeGeneration);
    await waitForEventSourceOpen(client1, currentReconnect.eventSourceOpens + 1);
    await wait(250);
    const afterCurrentReconnect = await pageState(client1);
    assert(afterCurrentReconnect.pageLoads === currentReconnect.pageLoads && afterCurrentReconnect.reloadEvents === currentReconnect.reloadEvents,
        `current-generation reconnect reloaded: ${JSON.stringify(afterCurrentReconnect)}`);
    console.log("same-generation reconnect: ok");

    await reconnectReloadClient(client1, activeInstance, activeGeneration - 1);
    await waitForEventSourceOpen(client1, afterCurrentReconnect.eventSourceOpens + 1);
    const caughtUp = await waitForPageState(client1, {
        gox: "recovered",
        go: "activation-gap-follow-up",
        index: "index-rebuild",
        pageLoads: afterCurrentReconnect.pageLoads + 1,
        reloadEvents: afterCurrentReconnect.reloadEvents + 1,
        generation: activeGeneration,
    });
    counters.catchUpReloads++;
    assert(caughtUp.generation === activeGeneration, `catch-up loaded generation ${caughtUp.generation}`);
    console.log("stale-generation catch-up reload: ok");

    const previousProcessBaseline = caughtUp;
    const previousInstance = activeInstance === "f".repeat(32) ? "e".repeat(32) : "f".repeat(32);
    await reconnectReloadClient(client1, previousInstance, activeGeneration + 1000);
    await waitForEventSourceOpen(client1, previousProcessBaseline.eventSourceOpens + 1);
    const previousProcessCaughtUp = await waitForPageState(client1, {
        gox: "recovered",
        go: "activation-gap-follow-up",
        index: "index-rebuild",
        pageLoads: previousProcessBaseline.pageLoads + 1,
        reloadEvents: previousProcessBaseline.reloadEvents + 1,
        generation: activeGeneration,
        instance: activeInstance,
    });
    counters.catchUpReloads++;
    counters.previousProcessCatchUpReloads++;
    await wait(250);
    const stablePreviousProcessCatchUp = await pageState(client1);
    assert(stablePreviousProcessCatchUp.pageLoads === previousProcessCaughtUp.pageLoads
        && stablePreviousProcessCatchUp.reloadEvents === previousProcessCaughtUp.reloadEvents,
        `previous-process catch-up entered a reload loop: ${JSON.stringify(stablePreviousProcessCatchUp)}`);
    console.log("previous-process catch-up reload: ok");

    const client1Final = await pageState(client1);
    const client2Final = await pageState(client2);
    assert(client1Final.buildErrorPresentations === 0 && client2Final.buildErrorPresentations === 0,
        `final pages retained build-error presentations: ${JSON.stringify({ client1Final, client2Final })}`);
    assertAuthoredMarker(client1Final, "final page 1");
    assertAuthoredMarker(client2Final, "final page 2");
    assert(client1.runtimeErrors.length === 0 && client2.runtimeErrors.length === 0,
        `browser runtime errors = ${JSON.stringify({ page1: client1.runtimeErrors, page2: client2.runtimeErrors })}`);

    await closeEventSources(client2);
    await waitForNoEventStreams(client2);
    await client1.call("Target.closeTarget", { targetId: secondTarget.targetId });
    client2.close();
    client2 = null;

    const bodyRootTransitionBaseline = client1Final;
    await writeBodyRootIndex("body-root-index");
    await waitForBuild(nextBuild, "succeeded");
    const bodyRootGeneration = ++activeGeneration;
    const bodyRootInitial = await waitForBodyRootState(client1, {
        bodyRootVersion: "body-root-initial",
        bodyRootAppVisible: true,
        pageLoads: bodyRootTransitionBaseline.pageLoads + 1,
        reloadEvents: bodyRootTransitionBaseline.reloadEvents + 1,
        generation: bodyRootGeneration,
    });
    recordSuccessfulReload();
    nextBuild++;
    assert(bodyRootInitial.bodyRoot, `body-root fixture did not mount into body: ${JSON.stringify(bodyRootInitial)}`);
    assert(bodyRootInitial.eventSourceCount === 1 && bodyRootInitial.eventSourceOpens === bodyRootTransitionBaseline.eventSourceOpens + 1,
        `body-root reload client state = ${JSON.stringify(bodyRootInitial)}`);
    assert(bodyRootInitial.buildErrorPresentations === 0,
        `body-root fixture started with a build-error presentation: ${JSON.stringify(bodyRootInitial)}`);
    const bodyRootInteractive = await exerciseBodyRootInteraction(client1, bodyRootInitial);

    const bodyRootFailureBuild = nextBuild;
    await writeBrokenBodyRootGo();
    await waitForBuild(bodyRootFailureBuild, "failed");
    counters.failedPackageAttempts++;
    nextBuild++;
    const bodyRootFailure = await waitForBodyRootBuildError(client1, bodyRootFailureBuild, "go WASM build failed");
    assert(bodyRootFailure.pageLoads === bodyRootInteractive.pageLoads
        && bodyRootFailure.reloadEvents === bodyRootInteractive.reloadEvents
        && bodyRootFailure.generation === bodyRootGeneration,
        `body-root failure changed the active page: ${JSON.stringify({ bodyRootInteractive, bodyRootFailure })}`);
    assert(bodyRootFailure.buildErrorEvents === bodyRootInteractive.buildErrorEvents + 1,
        `body-root failure events = ${bodyRootFailure.buildErrorEvents}, want ${bodyRootInteractive.buildErrorEvents + 1}`);
    assert(bodyRootFailure.eventSourceCount === bodyRootInteractive.eventSourceCount
        && bodyRootFailure.eventSourceOpens === bodyRootInteractive.eventSourceOpens,
        `body-root failure reconnected EventSource: ${JSON.stringify({ bodyRootInteractive, bodyRootFailure })}`);

    await client1.evaluate(`document.querySelector("#body-root-replace-control").click()`);
    const bodyRootReplacement = await waitForBodyRootState(client1, {
        bodyRootReplacementVisible: true,
        bodyRootCleanupCount: 1,
        pageLoads: bodyRootFailure.pageLoads,
        reloadEvents: bodyRootFailure.reloadEvents,
        buildErrorEvents: bodyRootFailure.buildErrorEvents,
        eventSourceOpens: bodyRootFailure.eventSourceOpens,
        eventSourceCount: bodyRootFailure.eventSourceCount,
        generation: bodyRootGeneration,
    });
    assert(bodyRootReplacement.devPanelIdentity === bodyRootFailure.devPanelIdentity,
        `body-root build-error presentation disappeared after repeated Mount: ${JSON.stringify({ bodyRootFailure, bodyRootReplacement })}`);
    assert(bodyRootFailure.devPanelParent === "documentElement" && !bodyRootFailure.devPanelInsideBody,
        `body-root failure panel was not outside body: ${JSON.stringify(bodyRootFailure)}`);
    assert(bodyRootReplacement.devPanelParent === "documentElement" && !bodyRootReplacement.devPanelInsideBody,
        `body-root replacement panel entered body: ${JSON.stringify(bodyRootReplacement)}`);
    assert(bodyRootReplacement.buildErrorBuild === bodyRootFailureBuild
        && bodyRootReplacement.buildErrorMessage === bodyRootFailure.buildErrorMessage,
        `body-root replacement changed the retained failure: ${JSON.stringify({ bodyRootFailure, bodyRootReplacement })}`);
    const bodyRootReplacementInteractive = await exerciseBodyRootReplacementInteraction(client1, bodyRootReplacement);

    await writeBodyRootStatus("body-root-recovered");
    await waitForBuild(nextBuild, "succeeded");
    const bodyRootRecoveryGeneration = ++activeGeneration;
    const bodyRootRecovered = await waitForBodyRootState(client1, {
        bodyRootVersion: "body-root-recovered",
        bodyRootAppVisible: true,
        pageLoads: bodyRootReplacementInteractive.pageLoads + 1,
        reloadEvents: bodyRootReplacementInteractive.reloadEvents + 1,
        buildErrorEvents: bodyRootReplacementInteractive.buildErrorEvents,
        eventSourceOpens: bodyRootReplacementInteractive.eventSourceOpens + 1,
        eventSourceCount: 1,
        generation: bodyRootRecoveryGeneration,
        buildErrorPresentations: 0,
    });
    recordSuccessfulReload();
    nextBuild++;
    assert(bodyRootRecovered.panelCountBeforeUnload === 0,
        `body-root panel remained when recovery reload began: ${JSON.stringify(bodyRootRecovered)}`);
    assert(client1.runtimeErrors.length === 0,
        `body-root scenario caused browser runtime errors: ${JSON.stringify(client1.runtimeErrors)}`);
    bodyRootEvidence = {
        failureBuild: bodyRootFailureBuild,
        failureGeneration: bodyRootGeneration,
        recoveryGeneration: bodyRootRecoveryGeneration,
        buildErrorEvents: bodyRootRecovered.buildErrorEvents,
        eventSourceOpens: bodyRootRecovered.eventSourceOpens,
        pageLoads: bodyRootRecovered.pageLoads,
        reloadEvents: bodyRootRecovered.reloadEvents,
        initialInteractionCount: bodyRootInteractive.bodyRootInteractionCount,
        replacementInteractionCount: bodyRootReplacementInteractive.bodyRootReplacementInteractionCount,
        cleanupCount: bodyRootReplacementInteractive.bodyRootCleanupCount,
        runtimeErrors: 0,
    };
    console.log("body-root build-error persistence and recovery: ok");

    const generationRoots = [...await listGenerationRoots()].filter((entry) => !generationRootsBefore.has(entry));
    assert(generationRoots.length === 1, `development generation roots = ${JSON.stringify(generationRoots)}, want one`);
    const shutdownBaseline = await pageState(client1);
    await closeEventSourcesOnError(client1);
    dev.kill("SIGINT");
    const devExit = await waitForExit(dev, 15_000);
    assert(devExit.code === 0, `goxc dev exited with ${JSON.stringify(devExit)}\n${devOutput}`);
    dev = null;
    await waitForHTTPShutdown(serverURL);
    await waitForEventSourceError(client1, shutdownBaseline.eventSourceErrors + 1);
    await waitForNoEventStreams(client1);
    for (const generationRoot of generationRoots) {
        assert(!existsSync(join(tmpdir(), generationRoot)), `generation root remained after shutdown: ${generationRoot}`);
    }

    console.log(`successful package attempts: ${counters.successfulPackageAttempts}`);
    console.log(`failed package attempts: ${counters.failedPackageAttempts}`);
    console.log(`completed generations: ${counters.completedGenerations}`);
    console.log(`generation activations: ${counters.generationActivations}`);
    console.log(`reload events published: ${counters.reloadEventsPublished}`);
    console.log(`reload events received by client 1: ${client1Final.reloadEvents}`);
    console.log(`reload events received by client 2: ${client2Final.reloadEvents}`);
    console.log(`catch-up reloads: ${counters.catchUpReloads}`);
    console.log(`previous-process catch-up reloads: ${counters.previousProcessCatchUpReloads}`);
    console.log(`embedded payload reloads: ${counters.embedReloads}`);
    console.log(`embedded payload failed attempts: ${counters.embedFailedAttempts}`);
    console.log("connected subscribers at shutdown: 0");
    console.log(`publication probe batches: ${counters.publicationProbeBatches}`);
    console.log(`complete responses during rebuild: ${counters.completeResponsesDuringBuild}`);
    console.log(`old-generation index responses: ${counters.oldGenerationIndexResponses}`);
    console.log(`new-generation index responses: ${counters.newGenerationIndexResponses}`);
    console.log(`404 responses during rebuild: ${counters.responses404DuringBuild}`);
    console.log(`partial responses during rebuild: ${counters.partialResponsesDuringBuild}`);
    console.log(`build error evidence: ${JSON.stringify(buildErrorEvidence)}`);
    console.log(`body-root evidence: ${JSON.stringify(bodyRootEvidence)}`);
    console.log("goxc dev reload browser smoke: ok");
} catch (error) {
    throw new Error(`${error.message}\n\nDevelopment output:\n${devOutput.slice(-12000)}\n\nChrome stderr:\n${browserError.slice(-6000)}`);
} finally {
    await activationGapProbe?.close();
    client1?.close();
    client2?.close();
    if (dev) {
        await stopProcess(dev, "SIGINT", 5000);
    }
    if (browser) {
        await stopProcess(browser, "SIGTERM", 3000);
    }
    await rm(root, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

function recordSuccessfulReload() {
    counters.successfulPackageAttempts++;
    counters.completedGenerations++;
    counters.generationActivations++;
    counters.reloadEventsPublished++;
}

async function writeApplication(gox, go, index) {
    await mkdir(join(appDir, "assets"), { recursive: true });
    await writeFile(join(appDir, "go.mod"), `module example.com/goframe-dev-reload\n\ngo 1.22\n\nrequire github.com/graybuton/goframe v0.0.0\n\nreplace github.com/graybuton/goframe => ${repositoryRoot}\n`);
    await writeFile(join(appDir, "goframe.json"), `{"name":"dev-reload-smoke","entry":".","compiler":"go","assets":"assets"}\n`);
    await writeFile(join(appDir, "main.go"), `//go:build js && wasm\n\npackage main\n\nimport (\n    "syscall/js"\n\n    gf "github.com/graybuton/goframe/pkg/goframe"\n)\n\nfunc main() {\n    done := make(chan struct{})\n    app := App\n    body := js.Global().Get("document").Get("body")\n    if !body.IsUndefined() && !body.IsNull() && body.Get("id").String() == "root" {\n        app = BodyRootApp\n    }\n    gf.Mount("root", app)\n    <-done\n}\n`);
    await writeGoMessage(go);
    await writeFile(join(appDir, "embedded.go"), `package main\n\nimport _ "embed"\n\n//go:embed embedded-message.txt\nvar embeddedMessage string\n`);
    await writeEmbeddedMessage("embed-alpha");
    await writeGOX(gox);
    await writeBodyRootGOX();
    await writeBodyRootStatus("body-root-initial");
    await writeIndex(index);
    await writeFile(join(appDir, "assets", "marker.txt"), "complete asset\n");
}

async function writeGOX(value) {
    await writeFile(join(appDir, "app.gox"), `package main\n\nimport gf "github.com/graybuton/goframe/pkg/goframe"\n\nfunc App() gf.Node {\n    interactionCount, setInteractionCount := gf.UseState(0)\n    return <main id="app-version"><span id="gox-version">${value}</span><span id="go-version">{message()}</span><span id="embedded-value">{embeddedMessage}</span><section id="authored-build-error-marker" data-goframe-dev-build-error=""><strong data-goframe-dev-build-error-heading="">authored heading</strong><pre data-goframe-dev-build-error-message="">authored message</pre><button id="interaction-control" type="button" onClick={func() { setInteractionCount(interactionCount + 1) }}>Interaction <span id="interaction-count">{interactionCount}</span></button></section></main>\n}\n`);
}

async function writeBrokenGOX() {
    await writeFile(join(appDir, "app.gox"), `package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func App() gf.Node {
    return <main data-broken={}><img data-goframe-diagnostic-injected src="missing" onerror="window.__goframeDevDiagnosticExecuted=true" /></main>
}
`);
}

async function writeReplacementBrokenGOX() {
    await writeFile(join(appDir, "app.gox"), `package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func App() gf.Node {
    return <main data-replacement={}></main>
}
`);
}

async function writeBodyRootGOX() {
    await writeFile(join(appDir, "body_root.gox"), `package main

import gf "github.com/graybuton/goframe/pkg/goframe"

var bodyRootCleanupCount int

func BodyRootApp() gf.Node {
    interactionCount, setInteractionCount := gf.UseState(0)
    gf.UseUnmount(func() { bodyRootCleanupCount++ })
    return <main id="body-root-app"><span id="body-root-version">{bodyRootBuildStatus()}</span><button id="body-root-interaction" type="button" onClick={func() { setInteractionCount(interactionCount + 1) }}>Initial interaction <span id="body-root-interaction-count">{interactionCount}</span></button><button id="body-root-replace-control" type="button" onClick={func() { gf.Mount("root", BodyRootReplacementApp) }}>Replace body application</button></main>
}

func BodyRootReplacementApp() gf.Node {
    interactionCount, setInteractionCount := gf.UseState(0)
    return <main id="body-root-replacement"><span id="body-root-cleanup-count">{bodyRootCleanupCount}</span><button id="body-root-replacement-interaction" type="button" onClick={func() { setInteractionCount(interactionCount + 1) }}>Replacement interaction <span id="body-root-replacement-interaction-count">{interactionCount}</span></button></main>
}
`);
}

async function writeBrokenBodyRootGo() {
    await writeFile(join(appDir, "body_root_status.go"), `package main\n\nfunc bodyRootBuildStatus() string { return bodyRootBroken }\n`);
}

async function writeBodyRootStatus(value) {
    await writeFile(join(appDir, "body_root_status.go"), `package main\n\nfunc bodyRootBuildStatus() string { return ${JSON.stringify(value)} }\n`);
}

async function writeGoMessage(value) {
    await writeFile(join(appDir, "message.go"), `package main\n\nfunc message() string { return ${JSON.stringify(value)} }\n`);
}

async function writeEmbeddedMessage(value) {
    await writeFile(join(appDir, "embedded-message.txt"), value);
}

async function writeIndex(value) {
    await writeFile(join(appDir, "assets", "index.html"), `<!doctype html>
<html><body>
<div id="index-version">${value}</div>
<div id="page-load-count"></div>
<div id="root">Loading...</div>
<script>
var count = Number(sessionStorage.getItem("goframe-dev-page-loads") || "0") + 1;
sessionStorage.setItem("goframe-dev-page-loads", String(count));
document.querySelector("#page-load-count").textContent = String(count);
</script>
<script src="wasm_exec.js"></script>
<script>var go = new Go(); WebAssembly.instantiateStreaming(fetch("bundle.wasm"), go.importObject).then(function (result) { go.run(result.instance); });</script>
</body></html>
`);
}

async function writeBodyRootIndex(value) {
    await writeFile(join(appDir, "assets", "index.html"), `<!doctype html>
<html><body id="root">
<div id="index-version">${value}</div>
<div id="page-load-count"></div>
<script>
var count = Number(sessionStorage.getItem("goframe-dev-page-loads") || "0") + 1;
sessionStorage.setItem("goframe-dev-page-loads", String(count));
document.querySelector("#page-load-count").textContent = String(count);
</script>
<script src="wasm_exec.js"></script>
<script>window.addEventListener("load", function () { var go = new Go(); WebAssembly.instantiateStreaming(fetch("bundle.wasm"), go.importObject).then(function (result) { go.run(result.instance); }); }, { once: true });</script>
</body></html>
`);
}

async function prepareClient(client) {
    client.runtimeErrors = [];
    client.on("Runtime.exceptionThrown", ({ exceptionDetails }) => {
        client.runtimeErrors.push(exceptionDetails?.text || "unknown browser exception");
    });
    await client.call("Runtime.enable");
    await client.call("Page.enable");
    await client.call("Network.enable");
    client.eventStreams = new Set();
    client.on("Network.requestWillBeSent", ({ requestId, request }) => {
        if (request.url.includes("/_goframe/dev/events")) client.eventStreams.add(requestId);
    });
    const finish = ({ requestId }) => client.eventStreams.delete(requestId);
    client.on("Network.loadingFinished", finish);
    client.on("Network.loadingFailed", finish);
    await client.call("Page.addScriptToEvaluateOnNewDocument", { source: reloadEvidenceScript() });
}

function reloadEvidenceScript() {
    return `(() => {
        const NativeEventSource = window.EventSource;
        if (typeof NativeEventSource !== "function") return;
        window.__goframeDevEventSources = [];
        window.__goframeDevDiagnosticExecuted = false;
        window.addEventListener("beforeunload", () => {
            const authoredMarker = document.querySelector("#authored-build-error-marker");
            const devPanels = [...document.querySelectorAll("[data-goframe-dev-build-error]")]
                .filter((node) => node !== authoredMarker);
            sessionStorage.setItem(
                "goframe-dev-panel-count-before-unload",
                String(devPanels.length),
            );
            sessionStorage.setItem("goframe-dev-authored-marker-count-before-unload", String(authoredMarker ? 1 : 0));
            sessionStorage.setItem(
                "goframe-dev-authored-marker-identity-before-unload",
                String(window.__goframeDevNodeIdentity?.(authoredMarker) || 0),
            );
            sessionStorage.setItem(
                "goframe-dev-authored-heading-before-unload",
                authoredMarker?.querySelector("[data-goframe-dev-build-error-heading]")?.textContent || "",
            );
            sessionStorage.setItem(
                "goframe-dev-authored-message-before-unload",
                authoredMarker?.querySelector("[data-goframe-dev-build-error-message]")?.textContent || "",
            );
        });
        const nodeIDs = new WeakMap();
        window.__goframeDevNodeIdentity = (node) => {
            if (!node) return 0;
            let identity = nodeIDs.get(node);
            if (!identity) {
                identity = Number(sessionStorage.getItem("goframe-dev-next-node-identity") || "0") + 1;
                sessionStorage.setItem("goframe-dev-next-node-identity", String(identity));
                nodeIDs.set(node, identity);
            }
            return identity;
        };
        const increment = (key) => {
            const next = Number(sessionStorage.getItem(key) || "0") + 1;
            sessionStorage.setItem(key, String(next));
        };
        function WrappedEventSource(url, options) {
            const source = options === undefined ? new NativeEventSource(url) : new NativeEventSource(url, options);
            window.__goframeDevEventSources.push(source);
            source.addEventListener("open", () => increment("goframe-dev-event-source-opens"));
            source.addEventListener("error", () => increment("goframe-dev-event-source-errors"));
            source.addEventListener("build-error", (event) => {
                try {
                    const failure = JSON.parse(event.data);
                    if (failure && Number.isInteger(failure.build) && failure.build > 0 && typeof failure.message === "string") {
                        increment("goframe-dev-build-error-events");
                    }
                } catch (_) {}
            });
            source.addEventListener("reload", () => increment("goframe-dev-reload-events"));
            return source;
        }
        WrappedEventSource.prototype = NativeEventSource.prototype;
        Object.setPrototypeOf(WrappedEventSource, NativeEventSource);
        window.EventSource = WrappedEventSource;
    })()`;
}

async function pageState(client) {
    return await client.evaluate(`(() => {
        const script = document.querySelector("script[data-goframe-dev-reload]");
        const eventSources = window.__goframeDevEventSources || [];
        const activeEventSource = eventSources[eventSources.length - 1];
        let activeEventSourceURL = null;
        try {
            activeEventSourceURL = activeEventSource ? new URL(activeEventSource.url, location.href) : null;
        } catch (_) {}
        const applicationRoot = document.querySelector("#app-version");
        const authoredMarker = document.querySelector("#authored-build-error-marker");
        const buildErrorMarkers = [...document.querySelectorAll("[data-goframe-dev-build-error]")];
        const devPanels = buildErrorMarkers.filter((node) => node !== authoredMarker);
        const panel = devPanels[0] || null;
        return {
            href: location.href,
            readyState: document.readyState,
            gox: document.querySelector("#gox-version")?.textContent || "",
            go: document.querySelector("#go-version")?.textContent || "",
            embedded: document.querySelector("#embedded-value")?.textContent || "",
            index: document.querySelector("#index-version")?.textContent || "",
            pageLoads: Number(sessionStorage.getItem("goframe-dev-page-loads") || "0"),
            reloadEvents: Number(sessionStorage.getItem("goframe-dev-reload-events") || "0"),
            buildErrorEvents: Number(sessionStorage.getItem("goframe-dev-build-error-events") || "0"),
            eventSourceOpens: Number(sessionStorage.getItem("goframe-dev-event-source-opens") || "0"),
            eventSourceErrors: Number(sessionStorage.getItem("goframe-dev-event-source-errors") || "0"),
            panelCountBeforeUnload: Number(sessionStorage.getItem("goframe-dev-panel-count-before-unload") || "-1"),
            authoredMarkerCountBeforeUnload: Number(sessionStorage.getItem("goframe-dev-authored-marker-count-before-unload") || "-1"),
            authoredMarkerIdentityBeforeUnload: Number(sessionStorage.getItem("goframe-dev-authored-marker-identity-before-unload") || "0"),
            authoredHeadingBeforeUnload: sessionStorage.getItem("goframe-dev-authored-heading-before-unload") || "",
            authoredMessageBeforeUnload: sessionStorage.getItem("goframe-dev-authored-message-before-unload") || "",
            generation: Number(script?.getAttribute("data-goframe-generation") || activeEventSourceURL?.searchParams.get("generation") || "0"),
            instance: script?.getAttribute("data-goframe-instance") || activeEventSourceURL?.searchParams.get("instance") || "",
            reloadTags: document.querySelectorAll("script[data-goframe-dev-reload]").length,
            eventSourceCount: eventSources.length,
            appRootIdentity: window.__goframeDevNodeIdentity?.(applicationRoot) || 0,
            appRootCount: document.querySelectorAll("#app-version").length,
            interactionCount: Number(document.querySelector("#interaction-count")?.textContent || "0"),
            authoredMarkerPresent: Boolean(authoredMarker),
            authoredMarkerIdentity: window.__goframeDevNodeIdentity?.(authoredMarker) || 0,
            authoredHeading: authoredMarker?.querySelector("[data-goframe-dev-build-error-heading]")?.textContent || "",
            authoredMessage: authoredMarker?.querySelector("[data-goframe-dev-build-error-message]")?.textContent || "",
            authoredMarkerInsideAppRoot: Boolean(authoredMarker?.closest("#app-version")),
            buildErrorMarkers: buildErrorMarkers.length,
            buildErrorPresentations: devPanels.length,
            devPanelIdentity: window.__goframeDevNodeIdentity?.(panel) || 0,
            devPanelParent: panel?.parentNode === document.documentElement
                ? "documentElement"
                : panel?.parentNode === document.body
                    ? "body"
                    : "",
            devPanelInsideBody: Boolean(panel && document.body?.contains(panel)),
            buildErrorBuild: Number(panel?.getAttribute("data-goframe-dev-build") || "0"),
            buildErrorHeading: panel?.querySelector("[data-goframe-dev-build-error-heading]")?.textContent || "",
            buildErrorMessage: panel?.querySelector("[data-goframe-dev-build-error-message]")?.textContent || "",
            buildErrorRole: panel?.getAttribute("role") || "",
            buildErrorLive: panel?.getAttribute("aria-live") || "",
            buildErrorInsideAppRoot: Boolean(panel?.closest("#app-version")),
            injectedDiagnosticElements: document.querySelectorAll("[data-goframe-diagnostic-injected]").length,
            diagnosticExecuted: Boolean(window.__goframeDevDiagnosticExecuted),
            bodyRoot: document.body?.id === "root",
            bodyRootAppVisible: Boolean(document.querySelector("#body-root-app")),
            bodyRootVersion: document.querySelector("#body-root-version")?.textContent || "",
            bodyRootInteractionCount: Number(document.querySelector("#body-root-interaction-count")?.textContent || "0"),
            bodyRootReplacementVisible: Boolean(document.querySelector("#body-root-replacement")),
            bodyRootReplacementInteractionCount: Number(document.querySelector("#body-root-replacement-interaction-count")?.textContent || "0"),
            bodyRootCleanupCount: Number(document.querySelector("#body-root-cleanup-count")?.textContent || "0"),
        };
    })()`);
}

async function waitForBodyRootState(client, expected) {
    let last = null;
    for (let attempt = 0; attempt < 300; attempt++) {
        last = await pageState(client);
        const matches = Object.entries(expected).every(([key, value]) => last[key] === value);
        if (matches && last.readyState === "complete" && last.bodyRoot) return last;
        await wait(50);
    }
    throw new Error(`HARNESS FAILURE: body-root state did not match ${JSON.stringify(expected)}; last=${JSON.stringify(last)}`);
}

async function waitForPageState(client, expected) {
    let last = null;
    for (let attempt = 0; attempt < 300; attempt++) {
        last = await pageState(client);
        const matches = Object.entries(expected).every(([key, value]) => last[key] === value);
        if (matches && last.readyState === "complete" && last.reloadTags === 1) return last;
        await wait(50);
    }
    throw new Error(`HARNESS FAILURE: page state did not match ${JSON.stringify(expected)}; last=${JSON.stringify(last)}`);
}

async function assertSuccessfulReload(client, baseline, expected, generation) {
    const state = await waitForPageState(client, {
        ...expected,
        pageLoads: baseline.pageLoads + 1,
        reloadEvents: baseline.reloadEvents + 1,
        generation,
    });
    assert(state.reloadTags === 1, `reload tag count = ${state.reloadTags}, want 1`);
    return state;
}

async function waitForBuildError(client, build, messageFragment) {
    let last = null;
    for (let attempt = 0; attempt < 200; attempt++) {
        last = await pageState(client);
        if (last.buildErrorPresentations === 1
            && last.buildErrorBuild === build
            && last.buildErrorMessage.includes(messageFragment)) {
            assert(last.buildErrorHeading === `Build ${build} failed`,
                `build error heading = ${JSON.stringify(last.buildErrorHeading)}, want build ${build}`);
            assert(last.buildErrorRole === "alert" && last.buildErrorLive === "assertive",
                `build error accessibility = ${JSON.stringify({ role: last.buildErrorRole, live: last.buildErrorLive })}`);
            assert(!last.buildErrorInsideAppRoot, "build error presentation entered the application root");
            assert(last.appRootCount === 1, `application root count = ${last.appRootCount}, want 1`);
            assertAuthoredMarker(last, `build ${build}`);
            assert(last.buildErrorMarkers === 2,
                `build ${build} marker count = ${last.buildErrorMarkers}, want authored marker plus one development panel`);
            return last;
        }
        await wait(25);
    }
    throw new Error(`HARNESS FAILURE: build ${build} presentation did not contain ${JSON.stringify(messageFragment)}; last=${JSON.stringify(last)}`);
}

async function waitForBodyRootBuildError(client, build, messageFragment) {
    let last = null;
    for (let attempt = 0; attempt < 200; attempt++) {
        last = await pageState(client);
        if (last.bodyRoot
            && last.buildErrorPresentations === 1
            && last.buildErrorBuild === build
            && last.buildErrorMessage.includes(messageFragment)) {
            assert(last.buildErrorHeading === `Build ${build} failed`,
                `body-root build error heading = ${JSON.stringify(last.buildErrorHeading)}, want build ${build}`);
            assert(last.buildErrorRole === "alert" && last.buildErrorLive === "assertive",
                `body-root build error accessibility = ${JSON.stringify({ role: last.buildErrorRole, live: last.buildErrorLive })}`);
            return last;
        }
        await wait(25);
    }
    throw new Error(`HARNESS FAILURE: body-root build ${build} presentation did not contain ${JSON.stringify(messageFragment)}; last=${JSON.stringify(last)}`);
}

function assertFailedBuildPreserved(baseline, state, generation, label) {
    assert(state.pageLoads === baseline.pageLoads && state.reloadEvents === baseline.reloadEvents,
        `${label} reloaded the browser: ${JSON.stringify({ baseline, state })}`);
    assert(state.generation === generation, `${label} changed generation to ${state.generation}, want ${generation}`);
    assert(state.appRootIdentity === baseline.appRootIdentity,
        `${label} replaced the application root: ${JSON.stringify({ baseline, state })}`);
    assert(state.gox === baseline.gox && state.go === baseline.go && state.index === baseline.index,
        `${label} changed active application content: ${JSON.stringify({ baseline, state })}`);
    assertAuthoredMarker(state, label);
    assert(state.authoredMarkerIdentity === baseline.authoredMarkerIdentity,
        `${label} replaced the authored marker: ${JSON.stringify({ baseline, state })}`);
}

function assertAuthoredMarker(state, label) {
    assert(state.authoredMarkerPresent && state.authoredMarkerInsideAppRoot,
        `${label} removed the authored build-error marker: ${JSON.stringify(state)}`);
    assert(state.authoredHeading === "authored heading" && state.authoredMessage === "authored message",
        `${label} changed authored build-error content: ${JSON.stringify(state)}`);
}

async function exerciseInteraction(client, baseline) {
    await client.evaluate(`document.querySelector("#interaction-control").click()`);
    return await waitForPageState(client, {
        interactionCount: baseline.interactionCount + 1,
        pageLoads: baseline.pageLoads,
        reloadEvents: baseline.reloadEvents,
        generation: baseline.generation,
        appRootIdentity: baseline.appRootIdentity,
    });
}

async function exerciseBodyRootInteraction(client, baseline) {
    await client.evaluate(`document.querySelector("#body-root-interaction").click()`);
    return await waitForBodyRootState(client, {
        bodyRootInteractionCount: baseline.bodyRootInteractionCount + 1,
        pageLoads: baseline.pageLoads,
        reloadEvents: baseline.reloadEvents,
        generation: baseline.generation,
    });
}

async function exerciseBodyRootReplacementInteraction(client, baseline) {
    await client.evaluate(`document.querySelector("#body-root-replacement-interaction").click()`);
    return await waitForBodyRootState(client, {
        bodyRootReplacementVisible: true,
        bodyRootReplacementInteractionCount: baseline.bodyRootReplacementInteractionCount + 1,
        pageLoads: baseline.pageLoads,
        reloadEvents: baseline.reloadEvents,
        generation: baseline.generation,
    });
}

async function dispatchMalformedBuildError(client) {
    await client.evaluate(`(() => {
        const sources = window.__goframeDevEventSources || [];
        const source = sources[sources.length - 1];
        if (!source) throw new Error("no development EventSource to probe");
        source.dispatchEvent(new MessageEvent("build-error", { data: "{malformed" }));
    })()`);
}

async function removeDevBuildErrorPanel(client) {
    await client.evaluate(`(() => {
        const authoredMarker = document.querySelector("#authored-build-error-marker");
        for (const panel of document.querySelectorAll("[data-goframe-dev-build-error]")) {
            if (panel !== authoredMarker) panel.remove();
        }
    })()`);
}

async function reconnectReloadClient(client, instance, generation, removeBuildError = false) {
    await client.evaluate(`(() => {
        for (const source of window.__goframeDevEventSources || []) source.close();
        if (${JSON.stringify(removeBuildError)}) {
            const authoredMarker = document.querySelector("#authored-build-error-marker");
            for (const panel of document.querySelectorAll("[data-goframe-dev-build-error]")) {
                if (panel !== authoredMarker) panel.remove();
            }
        }
        const script = document.createElement("script");
        script.src = ${JSON.stringify("/_goframe/dev/reload.js")} + "?reconnect=" + Date.now();
        script.setAttribute("data-goframe-instance", ${JSON.stringify(String(instance))});
        script.setAttribute("data-goframe-generation", ${JSON.stringify(String(generation))});
        document.body.appendChild(script);
    })()`);
}

async function waitForEventSourceOpen(client, want) {
    for (let attempt = 0; attempt < 200; attempt++) {
        const state = await pageState(client);
        if (state.eventSourceOpens >= want) return;
        await wait(25);
    }
    throw new Error(`HARNESS FAILURE: EventSource open count did not reach ${want}`);
}

async function waitForEventSourceError(client, want) {
    for (let attempt = 0; attempt < 200; attempt++) {
        const state = await pageState(client);
        if (state.eventSourceErrors >= want) return;
        await wait(25);
    }
    throw new Error(`HARNESS FAILURE: EventSource error count did not reach ${want}`);
}

async function closeEventSources(client) {
    await client.evaluate(`(() => {
        for (const source of window.__goframeDevEventSources || []) source.close();
    })()`);
}

async function closeEventSourcesOnError(client) {
    await client.evaluate(`(() => {
        for (const source of window.__goframeDevEventSources || []) {
            source.addEventListener("error", () => source.close(), { once: true });
        }
    })()`);
}

async function probeCompletedGenerationUntilBuildCompletes(serverURL, wasmPath, generation, build) {
    const evidence = {
        batches: 0,
        completeResponses: 0,
        oldGenerationIndexResponses: 0,
        newGenerationIndexResponses: 0,
        responses404: 0,
        partialResponses: 0,
    };
    const deadline = Date.now() + 30_000;
    for (;;) {
        if (hasBuildResult(build, "failed")) {
            throw new Error(`APP FAILURE: dev build ${build} failed during the publication probe`);
        }
        if (dev && (dev.exitCode !== null || dev.signalCode !== null)) {
            throw new Error(`HARNESS FAILURE: goxc dev exited during publication probe for build ${build}`);
        }
        if (Date.now() >= deadline) {
            throw new Error(`HARNESS FAILURE: publication probe timed out waiting for build ${build}`);
        }

        const batch = await probeCompletedGenerationBatch(serverURL, wasmPath, generation, build, evidence.batches);
        evidence.batches++;
        evidence.completeResponses += batch.completeResponses;
        evidence.oldGenerationIndexResponses += batch.oldGenerationIndexResponses;
        evidence.newGenerationIndexResponses += batch.newGenerationIndexResponses;
        evidence.responses404 += batch.responses404;
        evidence.partialResponses += batch.partialResponses;

        if (hasBuildResult(build, "failed")) {
            throw new Error(`APP FAILURE: dev build ${build} failed during the publication probe`);
        }
        if (hasBuildResult(build, "succeeded")) return evidence;
        await wait(0);
    }
}

async function probeCompletedGenerationBatch(serverURL, wasmPath, generation, build, batch) {
    const signal = AbortSignal.timeout(5_000);
    const query = `publication=${build}-${batch}`;
    const [indexResponse, metadataResponse, wasmResponse] = await Promise.all([
        fetch(`${serverURL}/?${query}`, { cache: "no-store", signal }),
        fetch(`${serverURL}/goframe-package.json?${query}`, { cache: "no-store", signal }),
        fetch(`${serverURL}/${wasmPath}?${query}`, { cache: "no-store", signal }),
    ]);
    let responses404 = 0;
    for (const response of [indexResponse, metadataResponse, wasmResponse]) {
        if (response.status === 404) responses404++;
        assert(response.ok, `HTTP ${response.status} during publication probe for ${response.url}`);
    }

    const index = await indexResponse.text();
    const metadataText = await metadataResponse.text();
    const wasm = await wasmResponse.arrayBuffer();
    let metadata = null;
    try {
        metadata = JSON.parse(metadataText);
    } catch {}

    const markerCount = index.split("data-goframe-dev-reload").length - 1;
    const servedGeneration = Number(index.match(/data-goframe-generation="(\d+)"/)?.[1] || "0");
    const indexComplete = markerCount === 1
        && servedGeneration > 0
        && (servedGeneration === generation || servedGeneration === generation + 1);
    const metadataComplete = metadata?.version === 1 && metadata?.entrypoints?.wasm === wasmPath;
    const wasmComplete = wasm.byteLength > 0;
    const partialResponses = [indexComplete, metadataComplete, wasmComplete].filter((complete) => !complete).length;
    assert(partialResponses === 0,
        `partial package response during publication probe: ${JSON.stringify({ markerCount, servedGeneration, metadata, wasmBytes: wasm.byteLength })}`);

    return {
        completeResponses: 3,
        oldGenerationIndexResponses: servedGeneration === generation ? 1 : 0,
        newGenerationIndexResponses: servedGeneration === generation + 1 ? 1 : 0,
        responses404,
        partialResponses,
    };
}

async function openReloadProbe(serverURL, instance, generation) {
    const controller = new AbortController();
    const response = await fetch(`${serverURL}/_goframe/dev/events?instance=${encodeURIComponent(instance)}&generation=${encodeURIComponent(generation)}`, {
        signal: controller.signal,
    });
    assert(response.ok && response.body, `activation-gap probe HTTP ${response.status}`);
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    const state = {
        connected: false,
        events: [],
        failure: null,
    };
    let buffer = "";
    const pump = (async () => {
        for (;;) {
            const { done, value } = await reader.read();
            if (done) return;
            buffer += decoder.decode(value, { stream: true });
            for (;;) {
                const boundary = buffer.indexOf("\n\n");
                if (boundary < 0) break;
                const block = buffer.slice(0, boundary);
                buffer = buffer.slice(boundary + 2);
                if (block.startsWith(": connected")) state.connected = true;
                const event = block.split("\n").find((line) => line.startsWith("event: "))?.slice(7);
                const data = block.split("\n").find((line) => line.startsWith("data: "))?.slice(6);
                if (event === "reload" && data) state.events.push(Number(data));
            }
        }
    })().catch((error) => {
        if (!controller.signal.aborted) state.failure = error;
    });

    const waitFor = async (predicate, label) => {
        for (let attempt = 0; attempt < 200; attempt++) {
            if (state.failure) throw state.failure;
            if (predicate()) return;
            await wait(25);
        }
        throw new Error(`HARNESS FAILURE: timed out waiting for ${label}`);
    };
    return {
        events: state.events,
        assertHealthy() {
            if (state.failure) throw state.failure;
        },
        async waitConnected() {
            await waitFor(() => state.connected, "activation-gap SSE connection");
        },
        async waitForEvents(count) {
            await waitFor(() => state.events.length >= count, `${count} activation-gap SSE events`);
        },
        async close() {
            controller.abort();
            await reader.cancel().catch(() => {});
            await pump;
        },
    };
}

function captureLines(stream, source) {
    let buffer = "";
    stream.setEncoding("utf8");
    stream.on("data", (chunk) => {
        devOutput += chunk;
        buffer += chunk;
        while (buffer.includes("\n")) {
            const index = buffer.indexOf("\n");
            lines.push({ source, text: buffer.slice(0, index).replace(/\r$/, "") });
            buffer = buffer.slice(index + 1);
        }
    });
}

async function waitForLine(predicate, timeout = 20_000, start = 0) {
    const deadline = Date.now() + timeout;
    while (Date.now() < deadline) {
        const line = lines.slice(start).find(predicate);
        if (line) return line;
        if (dev?.exitCode !== null) throw new Error(`HARNESS FAILURE: goxc dev exited early with ${dev.exitCode}`);
        await wait(25);
    }
    throw new Error(`HARNESS FAILURE: timed out waiting for development output\n${devOutput}`);
}

async function waitForBuild(number, result, start = 0) {
    return await waitForLine((line) => line.text.includes(`dev build ${number} ${result}`), 30_000, start);
}

async function waitForBuildStarted(number, start = 0) {
    return await waitForLine((line) => line.text.startsWith(`dev build ${number} (`), 30_000, start);
}

function hasBuildResult(number, result) {
    return lines.some((line) => line.text.includes(`dev build ${number} ${result}`));
}

function countBuildStartsSince(start) {
    return lines.slice(start).filter((line) => /^dev build \d+ \((initial|rebuild)\):/.test(line.text)).length;
}

async function findCanonicalPackageDirectory(root) {
    for (let attempt = 0; attempt < 100; attempt++) {
        const found = await findDirectoryEnding(root, join("package", "standalone"));
        if (found) return found;
        await wait(50);
    }
    throw new Error(`HARNESS FAILURE: canonical package directory not found below ${root}`);
}

async function findDirectoryEnding(root, suffix) {
    let entries;
    try {
        entries = await readdir(root, { withFileTypes: true });
    } catch {
        return null;
    }
    for (const entry of entries) {
        if (!entry.isDirectory()) continue;
        const path = join(root, entry.name);
        if (path.endsWith(suffix)) return path;
        const nested = await findDirectoryEnding(path, suffix);
        if (nested) return nested;
    }
    return null;
}

async function listGenerationRoots() {
    const entries = await readdir(tmpdir(), { withFileTypes: true });
    return new Set(entries.filter((entry) => entry.isDirectory() && entry.name.startsWith("goxc-dev-generations-")).map((entry) => entry.name));
}

async function runProcess(command, args, options) {
    const child = spawn(command, args, { ...options, stdio: ["ignore", "pipe", "pipe"] });
    let output = "";
    child.stdout.on("data", (chunk) => { output += chunk; });
    child.stderr.on("data", (chunk) => { output += chunk; });
    const result = await waitForExit(child, 60_000);
    if (result.code !== 0) throw new Error(`${command} ${args.join(" ")} failed: ${output}`);
}

async function freePort() {
    return await new Promise((resolvePort, reject) => {
        const server = createServer();
        server.once("error", reject);
        server.listen(0, "127.0.0.1", () => {
            const address = server.address();
            server.close(() => resolvePort(address.port));
        });
    });
}

async function waitForPage(port) {
    for (let attempt = 0; attempt < 100; attempt++) {
        try {
            const targets = await fetchTargets(port);
            const page = targets.find((entry) => entry.type === "page" && entry.webSocketDebuggerUrl);
            if (page) return page;
        } catch {}
        await wait(50);
    }
    throw new Error("HARNESS FAILURE: Chrome DevTools did not become ready");
}

async function waitForTarget(port, targetID) {
    for (let attempt = 0; attempt < 100; attempt++) {
        const target = (await fetchTargets(port)).find((entry) => entry.id === targetID && entry.webSocketDebuggerUrl);
        if (target) return target;
        await wait(50);
    }
    throw new Error(`HARNESS FAILURE: Chrome target ${targetID} did not become ready`);
}

async function fetchTargets(port) {
    const response = await fetch(`http://127.0.0.1:${port}/json`);
    if (!response.ok) throw new Error(`CDP /json returned HTTP ${response.status}`);
    return await response.json();
}

async function navigate(client, url) {
    await client.call("Page.navigate", { url });
}

async function connect(url) {
    const socket = new WebSocket(url);
    await new Promise((resolveOpen, reject) => {
        socket.addEventListener("open", resolveOpen, { once: true });
        socket.addEventListener("error", reject, { once: true });
    });
    let nextID = 1;
    const pending = new Map();
    const listeners = new Map();
    socket.addEventListener("message", (event) => {
        const message = JSON.parse(event.data);
        if (message.id && pending.has(message.id)) {
            const request = pending.get(message.id);
            pending.delete(message.id);
            if (message.error) request.reject(new Error(message.error.message));
            else request.resolve(message.result);
            return;
        }
        for (const listener of listeners.get(message.method) || []) listener(message.params || {});
    });
    return {
        eventStreams: new Set(),
        call(method, params = {}) {
            return new Promise((resolveCall, reject) => {
                const id = nextID++;
                pending.set(id, { resolve: resolveCall, reject });
                socket.send(JSON.stringify({ id, method, params }));
            });
        },
        on(method, listener) {
            const current = listeners.get(method) || [];
            current.push(listener);
            listeners.set(method, current);
        },
        async evaluate(expression) {
            const result = await this.call("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true });
            if (result.exceptionDetails) throw new Error(`browser evaluation failed: ${JSON.stringify(result.exceptionDetails)}`);
            return result.result.value;
        },
        close() { socket.close(); },
    };
}

async function waitForNoEventStreams(client) {
    for (let attempt = 0; attempt < 400; attempt++) {
        if (client.eventStreams.size === 0) return;
        await wait(25);
    }
    throw new Error(`HARNESS FAILURE: ${client.eventStreams.size} EventSource requests remained after shutdown`);
}

async function waitForHTTPShutdown(serverURL) {
    for (let attempt = 0; attempt < 100; attempt++) {
        try {
            await fetch(`${serverURL}/`, { signal: AbortSignal.timeout(100) });
        } catch {
            return;
        }
        await wait(25);
    }
    throw new Error("HARNESS FAILURE: development HTTP listener remained open after shutdown");
}

async function waitForExit(child, timeout) {
    if (child.exitCode !== null || child.signalCode !== null) {
        return { code: child.exitCode, signal: child.signalCode };
    }
    return await new Promise((resolveExit, reject) => {
        const onExit = (code, signal) => {
            clearTimeout(timer);
            resolveExit({ code, signal });
        };
        const timer = setTimeout(() => {
            child.off("exit", onExit);
            reject(new Error("process exit timed out"));
        }, timeout);
        child.once("exit", onExit);
    });
}

async function stopProcess(child, signal, timeout) {
    if (child.exitCode !== null || child.signalCode !== null) return;
    child.kill(signal);
    try {
        await waitForExit(child, timeout);
        return;
    } catch {}

    child.kill("SIGKILL");
    await waitForExit(child, timeout).catch(() => {});
}

function assert(condition, message) {
    if (!condition) throw new Error(`APP FAILURE: ${message}`);
}

function wait(duration) {
    return new Promise((resolveWait) => setTimeout(resolveWait, duration));
}

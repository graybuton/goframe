import { spawn } from "node:child_process";
import { mkdtemp, readFile, rm, stat } from "node:fs/promises";
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
const workspaceAppDir = join(appDir, ".goframe", "work", "dev", "examples", "server-backed", "cmd", "app");
const chrome = process.env.CHROME ?? "google-chrome";
const debugPort = Number(process.env.GOFRAME_SERVER_BACKED_CHROME_DEBUG_PORT ?? await pickFreePort());
const backendPort = Number(process.env.GOFRAME_SERVER_BACKED_SMOKE_PORT ?? await pickFreePort());
const profile = await mkdtempCompat("goframe-server-backed-smoke-");
const appURL = `http://127.0.0.1:${backendPort}/?smoke=${Date.now()}`;
const expectedApp = new URL(appURL);
const initialName = "GoFrame";
const initialMessage = "Hello, GoFrame, from Go backend!";
const directName = "Lin";
const directMessage = "Hello, Lin, from Go backend!";
const updatedName = "Ada";
const updatedMessage = "Hello, Ada, from Go backend!";
const slowName = "slow";
const slowMessage = "Hello, slow, from Go backend!";
const slowDelayMS = 750;
const staleSettleMS = slowDelayMS + 300;
const failureName = "fail";
const failureStatus = "failed";
const failureMessage = "goframe: fetch returned HTTP 500";
const savedGreetingTarget = "/saved-greeting";
const savedGreetingHash = `#${savedGreetingTarget}`;
const savedInitialName = "GoFrame";
const savedSlowName = "slow";
const savedFailureName = "fail";
const savedRecoveryName = "Grace";
const savedFailureMessage = "controlled saved greeting failure";
const componentNames = [
    "App",
    "ServerBackedShell",
    "RouterView",
    "RouterRoute",
    "HomeRoute",
    "GreetingRoute",
    "SavedGreetingRoute",
    "NotFoundRoute",
];

let backend = null;
let browser = null;
let backendError = "";
let browserError = "";
let browserExit = null;

try {
    await runCommand("go", ["run", "./cmd/goxc", "package", "./examples/server-backed", "--compiler=go"], { cwd: rootDir });
    await rebuildDebugBundle();

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
    await assertBackendAPI(directName, directMessage);
    await assertBackendAPI(updatedName, updatedMessage);
    await assertBackendAPI(slowName, slowMessage);
    await assertBackendAPIFailure(failureName, 500);
    await assertSavedGreetingAPI(savedInitialName);

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
    await client.call("Page.addScriptToEvaluateOnNewDocument", {
        source: installServerBackedEvidenceExpression(),
    });
    await navigateToApp(client, appURL);
    await waitForAppPage(client, expectedApp);
    await initializeBrowserEvidence(client);

    assertState(await appState(client), {
        app: true,
        shell: true,
        form: true,
        routeContent: true,
        route: "home",
        hash: "",
        routeTarget: "/",
        loading: false,
        ready: false,
        failed: false,
        status: "",
        message: "",
        origin: expectedApp.origin,
        input: initialName,
        appSame: true,
        shellSame: true,
        routeContentSame: true,
    }, "server-backed initial route");
    assertEvidence(await evidenceState(client), {
        fetchesStarted: 0,
        aborts: 0,
        staleResultAppearances: 0,
        appIdentityChanges: 0,
        shellIdentityChanges: 0,
        routeContentIdentityChanges: 0,
    }, "initial route evidence");

    await startScenario(client, "direct-hash-navigation");
    await navigateGreetingHash(client, directName);
    await waitForLoading(client, directName, "direct Lin route loading");
    await waitForGreeting(client, directMessage, "direct Lin backend greeting");
    const directReport = await finishScenario(client, "direct-hash-navigation");
    assertGreetingRouteReport(directReport, "direct hash navigation", {
        outcome: "success",
        routeChanges: 1,
        samePattern: false,
    });
    assertState(await appState(client), {
        route: "greeting",
        hash: greetingHash(directName),
        routeTarget: greetingTarget(directName),
        key: `/api/greeting?name=${encodeURIComponent(directName)}`,
        status: "ready",
        message: directMessage,
        input: directName,
        appSame: true,
        shellSame: true,
        routeContentSame: true,
    }, "server-backed direct hash navigation");

    await prepareGreetingName(client, updatedName);
    await startScenario(client, "successful-navigation");
    await dispatchGreetingSubmit(client);
    await waitForLoading(client, updatedName, "successful navigation loading");
    assertState(await appState(client), {
        route: "greeting",
        hash: greetingHash(updatedName),
        routeTarget: greetingTarget(updatedName),
        status: "loading",
        loading: true,
        ready: false,
        failed: false,
        message: "",
    }, "server-backed successful navigation loading");
    await waitForGreeting(client, updatedMessage, "updated backend greeting");
    const successReport = await finishScenario(client, "successful-navigation");
    assertGreetingRouteReport(successReport, "successful navigation", {
        outcome: "success",
        routeChanges: 1,
        samePattern: true,
    });
    assertState(await appState(client), {
        app: true,
        route: "greeting",
        hash: greetingHash(updatedName),
        routeTarget: greetingTarget(updatedName),
        ready: true,
        failed: false,
        status: "ready",
        message: updatedMessage,
        origin: expectedApp.origin,
        input: updatedName,
        appSame: true,
        shellSame: true,
        routeContentSame: true,
    }, "server-backed form submit render");

    const successfulReloadBaseline = await evidenceState(client);
    const successfulReloadHash = (await appState(client)).hash;
    await startScenario(client, "successful-same-target-reload");
    await dispatchGreetingSubmit(client);
    await waitForFetchCount(client, successfulReloadBaseline.fetchesStarted + 1, "successful same-target reload fetch");
    await waitForLoading(client, updatedName, "successful same-target reload loading");
    await waitForGreeting(client, updatedMessage, "successful same-target reload ready");
    const successfulReloadReport = await finishScenario(client, "successful-same-target-reload");
    assertGreetingRouteReport(successfulReloadReport, "successful same-target reload", {
        outcome: "success",
        routeChanges: 0,
        samePattern: true,
    });
    assertState(await appState(client), {
        hash: successfulReloadHash,
        routeTarget: greetingTarget(updatedName),
        status: "ready",
        message: updatedMessage,
        input: updatedName,
    }, "server-backed successful same-target reload");

    await prepareGreetingName(client, slowName);
    await startScenario(client, "same-pattern-slow-start");
    await dispatchGreetingSubmit(client);
    const supersededSlowRequest = await waitForSlowActive(client, "same-pattern slow request active");
    const slowStartReport = await finishScenario(client, "same-pattern-slow-start");
    assertGreetingRouteReport(slowStartReport, "same-pattern slow start", {
        outcome: "pending",
        routeChanges: 1,
        samePattern: true,
    });

    await prepareGreetingName(client, updatedName);
    await startScenario(client, "same-pattern-supersede");
    await dispatchGreetingSubmit(client);
    await waitForLoading(client, updatedName, "newer Ada route loading after slow request");
    await waitForRequestAbort(client, supersededSlowRequest.id, "same-pattern slow request abort");
    await waitForGreeting(client, updatedMessage, "newer Ada backend greeting after slow request");
    await wait(staleSettleMS);
    const supersedeReport = await finishScenario(client, "same-pattern-supersede");
    assertGreetingRouteReport(supersedeReport, "same-pattern supersede", {
        aborted: 1,
        outcome: "success",
        routeChanges: 1,
        samePattern: true,
    });
    assertState(await appState(client), {
        app: true,
        route: "greeting",
        hash: greetingHash(updatedName),
        routeTarget: greetingTarget(updatedName),
        ready: true,
        failed: false,
        status: "ready",
        message: updatedMessage,
        input: updatedName,
        error: "",
        errorNonEmpty: false,
        appSame: true,
        shellSame: true,
        routeContentSame: true,
    }, "server-backed stale slow result ignored");
    assertRequest(await requestEvidence(client, supersededSlowRequest.id), {
        name: slowName,
        outcome: "aborted",
        aborted: true,
    }, "same-pattern superseded request");

    await prepareGreetingName(client, slowName);
    await startScenario(client, "unmount-slow-start");
    await dispatchGreetingSubmit(client);
    const unmountedSlowRequest = await waitForSlowActive(client, "unmount slow request active");
    const unmountSlowStartReport = await finishScenario(client, "unmount-slow-start");
    assertGreetingRouteReport(unmountSlowStartReport, "unmount slow start", {
        outcome: "pending",
        routeChanges: 1,
        samePattern: true,
    });

    await startScenario(client, "route-unmount-cancellation");
    await navigateHome(client);
    await waitForRoute(client, "home", "/");
    await waitForRequestAbort(client, unmountedSlowRequest.id, "route unmount slow request abort");
    await wait(staleSettleMS);
    const unmountReport = await finishScenario(client, "route-unmount-cancellation");
    assertHomeNavigationReport(unmountReport, "route unmount cancellation");
    assertState(await appState(client), {
        route: "home",
        hash: "#/",
        routeTarget: "/",
        message: "",
        loading: false,
        ready: false,
        failed: false,
        input: initialName,
        appSame: true,
        shellSame: true,
        routeContentSame: true,
    }, "server-backed route unmount cancellation");
    assertRequest(await requestEvidence(client, unmountedSlowRequest.id), {
        name: slowName,
        outcome: "aborted",
        aborted: true,
    }, "route-unmounted request");

    await prepareGreetingName(client, failureName);
    await startScenario(client, "controlled-failure");
    await dispatchGreetingSubmit(client);
    await waitForLoading(client, failureName, "controlled backend failure loading");
    await waitForFailure(client, failureMessage, "controlled backend failure");
    const failureReport = await finishScenario(client, "controlled-failure");
    assertGreetingRouteReport(failureReport, "controlled failure", {
        outcome: "failed",
        routeChanges: 1,
        samePattern: false,
    });
    assertState(await appState(client), {
        app: true,
        route: "greeting",
        hash: greetingHash(failureName),
        routeTarget: greetingTarget(failureName),
        ready: false,
        failed: true,
        status: failureStatus,
        input: failureName,
        error: failureMessage,
        errorNonEmpty: true,
        appSame: true,
        shellSame: true,
        routeContentSame: true,
    }, "server-backed controlled failure render");

    const failedRetryBaseline = await evidenceState(client);
    const failedRetryHash = (await appState(client)).hash;
    await startScenario(client, "failed-same-target-retry");
    await dispatchGreetingSubmit(client);
    await waitForFetchCount(client, failedRetryBaseline.fetchesStarted + 1, "failed same-target retry fetch");
    await waitForLoading(client, failureName, "failed same-target retry loading");
    await waitForFailure(client, failureMessage, "failed same-target retry result");
    const failedRetryReport = await finishScenario(client, "failed-same-target-retry");
    assertGreetingRouteReport(failedRetryReport, "failed same-target retry", {
        outcome: "failed",
        routeChanges: 0,
        samePattern: true,
    });
    assertState(await appState(client), {
        hash: failedRetryHash,
        routeTarget: greetingTarget(failureName),
        status: failureStatus,
        input: failureName,
        error: failureMessage,
        errorNonEmpty: true,
    }, "server-backed failed same-target retry");

    await prepareGreetingName(client, updatedName);
    await startScenario(client, "failure-recovery");
    await dispatchGreetingSubmit(client);
    await waitForLoading(client, updatedName, "failure recovery loading");
    await waitForGreeting(client, updatedMessage, "recovered backend greeting");
    const recoveryReport = await finishScenario(client, "failure-recovery");
    assertGreetingRouteReport(recoveryReport, "failure recovery", {
        outcome: "success",
        routeChanges: 1,
        samePattern: true,
    });
    assertState(await appState(client), {
        app: true,
        route: "greeting",
        hash: greetingHash(updatedName),
        routeTarget: greetingTarget(updatedName),
        ready: true,
        failed: false,
        status: "ready",
        message: updatedMessage,
        origin: expectedApp.origin,
        input: updatedName,
        error: "",
        errorNonEmpty: false,
        appSame: true,
        shellSame: true,
        routeContentSame: true,
    }, "server-backed recovery render");

    await startScenario(client, "browser-back");
    await client.evaluate("history.back()");
    await waitForLoading(client, failureName, "browser back failure loading");
    await waitForFailure(client, failureMessage, "browser back failed route");
    const backReport = await finishScenario(client, "browser-back");
    assertGreetingRouteReport(backReport, "browser back", {
        outcome: "failed",
        routeChanges: 1,
        samePattern: true,
    });
    assertState(await appState(client), {
        route: "greeting",
        hash: greetingHash(failureName),
        routeTarget: greetingTarget(failureName),
        status: "failed",
        error: failureMessage,
        input: failureName,
        appSame: true,
        shellSame: true,
        routeContentSame: true,
    }, "server-backed browser back");

    await startScenario(client, "browser-forward");
    await client.evaluate("history.forward()");
    await waitForLoading(client, updatedName, "browser forward Ada loading");
    await waitForGreeting(client, updatedMessage, "browser forward Ada ready");
    const forwardReport = await finishScenario(client, "browser-forward");
    assertGreetingRouteReport(forwardReport, "browser forward", {
        outcome: "success",
        routeChanges: 1,
        samePattern: true,
    });
    assertState(await appState(client), {
        route: "greeting",
        hash: greetingHash(updatedName),
        routeTarget: greetingTarget(updatedName),
        status: "ready",
        message: updatedMessage,
        input: updatedName,
        appSame: true,
        shellSame: true,
        routeContentSame: true,
    }, "server-backed browser forward");

    const routeEvidence = await evidenceState(client);
    assertEvidence(routeEvidence, {
        fetchesStarted: 11,
        aborts: 2,
        successfulCompletions: 6,
        failedCompletions: 3,
        staleResultAppearances: 0,
        appIdentityChanges: 0,
        shellIdentityChanges: 0,
        routeContentIdentityChanges: 0,
    }, "final route/resource evidence");
    assertStringArray(routeEvidence.routeTargetsVisited, [
        "",
        greetingHash(directName),
        greetingHash(updatedName),
        greetingHash(slowName),
        greetingHash(updatedName),
        greetingHash(slowName),
        "#/",
        greetingHash(failureName),
        greetingHash(updatedName),
        greetingHash(failureName),
        greetingHash(updatedName),
    ], "route targets visited");
    console.log(`server-backed async navigation evidence: ${JSON.stringify(routeEvidence)}`);

    await startScenario(client, "saved-initial-state");
    await navigateSavedGreeting(client);
    await waitForSavedReadLoading(client, "saved greeting initial read loading");
    await waitForSavedCommitted(client, savedInitialName, "saved greeting initial committed state");
    const savedInitialReport = await finishScenario(client, "saved-initial-state");
    assertSavedGreetingReport(savedInitialReport, "saved greeting initial state", {
        routeChanges: 1,
        samePattern: false,
        getsStarted: 1,
        getsCompleted: 1,
        postsStarted: 0,
    });
    assertState(await appState(client), {
        route: "savedGreeting",
        hash: savedGreetingHash,
        routeTarget: savedGreetingTarget,
        savedReadStatus: "ready",
        savedCommitted: savedInitialName,
        mutationStatus: "idle",
        mutationError: "",
        mutationErrorNonEmpty: false,
        mutationPending: false,
        mutationSubmitDisabled: false,
        appSame: true,
        shellSame: true,
        routeContentSame: true,
    }, "server-backed saved greeting initial state");

    await prepareSavedGreetingName(client, "   ");
    await startScenario(client, "saved-client-validation");
    await dispatchSavedGreetingSubmit(client);
    await waitForSavedValidationFailure(client, savedInitialName, "saved greeting client validation");
    const validationReport = await finishScenario(client, "saved-client-validation");
    assertSavedGreetingReport(validationReport, "saved greeting client validation", {
        routeChanges: 0,
        samePattern: true,
        getsStarted: 0,
        postsStarted: 0,
    });
    assertState(await appState(client), {
        savedCommitted: savedInitialName,
        mutationStatus: "validation failed",
        mutationError: "Name is required.",
        mutationErrorNonEmpty: true,
        mutationPending: false,
    }, "server-backed saved greeting validation state");

    await prepareSavedGreetingName(client, savedSlowName);
    const slowMutationBaseline = await evidenceState(client);
    await startScenario(client, "saved-slow-duplicate");
    await dispatchSavedGreetingSubmit(client);
    await waitForSavedMutationPending(client, savedInitialName, "saved greeting slow mutation pending");
    await waitForMutationPostCount(client, slowMutationBaseline.mutationPostsStarted + 1, "saved greeting slow POST start");
    await dispatchSavedGreetingSubmit(client);
    await waitForDuplicateSubmitCount(client, slowMutationBaseline.duplicateSubmitAttempts + 1, "saved greeting duplicate submit attempt");
    await wait(100);
    assertEvidence(await evidenceState(client), {
        mutationPostsStarted: slowMutationBaseline.mutationPostsStarted + 1,
        activeMutationRequests: 1,
    }, "saved greeting duplicate POST suppression");
    await waitForSavedCommitted(client, savedSlowName, "saved greeting slow committed state");
    const slowMutationReport = await finishScenario(client, "saved-slow-duplicate");
    assertSavedGreetingReport(slowMutationReport, "saved greeting slow mutation", {
        routeChanges: 0,
        samePattern: true,
        getsStarted: 1,
        getsCompleted: 1,
        postsStarted: 1,
        postsCompleted: 1,
        duplicateSubmitAttempts: 1,
        committedValues: [savedSlowName],
    });
    assertState(await appState(client), {
        savedReadStatus: "ready",
        savedCommitted: savedSlowName,
        mutationStatus: "success",
        mutationError: "",
        mutationErrorNonEmpty: false,
        mutationPending: false,
        mutationSubmitDisabled: false,
    }, "server-backed saved greeting slow success");

    await prepareSavedGreetingName(client, savedFailureName);
    await startScenario(client, "saved-controlled-failure");
    await dispatchSavedGreetingSubmit(client);
    await waitForSavedMutationPending(client, savedSlowName, "saved greeting controlled failure pending");
    await waitForSavedMutationFailure(client, savedSlowName, "saved greeting controlled server failure");
    const savedFailureReport = await finishScenario(client, "saved-controlled-failure");
    assertSavedGreetingReport(savedFailureReport, "saved greeting controlled failure", {
        routeChanges: 0,
        samePattern: true,
        getsStarted: 0,
        postsStarted: 1,
        postsFailed: 1,
    });
    assertState(await appState(client), {
        savedCommitted: savedSlowName,
        mutationStatus: "server failed",
        mutationError: savedFailureMessage,
        mutationErrorNonEmpty: true,
        mutationPending: false,
    }, "server-backed saved greeting server failure state");
    await assertSavedGreetingAPI(savedSlowName);

    await prepareSavedGreetingName(client, savedRecoveryName);
    await startScenario(client, "saved-recovery");
    await dispatchSavedGreetingSubmit(client);
    await waitForSavedMutationPending(client, savedSlowName, "saved greeting recovery pending");
    await waitForSavedCommitted(client, savedRecoveryName, "saved greeting recovery committed state");
    const savedRecoveryReport = await finishScenario(client, "saved-recovery");
    assertSavedGreetingReport(savedRecoveryReport, "saved greeting recovery", {
        routeChanges: 0,
        samePattern: true,
        getsStarted: 1,
        getsCompleted: 1,
        postsStarted: 1,
        postsCompleted: 1,
        committedValues: [savedRecoveryName],
    });
    assertState(await appState(client), {
        savedReadStatus: "ready",
        savedCommitted: savedRecoveryName,
        mutationStatus: "success",
        mutationError: "",
        mutationErrorNonEmpty: false,
        mutationPending: false,
        mutationSubmitDisabled: false,
    }, "server-backed saved greeting recovery state");
    await assertSavedGreetingAPI(savedRecoveryName);

    const finalEvidence = await evidenceState(client);
    assertEvidence(finalEvidence, {
        fetchesStarted: 11,
        aborts: 2,
        successfulCompletions: 6,
        failedCompletions: 3,
        savedGetsStarted: 3,
        savedGetsCompleted: 3,
        savedGetsFailed: 0,
        mutationPostsStarted: 3,
        mutationPostsCompleted: 2,
        mutationPostsFailed: 1,
        activeMutationRequests: 0,
        duplicateSubmitAttempts: 1,
        readReloadsAfterSuccess: 2,
        staleCommittedValueAppearances: 0,
        appIdentityChanges: 0,
        shellIdentityChanges: 0,
        routeContentIdentityChanges: 0,
    }, "final mutation evidence");
    assertStringArray(finalEvidence.committedValuesObserved, [
        savedInitialName,
        savedSlowName,
        savedRecoveryName,
    ], "saved greeting committed values observed");
    assertStringArray(finalEvidence.routeTargetsVisited, [
        "",
        greetingHash(directName),
        greetingHash(updatedName),
        greetingHash(slowName),
        greetingHash(updatedName),
        greetingHash(slowName),
        "#/",
        greetingHash(failureName),
        greetingHash(updatedName),
        greetingHash(failureName),
        greetingHash(updatedName),
        savedGreetingHash,
    ], "final route targets visited");
    console.log(`server-backed mutation evidence: ${JSON.stringify(finalEvidence)}`);

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

async function assertSavedGreetingAPI(expected) {
    const response = await fetch(`http://127.0.0.1:${backendPort}/api/saved-greeting`);
    if (!response.ok) {
        throw new Error(`HARNESS FAILURE: saved greeting API returned HTTP ${response.status}`);
    }
    const text = await response.text();
    if (text !== expected) {
        throw new Error(`HARNESS FAILURE: saved greeting API returned ${JSON.stringify(text)}, want ${JSON.stringify(expected)}`);
    }
    if (response.headers.get("cache-control") !== "no-store") {
        throw new Error("HARNESS FAILURE: saved greeting API did not return Cache-Control: no-store");
    }
}

async function rebuildDebugBundle() {
    const manifest = JSON.parse(await readFile(join(packageDir, "asset-manifest.json"), "utf8"));
    const wasm = manifest.entrypoints?.wasm;
    if (typeof wasm !== "string" || wasm === "") {
        throw new Error("HARNESS FAILURE: server-backed asset manifest has no WASM entrypoint");
    }
    await runCommand("go", ["build", "-tags=goframe_debug", "-o", join(packageDir, wasm), "."], {
        cwd: workspaceAppDir,
        env: {
            GOOS: "js",
            GOARCH: "wasm",
        },
    });
}

function greetingTarget(name) {
    return `/greeting?name=${encodeURIComponent(name)}`;
}

function greetingHash(name) {
    return `#${greetingTarget(name)}`;
}

async function initializeBrowserEvidence(client) {
    const initialized = await client.evaluate("window.__serverBackedEvidence?.initialize() ?? null");
    if (!initialized?.ready) {
        throw new Error(`APP FAILURE: server-backed evidence did not initialize: ${JSON.stringify(initialized)}`);
    }
}

async function prepareGreetingName(client, name) {
    const baseline = await client.callFunction(`function(name) {
        const input = document.querySelector("[data-testid='greeting-name']");
        if (!input) {
            return { ok: false, reason: "missing input" };
        }
        const owner = document.querySelector("[data-testid='server-backed-home']")
            ? "HomeRoute"
            : document.querySelector("[data-testid='server-backed-greeting-route']")
                ? "GreetingRoute"
                : "";
        if (!owner) {
            return { ok: false, reason: "missing state-owning route" };
        }
        const renders = window.goframeComponentRenderCounts?.[owner] || 0;
        const patches = window.goframeComponentPatchCounts?.[owner] || 0;
        input.value = name;
        input.dispatchEvent(new Event("input", { bubbles: true }));
        return { ok: true, owner, renders, patches, value: input.value };
    }`, name);
    if (!baseline?.ok) {
        throw new Error(`APP FAILURE: could not prepare greeting input: ${JSON.stringify(baseline)}`);
    }

    let updated = null;
    await waitForCondition(async () => {
        updated = await inputPreparationState(client, baseline.owner);
        return updated.value === name &&
            updated.renders > baseline.renders &&
            updated.patches > baseline.patches;
    }, `controlled input ${name} framework update`);

    await waitForBrowserFrames(client, 2);
    const settled = await inputPreparationState(client, baseline.owner);
    if (settled.value !== name ||
        settled.renders !== updated.renders ||
        settled.patches !== updated.patches) {
        throw new Error(`APP FAILURE: controlled input ${name} did not settle: ${JSON.stringify({ baseline, updated, settled })}`);
    }
    const report = { name, owner: baseline.owner, baseline, updated, settled };
    console.log(`server-backed input preparation: ${JSON.stringify(report)}`);
    return report;
}

async function prepareSavedGreetingName(client, name) {
    const baseline = await client.callFunction(`function(name) {
        const input = document.querySelector("[data-testid='saved-greeting-input']");
        if (!input) {
            return { ok: false, reason: "missing saved greeting input" };
        }
        const owner = "SavedGreetingRoute";
        const renders = window.goframeComponentRenderCounts?.[owner] || 0;
        const patches = window.goframeComponentPatchCounts?.[owner] || 0;
        input.value = name;
        input.dispatchEvent(new Event("input", { bubbles: true }));
        return { ok: true, owner, renders, patches, value: input.value };
    }`, name);
    if (!baseline?.ok) {
        throw new Error(`APP FAILURE: could not prepare saved greeting input: ${JSON.stringify(baseline)}`);
    }

    let updated = null;
    await waitForCondition(async () => {
        updated = await savedInputPreparationState(client);
        return updated.value === name &&
            updated.renders > baseline.renders &&
            updated.patches > baseline.patches;
    }, `controlled saved greeting input ${name} framework update`);

    await waitForBrowserFrames(client, 2);
    const settled = await savedInputPreparationState(client);
    if (settled.value !== name ||
        settled.renders !== updated.renders ||
        settled.patches !== updated.patches) {
        throw new Error(`APP FAILURE: controlled saved greeting input ${name} did not settle: ${JSON.stringify({ baseline, updated, settled })}`);
    }
    const report = { name, owner: baseline.owner, baseline, updated, settled };
    console.log(`server-backed saved input preparation: ${JSON.stringify(report)}`);
    return report;
}

async function savedInputPreparationState(client) {
    return await client.callFunction(`function() {
        const owner = "SavedGreetingRoute";
        return {
            owner,
            renders: window.goframeComponentRenderCounts?.[owner] || 0,
            patches: window.goframeComponentPatchCounts?.[owner] || 0,
            value: document.querySelector("[data-testid='saved-greeting-input']")?.value ?? "",
        };
    }`);
}

async function inputPreparationState(client, owner) {
    return await client.callFunction(`function(owner) {
        return {
            owner,
            renders: window.goframeComponentRenderCounts?.[owner] || 0,
            patches: window.goframeComponentPatchCounts?.[owner] || 0,
            value: document.querySelector("[data-testid='greeting-name']")?.value ?? "",
        };
    }`, owner);
}

async function waitForBrowserFrames(client, count) {
    await client.callFunction(`async function(count) {
        for (let index = 0; index < count; index++) {
            await new Promise((resolve) => requestAnimationFrame(resolve));
        }
        return true;
    }`, count);
}

async function dispatchGreetingSubmit(client) {
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

async function dispatchSavedGreetingSubmit(client) {
    const result = await client.evaluate(`(() => {
        const form = document.querySelector("[data-testid='saved-greeting-form']");
        if (!form) {
            return { ok: false, reason: "missing saved greeting form" };
        }
        form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
        return { ok: true };
    })()`);
    if (!result?.ok) {
        throw new Error(`APP FAILURE: could not submit saved greeting form: ${JSON.stringify(result)}`);
    }
}

async function waitForGreeting(client, expected, label) {
    await waitForCondition(async () => {
        const state = await appState(client);
        return state.ready && state.message === expected && state.origin === expectedApp.origin;
    }, label);
}

async function waitForFetchCount(client, expected, label) {
    await waitForCondition(async () => {
        return (await evidenceState(client)).fetchesStarted === expected;
    }, label);
}

async function waitForLoading(client, name, label) {
    await waitForCondition(async () => {
        const state = await appState(client);
        return state.route === "greeting" &&
            state.hash === greetingHash(name) &&
            state.routeTarget === greetingTarget(name) &&
            state.key === `/api/greeting?name=${encodeURIComponent(name)}` &&
            state.status === "loading" &&
            state.input === name &&
            state.loading &&
            !state.ready &&
            !state.failed;
    }, label);
}

async function waitForSlowActive(client, label) {
    let request = null;
    await waitForCondition(async () => {
        const state = await appState(client);
        const evidence = await evidenceState(client);
        request = [...evidence.requests].reverse().find((entry) => entry.name === slowName && entry.outcome === "pending") ?? null;
        return state.hash === greetingHash(slowName) &&
            state.status === "loading" &&
            state.loading &&
            request !== null;
    }, label);
    return request;
}

async function waitForRequestAbort(client, requestID, label) {
    await waitForCondition(async () => {
        const request = await requestEvidence(client, requestID);
        return request?.aborted === true && request.outcome === "aborted";
    }, label);
}

async function waitForFailure(client, expectedError, label) {
    await waitForCondition(async () => {
        const state = await appState(client);
        return state.failed && state.status === failureStatus && state.error === expectedError;
    }, label);
}

async function waitForRoute(client, route, target) {
    await waitForCondition(async () => {
        const state = await appState(client);
        return state.route === route && state.routeTarget === target;
    }, `route ${route} at ${target}`);
}

async function waitForSavedReadLoading(client, label) {
    await waitForCondition(async () => {
        const state = await appState(client);
        return state.route === "savedGreeting" &&
            state.hash === savedGreetingHash &&
            state.routeTarget === savedGreetingTarget &&
            state.savedResourceKey === "/api/saved-greeting" &&
            state.savedReadStatus === "loading" &&
            state.savedReadLoading &&
            !state.savedReadReady &&
            !state.savedReadFailed;
    }, label);
}

async function waitForSavedCommitted(client, expected, label) {
    await waitForCondition(async () => {
        const state = await appState(client);
        return state.route === "savedGreeting" &&
            state.savedReadStatus === "ready" &&
            state.savedReadReady &&
            !state.savedReadLoading &&
            !state.savedReadFailed &&
            state.savedCommitted === expected &&
            state.mutationStatus !== "pending";
    }, label);
}

async function waitForSavedValidationFailure(client, committed, label) {
    await waitForCondition(async () => {
        const state = await appState(client);
        return state.savedCommitted === committed &&
            state.savedReadStatus === "ready" &&
            state.mutationStatus === "validation failed" &&
            state.mutationError === "Name is required." &&
            !state.mutationPending;
    }, label);
}

async function waitForSavedMutationPending(client, committed, label) {
    await waitForCondition(async () => {
        const state = await appState(client);
        return state.savedCommitted === committed &&
            state.savedReadStatus === "ready" &&
            state.mutationStatus === "pending" &&
            state.mutationPending &&
            state.mutationSubmitDisabled &&
            !state.mutationErrorNonEmpty;
    }, label);
}

async function waitForSavedMutationFailure(client, committed, label) {
    await waitForCondition(async () => {
        const state = await appState(client);
        return state.savedCommitted === committed &&
            state.savedReadStatus === "ready" &&
            state.mutationStatus === "server failed" &&
            state.mutationError === savedFailureMessage &&
            state.mutationErrorNonEmpty &&
            !state.mutationPending;
    }, label);
}

async function waitForMutationPostCount(client, expected, label) {
    await waitForCondition(async () => {
        return (await evidenceState(client)).mutationPostsStarted === expected;
    }, label);
}

async function waitForDuplicateSubmitCount(client, expected, label) {
    await waitForCondition(async () => {
        return (await evidenceState(client)).duplicateSubmitAttempts === expected;
    }, label);
}

async function navigateHome(client) {
    await client.evaluate(`document.querySelector("[data-testid='server-backed-home-link']")?.click()`);
}

async function navigateSavedGreeting(client) {
    await client.evaluate(`document.querySelector("[data-testid='server-backed-saved-link']")?.click()`);
}

async function navigateGreetingHash(client, name) {
    await client.callFunction(`function(target) {
        window.location.hash = target;
        return window.location.hash;
    }`, greetingTarget(name));
}

async function appState(client) {
    return await client.callFunction(`function() {
        const message = document.querySelector("[data-testid='greeting-message']")?.textContent.trim() ?? "";
        const error = document.querySelector("[data-testid='greeting-error']")?.textContent.trim() ?? "";
        const key = document.querySelector("[data-testid='greeting-resource-key']")?.textContent.trim() ?? "";
        const home = document.querySelector("[data-testid='server-backed-home']");
        const greeting = document.querySelector("[data-testid='server-backed-greeting-route']");
        const savedGreeting = document.querySelector("[data-testid='server-backed-saved-route']");
        const notFound = document.querySelector("[data-testid='server-backed-not-found']");
        const mutationError = document.querySelector("[data-testid='saved-greeting-mutation-error']")?.textContent.trim() ?? "";
        const mutationStatus = document.querySelector("[data-testid='saved-greeting-mutation-status']")?.textContent.trim() ?? "";
        const identity = window.__serverBackedEvidence?.checkIdentity() ?? {};
        return {
            app: Boolean(document.querySelector("[data-testid='server-backed-app']")),
            shell: Boolean(document.querySelector("[data-testid='server-backed-shell']")),
            form: Boolean(document.querySelector("[data-testid='greeting-form']")),
            routeContent: Boolean(document.querySelector("[data-testid='server-backed-route-content']")),
            route: home ? "home" : greeting ? "greeting" : savedGreeting ? "savedGreeting" : notFound ? "notFound" : "missing",
            hash: window.location.hash,
            routeTarget: document.querySelector("[data-testid='server-backed-route-target']")?.textContent.trim() ?? "",
            loading: Boolean(document.querySelector("[data-testid='greeting-loading']")),
            ready: Boolean(document.querySelector("[data-testid='greeting-message']")),
            failed: Boolean(document.querySelector("[data-testid='greeting-error']")),
            status: document.querySelector("[data-testid='greeting-status']")?.textContent.trim() ?? "",
            input: document.querySelector("[data-testid='greeting-name']")?.value ?? "",
            key,
            message,
            error,
            errorNonEmpty: error.length > 0,
            savedForm: Boolean(document.querySelector("[data-testid='saved-greeting-form']")),
            savedInput: document.querySelector("[data-testid='saved-greeting-input']")?.value ?? "",
            savedResourceKey: document.querySelector("[data-testid='saved-greeting-resource-key']")?.textContent.trim() ?? "",
            savedReadStatus: document.querySelector("[data-testid='saved-greeting-read-status']")?.textContent.trim() ?? "",
            savedReadLoading: Boolean(document.querySelector("[data-testid='saved-greeting-read-loading']")),
            savedReadReady: Boolean(document.querySelector("[data-testid='saved-greeting-committed']")),
            savedReadFailed: Boolean(document.querySelector("[data-testid='saved-greeting-read-error']")),
            savedCommitted: document.querySelector("[data-testid='saved-greeting-committed']")?.textContent.trim() ?? "",
            mutationStatus,
            mutationPending: mutationStatus === "pending",
            mutationError,
            mutationErrorNonEmpty: mutationError.length > 0,
            mutationSubmitDisabled: document.querySelector("[data-testid='saved-greeting-submit']")?.disabled ?? false,
            origin: window.location.origin,
            appSame: identity.appSame ?? false,
            shellSame: identity.shellSame ?? false,
            routeContentSame: identity.routeContentSame ?? false,
        };
    }`);
}

async function evidenceState(client) {
    return await client.evaluate("window.__serverBackedEvidence?.snapshot() ?? null");
}

async function requestEvidence(client, requestID) {
    return await client.callFunction(`function(requestID) {
        return window.__serverBackedEvidence?.request(requestID) ?? null;
    }`, requestID);
}

async function startScenario(client, label) {
    const started = await client.callFunction(`function(label) {
        return window.__serverBackedAudit?.start(label) ?? false;
    }`, label);
    if (!started) {
        throw new Error(`APP FAILURE: server-backed audit did not start ${label}`);
    }
}

async function finishScenario(client, label) {
    const report = await client.callFunction(`function(label) {
        return window.__serverBackedAudit?.finish(label) ?? null;
    }`, label);
    if (!report) {
        throw new Error(`APP FAILURE: server-backed audit did not finish ${label}`);
    }
    assertFiniteReport(report, label);
    console.log(`server-backed DOM bridge ${label}: ${JSON.stringify(report)}`);
    return report;
}

function assertGreetingRouteReport(report, label, options) {
    assertSchedulingReport(report, label, "GreetingRoute");
    const expectedRequests = {
        started: 1,
        aborted: options.aborted ?? 0,
        succeeded: options.outcome === "success" ? 1 : 0,
        failed: options.outcome === "failed" ? 1 : 0,
    };
    assertState(report.requests, expectedRequests, `${label} request delta`);
    if (report.routeTargets.length !== options.routeChanges || report.routeContentMutations < 1) {
        throw new Error(`APP FAILURE: ${label} route evidence changed: ${JSON.stringify(report)}`);
    }
    assertReportIdentity(report, label, options.samePattern);
}

function assertHomeNavigationReport(report, label) {
    assertSchedulingReport(report, label, "HomeRoute");
    assertState(report.requests, {
        started: 0,
        aborted: 1,
        succeeded: 0,
        failed: 0,
    }, `${label} request delta`);
    if (report.routeTargets.length !== 1 || report.routeContentMutations < 1) {
        throw new Error(`APP FAILURE: ${label} home evidence changed: ${JSON.stringify(report)}`);
    }
    assertReportIdentity(report, label, false);
}

function assertSavedGreetingReport(report, label, options) {
    assertSchedulingReport(report, label, "SavedGreetingRoute");
    assertState(report.requests, {
        started: 0,
        aborted: 0,
        succeeded: 0,
        failed: 0,
    }, `${label} greeting request delta`);
    assertState(report.savedRequests, {
        getsStarted: options.getsStarted ?? 0,
        getsCompleted: options.getsCompleted ?? 0,
        getsFailed: options.getsFailed ?? 0,
        postsStarted: options.postsStarted ?? 0,
        postsCompleted: options.postsCompleted ?? 0,
        postsFailed: options.postsFailed ?? 0,
        duplicateSubmitAttempts: options.duplicateSubmitAttempts ?? 0,
        activeMutationRequests: 0,
    }, `${label} saved request delta`);
    if (report.routeTargets.length !== options.routeChanges || report.routeContentMutations < 1) {
        throw new Error(`APP FAILURE: ${label} route evidence changed: ${JSON.stringify(report)}`);
    }
    assertState(report.identity, {
        appSame: true,
        shellSame: true,
        routeContentSame: true,
        savedFormSame: options.samePattern,
        savedInputSame: options.samePattern,
    }, `${label} identity`);
    if (options.committedValues) {
        assertStringArray(report.committedValues, options.committedValues, `${label} committed values`);
    }
}

function assertSchedulingReport(report, label, component) {
    if (report.flushes < 1 ||
        report.scheduling.requestAnimationFrame !== report.scheduling.requestAnimationFrameCallbacks ||
        report.scheduling.requestAnimationFrameCallbacks !== report.flushes ||
        report.scheduling.queueMicrotask !== 0 ||
        report.scheduling.queueMicrotaskCallbacks !== 0 ||
        report.componentRenders[component] < 1) {
        throw new Error(`APP FAILURE: ${label} scheduling invariants changed: ${JSON.stringify(report)}`);
    }
}

function assertReportIdentity(report, label, samePattern) {
    const expected = {
        appSame: true,
        shellSame: true,
        routeContentSame: true,
        formSame: samePattern,
        inputSame: samePattern,
    };
    assertState(report.identity, expected, `${label} identity`);
}

function assertFiniteReport(report, label) {
    for (const [groupName, group] of Object.entries({
        operations: report.operations,
        scheduling: report.scheduling,
        componentRenders: report.componentRenders,
        componentPatches: report.componentPatches,
    })) {
        for (const [name, value] of Object.entries(group)) {
            if (!Number.isFinite(value) || value < 0) {
                throw new Error(`APP FAILURE: ${label} ${groupName}.${name} is invalid: ${JSON.stringify(report)}`);
            }
        }
    }
}

function assertRequest(actual, expected, label) {
    if (!actual) {
        throw new Error(`APP FAILURE: ${label}: missing request evidence`);
    }
    assertState(actual, expected, label);
}

function assertEvidence(actual, expected, label) {
    if (!actual) {
        throw new Error(`APP FAILURE: ${label}: missing evidence`);
    }
    assertState(actual, expected, label);
}

function assertStringArray(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        throw new Error(`APP FAILURE: ${label}: got ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`);
    }
    console.log(`${label}: ok`);
}

function emptyOperations() {
    return {
        createElement: 0,
        createTextNode: 0,
        createComment: 0,
        createDocumentFragment: 0,
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

function installServerBackedEvidenceExpression() {
    return `(() => {
    const componentNames = ${JSON.stringify(componentNames)};
    const slowMessage = ${JSON.stringify(slowMessage)};
    const requests = [];
    const savedRequests = [];
    const committedValuesObserved = [];
    const routeTargetsVisited = [];
    const operations = ${JSON.stringify(emptyOperations())};
    const scheduling = {
        requestAnimationFrame: 0,
        requestAnimationFrameCallbacks: 0,
        queueMicrotask: 0,
        queueMicrotaskCallbacks: 0,
    };
    const renderReports = [];
    let nextRequestID = 1;
    let nextSavedRequestID = 1;
    let aborts = 0;
    let successfulCompletions = 0;
    let failedCompletions = 0;
    let staleResultAppearances = 0;
    let routeContentMutations = 0;
    let lastMessage = "";
    let lastCommittedValue = "";
    let savedGetsStarted = 0;
    let savedGetsCompleted = 0;
    let savedGetsFailed = 0;
    let mutationPostsStarted = 0;
    let mutationPostsCompleted = 0;
    let mutationPostsFailed = 0;
    let activeMutationRequests = 0;
    let duplicateSubmitAttempts = 0;
    let readReloadsAfterSuccess = 0;
    let staleCommittedValueAppearances = 0;

    window.goframeComponentRenderCounts = {};
    window.goframeComponentPatchCounts = {};
    window.goframeComponentRenderProbe = (name) => {
        window.goframeComponentRenderCounts[name] =
            (window.goframeComponentRenderCounts[name] || 0) + 1;
    };
    window.goframeComponentPatchProbe = (name) => {
        window.goframeComponentPatchCounts[name] =
            (window.goframeComponentPatchCounts[name] || 0) + 1;
    };
    window.goframeRenderProbe = (phase, duration) => {
        renderReports.push({ phase, duration });
    };

    const evidence = {
        stable: null,
        identityChanged: {
            app: false,
            shell: false,
            routeContent: false,
        },
        initialize() {
            this.stable = {
                app: document.querySelector("[data-testid='server-backed-app']"),
                shell: document.querySelector("[data-testid='server-backed-shell']"),
                routeContent: document.querySelector("[data-testid='server-backed-route-content']"),
            };
            if (!this.stable.app || !this.stable.shell || !this.stable.routeContent) {
                return { ready: false };
            }
            routeTargetsVisited.push(window.location.hash);
            new MutationObserver((records) => {
                routeContentMutations += records.length;
                const message = document.querySelector("[data-testid='greeting-message']")?.textContent.trim() || "";
                if (message === slowMessage && lastMessage !== slowMessage) {
                    staleResultAppearances++;
                }
                lastMessage = message;
                const committed = document.querySelector("[data-testid='saved-greeting-committed']")?.textContent.trim() || "";
                if (committed && committed !== lastCommittedValue) {
                    if (committedValuesObserved.includes(committed) &&
                        committedValuesObserved.at(-1) !== committed) {
                        staleCommittedValueAppearances++;
                    }
                    committedValuesObserved.push(committed);
                    lastCommittedValue = committed;
                }
            }).observe(this.stable.routeContent, {
                childList: true,
                subtree: true,
                characterData: true,
            });
            return { ready: true };
        },
        checkIdentity() {
            if (!this.stable) return {};
            const current = {
                app: document.querySelector("[data-testid='server-backed-app']"),
                shell: document.querySelector("[data-testid='server-backed-shell']"),
                routeContent: document.querySelector("[data-testid='server-backed-route-content']"),
            };
            for (const name of Object.keys(current)) {
                if (current[name] !== this.stable[name]) this.identityChanged[name] = true;
            }
            return {
                appSame: !this.identityChanged.app,
                shellSame: !this.identityChanged.shell,
                routeContentSame: !this.identityChanged.routeContent,
            };
        },
        request(id) {
            const request = requests.find((entry) => entry.id === id);
            return request ? { ...request } : null;
        },
        snapshot() {
            return {
                routeTargetsVisited: [...routeTargetsVisited],
                fetchesStarted: requests.length,
                aborts,
                successfulCompletions,
                failedCompletions,
                staleResultAppearances,
                savedGetsStarted,
                savedGetsCompleted,
                savedGetsFailed,
                mutationPostsStarted,
                mutationPostsCompleted,
                mutationPostsFailed,
                activeMutationRequests,
                duplicateSubmitAttempts,
                readReloadsAfterSuccess,
                staleCommittedValueAppearances,
                committedValuesObserved: [...committedValuesObserved],
                appIdentityChanges: this.identityChanged.app ? 1 : 0,
                shellIdentityChanges: this.identityChanged.shell ? 1 : 0,
                routeContentIdentityChanges: this.identityChanged.routeContent ? 1 : 0,
                routeContentMutations,
                requests: requests.map((request) => ({ ...request })),
                savedRequests: savedRequests.map((request) => ({ ...request })),
            };
        },
    };
    window.__serverBackedEvidence = evidence;

    window.addEventListener("hashchange", () => {
        routeTargetsVisited.push(window.location.hash);
    });

    document.addEventListener("submit", (event) => {
        if (event.target?.matches?.("[data-testid='saved-greeting-form']") &&
            activeMutationRequests > 0) {
            duplicateSubmitAttempts++;
        }
    }, true);

    const originalFetch = window.fetch.bind(window);
    window.fetch = function(input, init) {
        const requestURL = new URL(typeof input === "string" ? input : input.url, window.location.href);
        if (requestURL.pathname === "/api/greeting") {
            const signal = init?.signal;
            const request = {
                id: nextRequestID++,
                name: requestURL.searchParams.get("name") || "",
                target: requestURL.pathname + requestURL.search,
                outcome: "pending",
                aborted: false,
            };
            requests.push(request);
            if (signal) {
                signal.addEventListener("abort", () => {
                    if (request.aborted) return;
                    request.aborted = true;
                    request.outcome = "aborted";
                    aborts++;
                }, { once: true });
            }
            return originalFetch(input, init).then((response) => {
                // Keep loading observable without changing the backend or runtime.
                return new Promise((resolve) => {
                    window.setTimeout(() => {
                        if (!request.aborted) {
                            request.outcome = response.ok ? "success" : "failed";
                            if (response.ok) successfulCompletions++;
                            else failedCompletions++;
                        }
                        resolve(response);
                    }, request.name === ${JSON.stringify(slowName)} ? 0 : 140);
                });
            }, (error) => {
                if (signal?.aborted) {
                    request.aborted = true;
                    request.outcome = "aborted";
                } else {
                    request.outcome = "failed";
                    failedCompletions++;
                }
                throw error;
            });
        }
        if (requestURL.pathname !== "/api/saved-greeting") {
            return originalFetch(input, init);
        }

        const method = String(init?.method || "GET").toUpperCase();
        const signal = init?.signal;
        const name = method === "POST"
            ? new URLSearchParams(typeof init?.body === "string" ? init.body : "").get("name") || ""
            : "";
        const request = {
            id: nextSavedRequestID++,
            method,
            name,
            target: requestURL.pathname + requestURL.search,
            outcome: "pending",
            aborted: false,
        };
        savedRequests.push(request);
        if (method === "GET") {
            if (savedGetsStarted > 0) readReloadsAfterSuccess++;
            savedGetsStarted++;
        } else if (method === "POST") {
            mutationPostsStarted++;
            activeMutationRequests++;
        }
        const finish = (outcome) => {
            if (request.outcome !== "pending") return;
            request.outcome = outcome;
            if (method === "GET") {
                if (outcome === "success") savedGetsCompleted++;
                else savedGetsFailed++;
            } else if (method === "POST") {
                activeMutationRequests--;
                if (outcome === "success") mutationPostsCompleted++;
                else mutationPostsFailed++;
            }
        };
        if (signal) {
            signal.addEventListener("abort", () => {
                if (request.aborted) return;
                request.aborted = true;
                if (request.outcome === "pending" && method === "POST") {
                    activeMutationRequests--;
                }
                request.outcome = "aborted";
            }, { once: true });
        }
        return originalFetch(input, init).then((response) => {
            return new Promise((resolve) => {
                window.setTimeout(() => {
                    if (!request.aborted) finish(response.ok ? "success" : "failed");
                    resolve(response);
                }, method === "POST" && name === ${JSON.stringify(savedSlowName)} ? 0 : 140);
            });
        }, (error) => {
            if (signal?.aborted) {
                request.aborted = true;
                request.outcome = "aborted";
            } else {
                finish("failed");
            }
            throw error;
        });
    };

    const audit = {
        componentBaseline: null,
        renderBaseline: 0,
        requestBaseline: null,
        routeTargetBaseline: 0,
        routeMutationBaseline: 0,
        formBaseline: null,
        inputBaseline: null,
        savedFormBaseline: null,
        savedInputBaseline: null,
        committedValueBaseline: 0,
        start(label) {
            for (const name of Object.keys(operations)) operations[name] = 0;
            for (const name of Object.keys(scheduling)) scheduling[name] = 0;
            this.componentBaseline = snapshotComponentCounts();
            this.renderBaseline = renderReports.length;
            this.requestBaseline = {
                fetchesStarted: requests.length,
                aborts,
                successfulCompletions,
                failedCompletions,
                savedGetsStarted,
                savedGetsCompleted,
                savedGetsFailed,
                mutationPostsStarted,
                mutationPostsCompleted,
                mutationPostsFailed,
                activeMutationRequests,
                duplicateSubmitAttempts,
            };
            this.routeTargetBaseline = routeTargetsVisited.length;
            this.routeMutationBaseline = routeContentMutations;
            this.formBaseline = document.querySelector("[data-testid='greeting-form']");
            this.inputBaseline = document.querySelector("[data-testid='greeting-name']");
            this.savedFormBaseline = document.querySelector("[data-testid='saved-greeting-form']");
            this.savedInputBaseline = document.querySelector("[data-testid='saved-greeting-input']");
            this.committedValueBaseline = committedValuesObserved.length;
            this.label = label;
            return true;
        },
        finish(label) {
            if (!this.componentBaseline || this.label !== label) return null;
            const current = snapshotComponentCounts();
            const componentRenders = {};
            const componentPatches = {};
            for (const name of componentNames) {
                componentRenders[name] = current.renders[name] - this.componentBaseline.renders[name];
                componentPatches[name] = current.patches[name] - this.componentBaseline.patches[name];
            }
            const updates = renderReports.slice(this.renderBaseline).filter((entry) => entry.phase === "update");
            return {
                scenario: label,
                flushes: updates.length,
                updateDurations: updates.map((entry) => entry.duration),
                scheduling: { ...scheduling },
                operations: { ...operations },
                componentRenders,
                componentPatches,
                routeContentMutations: routeContentMutations - this.routeMutationBaseline,
                routeTargets: routeTargetsVisited.slice(this.routeTargetBaseline),
                identity: {
                    ...evidence.checkIdentity(),
                    formSame: document.querySelector("[data-testid='greeting-form']") === this.formBaseline,
                    inputSame: document.querySelector("[data-testid='greeting-name']") === this.inputBaseline,
                    savedFormSame: document.querySelector("[data-testid='saved-greeting-form']") === this.savedFormBaseline,
                    savedInputSame: document.querySelector("[data-testid='saved-greeting-input']") === this.savedInputBaseline,
                },
                requests: {
                    started: requests.length - this.requestBaseline.fetchesStarted,
                    aborted: aborts - this.requestBaseline.aborts,
                    succeeded: successfulCompletions - this.requestBaseline.successfulCompletions,
                    failed: failedCompletions - this.requestBaseline.failedCompletions,
                },
                savedRequests: {
                    getsStarted: savedGetsStarted - this.requestBaseline.savedGetsStarted,
                    getsCompleted: savedGetsCompleted - this.requestBaseline.savedGetsCompleted,
                    getsFailed: savedGetsFailed - this.requestBaseline.savedGetsFailed,
                    postsStarted: mutationPostsStarted - this.requestBaseline.mutationPostsStarted,
                    postsCompleted: mutationPostsCompleted - this.requestBaseline.mutationPostsCompleted,
                    postsFailed: mutationPostsFailed - this.requestBaseline.mutationPostsFailed,
                    duplicateSubmitAttempts: duplicateSubmitAttempts - this.requestBaseline.duplicateSubmitAttempts,
                    activeMutationRequests,
                },
                committedValues: committedValuesObserved.slice(this.committedValueBaseline),
            };
        },
    };
    window.__serverBackedAudit = audit;

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
    wrap(Document.prototype, "createDocumentFragment", "createDocumentFragment");
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

    return true;

    function snapshotComponentCounts() {
        const renders = {};
        const patches = {};
        for (const name of componentNames) {
            renders[name] = window.goframeComponentRenderCounts[name] || 0;
            patches[name] = window.goframeComponentPatchCounts[name] || 0;
        }
        return { renders, patches };
    }
})()`;
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
                ...options.env,
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

import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "node:net";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const rootDir = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const fixtureDir = join(rootDir, "scripts", "fixtures", "document-state");
const chrome = process.env.CHROME ?? "google-chrome";
const tempRoot = await mkdtemp(join(tmpdir(), "goframe-document-state-"));
const profile = await mkdtemp(join(tmpdir(), "goframe-document-state-chrome-"));
const appPort = Number(process.env.GOFRAME_DOCUMENT_STATE_PORT ?? await pickFreePort());
const debugPort = Number(
    process.env.GOFRAME_DOCUMENT_STATE_CHROME_DEBUG_PORT ?? await pickFreePort(),
);
const origin = `http://127.0.0.1:${appPort}`;
const appURL = `${origin}/?smoke=${Date.now()}#/`;

const authoredBaseline = {
    title: "GoFrame document-state fixture",
    description: "Authored document-state baseline",
};
const unrelatedValue = "preserve-me";
const speculativeState = {
    title: "Speculative failure · GoFrame",
    description: "This description must never commit.",
};

let browser = null;
let browserError = "";
let client = null;
let server = null;
const commandOutput = [];

try {
    const goxc = await prepareTools();
    await runCommand(goxc, ["package", fixtureDir, "--compiler=tinygo"]);
    await runCommand(goxc, ["package", fixtureDir, "--compiler=go"]);

    server = await startServer(
        goxc,
        ["serve", fixtureDir, `--port=${appPort}`],
        `${origin}/`,
        "document-state fixture",
    );

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

    const page = await waitForPage(debugPort);
    client = await connect(page.webSocketDebuggerUrl);
    await client.call("Runtime.enable");
    await client.call("Page.enable");
    await client.call("Page.addScriptToEvaluateOnNewDocument", {
        source: `(${installHeadObserver.toString()})()`,
    });

    await client.call("Page.navigate", { url: appURL });
    await waitForCondition(async () => {
        const state = await documentState(client);
        return state.baselineCaptured && state.app;
    }, "authored baseline capture and fixture boot");
    await assertAuthoredBaselineCapture(client);
    await rememberAppRoot(client);

    await setObservationPhase(client, "initial-home");
    await waitForDocumentState(client, routeState(
        "home",
        "/",
        "Home · GoFrame",
        "Document-state home route.",
    ), "initial home owner");

    await setObservationPhase(client, "same-pattern");
    await click(client, "document-user-42-link");
    await waitForDocumentState(client, userState("42"), "user 42");
    await click(client, "document-user-7-link");
    await waitForDocumentState(client, userState("7"), "same-pattern user 7");

    await setObservationPhase(client, "not-found");
    await click(client, "document-not-found-link");
    await waitForDocumentState(client, routeState(
        "not-found",
        "/does-not-exist",
        "Not found · GoFrame",
        "No document-state route matched.",
    ), "not-found owner");

    await setObservationPhase(client, "nested-override");
    await click(client, "document-user-42-link");
    await waitForDocumentState(client, userState("42"), "nested flow user 42");
    await click(client, "document-editor-open");
    await waitForDocumentState(client, editorState("42"), "editing user 42");
    await click(client, "document-user-7-link");
    await waitForDocumentState(client, editorState("7"), "parent update under editor");
    await click(client, "document-editor-close");
    await waitForDocumentState(client, userState("7"), "editor reveals latest parent");

    await setObservationPhase(client, "cross-pattern");
    await click(client, "document-editor-open");
    await waitForDocumentState(client, editorState("7"), "editor before cross-pattern navigation");
    await click(client, "document-search-link");
    const search = routeState(
        "search",
        "/search?q=runtime",
        "Search: runtime · GoFrame",
        "Search results for runtime.",
    );
    await waitForDocumentState(client, search, "cross-pattern search");
    await assertStableDocumentState(client, search, "cross-pattern cleanup");

    const rapidStart = await headSnapshotCount(client);
    await setObservationPhase(
        client,
        "rapid-update",
        ["User A · GoFrame", "User B · GoFrame"],
        ["Profile for user A.", "Profile for user B."],
    );
    await click(client, "document-rapid-update");
    const userC = userState("C");
    await waitForDocumentState(client, userC, "rapid user C");
    await assertStableDocumentState(client, userC, "rapid user C settle");
    await assertNoStaleRapidSnapshots(client, rapidStart);

    await setObservationPhase(client, "history");
    await click(client, "document-home-link");
    const home = routeState(
        "home",
        "/",
        "Home · GoFrame",
        "Document-state home route.",
    );
    await waitForDocumentState(client, home, "history home commit");
    await click(client, "document-search-link");
    await waitForDocumentState(client, search, "history search commit");
    await client.evaluate("history.back()");
    await waitForDocumentState(client, home, "Back");
    await client.evaluate("history.forward()");
    await waitForDocumentState(client, search, "Forward");

    await setObservationPhase(client, "failed-render");
    const beforeFailure = await documentState(client);
    await click(client, "document-failure-trigger");
    await waitForCondition(async () => {
        const state = await documentState(client);
        return state.failureFallback &&
            state.errorBoundaryCaptures === beforeFailure.errorBoundaryCaptures + 1;
    }, "speculative failure fallback");
    await assertDocumentState(client, search, "failed render retains route owner");
    const afterFailure = await documentState(client);
    assert(
        afterFailure.speculativeAppearances === 0,
        `speculative metadata appeared ${afterFailure.speculativeAppearances} times`,
    );
    assert(
        !afterFailure.ownershipEvents.some((event) => event.owner === "speculative"),
        "speculative owner reached the committed ownership model",
    );
    await click(client, "document-failure-reset");
    await waitForCondition(async () => {
        const state = await documentState(client);
        return !state.failureFallback;
    }, "failed subtree removal");
    await assertDocumentState(client, search, "failed subtree removal retains route owner");

    await setObservationPhase(client, "scope-unmount");
    await click(client, "document-scope-unmount");
    await waitForBaselineRestoration(client);
    await assertHeadNodeIdentity(client, "scope unmount");

    await setObservationPhase(client, "scope-remount");
    await click(client, "document-scope-remount");
    await waitForDocumentState(client, search, "scope remount current route");
    await assertHeadNodeIdentity(client, "scope remount");
    await assertAppRootIdentity(client);

    const final = await documentState(client);
    const counters = collectBehavioralCounters(final);
    assert(counters.routeMetadataCommits > 0, "route metadata commits were not observed");
    assert(counters.nestedOwnerActivations > 0, "nested owner activation was not observed");
    assert(counters.nestedOwnerReleases > 0, "nested owner release was not observed");
    assert(counters.ownerUpdates > 0, "owner updates were not observed");
    assert(counters.ownerRemovals > 0, "owner removals were not observed");
    assert(counters.titleMutationBatches > 0, "title mutations were not observed");
    assert(counters.descriptionMutationBatches > 0, "description mutations were not observed");
    assert(counters.headSnapshots > 0, "head snapshots were not observed");
    assert(counters.baselineRestorations === 1, `baseline restorations = ${counters.baselineRestorations}, want 1`);
    assert(counters.scopeMounts === 2, `scope mounts = ${counters.scopeMounts}, want 2`);
    assert(counters.scopeUnmounts === 1, `scope unmounts = ${counters.scopeUnmounts}, want 1`);
    assert(counters.errorBoundaryCaptures === 1, `boundary captures = ${counters.errorBoundaryCaptures}, want 1`);
    assert(counters.invalidPairs === 0, `invalid title/description pairs = ${counters.invalidPairs}`);
    assert(counters.duplicateDescriptions === 0, `duplicate descriptions = ${counters.duplicateDescriptions}`);
    assert(counters.staleAppearances === 0, `stale metadata appearances = ${counters.staleAppearances}`);
    assert(counters.speculativeAppearances === 0, `speculative metadata appearances = ${counters.speculativeAppearances}`);
    assert(counters.appRootIdentityChanges === 0, `app-root identity changes = ${counters.appRootIdentityChanges}`);
    assert(counters.unrelatedMetaMutations === 0, `unrelated-meta mutations = ${counters.unrelatedMetaMutations}`);

    console.log(`document state behavioral counters: ${JSON.stringify(counters)}`);
    console.log("document state ownership pressure: ok");
} catch (error) {
    const diagnostics = await collectDiagnostics();
    throw new Error(`${error.message}\n${JSON.stringify(diagnostics, null, 2)}`, {
        cause: error,
    });
} finally {
    client?.close();
    await stopProcess(browser, false);
    await stopProcess(server?.child, true);
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
    await rm(tempRoot, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
    await rm(join(fixtureDir, ".goframe"), {
        recursive: true,
        force: true,
        maxRetries: 5,
        retryDelay: 100,
    });
}

function routeState(route, target, title, description) {
    return {
        scopeActive: true,
        route,
        target,
        hash: `#${target}`,
        activeOwner: "route",
        title,
        description,
    };
}

function userState(id) {
    return routeState(
        "user",
        `/users/${id}`,
        `User ${id} · GoFrame`,
        `Profile for user ${id}.`,
    );
}

function editorState(id) {
    return {
        ...userState(id),
        activeOwner: "editor",
        title: `Editing User ${id} · GoFrame`,
        description: `Editing profile for user ${id}.`,
        editor: true,
    };
}

async function prepareTools() {
    if (process.env.GOXC) {
        return process.env.GOXC;
    }
    const goxc = join(tempRoot, "goxc");
    await runCommand("go", ["build", "-o", goxc, "./cmd/goxc"]);
    return goxc;
}

async function assertAuthoredBaselineCapture(browserClient) {
    const state = await documentState(browserClient);
    assert(state.baselineCaptured, "authored baseline was not captured before app ownership");
    assert(state.authoredTitle === authoredBaseline.title, `authored title = ${JSON.stringify(state.authoredTitle)}`);
    assert(
        state.authoredDescription === authoredBaseline.description,
        `authored description = ${JSON.stringify(state.authoredDescription)}`,
    );
    assert(state.authoredDescriptionCount === 1, `authored description count = ${state.authoredDescriptionCount}`);
    assert(state.authoredUnrelated === unrelatedValue, `authored unrelated meta = ${JSON.stringify(state.authoredUnrelated)}`);
}

async function waitForDocumentState(browserClient, expected, label) {
    await waitForCondition(async () => {
        const state = await documentState(browserClient);
        return documentStateMatches(state, expected);
    }, label);
    await assertDocumentState(browserClient, expected, label);
}

async function assertDocumentState(browserClient, expected, label) {
    const state = await documentState(browserClient);
    const mismatches = [];
    for (const [field, value] of Object.entries(expected)) {
        if (state[field] !== value) {
            mismatches.push(`${field}=${JSON.stringify(state[field])}, want ${JSON.stringify(value)}`);
        }
    }
    if (state.expectedTitle !== expected.title) {
        mismatches.push(`expectedTitle=${JSON.stringify(state.expectedTitle)}, want ${JSON.stringify(expected.title)}`);
    }
    if (state.expectedDescription !== expected.description) {
        mismatches.push(`expectedDescription=${JSON.stringify(state.expectedDescription)}, want ${JSON.stringify(expected.description)}`);
    }
    if (state.descriptionCount !== 1) {
        mismatches.push(`descriptionCount=${state.descriptionCount}, want 1`);
    }
    if (state.unrelated !== unrelatedValue) {
        mismatches.push(`unrelated=${JSON.stringify(state.unrelated)}, want ${JSON.stringify(unrelatedValue)}`);
    }
    if (!state.titleNodeSame || !state.descriptionNodeSame) {
        mismatches.push(`node identities title=${state.titleNodeSame} description=${state.descriptionNodeSame}`);
    }
    if (!state.appSame && state.appIdentityCaptured) {
        mismatches.push("application root identity changed");
    }
    if (mismatches.length > 0) {
        throw new Error(`APP FAILURE: ${label}: ${mismatches.join("; ")}`);
    }
    console.log(`${label}: ok`);
}

function documentStateMatches(state, expected) {
    return Object.entries(expected).every(([field, value]) => state[field] === value) &&
        state.expectedTitle === expected.title &&
        state.expectedDescription === expected.description &&
        state.descriptionCount === 1 &&
        state.unrelated === unrelatedValue;
}

async function assertStableDocumentState(browserClient, expected, label) {
    for (let attempt = 0; attempt < 5; attempt++) {
        await wait(100);
        await assertDocumentState(browserClient, expected, `${label} ${attempt + 1}/5`);
    }
}

async function waitForBaselineRestoration(browserClient) {
    await waitForCondition(async () => {
        const state = await documentState(browserClient);
        return state.scopeInactive &&
            state.activeOwner === "authored-baseline" &&
            state.title === authoredBaseline.title &&
            state.description === authoredBaseline.description &&
            state.expectedTitle === authoredBaseline.title &&
            state.expectedDescription === authoredBaseline.description;
    }, "authored baseline restoration");
    const state = await documentState(browserClient);
    assert(state.descriptionCount === 1, `baseline description count = ${state.descriptionCount}`);
    assert(state.unrelated === unrelatedValue, `baseline unrelated meta = ${JSON.stringify(state.unrelated)}`);
    console.log("authored baseline restoration: ok");
}

async function assertNoStaleRapidSnapshots(browserClient, start) {
    const result = await browserClient.evaluate(`(() => {
        const evidence = globalThis.__documentStateHeadEvidence;
        const staleTitles = new Set(["User A · GoFrame", "User B · GoFrame"]);
        const staleDescriptions = new Set(["Profile for user A.", "Profile for user B."]);
        const snapshots = evidence.snapshots.slice(${start});
        return {
            stale: snapshots.filter((snapshot) =>
                staleTitles.has(snapshot.title) ||
                staleDescriptions.has(snapshot.description)
            ),
            count: snapshots.length,
        };
    })()`);
    assert(result.stale.length === 0, `rapid updates exposed stale metadata: ${JSON.stringify(result)}`);
}

async function setObservationPhase(
    browserClient,
    phase,
    forbiddenTitles = [],
    forbiddenDescriptions = [],
) {
    await browserClient.evaluate(`(() => {
        const evidence = globalThis.__documentStateHeadEvidence;
        evidence.phase = ${JSON.stringify(phase)};
        evidence.forbiddenTitles = ${JSON.stringify(forbiddenTitles)};
        evidence.forbiddenDescriptions = ${JSON.stringify(forbiddenDescriptions)};
        return true;
    })()`);
}

async function headSnapshotCount(browserClient) {
    return await browserClient.evaluate(
        "globalThis.__documentStateHeadEvidence?.snapshots.length ?? 0",
    );
}

async function rememberAppRoot(browserClient) {
    const captured = await browserClient.evaluate(`(() => {
        globalThis.__documentStateAppRoot =
            document.querySelector("[data-testid='document-state-app']");
        return Boolean(globalThis.__documentStateAppRoot);
    })()`);
    assert(captured, "application root was unavailable for identity capture");
}

async function assertAppRootIdentity(browserClient) {
    const same = await browserClient.evaluate(`(
        globalThis.__documentStateAppRoot ===
        document.querySelector("[data-testid='document-state-app']")
    )`);
    assert(same, "application root identity changed");
}

async function assertHeadNodeIdentity(browserClient, label) {
    const state = await documentState(browserClient);
    assert(state.titleNodeSame, `title node identity changed during ${label}`);
    assert(state.descriptionNodeSame, `description node identity changed during ${label}`);
}

async function click(browserClient, testID) {
    const clicked = await browserClient.evaluate(`(() => {
        const element = document.querySelector(
            "[data-testid='${testID}']"
        );
        if (!element) return false;
        element.click();
        return true;
    })()`);
    assert(clicked, `missing element for click ${testID}`);
}

async function documentState(browserClient) {
    return await browserClient.evaluate(`(() => {
        const text = (id) =>
            document.querySelector("[data-testid='" + id + "']")
                ?.textContent.trim() ?? "";
        const descriptions = document.querySelectorAll(
            'head meta[name="description"]'
        );
        const unrelated = document.querySelector(
            'head meta[name="fixture-unrelated"]'
        );
        const head = globalThis.__documentStateHeadEvidence || {};
        const app = globalThis.goframeDocumentStateEvidence || {};
        const titleNode = document.querySelector("head title");
        const descriptionNode = descriptions[0] || null;
        return {
            href: location.href,
            hash: location.hash,
            app: Boolean(document.querySelector(
                "[data-testid='document-state-app']"
            )),
            scopeActive: Boolean(document.querySelector(
                "[data-testid='document-state-scope']"
            )),
            scopeInactive: Boolean(document.querySelector(
                "[data-testid='document-state-scope-inactive']"
            )),
            failureFallback: Boolean(document.querySelector(
                "[data-testid='document-failure-fallback']"
            )),
            editor: Boolean(document.querySelector(
                "[data-testid='document-editor']"
            )),
            route: text("document-route-name"),
            target: text("document-route-target"),
            activeOwner: text("document-active-owner"),
            expectedTitle: text("document-expected-title"),
            expectedDescription: text("document-expected-description"),
            title: document.title,
            description: descriptionNode?.getAttribute("content") ?? "",
            descriptionCount: descriptions.length,
            unrelated: unrelated?.getAttribute("content") ?? "",
            baselineCaptured: Boolean(head.baselineCaptured),
            authoredTitle: head.authoredTitle ?? "",
            authoredDescription: head.authoredDescription ?? "",
            authoredDescriptionCount: head.authoredDescriptionCount ?? 0,
            authoredUnrelated: head.authoredUnrelated ?? "",
            titleNodeSame: head.titleNode === titleNode,
            descriptionNodeSame: head.descriptionNode === descriptionNode,
            appIdentityCaptured: Boolean(globalThis.__documentStateAppRoot),
            appSame: globalThis.__documentStateAppRoot
                ? globalThis.__documentStateAppRoot === document.querySelector(
                    "[data-testid='document-state-app']"
                )
                : null,
            titleMutationBatches: head.titleMutationBatches ?? 0,
            descriptionMutationBatches:
                head.descriptionMutationBatches ?? 0,
            headSnapshots: head.headSnapshots ?? 0,
            invalidPairs: head.invalidPairs ?? 0,
            duplicateDescriptions: head.duplicateDescriptions ?? 0,
            staleAppearances: head.staleAppearances ?? 0,
            speculativeAppearances: head.speculativeAppearances ?? 0,
            unrelatedMetaMutations: head.unrelatedMetaMutations ?? 0,
            headSnapshotRecords: Array.from(head.snapshots || []),
            routeMetadataCommits: Array.from(app.ownershipEvents || [])
                .filter((event) =>
                    event.owner === "route" &&
                    (event.change === "added" || event.change === "updated")
                ).length,
            nestedOwnerActivations: app.nestedOwnerActivations ?? 0,
            nestedOwnerReleases: app.nestedOwnerReleases ?? 0,
            ownerUpdates: app.ownerUpdates ?? 0,
            ownerRemovals: app.ownerRemovals ?? 0,
            baselineRestorations: app.baselineRestorations ?? 0,
            scopeMounts: app.scopeMounts ?? 0,
            scopeUnmounts: app.scopeUnmounts ?? 0,
            errorBoundaryCaptures: app.errorBoundaryCaptures ?? 0,
            ownershipEvents: Array.from(app.ownershipEvents || []),
            runtimeErrors: Array.from(app.runtimeErrors || []),
        };
    })()`);
}

function collectBehavioralCounters(state) {
    return {
        routeMetadataCommits: state.routeMetadataCommits,
        nestedOwnerActivations: state.nestedOwnerActivations,
        nestedOwnerReleases: state.nestedOwnerReleases,
        ownerUpdates: state.ownerUpdates,
        ownerRemovals: state.ownerRemovals,
        titleMutationBatches: state.titleMutationBatches,
        descriptionMutationBatches: state.descriptionMutationBatches,
        headSnapshots: state.headSnapshots,
        invalidPairs: state.invalidPairs,
        duplicateDescriptions: state.duplicateDescriptions,
        staleAppearances: state.staleAppearances,
        speculativeAppearances: state.speculativeAppearances,
        baselineRestorations: state.baselineRestorations,
        scopeMounts: state.scopeMounts,
        scopeUnmounts: state.scopeUnmounts,
        errorBoundaryCaptures: state.errorBoundaryCaptures,
        appRootIdentityChanges: state.appSame ? 0 : 1,
        unrelatedMetaMutations: state.unrelatedMetaMutations,
    };
}

function installHeadObserver() {
    const evidence = {
        baselineCaptured: false,
        authoredTitle: "",
        authoredDescription: "",
        authoredDescriptionCount: 0,
        authoredUnrelated: "",
        titleNode: null,
        descriptionNode: null,
        unrelatedNode: null,
        titleMutationBatches: 0,
        descriptionMutationBatches: 0,
        headSnapshots: 0,
        invalidPairs: 0,
        duplicateDescriptions: 0,
        staleAppearances: 0,
        speculativeAppearances: 0,
        unrelatedMetaMutations: 0,
        phase: "document-parse",
        forbiddenTitles: [],
        forbiddenDescriptions: [],
        snapshots: [],
    };
    globalThis.__documentStateHeadEvidence = evidence;

    const captureBaseline = () => {
        if (evidence.baselineCaptured) return true;
        const titles = document.querySelectorAll("head title");
        const descriptions = document.querySelectorAll(
            'head meta[name="description"]'
        );
        const unrelated = document.querySelector(
            'head meta[name="fixture-unrelated"]'
        );
        if (titles.length !== 1 || descriptions.length !== 1 || !unrelated) {
            return false;
        }
        evidence.titleNode = titles[0];
        evidence.descriptionNode = descriptions[0];
        evidence.unrelatedNode = unrelated;
        evidence.authoredTitle = titles[0].textContent;
        evidence.authoredDescription =
            descriptions[0].getAttribute("content") ?? "";
        evidence.authoredDescriptionCount = descriptions.length;
        evidence.authoredUnrelated =
            unrelated.getAttribute("content") ?? "";
        evidence.baselineCaptured = true;
        return true;
    };

    const containsNode = (record, node) => {
        if (!node) return false;
        if (record.target === node || node.contains(record.target)) return true;
        return [...record.addedNodes, ...record.removedNodes].some(
            (changed) => changed === node ||
                (changed.nodeType === Node.ELEMENT_NODE &&
                    changed.contains(node))
        );
    };

    const isDescriptionNode = (node) =>
        node?.nodeType === Node.ELEMENT_NODE &&
        node.matches?.('meta[name="description"]');

    const observer = new MutationObserver((records) => {
        const baselineWasCaptured = evidence.baselineCaptured;
        if (!captureBaseline()) return;
        if (!baselineWasCaptured) return;
        const titleTouched = records.some((record) =>
            containsNode(record, evidence.titleNode)
        );
        const descriptionTouched = records.some((record) =>
            containsNode(record, evidence.descriptionNode) ||
            [...record.addedNodes, ...record.removedNodes].some(
                isDescriptionNode
            )
        );
        const unrelatedTouched = records.some((record) =>
            containsNode(record, evidence.unrelatedNode)
        );
        if (!titleTouched && !descriptionTouched && !unrelatedTouched) {
            return;
        }
        if (titleTouched) evidence.titleMutationBatches++;
        if (descriptionTouched) evidence.descriptionMutationBatches++;
        if (unrelatedTouched) evidence.unrelatedMetaMutations++;

        const text = (id) =>
            document.querySelector("[data-testid='" + id + "']")
                ?.textContent.trim() ?? "";
        const descriptions = document.querySelectorAll(
            'head meta[name="description"]'
        );
        const description = descriptions[0]?.getAttribute("content") ?? "";
        const unrelated = document.querySelector(
            'head meta[name="fixture-unrelated"]'
        )?.getAttribute("content") ?? "";
        const snapshot = {
            phase: evidence.phase,
            title: document.title,
            description,
            descriptionCount: descriptions.length,
            unrelated,
            route: text("document-route-name"),
            target: text("document-route-target"),
            activeOwner: text("document-active-owner"),
            expectedTitle: text("document-expected-title"),
            expectedDescription: text("document-expected-description"),
        };
        evidence.snapshots.push(snapshot);
        evidence.headSnapshots++;

        if (snapshot.expectedTitle && (
            snapshot.title !== snapshot.expectedTitle ||
            snapshot.description !== snapshot.expectedDescription
        )) {
            evidence.invalidPairs++;
        }
        if (snapshot.descriptionCount !== 1) {
            evidence.duplicateDescriptions++;
        }
        if (snapshot.unrelated !== evidence.authoredUnrelated) {
            evidence.unrelatedMetaMutations++;
        }
        if (
            snapshot.title === "Speculative failure · GoFrame" ||
            snapshot.description === "This description must never commit."
        ) {
            evidence.speculativeAppearances++;
        }
        if (
            evidence.forbiddenTitles.includes(snapshot.title) ||
            evidence.forbiddenDescriptions.includes(snapshot.description)
        ) {
            evidence.staleAppearances++;
        }
    });
    observer.observe(document, {
        subtree: true,
        childList: true,
        attributes: true,
        characterData: true,
    });
    queueMicrotask(captureBaseline);
    document.addEventListener("DOMContentLoaded", captureBaseline, {
        once: true,
    });
}

async function collectDiagnostics() {
    const diagnostics = {
        appURL,
        appPort,
        debugPort,
        browserStderr: browserError.slice(-6000),
        serverOutput: server?.output?.().slice(-6000) ?? "",
        commandOutput: commandOutput.slice(-8),
    };
    if (client) {
        try {
            const state = await documentState(client);
            diagnostics.page = {
                href: state.href,
                hash: state.hash,
                route: state.route,
                target: state.target,
                activeOwner: state.activeOwner,
                expectedTitle: state.expectedTitle,
                actualTitle: state.title,
                expectedDescription: state.expectedDescription,
                actualDescription: state.description,
                descriptionCount: state.descriptionCount,
                unrelated: state.unrelated,
                failureFallback: state.failureFallback,
                runtimeErrors: state.runtimeErrors,
                recentHeadSnapshots: state.headSnapshotRecords.slice(-12),
                recentOwnershipEvents: state.ownershipEvents.slice(-12),
            };
        } catch (error) {
            diagnostics.pageError = error.message;
        }
    }
    return diagnostics;
}

async function startServer(command, args, readyURL, label) {
    const child = spawn(command, args, {
        cwd: rootDir,
        detached: true,
        stdio: ["ignore", "pipe", "pipe"],
        env: {
            ...process.env,
            GOWORK: "off",
        },
    });
    let output = "";
    child.stdout.on("data", (chunk) => {
        output += chunk;
    });
    child.stderr.on("data", (chunk) => {
        output += chunk;
    });
    try {
        await waitForHTTP(readyURL, child, label, () => output);
    } catch (error) {
        await stopProcess(child, true);
        throw error;
    }
    return { child, output: () => output };
}

async function waitForHTTP(url, child, label, output) {
    let lastError = null;
    for (let attempt = 0; attempt < 120; attempt++) {
        if (child.exitCode !== null || child.signalCode !== null) {
            throw new Error(
                `HARNESS FAILURE: ${label} exited before HTTP was available\n${output()}`,
            );
        }
        try {
            const response = await fetch(url);
            if (response.ok) return;
            lastError = new Error(`HTTP ${response.status}`);
        } catch (error) {
            lastError = error;
        }
        await wait(100);
    }
    throw new Error(
        `HARNESS FAILURE: ${label} did not become ready: ${lastError?.message ?? ""}\n${output()}`,
    );
}

async function waitForPage(port) {
    let lastError = null;
    for (let attempt = 0; attempt < 100; attempt++) {
        if (browser?.exitCode !== null) {
            throw new Error(
                `HARNESS FAILURE: Chrome exited before CDP was ready\n${browserError}`,
            );
        }
        try {
            const response = await fetch(`http://127.0.0.1:${port}/json`);
            const targets = await response.json();
            const page = targets.find(
                (entry) => entry.type === "page" &&
                    entry.webSocketDebuggerUrl
            );
            if (page) return page;
        } catch (error) {
            lastError = error;
        }
        await wait(100);
    }
    throw new Error(
        `HARNESS FAILURE: Chrome DevTools did not become ready: ${lastError?.message ?? ""}\n${browserError}`,
    );
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
        if (message.error) {
            request.reject(new Error(message.error.message));
        } else {
            request.resolve(message.result);
        }
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
                throw new Error(
                    `APP FAILURE: browser evaluation failed: ${JSON.stringify(result.exceptionDetails)}`,
                );
            }
            return result.result.value;
        },
        close() {
            socket.close();
        },
    };
}

async function waitForCondition(check, label) {
    let lastError = null;
    for (let attempt = 0; attempt < 200; attempt++) {
        try {
            if (await check()) return;
        } catch (error) {
            lastError = error;
        }
        await wait(100);
    }
    throw new Error(
        `HARNESS FAILURE: timed out waiting for ${label}` +
        (lastError ? `: ${lastError.message}` : ""),
    );
}

function runCommand(command, args) {
    return new Promise((resolveCommand, reject) => {
        const child = spawn(command, args, {
            cwd: rootDir,
            stdio: ["ignore", "pipe", "pipe"],
            env: {
                ...process.env,
                GOWORK: "off",
            },
        });
        let output = "";
        child.stdout.on("data", (chunk) => {
            output += chunk;
            process.stdout.write(chunk);
        });
        child.stderr.on("data", (chunk) => {
            output += chunk;
            process.stderr.write(chunk);
        });
        child.on("error", reject);
        child.on("exit", (code, signal) => {
            commandOutput.push({
                command,
                args,
                status: signal ?? code,
                output: output.slice(-6000),
            });
            if (code === 0) {
                resolveCommand();
                return;
            }
            reject(
                new Error(
                    `${command} ${args.join(" ")} failed with ${signal ?? code}`,
                ),
            );
        });
    });
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
        const server = createServer();
        server.once("error", reject);
        server.listen(0, "127.0.0.1", () => {
            const address = server.address();
            server.close(() => resolvePort(address.port));
        });
    });
}

function assert(condition, message) {
    if (!condition) {
        throw new Error(`APP FAILURE: ${message}`);
    }
}

function wait(duration) {
    return new Promise((resolveWait) => setTimeout(resolveWait, duration));
}

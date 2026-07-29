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
const fixtureDir = join(
    rootDir,
    "scripts",
    "fixtures",
    "document-state-api-design",
);
const chrome = process.env.CHROME ?? "google-chrome";
const tempRoot = await mkdtemp(join(tmpdir(), "goframe-document-api-design-"));
const profile = await mkdtemp(join(tmpdir(), "goframe-document-api-design-chrome-"));
const appPort = Number(
    process.env.GOFRAME_DOCUMENT_API_DESIGN_PORT ?? await pickFreePort(),
);
const debugPort = Number(
    process.env.GOFRAME_DOCUMENT_API_DESIGN_CHROME_DEBUG_PORT ??
        await pickFreePort(),
);
const origin = `http://127.0.0.1:${appPort}`;
const modes = ["control", "hook", "component", "handle"];
const baseline = {
    title: "GoFrame document API design fixture",
    description: "Authored document API design baseline",
};
const routeA = {
    title: "Route A · GoFrame",
    description: "Route metadata A.",
};
const routeA2 = {
    title: "Route A2 · GoFrame",
    description: "Route metadata A2.",
};
const metadataB = {
    title: "Editor B · GoFrame",
    description: "Nested editor metadata B.",
};
const metadataC = {
    title: "Dialog C · GoFrame",
    description: "Nested dialog metadata C.",
};
const speculative = {
    title: "Speculative failure · GoFrame",
    description: "This pair must never commit.",
};

let browser = null;
let browserError = "";
let client = null;
let server = null;
const commandOutput = [];
const results = {};
const startedAt = Date.now();

try {
    const goxc = await prepareTools();
    await runCommand(goxc, ["package", fixtureDir, "--compiler=go"]);
    server = await startServer(
        goxc,
        ["serve", fixtureDir, `--port=${appPort}`],
        `${origin}/`,
        "document-state API design fixture",
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

    for (const mode of modes) {
        results[mode] = await runMode(client, mode);
    }

    const elapsedMilliseconds = Date.now() - startedAt;
    assert(
        elapsedMilliseconds <= 60_000,
        `focused comparison took ${elapsedMilliseconds}ms, limit is 60000ms`,
    );
    console.log(
        `document API design evidence: ${JSON.stringify({
            modes: results,
            elapsedMilliseconds,
        })}`,
    );
    console.log("document state API design browser smoke: ok");
} catch (error) {
    const diagnostics = await collectDiagnostics();
    throw new Error(`${error.message}\n${JSON.stringify(diagnostics, null, 2)}`, {
        cause: error,
    });
} finally {
    client?.close();
    await stopProcess(browser, false);
    await stopProcess(server?.child, true);
    await rm(profile, {
        recursive: true,
        force: true,
        maxRetries: 5,
        retryDelay: 100,
    });
    await rm(tempRoot, {
        recursive: true,
        force: true,
        maxRetries: 5,
        retryDelay: 100,
    });
    await rm(join(fixtureDir, ".goframe"), {
        recursive: true,
        force: true,
        maxRetries: 5,
        retryDelay: 100,
    });
}

async function runMode(browserClient, mode) {
    const appURL = `${origin}/?smoke=${Date.now()}#/${mode}`;
    await browserClient.call("Page.navigate", { url: appURL });
    await waitForCondition(async () => {
        const state = await pageState(browserClient);
        return state.app && state.mode === mode && state.baselineCaptured;
    }, `${mode} fixture boot`);
    await assertState(browserClient, baseline, {
        label: `${mode} initial baseline`,
        ownerCount: 0,
        scopeActive: true,
    });

    const initial = await pageState(browserClient);
    assert(initial.authoredTitle === baseline.title, `${mode} authored title changed`);
    assert(
        initial.authoredDescription === baseline.description,
        `${mode} authored description changed`,
    );
    assert(initial.authoredDescriptionCount === 1, `${mode} authored description count`);
    assert(initial.unrelated === "preserve-me", `${mode} unrelated metadata changed`);
    assert(initial.titleNodeSame, `${mode} title identity was not captured`);
    assert(initial.descriptionNodeSame, `${mode} description identity was not captured`);

    await setPhase(browserClient, "route-a");
    await click(browserClient, "activate-route-a");
    await waitForState(browserClient, routeA, {
        label: `${mode} route A`,
        ownerCount: 1,
    });
    const routeOwnerID = (await pageState(browserClient)).activeOwnerID;

    await setPhase(browserClient, "nested-b");
    await click(browserClient, "activate-b");
    await waitForState(browserClient, metadataB, {
        label: `${mode} nested B`,
        ownerCount: 2,
    });

    await setPhase(browserClient, "route-a2-beneath-b");
    await click(browserClient, "update-route-a2");
    await waitForCondition(async () => {
        const state = await pageState(browserClient);
        return pairMatches(state, metadataB) &&
            state.runtime.events.some((event) =>
                event.role === "route" &&
                event.change === "updated" &&
                event.title === metadataB.title
            );
    }, `${mode} route A2 retained beneath B`);
    await assertState(browserClient, metadataB, {
        label: `${mode} A2 beneath B`,
        ownerCount: 2,
    });

    await setPhase(browserClient, "release-selected-b");
    await click(browserClient, "release-b");
    await waitForState(browserClient, routeA2, {
        label: `${mode} release B reveals A2`,
        ownerCount: 1,
    });

    await setPhase(browserClient, "non-top-release");
    await click(browserClient, "activate-b");
    await waitForState(browserClient, metadataB, {
        label: `${mode} B reactivation`,
        ownerCount: 2,
    });
    await click(browserClient, "activate-c");
    await waitForState(browserClient, metadataC, {
        label: `${mode} C activation`,
        ownerCount: 3,
    });
    const beforeNonTop = await pageState(browserClient);
    await click(browserClient, "release-b");
    await waitForCondition(async () => {
        const state = await pageState(browserClient);
        return pairMatches(state, metadataC) && state.ownerCount === 2;
    }, `${mode} non-top B release`);
    const afterNonTop = await pageState(browserClient);
    assert(
        afterNonTop.titleMutationBatches === beforeNonTop.titleMutationBatches &&
            afterNonTop.descriptionMutationBatches ===
                beforeNonTop.descriptionMutationBatches,
        `${mode} non-top release caused a document write`,
    );

    await setPhase(browserClient, "same-value-rerender");
    const beforeSame = await pageState(browserClient);
    await click(browserClient, "same-value-update");
    await waitForCondition(async () => {
        const state = await pageState(browserClient);
        return state.runtime.renders > beforeSame.runtime.renders;
    }, `${mode} selected owner rerender`);
    const afterSame = await pageState(browserClient);
    assert(
        afterSame.runtime.transitions === beforeSame.runtime.transitions,
        `${mode} identical rerender reached the coordinator`,
    );
    assert(
        afterSame.titleMutationBatches === beforeSame.titleMutationBatches &&
            afterSame.descriptionMutationBatches ===
                beforeSame.descriptionMutationBatches,
        `${mode} identical rerender caused a document write`,
    );
    await assertState(browserClient, metadataC, {
        label: `${mode} identical update no-op`,
        ownerCount: 2,
    });

    const candidateChecks = await runCandidateProbe(browserClient, mode);

    await setPhase(browserClient, "speculative-failure");
    const beforeFailure = await pageState(browserClient);
    await click(browserClient, "activate-failure");
    await waitForCondition(async () => {
        const state = await pageState(browserClient);
        return state.failureFallback &&
            state.runtime.errorBoundaryCaptures ===
                beforeFailure.runtime.errorBoundaryCaptures + 1;
    }, `${mode} speculative fallback`);
    const failed = await pageState(browserClient);
    assert(pairMatches(failed, metadataC), `${mode} speculative owner changed selected pair`);
    assert(
        !failed.runtime.events.some((event) => event.role === "speculative"),
        `${mode} speculative owner reached the coordinator`,
    );
    assert(
        failed.speculativeAppearances === 0,
        `${mode} speculative metadata appeared in the document`,
    );
    await click(browserClient, "reset-failure");
    await waitForCondition(async () => {
        return !(await pageState(browserClient)).failureFallback;
    }, `${mode} speculative subtree reset`);

    await setPhase(browserClient, "scope-unmount");
    await click(browserClient, "unmount-scope");
    await waitForState(browserClient, baseline, {
        label: `${mode} authored baseline restoration`,
        ownerCount: 0,
        scopeActive: false,
    });
    const unmounted = await pageState(browserClient);
    assert(
        unmounted.runtime.baselineRestorations === 1,
        `${mode} baseline restorations = ${unmounted.runtime.baselineRestorations}`,
    );

    await setPhase(browserClient, "scope-remount");
    await click(browserClient, "remount-scope");
    await waitForState(browserClient, routeA2, {
        label: `${mode} scope remount`,
        ownerCount: 1,
        scopeActive: true,
    });
    const final = await pageState(browserClient);
    assert(
        final.activeOwnerID !== routeOwnerID,
        `${mode} remount reused owner lifetime ${routeOwnerID}`,
    );
    assert(final.runtime.scopeMounts === 2, `${mode} scope mounts = ${final.runtime.scopeMounts}`);
    assert(
        final.runtime.scopeUnmounts === 1,
        `${mode} scope unmounts = ${final.runtime.scopeUnmounts}`,
    );
    assert(final.invalidPairs === 0, `${mode} invalid pairs = ${final.invalidPairs}`);
    assert(
        final.duplicateDescriptions === 0,
        `${mode} duplicate descriptions = ${final.duplicateDescriptions}`,
    );
    assert(
        final.unrelatedMetaMutations === 0,
        `${mode} unrelated mutations = ${final.unrelatedMetaMutations}`,
    );
    assert(final.titleNodeReplacements === 0, `${mode} title node was replaced`);
    assert(
        final.descriptionNodeReplacements === 0,
        `${mode} description node was replaced`,
    );
    assert(final.titleNodeSame, `${mode} title identity changed`);
    assert(final.descriptionNodeSame, `${mode} description identity changed`);

    console.log(`${mode} document metadata ownership: ok`);
    return collectModeEvidence(final, candidateChecks);
}

async function runCandidateProbe(browserClient, mode) {
    if (mode === "control") {
        return {};
    }
    if (mode === "component") {
        const state = await pageState(browserClient);
        const nestedBLifetimes = state.runtime.componentLifetimes.filter(
            (event) => event.role === "nested-b",
        );
        assert(
            nestedBLifetimes.some((event) => event.action === "mount") &&
                nestedBLifetimes.some((event) => event.action === "unmount"),
            "component conditional ownership did not mount and unmount",
        );
        const directChild = await browserClient.evaluate(`(() => {
            const scope = document.querySelector(
                "[data-testid='candidate-scope']"
            );
            const route = document.querySelector(
                "[data-testid='route-owner-content']"
            );
            return Boolean(scope && route && route.parentElement === scope);
        })()`);
        assert(directChild, "component ownership added a wrapper element");
        return {
            conditionalMountRelease: true,
            wrapperElementsAdded: 0,
        };
    }

    await setPhase(browserClient, `${mode}-identity-probe`);
    const before = await pageState(browserClient);
    const elementCount = await browserClient.evaluate(
        "document.querySelector('[data-testid=\"candidate-scope\"]')?.querySelectorAll('*').length ?? -1",
    );
    await click(browserClient, "activate-probe");

    if (mode === "hook") {
        const expected = {
            title: "Hook probe two · GoFrame",
            description: "Hook slot two metadata.",
        };
        await waitForState(browserClient, expected, {
            label: "hook two-slot probe",
            ownerCount: 4,
        });
        const state = await pageState(browserClient);
        const events = state.runtime.events.filter((event) =>
            event.role === "hook-probe-one" ||
            event.role === "hook-probe-two"
        );
        assert(events.length === 2, `hook probe events = ${events.length}, want 2`);
        assert(
            events[0].ownerID !== events[1].ownerID,
            `hook slots shared owner ${events[0].ownerID}`,
        );
        const nextElementCount = await browserClient.evaluate(
            "document.querySelector('[data-testid=\"candidate-scope\"]')?.querySelectorAll('*').length ?? -1",
        );
        assert(
            nextElementCount === elementCount,
            `hook ownership added an element: ${elementCount} -> ${nextElementCount}`,
        );
        await click(browserClient, "release-probe");
        await waitForState(browserClient, metadataC, {
            label: `${mode} identity probe release`,
            ownerCount: 2,
        });
        return {
            hookSlotsDistinct: true,
            ownershipElementsAdded: 0,
        };
    }

    if (mode === "handle") {
        const expected = {
            title: "Handle probe · GoFrame",
            description: "One forwarded handle publication.",
        };
        await waitForState(browserClient, expected, {
            label: "handle forwarded duplicate probe",
            ownerCount: 3,
        });
        const state = await pageState(browserClient);
        assert(state.runtime.handleForwards > 0, "handle forwarding was not observed");
        assert(
            state.runtime.handleDuplicateCoalesces === 1,
            `handle duplicate coalesces = ${state.runtime.handleDuplicateCoalesces}`,
        );
        const probeAdds = state.runtime.events.filter((event) =>
            event.role === "handle-duplicate-probe" &&
            event.change === "added"
        );
        assert(probeAdds.length === 1, `handle probe owner additions = ${probeAdds.length}`);
        assert(state.runtime.handleForwardedOwnerIDs.every(
            (ownerID) => ownerID === probeAdds[0].ownerID,
        ), "helper forwarding changed handle identity");
        await click(browserClient, "release-probe");
        await waitForState(browserClient, metadataC, {
            label: `${mode} identity probe release`,
            ownerCount: 2,
        });
        const after = await pageState(browserClient);
        assert(after.runtime.adds > before.runtime.adds, `${mode} probe did not add an owner`);
        return {
            forwardedOwnerStable: true,
            ownerRecordsAdded: 1,
            duplicatePublicationsCoalesced:
                state.runtime.handleDuplicateCoalesces,
        };
    }
    throw new Error(`APP FAILURE: unsupported candidate probe mode ${mode}`);
}

function collectModeEvidence(state, candidateChecks) {
    return {
        runtime: {
            transitions: state.runtime.transitions,
            adds: state.runtime.adds,
            updates: state.runtime.updates,
            removes: state.runtime.removes,
            noops: state.runtime.noops,
            documentCommits: state.runtime.documentCommits,
            baselineRestorations: state.runtime.baselineRestorations,
            scopeMounts: state.runtime.scopeMounts,
            scopeUnmounts: state.runtime.scopeUnmounts,
            componentMounts: state.runtime.componentMounts,
            componentUnmounts: state.runtime.componentUnmounts,
            handleForwards: state.runtime.handleForwards,
            handleDuplicateCoalesces:
                state.runtime.handleDuplicateCoalesces,
            errorBoundaryCaptures: state.runtime.errorBoundaryCaptures,
            events: state.runtime.events.map((event) => ({
                candidate: event.candidate,
                role: event.role,
                change: event.change,
                ownerCount: event.ownerCount,
                title: event.title,
                description: event.description,
            })),
            runtimeErrors: state.runtime.runtimeErrors,
        },
        candidateChecks,
        observer: {
            titleMutationBatches: state.titleMutationBatches,
            descriptionMutationBatches: state.descriptionMutationBatches,
            headSnapshots: state.headSnapshots,
            invalidPairs: state.invalidPairs,
            duplicateDescriptions: state.duplicateDescriptions,
            speculativeAppearances: state.speculativeAppearances,
            unrelatedMetaMutations: state.unrelatedMetaMutations,
            titleNodeReplacements: state.titleNodeReplacements,
            descriptionNodeReplacements: state.descriptionNodeReplacements,
            snapshots: state.snapshots,
        },
    };
}

async function waitForState(browserClient, metadata, options) {
    await waitForCondition(async () => {
        const state = await pageState(browserClient);
        return pairMatches(state, metadata) &&
            state.ownerCount === options.ownerCount &&
            (options.scopeActive === undefined ||
                state.scopeActive === options.scopeActive);
    }, options.label);
    await assertState(browserClient, metadata, options);
}

async function assertState(browserClient, metadata, options) {
    const state = await pageState(browserClient);
    const mismatches = [];
    if (!pairMatches(state, metadata)) {
        mismatches.push(
            `actual=${JSON.stringify({
                title: state.title,
                description: state.description,
                expectedTitle: state.expectedTitle,
                expectedDescription: state.expectedDescription,
            })}`,
        );
    }
    if (state.ownerCount !== options.ownerCount) {
        mismatches.push(`ownerCount=${state.ownerCount}, want ${options.ownerCount}`);
    }
    if (
        options.scopeActive !== undefined &&
        state.scopeActive !== options.scopeActive
    ) {
        mismatches.push(`scopeActive=${state.scopeActive}, want ${options.scopeActive}`);
    }
    if (state.descriptionCount !== 1) {
        mismatches.push(`descriptionCount=${state.descriptionCount}, want 1`);
    }
    if (state.unrelated !== "preserve-me") {
        mismatches.push(`unrelated=${JSON.stringify(state.unrelated)}`);
    }
    if (!state.titleNodeSame || !state.descriptionNodeSame) {
        mismatches.push(
            `node identity title=${state.titleNodeSame} description=${state.descriptionNodeSame}`,
        );
    }
    if (mismatches.length > 0) {
        throw new Error(`APP FAILURE: ${options.label}: ${mismatches.join("; ")}`);
    }
    console.log(`${options.label}: ok`);
}

function pairMatches(state, metadata) {
    return state.title === metadata.title &&
        state.description === metadata.description &&
        state.expectedTitle === metadata.title &&
        state.expectedDescription === metadata.description;
}

async function pageState(browserClient) {
    return await browserClient.evaluate(`(() => {
        const text = (testID) =>
            document.querySelector("[data-testid='" + testID + "']")
                ?.textContent.trim() ?? "";
        const head = globalThis.__documentAPIDesignHeadEvidence || {};
        const runtime = globalThis.goframeDocumentAPIDesignEvidence || {};
        const descriptions = document.querySelectorAll(
            'head meta[name="description"]'
        );
        const titleNode = document.querySelector("head title");
        const descriptionNode = descriptions[0] || null;
        return {
            href: location.href,
            mode: text("design-mode").replace(/^Mode:\\s*/, ""),
            app: Boolean(document.querySelector("[data-testid='design-app']")),
            scopeActive: Boolean(document.querySelector(
                "[data-testid='candidate-scope']"
            )),
            scopeInactive: Boolean(document.querySelector(
                "[data-testid='candidate-scope-inactive']"
            )),
            failureFallback: Boolean(document.querySelector(
                "[data-testid='failure-fallback']"
            )),
            activeOwnerID: text("active-owner-id"),
            ownerCount: Number(text("active-owner-count") || 0),
            expectedTitle: text("expected-title"),
            expectedDescription: text("expected-description"),
            title: document.title,
            description: descriptionNode?.getAttribute("content") ?? "",
            descriptionCount: descriptions.length,
            unrelated: document.querySelector(
                'head meta[name="fixture-unrelated"]'
            )?.getAttribute("content") ?? "",
            baselineCaptured: Boolean(head.baselineCaptured),
            authoredTitle: head.authoredTitle ?? "",
            authoredDescription: head.authoredDescription ?? "",
            authoredDescriptionCount: head.authoredDescriptionCount ?? 0,
            titleNodeSame: Boolean(
                head.titleNode && titleNode === head.titleNode
            ),
            descriptionNodeSame: Boolean(
                head.descriptionNode &&
                descriptionNode === head.descriptionNode
            ),
            titleMutationBatches: head.titleMutationBatches ?? 0,
            descriptionMutationBatches: head.descriptionMutationBatches ?? 0,
            headSnapshots: head.headSnapshots ?? 0,
            invalidPairs: head.invalidPairs ?? 0,
            duplicateDescriptions: head.duplicateDescriptions ?? 0,
            speculativeAppearances: head.speculativeAppearances ?? 0,
            unrelatedMetaMutations: head.unrelatedMetaMutations ?? 0,
            titleNodeReplacements: head.titleNodeReplacements ?? 0,
            descriptionNodeReplacements:
                head.descriptionNodeReplacements ?? 0,
            snapshots: head.snapshots ?? [],
            runtime: {
                mode: runtime.mode ?? "",
                transitions: runtime.transitions ?? 0,
                adds: runtime.adds ?? 0,
                updates: runtime.updates ?? 0,
                removes: runtime.removes ?? 0,
                noops: runtime.noops ?? 0,
                renders: runtime.renders ?? 0,
                documentCommits: runtime.documentCommits ?? 0,
                baselineRestorations: runtime.baselineRestorations ?? 0,
                scopeMounts: runtime.scopeMounts ?? 0,
                scopeUnmounts: runtime.scopeUnmounts ?? 0,
                componentMounts: runtime.componentMounts ?? 0,
                componentUnmounts: runtime.componentUnmounts ?? 0,
                handleForwards: runtime.handleForwards ?? 0,
                handleDuplicateCoalesces:
                    runtime.handleDuplicateCoalesces ?? 0,
                errorBoundaryCaptures:
                    runtime.errorBoundaryCaptures ?? 0,
                events: runtime.events ?? [],
                ownerRenders: runtime.ownerRenders ?? [],
                componentLifetimes: runtime.componentLifetimes ?? [],
                handleForwardedOwnerIDs:
                    runtime.handleForwardedOwnerIDs ?? [],
                handleDuplicateOwnerIDs:
                    runtime.handleDuplicateOwnerIDs ?? [],
                runtimeErrors: runtime.runtimeErrors ?? [],
            },
        };
    })()`);
}

async function setPhase(browserClient, phase) {
    await browserClient.evaluate(`(() => {
        const evidence = globalThis.__documentAPIDesignHeadEvidence;
        if (!evidence) return false;
        evidence.phase = ${JSON.stringify(phase)};
        return true;
    })()`);
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

function installHeadObserver() {
    const evidence = {
        baselineCaptured: false,
        authoredTitle: "",
        authoredDescription: "",
        authoredDescriptionCount: 0,
        titleNode: null,
        descriptionNode: null,
        unrelatedNode: null,
        titleMutationBatches: 0,
        descriptionMutationBatches: 0,
        headSnapshots: 0,
        invalidPairs: 0,
        duplicateDescriptions: 0,
        speculativeAppearances: 0,
        unrelatedMetaMutations: 0,
        titleNodeReplacements: 0,
        descriptionNodeReplacements: 0,
        phase: "document-parse",
        snapshots: [],
    };
    globalThis.__documentAPIDesignHeadEvidence = evidence;

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

    const observer = new MutationObserver((records) => {
        const capturedBeforeBatch = evidence.baselineCaptured;
        if (!captureBaseline() || !capturedBeforeBatch) return;

        const titleTouched = records.some((record) =>
            containsNode(record, evidence.titleNode)
        );
        const descriptionTouched = records.some((record) =>
            containsNode(record, evidence.descriptionNode)
        );
        if (titleTouched) evidence.titleMutationBatches++;
        if (descriptionTouched) evidence.descriptionMutationBatches++;
        if (
            records.some((record) =>
                containsNode(record, evidence.unrelatedNode)
            )
        ) {
            evidence.unrelatedMetaMutations++;
        }
        for (const record of records) {
            const changed = [...record.addedNodes, ...record.removedNodes];
            if (changed.includes(evidence.titleNode)) {
                evidence.titleNodeReplacements++;
            }
            if (changed.includes(evidence.descriptionNode)) {
                evidence.descriptionNodeReplacements++;
            }
        }
        if (!titleTouched && !descriptionTouched) return;

        const descriptions = document.querySelectorAll(
            'head meta[name="description"]'
        );
        const snapshot = {
            phase: evidence.phase,
            title: document.title,
            description:
                descriptions[0]?.getAttribute("content") ?? "",
            descriptionCount: descriptions.length,
        };
        evidence.snapshots.push(snapshot);
        evidence.headSnapshots++;
        if (snapshot.descriptionCount !== 1) {
            evidence.duplicateDescriptions++;
        }
        const expectedTitle = document.querySelector(
            "[data-testid='expected-title']"
        )?.textContent.trim() ?? "";
        const expectedDescription = document.querySelector(
            "[data-testid='expected-description']"
        )?.textContent.trim() ?? "";
        if (
            expectedTitle &&
            (snapshot.title !== expectedTitle ||
                snapshot.description !== expectedDescription)
        ) {
            evidence.invalidPairs++;
        }
        if (
            snapshot.title === "Speculative failure · GoFrame" ||
            snapshot.description === "This pair must never commit."
        ) {
            evidence.speculativeAppearances++;
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

async function prepareTools() {
    if (process.env.GOXC) {
        return process.env.GOXC;
    }
    const goxc = join(tempRoot, "goxc");
    await runCommand("go", ["build", "-o", goxc, "./cmd/goxc"]);
    return goxc;
}

async function collectDiagnostics() {
    const diagnostics = {
        appPort,
        debugPort,
        browserStderr: browserError.slice(-6000),
        serverOutput: server?.output?.().slice(-6000) ?? "",
        commandOutput: commandOutput.slice(-8),
    };
    if (client) {
        try {
            diagnostics.page = await pageState(client);
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
        `HARNESS FAILURE: ${label} did not become ready: ` +
        `${lastError?.message ?? ""}\n${output()}`,
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
        `HARNESS FAILURE: Chrome DevTools did not become ready: ` +
        `${lastError?.message ?? ""}\n${browserError}`,
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
                    "APP FAILURE: browser evaluation failed: " +
                    JSON.stringify(result.exceptionDetails),
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

function wait(milliseconds) {
    return new Promise((resolveWait) => {
        setTimeout(resolveWait, milliseconds);
    });
}

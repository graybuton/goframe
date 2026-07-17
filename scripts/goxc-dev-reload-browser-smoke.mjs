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
const lines = [];
const counters = {
    successfulPackageAttempts: 0,
    failedPackageAttempts: 0,
    completedGenerations: 0,
    generationActivations: 0,
    reloadEventsPublished: 0,
    catchUpReloads: 0,
    previousProcessCatchUpReloads: 0,
    requestsServedByOldGeneration: 0,
    responses404DuringBuild: 0,
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
    const initial = await waitForPageState(client1, { gox: "gox-initial", go: "go-initial", index: "index-initial" });
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
    const publicationProbe = probeOldGenerationUntilBuildCompletes(serverURL, wasmPath, activeGeneration, nextBuild);
    await waitForBuild(nextBuild, "succeeded", goxStart);
    const publicationEvidence = await publicationProbe;
    counters.requestsServedByOldGeneration += publicationEvidence.oldGenerationResponses;
    counters.responses404DuringBuild += publicationEvidence.responses404;
    assert(publicationEvidence.oldGenerationResponses > 0, "no old-generation response was observed during the later build");
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
        await wait(20);
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
    await writeFile(join(appDir, "app.gox"), `package main\n\nfunc App() { return <main>broken</main> }\n`);
    await waitForBuild(nextBuild, "failed", failureStart);
    counters.failedPackageAttempts++;
    nextBuild++;
    const failureRoot = await fetch(`${serverURL}/`, { cache: "no-store" });
    const failureMetadata = await fetch(`${serverURL}/goframe-package.json`, { cache: "no-store" });
    assert(failureRoot.ok && failureMetadata.ok, `last successful package unavailable after failure: ${failureRoot.status}/${failureMetadata.status}`);
    await wait(250);
    const afterFailure = await pageState(client1);
    assert(afterFailure.pageLoads === failureBaseline.pageLoads && afterFailure.reloadEvents === failureBaseline.reloadEvents,
        `failed build reloaded the browser: ${JSON.stringify({ failureBaseline, afterFailure })}`);
    assert(afterFailure.generation === activeGeneration, `failed build changed generation to ${afterFailure.generation}`);
    console.log("failed build preservation: ok");

    const recoveryBaseline = afterFailure;
    await writeGOX("recovered");
    await waitForBuild(nextBuild, "succeeded");
    await assertSuccessfulReload(client1, recoveryBaseline, { gox: "recovered", go: "go-rebuild", index: "index-rebuild" }, ++activeGeneration);
    recordSuccessfulReload();
    nextBuild++;
    console.log("failed build recovery reload: ok");

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
    const generationRoots = [...await listGenerationRoots()].filter((entry) => !generationRootsBefore.has(entry));
    assert(generationRoots.length === 1, `development generation roots = ${JSON.stringify(generationRoots)}, want one`);
    dev.kill("SIGINT");
    const devExit = await waitForExit(dev, 15_000);
    assert(devExit.code === 0, `goxc dev exited with ${JSON.stringify(devExit)}\n${devOutput}`);
    dev = null;
    await waitForHTTPShutdown(serverURL);
    await waitForNoEventStreams(client1);
    await waitForNoEventStreams(client2);
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
    console.log("connected subscribers at shutdown: 0");
    console.log(`requests served by old generation during later build: ${counters.requestsServedByOldGeneration}`);
    console.log(`404 responses during later build: ${counters.responses404DuringBuild}`);
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
    await writeFile(join(appDir, "main.go"), `//go:build js && wasm\n\npackage main\n\nimport gf "github.com/graybuton/goframe/pkg/goframe"\n\nfunc main() {\n    done := make(chan struct{})\n    gf.Mount("root", App)\n    <-done\n}\n`);
    await writeGoMessage(go);
    await writeGOX(gox);
    await writeIndex(index);
    await writeFile(join(appDir, "assets", "marker.txt"), "complete asset\n");
}

async function writeGOX(value) {
    await writeFile(join(appDir, "app.gox"), `package main\n\nimport gf "github.com/graybuton/goframe/pkg/goframe"\n\nfunc App() gf.Node {\n    return <main id="app-version"><span id="gox-version">${value}</span><span id="go-version">{message()}</span></main>\n}\n`);
}

async function writeGoMessage(value) {
    await writeFile(join(appDir, "message.go"), `package main\n\nfunc message() string { return ${JSON.stringify(value)} }\n`);
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

async function prepareClient(client) {
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
        const increment = (key) => {
            const next = Number(sessionStorage.getItem(key) || "0") + 1;
            sessionStorage.setItem(key, String(next));
        };
        function WrappedEventSource(url, options) {
            const source = options === undefined ? new NativeEventSource(url) : new NativeEventSource(url, options);
            window.__goframeDevEventSources.push(source);
            source.addEventListener("open", () => increment("goframe-dev-event-source-opens"));
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
        return {
            href: location.href,
            readyState: document.readyState,
            gox: document.querySelector("#gox-version")?.textContent || "",
            go: document.querySelector("#go-version")?.textContent || "",
            index: document.querySelector("#index-version")?.textContent || "",
            pageLoads: Number(sessionStorage.getItem("goframe-dev-page-loads") || "0"),
            reloadEvents: Number(sessionStorage.getItem("goframe-dev-reload-events") || "0"),
            eventSourceOpens: Number(sessionStorage.getItem("goframe-dev-event-source-opens") || "0"),
            generation: Number(script?.getAttribute("data-goframe-generation") || "0"),
            instance: script?.getAttribute("data-goframe-instance") || "",
            reloadTags: document.querySelectorAll("script[data-goframe-dev-reload]").length,
        };
    })()`);
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
}

async function reconnectReloadClient(client, instance, generation) {
    await client.evaluate(`(() => {
        for (const source of window.__goframeDevEventSources || []) source.close();
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

async function probeOldGenerationUntilBuildCompletes(serverURL, wasmPath, generation, build) {
    let oldGenerationResponses = 0;
    let responses404 = 0;
    assert(!hasBuildResult(build, "succeeded"), `build ${build} completed before the publication probe started`);
    const [indexResponse, metadataResponse, wasmResponse] = await Promise.all([
        fetch(`${serverURL}/?publication=0`, { cache: "no-store" }),
        fetch(`${serverURL}/goframe-package.json?publication=0`, { cache: "no-store" }),
        fetch(`${serverURL}/${wasmPath}?publication=0`, { cache: "no-store" }),
    ]);
    for (const response of [indexResponse, metadataResponse, wasmResponse]) {
        if (response.status === 404) responses404++;
        assert(response.ok, `HTTP ${response.status} during later build for ${response.url}`);
    }
    const index = await indexResponse.text();
    const metadata = await metadataResponse.json();
    const wasm = await wasmResponse.arrayBuffer();
    const servedGeneration = Number(index.match(/data-goframe-generation="(\d+)"/)?.[1] || "0");
    assert(metadata.version === 1 && wasm.byteLength > 0, "partial package response observed during later build");
    if (servedGeneration === generation) oldGenerationResponses += 3;
    return { oldGenerationResponses, responses404 };
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
    for (let attempt = 0; attempt < 100; attempt++) {
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

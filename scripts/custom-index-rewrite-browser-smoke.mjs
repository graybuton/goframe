import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdtemp, readFile, readdir, rm } from "node:fs/promises";
import { createServer as createPortServer } from "node:net";
import { tmpdir } from "node:os";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const rootDir = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const fixtureRoot = join(rootDir, "scripts", "fixtures", "custom-index-rewrite");
const compiler = process.env.GOFRAME_CUSTOM_INDEX_COMPILER ?? "go";
const goxc = process.env.GOXC ?? "goxc";
const chrome = process.env.CHROME ?? "google-chrome";
const goToolchain = process.env.GOTOOLCHAIN ?? "go1.26.6";
const debugPort = Number(
    process.env.GOFRAME_CUSTOM_INDEX_CHROME_DEBUG_PORT ?? await pickFreePort(),
);

if (compiler !== "go" && compiler !== "tinygo") {
    throw new Error(`HARNESS FAILURE: unsupported compiler ${JSON.stringify(compiler)}`);
}

const authoredSentinels = [
    "<!-- authored sentinel: bundle.wasm wasm_exec.js styles.css -->",
    '<script type="application/json" id="fixture-json">{"asset":"bundle.wasm","runtime":"wasm_exec.js"}</script>',
    '<style id="fixture-style">.sentinel::after { content: "bundle.wasm"; }</style>',
    '<p data-testid="authored-text">bundle.wasm is documented here.</p>',
    '<a data-testid="authored-link" href="styles.css">Stylesheet documentation</a>',
    'data-example="bundle.wasm"',
    'const documentation = "bundle.wasm";',
    'const runtimeExample = "wasm_exec.js";',
    'const similarNames = "my-bundle.wasm bundle.wasm.backup bundle.wasm.map wasm_exec.js.map custom-wasm_exec.js";',
];

const legacyJavaScriptSentinels = [
    "const quotient = 10 / 2;",
    "let counter = 0;",
    "const postfixed = counter++ / 2;",
    'const legacyFetchPattern = /fetch\\("bundle\\.wasm"\\)/;',
    'const nestedTemplate = `outer ${`fetch("bundle.wasm")`}`;',
    'if (true) {}\n        /fetch("bundle.wasm")/.test("value");',
    'class AuthoredSyntax {\n            #value = "fetch(\\"bundle.wasm\\")";\n        }',
];

const legacyHTMLSentinels = [
    '<script id="fixture-double-escaped" type="application/json"><!--<script></script><script src="wasm_exec.js?fixture=double-escaped"></script>--></script>',
    `<svg id="fixture-svg-cdata" style="display:none">
<![CDATA[
<script src="wasm_exec.js"></script>
<link rel="stylesheet" href="styles.css">
</head>
<!-- goframe:runtime -->
]]>
    </svg>`,
    `<math id="fixture-math-cdata" style="display:none">
<![CDATA[
<script src="wasm_exec.js"></script>
<link rel="stylesheet" href="styles.css">
</head>
<!-- goframe:runtime -->
]]>
    </math>`,
    '<script id="fixture-nbsp-script" type=" application/javascript" src="wasm_exec.js"></script>',
    '<link id="fixture-nbsp-rel" rel="alternate stylesheet" href="styles.css">',
    '<link id="fixture-nbsp-as" rel="preload" as=" style" href="styles.css">',
    '<svg id="fixture-breakout" style="display:none">',
    '<p id="fixture-breakout-html"></p>',
    '<input id="fixture-compact-input" disabled/>',
    'type="text/javascript1.5"',
];

let tempRoot = null;
let profile = null;
let browser = null;
let browserError = "";
let client = null;
let server = null;
let serverError = "";
const cdpRuntimeErrors = [];
const cdpUnexpectedHTTPFailures = [];
const cdpDecoyRequests = [];
const cdpPackageAssetRequests = [];

try {
    tempRoot = await mkdtemp(join(tmpdir(), `goframe-custom-index-${compiler}-`));
    profile = await mkdtemp(join(tmpdir(), "goframe-custom-index-chrome-"));
    const scenarios = [];
    for (const mode of ["marker", "legacy"]) {
        scenarios.push(await prepareScenario(mode));
    }

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
    client.on("Network.responseReceived", (params) => {
        const response = params.response;
        const pathname = new URL(response.url).pathname;
        if (pathname === "/wasm_exec.js" || pathname === "/styles.css") {
            cdpDecoyRequests.push({ url: response.url, status: response.status });
        }
        if (pathname.startsWith("/assets/")) {
            cdpPackageAssetRequests.push({ pathname, status: response.status });
        }
        if (response.status >= 400 && pathname !== "/favicon.ico") {
            cdpUnexpectedHTTPFailures.push({ url: response.url, status: response.status });
        }
    });
    await client.call("Runtime.enable");
    await client.call("Page.enable");
    await client.call("Network.enable");

    for (const scenario of scenarios) {
        scenario.browser = await runBrowserScenario(scenario);
    }

    const stableScenarios = scenarios.map((scenario) => ({
        mode: scenario.mode,
        indexBytes: scenario.indexBytes,
        indexSha256: scenario.indexSha256,
        inspectContractSha256: scenario.inspectContractSha256,
        artifactCount: scenario.artifactCount,
        paths: scenario.paths,
        browser: scenario.browser,
    }));
    const report = {
        compiler,
        behaviorSha256: sha256(JSON.stringify(stableScenarios)),
        scenarios: stableScenarios,
    };
    console.log(`custom index rewrite evidence: ${JSON.stringify(report)}`);
    console.log(`custom index rewrite browser smoke (${compiler}): ok`);
} catch (error) {
    throw new Error(`${error.message}\n${JSON.stringify(await diagnostics(), null, 2)}`, {
        cause: error,
    });
} finally {
    client?.close();
    await stopProcess(server);
    await stopProcess(browser);
    if (profile) {
        await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
    }
    if (tempRoot) {
        await rm(tempRoot, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
    }
}

async function prepareScenario(mode) {
    const fixture = join(fixtureRoot, mode);
    const workspace = join(tempRoot, "workspaces", mode);
    const packageOutput = await runCommand(goxc, [
        "package",
        fixture,
        `--compiler=${compiler}`,
        `--workspace=${workspace}`,
        "--asset-hash",
        "--preload",
        "--compress=gzip,br",
    ], commandEnvironment());
    const packageDirectory = packagedDirectory(packageOutput);
    const sourceHTML = await readFile(join(fixture, "index.html"), "utf8");
    const packagedHTML = await readFile(join(packageDirectory, "index.html"), "utf8");
    const assetManifest = JSON.parse(
        await readFile(join(packageDirectory, "asset-manifest.json"), "utf8"),
    );
    const packageMetadata = JSON.parse(
        await readFile(join(packageDirectory, "goframe-package.json"), "utf8"),
    );
    assertAuthoredSentinels(sourceHTML, packagedHTML, mode);
    assertPackageContract(mode, packagedHTML, assetManifest, packageMetadata);

    const inspectJSON = await runCommand(goxc, [
        "inspect",
        `--dir=${packageDirectory}`,
        "--format=json",
    ], commandEnvironment());
    const inspectReport = JSON.parse(inspectJSON);

    const exportDirectory = join(tempRoot, "exports", mode);
    await runCommand(goxc, [
        "export",
        fixture,
        `--workspace=${workspace}`,
        `--out=${exportDirectory}`,
    ], commandEnvironment());
    const exportedInspectJSON = await runCommand(goxc, [
        "inspect",
        `--dir=${exportDirectory}`,
        "--format=json",
    ], commandEnvironment());
    assert(
        exportedInspectJSON === inspectJSON,
        `${mode} exported inspect report differs from its package report`,
    );
    const packageGraph = await snapshotTree(packageDirectory);
    const exportGraph = await snapshotTree(exportDirectory);
    assertDeepEqual(exportGraph, packageGraph, `${mode} exported package graph`);

    return {
        mode,
        fixture,
        workspace,
        packageDirectory,
        indexBytes: Buffer.byteLength(packagedHTML),
        indexSha256: sha256(packagedHTML),
        inspectContractSha256: stableInspectContractHash(inspectReport),
        artifactCount: inspectReport.artifacts.length,
        paths: {
            wasm: assetManifest.entrypoints.wasm,
            runtime: assetManifest.entrypoints.runtime,
            style: assetManifest.entrypoints.styles[0],
        },
    };
}

function assertAuthoredSentinels(source, packaged, mode) {
    for (const sentinel of authoredSentinels) {
        assert(source.includes(sentinel), `${mode} source is missing sentinel ${JSON.stringify(sentinel)}`);
        assert(packaged.includes(sentinel), `${mode} package changed sentinel ${JSON.stringify(sentinel)}`);
    }
    if (mode === "legacy") {
        assert(source.includes("./wasm&lowbar;exec.js?fixture=legacy#runtime"), "legacy source is missing the named runtime reference");
        assert(source.includes("./styles&period;css?fixture=legacy#theme"), "legacy source is missing the named stylesheet reference");
        assert(!packaged.includes("wasm&lowbar;exec.js?fixture=legacy#runtime"), "legacy package retained the named runtime reference");
        assert(!packaged.includes("styles&period;css?fixture=legacy#theme"), "legacy package retained the named stylesheet reference");
        const falseHead = '<script>const falseHeadExample = "</head>";</script>';
        assert(source.includes(falseHead) && packaged.includes(falseHead), "legacy false head sentinel changed");
        for (const sentinel of legacyJavaScriptSentinels) {
            assert(source.includes(sentinel), `legacy source is missing JavaScript sentinel ${JSON.stringify(sentinel)}`);
            assert(packaged.includes(sentinel), `legacy package changed JavaScript sentinel ${JSON.stringify(sentinel)}`);
        }
        for (const sentinel of legacyHTMLSentinels) {
            assert(source.includes(sentinel), `legacy source is missing HTML sentinel ${JSON.stringify(sentinel)}`);
            assert(packaged.includes(sentinel), `legacy package changed HTML sentinel ${JSON.stringify(sentinel)}`);
        }
    }
}

function assertPackageContract(mode, html, manifest, metadata) {
    assert(metadata.hashAssets === true, `${mode} package did not enable asset hashing`);
    assert(metadata.preload === true, `${mode} package did not enable preload`);
    const wasm = manifest.entrypoints.wasm;
    const runtime = manifest.entrypoints.runtime;
    const styles = manifest.entrypoints.styles;
    assert(Array.isArray(styles) && styles.length === 1, `${mode} package styles = ${JSON.stringify(styles)}`);
    const style = styles[0];
    for (const [logicalName, path] of [["bundle.wasm", wasm], ["wasm_exec.js", runtime], ["styles.css", style]]) {
        assert(/^assets\/.+\.[0-9a-f]{8}\.[^.]+$/.test(path), `${mode} ${logicalName} path is not hashed: ${path}`);
        const compressed = manifest.assets[logicalName]?.compressed;
        assert(compressed?.gzip === `${path}.gz`, `${mode} ${logicalName} gzip sidecar is missing`);
        assert(compressed?.br === `${path}.br`, `${mode} ${logicalName} Brotli sidecar is missing`);
    }
    for (const path of [wasm, runtime, style]) {
        assert(html.includes(`<link rel="preload" href="${path}"`), `${mode} preload is missing ${path}`);
    }
    if (mode === "marker") {
        for (const name of ["preload", "runtime", "bootstrap"]) {
            assert(html.includes(`<!-- goframe:${name} -->`), `marker package lost ${name} start marker`);
            assert(html.includes(`<!-- /goframe:${name} -->`), `marker package lost ${name} end marker`);
        }
        assert(html.includes(`<script src="${runtime}"></script>`), "marker runtime target is stale");
        assert(html.includes(`fetch("${wasm}")`), "marker WASM target is stale");
        assert(html.includes(`href="${style}?fixture=marker#theme"`), "marker stylesheet target is stale");
        assert(!html.includes("authored preload interior"), "marker preload interior was not replaced");
    } else {
        assert(html.includes(`SRC=' ${runtime}?fixture=legacy#runtime '`), "legacy runtime target is stale");
        assert(html.includes(`fetch ( ' ${wasm}?fixture=legacy#wasm ' )`), "legacy WASM target is stale");
        assert(html.includes(`href=' ${style}?fixture=legacy#theme '`), "legacy stylesheet target is stale");
        assert(html.includes('type="text/javascript1.5"'), "legacy runtime MIME type changed");
        assert(html.includes('<input id="fixture-compact-input" disabled/>'), "compact boolean tag changed");
        const preload = html.indexOf(`<link rel="preload" href="${wasm}"`);
        const structuralHead = html.lastIndexOf("</head>");
        const falseHead = html.indexOf('const falseHeadExample = "</head>";');
        assert(falseHead >= 0 && preload > falseHead && preload < structuralHead, "legacy preload was not structurally inserted");
    }
}

async function runBrowserScenario(scenario) {
    const port = await pickFreePort();
    serverError = "";
    server = spawn(goxc, [
        "serve",
        scenario.fixture,
        `--workspace=${scenario.workspace}`,
        `--port=${port}`,
    ], {
        cwd: rootDir,
        env: commandEnvironment(),
        stdio: ["ignore", "ignore", "pipe"],
    });
    server.stderr.on("data", (chunk) => {
        serverError += chunk;
    });
    server.on("error", (error) => {
        serverError += error.message;
    });
    await waitForServer(port, server);

    cdpRuntimeErrors.length = 0;
    cdpUnexpectedHTTPFailures.length = 0;
    cdpDecoyRequests.length = 0;
    cdpPackageAssetRequests.length = 0;
    const url = `http://127.0.0.1:${port}/?mode=${scenario.mode}&compiler=${compiler}&run=${Date.now()}`;
    await client.call("Page.navigate", { url });
    await waitForFixture(url);
    const before = await pageState();
    assert(before.count === "0", `${scenario.mode} initial count = ${JSON.stringify(before.count)}`);
    assert(before.appColor === "rgb(13, 71, 161)", `${scenario.mode} stylesheet color = ${before.appColor}`);
    assert(before.buttonBackground === "rgb(255, 235, 59)", `${scenario.mode} button background = ${before.buttonBackground}`);
    assert(before.authoredText === "bundle.wasm is documented here.", `${scenario.mode} authored text changed`);
    assert(before.authoredHref === "styles.css", `${scenario.mode} authored link changed to ${before.authoredHref}`);
    assert(before.bodyExample === "bundle.wasm", `${scenario.mode} data attribute changed`);
    assert(before.inlineJSON === '{"asset":"bundle.wasm","runtime":"wasm_exec.js"}', `${scenario.mode} inline JSON changed`);
    assertDeepEqual(before.runtimeErrors, [], `${scenario.mode} GoFrame runtime errors before interaction`);
    if (scenario.mode === "legacy") {
        for (const [name, value] of [["SVG", before.svgCDATA], ["MathML", before.mathCDATA]]) {
            assert(value.includes('<script src="wasm_exec.js"></script>'), `${name} CDATA runtime decoy changed`);
            assert(value.includes('<link rel="stylesheet" href="styles.css">'), `${name} CDATA style decoy changed`);
            assert(value.includes("</head>"), `${name} CDATA head decoy changed`);
            assert(value.includes("<!-- goframe:runtime -->"), `${name} CDATA marker decoy changed`);
        }
        assert(before.nbspScriptType === " application/javascript", "legacy NBSP script type changed");
        assert(before.nbspScriptSource === "wasm_exec.js", "legacy NBSP script source changed");
        assert(before.nbspRel === "alternate stylesheet", "legacy NBSP rel changed");
        assert(before.nbspRelHref === "styles.css", "legacy NBSP rel href changed");
        assert(before.nbspAs === " style", "legacy NBSP as changed");
        assert(before.nbspAsHref === "styles.css", "legacy NBSP as href changed");
        assert(before.doubleEscapedText.includes('<script src="wasm_exec.js?fixture=double-escaped"></script>'), "legacy double-escaped runtime decoy changed");
        assert(before.breakoutSVGNamespace === "http://www.w3.org/2000/svg", "legacy SVG namespace changed");
        assert(before.breakoutHTMLNamespace === "http://www.w3.org/1999/xhtml", "foreign breakout did not create an HTML element");
        assert(before.breakoutRuntimeNamespace === "http://www.w3.org/1999/xhtml", "runtime script did not enter the HTML namespace");
        assert(before.compactInputNamespace === "http://www.w3.org/1999/xhtml", "compact input did not enter the HTML namespace");
        assert(before.compactInputDisabled === true, "compact boolean attribute was not accepted");
        assert(before.breakoutInsideSVG === false, "foreign breakout element remained under the SVG node");
        assert(before.runtimeInsideSVG === false, "runtime script remained under the SVG node");
    }

    const requestedAssets = [...new Set(cdpPackageAssetRequests
        .filter((request) => request.status === 200)
        .map((request) => request.pathname))].sort();
    for (const path of Object.values(scenario.paths)) {
        assert(requestedAssets.includes(`/${path}`), `${scenario.mode} browser did not request ${path}`);
    }

    const assetStatuses = await client.evaluate(`Promise.all(${JSON.stringify(Object.values(scenario.paths))}.map(async (path) => {
        const response = await fetch("/" + path, { cache: "no-store" });
        return [path, response.status, response.headers.get("content-type")];
    }))`);
    for (const [path, status] of assetStatuses) {
        assert(status === 200, `${scenario.mode} final asset ${path} returned ${status}`);
    }

    await client.evaluate(`document.querySelector("[data-testid='custom-index-increment']").click()`);
    for (let attempt = 0; attempt < 100; attempt++) {
        if ((await pageState()).count === "1") break;
        await wait(20);
    }
    const after = await pageState();
    assert(after.count === "1", `${scenario.mode} interaction did not update the current application`);
    assertDeepEqual(after.runtimeErrors, [], `${scenario.mode} GoFrame runtime errors after interaction`);
    assert(cdpRuntimeErrors.length === 0, `${scenario.mode} CDP runtime errors: ${JSON.stringify(cdpRuntimeErrors)}`);
    assert(cdpDecoyRequests.length === 0, `${scenario.mode} inert authored references were fetched: ${JSON.stringify(cdpDecoyRequests)}`);
    assert(cdpUnexpectedHTTPFailures.length === 0, `${scenario.mode} unexpected HTTP failures: ${JSON.stringify(cdpUnexpectedHTTPFailures)}`);

    await stopProcess(server);
    server = null;
    return {
        initialCount: before.count,
        finalCount: after.count,
        appColor: after.appColor,
        buttonBackground: after.buttonBackground,
        assetStatuses,
        requestedAssets,
        breakoutNamespace: after.breakoutHTMLNamespace,
        runtimeNamespace: after.breakoutRuntimeNamespace,
        compactInputDisabled: after.compactInputDisabled,
        runtimeErrorCount: cdpRuntimeErrors.length + after.runtimeErrors.length,
        unexpectedHTTPFailureCount: cdpUnexpectedHTTPFailures.length,
    };
}

async function waitForFixture(expectedURL) {
    for (let attempt = 0; attempt < 150; attempt++) {
        const state = await client.evaluate(`(() => ({
            href: location.href,
            ready: Boolean(document.querySelector("[data-testid='custom-index-app']")),
        }))()`);
        if (state.href.startsWith("chrome-error://")) {
            throw new Error(`HARNESS FAILURE: Chrome loaded an error page for ${expectedURL}`);
        }
        if (state.ready && state.href === expectedURL) {
            await wait(50);
            return;
        }
        await wait(100);
    }
    throw new Error(`HARNESS FAILURE: fixture did not become ready at ${expectedURL}`);
}

async function pageState() {
    return await client.evaluate(`(() => {
        const app = document.querySelector("[data-testid='custom-index-app']");
        const button = document.querySelector("[data-testid='custom-index-increment']");
        const breakout = document.querySelector("#fixture-breakout-html");
        const runtime = document.querySelector("#fixture-breakout-runtime");
        const compactInput = document.querySelector("#fixture-compact-input");
        return {
            count: document.querySelector("[data-testid='custom-index-count']")?.textContent ?? null,
            appColor: app ? getComputedStyle(app).color : null,
            buttonBackground: button ? getComputedStyle(button).backgroundColor : null,
            authoredText: document.querySelector("[data-testid='authored-text']")?.textContent ?? null,
            authoredHref: document.querySelector("[data-testid='authored-link']")?.getAttribute("href") ?? null,
            bodyExample: document.body?.getAttribute("data-example") ?? null,
            inlineJSON: document.querySelector("#fixture-json")?.textContent ?? null,
            doubleEscapedText: document.querySelector("#fixture-double-escaped")?.textContent ?? "",
            svgCDATA: document.querySelector("#fixture-svg-cdata")?.textContent ?? "",
            mathCDATA: document.querySelector("#fixture-math-cdata")?.textContent ?? "",
            nbspScriptType: document.querySelector("#fixture-nbsp-script")?.getAttribute("type") ?? null,
            nbspScriptSource: document.querySelector("#fixture-nbsp-script")?.getAttribute("src") ?? null,
            nbspRel: document.querySelector("#fixture-nbsp-rel")?.getAttribute("rel") ?? null,
            nbspRelHref: document.querySelector("#fixture-nbsp-rel")?.getAttribute("href") ?? null,
            nbspAs: document.querySelector("#fixture-nbsp-as")?.getAttribute("as") ?? null,
            nbspAsHref: document.querySelector("#fixture-nbsp-as")?.getAttribute("href") ?? null,
            breakoutSVGNamespace: document.querySelector("#fixture-breakout")?.namespaceURI ?? null,
            breakoutHTMLNamespace: breakout?.namespaceURI ?? null,
            breakoutRuntimeNamespace: runtime?.namespaceURI ?? null,
            compactInputNamespace: compactInput?.namespaceURI ?? null,
            compactInputDisabled: compactInput?.disabled ?? null,
            breakoutInsideSVG: Boolean(breakout?.closest("svg")),
            runtimeInsideSVG: Boolean(runtime?.closest("svg")),
            runtimeErrors: Array.from(window.__customIndexRuntimeErrors ?? []),
        };
    })()`);
}

function packagedDirectory(output) {
    const match = output.match(/^packaged (.+)$/m);
    if (!match) throw new Error(`HARNESS FAILURE: package output omitted its destination\n${output}`);
    return match[1].trim();
}

function stableInspectContractHash(report) {
    const stable = structuredClone(report);
    for (const artifact of stable.artifacts) {
        if (artifact.path === "goframe-package.json") artifact.sha256 = "<generated-at-normalized>";
    }
    return sha256(JSON.stringify(stable));
}

function commandEnvironment() {
    return {
        ...process.env,
        GOCACHE: process.env.GOCACHE ?? join(tempRoot, "gocache"),
        GOTOOLCHAIN: goToolchain,
        GOWORK: "off",
        XDG_CACHE_HOME: process.env.XDG_CACHE_HOME ?? join(tempRoot, "xdg-cache"),
    };
}

async function snapshotTree(root) {
    const entries = [];
    await walk(root);
    return entries;

    async function walk(directory) {
        const children = await readdir(directory, { withFileTypes: true });
        children.sort((left, right) => Buffer.from(left.name).compare(Buffer.from(right.name)));
        for (const child of children) {
            const path = join(directory, child.name);
            const name = relative(root, path).split("\\").join("/");
            if (child.isDirectory()) {
                await walk(path);
                continue;
            }
            assert(child.isFile(), `package graph contains unsupported entry ${name}`);
            const bytes = await readFile(path);
            entries.push({ name, bytes: bytes.length, sha256: sha256(bytes) });
        }
    }
}

async function waitForServer(port, child) {
    let lastError = null;
    for (let attempt = 0; attempt < 150; attempt++) {
        if (child.exitCode !== null) {
            throw new Error(`HARNESS FAILURE: goxc serve exited before readiness\n${serverError}`);
        }
        try {
            const response = await fetch(`http://127.0.0.1:${port}/`, { cache: "no-store" });
            if (response.ok) return;
        } catch (error) {
            lastError = error;
        }
        await wait(50);
    }
    throw new Error(`HARNESS FAILURE: goxc serve did not become ready: ${lastError?.message ?? ""}\n${serverError}`);
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
        for (const listener of listeners.get(message.method) ?? []) listener(message.params ?? {});
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
        const portServer = createPortServer();
        portServer.once("error", reject);
        portServer.listen(0, "127.0.0.1", () => {
            const address = portServer.address();
            portServer.close(() => resolvePort(address.port));
        });
    });
}

async function diagnostics() {
    let page = null;
    if (client) {
        try {
            page = await pageState();
        } catch (error) {
            page = { error: error.message };
        }
    }
    return {
        compiler,
        debugPort,
        browserStderr: browserError.slice(-6000),
        serverStderr: serverError.slice(-6000),
        cdpRuntimeErrors,
        cdpUnexpectedHTTPFailures,
        cdpDecoyRequests,
        cdpPackageAssetRequests,
        page,
    };
}

function sha256(value) {
    return createHash("sha256").update(value).digest("hex");
}

function assert(condition, message) {
    if (!condition) throw new Error(`APP FAILURE: ${message}`);
}

function assertDeepEqual(actual, expected, description) {
    const actualJSON = JSON.stringify(actual);
    const expectedJSON = JSON.stringify(expected);
    assert(actualJSON === expectedJSON, `${description}: got ${actualJSON}, want ${expectedJSON}`);
}

function wait(milliseconds) {
    return new Promise((resolveWait) => setTimeout(resolveWait, milliseconds));
}

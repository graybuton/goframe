import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { cp, mkdir, mkdtemp, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { createServer as createHTTPServer } from "node:http";
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
    `<math id="fixture-spaced-annotation" style="display:none">
        <annotation-xml encoding=" text/html ">
            <script id="fixture-spaced-annotation-script" type="application/json" src="wasm_exec.js?fixture=annotation-decoy"></script>
        </annotation-xml>
    </math>`,
    `<svg id="fixture-punctuation-svg" style="display:none">
        <title_extra id="fixture-punctuation-tag">
            <script id="fixture-punctuation-script" type="application/json" src="wasm_exec.js?fixture=punctuation-decoy"></script>
        </title_extra>
    </svg>`,
    '<!-- authored scanner close --!>',
    '<div id="fixture-legacy-owned" style="display:none">',
    '<p id="fixture-legacy-html"></p>',
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
    const generatedURLScenario = await prepareGeneratedURLScenario();
    const targetOnlyFixture = await prepareVariantFixture("base-target", "base-target.html");
    const targetOnlyScenario = await prepareScenario("base-target", targetOnlyFixture, "marker");
    const negativeManaged = await verifyActiveBaseManagedFailure(targetOnlyScenario);
    const negativeManagedStructure = await verifyManagedStructureFailures(targetOnlyScenario);
    scenarios.push(targetOnlyScenario);
    const authoredBase = await prepareAuthoredBaseScenario();

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
        const decodedPathname = decodeURLPathname(pathname);
        if (decodedPathname === "/wasm_exec.js" || decodedPathname === "/styles.css") {
            cdpDecoyRequests.push({ url: response.url, status: response.status });
        }
        if (decodedPathname.startsWith("/assets/")) {
            cdpPackageAssetRequests.push({ pathname, decodedPathname, status: response.status });
        }
        if (response.status >= 400 && pathname !== "/favicon.ico") {
            cdpUnexpectedHTTPFailures.push({ url: response.url, status: response.status });
        }
    });
    await client.call("Runtime.enable");
    await client.call("Page.enable");
    await client.call("Network.enable");
    const attributeOracle = await runAttributeOracle();
    const semanticOracle = await runManagedFirstSemanticOracle();
    const baseOracle = await runBaseResolutionOracle();
    const javascriptSourceOracle = runGeneratedJavaScriptSourceOracle(generatedURLScenario);

    for (const scenario of scenarios) {
        scenario.browser = await runBrowserScenario(scenario);
    }
    generatedURLScenario.browser = await runGeneratedURLBrowserScenario(generatedURLScenario);
    authoredBase.browser = await runAuthoredBaseBrowserScenario(authoredBase);

    const stableScenarios = scenarios.map((scenario) => ({
        mode: scenario.mode,
        indexBytes: scenario.indexBytes,
        indexSha256: scenario.indexSha256,
        inspectContractSha256: scenario.inspectContractSha256,
        artifactCount: scenario.artifactCount,
        paths: scenario.paths,
        urls: scenario.urls,
        browser: scenario.browser,
    }));
    const stableAuthoredBase = {
        indexBytes: authoredBase.indexBytes,
        indexSha256: authoredBase.indexSha256,
        sourcePreserved: authoredBase.sourcePreserved,
        generatedURLCount: authoredBase.generatedURLCount,
        browser: authoredBase.browser,
    };
    const report = {
        compiler,
        attributeOracle,
        semanticOracle,
        baseOracle,
        javascriptSourceOracle,
        negativeManaged,
        negativeManagedStructure,
        authoredBase: stableAuthoredBase,
        generatedURL: {
            paths: generatedURLScenario.paths,
            urls: generatedURLScenario.urls,
            javascriptLiterals: generatedURLScenario.javascriptLiterals,
            browser: generatedURLScenario.browser,
        },
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

async function prepareScenario(mode, fixture = join(fixtureRoot, mode), contractMode = mode) {
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
    assertAuthoredSentinels(sourceHTML, packagedHTML, mode, contractMode);
    assertPackageContract(mode, packagedHTML, assetManifest, packageMetadata, contractMode);

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
        contractMode,
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
        urls: {
            wasm: encodePackagePathAsBrowserURL(assetManifest.entrypoints.wasm),
            runtime: encodePackagePathAsBrowserURL(assetManifest.entrypoints.runtime),
            style: encodePackagePathAsBrowserURL(assetManifest.entrypoints.styles[0]),
        },
    };
}

async function prepareGeneratedURLScenario() {
    const mode = "generated-url";
    const fixture = join(tempRoot, "fixtures", mode);
    await cp(join(fixtureRoot, "marker"), fixture, { recursive: true });
    const styleSource = await readFile(join(fixture, "styles.css"), "utf8");
    await rm(join(fixture, "index.html"));
    await rm(join(fixture, "styles.css"));

    const wasmName = "bundle space#query?percent%界😀.wasm";
    const styleName = "style space&query?#percent%\"'界😀.css";
    await writeFile(join(fixture, styleName), styleSource);
    await writeFile(join(fixture, "goframe.json"), `${JSON.stringify({
        name: "custom-index-generated-url",
        entry: ".",
        output: "dist",
        compiler: "tinygo",
        wasm: wasmName,
        assets: [styleName],
    }, null, 2)}\n`);

    const workspace = join(tempRoot, "workspaces", mode);
    const packageOutput = await runCommand(goxc, [
        "package",
        fixture,
        `--compiler=${compiler}`,
        `--workspace=${workspace}`,
        "--asset-hash",
        "--preload",
    ], commandEnvironment());
    const packageDirectory = packagedDirectory(packageOutput);
    const packagedHTML = await readFile(join(packageDirectory, "index.html"), "utf8");
    const manifest = JSON.parse(await readFile(join(packageDirectory, "asset-manifest.json"), "utf8"));
    const inspectReport = JSON.parse(await runCommand(goxc, [
        "inspect",
        `--dir=${packageDirectory}`,
        "--format=json",
    ], commandEnvironment()));
    const paths = {
        wasm: manifest.entrypoints.wasm,
        runtime: manifest.entrypoints.runtime,
        style: manifest.entrypoints.styles[0],
    };
    const urls = Object.fromEntries(
        Object.entries(paths).map(([name, path]) => [name, encodePackagePathAsBrowserURL(path)]),
    );
    for (const [name, url] of Object.entries(urls)) {
        assert(packagedHTML.includes(url), `generated URL package index is missing ${name} URL ${JSON.stringify(url)}`);
        assert(inspectReport.artifacts.some((artifact) => artifact.path === paths[name]), `generated URL inspect report is missing ${JSON.stringify(paths[name])}`);
    }
    assert(!packagedHTML.includes(wasmName), "generated URL package retained the raw WASM package path");
    assert(!packagedHTML.includes(styleName), "generated URL package retained the raw stylesheet package path");

    return {
        mode,
        fixture,
        workspace,
        packageDirectory,
        packagedHTML,
        paths,
        urls,
        javascriptLiterals: [],
    };
}

async function prepareVariantFixture(name, htmlName) {
    const fixture = join(tempRoot, "fixtures", name);
    await cp(join(fixtureRoot, "marker"), fixture, { recursive: true });
    await cp(join(fixtureRoot, htmlName), join(fixture, "index.html"));
    return fixture;
}

async function verifyActiveBaseManagedFailure(scenario) {
    const indexPath = join(scenario.fixture, "index.html");
    const targetOnlySource = await readFile(indexPath, "utf8");
    const activeBaseSource = targetOnlySource.replace(
        '<base target="_blank">',
        '<base href="/redirected/">',
    );
    assert(activeBaseSource !== targetOnlySource, "negative managed fixture did not replace its target-only base");
    const packageBefore = await snapshotTree(scenario.packageDirectory);
    await writeFile(indexPath, activeBaseSource);
    const sourceBefore = await snapshotTree(scenario.fixture);
    let result;
    try {
        result = await runCommandResult(goxc, [
            "package",
            scenario.fixture,
            `--compiler=${compiler}`,
            `--workspace=${scenario.workspace}`,
            "--asset-hash",
            "--preload",
            "--compress=gzip,br",
        ], commandEnvironment());
        const sourceAfter = await snapshotTree(scenario.fixture);
        const packageAfter = await snapshotTree(scenario.packageDirectory);
        assertDeepEqual(sourceAfter, sourceBefore, "active-base managed source graph");
        assertDeepEqual(packageAfter, packageBefore, "active-base managed previous package graph");
    } finally {
        await writeFile(indexPath, targetOnlySource);
    }
    assert(result.code !== 0, "active-base managed package unexpectedly succeeded");
    assert(result.output.includes("active <base href>"), `active-base managed error = ${JSON.stringify(result.output)}`);
    assert(result.output.includes("goframe:preload output"), `active-base managed error omitted the blocked operation: ${JSON.stringify(result.output)}`);
    assert(!/^packaged /m.test(result.output), `active-base managed package emitted success output: ${JSON.stringify(result.output)}`);
    return {
        rejected: true,
        operation: "goframe:preload output",
        previousPackagePreserved: true,
        sourcePreserved: true,
        successOutputBytes: 0,
    };
}

async function verifyManagedStructureFailures(scenario) {
    const indexPath = join(scenario.fixture, "index.html");
    const originalSource = await readFile(indexPath, "utf8");
    const packageBefore = await snapshotTree(scenario.packageDirectory);
    const temporaryRoot = join(tempRoot, "managed-structure-tmp");
    await mkdir(temporaryRoot, { recursive: true });
    const cases = [
        {
            name: "foreign runtime block",
            source: `<!doctype html><html><head></head><body><svg><!-- goframe:runtime --><!-- /goframe:runtime --></svg></body></html>`,
            wants: ["goframe:runtime", "SVG or MathML ancestry", "safe HTML parent"],
        },
        {
            name: "foreign bootstrap block",
            source: `<!doctype html><html><head></head><body><math><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></math></body></html>`,
            wants: ["goframe:bootstrap", "SVG or MathML ancestry", "safe HTML parent"],
        },
        {
            name: "cross-parent runtime block",
            source: `<!doctype html><html><head><!-- goframe:runtime --></head><body id="app"><!-- /goframe:runtime --></body></html>`,
            wants: ["goframe:runtime", "different structural contexts", "safe HTML parent"],
        },
        {
            name: "reversed owned runtime order",
            source: `<!doctype html><html><head></head><body><!-- goframe:bootstrap --><!-- /goframe:bootstrap --><!-- goframe:runtime --><!-- /goframe:runtime --></body></html>`,
            wants: ["GoFrame-owned bootstrap may execute before its runtime", "blocking runtime integration before the bootstrap"],
        },
    ];
    const results = [];
    try {
        for (const test of cases) {
            await writeFile(indexPath, test.source);
            const sourceBefore = await snapshotTree(scenario.fixture);
            const result = await runCommandResult(goxc, [
                "package",
                scenario.fixture,
                `--compiler=${compiler}`,
                `--workspace=${scenario.workspace}`,
                "--asset-hash",
                "--preload",
                "--compress=gzip,br",
            ], {
                ...commandEnvironment(),
                TMPDIR: temporaryRoot,
            });
            assert(result.code !== 0, `${test.name} unexpectedly packaged successfully`);
            for (const want of test.wants) {
                assert(result.output.includes(want), `${test.name} error omitted ${JSON.stringify(want)}: ${JSON.stringify(result.output)}`);
            }
            const successOutput = result.output.match(/^packaged .+$/gm)?.join("\n") ?? "";
            assert(successOutput.length === 0, `${test.name} emitted success output: ${JSON.stringify(result.output)}`);
            assertDeepEqual(await snapshotTree(scenario.fixture), sourceBefore, `${test.name} source graph`);
            assertDeepEqual(await snapshotTree(scenario.packageDirectory), packageBefore, `${test.name} previous package graph`);
            assertDeepEqual(await readdir(temporaryRoot), [], `${test.name} temporary stage cleanup`);
            results.push({
                name: test.name,
                rejected: true,
                sourcePreserved: true,
                previousPackagePreserved: true,
                temporaryStageRemoved: true,
                successOutputBytes: Buffer.byteLength(successOutput),
            });
        }
    } finally {
        await writeFile(indexPath, originalSource);
    }
    assertDeepEqual(await snapshotTree(scenario.packageDirectory), packageBefore, "managed-structure final previous package graph");
    return results;
}

async function prepareAuthoredBaseScenario() {
    const mode = "base-authored";
    const fixture = await prepareVariantFixture(mode, "base-authored.html");
    const workspace = join(tempRoot, "workspaces", mode);
    const sourceHTML = await readFile(join(fixture, "index.html"), "utf8");
    const packageOutput = await runCommand(goxc, [
        "package",
        fixture,
        `--compiler=${compiler}`,
        `--workspace=${workspace}`,
        "--asset-hash",
        "--compress=gzip,br",
    ], commandEnvironment());
    const packageDirectory = packagedDirectory(packageOutput);
    const packagedHTML = await readFile(join(packageDirectory, "index.html"), "utf8");
    assert(packagedHTML === sourceHTML, "authored active-base index changed during packaging");
    assert(!packagedHTML.includes("assets/"), "authored active-base index gained a package-owned URL");
    assert(!packagedHTML.includes("goframe:"), "authored active-base index gained a managed marker");
    return {
        mode,
        fixture,
        workspace,
        packageDirectory,
        indexBytes: Buffer.byteLength(packagedHTML),
        indexSha256: sha256(packagedHTML),
        sourcePreserved: true,
        generatedURLCount: 0,
    };
}

function assertAuthoredSentinels(source, packaged, mode, contractMode) {
    for (const sentinel of authoredSentinels) {
        assert(source.includes(sentinel), `${mode} source is missing sentinel ${JSON.stringify(sentinel)}`);
        assert(packaged.includes(sentinel), `${mode} package changed sentinel ${JSON.stringify(sentinel)}`);
    }
    if (contractMode === "legacy") {
        assert(source.includes("%77asm_exec.js?fixture=legacy#runtime"), "legacy source is missing the percent-encoded runtime reference");
        assert(source.includes("%6dy%20%26copy%3B%20style.css?fixture=legacy&copy;=x#theme"), "legacy source is missing the percent-encoded stylesheet reference");
        assert(source.includes("%62undle.wasm?fixture=legacy#wasm"), "legacy source is missing the percent-encoded bootstrap reference");
        assert(!packaged.includes("%77asm_exec.js?fixture=legacy#runtime"), "legacy package retained the percent-encoded runtime reference");
        assert(!packaged.includes("%6dy%20%26copy%3B%20style.css"), "legacy package retained the percent-encoded stylesheet reference");
        assert(!packaged.includes("%62undle.wasm?fixture=legacy#wasm"), "legacy package retained the percent-encoded bootstrap reference");
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

function assertPackageContract(mode, html, manifest, metadata, contractMode) {
    assert(metadata.hashAssets === true, `${mode} package did not enable asset hashing`);
    assert(metadata.preload === true, `${mode} package did not enable preload`);
    const wasm = manifest.entrypoints.wasm;
    const runtime = manifest.entrypoints.runtime;
    const styles = manifest.entrypoints.styles;
    assert(Array.isArray(styles) && styles.length === 1, `${mode} package styles = ${JSON.stringify(styles)}`);
    const style = styles[0];
    const urls = {
        wasm: encodePackagePathAsBrowserURL(wasm),
        runtime: encodePackagePathAsBrowserURL(runtime),
        style: encodePackagePathAsBrowserURL(style),
    };
    const styleLogicalName = contractMode === "legacy" ? "my &copy; style.css" : "styles.css";
    for (const [logicalName, path] of [["bundle.wasm", wasm], ["wasm_exec.js", runtime], [styleLogicalName, style]]) {
        assert(/^assets\/.+\.[0-9a-f]{8}\.[^.]+$/.test(path), `${mode} ${logicalName} path is not hashed: ${path}`);
        const compressed = manifest.assets[logicalName]?.compressed;
        assert(compressed?.gzip === `${path}.gz`, `${mode} ${logicalName} gzip sidecar is missing`);
        assert(compressed?.br === `${path}.br`, `${mode} ${logicalName} Brotli sidecar is missing`);
    }
    for (const url of Object.values(urls)) {
        assert(html.includes(`<link rel="preload" href="${encodeDoubleQuotedAttribute(url)}"`), `${mode} preload is missing ${url}`);
    }
    if (contractMode === "marker") {
        for (const name of ["preload", "runtime", "bootstrap"]) {
            assert(html.includes(`<!-- goframe:${name} -->`), `marker package lost ${name} start marker`);
            assert(html.includes(`<!-- /goframe:${name} -->`), `marker package lost ${name} end marker`);
        }
        assert(html.includes(`<script src="${urls.runtime}"></script>`), "marker runtime target is stale");
        assert(html.includes(`fetch("${urls.wasm}")`), "marker WASM target is stale");
        assert(html.includes(`href="${encodeDoubleQuotedAttribute(urls.style)}?fixture=marker#theme"`), "marker stylesheet target is stale");
        assert(!html.includes("authored preload interior"), "marker preload interior was not replaced");
    } else {
        assert(html.includes(`SRC=' ${urls.runtime}?fixture=legacy#runtime '`), "legacy runtime target is stale");
        assert(html.includes(`fetch ( ' ${urls.wasm}?fixture=legacy#wasm ' )`), "legacy WASM target is stale");
        assert(html.includes(`href=${encodeUnquotedAttribute(urls.style)}?fixture=legacy&copy;=x#theme`), "legacy stylesheet target is stale");
        assert(html.includes('type="text/javascript1.5"'), "legacy runtime MIME type changed");
        assert(html.includes('<input id="fixture-compact-input" disabled/>'), "compact boolean tag changed");
        const preload = html.indexOf(`<link rel="preload" href="${urls.wasm}"`);
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
    if (scenario.contractMode === "legacy") {
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
        assert(before.legacyHTMLNamespace === "http://www.w3.org/1999/xhtml", "legacy owned element left the HTML namespace");
        assert(before.legacyRuntimeNamespace === "http://www.w3.org/1999/xhtml", "legacy runtime script left the HTML namespace");
        assert(before.compactInputNamespace === "http://www.w3.org/1999/xhtml", "compact input did not enter the HTML namespace");
        assert(before.spacedAnnotationNamespace === "http://www.w3.org/1998/Math/MathML", "spaced annotation child left the MathML namespace");
        assert(before.punctuationTagName === "title_extra", `punctuation tag name = ${JSON.stringify(before.punctuationTagName)}`);
        assert(before.punctuationScriptNamespace === "http://www.w3.org/2000/svg", "punctuation tag child left the SVG namespace");
        assert(before.scannerCommentPresent === true, "incorrectly closed authored comment was not exposed as a browser comment");
        assert(before.compactInputDisabled === true, "compact boolean attribute was not accepted");
        assert(before.encodedStyleHref === `${scenario.urls.style}?fixture=legacy©=x#theme`, `legacy stylesheet semantic href = ${JSON.stringify(before.encodedStyleHref)}`);
        assert(before.encodedStyleAttributeCount === 4, `legacy stylesheet attribute count = ${before.encodedStyleAttributeCount}`);
        assert(before.encodedStyleNamespace === "http://www.w3.org/1999/xhtml", "legacy stylesheet left the HTML namespace");
    }
    if (scenario.mode === "base-target") {
        assert(before.baseHref === null, `target-only base href = ${JSON.stringify(before.baseHref)}`);
        assert(before.baseTarget === "_blank", `target-only base target = ${JSON.stringify(before.baseTarget)}`);
        assert(before.runtimeSource === scenario.urls.runtime, `target-only runtime source = ${JSON.stringify(before.runtimeSource)}`);
        assert(before.runtimeResolvedPath === `/${scenario.urls.runtime}`, `target-only resolved runtime path = ${JSON.stringify(before.runtimeResolvedPath)}`);
        assert(before.stylesheetResolvedPath === `/${scenario.urls.style}`, `target-only resolved stylesheet path = ${JSON.stringify(before.stylesheetResolvedPath)}`);
        const preloadPaths = before.preloads.map((preload) => preload.raw).sort();
        assertDeepEqual(preloadPaths, Object.values(scenario.urls).sort(), "target-only preload package URLs");
        for (const preload of before.preloads) {
            assert(preload.resolvedPath === `/${preload.raw}`, `target-only resolved preload path = ${JSON.stringify(preload)}`);
        }
    }

    const requestedAssets = [...new Set(cdpPackageAssetRequests
        .filter((request) => request.status === 200)
        .map((request) => request.decodedPathname))].sort();
    for (const path of Object.values(scenario.paths)) {
        assert(requestedAssets.includes(`/${path}`), `${scenario.mode} browser did not request ${path}`);
    }
    if (scenario.contractMode === "legacy") {
        const entityDecodedStyle = `/${scenario.paths.style.replace("&copy;", "©")}`;
        assert(!cdpPackageAssetRequests.some((request) => request.decodedPathname === "/assets/my"), "legacy stylesheet request was truncated at its generated space");
        assert(!cdpPackageAssetRequests.some((request) => request.decodedPathname === entityDecodedStyle), "legacy stylesheet ampersand was decoded as an HTML named reference");
    }

    const expectedAssets = Object.keys(scenario.paths).map((name) => ({
        path: scenario.paths[name],
        url: scenario.urls[name],
    }));
    const assetStatuses = await client.evaluate(`Promise.all(${JSON.stringify(expectedAssets)}.map(async (asset) => {
        const response = await fetch("/" + asset.url, { cache: "no-store" });
        return [asset.path, asset.url, response.status, response.headers.get("content-type")];
    }))`);
    for (const [path, urlPath, status] of assetStatuses) {
        assert(status === 200, `${scenario.mode} final asset ${path} returned ${status}`);
        assert(urlPath === encodePackagePathAsBrowserURL(path), `${scenario.mode} browser URL for ${path} changed to ${urlPath}`);
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
        legacyHTMLNamespace: after.legacyHTMLNamespace,
        runtimeNamespace: after.legacyRuntimeNamespace,
        spacedAnnotationNamespace: after.spacedAnnotationNamespace,
        punctuationTagName: after.punctuationTagName,
        punctuationScriptNamespace: after.punctuationScriptNamespace,
        scannerCommentPresent: after.scannerCommentPresent,
        compactInputDisabled: after.compactInputDisabled,
        baseHref: after.baseHref,
        baseTarget: after.baseTarget,
        runtimeResolvedPath: after.runtimeResolvedPath,
        stylesheetResolvedPath: after.stylesheetResolvedPath,
        preloadResolvedPaths: after.preloads.map((preload) => preload.resolvedPath).sort(),
        runtimeErrorCount: cdpRuntimeErrors.length + after.runtimeErrors.length,
        unexpectedHTTPFailureCount: cdpUnexpectedHTTPFailures.length,
    };
}

function runGeneratedJavaScriptSourceOracle(scenario) {
    const literals = [];
    const pattern = /\bfetch\(\s*((['"])(?:\\.|(?!\2)[^\\])*\2)\s*\)/g;
    for (const match of scenario.packagedHTML.matchAll(pattern)) {
        const source = match[1];
        let value;
        try {
            value = Function(`"use strict"; return (${source});`)();
        } catch (error) {
            throw new Error(`APP FAILURE: generated JavaScript literal ${JSON.stringify(source)} did not evaluate: ${error.message}`);
        }
        assert(value === scenario.urls.wasm, `generated JavaScript literal evaluated to ${JSON.stringify(value)}, want ${JSON.stringify(scenario.urls.wasm)}`);
        assert(!source.includes("\\a") && !source.includes("\\U"), `generated JavaScript literal contains a Go-only escape: ${JSON.stringify(source)}`);
        literals.push({ source, value });
    }
    assert(literals.length === 1, `generated index JavaScript literal count = ${literals.length}, want 1`);
    scenario.javascriptLiterals = literals;
    return literals;
}

async function runGeneratedURLBrowserScenario(scenario) {
    const port = await pickFreePort();
    serverError = "";
    server = spawn(goxc, [
        "serve",
        `--dir=${scenario.packageDirectory}`,
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
    assert(before.count === "0", `generated URL initial count = ${JSON.stringify(before.count)}`);
    assert(before.appColor === "rgb(13, 71, 161)", `generated URL stylesheet color = ${before.appColor}`);
    assert(before.runtimeSource === scenario.urls.runtime, `generated runtime source = ${JSON.stringify(before.runtimeSource)}`);
    assert(before.runtimeResolvedPath === `/${scenario.urls.runtime}`, `generated runtime request path = ${JSON.stringify(before.runtimeResolvedPath)}`);
    assert(before.stylesheetResolvedPath === `/${scenario.urls.style}`, `generated stylesheet request path = ${JSON.stringify(before.stylesheetResolvedPath)}`);

    const successfulRequests = cdpPackageAssetRequests.filter((request) => request.status === 200);
    for (const name of Object.keys(scenario.paths)) {
        const expectedPathname = `/${scenario.urls[name]}`;
        const expectedDecodedPathname = `/${scenario.paths[name]}`;
        assert(successfulRequests.some((request) => request.pathname === expectedPathname && request.decodedPathname === expectedDecodedPathname), `generated ${name} request did not preserve the exact package filename: ${JSON.stringify(successfulRequests)}`);
    }
    const assetStatuses = await client.evaluate(`Promise.all(${JSON.stringify(Object.entries(scenario.urls))}.map(async ([name, urlPath]) => {
        const response = await fetch("/" + urlPath, { cache: "no-store" });
        return [name, urlPath, response.status];
    }))`);
    for (const [name, urlPath, status] of assetStatuses) {
        assert(status === 200, `generated URL ${name} asset ${urlPath} returned ${status}`);
    }

    await client.evaluate(`document.querySelector("[data-testid='custom-index-increment']").click()`);
    for (let attempt = 0; attempt < 100; attempt++) {
        if ((await pageState()).count === "1") break;
        await wait(20);
    }
    const after = await pageState();
    assert(after.count === "1", "generated URL interaction did not update the application");
    assertDeepEqual(after.runtimeErrors, [], "generated URL GoFrame runtime errors");
    assert(cdpRuntimeErrors.length === 0, `generated URL CDP runtime errors: ${JSON.stringify(cdpRuntimeErrors)}`);
    assert(cdpUnexpectedHTTPFailures.length === 0, `generated URL HTTP failures: ${JSON.stringify(cdpUnexpectedHTTPFailures)}`);

    await stopProcess(server);
    server = null;
    return {
        initialCount: before.count,
        finalCount: after.count,
        requests: successfulRequests,
        assetStatuses,
        runtimeErrorCount: cdpRuntimeErrors.length + after.runtimeErrors.length,
        unexpectedHTTPFailureCount: cdpUnexpectedHTTPFailures.length,
    };
}

async function runAuthoredBaseBrowserScenario(scenario) {
    const port = await pickFreePort();
    serverError = "";
    server = spawn(goxc, [
        "serve",
        `--dir=${scenario.packageDirectory}`,
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
    const url = `http://127.0.0.1:${port}/?mode=${scenario.mode}&run=${Date.now()}`;
    await client.call("Page.navigate", { url });
    await waitForOracleDocument(url);
    const state = await client.evaluate(`(() => ({
        pagePresent: Boolean(document.querySelector("[data-testid='authored-base-page']")),
        baseHref: document.querySelector("base")?.getAttribute("href") ?? null,
        baseURI: document.baseURI,
        authoredHref: document.querySelector("[data-testid='authored-external-link']")?.getAttribute("href") ?? null,
        authoredResolvedHref: document.querySelector("[data-testid='authored-external-link']")?.href ?? null,
        generatedAssetElements: document.querySelectorAll("script[src^='assets/'], link[href^='assets/']").length,
    }))()`);
    assert(state.pagePresent, "authored active-base page did not load");
    assert(state.baseHref === "/authored/", `authored active-base href = ${JSON.stringify(state.baseHref)}`);
    assert(new URL(state.baseURI).pathname === "/authored/", `authored active-base URI = ${JSON.stringify(state.baseURI)}`);
    assert(state.authoredHref === "https://example.invalid/docs", `authored external href = ${JSON.stringify(state.authoredHref)}`);
    assert(state.authoredResolvedHref === state.authoredHref, `authored external URL was changed to ${JSON.stringify(state.authoredResolvedHref)}`);
    assert(state.generatedAssetElements === 0, `authored active-base page gained ${state.generatedAssetElements} generated asset elements`);
    assert(cdpPackageAssetRequests.length === 0, `authored active-base page requested package assets: ${JSON.stringify(cdpPackageAssetRequests)}`);
    assert(cdpRuntimeErrors.length === 0, `authored active-base runtime errors: ${JSON.stringify(cdpRuntimeErrors)}`);
    assert(cdpUnexpectedHTTPFailures.length === 0, `authored active-base HTTP failures: ${JSON.stringify(cdpUnexpectedHTTPFailures)}`);

    await stopProcess(server);
    server = null;
    return {
        baseHref: state.baseHref,
        basePathname: new URL(state.baseURI).pathname,
        authoredHref: state.authoredHref,
        generatedAssetElements: state.generatedAssetElements,
        packageAssetRequestCount: cdpPackageAssetRequests.length,
        runtimeErrorCount: cdpRuntimeErrors.length,
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
        const legacyHTML = document.querySelector("#fixture-legacy-html");
        const runtime = document.querySelector("#fixture-legacy-runtime");
        const compactInput = document.querySelector("#fixture-compact-input");
        const spacedAnnotationScript = document.querySelector("#fixture-spaced-annotation-script");
        const punctuationTag = document.querySelector("#fixture-punctuation-tag");
        const punctuationScript = document.querySelector("#fixture-punctuation-script");
        const commentWalker = document.createTreeWalker(document, NodeFilter.SHOW_COMMENT);
        let scannerCommentPresent = false;
        for (let comment = commentWalker.nextNode(); comment; comment = commentWalker.nextNode()) {
            if (comment.data === " authored scanner close ") scannerCommentPresent = true;
        }
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
            legacyHTMLNamespace: legacyHTML?.namespaceURI ?? null,
            legacyRuntimeNamespace: runtime?.namespaceURI ?? null,
            compactInputNamespace: compactInput?.namespaceURI ?? null,
            spacedAnnotationNamespace: spacedAnnotationScript?.namespaceURI ?? null,
            punctuationTagName: punctuationTag?.localName ?? null,
            punctuationScriptNamespace: punctuationScript?.namespaceURI ?? null,
            scannerCommentPresent,
            compactInputDisabled: compactInput?.disabled ?? null,
            encodedStyleHref: document.querySelector("#fixture-encoded-style")?.getAttribute("href") ?? null,
            encodedStyleAttributeCount: document.querySelector("#fixture-encoded-style")?.attributes.length ?? null,
            encodedStyleNamespace: document.querySelector("#fixture-encoded-style")?.namespaceURI ?? null,
            baseHref: document.querySelector("base")?.getAttribute("href") ?? null,
            baseTarget: document.querySelector("base")?.getAttribute("target") ?? null,
            baseURI: document.baseURI,
            runtimeSource: document.querySelector("script[src^='assets/']")?.getAttribute("src") ?? null,
            runtimeResolvedPath: document.querySelector("script[src^='assets/']") ? new URL(document.querySelector("script[src^='assets/']").src).pathname : null,
            stylesheetResolvedPath: document.querySelector("link[rel~='stylesheet'][href^='assets/']") ? new URL(document.querySelector("link[rel~='stylesheet'][href^='assets/']").href).pathname : null,
            preloads: Array.from(document.querySelectorAll("link[rel='preload'][href^='assets/']"), (element) => ({
                raw: element.getAttribute("href"),
                resolvedPath: new URL(element.href).pathname,
            })),
            runtimeErrors: Array.from(window.__customIndexRuntimeErrors ?? []),
        };
    })()`);
}

async function runAttributeOracle() {
    const cases = [
        {
            name: "double quoted",
            source: `<link href="assets/my &quot; &amp;copy; style.css?v=1&copy;=x#theme">`,
            selector: "link",
            attribute: "href",
            expected: `assets/my " &copy; style.css?v=1©=x#theme`,
        },
        {
            name: "single quoted",
            source: `<link href='assets/my &#39; &amp;copy; style.css?v=1&copy;=x#theme'>`,
            selector: "link",
            attribute: "href",
            expected: `assets/my ' &copy; style.css?v=1©=x#theme`,
        },
        {
            name: "unquoted",
            source: `<link href=assets/my&#32;&amp;copy;&#32;style.css?v=1&copy;=x#theme>`,
            selector: "link",
            attribute: "href",
            expected: `assets/my &copy; style.css?v=1©=x#theme`,
        },
        {
            name: "literal NUL",
            source: `<script src="\0wasm_exec.js"></script>`,
            selector: "script",
            attribute: "src",
            expected: `�wasm_exec.js`,
        },
        {
            name: "named reference",
            source: `<link href="styles&period;css?value=&copy;#theme">`,
            selector: "link",
            attribute: "href",
            expected: `styles.css?value=©#theme`,
        },
        {
            name: "numeric reference",
            source: `<link href="styles&#46;css?value=&#169;#theme">`,
            selector: "link",
            attribute: "href",
            expected: `styles.css?value=©#theme`,
        },
    ];
    const results = await client.evaluate(`(() => {
        const cases = ${JSON.stringify(cases)};
        return cases.map((test) => {
            const document = new DOMParser().parseFromString(test.source, "text/html");
            const element = document.querySelector(test.selector);
            return {
                name: test.name,
                value: element?.getAttribute(test.attribute) ?? null,
                attributeCount: element?.attributes.length ?? null,
                namespace: element?.namespaceURI ?? null,
            };
        });
    })()`);
    for (const [index, test] of cases.entries()) {
        const result = results[index];
        assert(result.value === test.expected, `${test.name} browser value = ${JSON.stringify(result.value)}, want ${JSON.stringify(test.expected)}`);
        assert(result.attributeCount === 1, `${test.name} browser attribute count = ${result.attributeCount}`);
        assert(result.namespace === "http://www.w3.org/1999/xhtml", `${test.name} browser namespace = ${result.namespace}`);
    }
    assert(!results.find((result) => result.name === "literal NUL").value.includes("\0"), "literal NUL remained in the browser attribute value");
    return results;
}

async function runBaseResolutionOracle() {
    const pages = new Map();
    const oracleServer = createHTTPServer((request, response) => {
        const url = new URL(request.url, "http://127.0.0.1");
        const source = pages.get(url.pathname);
        if (source === undefined) {
            response.writeHead(404).end("missing base oracle case");
            return;
        }
        response.setHeader("cache-control", "no-store");
        response.setHeader("content-type", "text/html; charset=utf-8");
        response.end(source);
    });
    await new Promise((resolveListen, reject) => {
        oracleServer.once("error", reject);
        oracleServer.listen(0, "127.0.0.1", resolveListen);
    });
    const address = oracleServer.address();
    const origin = `http://127.0.0.1:${address.port}`;
    const results = [];

    try {
        const values = [
            ["root relative", "/other/"],
            ["relative subdirectory", "subdirectory/"],
            ["absolute URL", "https://example.invalid/deployment/"],
            ["protocol relative", "//example.invalid/deployment/"],
            ["empty", ""],
            ["dot", "."],
            ["dot slash", "./"],
            ["fragment", "#deployment"],
            ["query", "?deployment=base"],
        ];
        for (const [name, href] of values) {
            await runCase({
                name: `value: ${name}`,
                source: `<base href="${href}"><a id="asset" href="assets/runtime.js">asset</a>`,
                expectedBase(documentURL) {
                    return new URL(href, documentURL).href;
                },
            });
        }

        const contexts = [
            {
                name: "target only",
                source: `<base target="_blank"><a id="asset" href="assets/runtime.js">asset</a>`,
            },
            {
                name: "body base",
                source: `<body><base href="/body-base/"><a id="asset" href="assets/runtime.js">asset</a></body>`,
                expectedHref: "/body-base/",
            },
            {
                name: "ordinary template",
                source: `<template><base href="/template-base/"></template><a id="asset" href="assets/runtime.js">asset</a>`,
            },
            {
                name: "declarative shadow template",
                source: `<host-element><template shadowrootmode="open"><base href="/shadow-base/"></template></host-element><a id="asset" href="assets/runtime.js">asset</a>`,
            },
            {
                name: "SVG foreign content",
                source: `<svg><base href="/svg-base/"></base></svg><a id="asset" href="assets/runtime.js">asset</a>`,
            },
            {
                name: "MathML foreign content",
                source: `<math><base href="/math-base/"></base></math><a id="asset" href="assets/runtime.js">asset</a>`,
            },
            {
                name: "comment text",
                source: `<!-- <base href="/comment-base/"> --><a id="asset" href="assets/runtime.js">asset</a>`,
            },
            {
                name: "script text",
                source: `<script type="application/json"><base href="/script-base/"></script><a id="asset" href="assets/runtime.js">asset</a>`,
            },
            {
                name: "noscript text",
                source: `<noscript><base href="/noscript-base/"></noscript><a id="asset" href="assets/runtime.js">asset</a>`,
            },
            {
                name: "first active base wins",
                source: `<base href="/first-base/"><base href="/second-base/"><a id="asset" href="assets/runtime.js">asset</a>`,
                expectedHref: "/first-base/",
            },
        ];
        for (const test of contexts) {
            await runCase({
                name: `context: ${test.name}`,
                source: test.source,
                expectedBase(documentURL) {
                    return test.expectedHref ? new URL(test.expectedHref, documentURL).href : documentURL;
                },
            });
        }
    } finally {
        await new Promise((resolveClose) => oracleServer.close(resolveClose));
    }
    return results;

    async function runCase(test) {
        const path = `/base/${encodeURIComponent(test.name)}`;
        pages.set(path, `<!doctype html><meta charset="utf-8">${test.source}`);
        const documentURL = `${origin}${path}?run=${Date.now()}`;
        await client.call("Page.navigate", { url: documentURL });
        await waitForOracleDocument(documentURL);
        const result = await client.evaluate(`(() => ({
            baseURI: document.baseURI,
            rawAsset: document.querySelector("#asset")?.getAttribute("href") ?? null,
            resolvedAsset: document.querySelector("#asset")?.href ?? null,
        }))()`);
        const expectedBase = test.expectedBase(documentURL);
        const expectedAsset = new URL("assets/runtime.js", expectedBase).href;
        assert(result.baseURI === expectedBase, `${test.name} base URI = ${JSON.stringify(result.baseURI)}, want ${JSON.stringify(expectedBase)}`);
        assert(result.rawAsset === "assets/runtime.js", `${test.name} raw asset path changed`);
        assert(result.resolvedAsset === expectedAsset, `${test.name} resolved asset = ${JSON.stringify(result.resolvedAsset)}, want ${JSON.stringify(expectedAsset)}`);
        results.push({
            name: test.name,
            baseURI: result.baseURI,
            resolvedAsset: result.resolvedAsset,
        });
    }
}

async function runManagedFirstSemanticOracle() {
    const pages = new Map();
    const requests = [];
    const orderedRuntimeDelayMS = 75;
    const orderedRuntimeResources = new Set([
        "blocking.js",
        "reversed.js",
        "async.js",
        "defer.js",
        "module.js",
    ]);
    const orderedRuntimeSchedules = [];
    const oracleServer = createHTTPServer((request, response) => {
        const url = new URL(request.url, "http://127.0.0.1");
        requests.push(url.pathname);
        response.setHeader("cache-control", "no-store");
        response.setHeader("connection", "close");
        if (url.pathname.startsWith("/case/")) {
            const source = pages.get(url.pathname);
            if (source === undefined) {
                response.writeHead(404).end("missing oracle case");
                return;
            }
            response.setHeader("content-type", "text/html; charset=utf-8");
            response.end(source);
            return;
        }
        if (url.pathname.startsWith("/runtime/")) {
            response.setHeader("content-type", "text/javascript; charset=utf-8");
            response.end(`window.__oracleRuntimeLoads = [...(window.__oracleRuntimeLoads ?? []), ${JSON.stringify(url.pathname)}];`);
            return;
        }
        if (url.pathname.startsWith("/ordered-runtime/")) {
            const resource = url.pathname.slice("/ordered-runtime/".length);
            if (!orderedRuntimeResources.has(resource)) {
                response.writeHead(404).end("missing ordered runtime resource");
                return;
            }
            response.setHeader("content-type", "text/javascript; charset=utf-8");
            orderedRuntimeSchedules.push({
                resource,
                query: url.search,
                delayMS: orderedRuntimeDelayMS,
            });
            setTimeout(() => {
                response.end(`window.__oracleRuntimeReady = true; window.__oracleOrder.push("runtime");`);
            }, orderedRuntimeDelayMS);
            return;
        }
        if (url.pathname.startsWith("/style/")) {
            response.setHeader("content-type", "text/css; charset=utf-8");
            response.end(`:host, html { --goframe-oracle-style: ${JSON.stringify(url.pathname)}; }`);
            return;
        }
        response.writeHead(404).end("missing oracle resource");
    });
    await new Promise((resolveListen, reject) => {
        oracleServer.once("error", reject);
        oracleServer.listen(0, "127.0.0.1", resolveListen);
    });
    const address = oracleServer.address();
    const origin = `http://127.0.0.1:${address.port}`;
    const results = [];

    try {
        for (const test of [
            { name: "quotation mark", rawValue: `a"`, expectedValue: `a"` },
            { name: "apostrophe", rawValue: `a'`, expectedValue: `a'` },
            { name: "equals", rawValue: "a=b", expectedValue: "a=b" },
            { name: "grave accent", rawValue: "a`b", expectedValue: "a`b" },
            { name: "less-than", rawValue: "a<b", expectedValue: "a<b" },
        ]) {
            const runtimePath = `/runtime/tag-state-${encodeURIComponent(test.name)}.js`;
            await runCase({
                name: `tag state unquoted ${test.name}`,
                classification: "simple profile",
                source: `<div id=target data-x=${test.rawValue}><script id=runtime src="${origin}${runtimePath}"></script>`,
                expression: `(() => {
                    const target = document.querySelector("#target");
                    const runtime = document.querySelector("#runtime");
                    return {
                        value: target?.getAttribute("data-x") ?? null,
                        attributeCount: target?.attributes.length ?? null,
                        runtimePresent: Boolean(runtime),
                        runtimeNamespace: runtime?.namespaceURI ?? null,
                        executed: (window.__oracleRuntimeLoads ?? []).includes(${JSON.stringify(runtimePath)}),
                    };
                })()`,
                validate(result, caseRequests) {
                    assert(result.value === test.expectedValue, `unquoted ${test.name} value = ${JSON.stringify(result.value)}`);
                    assert(result.attributeCount === 2, `unquoted ${test.name} attribute count = ${result.attributeCount}`);
                    assert(result.runtimePresent && result.runtimeNamespace === "http://www.w3.org/1999/xhtml", `unquoted ${test.name} hid the following script`);
                    assert(result.executed && caseRequests.includes(runtimePath), `unquoted ${test.name} runtime was not requested`);
                },
            });
        }
        for (const quote of ['"', "'"]) {
            const name = quote === '"' ? "double quoted greater-than" : "single quoted greater-than";
            const runtimePath = `/runtime/tag-state-${encodeURIComponent(name)}.js`;
            await runCase({
                name: `tag state ${name}`,
                classification: "simple profile",
                source: `<div id=target data-x=${quote}a>b${quote}><script id=runtime src="${origin}${runtimePath}"></script>`,
                expression: `(() => ({
                    value: document.querySelector("#target")?.getAttribute("data-x") ?? null,
                    runtimePresent: Boolean(document.querySelector("#runtime")),
                    executed: (window.__oracleRuntimeLoads ?? []).includes(${JSON.stringify(runtimePath)}),
                }))()`,
                validate(result, caseRequests) {
                    assert(result.value === "a>b", `${name} value = ${JSON.stringify(result.value)}`);
                    assert(result.runtimePresent && result.executed && caseRequests.includes(runtimePath), `${name} did not preserve the following runtime request`);
                },
            });
        }
        await runCase({
            name: "tag state compact solidus",
            classification: "simple profile",
            source: `<input id=boolean disabled/><input id=unquoted value=x/><input id=quoted value="x"/>`,
            expression: `(() => ({
                booleanDisabled: document.querySelector("#boolean")?.disabled ?? null,
                booleanAttributeCount: document.querySelector("#boolean")?.attributes.length ?? null,
                unquotedValue: document.querySelector("#unquoted")?.getAttribute("value") ?? null,
                quotedValue: document.querySelector("#quoted")?.getAttribute("value") ?? null,
                inputCount: document.querySelectorAll("input").length,
            }))()`,
            validate(result) {
                assert(result.booleanDisabled && result.booleanAttributeCount === 2, "compact boolean solidus behavior changed");
                assert(result.unquotedValue === "x/", `unquoted solidus value = ${JSON.stringify(result.unquotedValue)}`);
                assert(result.quotedValue === "x" && result.inputCount === 3, "quoted solidus behavior changed");
            },
        });
        await runCase({
            name: "select runtime",
            classification: "unsupported markerless rewrite",
            source: `<select><svg><script id="runtime" src="${origin}/runtime/select.js"></script></svg></select>`,
            expression: `(() => {
                const runtime = document.querySelector("#runtime");
                return {
                    present: Boolean(runtime),
                    namespace: runtime?.namespaceURI ?? null,
                    parent: runtime?.parentElement?.localName ?? null,
                    executed: (window.__oracleRuntimeLoads ?? []).includes("/runtime/select.js"),
                };
            })()`,
            validate(result, caseRequests) {
                result.requested = caseRequests.includes("/runtime/select.js");
                assert(result.present, "select runtime element was not created in Chrome");
            },
        });
        await runCase({
            name: "select in table runtime",
            classification: "unsupported markerless rewrite",
            source: `<table id="table"><select><svg><script id="runtime" src="${origin}/runtime/select-table.js"></script></svg></select></table>`,
            expression: `(() => {
                const runtime = document.querySelector("#runtime");
                return {
                    present: Boolean(runtime),
                    namespace: runtime?.namespaceURI ?? null,
                    parent: runtime?.parentElement?.localName ?? null,
                    insideTable: Boolean(runtime?.closest("table")),
                    tableParent: document.querySelector("#table")?.parentElement?.localName ?? null,
                    executed: (window.__oracleRuntimeLoads ?? []).includes("/runtime/select-table.js"),
                };
            })()`,
            validate(result, caseRequests) {
                result.requested = caseRequests.includes("/runtime/select-table.js");
                assert(result.present, "select-in-table runtime element was not created in Chrome");
            },
        });
        await runCase({
            name: "table foster parenting",
            classification: "managed-only profile",
            source: `<div id="container"><table id="table"><div id="foster">authored</div><tbody><tr><td>cell</td></tr></tbody></table></div>`,
            expression: `(() => {
                const foster = document.querySelector("#foster");
                const table = document.querySelector("#table");
                return {
                    parent: foster?.parentElement?.id ?? null,
                    beforeTable: foster?.nextElementSibling === table,
                    insideTable: Boolean(foster?.closest("table")),
                };
            })()`,
            validate(result) {
                assert(result.parent === "container" && result.beforeTable && !result.insideTable, "table foster-parenting oracle changed");
            },
        });
        await runCase({
            name: "ordinary template",
            classification: "simple profile",
            source: `<template id="ordinary"><link rel="stylesheet" href="${origin}/style/ordinary.css"><script src="${origin}/runtime/ordinary.js"></script></template>`,
            expression: `(() => {
                const template = document.querySelector("#ordinary");
                return {
                    templatePresent: Boolean(template),
                    linkInContent: Boolean(template?.content.querySelector("link")),
                    scriptInContent: Boolean(template?.content.querySelector("script")),
                    executed: (window.__oracleRuntimeLoads ?? []).length !== 0,
                };
            })()`,
            validate(result, caseRequests) {
                assert(result.templatePresent && result.linkInContent && result.scriptInContent, "ordinary template content was not inert template content");
                assert(!result.executed && !caseRequests.some((path) => path.startsWith("/runtime/") || path.startsWith("/style/")), "ordinary template loaded an inert resource");
            },
        });
        for (const mode of ["open", "closed"]) {
            await runCase({
                name: `declarative shadow ${mode}`,
                classification: "unsupported markerless rewrite",
                source: `<host-element id="host"><template shadowrootmode="${mode}"><link rel="stylesheet" href="${origin}/style/shadow-${mode}.css"></template></host-element>`,
                expression: `(() => ({
                    templatePresent: Boolean(document.querySelector("#host > template")),
                    openShadowRoot: Boolean(document.querySelector("#host")?.shadowRoot),
                }))()`,
                validate(result, caseRequests) {
                    assert(!result.templatePresent, `declarative shadow ${mode} template was not consumed`);
                    assert(result.openShadowRoot === (mode === "open"), `declarative shadow ${mode} visibility changed`);
                    assert(caseRequests.includes(`/style/shadow-${mode}.css`), `declarative shadow ${mode} stylesheet was not requested`);
                },
            });
        }
        await runCase({
            name: "invalid shadowrootmode",
            classification: "simple profile",
            source: `<host-element id="host"><template shadowrootmode=" open "><link rel="stylesheet" href="${origin}/style/shadow-invalid.css"></template></host-element>`,
            expression: `(() => ({
                templatePresent: Boolean(document.querySelector("#host > template")),
                openShadowRoot: Boolean(document.querySelector("#host")?.shadowRoot),
            }))()`,
            validate(result, caseRequests) {
                assert(result.templatePresent && !result.openShadowRoot, "invalid shadowrootmode created a shadow root");
                assert(!caseRequests.includes("/style/shadow-invalid.css"), "invalid shadowrootmode loaded inert template style");
            },
        });
        await runCase({
            name: "multiple declarative templates",
            classification: "managed-only profile",
            source: `<host-element id="host"><template shadowrootmode="open"><span id="first"></span></template><template shadowrootmode="open"><span id="second"></span></template></host-element>`,
            expression: `(() => ({
                firstInShadow: Boolean(document.querySelector("#host")?.shadowRoot?.querySelector("#first")),
                secondTemplatePresent: Boolean(document.querySelector("#host > template")),
                secondInShadow: Boolean(document.querySelector("#host")?.shadowRoot?.querySelector("#second")),
            }))()`,
            validate(result) {
                assert(result.firstInShadow && result.secondTemplatePresent && !result.secondInShadow, "multiple declarative-template behavior changed");
            },
        });
        await runCase({
            name: "frameset",
            classification: "managed-only profile",
            source: `<!doctype html><html><head></head><frameset id="frames"><frame id="frame" src="about:blank"></frameset></html>`,
            expression: `(() => ({
                framesetPresent: Boolean(document.querySelector("#frames")),
                frameParent: document.querySelector("#frame")?.parentElement?.localName ?? null,
                bodyPresent: Boolean(document.body),
            }))()`,
            validate(result) {
                assert(result.framesetPresent && result.frameParent === "frameset", "frameset oracle changed");
            },
        });
        await runCase({
            name: "noscript with scripting enabled",
            classification: "managed-only profile",
            source: `<noscript><script id="runtime" src="${origin}/runtime/noscript.js"></script></noscript><p id="after">after</p>`,
            expression: `(() => ({
                runtimeElementPresent: Boolean(document.querySelector("#runtime")),
                afterPresent: Boolean(document.querySelector("#after")),
                executed: (window.__oracleRuntimeLoads ?? []).length !== 0,
            }))()`,
            validate(result, caseRequests) {
                assert(!result.runtimeElementPresent && result.afterPresent && !result.executed, "noscript oracle changed");
                assert(!caseRequests.includes("/runtime/noscript.js"), "noscript loaded an inert runtime");
            },
        });
        await runCase({
            name: "active base href",
            classification: "managed-only profile",
            source: `<base href="https://example.test/nested/"><a id="asset" href="wasm_exec.js">asset</a>`,
            expression: `(() => ({
                raw: document.querySelector("#asset")?.getAttribute("href") ?? null,
                resolved: document.querySelector("#asset")?.href ?? null,
            }))()`,
            validate(result) {
                assert(result.raw === "wasm_exec.js" && result.resolved === "https://example.test/nested/wasm_exec.js", "active base URL resolution changed");
            },
        });
        await runCase({
            name: "bogus comments",
            classification: "simple profile",
            source: `<div id="pi"><?x "><span id="after-pi"></span>"></div><div id="declaration"><!unknown "><span id="after-declaration"></span>"></div><div id="cdata"><![CDATA["><span id="after-cdata"></span>"]></div>`,
            expression: `(() => ({
                afterPIParent: document.querySelector("#after-pi")?.parentElement?.id ?? null,
                afterDeclarationParent: document.querySelector("#after-declaration")?.parentElement?.id ?? null,
                afterCDATAParent: document.querySelector("#after-cdata")?.parentElement?.id ?? null,
            }))()`,
            validate(result) {
                assert(result.afterPIParent === "pi" && result.afterDeclarationParent === "declaration" && result.afterCDATAParent === "cdata", "bogus-comment boundary changed");
            },
        });
        await runCase({
            name: "abrupt doctype termination",
            classification: "simple profile",
            source: `<!DOCTYPE html PUBLIC "x><script id="runtime" src="${origin}/runtime/doctype.js"></script>">`,
            expression: `(() => ({
                runtimePresent: Boolean(document.querySelector("#runtime")),
                runtimeNamespace: document.querySelector("#runtime")?.namespaceURI ?? null,
                executed: (window.__oracleRuntimeLoads ?? []).includes("/runtime/doctype.js"),
            }))()`,
            validate(result, caseRequests) {
                assert(result.runtimePresent && result.runtimeNamespace === "http://www.w3.org/1999/xhtml", "abrupt DOCTYPE did not expose the following HTML script");
                assert(result.executed && caseRequests.includes("/runtime/doctype.js"), "abrupt DOCTYPE runtime was not requested");
            },
        });
        await runCase({
            name: "foreign runtime src",
            classification: "unsupported managed placement",
            source: `<svg><script id="foreign-runtime" src="${origin}/runtime/foreign-managed.js"></script></svg>`,
            expression: `(() => ({
                namespace: document.querySelector("#foreign-runtime")?.namespaceURI ?? null,
                executed: (window.__oracleRuntimeLoads ?? []).includes("/runtime/foreign-managed.js"),
            }))()`,
            validate(result, caseRequests) {
                assert(result.namespace === "http://www.w3.org/2000/svg", "foreign managed runtime would not create an SVG script");
                result.requested = caseRequests.includes("/runtime/foreign-managed.js");
            },
        });
        for (const test of [
            {
                name: "blocking runtime before bootstrap",
                runtime: `<script src="${origin}/ordered-runtime/blocking.js?fixture=ignored"></script>`,
                beforeRuntime: "",
                want: ["runtime", "bootstrap:true"],
            },
            {
                name: "bootstrap before blocking runtime",
                runtime: `<script src="${origin}/ordered-runtime/reversed.js"></script>`,
                beforeRuntime: `<script>window.__oracleOrder.push("bootstrap:" + Boolean(window.__oracleRuntimeReady));</script>`,
                want: ["bootstrap:false", "runtime"],
            },
            {
                name: "async runtime before bootstrap",
                runtime: `<script async src="${origin}/ordered-runtime/async.js"></script>`,
                beforeRuntime: "",
                want: ["bootstrap:false", "runtime"],
            },
            {
                name: "defer runtime before bootstrap",
                runtime: `<script defer src="${origin}/ordered-runtime/defer.js"></script>`,
                beforeRuntime: "",
                want: ["bootstrap:false", "runtime"],
            },
            {
                name: "module runtime before bootstrap",
                runtime: `<script type="module" src="${origin}/ordered-runtime/module.js"></script>`,
                beforeRuntime: "",
                want: ["bootstrap:false", "runtime"],
            },
        ]) {
            const bootstrap = test.beforeRuntime === "" ? `<script>window.__oracleOrder.push("bootstrap:" + Boolean(window.__oracleRuntimeReady));</script>` : "";
            await runCase({
                name: test.name,
                classification: test.want[0] === "runtime" ? "proven runtime order" : "unproven runtime order",
                source: `<script>window.__oracleOrder = [];</script>${test.beforeRuntime}${test.runtime}${bootstrap}`,
                expression: `window.__oracleOrder`,
                validate(result) {
                    assertDeepEqual(result, test.want, `${test.name} execution order`);
                },
            });
        }
        const schedulesBeforeUnknown = orderedRuntimeSchedules.length;
        const unknownResponse = await fetch(
            `${origin}/ordered-runtime/unknown.js?delay=999999`,
        );
        assert(unknownResponse.status === 404, "unknown ordered runtime resource was not rejected");
        await unknownResponse.text();
        assert(
            orderedRuntimeSchedules.length === schedulesBeforeUnknown,
            "unknown ordered runtime resource scheduled work",
        );
        assertDeepEqual(
            orderedRuntimeSchedules.map(({ resource }) => resource).sort(),
            [...orderedRuntimeResources].sort(),
            "ordered runtime scheduled resources",
        );
        assert(
            orderedRuntimeSchedules.every(({ delayMS }) => delayMS === orderedRuntimeDelayMS),
            "ordered runtime delay was not fixture-owned",
        );
        assert(
            orderedRuntimeSchedules.find(({ resource }) => resource === "blocking.js")?.query ===
                "?fixture=ignored",
            "irrelevant ordered runtime query was not preserved as inert input",
        );
        results.push({
            name: "ordered runtime fixture boundary",
            classification: "fixture-owned timing",
            delayMS: orderedRuntimeDelayMS,
            resources: orderedRuntimeSchedules.map(({ resource }) => resource).sort(),
            unknownStatus: unknownResponse.status,
            unknownScheduledTimers: orderedRuntimeSchedules.length - schedulesBeforeUnknown,
        });
        await runCase({
            name: "balanced foreign content",
            classification: "simple profile",
            source: `<svg id="svg"><g><script id="foreign-script"></script></g><foreignObject><p id="html-child"></p></foreignObject></svg>`,
            expression: `(() => ({
                scriptNamespace: document.querySelector("#foreign-script")?.namespaceURI ?? null,
                htmlChildNamespace: document.querySelector("#html-child")?.namespaceURI ?? null,
                htmlChildParent: document.querySelector("#html-child")?.parentElement?.localName ?? null,
            }))()`,
            validate(result) {
                assert(result.scriptNamespace === "http://www.w3.org/2000/svg", "balanced foreign script namespace changed");
                assert(result.htmlChildNamespace === "http://www.w3.org/1999/xhtml" && result.htmlChildParent === "foreignObject", "foreign integration point changed");
            },
        });
        await runCase({
            name: "misnested closing tags",
            classification: "managed-only profile",
            source: `<div id="outer"><span id="span"></div><p id="after"></p>`,
            expression: `(() => ({
                spanParent: document.querySelector("#span")?.parentElement?.id ?? null,
                afterInsideOuter: Boolean(document.querySelector("#after")?.closest("#outer")),
            }))()`,
            validate(result) {
                assert(result.spanParent === "outer" && !result.afterInsideOuter, "misnested-close recovery changed");
            },
        });
    } finally {
        await new Promise((resolveClose) => oracleServer.close(resolveClose));
    }
    return results;

    async function runCase(test) {
        const path = `/case/${encodeURIComponent(test.name)}`;
        pages.set(path, `<!doctype html><meta charset="utf-8">${test.source}`);
        const requestStart = requests.length;
        const url = `${origin}${path}?run=${Date.now()}`;
        await client.call("Page.navigate", { url });
        await waitForOracleDocument(url);
        const result = await client.evaluate(test.expression);
        const caseRequests = [...new Set(requests.slice(requestStart))].sort();
        test.validate(result, caseRequests);
        results.push({
            name: test.name,
            classification: test.classification,
            ...result,
            requests: caseRequests.filter((request) => request !== path && request !== "/favicon.ico"),
        });
    }
}

async function waitForOracleDocument(expectedURL) {
    for (let attempt = 0; attempt < 100; attempt++) {
        const state = await client.evaluate(`({ href: location.href, readyState: document.readyState })`);
        if (state.href === expectedURL && state.readyState === "complete") {
            await wait(25);
            return;
        }
        await wait(25);
    }
    throw new Error(`HARNESS FAILURE: semantic oracle did not load ${expectedURL}`);
}

function encodeDoubleQuotedAttribute(value) {
    return value.replaceAll("&", "&amp;").replaceAll('"', "&quot;");
}

function encodeUnquotedAttribute(value) {
    const replacements = new Map([
        ["&", "&amp;"],
        [" ", "&#32;"],
        ["\t", "&#9;"],
        ["\n", "&#10;"],
        ["\r", "&#13;"],
        ["\f", "&#12;"],
        ['"', "&quot;"],
        ["'", "&#39;"],
        ["`", "&#96;"],
        ["=", "&#61;"],
        ["<", "&lt;"],
        [">", "&gt;"],
    ]);
    return Array.from(value, (character) => replacements.get(character) ?? character).join("");
}

function encodePackagePathAsBrowserURL(value) {
    const unreserved = (byte) =>
        (byte >= 0x41 && byte <= 0x5a) ||
        (byte >= 0x61 && byte <= 0x7a) ||
        (byte >= 0x30 && byte <= 0x39) ||
        byte === 0x2d || byte === 0x2e || byte === 0x5f || byte === 0x7e;
    return value.split("/").map((segment) => {
        let encoded = "";
        for (const byte of Buffer.from(segment, "utf8")) {
            assert(byte !== 0, "package URL oracle received a NUL byte");
            encoded += unreserved(byte) ? String.fromCharCode(byte) : `%${byte.toString(16).toUpperCase().padStart(2, "0")}`;
        }
        return encoded;
    }).join("/");
}

function decodeURLPathname(pathname) {
    try {
        return decodeURIComponent(pathname);
    } catch {
        return pathname;
    }
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

async function runCommand(command, args, environment) {
    const result = await runCommandResult(command, args, environment);
    if (result.code === 0) return result.output;
    throw new Error(
        `HARNESS FAILURE: ${command} ${args.join(" ")} failed with ${result.signal ?? result.code}\n${result.output}`,
    );
}

function runCommandResult(command, args, environment) {
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
            resolveCommand({ code, signal, output });
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

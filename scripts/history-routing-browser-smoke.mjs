import { spawn } from "node:child_process";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "node:net";

if (typeof WebSocket === "undefined") {
    throw new Error("WebSocket is unavailable; run Node with --experimental-websocket");
}

const rootDir = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const fixtureDir = join(rootDir, "scripts", "fixtures", "history-routing");
const fixturePackageDir = join(fixtureDir, ".goframe", "package", "standalone");
const routerDir = join(rootDir, "examples", "router");
const routerPackageDir = join(routerDir, ".goframe", "package", "standalone");
const chrome = process.env.CHROME ?? "google-chrome";
const tempRoot = await mkdtemp(join(tmpdir(), "goframe-history-routing-"));
const profile = await mkdtemp(join(tmpdir(), "goframe-history-routing-chrome-"));
const debugPort = Number(process.env.GOFRAME_HISTORY_ROUTING_CHROME_DEBUG_PORT ?? await pickFreePort());
const historyServer = join(tempRoot, "history-routing-server");
const counters = {
    pushNavigations: 0,
    replaceNavigations: 0,
    popstateEvents: 0,
    directDeepLinkBoots: 0,
    refreshRecoveries: 0,
    applicationNotFoundRenders: 0,
    htmlFallbackResponses: 0,
    staticFileResponses: 0,
    missingAsset404Responses: 0,
    api404Responses: 0,
    incorrectFallbackResponses: 0,
    routerMounts: 0,
    routerUnmounts: 0,
    listenerAdditions: 0,
    listenerRemovals: 0,
    appRootIdentityChanges: 0,
    routeTargetMismatches: 0,
};

let browser = null;
let browserError = "";
let client = null;

try {
    const goxc = await prepareTools();
    await runCommand(goxc, ["package", fixtureDir, "--compiler=go"]);
    await runCommand(goxc, ["package", routerDir, "--compiler=go"]);

    const assetManifest = JSON.parse(await readFile(join(fixturePackageDir, "asset-manifest.json"), "utf8"));
    const wasmPath = assetManifest.entrypoints?.wasm;
    const stylePath = assetManifest.entrypoints?.styles?.[0];
    if (!wasmPath || !stylePath) {
        throw new Error(`HARNESS FAILURE: fixture package entrypoints are incomplete: ${JSON.stringify(assetManifest.entrypoints)}`);
    }

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

    await runHashBaseline(client, goxc);
    await runStrictPathPhase(client);
    await runRootFallbackPhase(client, wasmPath, stylePath);
    await runSubpathFallbackPhase(client, wasmPath, stylePath);

    assert(counters.routeTargetMismatches === 0, "route-target mismatches must remain zero");
    assert(counters.incorrectFallbackResponses === 0, "incorrect fallback responses must remain zero");
    assert(counters.appRootIdentityChanges === 0, "app-root identity changes must remain zero");
    assert(counters.listenerAdditions === counters.listenerRemovals, "listener additions and removals must balance after teardown");

    console.log(`history routing behavioral counters: ${JSON.stringify(counters)}`);
    console.log("history routing deployment pressure: ok");
} finally {
    client?.close();
    await stopProcess(browser, false);
    await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
    await rm(tempRoot, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
    await rm(join(fixtureDir, ".goframe"), { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
    await rm(join(routerDir, ".goframe"), { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function prepareTools() {
    await runCommand("go", ["build", "-o", historyServer, "./scripts/fixtures/history-routing/cmd/server"]);
    if (process.env.GOXC) {
        return process.env.GOXC;
    }
    const goxc = join(tempRoot, "goxc");
    await runCommand("go", ["build", "-o", goxc, "./cmd/goxc"]);
    return goxc;
}

async function runHashBaseline(browserClient, goxc) {
    const port = await pickFreePort();
    const server = await startServer(
        goxc,
        ["serve", `--dir=${routerPackageDir}`, `--port=${port}`],
        `http://127.0.0.1:${port}/`,
        "hash-router baseline",
    );
    const origin = `http://127.0.0.1:${port}`;
    try {
        const root = await fetch(`${origin}/`);
        const hash = await fetch(`${origin}/#/issues/1`);
        const path = await fetch(`${origin}/users/42`);
        assert(root.status === 200, `hash baseline root returned ${root.status}`);
        assert(hash.status === 200, `hash baseline hash URL returned ${hash.status}`);
        assert(path.status === 404, `hash baseline path URL returned ${path.status}`);

        await navigate(browserClient, `${origin}/`);
        await waitForHashRoute(browserClient, "home", "");

        await navigate(browserClient, `${origin}/#/issues/1`);
        await waitForHashRoute(browserClient, "details", "#/issues/1");
        await browserClient.call("Page.reload", { ignoreCache: true });
        await waitForHashRoute(browserClient, "details", "#/issues/1");

        await navigate(browserClient, `${origin}/users/42`);
        await waitForNoApp(browserClient, "router-shell");
        console.log("phase 1 hash-router baseline: root/hash/refresh 200; clean path 404");
    } finally {
        await stopProcess(server.child, true);
    }
}

async function runStrictPathPhase(browserClient) {
    const port = await pickFreePort();
    const origin = `http://127.0.0.1:${port}`;
    const server = await startFixtureServer(port, "strict", "/", `${origin}/`);
    try {
        await navigate(browserClient, `${origin}/`);
        await waitForHistoryRoute(browserClient, "home", "/");
        await rememberAppRoot(browserClient);

        await click(browserClient, "history-user-link");
        await waitForHistoryRoute(browserClient, "user", "/users/42");
        counters.pushNavigations++;
        await assertHistoryState(browserClient, {
            pathname: "/users/42",
            search: "",
            param: "42",
            basePath: "/",
            appSame: true,
        }, "strict client push");

        const response = await fetch(`${origin}/users/42`, { headers: { Accept: "text/html" } });
        assert(response.status === 404, `strict deep route returned ${response.status}`);
        await browserClient.call("Page.reload", { ignoreCache: true });
        await waitForNoApp(browserClient, "history-app");
        console.log("phase 2 strict server: client push works; direct deep route and refresh remain 404");
    } finally {
        await stopProcess(server.child, true);
    }
}

async function runRootFallbackPhase(browserClient, wasmPath, stylePath) {
    const port = await pickFreePort();
    const origin = `http://127.0.0.1:${port}`;
    const server = await startFixtureServer(port, "fallback", "/", `${origin}/`);
    try {
        await navigate(browserClient, `${origin}/users/42?tab=activity`);
        await waitForHistoryRoute(browserClient, "user", "/users/42?tab=activity");
        counters.directDeepLinkBoots++;
        await assertHistoryState(browserClient, {
            pathname: "/users/42",
            search: "?tab=activity",
            param: "42",
            query: "tab=activity",
            queryValue: "activity",
            basePath: "/",
        }, "root direct deep link");

        await browserClient.call("Page.reload", { ignoreCache: true });
        await waitForHistoryRoute(browserClient, "user", "/users/42?tab=activity");
        counters.refreshRecoveries++;
        await rememberAppRoot(browserClient);

        await click(browserClient, "history-search-link");
        await waitForHistoryRoute(browserClient, "search", "/search?q=goframe");
        counters.pushNavigations++;
        await assertHistoryState(browserClient, {
            pathname: "/search",
            search: "?q=goframe",
            query: "q=goframe",
            queryValue: "goframe",
            basePath: "/",
            appSame: true,
        }, "root push navigation");

        const historyLength = await browserClient.evaluate("history.length");
        await click(browserClient, "history-settings-replace");
        await waitForHistoryRoute(browserClient, "settings", "/settings/");
        counters.replaceNavigations++;
        await assertHistoryState(browserClient, {
            pathname: "/settings/",
            search: "",
            basePath: "/",
            appSame: true,
        }, "root replace navigation");
        assert(await browserClient.evaluate("history.length") === historyLength, "replaceState created a history entry");

        await browserClient.evaluate("history.back()");
        await waitForHistoryRoute(browserClient, "user", "/users/42?tab=activity");
        counters.popstateEvents++;
        await assertHistoryState(browserClient, {
            pathname: "/users/42",
            search: "?tab=activity",
            queryValue: "activity",
            appSame: true,
        }, "root Back");

        await browserClient.evaluate("history.forward()");
        await waitForHistoryRoute(browserClient, "settings", "/settings/");
        counters.popstateEvents++;
        await assertHistoryState(browserClient, {
            pathname: "/settings/",
            appSame: true,
        }, "root Forward");

        await click(browserClient, "history-not-found-link");
        await waitForHistoryRoute(browserClient, "not-found", "/does-not-exist");
        counters.pushNavigations++;
        counters.applicationNotFoundRenders++;
        await assertHistoryState(browserClient, {
            pathname: "/does-not-exist",
            notFound: true,
            appSame: true,
        }, "root application not-found");

        await runHTTPContractProbes(origin, "/", wasmPath, stylePath);
        console.log("phases 3-4 root fallback: direct/refresh/navigation/history and bounded HTTP fallback passed");
    } finally {
        await stopProcess(server.child, true);
    }
}

async function runSubpathFallbackPhase(browserClient, wasmPath, stylePath) {
    const port = await pickFreePort();
    const origin = `http://127.0.0.1:${port}`;
    const server = await startFixtureServer(port, "fallback", "/app/", `${origin}/app/`);
    try {
        await navigate(browserClient, `${origin}/app/users/42?tab=activity`);
        await waitForHistoryRoute(browserClient, "user", "/users/42?tab=activity");
        counters.directDeepLinkBoots++;
        await assertHistoryState(browserClient, {
            pathname: "/app/users/42",
            search: "?tab=activity",
            param: "42",
            queryValue: "activity",
            basePath: "/app/",
            documentBasePath: "/app/",
            stylesheetPath: `/app/${stylePath}`,
        }, "subpath direct deep link");

        await browserClient.call("Page.reload", { ignoreCache: true });
        await waitForHistoryRoute(browserClient, "user", "/users/42?tab=activity");
        counters.refreshRecoveries++;
        await rememberAppRoot(browserClient);

        await click(browserClient, "history-search-link");
        await waitForHistoryRoute(browserClient, "search", "/search?q=goframe");
        counters.pushNavigations++;
        await assertHistoryState(browserClient, {
            pathname: "/app/search",
            search: "?q=goframe",
            queryValue: "goframe",
            basePath: "/app/",
            appSame: true,
        }, "subpath push navigation");

        const historyLength = await browserClient.evaluate("history.length");
        await click(browserClient, "history-settings-replace");
        await waitForHistoryRoute(browserClient, "settings", "/settings/");
        counters.replaceNavigations++;
        await assertHistoryState(browserClient, {
            pathname: "/app/settings/",
            basePath: "/app/",
            appSame: true,
        }, "subpath replace navigation");
        assert(await browserClient.evaluate("history.length") === historyLength, "subpath replaceState created a history entry");

        await browserClient.evaluate("history.back()");
        await waitForHistoryRoute(browserClient, "user", "/users/42?tab=activity");
        counters.popstateEvents++;
        await assertHistoryState(browserClient, {
            pathname: "/app/users/42",
            search: "?tab=activity",
            appSame: true,
        }, "subpath Back");

        await browserClient.evaluate("history.forward()");
        await waitForHistoryRoute(browserClient, "settings", "/settings/");
        counters.popstateEvents++;
        await assertHistoryState(browserClient, {
            pathname: "/app/settings/",
            appSame: true,
        }, "subpath Forward");

        await runSubpathHTTPProbes(origin, wasmPath, stylePath);
        await verifyListenerCleanup(browserClient);
        console.log("phases 5-6 subpath fallback and listener cleanup: passed");
    } finally {
        await stopProcess(server.child, true);
    }
}

async function runHTTPContractProbes(origin, base, wasmPath, stylePath) {
    await probeResponse(origin, "/users/42", {
        status: 200,
        className: "fallback",
        containsIndex: true,
        base,
    });
    for (const path of [
        "/assets/missing.js",
        "/assets/missing.css",
        "/assets/missing.wasm",
        "/missing.js",
    ]) {
        await probeResponse(origin, path, {
            status: 404,
            className: "not-found",
            containsIndex: false,
            category: "missing-asset",
        });
    }
    await probeResponse(origin, "/api/missing", {
        status: 404,
        className: "not-found",
        containsIndex: false,
        category: "api",
    });
    await probeResponse(origin, `/${wasmPath}`, {
        status: 200,
        className: "static",
        contentType: "application/wasm",
        nonEmpty: true,
    });
    await probeResponse(origin, `/${stylePath}`, {
        status: 200,
        className: "static",
        contentTypePrefix: "text/css",
        nonEmpty: true,
    });
}

async function runSubpathHTTPProbes(origin, wasmPath, stylePath) {
    await probeResponse(origin, "/app/users/42", {
        status: 200,
        className: "fallback",
        containsIndex: true,
        base: "/app/",
    });
    await probeResponse(origin, `/app/${wasmPath}`, {
        status: 200,
        className: "static",
        contentType: "application/wasm",
        nonEmpty: true,
    });
    await probeResponse(origin, `/app/${stylePath}`, {
        status: 200,
        className: "static",
        contentTypePrefix: "text/css",
        nonEmpty: true,
    });
    await probeResponse(origin, "/app/assets/missing.js", {
        status: 404,
        className: "not-found",
        containsIndex: false,
        category: "missing-asset",
    });
    await probeResponse(origin, "/app/api/missing", {
        status: 404,
        className: "not-found",
        containsIndex: false,
        category: "api",
    });
    await probeResponse(origin, "/users/42", {
        status: 404,
        className: "not-found",
        containsIndex: false,
    });
}

async function probeResponse(origin, path, expected) {
    const response = await fetch(`${origin}${path}`, {
        headers: { Accept: "text/html,application/xhtml+xml" },
        redirect: "manual",
    });
    const content = new Uint8Array(await response.arrayBuffer());
    const text = new TextDecoder().decode(content);
    const className = response.headers.get("x-goframe-history-response") ?? "";
    const contentType = response.headers.get("content-type") ?? "";
    const correct =
        response.status === expected.status &&
        className === expected.className &&
        (!expected.containsIndex || text.includes("GoFrame history routing pressure fixture")) &&
        (expected.containsIndex !== false || !text.includes("GoFrame history routing pressure fixture")) &&
        (!expected.base || text.includes(`<base href="${expected.base}" />`)) &&
        (!expected.contentType || contentType === expected.contentType) &&
        (!expected.contentTypePrefix || contentType.startsWith(expected.contentTypePrefix)) &&
        (!expected.nonEmpty || content.length > 0);
    if (!correct) {
        counters.incorrectFallbackResponses++;
        throw new Error(`HARNESS FAILURE: HTTP probe ${path}: ${JSON.stringify({
            status: response.status,
            className,
            contentType,
            bodyPrefix: text.slice(0, 160),
            expected,
        })}`);
    }

    if (className === "fallback") {
        counters.htmlFallbackResponses++;
    } else if (className === "static") {
        counters.staticFileResponses++;
    }
    if (expected.category === "missing-asset") {
        counters.missingAsset404Responses++;
    } else if (expected.category === "api") {
        counters.api404Responses++;
    }
}

async function verifyListenerCleanup(browserClient) {
    const before = await historyState(browserClient);
    assert(before.routerMounts === 1, `router mount count before teardown = ${before.routerMounts}`);
    assert(before.listenerAdds === 1, `listener add count before teardown = ${before.listenerAdds}`);

    await click(browserClient, "history-router-unmount");
    await waitForCondition(async () => {
        const state = await historyState(browserClient);
        return state.routerInactive && state.routerUnmounts === 1 && state.listenerRemovals === 1;
    }, "router listener cleanup");

    const removed = await historyState(browserClient);
    const popstatesBefore = removed.popstates;
    const originalLocation = removed.pathname + removed.search;
    await browserClient.evaluate(`history.pushState(null, "", "/app/users/99"); history.back()`);
    await waitForCondition(async () => {
        const state = await historyState(browserClient);
        return state.pathname + state.search === originalLocation;
    }, "history activity after router teardown");
    await wait(100);
    const after = await historyState(browserClient);
    assert(after.routerInactive, "router owner remounted after teardown");
    assert(after.popstates === popstatesBefore, `inactive router observed popstate: ${after.popstates} != ${popstatesBefore}`);

    counters.routerMounts = after.routerMounts;
    counters.routerUnmounts = after.routerUnmounts;
    counters.listenerAdditions = after.listenerAdds;
    counters.listenerRemovals = after.listenerRemovals;
}

async function assertHistoryState(browserClient, expected, label) {
    const actual = await historyState(browserClient);
    const targetMatches = actual.target === (expected.target ?? actual.pathname.replace(/^\/app(?=\/)/, "") + actual.search);
    if (!targetMatches) {
        counters.routeTargetMismatches++;
    }
    if (expected.appSame === true && !actual.appSame) {
        counters.appRootIdentityChanges++;
    }
    const fields = { ...expected };
    delete fields.target;
    for (const [key, value] of Object.entries(fields)) {
        if (actual[key] !== value) {
            throw new Error(`APP FAILURE: ${label}: ${key} got ${JSON.stringify(actual[key])}, want ${JSON.stringify(value)}; state=${JSON.stringify(actual)}`);
        }
    }
    if (!targetMatches) {
        throw new Error(`APP FAILURE: ${label}: displayed target ${JSON.stringify(actual.target)} does not match URL ${JSON.stringify(actual.pathname + actual.search)}`);
    }
    console.log(`${label}: ok`);
}

async function rememberAppRoot(browserClient) {
    await browserClient.evaluate(`window.__historyPressureApp = document.querySelector("[data-testid='history-app']")`);
}

async function click(browserClient, testID) {
    await browserClient.evaluate(`document.querySelector("[data-testid='${testID}']").click()`);
}

async function waitForHistoryRoute(browserClient, route, target) {
    await waitForCondition(async () => {
        const state = await historyState(browserClient);
        return state.app && state.route === route && state.target === target && state.listenerAdds === 1;
    }, `history route ${route} ${target}`);
}

async function historyState(browserClient) {
    return await browserClient.evaluate(`(() => {
        const text = (id) => document.querySelector("[data-testid='" + id + "']")?.textContent.trim() ?? "";
        const number = (id) => Number(text(id) || "0");
        const base = document.querySelector("base");
        const stylesheet = [...document.querySelectorAll("link[rel='stylesheet']")][0];
        return {
            app: Boolean(document.querySelector("[data-testid='history-app']")),
            routerInactive: Boolean(document.querySelector("[data-testid='history-router-inactive']")),
            route: text("history-route-name"),
            target: text("history-route-target"),
            query: text("history-route-query"),
            param: text("history-route-param"),
            queryValue: text("history-route-query-value"),
            basePath: text("history-base-path"),
            notFound: Boolean(document.querySelector("[data-testid='history-not-found']")),
            pathname: location.pathname,
            search: location.search,
            documentBasePath: base ? new URL(base.href).pathname : "",
            stylesheetPath: stylesheet ? new URL(stylesheet.href).pathname : "",
            appSame: window.__historyPressureApp
                ? window.__historyPressureApp === document.querySelector("[data-testid='history-app']")
                : null,
            pushes: number("history-push-count"),
            replaces: number("history-replace-count"),
            popstates: number("history-popstate-count"),
            routerMounts: number("history-router-mount-count"),
            routerUnmounts: number("history-router-unmount-count"),
            listenerAdds: number("history-listener-add-count"),
            listenerRemovals: number("history-listener-remove-count"),
        };
    })()`);
}

async function waitForHashRoute(browserClient, route, hash) {
    await waitForCondition(async () => {
        const state = await browserClient.evaluate(`(() => ({
            hash: location.hash,
            home: Boolean(document.querySelector("[data-testid='router-home']")),
            details: Boolean(document.querySelector("[data-testid='router-issue-details']")),
        }))()`);
        const actualRoute = state.home ? "home" : state.details ? "details" : "missing";
        return actualRoute === route && state.hash === hash;
    }, `hash route ${route} ${hash}`);
}

async function waitForNoApp(browserClient, testID) {
    await waitForCondition(async () => {
        return await browserClient.evaluate(`document.readyState === "complete" && !document.querySelector("[data-testid='${testID}']")`);
    }, `document without ${testID}`);
}

async function startFixtureServer(port, mode, base, readyURL) {
    return await startServer(
        historyServer,
        [
            `--package=${fixturePackageDir}`,
            `--addr=127.0.0.1:${port}`,
            `--mode=${mode}`,
            `--base=${base}`,
        ],
        readyURL,
        `history fixture ${mode} ${base}`,
    );
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
            throw new Error(`HARNESS FAILURE: ${label} exited before HTTP was available\n${output()}`);
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
    throw new Error(`HARNESS FAILURE: ${label} did not become ready: ${lastError?.message ?? ""}\n${output()}`);
}

async function navigate(browserClient, url) {
    await browserClient.call("Page.navigate", { url });
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
            if (page) {
                return page;
            }
        } catch (error) {
            lastError = error;
        }
        await wait(100);
    }
    throw new Error(`HARNESS FAILURE: Chrome DevTools did not become ready: ${lastError?.message ?? ""}\n${browserError}`);
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

async function waitForCondition(check, label) {
    let lastError = null;
    for (let attempt = 0; attempt < 200; attempt++) {
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

function runCommand(command, args) {
    return new Promise((resolveCommand, reject) => {
        const child = spawn(command, args, {
            cwd: rootDir,
            stdio: "inherit",
            env: {
                ...process.env,
                GOWORK: "off",
            },
        });
        child.on("error", reject);
        child.on("exit", (code, signal) => {
            if (code === 0) {
                resolveCommand();
                return;
            }
            reject(new Error(`${command} ${args.join(" ")} failed with ${signal ?? code}`));
        });
    });
}

async function stopProcess(child, processGroup) {
    if (!child || child.exitCode !== null || child.signalCode !== null) {
        return;
    }
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

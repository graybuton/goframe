#!/usr/bin/env node

import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { cpSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, posix, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const rootDir = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const goxc = process.env.GOXC;
const applications = [
    "counter",
    "components",
    "todo",
    "dashboard",
    "context",
    "virtualized",
    "multipackage",
    "cmdapp",
    "router",
    "router-dashboard",
    "resource",
];

if (!goxc) {
    throw new Error("package graph inspection: GOXC must name the goxc executable");
}

const reports = new Map();
for (const application of applications) {
    const appPath = `examples/${application}`;
    const first = inspect([appPath, "--format=json"]);
    const second = inspect([appPath, "--format=json"]);
    if (first !== second) {
        throw new Error(`package graph inspection: ${application} JSON is not byte-identical`);
    }

    validateReport(application, JSON.parse(first));
    reports.set(application, first);
    console.log(`package graph inspection: ${application}: ${digest(first)}`);
}

const counterRoot = join(
    rootDir,
    "examples/counter/.goframe/package/standalone",
);
const counterByDir = inspect(["--dir", counterRoot, "--format=json"]);
if (counterByDir !== reports.get("counter")) {
    throw new Error("package graph inspection: app and --dir reports differ");
}

const tempRoot = mkdtempSync(join(tmpdir(), "goframe-package-graph-"));
try {
    const copiedRoot = join(tempRoot, "standalone");
    cpSync(counterRoot, copiedRoot, { recursive: true });
    const copied = inspect(["--dir", copiedRoot, "--format=json"]);
    if (copied !== reports.get("counter")) {
        throw new Error("package graph inspection: copied package report differs");
    }
} finally {
    rmSync(tempRoot, { recursive: true, force: true });
}

const combined = createHash("sha256");
for (const application of applications) {
    combined.update(application);
    combined.update("\0");
    combined.update(reports.get(application));
}
console.log(`package graph inspection: combined: ${combined.digest("hex")}`);
console.log("package graph inspection: ok");

function inspect(args) {
    const result = spawnSync(goxc, ["inspect", ...args], {
        cwd: rootDir,
        encoding: "utf8",
        env: process.env,
        maxBuffer: 16 * 1024 * 1024,
    });
    if (result.error) {
        throw result.error;
    }
    if (result.status !== 0) {
        throw new Error(
            `package graph inspection: ${goxc} inspect ${args.join(" ")} failed ` +
            `with ${result.signal ?? result.status}\n${result.stderr}`,
        );
    }
    if (result.stderr !== "") {
        throw new Error(
            `package graph inspection: inspect wrote stderr for ${args.join(" ")}\n` +
            result.stderr,
        );
    }
    return result.stdout;
}

function validateReport(application, report) {
    assertKeys(report, [
        "schemaVersion",
        "package",
        "entrypoints",
        "artifacts",
        "edges",
        "summary",
    ], `${application} report`);
    assert(report.schemaVersion === 1, `${application} schemaVersion must be 1`);

    assertKeys(report.package, [
        "name",
        "compiler",
        "toolchainVersion",
        "hashAssets",
        "preload",
    ], `${application} package`);
    assertString(report.package.name, `${application} package.name`);
    assertString(report.package.compiler, `${application} package.compiler`);
    assertString(report.package.toolchainVersion, `${application} package.toolchainVersion`);
    assertBoolean(report.package.hashAssets, `${application} package.hashAssets`);
    assertBoolean(report.package.preload, `${application} package.preload`);

    assertKeys(report.entrypoints, ["html", "wasm", "runtime", "styles"], `${application} entrypoints`);
    for (const name of ["html", "wasm", "runtime"]) {
        assertSafePath(report.entrypoints[name], `${application} entrypoints.${name}`);
    }
    assertStringArray(report.entrypoints.styles, `${application} entrypoints.styles`);
    assertSortedUnique(report.entrypoints.styles, `${application} entrypoints.styles`);
    for (const style of report.entrypoints.styles) {
        assertSafePath(style, `${application} style entrypoint`);
    }

    assert(Array.isArray(report.artifacts), `${application} artifacts must be an array`);
    const artifactPaths = new Set();
    let totalBytes = 0;
    let previousPath = "";
    for (const artifact of report.artifacts) {
        assertKeys(artifact, [
            "path",
            "logicalName",
            "mediaType",
            "bytes",
            "sha256",
            "declaredHash",
            "encoding",
            "roles",
        ], `${application} artifact`);
        assertSafePath(artifact.path, `${application} artifact.path`);
        assert(!artifactPaths.has(artifact.path), `${application} duplicate artifact ${artifact.path}`);
        assert(previousPath < artifact.path, `${application} artifacts are not path-sorted`);
        previousPath = artifact.path;
        artifactPaths.add(artifact.path);
        assertString(artifact.logicalName, `${application} artifact.logicalName`, true);
        assertString(artifact.mediaType, `${application} artifact.mediaType`);
        assert(Number.isSafeInteger(artifact.bytes) && artifact.bytes >= 0,
            `${application} artifact.bytes must be a non-negative integer`);
        assert(/^[0-9a-f]{64}$/.test(artifact.sha256),
            `${application} artifact.sha256 must be lowercase SHA-256`);
        assertString(artifact.declaredHash, `${application} artifact.declaredHash`, true);
        assert(
            artifact.declaredHash === "" || /^[0-9a-f]{8}$/.test(artifact.declaredHash),
            `${application} artifact.declaredHash must be empty or lowercase 8-hex`,
        );
        assertString(artifact.encoding, `${application} artifact.encoding`, true);
        assertStringArray(artifact.roles, `${application} artifact.roles`);
        assertSortedUnique(artifact.roles, `${application} artifact.roles`);
        totalBytes += artifact.bytes;
    }

    for (const entrypoint of [
        report.entrypoints.html,
        report.entrypoints.wasm,
        report.entrypoints.runtime,
        ...report.entrypoints.styles,
    ]) {
        assert(artifactPaths.has(entrypoint),
            `${application} entrypoint ${entrypoint} does not resolve to an artifact`);
    }

    assert(Array.isArray(report.edges), `${application} edges must be an array`);
    let previousEdge = "";
    for (const edge of report.edges) {
        assertKeys(edge, ["from", "to", "kind", "encoding"], `${application} edge`);
        assertSafePath(edge.from, `${application} edge.from`);
        assertSafePath(edge.to, `${application} edge.to`);
        assertString(edge.kind, `${application} edge.kind`);
        assertString(edge.encoding, `${application} edge.encoding`, true);
        assert(artifactPaths.has(edge.from), `${application} edge source ${edge.from} is absent`);
        assert(artifactPaths.has(edge.to), `${application} edge target ${edge.to} is absent`);
        const edgeKey = `${edge.from}\0${edge.kind}\0${edge.encoding}\0${edge.to}`;
        assert(previousEdge < edgeKey, `${application} edges are not deterministically sorted`);
        previousEdge = edgeKey;
    }

    assertKeys(report.summary, ["artifactCount", "edgeCount", "totalBytes"], `${application} summary`);
    assert(report.summary.artifactCount === report.artifacts.length,
        `${application} artifact count does not match`);
    assert(report.summary.edgeCount === report.edges.length,
        `${application} edge count does not match`);
    assert(report.summary.totalBytes === totalBytes,
        `${application} total bytes does not match`);
}

function assertKeys(value, expected, description) {
    assert(value !== null && typeof value === "object" && !Array.isArray(value),
        `${description} must be an object`);
    const actual = Object.keys(value).sort();
    const wanted = [...expected].sort();
    assert(JSON.stringify(actual) === JSON.stringify(wanted),
        `${description} keys differ: ${actual.join(", ")}`);
}

function assertSafePath(value, description) {
    assertString(value, description);
    assert(!value.startsWith("/") && !/^[A-Za-z]:/.test(value),
        `${description} must be relative`);
    assert(!value.includes("\\"), `${description} must use slash separators`);
    assert(value !== "." && posix.normalize(value) === value,
        `${description} must be normalized`);
    assert(!value.split("/").includes(".."), `${description} must remain contained`);
}

function assertString(value, description, allowEmpty = false) {
    assert(typeof value === "string", `${description} must be a string`);
    assert(allowEmpty || value.length > 0, `${description} must not be empty`);
}

function assertBoolean(value, description) {
    assert(typeof value === "boolean", `${description} must be a boolean`);
}

function assertStringArray(value, description) {
    assert(Array.isArray(value), `${description} must be an array`);
    for (const item of value) {
        assertString(item, `${description} item`);
    }
}

function assertSortedUnique(values, description) {
    const sorted = [...new Set(values)].sort();
    assert(JSON.stringify(values) === JSON.stringify(sorted),
        `${description} must be sorted and unique`);
}

function assert(condition, message) {
    if (!condition) {
        throw new Error(`package graph inspection: ${message}`);
    }
}

function digest(value) {
    return createHash("sha256").update(value).digest("hex");
}

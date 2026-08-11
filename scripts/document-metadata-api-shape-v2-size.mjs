import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { cp, mkdir, mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const rootDir = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const fixtureRelative = "scripts/fixtures/document-metadata-api-shape-v2";
const fixtureSource = join(rootDir, fixtureRelative);
const goToolchain = process.env.GOTOOLCHAIN ?? "go1.26.5";
const candidates = ["control", "hook", "component", "handle"];
const compilers = ["go", "tinygo"];
const symbols = [
    "documentMetadataCoordinator",
    "DocumentMetadataHandoffExperiment",
    "UseDocumentMetadata",
    "DocumentMetadataComponent",
    "UseDocumentMetadataOwner",
    "UseOwnedDocumentMetadata",
];

const tempRoot = await mkdtemp(join(tmpdir(), "goframe-document-api-shape-v2-size-"));
try {
    const workspace = join(tempRoot, "workspace");
    await mkdir(workspace, { recursive: true });
    for (const name of ["go.mod", "cmd", "pkg"]) {
        await cp(join(rootDir, name), join(workspace, name), { recursive: true });
    }
    const fixture = join(workspace, fixtureRelative);
    await mkdir(dirname(fixture), { recursive: true });
    await cp(fixtureSource, fixture, { recursive: true });

    const environment = {
        ...process.env,
        GOCACHE: join(tempRoot, "gocache"),
        GOTOOLCHAIN: goToolchain,
        GOWORK: "off",
        GOFLAGS: "-buildvcs=false",
        XDG_CACHE_HOME: join(tempRoot, "xdg-cache"),
    };
    await run("go", [
        "run",
        "./cmd/goxc",
        "generate",
        fixtureRelative,
        "--in-place",
    ], workspace, environment);

    const measurements = {};
    for (const compiler of compilers) {
        measurements[compiler] = {};
        for (const candidate of candidates) {
            const artifact = join(tempRoot, "matched", "candidate.wasm");
            await mkdir(dirname(artifact), { recursive: true });
            const entry = `./${fixtureRelative}/cmd/size-${candidate}`;
            if (compiler === "go") {
                await run("go", [
                    "build",
                    "-buildvcs=false",
                    "-trimpath",
                    "-ldflags=-buildid=",
                    "-tags=goframe_document_state_experiment",
                    "-o",
                    artifact,
                    entry,
                ], workspace, {
                    ...environment,
                    GOOS: "js",
                    GOARCH: "wasm",
                    CGO_ENABLED: "0",
                });
            } else {
                await run("tinygo", [
                    "build",
                    "-target=wasm",
                    "-no-debug",
                    "-panic=trap",
                    "-tags=goframe_document_state_experiment",
                    "-o",
                    artifact,
                    entry,
                ], workspace, environment);
            }
            const bytes = await readFile(artifact);
            measurements[compiler][candidate] = await measure(bytes);
        }
        const control = measurements[compiler].control;
        for (const candidate of candidates) {
            measurements[compiler][candidate].delta = Object.fromEntries(
                ["raw", "gzip", "brotli", "zstandard", "code", "data", "name"]
                    .map((key) => [key, measurements[compiler][candidate][key] - control[key]]),
            );
        }
    }
    console.log(JSON.stringify({
        goToolchain,
        tinygo: await runCapture("tinygo", ["version"], workspace, environment),
        measurements,
    }));
} finally {
    await rm(tempRoot, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 });
}

async function measure(bytes) {
    const compressed = await Promise.all([
        runBinary("gzip", ["-n", "-9", "-c"], bytes),
        runBinary("brotli", ["-q", "11", "-c"], bytes),
        runBinary("zstd", ["-q", "-19", "-c"], bytes),
    ]);
    const sections = wasmSections(bytes);
    return {
        raw: bytes.length,
        gzip: compressed[0].length,
        brotli: compressed[1].length,
        zstandard: compressed[2].length,
        code: sections.code,
        data: sections.data,
        name: sections.name,
        sha256: digest(bytes),
        gzipSHA256: digest(compressed[0]),
        brotliSHA256: digest(compressed[1]),
        zstandardSHA256: digest(compressed[2]),
        symbols: Object.fromEntries(symbols.map((symbol) => [
            symbol,
            bytes.includes(Buffer.from(symbol)),
        ])),
    };
}

function wasmSections(bytes) {
    let offset = 8;
    const result = { code: 0, data: 0, name: 0 };
    while (offset < bytes.length) {
        const id = bytes[offset++];
        const sizeValue = readULEB(bytes, offset);
        offset = sizeValue.next;
        const start = offset;
        const end = start + sizeValue.value;
        if (end > bytes.length) throw new Error("invalid WASM section length");
        if (id === 10) result.code += sizeValue.value;
        if (id === 11) result.data += sizeValue.value;
        if (id === 0) {
            const nameLength = readULEB(bytes, start);
            const nameStart = nameLength.next;
            const nameEnd = nameStart + nameLength.value;
            if (bytes.subarray(nameStart, nameEnd).toString() === "name") {
                result.name += sizeValue.value;
            }
        }
        offset = end;
    }
    return result;
}

function readULEB(bytes, start) {
    let value = 0;
    let shift = 0;
    let offset = start;
    while (offset < bytes.length) {
        const byte = bytes[offset++];
        value |= (byte & 0x7f) << shift;
        if ((byte & 0x80) === 0) return { value, next: offset };
        shift += 7;
        if (shift > 35) throw new Error("invalid ULEB128 value");
    }
    throw new Error("truncated ULEB128 value");
}

function digest(bytes) {
    return createHash("sha256").update(bytes).digest("hex");
}

function run(command, args, cwd, env) {
    return runProcess(command, args, cwd, env, null).then(() => undefined);
}

function runCapture(command, args, cwd, env) {
    return runProcess(command, args, cwd, env, null).then((output) =>
        output.stdout.toString().trim()
    );
}

function runBinary(command, args, input) {
    return runProcess(command, args, rootDir, process.env, input).then(
        (output) => output.stdout,
    );
}

function runProcess(command, args, cwd, env, input) {
    return new Promise((resolveProcess, reject) => {
        const child = spawn(command, args, {
            cwd,
            env,
            stdio: ["pipe", "pipe", "pipe"],
        });
        const stdout = [];
        const stderr = [];
        child.stdout.on("data", (chunk) => stdout.push(chunk));
        child.stderr.on("data", (chunk) => stderr.push(chunk));
        child.once("error", reject);
        child.once("exit", (code, signal) => {
            const output = {
                stdout: Buffer.concat(stdout),
                stderr: Buffer.concat(stderr),
            };
            if (code === 0) {
                resolveProcess(output);
                return;
            }
            reject(new Error(
                `${command} ${args.join(" ")} failed with ${signal ?? code}\n${output.stderr}`,
            ));
        });
        child.stdin.end(input ?? undefined);
    });
}

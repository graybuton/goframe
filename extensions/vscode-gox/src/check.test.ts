import assert = require("node:assert/strict");
import * as path from "node:path";
import test = require("node:test");

import {
  CheckDiagnostic,
  RunGenerationCoordinator,
  buildCheckInvocation,
  diagnosticRange,
  groupDiagnosticsByFile,
  interpretCheckProcessResult,
  parseCheckReport,
  planWorkspaceDiagnosticRemoval,
  planWorkspaceDiagnosticUpdate,
  resourcePathRelation,
} from "./check";

const workspacePath = path.resolve("workspace with spaces");
const firstFile = path.join(workspacePath, "first.gox");
const secondFile = path.join(workspacePath, "nested", "second.gox");

function sourceBytes(source: string): Uint8Array {
  return Buffer.from(source, "utf8");
}

function diagnostic(file = firstFile): CheckDiagnostic {
  return {
    file,
    line: 4,
    column: 15,
    severity: "error",
    message: "gox: source diagnostic",
    source: "<main>{}</main>",
  };
}

function cleanJSON(): string {
  return JSON.stringify({
    schemaVersion: 1,
    ok: true,
    filesChecked: 2,
    diagnostics: [],
  });
}

function diagnosticJSON(): string {
  return JSON.stringify({
    schemaVersion: 1,
    ok: false,
    filesChecked: 2,
    diagnostics: [diagnostic()],
  });
}

test("parses a clean schema-v1 report", () => {
  const report = parseCheckReport(`\n${cleanJSON()}\n`);
  assert.equal(report.schemaVersion, 1);
  assert.equal(report.ok, true);
  assert.equal(report.filesChecked, 2);
  assert.deepEqual(report.diagnostics, []);
});

test("parses a diagnostic schema-v1 report and permits unknown fields", () => {
  const value = JSON.parse(diagnosticJSON());
  value.transportNote = "compatible";
  value.diagnostics[0].futureField = 1;
  const report = parseCheckReport(JSON.stringify(value));
  assert.equal(report.ok, false);
  assert.deepEqual(report.diagnostics, [diagnostic()]);
});

test("rejects an unsupported schema version", () => {
  const value = JSON.parse(cleanJSON());
  value.schemaVersion = 2;
  assert.throws(() => parseCheckReport(JSON.stringify(value)), /unsupported schemaVersion 2/);
});

test("rejects malformed JSON and extra output", () => {
  assert.throws(() => parseCheckReport(""), /invalid JSON report/);
  assert.throws(() => parseCheckReport("notice\n{}"), /invalid JSON report/);
  assert.throws(() => parseCheckReport(`${cleanJSON()}\nnotice`), /invalid JSON report/);
});

test("rejects missing and wrong-type required fields", () => {
  const cases: Array<[string, (value: Record<string, unknown>) => void, RegExp]> = [
    ["schemaVersion", (value) => delete value.schemaVersion, /schemaVersion must be an integer/],
    ["ok", (value) => { value.ok = "true"; }, /ok must be a boolean/],
    [
      "filesChecked",
      (value) => { value.filesChecked = -1; },
      /filesChecked must be a non-negative integer/,
    ],
    ["diagnostics", (value) => { value.diagnostics = {}; }, /diagnostics must be an array/],
  ];
  for (const [name, mutate, expected] of cases) {
    const value = JSON.parse(cleanJSON()) as Record<string, unknown>;
    mutate(value);
    assert.throws(() => parseCheckReport(JSON.stringify(value)), expected, name);
  }
});

test("validates every diagnostic field", () => {
  const cases: Array<[string, unknown, RegExp]> = [
    ["file", "relative.gox", /file must be a non-empty absolute path/],
    ["line", -1, /line must be a non-negative integer/],
    ["column", 1.5, /column must be a non-negative integer/],
    ["severity", "warning", /severity must be "error"/],
    ["message", "", /message must be a non-empty string/],
    ["source", 1, /source must be a string/],
  ];
  for (const [field, invalid, expected] of cases) {
    const value = JSON.parse(diagnosticJSON());
    value.diagnostics[0][field] = invalid;
    assert.throws(() => parseCheckReport(JSON.stringify(value)), expected, field);
  }
});

test("rejects reports with inconsistent ok and diagnostics", () => {
  const clean = JSON.parse(cleanJSON());
  clean.ok = false;
  assert.throws(() => parseCheckReport(JSON.stringify(clean)), /ok must be true/);

  const failed = JSON.parse(diagnosticJSON());
  failed.ok = true;
  assert.throws(() => parseCheckReport(JSON.stringify(failed)), /ok must be true/);
});

test("interprets exit 0 with a clean report as completed", () => {
  const result = interpretCheckProcessResult({
    exitCode: 0,
    signal: null,
    stdout: cleanJSON(),
    stderr: "",
  });
  assert.equal(result.kind, "completed");
  if (result.kind === "completed") {
    assert.equal(result.report.ok, true);
  }
});

test("interprets exit 1 with diagnostics as completed", () => {
  const result = interpretCheckProcessResult({
    exitCode: 1,
    signal: null,
    stdout: diagnosticJSON(),
    stderr: "  \n",
  });
  assert.equal(result.kind, "completed");
  if (result.kind === "completed") {
    assert.equal(result.report.diagnostics.length, 1);
  }
});

test("treats exit 1 without a valid report as a tooling error", () => {
  const result = interpretCheckProcessResult({
    exitCode: 1,
    signal: null,
    stdout: "",
    stderr: "goxc: no .gox files found",
  });
  assert.equal(result.kind, "toolingError");
});

test("rejects process/report mismatches and other exit codes", () => {
  for (const [exitCode, stdout] of [[0, diagnosticJSON()], [1, cleanJSON()], [2, cleanJSON()]] as const) {
    const result = interpretCheckProcessResult({
      exitCode,
      signal: null,
      stdout,
      stderr: "",
    });
    assert.equal(result.kind, "toolingError");
  }
});

test("rejects stderr pollution for a completed JSON result", () => {
  const result = interpretCheckProcessResult({
    exitCode: 0,
    signal: null,
    stdout: cleanJSON(),
    stderr: "warning",
  });
  assert.deepEqual(result, {
    kind: "toolingError",
    error: "completed JSON report included non-empty stderr",
  });
});

test("classifies launch and signal failures as tooling errors", () => {
  const launch = interpretCheckProcessResult({
    exitCode: null,
    signal: null,
    stdout: "",
    stderr: "",
    launchError: new Error("ENOENT"),
  });
  assert.equal(launch.kind, "toolingError");
  const signal = interpretCheckProcessResult({
    exitCode: null,
    signal: "SIGTERM",
    stdout: cleanJSON(),
    stderr: "",
  });
  assert.equal(signal.kind, "toolingError");
});

test("groups diagnostics by absolute file path", () => {
  const grouped = groupDiagnosticsByFile([
    diagnostic(firstFile),
    diagnostic(secondFile),
    { ...diagnostic(firstFile), line: 8 },
  ]);
  assert.deepEqual([...grouped.keys()], [firstFile, secondFile]);
  assert.equal(grouped.get(firstFile)?.length, 2);
  assert.equal(grouped.get(secondFile)?.length, 1);
});

test("maps an ASCII byte column to a UTF-16 editor range", () => {
  assert.deepEqual(diagnosticRange(sourceBytes("first\nabcdefghijklmnop"), 2, 15), {
    startLine: 1,
    startCharacter: 14,
    endLine: 1,
    endCharacter: 15,
  });
});

test("maps a byte column after a two-byte UTF-8 character", () => {
  assert.deepEqual(diagnosticRange(sourceBytes("<p>é{}</p>"), 1, 6), {
    startLine: 0,
    startCharacter: 4,
    endLine: 0,
    endCharacter: 5,
  });
});

test("maps a byte column after a four-byte UTF-8 character", () => {
  assert.deepEqual(diagnosticRange(sourceBytes("<p>🙂{}</p>"), 1, 8), {
    startLine: 0,
    startCharacter: 5,
    endLine: 0,
    endCharacter: 6,
  });
});

test("maps indentation and mixed non-ASCII prefixes", () => {
  assert.deepEqual(diagnosticRange(sourceBytes("header\n    é🙂{}"), 2, 11), {
    startLine: 1,
    startCharacter: 7,
    endLine: 1,
    endCharacter: 8,
  });
});

test("maps CRLF source without treating carriage return as editor text", () => {
  assert.deepEqual(diagnosticRange(sourceBytes("first\r\n<p>é{}</p>\r\n"), 2, 6), {
    startLine: 1,
    startCharacter: 4,
    endLine: 1,
    endCharacter: 5,
  });
});

test("uses a zero-width range at the end of a source line", () => {
  assert.deepEqual(diagnosticRange(sourceBytes("é"), 1, 3), {
    startLine: 0,
    startCharacter: 1,
    endLine: 0,
    endCharacter: 1,
  });
});

test("clamps a byte column beyond the source line to its end", () => {
  assert.deepEqual(diagnosticRange(sourceBytes("éx"), 1, 99), {
    startLine: 0,
    startCharacter: 2,
    endLine: 0,
    endCharacter: 2,
  });
});

test("maps a byte column inside a UTF-8 sequence to a file-level range", () => {
  const fileRange = {
    startLine: 0,
    startCharacter: 0,
    endLine: 0,
    endCharacter: 0,
  };
  assert.deepEqual(diagnosticRange(sourceBytes("éx"), 1, 2), fileRange);
});

test("maps a missing source line to a file-level range", () => {
  assert.deepEqual(diagnosticRange(sourceBytes("one line"), 2, 1), {
    startLine: 0,
    startCharacter: 0,
    endLine: 0,
    endCharacter: 0,
  });
});

test("maps unknown or partial locations to a file-level range", () => {
  const fileRange = {
    startLine: 0,
    startCharacter: 0,
    endLine: 0,
    endCharacter: 0,
  };
  const source = sourceBytes("<main />");
  assert.deepEqual(diagnosticRange(source, 0, 0), fileRange);
  assert.deepEqual(diagnosticRange(source, 4, 0), fileRange);
  assert.deepEqual(diagnosticRange(undefined, 1, 1), fileRange);
});

test("covers one complete supplementary code point at the diagnostic start", () => {
  assert.deepEqual(diagnosticRange(sourceBytes("🙂x"), 1, 1), {
    startLine: 0,
    startCharacter: 0,
    endLine: 0,
    endCharacter: 2,
  });
});

test("maps multiple diagnostics from the same saved source bytes", () => {
  const source = sourceBytes("éx\n🙂y");
  assert.deepEqual(diagnosticRange(source, 1, 3), {
    startLine: 0,
    startCharacter: 1,
    endLine: 0,
    endCharacter: 2,
  });
  assert.deepEqual(diagnosticRange(source, 2, 5), {
    startLine: 1,
    startCharacter: 2,
    endLine: 1,
    endCharacter: 3,
  });
});

test("later runs invalidate earlier generations", () => {
  const coordinator = new RunGenerationCoordinator();
  const first = coordinator.begin("workspace-a");
  const second = coordinator.begin("workspace-a");
  assert.equal(coordinator.isCurrent("workspace-a", first), false);
  assert.equal(coordinator.isCurrent("workspace-a", second), true);
});

test("stale completion is rejected after explicit invalidation", () => {
  const coordinator = new RunGenerationCoordinator();
  const run = coordinator.begin("workspace-a");
  coordinator.invalidate("workspace-a");
  assert.equal(coordinator.isCurrent("workspace-a", run), false);
});

test("plans stale-key removal for one workspace", () => {
  const ownership = new Map<string, ReadonlySet<string>>([
    ["workspace-a", new Set(["file-a", "stale-a"])],
  ]);
  const plan = planWorkspaceDiagnosticUpdate(ownership, "workspace-a", ["file-a", "new-a"]);
  assert.deepEqual(plan.staleKeys, ["stale-a"]);
  assert.deepEqual(plan.nextKeys, ["file-a", "new-a"]);
  assert.deepEqual([...plan.ownership.get("workspace-a") ?? []].sort(), ["file-a", "new-a"]);
});

test("workspace update leaves another workspace ownership untouched", () => {
  const ownership = new Map<string, ReadonlySet<string>>([
    ["workspace-a", new Set(["file-a"])],
    ["workspace-b", new Set(["file-b"])],
  ]);
  const plan = planWorkspaceDiagnosticUpdate(ownership, "workspace-a", []);
  assert.deepEqual([...plan.ownership.get("workspace-b") ?? []], ["file-b"]);
  assert.deepEqual([...ownership.get("workspace-a") ?? []], ["file-a"]);
});

test("plans exact owned URI removal", () => {
  const ownership = new Map<string, ReadonlySet<string>>([
    ["workspace-a", new Set(["file-a", "file-b"])],
  ]);
  const plan = planWorkspaceDiagnosticRemoval(ownership, "workspace-a", ["file-a"]);
  assert.deepEqual(plan.removedKeys, ["file-a"]);
  assert.deepEqual(plan.nextKeys, ["file-b"]);
});

test("unknown URI removal is a no-op", () => {
  const ownership = new Map<string, ReadonlySet<string>>([
    ["workspace-a", new Set(["file-a"])],
  ]);
  const plan = planWorkspaceDiagnosticRemoval(ownership, "workspace-a", ["unknown"]);
  assert.deepEqual(plan.removedKeys, []);
  assert.deepEqual(plan.nextKeys, ["file-a"]);
  assert.deepEqual([...plan.ownership.get("workspace-a") ?? []], ["file-a"]);
});

test("plans multiple owned URI removals", () => {
  const ownership = new Map<string, ReadonlySet<string>>([
    ["workspace-a", new Set(["file-a", "file-b", "file-c"])],
  ]);
  const plan = planWorkspaceDiagnosticRemoval(
    ownership,
    "workspace-a",
    ["file-c", "file-a"],
  );
  assert.deepEqual(plan.removedKeys, ["file-a", "file-c"]);
  assert.deepEqual(plan.nextKeys, ["file-b"]);
});

test("removing the last owned URI leaves an empty set", () => {
  const ownership = new Map<string, ReadonlySet<string>>([
    ["workspace-a", new Set(["file-a"])],
  ]);
  const plan = planWorkspaceDiagnosticRemoval(ownership, "workspace-a", ["file-a"]);
  assert.deepEqual(plan.nextKeys, []);
  assert.deepEqual([...plan.ownership.get("workspace-a") ?? []], []);
});

test("workspace removal planning leaves another workspace untouched", () => {
  const ownership = new Map<string, ReadonlySet<string>>([
    ["workspace-a", new Set(["file-a"])],
    ["workspace-b", new Set(["file-b"])],
  ]);
  const plan = planWorkspaceDiagnosticRemoval(ownership, "workspace-a", ["file-a"]);
  assert.deepEqual([...plan.ownership.get("workspace-b") ?? []], ["file-b"]);
});

test("resource path relation recognizes directory descendants", () => {
  assert.equal(
    resourcePathRelation(
      { scheme: "file", fsPath: path.join(workspacePath, "removed") },
      { scheme: "file", fsPath: path.join(workspacePath, "removed", "nested", "app.gox") },
    ),
    "descendant",
  );
});

test("resource path relation rejects a prefix sibling", () => {
  assert.equal(
    resourcePathRelation(
      { scheme: "file", fsPath: path.join(workspacePath, "app") },
      { scheme: "file", fsPath: path.join(workspacePath, "application", "app.gox") },
    ),
    "outside",
  );
});

test("resource path relation recognizes exact file paths", () => {
  assert.equal(
    resourcePathRelation(
      { scheme: "file", fsPath: firstFile },
      { scheme: "file", fsPath: firstFile },
    ),
    "exact",
  );
});

test("resource path relation rejects outside paths and mismatched schemes", () => {
  const parent = { scheme: "file", fsPath: path.join(workspacePath, "app") };
  assert.equal(
    resourcePathRelation(parent, {
      scheme: "file",
      fsPath: path.join(workspacePath, "other", "app.gox"),
    }),
    "outside",
  );
  assert.equal(
    resourcePathRelation(parent, {
      scheme: "untitled",
      fsPath: path.join(workspacePath, "app", "app.gox"),
    }),
    "outside",
  );
});

test("workspace removal planning does not mutate its ownership input", () => {
  const workspaceA = new Set(["file-a", "file-b"]);
  const workspaceB = new Set(["file-c"]);
  const ownership = new Map<string, ReadonlySet<string>>([
    ["workspace-a", workspaceA],
    ["workspace-b", workspaceB],
  ]);
  const plan = planWorkspaceDiagnosticRemoval(ownership, "workspace-a", ["file-a"]);
  assert.deepEqual([...workspaceA], ["file-a", "file-b"]);
  assert.deepEqual([...workspaceB], ["file-c"]);
  assert.notEqual(plan.ownership.get("workspace-a"), workspaceA);
  assert.notEqual(plan.ownership.get("workspace-b"), workspaceB);
});

test("a renamed directory identifies multiple descendant diagnostic keys", () => {
  const removedDirectory = {
    scheme: "file",
    fsPath: path.join(workspacePath, "old"),
  };
  const resources = new Map([
    ["first", { scheme: "file", fsPath: path.join(workspacePath, "old", "first.gox") }],
    [
      "second",
      { scheme: "file", fsPath: path.join(workspacePath, "old", "nested", "second.gox") },
    ],
    ["other", { scheme: "file", fsPath: path.join(workspacePath, "other.gox") }],
  ]);
  const removedKeys = [...resources]
    .filter(([, candidate]) => resourcePathRelation(removedDirectory, candidate) === "descendant")
    .map(([key]) => key);
  const ownership = new Map<string, ReadonlySet<string>>([
    ["workspace-a", new Set(resources.keys())],
  ]);
  const plan = planWorkspaceDiagnosticRemoval(ownership, "workspace-a", removedKeys);
  assert.deepEqual(plan.removedKeys, ["first", "second"]);
  assert.deepEqual(plan.nextKeys, ["other"]);
});

test("resource path relation supports normalized Windows paths", () => {
  assert.equal(
    resourcePathRelation(
      { scheme: "file", fsPath: "C:/Work/App/." },
      { scheme: "file", fsPath: "c:/work/app/nested/../file.gox" },
      path.win32,
    ),
    "descendant",
  );
});

test("builds a non-shell invocation with the workspace path as one argument", () => {
  assert.deepEqual(buildCheckInvocation("/tools/goxc binary", workspacePath), {
    executable: "/tools/goxc binary",
    args: ["check", workspacePath, "--format=json"],
    cwd: workspacePath,
  });
});

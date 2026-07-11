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
  planWorkspaceDiagnosticUpdate,
} from "./check";

const workspacePath = path.resolve("workspace with spaces");
const firstFile = path.join(workspacePath, "first.gox");
const secondFile = path.join(workspacePath, "nested", "second.gox");

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

test("maps one-based locations to zero-based one-character ranges", () => {
  assert.deepEqual(diagnosticRange(4, 15), {
    startLine: 3,
    startCharacter: 14,
    endLine: 3,
    endCharacter: 15,
  });
});

test("maps unknown or partial locations to a file-level range", () => {
  const fileRange = {
    startLine: 0,
    startCharacter: 0,
    endLine: 0,
    endCharacter: 0,
  };
  assert.deepEqual(diagnosticRange(0, 0), fileRange);
  assert.deepEqual(diagnosticRange(4, 0), fileRange);
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

test("builds a non-shell invocation with the workspace path as one argument", () => {
  assert.deepEqual(buildCheckInvocation("/tools/goxc binary", workspacePath), {
    executable: "/tools/goxc binary",
    args: ["check", workspacePath, "--format=json"],
    cwd: workspacePath,
  });
});

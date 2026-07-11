/// <reference types="node" />

import * as path from "node:path";
import { TextDecoder } from "node:util";

export const checkSchemaVersion = 1;

export interface CheckDiagnostic {
  file: string;
  line: number;
  column: number;
  severity: "error";
  message: string;
  source: string;
}

export interface CheckReport {
  schemaVersion: 1;
  ok: boolean;
  filesChecked: number;
  diagnostics: CheckDiagnostic[];
}

export interface CheckProcessResult {
  exitCode: number | null;
  signal: string | null;
  stdout: string;
  stderr: string;
  launchError?: Error;
}

export type CheckProcessInterpretation =
  | { kind: "completed"; report: CheckReport }
  | { kind: "toolingError"; error: string };

export interface DiagnosticRange {
  startLine: number;
  startCharacter: number;
  endLine: number;
  endCharacter: number;
}

export interface CheckInvocation {
  executable: string;
  args: string[];
  cwd: string;
}

export interface WorkspaceDiagnosticUpdatePlan {
  staleKeys: string[];
  nextKeys: string[];
  ownership: Map<string, Set<string>>;
}

export function parseCheckReport(stdout: string): CheckReport {
  let parsed: unknown;
  try {
    parsed = JSON.parse(stdout);
  } catch (error) {
    throw new Error(`invalid JSON report: ${errorMessage(error)}`);
  }

  if (!isRecord(parsed)) {
    throw new Error("check report must be a JSON object");
  }

  const schemaVersion = parsed.schemaVersion;
  if (!Number.isInteger(schemaVersion)) {
    throw new Error("schemaVersion must be an integer");
  }
  if (schemaVersion !== checkSchemaVersion) {
    throw new Error(`unsupported schemaVersion ${String(schemaVersion)}`);
  }
  if (typeof parsed.ok !== "boolean") {
    throw new Error("ok must be a boolean");
  }
  if (!isNonNegativeInteger(parsed.filesChecked)) {
    throw new Error("filesChecked must be a non-negative integer");
  }
  if (!Array.isArray(parsed.diagnostics)) {
    throw new Error("diagnostics must be an array");
  }

  const diagnostics = parsed.diagnostics.map(parseDiagnostic);
  if (parsed.ok !== (diagnostics.length === 0)) {
    throw new Error("ok must be true only when diagnostics is empty");
  }

  return {
    schemaVersion: checkSchemaVersion,
    ok: parsed.ok,
    filesChecked: parsed.filesChecked,
    diagnostics,
  };
}

export function interpretCheckProcessResult(
  result: CheckProcessResult,
): CheckProcessInterpretation {
  if (result.launchError) {
    return {
      kind: "toolingError",
      error: `process launch failed: ${result.launchError.message}`,
    };
  }
  if (result.signal) {
    return {
      kind: "toolingError",
      error: `process terminated by signal ${result.signal}`,
    };
  }

  let report: CheckReport;
  try {
    report = parseCheckReport(result.stdout);
  } catch (error) {
    return { kind: "toolingError", error: errorMessage(error) };
  }

  if (result.stderr.trim() !== "") {
    return {
      kind: "toolingError",
      error: "completed JSON report included non-empty stderr",
    };
  }
  if (result.exitCode === 0 && report.ok && report.diagnostics.length === 0) {
    return { kind: "completed", report };
  }
  if (result.exitCode === 1 && !report.ok && report.diagnostics.length > 0) {
    return { kind: "completed", report };
  }
  if (result.exitCode !== 0 && result.exitCode !== 1) {
    return {
      kind: "toolingError",
      error: `unexpected process exit code ${String(result.exitCode)}`,
    };
  }
  return {
    kind: "toolingError",
    error: `exit code ${String(result.exitCode)} is inconsistent with the JSON report`,
  };
}

export function groupDiagnosticsByFile(
  diagnostics: readonly CheckDiagnostic[],
): Map<string, CheckDiagnostic[]> {
  const grouped = new Map<string, CheckDiagnostic[]>();
  for (const diagnostic of diagnostics) {
    const current = grouped.get(diagnostic.file);
    if (current) {
      current.push(diagnostic);
    } else {
      grouped.set(diagnostic.file, [diagnostic]);
    }
  }
  return grouped;
}

export function diagnosticRange(
  source: Uint8Array | undefined,
  line: number,
  column: number,
): DiagnosticRange {
  if (!source || line <= 0 || column <= 0) {
    return fileLevelDiagnosticRange();
  }

  const rawLine = sourceLineBytes(source, line);
  if (!rawLine) {
    return fileLevelDiagnosticRange();
  }

  const byteOffset = Math.min(column - 1, rawLine.length);
  try {
    const lineText = utf8Decoder.decode(rawLine);
    const startCharacter = utf8Decoder.decode(rawLine.subarray(0, byteOffset)).length;
    const codePoint = lineText.codePointAt(startCharacter);
    const width = byteOffset < rawLine.length && codePoint !== undefined
      ? codePoint > 0xffff ? 2 : 1
      : 0;
    return {
      startLine: line - 1,
      startCharacter,
      endLine: line - 1,
      endCharacter: startCharacter + width,
    };
  } catch {
    return fileLevelDiagnosticRange();
  }
}

export function buildCheckInvocation(executable: string, workspacePath: string): CheckInvocation {
  if (executable.length === 0) {
    throw new Error("goxc executable must not be empty");
  }
  return {
    executable,
    args: ["check", workspacePath, "--format=json"],
    cwd: workspacePath,
  };
}

export class RunGenerationCoordinator {
  private readonly generations = new Map<string, number>();

  begin(workspaceKey: string): number {
    const generation = (this.generations.get(workspaceKey) ?? 0) + 1;
    this.generations.set(workspaceKey, generation);
    return generation;
  }

  invalidate(workspaceKey: string): number {
    return this.begin(workspaceKey);
  }

  isCurrent(workspaceKey: string, generation: number): boolean {
    return this.generations.get(workspaceKey) === generation;
  }
}

export function planWorkspaceDiagnosticUpdate(
  currentOwnership: ReadonlyMap<string, ReadonlySet<string>>,
  workspaceKey: string,
  nextKeyValues: Iterable<string>,
): WorkspaceDiagnosticUpdatePlan {
  const nextKeys = new Set(nextKeyValues);
  const previousKeys = currentOwnership.get(workspaceKey) ?? new Set<string>();
  const staleKeys = [...previousKeys].filter((key) => !nextKeys.has(key)).sort();
  const ownership = new Map<string, Set<string>>();
  for (const [key, values] of currentOwnership) {
    ownership.set(key, new Set(values));
  }
  ownership.set(workspaceKey, nextKeys);
  return {
    staleKeys,
    nextKeys: [...nextKeys].sort(),
    ownership,
  };
}

function parseDiagnostic(value: unknown, index: number): CheckDiagnostic {
  if (!isRecord(value)) {
    throw new Error(`diagnostics[${index}] must be an object`);
  }
  if (typeof value.file !== "string" || value.file.length === 0 || !path.isAbsolute(value.file)) {
    throw new Error(`diagnostics[${index}].file must be a non-empty absolute path`);
  }
  if (!isNonNegativeInteger(value.line)) {
    throw new Error(`diagnostics[${index}].line must be a non-negative integer`);
  }
  if (!isNonNegativeInteger(value.column)) {
    throw new Error(`diagnostics[${index}].column must be a non-negative integer`);
  }
  if (value.severity !== "error") {
    throw new Error(`diagnostics[${index}].severity must be "error"`);
  }
  if (typeof value.message !== "string" || value.message.trim() === "") {
    throw new Error(`diagnostics[${index}].message must be a non-empty string`);
  }
  if (typeof value.source !== "string") {
    throw new Error(`diagnostics[${index}].source must be a string`);
  }
  return {
    file: value.file,
    line: value.line,
    column: value.column,
    severity: value.severity,
    message: value.message,
    source: value.source,
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isNonNegativeInteger(value: unknown): value is number {
  return Number.isInteger(value) && typeof value === "number" && value >= 0;
}

const utf8Decoder = new TextDecoder("utf-8", { fatal: true });

function sourceLineBytes(source: Uint8Array, line: number): Uint8Array | undefined {
  let currentLine = 1;
  let lineStart = 0;
  for (let index = 0; index < source.length; index++) {
    if (source[index] !== 0x0a) {
      continue;
    }
    if (currentLine === line) {
      const lineEnd = index > lineStart && source[index - 1] === 0x0d ? index - 1 : index;
      return source.subarray(lineStart, lineEnd);
    }
    currentLine++;
    lineStart = index + 1;
  }
  return currentLine === line ? source.subarray(lineStart) : undefined;
}

function fileLevelDiagnosticRange(): DiagnosticRange {
  return {
    startLine: 0,
    startCharacter: 0,
    endLine: 0,
    endCharacter: 0,
  };
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

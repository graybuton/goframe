import { ChildProcess, spawn } from "node:child_process";

import * as vscode from "vscode";

import {
  CheckInvocation,
  CheckProcessResult,
  CheckReport,
  RunGenerationCoordinator,
  buildCheckInvocation,
  diagnosticRange,
  groupDiagnosticsByFile,
  interpretCheckProcessResult,
  planWorkspaceDiagnosticRemoval,
  planWorkspaceDiagnosticUpdate,
  resourcePathRelation,
} from "./check";

type ProjectCommand = "generate" | "package" | "serve";
type CheckSource = "manual" | "save" | "rename";

interface WorkspaceRunState {
  child?: ChildProcess;
  ownedUris: Set<string>;
}

interface PathRemovalResult {
  matchedDescendant: boolean;
}

export function activate(context: vscode.ExtensionContext): void {
  const diagnostics = vscode.languages.createDiagnosticCollection("gox");
  const output = vscode.window.createOutputChannel("GOX Diagnostics");
  const checks = new CheckController(diagnostics, output);

  context.subscriptions.push(
    diagnostics,
    output,
    checks,
    vscode.commands.registerCommand("gox.generate", () => runProjectCommand("generate")),
    vscode.commands.registerCommand("gox.package", () => runProjectCommand("package")),
    vscode.commands.registerCommand("gox.serve", () => runProjectCommand("serve")),
    vscode.commands.registerCommand("gox.doctor", runDoctor),
    vscode.commands.registerCommand("gox.check", async () => {
      const workspace = currentWorkspace();
      if (!workspace) {
        await vscode.window.showWarningMessage(
          "GOX: Open a workspace folder before checking the current project.",
        );
        return;
      }
      await checks.run(workspace, "manual");
    }),
    vscode.workspace.onDidSaveTextDocument((document) => {
      if (!isAuthoredGOXDocument(document)) {
        return;
      }
      const workspace = vscode.workspace.getWorkspaceFolder(document.uri);
      if (workspace) {
        void checks.run(workspace, "save");
      }
    }),
    vscode.workspace.onDidDeleteFiles((event) => {
      checks.handleDeletedFiles(event.files);
    }),
    vscode.workspace.onDidRenameFiles((event) => {
      checks.handleRenamedFiles(event.files);
    }),
    vscode.workspace.onDidChangeWorkspaceFolders((event) => {
      for (const workspace of event.removed) {
        checks.removeWorkspace(workspace);
      }
    }),
  );
}

export function deactivate(): void {}

class CheckController implements vscode.Disposable {
  private readonly states = new Map<string, WorkspaceRunState>();
  private readonly generations = new RunGenerationCoordinator();

  constructor(
    private readonly diagnostics: vscode.DiagnosticCollection,
    private readonly output: vscode.OutputChannel,
  ) {}

  run(workspace: vscode.WorkspaceFolder, source: CheckSource): Promise<void> {
    if (!vscode.workspace.isTrusted) {
      if (source === "manual") {
        void vscode.window.showWarningMessage(
          "GOX checks require a trusted workspace because they execute the configured goxc executable.",
        );
      }
      return Promise.resolve();
    }

    const workspaceKey = workspace.uri.toString();
    const state = this.workspaceState(workspaceKey);
    const generation = this.generations.begin(workspaceKey);
    this.cancelChild(state);

    const executable = configuredGoxcExecutable(workspace);
    const fallbackInvocation: CheckInvocation = {
      executable,
      args: ["check", workspace.uri.fsPath, "--format=json"],
      cwd: workspace.uri.fsPath,
    };
    let invocation: CheckInvocation;
    try {
      invocation = buildCheckInvocation(executable, workspace.uri.fsPath);
    } catch (error) {
      const launchError = asError(error);
      const result: CheckProcessResult = {
        exitCode: null,
        signal: null,
        stdout: "",
        stderr: "",
        launchError,
      };
      this.handleToolingError(workspace, source, fallbackInvocation, result, launchError.message);
      return Promise.resolve();
    }

    return new Promise((resolve) => {
      let child: ChildProcess;
      try {
        child = spawn(invocation.executable, invocation.args, {
          cwd: invocation.cwd,
          shell: false,
          windowsHide: true,
        });
      } catch (error) {
        const launchError = asError(error);
        const result: CheckProcessResult = {
          exitCode: null,
          signal: null,
          stdout: "",
          stderr: "",
          launchError,
        };
        this.handleToolingError(
          workspace,
          source,
          invocation,
          result,
          `process launch failed: ${launchError.message}`,
        );
        resolve();
        return;
      }

      state.child = child;
      let stdout = "";
      let stderr = "";
      let settled = false;
      child.stdout?.setEncoding("utf8");
      child.stderr?.setEncoding("utf8");
      child.stdout?.on("data", (chunk: string) => { stdout += chunk; });
      child.stderr?.on("data", (chunk: string) => { stderr += chunk; });

      const finish = (result: CheckProcessResult): void => {
        if (settled) {
          return;
        }
        settled = true;
        if (!this.generations.isCurrent(workspaceKey, generation)) {
          resolve();
          return;
        }
        if (state.child === child) {
          state.child = undefined;
        }

        const interpretation = interpretCheckProcessResult(result);
        if (interpretation.kind === "toolingError") {
          this.handleToolingError(
            workspace,
            source,
            invocation,
            result,
            interpretation.error,
          );
        } else {
          void this.applyReport(
            workspace,
            state,
            interpretation.report,
            generation,
          ).then(resolve);
          return;
        }
        resolve();
      };

      child.once("error", (error) => {
        finish({
          exitCode: null,
          signal: null,
          stdout,
          stderr,
          launchError: error,
        });
      });
      child.once("close", (exitCode, signal) => {
        finish({ exitCode, signal, stdout, stderr });
      });
    });
  }

  removeWorkspace(workspace: vscode.WorkspaceFolder): void {
    const workspaceKey = workspace.uri.toString();
    const state = this.states.get(workspaceKey);
    this.generations.invalidate(workspaceKey);
    if (!state) {
      return;
    }
    this.cancelChild(state);
    for (const uriKey of state.ownedUris) {
      this.diagnostics.delete(vscode.Uri.parse(uriKey));
    }
    this.states.delete(workspaceKey);
  }

  handleDeletedFiles(files: readonly vscode.Uri[]): void {
    this.removePaths(files);
  }

  handleRenamedFiles(files: vscode.FileRenameEvent["files"]): void {
    const removals = this.removePaths(files.map((file) => file.oldUri));
    const destinationWorkspaces = new Map<string, vscode.WorkspaceFolder>();
    files.forEach((file, index) => {
      const destination = vscode.workspace.getWorkspaceFolder(file.newUri);
      if (!destination) {
        return;
      }
      if (isAuthoredGOXUri(file.newUri) || removals[index].matchedDescendant) {
        destinationWorkspaces.set(destination.uri.toString(), destination);
      }
    });
    for (const workspace of destinationWorkspaces.values()) {
      void this.run(workspace, "rename");
    }
  }

  dispose(): void {
    for (const [workspaceKey, state] of this.states) {
      this.generations.invalidate(workspaceKey);
      this.cancelChild(state);
    }
    this.states.clear();
  }

  private workspaceState(workspaceKey: string): WorkspaceRunState {
    let state = this.states.get(workspaceKey);
    if (!state) {
      state = { ownedUris: new Set<string>() };
      this.states.set(workspaceKey, state);
    }
    return state;
  }

  private cancelChild(state: WorkspaceRunState): void {
    const child = state.child;
    state.child = undefined;
    if (child && child.exitCode === null && child.signalCode === null) {
      child.kill();
    }
  }

  private removePaths(paths: readonly vscode.Uri[]): PathRemovalResult[] {
    const results = paths.map(() => ({ matchedDescendant: false }));
    const workspaceGroups = new Map<string, Array<{ index: number; uri: vscode.Uri }>>();
    paths.forEach((uri, index) => {
      const workspace = vscode.workspace.getWorkspaceFolder(uri);
      if (!workspace) {
        return;
      }
      const workspaceKey = workspace.uri.toString();
      const current = workspaceGroups.get(workspaceKey);
      if (current) {
        current.push({ index, uri });
      } else {
        workspaceGroups.set(workspaceKey, [{ index, uri }]);
      }
    });

    for (const [workspaceKey, removedPaths] of workspaceGroups) {
      this.generations.invalidate(workspaceKey);
      const state = this.states.get(workspaceKey);
      if (!state) {
        continue;
      }
      this.cancelChild(state);

      const removedKeys = new Set<string>();
      for (const uriKey of state.ownedUris) {
        const candidate = vscode.Uri.parse(uriKey);
        for (const removedPath of removedPaths) {
          const relation = resourcePathRelation(
            { scheme: removedPath.uri.scheme, fsPath: removedPath.uri.fsPath },
            { scheme: candidate.scheme, fsPath: candidate.fsPath },
          );
          if (relation === "outside") {
            continue;
          }
          removedKeys.add(uriKey);
          if (relation === "descendant") {
            results[removedPath.index].matchedDescendant = true;
          }
        }
      }

      const ownership = new Map<string, ReadonlySet<string>>();
      for (const [key, current] of this.states) {
        ownership.set(key, current.ownedUris);
      }
      const plan = planWorkspaceDiagnosticRemoval(ownership, workspaceKey, removedKeys);
      for (const uriKey of plan.removedKeys) {
        this.diagnostics.delete(vscode.Uri.parse(uriKey));
      }
      state.ownedUris = new Set(plan.nextKeys);
    }

    return results;
  }

  private async applyReport(
    workspace: vscode.WorkspaceFolder,
    state: WorkspaceRunState,
    report: CheckReport,
    generation: number,
  ): Promise<void> {
    const workspaceKey = workspace.uri.toString();
    const sourceGroups = new Map<string, { uri: vscode.Uri; items: CheckReport["diagnostics"] }>();
    for (const [file, fileDiagnostics] of groupDiagnosticsByFile(report.diagnostics)) {
      const uri = vscode.Uri.file(file);
      const owner = vscode.workspace.getWorkspaceFolder(uri);
      if (!owner || owner.uri.toString() !== workspaceKey) {
        continue;
      }
      const uriKey = uri.toString();
      const current = sourceGroups.get(uriKey);
      if (current) {
        current.items.push(...fileDiagnostics);
      } else {
        sourceGroups.set(uriKey, { uri, items: [...fileDiagnostics] });
      }
    }

    const mappedGroups = await Promise.all(
      [...sourceGroups.entries()].map(async ([uriKey, current]) => {
        let source: Uint8Array | undefined;
        try {
          source = await vscode.workspace.fs.readFile(current.uri);
        } catch {
          source = undefined;
        }
        return [
          uriKey,
          {
            uri: current.uri,
            diagnostics: current.items.map((item) => {
              const location = diagnosticRange(source, item.line, item.column);
              const diagnostic = new vscode.Diagnostic(
                new vscode.Range(
                  location.startLine,
                  location.startCharacter,
                  location.endLine,
                  location.endCharacter,
                ),
                item.message,
                vscode.DiagnosticSeverity.Error,
              );
              diagnostic.source = "goxc";
              return diagnostic;
            }),
          },
        ] as const;
      }),
    );
    if (!this.generations.isCurrent(workspaceKey, generation)) {
      return;
    }

    const grouped = new Map(mappedGroups);

    const ownership = new Map<string, ReadonlySet<string>>();
    for (const [key, current] of this.states) {
      ownership.set(key, current.ownedUris);
    }
    const plan = planWorkspaceDiagnosticUpdate(ownership, workspaceKey, grouped.keys());
    for (const uriKey of plan.staleKeys) {
      this.diagnostics.delete(vscode.Uri.parse(uriKey));
    }
    for (const uriKey of plan.nextKeys) {
      const current = grouped.get(uriKey);
      if (current) {
        this.diagnostics.set(current.uri, current.diagnostics);
      }
    }
    state.ownedUris = new Set(plan.nextKeys);
  }

  private handleToolingError(
    workspace: vscode.WorkspaceFolder,
    source: CheckSource,
    invocation: CheckInvocation,
    result: CheckProcessResult,
    error: string,
  ): void {
    this.output.appendLine(`[${new Date().toISOString()}] goxc check tooling error`);
    this.output.appendLine(`workspace: ${workspace.uri.fsPath}`);
    this.output.appendLine(`executable: ${JSON.stringify(invocation.executable)}`);
    this.output.appendLine(`arguments: ${JSON.stringify(invocation.args)}`);
    this.output.appendLine(`cwd: ${invocation.cwd}`);
    this.output.appendLine(`exit code: ${String(result.exitCode)}`);
    this.output.appendLine(`signal: ${String(result.signal)}`);
    this.output.appendLine(`launch error: ${result.launchError?.message ?? ""}`);
    this.output.appendLine(`transport error: ${error}`);
    this.output.appendLine(`stdout:\n${result.stdout}`);
    this.output.appendLine(`stderr:\n${result.stderr}`);
    this.output.appendLine("");
    if (source === "manual") {
      void vscode.window.showErrorMessage(
        'GOX check failed. Open the "GOX Diagnostics" output channel for details.',
      );
    }
  }
}

async function runProjectCommand(command: ProjectCommand): Promise<void> {
  const workspace = currentWorkspace();
  if (!workspace) {
    await vscode.window.showWarningMessage(
      `GOX: Open a workspace folder before running goxc ${command}.`,
    );
    return;
  }

  const terminal = vscode.window.createTerminal({
    name: `GOX: ${command}`,
    cwd: workspace.uri,
  });
  terminal.show();
  terminal.sendText(`${terminalGoxcPath(workspace)} ${command} .`);
}

function runDoctor(): void {
  const workspace = currentWorkspace();
  const terminal = vscode.window.createTerminal({
    name: "GOX: doctor",
    cwd: workspace?.uri,
  });
  terminal.show();
  terminal.sendText(`${terminalGoxcPath(workspace)} doctor`);
}

function currentWorkspace(): vscode.WorkspaceFolder | undefined {
  const activeDocument = vscode.window.activeTextEditor?.document.uri;
  if (activeDocument) {
    const folder = vscode.workspace.getWorkspaceFolder(activeDocument);
    if (folder) {
      return folder;
    }
  }
  return vscode.workspace.workspaceFolders?.[0];
}

function isAuthoredGOXDocument(document: vscode.TextDocument): boolean {
  return isAuthoredGOXUri(document.uri);
}

function isAuthoredGOXUri(uri: vscode.Uri): boolean {
  return uri.scheme === "file" && uri.fsPath.toLowerCase().endsWith(".gox");
}

function configuredGoxcExecutable(workspace?: vscode.WorkspaceFolder): string {
  return vscode.workspace
    .getConfiguration("gox", workspace?.uri)
    .get<string>("goxcPath", "goxc");
}

function terminalGoxcPath(workspace?: vscode.WorkspaceFolder): string {
  return shellQuote(configuredGoxcExecutable(workspace));
}

function shellQuote(value: string): string {
  if (/^[A-Za-z0-9_./:\\-]+$/.test(value)) {
    return value;
  }
  return `'${value.replace(/'/g, `'\\''`)}'`;
}

function asError(error: unknown): Error {
  return error instanceof Error ? error : new Error(String(error));
}

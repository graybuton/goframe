import * as vscode from "vscode";

type ProjectCommand = "generate" | "package" | "serve";

export function activate(context: vscode.ExtensionContext): void {
  context.subscriptions.push(
    vscode.commands.registerCommand("gox.generate", () => runProjectCommand("generate")),
    vscode.commands.registerCommand("gox.package", () => runProjectCommand("package")),
    vscode.commands.registerCommand("gox.serve", () => runProjectCommand("serve")),
    vscode.commands.registerCommand("gox.doctor", runDoctor),
  );
}

export function deactivate(): void {}

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
  terminal.sendText(`${goxcPath()} ${command} .`);
}

function runDoctor(): void {
  const workspace = currentWorkspace();
  const terminal = vscode.window.createTerminal({
    name: "GOX: doctor",
    cwd: workspace?.uri,
  });
  terminal.show();
  terminal.sendText(`${goxcPath()} doctor`);
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

function goxcPath(): string {
  const configured = vscode.workspace.getConfiguration("gox").get<string>("goxcPath", "goxc");
  return shellQuote(configured);
}

function shellQuote(value: string): string {
  if (/^[A-Za-z0-9_./:\\-]+$/.test(value)) {
    return value;
  }
  return `'${value.replace(/'/g, `'\\''`)}'`;
}

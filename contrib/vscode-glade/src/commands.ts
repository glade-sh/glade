import { spawn } from "child_process";
import * as vscode from "vscode";
import { debugAnonymousConfig, debugAnonymousSessionOptions, editorAnonymousSource, execAnonymousArgs } from "./commandModel";
import { configuredActiveEnvironment } from "./localOrg";
import { findProjectContext } from "./projectContext";

export function registerGladeCommands(context: vscode.ExtensionContext): void {
  const output = vscode.window.createOutputChannel("Glade");
  context.subscriptions.push(output);
  context.subscriptions.push(
    vscode.commands.registerCommand("glade.executeAnonymous", async () => {
      const source = sourceFromEditor();
      if (!source) {
        void vscode.window.showInformationMessage("Open an anonymous Apex scratch buffer or select Apex to run.");
        return;
      }
      const project = await findProjectContext();
      if (!project) {
        void vscode.window.showErrorMessage("Glade execute requires an SFDX project.");
        return;
      }
      const environment = configuredActiveEnvironment(project);
      output.clear();
      output.show(true);
      output.appendLine(`> glade exec --debug-log - --project ${project.projectRoot} --db ${environment.dbPath} <anonymous apex>`);
      const child = spawn("glade", execAnonymousArgs(source, project.projectRoot, environment.dbPath), { cwd: project.projectRoot });
      child.stdout.on("data", (chunk: Buffer) => output.append(chunk.toString()));
      child.stderr.on("data", (chunk: Buffer) => output.append(chunk.toString()));
      child.on("error", (error: Error) => {
        void vscode.window.showErrorMessage(`glade exec failed: ${error.message}`);
      });
      child.on("close", (code: number | null) => {
        if (code && code !== 0) {
          void vscode.window.showErrorMessage(`glade exec exited with code ${code}. See the Glade output channel.`);
        }
      });
    }),
  );
  context.subscriptions.push(
    vscode.commands.registerCommand("glade.debugAnonymous", async () => {
      const source = sourceFromEditor();
      if (!source) {
        void vscode.window.showInformationMessage("Open an anonymous Apex scratch buffer or select Apex to debug.");
        return;
      }
      const project = await findProjectContext();
      if (!project) {
        void vscode.window.showErrorMessage("Glade debug requires an SFDX project.");
        return;
      }
      const environment = configuredActiveEnvironment(project);
      const folder = vscode.workspace.getWorkspaceFolder(vscode.Uri.file(project.projectRoot));
      await vscode.debug.startDebugging(folder, debugAnonymousConfig(project.projectRoot, source, environment.dbPath), debugAnonymousSessionOptions());
    }),
  );
}

function sourceFromEditor(): string | undefined {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    return undefined;
  }
  const document = editor.document;
  const selection = editor.selection;
  return editorAnonymousSource({
    text: document.getText(),
    selection: selection.isEmpty
      ? undefined
      : {
          start: document.offsetAt(selection.start),
          end: document.offsetAt(selection.end),
        },
  });
}

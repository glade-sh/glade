import * as fs from "fs";
import * as path from "path";
import * as vscode from "vscode";
import { runGladeJSON } from "./gladeCli";
import {
  ConfigShowInfo,
  GladeProjectContext,
  nearestSfdxRoot,
  parseConfigShowInfo,
} from "./projectModel";

export async function findProjectContext(): Promise<GladeProjectContext | undefined> {
  const folders = vscode.workspace.workspaceFolders || [];
  if (folders.length === 0) {
    return undefined;
  }
  const activePath = vscode.window.activeTextEditor?.document.uri.fsPath;
  const sfdxFiles = await vscode.workspace.findFiles("**/sfdx-project.json", "**/{node_modules,.git,.sfdx,.sf}/**", 50);
  const sfdxPaths = sfdxFiles.map((uri) => uri.fsPath);
  if (sfdxPaths.length === 0) {
    return undefined;
  }
  let root: string | undefined;
  if (activePath) {
    root = nearestSfdxRoot(activePath, sfdxPaths);
  }
  if (!root) {
    root = path.dirname(sfdxPaths[0]);
  }
  const folder = vscode.workspace.getWorkspaceFolder(vscode.Uri.file(root)) || folders[0];
  const info = await runGladeJSON<ConfigShowInfo>(
    ["config", "show", "--project", root, "--json"],
    { cwd: root },
    "glade config show",
  );
  return parseConfigShowInfo(info, folder.uri.fsPath);
}

export function defaultDbPath(context: GladeProjectContext): string {
  return path.join(context.projectRoot, ".glade", "org.sqlite");
}

export function pathExists(file: string): boolean {
  return fs.existsSync(file);
}

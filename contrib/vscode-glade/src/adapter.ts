import { spawnSync } from "child_process";
import * as vscode from "vscode";

export interface GladeDebugConfiguration extends vscode.DebugConfiguration {
  program: string;
  project?: string;
  dbPath?: string;
  dryRun?: boolean;
  className?: string;
  methodName?: string;
}

export function resolveGladeConfiguration(
  folder: vscode.WorkspaceFolder | undefined,
  config: GladeDebugConfiguration,
): vscode.ProviderResult<GladeDebugConfiguration> {
  const probe = spawnSync("glade", ["version"], { encoding: "utf8" });
  if (probe.error || probe.status !== 0) {
    void vscode.window.showErrorMessage("The Glade debugger requires a global `glade` command on PATH.");
    return undefined;
  }
  const resolved = { ...config };
  resolved.type = resolved.type || "glade";
  resolved.request = resolved.request || "launch";
  resolved.name = resolved.name || "Glade Apex";
  resolved.project = resolved.project || folder?.uri.fsPath;
  return resolved;
}

export function adapterExecutable(config: GladeDebugConfiguration): vscode.DebugAdapterExecutable {
  return new vscode.DebugAdapterExecutable("glade", debugAdapterArgs(config));
}

export function debugAdapterArgs(config: GladeDebugConfiguration): string[] {
  const args = ["dap"];
  if (config.project) {
    args.push("--project", config.project);
  }
  if (config.dbPath) {
    args.push("--db", config.dbPath);
  }
  if (config.dryRun) {
    args.push("--dry-run");
  }
  return args;
}

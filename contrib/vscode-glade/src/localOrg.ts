import * as path from "path";
import * as vscode from "vscode";
import { activeEnvironment, GladeEnvironment, normalizeEnvironments } from "./environments";
import { runGladeJSON } from "./gladeCli";
import { DBInspectResult, LocalOrgObjectRow, objectRowsFromInspect } from "./localOrgModel";
import { GladeProjectContext } from "./projectModel";

export { salesforceTargetStatusArgs } from "./hub/salesforce";

export function configuredEnvironments(project: GladeProjectContext): GladeEnvironment[] {
  const config = vscode.workspace.getConfiguration("glade");
  const raw = config.get<GladeEnvironment[]>("environments") || [];
  return normalizeEnvironments(raw, project.projectRoot);
}

export function configuredActiveEnvironment(project: GladeProjectContext): GladeEnvironment {
  const config = vscode.workspace.getConfiguration("glade");
  return activeEnvironment(config.get<string>("activeEnvironment") || "dev", configuredEnvironments(project));
}

export async function inspectLocalOrg(
  project: GladeProjectContext,
  environment = configuredActiveEnvironment(project),
): Promise<DBInspectResult> {
  return runGladeJSON<DBInspectResult>(
    dbInspectArgs(project, environment),
    { cwd: project.projectRoot },
    "glade db inspect",
  );
}

export async function inspectLocalOrgRows(
  project: GladeProjectContext,
  environment = configuredActiveEnvironment(project),
): Promise<LocalOrgObjectRow[]> {
  return objectRowsFromInspect(await inspectLocalOrg(project, environment));
}

export function defaultEnvironmentEntry(name: string): GladeEnvironment {
  return { name, dbPath: path.join(".glade", "envs", `${name}.sqlite`) };
}

export function dbSeedArgs(project: GladeProjectContext, environment: GladeEnvironment, fixture: string): string[] {
  return ["db", "seed", "--db", environment.dbPath, "--project", project.projectRoot, "--json", fixture];
}

export function dbResetArgs(project: GladeProjectContext, environment: GladeEnvironment): string[] {
  return ["db", "reset", "--db", environment.dbPath, "--project", project.projectRoot, "--json"];
}

export function dbExportArgs(project: GladeProjectContext, environment: GladeEnvironment): string[] {
  return ["db", "export", "--db", environment.dbPath, "--project", project.projectRoot];
}

export function dbInspectArgs(project: GladeProjectContext, environment: GladeEnvironment): string[] {
  return ["db", "inspect", "--db", environment.dbPath, "--project", project.projectRoot, "--json"];
}

export function schemaImportDescribeArgs(project: Pick<GladeProjectContext, "projectRoot">, input: string): string[] {
  return ["schema", "import", "describe", "--input", input, "--project-cache", project.projectRoot];
}

export function terminalCommand(args: string[], redirectPath?: string): string {
  const command = args.map(shellQuote).join(" ");
  return redirectPath ? `${command} > ${shellQuote(redirectPath)}` : command;
}

export function sendGladeTerminal(command: string): void {
  const terminal = vscode.window.createTerminal("Glade");
  terminal.show();
  terminal.sendText(command);
}

export function sendLocalOrgTerminal(command: string): void {
  const terminal = vscode.window.createTerminal("Glade Local Data");
  terminal.show();
  terminal.sendText(command);
}

function shellQuote(value: string): string {
  if (/^[A-Za-z0-9_./:=+-]+$/.test(value)) {
    return value;
  }
  return `'${value.replace(/'/g, "'\\''")}'`;
}

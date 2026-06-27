import * as path from "path";
import * as vscode from "vscode";
import { activeEnvironment, GladeEnvironment, normalizeEnvironments } from "./environments";
import { parseJSONRunResult, runGlade, runGladeJSON } from "./gladeCli";
import type { ProjectOrgState } from "./hub/model";
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

export type TUIView = "project" | "tests" | "data" | "plugins";

export function tuiArgs(
  project: Pick<GladeProjectContext, "projectRoot">,
  view: TUIView,
  dbPath?: string,
): string[] {
  const args = ["tui", "--project", project.projectRoot, "--view", view];
  if (dbPath) {
    args.push("--db", dbPath);
  }
  return args;
}

export function schemaImportDescribeArgs(project: Pick<GladeProjectContext, "projectRoot">, input: string): string[] {
  return ["schema", "import", "describe", "--input", input, "--project-cache", project.projectRoot];
}

export interface ProjectOrgStatus {
  alias?: string;
  status?: string;
  error?: string;
  instanceUrl?: string;
  db?: string;
}

export function orgCreateArgs(project: Pick<GladeProjectContext, "projectRoot">, alias: string): string[] {
  return ["org", "create", alias, "--project", project.projectRoot];
}

export function orgStartArgs(project: Pick<GladeProjectContext, "projectRoot">, alias: string): string[] {
  return ["org", "start", alias, "--project", project.projectRoot];
}

export function orgStatusArgs(project: Pick<GladeProjectContext, "projectRoot">, alias: string): string[] {
  return ["org", "status", alias, "--project", project.projectRoot, "--json"];
}

export async function checkProjectOrg(
  project: Pick<GladeProjectContext, "projectRoot">,
  alias: string,
): Promise<ProjectOrgState> {
  const result = await runGlade(orgStatusArgs(project, alias), { cwd: project.projectRoot });
  if (result.code !== 0) {
    return { alias, state: "missing", detail: conciseOrgError(result.stderr || result.stdout || `exit code ${result.code}`) };
  }
  return projectOrgStateFromStatus(
    parseJSONRunResult<ProjectOrgStatus>(result, "glade org status"),
    alias,
  );
}

export async function createProjectOrg(
  project: Pick<GladeProjectContext, "projectRoot">,
  alias: string,
): Promise<void> {
  const result = await runGlade(orgCreateArgs(project, alias), { cwd: project.projectRoot });
  if (result.code !== 0) {
    throw new Error(conciseOrgError(result.stderr || result.stdout || `exit code ${result.code}`));
  }
}

export function projectOrgStateFromStatus(status: ProjectOrgStatus, fallbackAlias = "my-glade-org"): ProjectOrgState {
  const alias = status.alias || fallbackAlias;
  if (status.status === "running") {
    return { alias, state: "running", detail: status.instanceUrl || status.db };
  }
  if (status.status === "stopped") {
    return { alias, state: "stopped", detail: status.error || status.db || "not running" };
  }
  return { alias, state: "unknown", detail: status.error || status.status || "not checked" };
}

export function terminalCommand(args: string[], redirectPath?: string): string {
  const command = args.map(shellQuote).join(" ");
  return redirectPath ? `${command} > ${shellQuote(redirectPath)}` : command;
}

export function sendGladeTerminal(command: string, cwd?: string): vscode.Terminal {
  const terminal = cwd
    ? vscode.window.createTerminal({ name: "Glade", cwd })
    : vscode.window.createTerminal("Glade");
  terminal.show();
  terminal.sendText(command);
  return terminal;
}

export function sendLocalOrgTerminal(command: string): vscode.Terminal {
  const terminal = vscode.window.createTerminal("Glade Local Data");
  terminal.show();
  terminal.sendText(command);
  return terminal;
}

function shellQuote(value: string): string {
  if (/^[A-Za-z0-9_./:=+-]+$/.test(value)) {
    return value;
  }
  return `'${value.replace(/'/g, "'\\''")}'`;
}

function conciseOrgError(value: string): string {
  const first = value.trim().split(/\r?\n/, 1)[0] || value;
  if (/no such file|cannot find|not exist|open .*org\.json/i.test(first)) {
    return "Create the local org first.";
  }
  return first;
}

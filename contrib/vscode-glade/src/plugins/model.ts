export const supportedActionViews = [
  "startHere",
  "runs",
  "localOrg",
  "debug",
  "plugins",
] as const;

export type PluginActionView = (typeof supportedActionViews)[number];
export type ActionView = PluginActionView;

export const supportedActionContexts = [
  "project",
  "activeApexFile",
  "activeDebugLog",
  "activeDataEnvironment",
  "lastLocalRun",
] as const;

export type PluginActionContext = (typeof supportedActionContexts)[number];
export type ActionContext = PluginActionContext;

export type PluginFindingSeverity = "error" | "warning" | "info" | "hint";
export type FindingSeverity = PluginFindingSeverity;

export interface InstalledPlugin {
  name: string;
  identityName?: string;
  canonicalName?: string;
  version: string;
  linked?: boolean;
  commands?: string[];
  commandRoots?: string[];
  executable?: string;
  manifest?: string;
  source?: string;
  editor?: PluginEditorManifest;
}

export interface PluginEditorManifest {
  actions?: PluginEditorAction[];
}

export interface PluginEditorActionInput {
  name: string;
  label?: string;
  description?: string;
  type?: "string" | "text" | "number" | "boolean" | "file" | "directory" | "choice";
  required?: boolean;
  default?: string | number | boolean;
  defaultValue?: string | number | boolean;
  options?: string[];
}
export type ActionInput = PluginEditorActionInput;

export type PluginOutputContract = string;
export type OutputContract = PluginOutputContract;

export interface PluginEditorAction {
  id: string;
  title: string;
  description?: string;
  icon?: string;
  command: string[];
  args?: string[];
  view?: PluginActionView;
  contexts?: PluginActionContext[];
  inputs?: PluginEditorActionInput[];
  output?: PluginOutputContract;
}
export type EditorAction = PluginEditorAction;

export type PluginAvailableContexts = Partial<Record<PluginActionContext, boolean>>;
export type AvailableActionContexts = PluginAvailableContexts;

export function isApexDebugLogPath(filePath: string | undefined): boolean {
  return filePath ? /\.(?:apexlog|log|txt)$/i.test(filePath) : false;
}

export function isApexDebugLogEditor(filePath: string | undefined, languageId: string | undefined): boolean {
  if (languageId === "apexlog") {
    return true;
  }
  return filePath ? /\.apexlog$/i.test(filePath) : false;
}

export interface PluginActionResolutionValues {
  projectRoot?: string;
  workspaceFolder?: string;
  activeFile?: string;
  activeDb?: string;
  outputDir?: string;
  inputs?: Record<string, string | number | boolean | null | undefined>;
}
export type ActionResolutionValues = PluginActionResolutionValues;

export interface ResolvedPluginAction {
  id: string;
  title: string;
  icon?: string;
  command: string[];
  args: string[];
  argv: string[];
  output?: PluginOutputContract;
}
export type ResolvedAction = ResolvedPluginAction;

export interface PluginActionResolutionError {
  code: "missingTokenValue";
  message: string;
  missingTokens: string[];
}
export type ActionResolutionError = PluginActionResolutionError;

export type PluginActionResolutionResult =
  | { ok: true; action: ResolvedPluginAction }
  | { ok: false; error: PluginActionResolutionError };
export type ActionResolutionResult = PluginActionResolutionResult;

export interface PluginFinding {
  severity: PluginFindingSeverity;
  file?: string;
  line?: number;
  column?: number;
  message: string;
  ruleId?: string;
  source?: string;
}
export type Finding = PluginFinding;

export interface PluginArtifact {
  label?: string;
  path?: string;
  uri?: string;
  kind?: string;
  mimeType?: string;
}
export type Artifact = PluginArtifact;

export interface GladeFindingsV1 {
  kind: "glade.findings.v1";
  summary?: unknown;
  findings: PluginFinding[];
  artifacts: PluginArtifact[];
}

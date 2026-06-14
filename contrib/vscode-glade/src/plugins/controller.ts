import * as fs from "fs";
import * as path from "path";
import { runGlade, GladeRunOptions, GladeRunResult, parseJSONRunResult } from "../gladeCli";
import { actionsForView, resolveAction } from "./actions";
import { pluginsListArgs } from "./cli";
import { parseFindingsOutput } from "./findings";
import {
  InstalledPlugin,
  PluginActionView,
  PluginArtifact,
  PluginAvailableContexts,
  PluginEditorAction,
  PluginFinding,
  PluginFindingSeverity,
} from "./model";

export interface PluginProjectContext {
  projectRoot?: string;
  workspaceFolder?: string;
}

export interface PluginInputOptions {
  title?: string;
  prompt?: string;
  value?: string;
  placeHolder?: string;
}

export interface PluginQuickPickItem {
  label: string;
  description?: string;
  command?: string;
  value?: string;
}

export interface PluginOpenDialogOptions {
  title: string;
  canSelectFiles?: boolean;
  canSelectFolders?: boolean;
  canSelectMany?: boolean;
  filters?: Record<string, string[]>;
}

export interface PluginDiagnosticEntry extends PluginFinding {
  file: string;
}

export interface PluginDiagnostics {
  set(entries: PluginDiagnosticEntry[]): void;
  clear(): void;
}

export interface PluginControllerHost {
  project(): Promise<PluginProjectContext | undefined>;
  activeFile?(): string | undefined;
  activeDb?(): string | undefined;
  inputBox(options: PluginInputOptions): Promise<string | undefined>;
  quickPick?(items: PluginQuickPickItem[], options: { title?: string; placeHolder?: string }): Promise<PluginQuickPickItem | undefined>;
  openDialog?(options: PluginOpenDialogOptions): Promise<string[] | undefined>;
  runGlade?(args: string[], options?: GladeRunOptions): Promise<GladeRunResult>;
  diagnostics: PluginDiagnostics;
  log?(message: string): void;
  executeCommand?(command: string): Promise<void>;
}

export interface PluginActionRow {
  id: string;
  label: string;
  description?: string;
  tooltip?: string;
  action: PluginEditorAction;
}

export interface PluginArtifactRow {
  id: string;
  label: string;
  description?: string;
  tooltip?: string;
  path?: string;
  uri?: string;
}

export class PluginController {
  private installed: InstalledPlugin[] = [];
  private artifacts: PluginArtifact[] = [];
  private findingCount = 0;

  constructor(private readonly host: PluginControllerHost) {}

  async refresh(): Promise<void> {
    try {
      const result = await this.run(pluginsListArgs());
      const parsed = parseJSONRunResult<{ plugins?: InstalledPlugin[] }>(result, "glade plugins list");
      this.installed = Array.isArray(parsed.plugins) ? parsed.plugins : [];
    } catch (error) {
      this.installed = [];
      this.log(`glade plugins list failed: ${errorMessage(error)}`);
    }
  }

  plugins(): InstalledPlugin[] {
    return [...this.installed];
  }

  latestArtifacts(): PluginArtifact[] {
    return [...this.artifacts];
  }

  latestFindingCount(): number {
    return this.findingCount;
  }

  actionsForView(view: PluginActionView, contexts: PluginAvailableContexts = {}): PluginEditorAction[] {
    return actionsForView(this.installed, view, contexts);
  }

  actionRowsForView(view: PluginActionView, contexts: PluginAvailableContexts = {}): PluginActionRow[] {
    return pluginActionRows(this.actionsForView(view, contexts), this.installed);
  }

  async managePlugins(): Promise<void> {
    const picked = await this.host.quickPick?.(
      [
        { label: "Refresh", command: "glade.refreshPlugins" },
        { label: "Link Local Plugin", command: "glade.linkLocalPlugin" },
        { label: "Install Plugin Archive", command: "glade.installPluginArchive" },
      ],
      { title: "Manage Plugins", placeHolder: "Choose a plugin task" },
    );
    if (picked?.command && this.host.executeCommand) {
      await this.host.executeCommand(picked.command);
    }
  }

  async linkLocalPlugin(): Promise<void> {
    const picked = await this.host.openDialog?.({
      title: "Link Local Glade Plugin",
      canSelectFiles: true,
      canSelectFolders: false,
      canSelectMany: false,
    });
    const executable = picked?.[0];
    if (!executable) {
      return;
    }
    await this.runPluginManagement(["plugins", "link", "--exec", executable], "glade plugins link");
  }

  async installPluginArchive(): Promise<void> {
    const picked = await this.host.openDialog?.({
      title: "Install Glade Plugin Archive",
      canSelectFiles: true,
      canSelectFolders: false,
      canSelectMany: false,
      filters: { Archives: ["tar.gz", "tgz", "zip"] },
    });
    const archive = picked?.[0];
    if (!archive) {
      return;
    }
    await this.runPluginManagement(["plugins", "install", archive, "--yes"], "glade plugins install");
  }

  async runAction(action: PluginEditorAction): Promise<void> {
    const project = await this.host.project();
    const root = project?.projectRoot || project?.workspaceFolder;
    if (!root) {
      this.log("plugin action requires a project workspace");
      return;
    }

    const inputs = await this.promptInputs(action);
    if (!inputs.ok) {
      return;
    }

    const outputDir = path.join(root, ".glade", "editor", "plugins", actionOutputName(action));
    await fs.promises.mkdir(outputDir, { recursive: true });

    const resolved = resolveAction(action, {
      projectRoot: project?.projectRoot,
      workspaceFolder: project?.workspaceFolder || project?.projectRoot,
      activeFile: this.host.activeFile?.(),
      activeDb: this.host.activeDb?.(),
      outputDir,
      inputs: inputs.values,
    });
    if (!resolved.ok) {
      this.log(resolved.error.message);
      return;
    }

    const result = await this.run(resolved.action.argv, { cwd: root });
    if (result.code !== 0) {
      const detail = result.stderr.trim() || result.stdout.trim() || `exit code ${result.code}`;
      this.log(`${action.title} failed: ${detail}`);
      return;
    }

    if (action.output === "glade.findings.v1") {
      try {
        const parsed = parseFindingsOutput(result.stdout);
        this.artifacts = parsed.artifacts;
        this.findingCount = parsed.findings.length;
        this.publishFindings(root, parsed.findings);
      } catch (error) {
        this.log(`${action.title} findings parse failed: ${errorMessage(error)}`);
      }
    }
  }

  private async runPluginManagement(args: string[], label: string): Promise<void> {
    try {
      const project = await this.host.project();
      const result = await this.run(args, { cwd: project?.projectRoot || project?.workspaceFolder });
      if (result.code !== 0) {
        const detail = result.stderr.trim() || result.stdout.trim() || `exit code ${result.code}`;
        this.log(`${label} failed: ${detail}`);
        return;
      }
      await this.refresh();
    } catch (error) {
      this.log(`${label} failed: ${errorMessage(error)}`);
    }
  }

  private async promptInputs(action: PluginEditorAction): Promise<{ ok: true; values: Record<string, string> } | { ok: false }> {
    const values: Record<string, string> = {};
    for (const input of action.inputs || []) {
      const defaultValue = input.defaultValue ?? input.default;
      const entered = await this.host.inputBox({
        title: input.label || input.name,
        prompt: input.description || input.label || input.name,
        value: defaultValue === undefined ? undefined : String(defaultValue),
        placeHolder: input.options?.join(", ") || input.name,
      });
      if (!entered && input.required) {
        return { ok: false };
      }
      if (entered) {
        values[input.name] = entered;
      }
    }
    return { ok: true, values };
  }

  private publishFindings(projectRoot: string, findings: PluginFinding[]): void {
    const entries: PluginDiagnosticEntry[] = [];
    for (const finding of findings) {
      if (!finding.file) {
        continue;
      }
      entries.push({
        ...finding,
        severity: normalizeDiagnosticSeverity(finding.severity),
        file: path.isAbsolute(finding.file) ? finding.file : path.join(projectRoot, finding.file),
      });
    }
    this.host.diagnostics.set(entries);
  }

  private run(args: string[], options: GladeRunOptions = {}): Promise<GladeRunResult> {
    return (this.host.runGlade || runGlade)(args, options);
  }

  private log(message: string): void {
    this.host.log?.(message);
  }
}

export function pluginActionRows(actions: PluginEditorAction[], plugins: InstalledPlugin[] = []): PluginActionRow[] {
  return actions.map((action) => {
    const plugin = pluginForAction(action, plugins);
    const description = plugin ? pluginLabel(plugin) : action.description;
    return {
      id: action.id,
      label: action.title,
      description,
      tooltip: action.description || description || action.title,
      action,
    };
  });
}

export function pluginArtifactRows(artifacts: PluginArtifact[]): PluginArtifactRow[] {
  return artifacts.map((artifact, index) => {
    const target = artifact.path || artifact.uri;
    return {
      id: `artifact.${index}.${target || artifact.label || "artifact"}`,
      label: artifact.label || path.basename(target || "Artifact"),
      description: artifact.kind || artifact.mimeType,
      tooltip: target || artifact.label,
      path: artifact.path,
      uri: artifact.uri,
    };
  });
}

export function actionOutputName(action: PluginEditorAction): string {
  return sanitizePathPart(action.id || action.command.join("-"));
}

function sanitizePathPart(value: string): string {
  return value.replace(/[^A-Za-z0-9._-]+/g, "-").replace(/\.+/g, "-").replace(/^-+|-+$/g, "") || "action";
}

function pluginForAction(action: PluginEditorAction, plugins: InstalledPlugin[]): InstalledPlugin | undefined {
  return plugins.find((plugin) => (plugin.editor?.actions || []).some((candidate) => candidate.id === action.id));
}

function pluginLabel(plugin: InstalledPlugin): string {
  const name = plugin.identityName || plugin.canonicalName || plugin.name;
  return plugin.linked ? `${name} linked` : name;
}

function normalizeDiagnosticSeverity(severity: PluginFindingSeverity): PluginFindingSeverity {
  return severity;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

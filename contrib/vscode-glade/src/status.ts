import * as vscode from "vscode";
import { configuredActiveEnvironment } from "./localOrg";
import { GladeProjectContext } from "./projectModel";
import { buildStatusText, buildStatusTooltip, GladeStatusSnapshot, StatusRunSummary } from "./statusModel";

export class GladeStatus {
  private readonly item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 40);
  private project?: GladeProjectContext;
  private lastRun?: StatusRunSummary;
  private changedRecords?: number;
  private lastCommand?: string;
  private busyLabel?: string;
  private missingDb = false;
  private pluginActionCount?: number;

  constructor(context: vscode.ExtensionContext) {
    this.item.command = "glade.statusQuickPick";
    context.subscriptions.push(this.item);
  }

  setProject(project: GladeProjectContext | undefined): void {
    this.project = project;
    this.render();
  }

  setLastRun(run: StatusRunSummary | undefined, command?: string): void {
    this.lastRun = run;
    this.lastCommand = command;
    this.busyLabel = undefined;
    this.render();
  }

  setChangedRecords(count: number | undefined): void {
    this.changedRecords = count;
    this.render();
  }

  setBusy(label: string | undefined): void {
    this.busyLabel = label;
    this.render();
  }

  setMissingDb(missing: boolean): void {
    this.missingDb = missing;
    this.render();
  }

  setPluginActionCount(count: number | undefined): void {
    this.pluginActionCount = count;
    this.render();
  }

  private snapshot(): GladeStatusSnapshot {
    const environment = this.project ? configuredActiveEnvironment(this.project) : undefined;
    return {
      projectReady: Boolean(this.project),
      projectRoot: this.project?.projectRoot,
      activeEnvironment: environment?.name,
      dbPath: environment?.dbPath,
      lastRun: this.lastRun,
      changedRecords: this.changedRecords,
      missingDb: this.missingDb,
      busyLabel: this.busyLabel,
      lastCommand: this.lastCommand,
      pluginActionCount: this.pluginActionCount,
    };
  }

  private render(): void {
    const snapshot = this.snapshot();
    this.item.text = buildStatusText(snapshot);
    this.item.tooltip = buildStatusTooltip(snapshot);
    this.item.show();
  }
}

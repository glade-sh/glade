import * as vscode from "vscode";
import { GladeEnvironment } from "../environments";
import { configuredActiveEnvironment } from "../localOrg";
import { DBInspectResult, LocalOrgObjectRow, LocalOrgSummary, objectRowsFromInspect, summaryFromInspect } from "../localOrgModel";
import { PluginActionRow } from "../plugins/controller";
import { GladeProjectContext } from "../projectModel";
import { pluginActionTreeRows } from "./pluginsView";
import { commandItem, GladeTreeItem } from "./tree";

export class LocalOrgView implements vscode.TreeDataProvider<GladeTreeItem> {
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  private project?: GladeProjectContext;
  private environment?: GladeEnvironment;
  private summary?: LocalOrgSummary;
  private rows: LocalOrgObjectRow[] = [];
  private pluginActions: PluginActionRow[] = [];
  readonly onDidChangeTreeData = this.changed.event;

  setProject(project: GladeProjectContext | undefined): void {
    this.project = project;
    const nextEnvironment = project ? configuredActiveEnvironment(project) : undefined;
    if (this.environment?.name !== nextEnvironment?.name) {
      this.summary = undefined;
      this.rows = [];
    }
    this.environment = nextEnvironment;
    this.refresh();
  }

  refresh(): void {
    this.changed.fire();
  }

  setPluginActions(actions: PluginActionRow[]): void {
    this.pluginActions = actions;
    this.refresh();
  }

  setInspect(result: DBInspectResult, environment?: GladeEnvironment): void {
    this.environment = environment;
    this.summary = summaryFromInspect(result);
    this.rows = objectRowsFromInspect(result);
    this.changed.fire();
  }

  getTreeItem(element: GladeTreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(): GladeTreeItem[] {
    const commands = [
      commandItem("Inspect active environment", "glade.inspectLocalOrg"),
      commandItem("Seed active environment", "glade.seedLocalOrg"),
      commandItem("Reset active environment", "glade.resetLocalOrg"),
      commandItem("Export active environment", "glade.exportLocalOrg"),
    ];
    const environmentRows = this.environment
      ? [labelItem(`Active: ${this.environment.name}`, this.environment.dbPath, this.environment.dbPath)]
      : this.project
        ? [labelItem("Active data", "not inspected", this.project.projectRoot)]
        : [];
    if (!this.summary) {
      return [...environmentRows, ...commands, ...pluginActionTreeRows(this.pluginActions)];
    }
    return [
      ...environmentRows,
      summaryItem("Objects", this.summary.objects),
      summaryItem("Records", this.summary.records),
      summaryItem("Users", this.summary.users),
      summaryItem("Profiles", this.summary.profiles),
      summaryItem("Permissions", this.summary.permissions),
      ...this.rows.map((row) => summaryItem(row.name, row.rows)),
      ...commands,
      ...pluginActionTreeRows(this.pluginActions),
    ];
  }
}

function labelItem(label: string, description?: string, tooltip?: string): GladeTreeItem {
  const item = new GladeTreeItem(label);
  item.description = description;
  item.tooltip = tooltip || description || label;
  return item;
}

function summaryItem(label: string, value: number): GladeTreeItem {
  const item = new GladeTreeItem(label);
  item.description = String(value);
  item.tooltip = `${label}: ${value}`;
  return item;
}

import * as vscode from "vscode";
import { configuredActiveEnvironment } from "../localOrg";
import { GladeProjectContext } from "../projectModel";
import { StartHereState } from "../startHereState";
import { buildStartHereRows, StartHereRow } from "../startHereModel";
import { GladeTreeItem } from "./tree";

export class StartHereView implements vscode.TreeDataProvider<GladeTreeItem> {
  private project?: GladeProjectContext;
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData = this.changed.event;

  constructor(private readonly state: StartHereState) {}

  setProject(project: GladeProjectContext | undefined): void {
    this.project = project;
    this.refresh();
  }

  refresh(): void {
    this.changed.fire();
  }

  getTreeItem(element: GladeTreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(): GladeTreeItem[] {
    const config = vscode.workspace.getConfiguration("glade");
    const runtime = this.state.snapshot();
    const rows = buildStartHereRows({
      project: this.project,
      activeEnvironment: this.project ? configuredActiveEnvironment(this.project) : undefined,
      localOrgSummary: runtime.localOrgSummary,
      missingDb: runtime.missingDb,
      watchRunning: runtime.watchRunning,
      lastRun: runtime.lastRun,
      changedSince: config.get<string>("changedSince") || "origin/main",
    });
    return rows.map(toTreeItem);
  }
}

function toTreeItem(row: StartHereRow): GladeTreeItem {
  const item = new GladeTreeItem(row.label);
  item.id = row.id;
  item.description = row.description;
  item.tooltip = row.tooltip || row.description || row.label;
  item.contextValue = row.contextValue;
  item.iconPath = iconFor(row);
  if (row.command) {
    item.command = { command: row.command, title: row.label };
  }
  return item;
}

function iconFor(row: StartHereRow): vscode.ThemeIcon | undefined {
  switch (row.id) {
    case "ready":
      return new vscode.ThemeIcon("check");
    case "environment":
      return new vscode.ThemeIcon("database");
    case "local-proof":
      return new vscode.ThemeIcon("play");
    case "last-run":
      return new vscode.ThemeIcon("history");
    case "watch":
      return new vscode.ThemeIcon("sync");
    case "salesforce":
      return new vscode.ThemeIcon("cloud");
    default:
      return undefined;
  }
}

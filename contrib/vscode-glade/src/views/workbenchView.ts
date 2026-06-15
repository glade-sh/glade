import * as vscode from "vscode";
import { WorkbenchTreeRow } from "../workbench/model";
import { commandItem, GladeTreeItem } from "./tree";

export class WorkbenchView implements vscode.TreeDataProvider<GladeTreeItem> {
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  private rows: WorkbenchTreeRow[] = [];
  readonly onDidChangeTreeData = this.changed.event;

  setRows(rows: WorkbenchTreeRow[]): void {
    this.rows = rows;
    this.refresh();
  }

  refresh(): void {
    this.changed.fire();
  }

  getTreeItem(element: GladeTreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(): GladeTreeItem[] {
    return [
      commandItem("Open Anonymous Apex Scratch", "glade.workbench.newAnonymousApex", "Open an untitled Apex editor for anonymous Apex.", new vscode.ThemeIcon("new-file")),
      commandItem("New SOQL Query", "glade.workbench.newSoql", "Create and run a saved SOQL query.", new vscode.ThemeIcon("add")),
      commandItem("Describe Local Data", "glade.workbench.describe", "Describe local objects or fields.", new vscode.ThemeIcon("symbol-field")),
      commandItem("Open Last Result", "glade.workbench.openResult", "Open the last Glade Workbench result file.", new vscode.ThemeIcon("file")),
      ...this.rows.map(toWorkbenchTreeItem),
    ];
  }
}

export function toWorkbenchTreeItem(row: WorkbenchTreeRow): GladeTreeItem {
  const item = new GladeTreeItem(row.label);
  item.id = row.id;
  item.description = row.count !== undefined ? String(row.count) : row.description;
  item.tooltip = row.description || row.label;
  item.contextValue = row.type === "entry" ? `gladeWorkbenchEntry.${row.kind}` : `gladeWorkbench.${row.type}`;
  item.iconPath = iconFor(row);
  if (row.type === "entry" && row.entryId) {
    item.command = {
      command: "glade.workbench.runEntry",
      title: row.label,
      arguments: [row.entryId],
    };
  }
  return item;
}

function iconFor(row: WorkbenchTreeRow): vscode.ThemeIcon | undefined {
  switch (row.type) {
    case "environment":
      return new vscode.ThemeIcon("database");
    case "group":
      return new vscode.ThemeIcon("search");
    case "entry":
      return new vscode.ThemeIcon("table");
    default:
      return undefined;
  }
}

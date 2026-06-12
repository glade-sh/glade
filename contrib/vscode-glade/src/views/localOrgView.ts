import * as vscode from "vscode";
import { DBInspectResult, LocalOrgObjectRow, LocalOrgSummary, objectRowsFromInspect, summaryFromInspect } from "../localOrgModel";
import { commandItem, GladeTreeItem } from "./tree";

export class LocalOrgView implements vscode.TreeDataProvider<GladeTreeItem> {
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  private summary?: LocalOrgSummary;
  private rows: LocalOrgObjectRow[] = [];
  readonly onDidChangeTreeData = this.changed.event;

  refresh(): void {
    this.changed.fire();
  }

  setInspect(result: DBInspectResult): void {
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
    if (!this.summary) {
      return commands;
    }
    return [
      summaryItem("Objects", this.summary.objects),
      summaryItem("Records", this.summary.records),
      summaryItem("Users", this.summary.users),
      summaryItem("Profiles", this.summary.profiles),
      summaryItem("Permissions", this.summary.permissions),
      ...this.rows.map((row) => summaryItem(row.name, row.rows)),
      ...commands,
    ];
  }
}

function summaryItem(label: string, value: number): GladeTreeItem {
  const item = new GladeTreeItem(label);
  item.description = String(value);
  item.tooltip = `${label}: ${value}`;
  return item;
}

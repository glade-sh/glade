import * as vscode from "vscode";
import { commandItem, GladeTreeItem } from "./tree";

export class RunsView implements vscode.TreeDataProvider<GladeTreeItem> {
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData = this.changed.event;

  refresh(): void {
    this.changed.fire();
  }

  getTreeItem(element: GladeTreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(): GladeTreeItem[] {
    return [
      commandItem("Run changed since origin/main", "glade.runChangedTests"),
      commandItem("Run failed tests", "glade.runFailedTests"),
      commandItem("Start watch", "glade.startWatch"),
      commandItem("Stop watch", "glade.stopWatch"),
    ];
  }
}

export class ApexTestsView implements vscode.TreeDataProvider<GladeTreeItem> {
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData = this.changed.event;

  refresh(): void {
    this.changed.fire();
  }

  getTreeItem(element: GladeTreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(): GladeTreeItem[] {
    return [
      commandItem("Refresh local tests", "glade.refresh"),
      new GladeTreeItem("Test Explorer wiring pending"),
    ];
  }
}

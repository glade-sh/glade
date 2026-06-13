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
      commandItem("Run local proof", "glade.runLocalProof", "Run changed local Apex tests and inspect active local data.", new vscode.ThemeIcon("play")),
      commandItem("Changed since origin/main", "glade.runChangedTests", "Run local tests affected by changes since origin/main.", new vscode.ThemeIcon("git-compare")),
      commandItem("Run failed local tests", "glade.runFailedTests", "Run the failed local Apex tests again.", new vscode.ThemeIcon("debug-rerun")),
      commandItem("Start watch", "glade.startWatch", "Start the local Apex watch loop.", new vscode.ThemeIcon("eye")),
      commandItem("Stop watch", "glade.stopWatch", "Stop the local Apex watch loop.", new vscode.ThemeIcon("circle-slash")),
    ];
  }
}

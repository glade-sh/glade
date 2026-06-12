import * as vscode from "vscode";
import { GladeProjectContext } from "../projectModel";
import { commandItem, GladeTreeItem } from "./tree";

export class ProjectView implements vscode.TreeDataProvider<GladeTreeItem> {
  private project?: GladeProjectContext;
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData = this.changed.event;

  setProject(project: GladeProjectContext | undefined): void {
    this.project = project;
    this.changed.fire();
  }

  getTreeItem(element: GladeTreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(): GladeTreeItem[] {
    if (!this.project) {
      return [commandItem("Open an SFDX project", "vscode.openFolder", "Open a folder containing sfdx-project.json")];
    }
    const root = new GladeTreeItem(this.project.projectRoot);
    root.description = "SFDX root";
    const namespace = new GladeTreeItem(`Namespace: ${this.project.namespace || "(none)"}`);
    const api = new GladeTreeItem(`Source API: ${this.project.sourceApiVersion || "(unknown)"}`);
    const dirs = new GladeTreeItem(`Package dirs: ${this.project.packageDirs.join(", ") || "."}`);
    return [root, namespace, api, dirs, commandItem("Refresh", "glade.refresh")];
  }
}

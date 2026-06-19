import * as vscode from "vscode";
import * as path from "path";
import { apexBreakpoints } from "../breakpoints";
import { PluginActionRow } from "../plugins/controller";
import { GladeProjectContext } from "../projectModel";
import { pluginActionTreeRows } from "./pluginsView";
import { commandItem, GladeTreeItem } from "./tree";

export class DebugView implements vscode.TreeDataProvider<GladeTreeItem> {
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  private pluginActions: PluginActionRow[] = [];
  private projectReady = false;
  readonly onDidChangeTreeData = this.changed.event;

  setProject(project: GladeProjectContext | undefined): void {
    this.projectReady = project !== undefined;
    this.refresh();
  }

  refresh(): void {
    this.changed.fire();
  }

  setPluginActions(actions: PluginActionRow[]): void {
    this.pluginActions = actions;
    this.refresh();
  }

  getTreeItem(element: GladeTreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(): GladeTreeItem[] {
    if (!this.projectReady) {
      return [];
    }
    const breakpoints = apexBreakpoints();
    return [
      ...breakpoints.map((breakpoint) => breakpointItem(breakpoint.file, breakpoint.line, breakpoint.enabled)),
      commandItem("Debug current test", "glade.debugCurrentTest", "Debug the local Apex test at the cursor.", new vscode.ThemeIcon("debug-alt")),
      ...pluginActionTreeRows(this.pluginActions),
    ];
  }
}

function breakpointItem(file: string, line: number, enabled: boolean): GladeTreeItem {
  const item = new GladeTreeItem(`${path.basename(file)}:${line}`);
  item.description = enabled ? "enabled" : "disabled";
  item.tooltip = file;
  return item;
}

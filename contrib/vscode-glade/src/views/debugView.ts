import * as vscode from "vscode";
import * as path from "path";
import { apexBreakpoints } from "../breakpoints";
import { commandItem, GladeTreeItem } from "./tree";

export class DebugView implements vscode.TreeDataProvider<GladeTreeItem> {
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  readonly onDidChangeTreeData = this.changed.event;

  refresh(): void {
    this.changed.fire();
  }

  getTreeItem(element: GladeTreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(): GladeTreeItem[] {
    const breakpoints = apexBreakpoints();
    return [
      new GladeTreeItem(`Breakpoints: ${breakpoints.length}`),
      ...breakpoints.map((breakpoint) => breakpointItem(breakpoint.file, breakpoint.line, breakpoint.enabled)),
      commandItem("Debug selected Apex", "glade.debugAnonymous"),
      commandItem("Debug current test", "glade.debugCurrentTest"),
    ];
  }
}

function breakpointItem(file: string, line: number, enabled: boolean): GladeTreeItem {
  const item = new GladeTreeItem(`${path.basename(file)}:${line}`);
  item.description = enabled ? "enabled" : "disabled";
  item.tooltip = file;
  return item;
}

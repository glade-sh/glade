import * as vscode from "vscode";
import * as path from "path";
import { apexBreakpoints } from "../breakpoints";
import { configuredActiveEnvironment } from "../localOrg";
import { GladeProjectContext } from "../projectModel";
import { commandItem, GladeTreeItem } from "./tree";

export class DebugView implements vscode.TreeDataProvider<GladeTreeItem> {
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  private project?: GladeProjectContext;
  readonly onDidChangeTreeData = this.changed.event;

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
    const breakpoints = apexBreakpoints();
    const environment = this.project ? configuredActiveEnvironment(this.project) : undefined;
    return [
      environmentItem(environment),
      new GladeTreeItem(`Breakpoints: ${breakpoints.length}`),
      ...breakpoints.map((breakpoint) => breakpointItem(breakpoint.file, breakpoint.line, breakpoint.enabled)),
      commandItem("Debug selected Apex", "glade.debugAnonymous", "Debug anonymous or selected local Apex.", new vscode.ThemeIcon("debug-start")),
      commandItem("Debug current test", "glade.debugCurrentTest", "Debug the local Apex test at the cursor.", new vscode.ThemeIcon("debug-alt")),
      commandItem("Open trace output", "glade.openOutput", "Open the Glade output channel.", new vscode.ThemeIcon("output")),
    ];
  }
}

function environmentItem(environment: ReturnType<typeof configuredActiveEnvironment> | undefined): GladeTreeItem {
  const item = new GladeTreeItem(environment ? `Env: ${environment.name}` : "Env: no SFDX project");
  item.description = environment ? path.basename(environment.dbPath) : undefined;
  item.tooltip = environment?.dbPath || "Open an SFDX project.";
  item.iconPath = new vscode.ThemeIcon("database");
  return item;
}

function breakpointItem(file: string, line: number, enabled: boolean): GladeTreeItem {
  const item = new GladeTreeItem(`${path.basename(file)}:${line}`);
  item.description = enabled ? "enabled" : "disabled";
  item.tooltip = file;
  return item;
}

import * as vscode from "vscode";
import { PluginActionRow } from "../plugins/controller";
import { pluginActionTreeRows } from "./pluginsView";
import { commandItem, GladeTreeItem } from "./tree";
import { StartHereRunSummary } from "../startHereModel";

export interface RunsViewState {
  projectReady?: boolean;
  failedTestCount?: number;
  lastRun?: StartHereRunSummary;
  watchRunning?: boolean;
}

export class RunsView implements vscode.TreeDataProvider<GladeTreeItem> {
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  private pluginActions: PluginActionRow[] = [];
  private state: RunsViewState = {};
  readonly onDidChangeTreeData = this.changed.event;

  refresh(): void {
    this.changed.fire();
  }

  setPluginActions(actions: PluginActionRow[]): void {
    this.pluginActions = actions;
    this.refresh();
  }

  setState(state: RunsViewState): void {
    this.state = { ...this.state, ...state };
    this.refresh();
  }

  getTreeItem(element: GladeTreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(): GladeTreeItem[] {
    if (!this.state.projectReady) {
      return [];
    }
    const rows = [
      commandItem("Run changed tests", "glade.runLocalProof", "Run changed local Apex tests and inspect active local data.", new vscode.ThemeIcon("play")),
    ];
    if (this.state.failedTestCount && this.state.failedTestCount > 0) {
      rows.push(commandItem("Run failed tests", "glade.runFailedTests", "Run the failed local Apex tests again.", new vscode.ThemeIcon("debug-rerun")));
    }
    rows.push(
      this.state.watchRunning
        ? commandItem("Stop watch", "glade.stopWatch", "Stop the local Apex watch loop.", new vscode.ThemeIcon("circle-slash"))
        : commandItem("Start watch", "glade.startWatch", "Start the local Apex watch loop.", new vscode.ThemeIcon("eye")),
    );
    return [
      ...rows,
      ...pluginActionTreeRows(this.pluginActions),
    ];
  }
}

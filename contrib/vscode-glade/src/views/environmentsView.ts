import * as path from "path";
import * as vscode from "vscode";
import { GladeEnvironment } from "../environments";
import { configuredActiveEnvironment, configuredEnvironments } from "../localOrg";
import { GladeProjectContext } from "../projectModel";
import { commandItem, GladeTreeItem } from "./tree";

export class EnvironmentsView implements vscode.TreeDataProvider<GladeTreeItem> {
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
    const items: GladeTreeItem[] = [];
    if (this.project) {
      const active = configuredActiveEnvironment(this.project);
      items.push(labelItem(`Active: ${active.name}`, active.dbPath));
      for (const environment of configuredEnvironments(this.project)) {
        items.push(environmentItem(environment, active.name));
      }
    } else {
      items.push(new GladeTreeItem("No SFDX project"));
    }
    items.push(
      commandItem("Create", "glade.createEnvironment", "Create a local data environment.", new vscode.ThemeIcon("add")),
      commandItem("Seed Active", "glade.seedLocalOrg", "Seed the active local data environment.", new vscode.ThemeIcon("cloud-upload")),
      commandItem("Reset Active", "glade.resetLocalOrg", "Reset the active local data environment.", new vscode.ThemeIcon("discard")),
      commandItem("Export Active", "glade.exportLocalOrg", "Export the active local data environment.", new vscode.ThemeIcon("save")),
    );
    return items;
  }
}

function environmentItem(environment: GladeEnvironment, activeName: string): GladeTreeItem {
  const active = environment.name === activeName;
  const item = labelItem(environment.name, active ? "active" : path.basename(environment.dbPath), environment.dbPath);
  item.contextValue = active
    ? environment.name === "dev" ? "gladeEnvironmentActiveDev" : "gladeEnvironmentActive"
    : environment.name === "dev" ? "gladeEnvironmentDev" : "gladeEnvironment";
  item.iconPath = new vscode.ThemeIcon(active ? "check" : "database");
  item.command = {
    command: "glade.switchEnvironment",
    title: "Switch Environment",
    arguments: [environment],
  };
  return item;
}

function labelItem(label: string, description?: string, tooltip?: string): GladeTreeItem {
  const item = new GladeTreeItem(label);
  item.description = description;
  item.tooltip = tooltip || description || label;
  return item;
}

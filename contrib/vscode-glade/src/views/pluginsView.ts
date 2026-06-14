import * as vscode from "vscode";
import { InstalledPlugin } from "../plugins/model";
import { PluginActionRow, PluginArtifactRow, pluginActionRows, pluginArtifactRows } from "../plugins/controller";
import { commandItem, GladeTreeItem } from "./tree";

export class PluginsView implements vscode.TreeDataProvider<GladeTreeItem> {
  private readonly changed = new vscode.EventEmitter<GladeTreeItem | undefined | null | void>();
  private plugins: InstalledPlugin[] = [];
  private actions: PluginActionRow[] = [];
  private artifacts: PluginArtifactRow[] = [];
  readonly onDidChangeTreeData = this.changed.event;

  setState(plugins: InstalledPlugin[], actionRows: PluginActionRow[], artifacts: PluginArtifactRow[]): void {
    this.plugins = plugins;
    this.actions = actionRows;
    this.artifacts = artifacts;
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
      commandItem("Manage Plugins", "glade.managePlugins", "Manage Glade plugins.", new vscode.ThemeIcon("extensions")),
      commandItem("Refresh", "glade.refreshPlugins", "Refresh installed Glade plugins.", new vscode.ThemeIcon("refresh")),
      commandItem("Link Local Plugin", "glade.linkLocalPlugin", "Link a local Glade plugin executable.", new vscode.ThemeIcon("link")),
      commandItem("Install Plugin Archive", "glade.installPluginArchive", "Install a Glade plugin archive.", new vscode.ThemeIcon("archive")),
      ...this.plugins.map(pluginItem),
      ...this.actions.map(actionItem),
      ...this.artifacts.map(artifactItem),
    ];
  }
}

export function pluginRows(plugins: InstalledPlugin[]): GladeTreeItem[] {
  return plugins.map(pluginItem);
}

export function pluginActionTreeRows(actions: ReturnType<typeof pluginActionRows>): GladeTreeItem[] {
  return actions.map(actionItem);
}

export function pluginArtifactTreeRows(artifacts: ReturnType<typeof pluginArtifactRows>): GladeTreeItem[] {
  return artifacts.map(artifactItem);
}

function pluginItem(plugin: InstalledPlugin): GladeTreeItem {
  const label = plugin.identityName || plugin.canonicalName || plugin.name;
  const item = new GladeTreeItem(label);
  item.description = plugin.linked ? `${plugin.version} linked` : plugin.version;
  item.tooltip = plugin.manifest || plugin.source || label;
  item.iconPath = new vscode.ThemeIcon(plugin.linked ? "link" : "extensions");
  return item;
}

function actionItem(row: PluginActionRow): GladeTreeItem {
  const item = new GladeTreeItem(row.label);
  item.id = row.id;
  item.description = row.description;
  item.tooltip = row.tooltip || row.label;
  item.iconPath = new vscode.ThemeIcon("run");
  item.command = { command: "glade.runPluginAction", title: row.label, arguments: [row.action] };
  return item;
}

function artifactItem(row: PluginArtifactRow): GladeTreeItem {
  const item = new GladeTreeItem(row.label);
  item.id = row.id;
  item.description = row.description;
  item.tooltip = row.tooltip || row.label;
  item.iconPath = new vscode.ThemeIcon("file");
  const target = row.path ? vscode.Uri.file(row.path) : row.uri ? vscode.Uri.parse(row.uri) : undefined;
  if (target) {
    item.command = { command: "vscode.open", title: row.label, arguments: [target] };
  }
  return item;
}

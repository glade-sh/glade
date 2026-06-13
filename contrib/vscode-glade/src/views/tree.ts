import * as vscode from "vscode";

export class GladeTreeItem extends vscode.TreeItem {
  constructor(
    label: string,
    collapsibleState: vscode.TreeItemCollapsibleState = vscode.TreeItemCollapsibleState.None,
  ) {
    super(label, collapsibleState);
  }
}

export function commandItem(label: string, command: string, tooltip?: string, icon?: vscode.ThemeIcon): GladeTreeItem {
  const item = new GladeTreeItem(label);
  item.command = { command, title: label };
  item.tooltip = tooltip || label;
  item.iconPath = icon;
  return item;
}

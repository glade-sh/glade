import * as vscode from "vscode";
import { GladeProjectContext } from "./projectModel";

export class GladeStatus {
  private readonly item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 40);

  constructor(context: vscode.ExtensionContext) {
    context.subscriptions.push(this.item);
  }

  setProject(project: GladeProjectContext | undefined): void {
    if (!project) {
      this.item.text = "Glade: no SFDX root";
      this.item.tooltip = "Open a Salesforce DX project with sfdx-project.json.";
      this.item.show();
      return;
    }
    const namespace = project.namespace ? ` ${project.namespace}` : "";
    this.item.text = `Glade: SFDX root${namespace}`;
    this.item.tooltip = project.projectRoot;
    this.item.show();
  }
}

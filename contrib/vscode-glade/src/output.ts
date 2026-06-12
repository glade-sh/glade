import * as vscode from "vscode";

export class GladeOutput {
  readonly tests = vscode.window.createOutputChannel("Glade Tests");
  readonly logs = vscode.window.createOutputChannel("Glade");

  dispose(): void {
    this.tests.dispose();
    this.logs.dispose();
  }
}

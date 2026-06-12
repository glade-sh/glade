import * as vscode from "vscode";
import { discoverApexTests } from "./tests/discovery";

export interface LocalTestTarget {
  className: string;
  methodName?: string;
}

export class GladeCodeLensProvider implements vscode.CodeLensProvider {
  provideCodeLenses(document: vscode.TextDocument): vscode.CodeLens[] {
    if (!vscode.workspace.getConfiguration("glade").get<boolean>("enableCodeLens", true)) {
      return [];
    }
    const source = document.getText();
    const discovered = discoverApexTests(document.uri.fsPath, source);
    if (!discovered) {
      return [];
    }
    const lenses: vscode.CodeLens[] = [];
    const classRange = rangeForMatch(document, source, new RegExp(`\\bclass\\s+${escapeRegExp(discovered.className)}\\b`));
    lenses.push(runLens(classRange, { className: discovered.className }, discovered.className));
    for (const methodName of discovered.methods) {
      const methodRange = rangeForMatch(document, source, new RegExp(`\\b${escapeRegExp(methodName)}\\s*\\(`));
      const target = { className: discovered.className, methodName };
      lenses.push(runLens(methodRange, target, `${discovered.className}.${methodName}`));
      lenses.push(debugLens(methodRange, target, `${discovered.className}.${methodName}`));
    }
    return lenses;
  }
}

function runLens(range: vscode.Range, target: LocalTestTarget, label: string): vscode.CodeLens {
  return new vscode.CodeLens(range, {
    title: `Run Local Test ${label}`,
    command: "glade.runLocalTestFromCodeLens",
    arguments: [target],
  });
}

function debugLens(range: vscode.Range, target: LocalTestTarget, label: string): vscode.CodeLens {
  return new vscode.CodeLens(range, {
    title: `Debug Local Test ${label}`,
    command: "glade.debugLocalTestFromCodeLens",
    arguments: [target],
  });
}

function rangeForMatch(document: vscode.TextDocument, source: string, pattern: RegExp): vscode.Range {
  const match = pattern.exec(source);
  const position = document.positionAt(match?.index || 0);
  return new vscode.Range(position, position);
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

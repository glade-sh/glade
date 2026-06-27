import * as vscode from "vscode";
import { EditorDiagnostic } from "./model";
import { toVsRange } from "./ranges";

export class ApexLogDiagnostics {
  readonly collection: vscode.DiagnosticCollection;

  constructor() {
    this.collection = vscode.languages.createDiagnosticCollection("glade-apexlog");
  }

  update(document: vscode.TextDocument, diagnostics: EditorDiagnostic[]): void {
    this.collection.set(document.uri, diagnostics.map(toDiagnostic));
  }

  clear(document: vscode.TextDocument): void {
    this.collection.delete(document.uri);
  }

  clearAll(): void {
    this.collection.clear();
  }

  dispose(): void {
    this.collection.dispose();
  }
}

function toDiagnostic(input: EditorDiagnostic): vscode.Diagnostic {
  const diagnostic = new vscode.Diagnostic(toVsRange(input.range), input.message, severity(input.severity));
  diagnostic.code = input.code;
  diagnostic.source = "glade";
  return diagnostic;
}

function severity(value: string): vscode.DiagnosticSeverity {
  switch (value) {
    case "error":
      return vscode.DiagnosticSeverity.Error;
    case "information":
      return vscode.DiagnosticSeverity.Information;
    case "hint":
      return vscode.DiagnosticSeverity.Hint;
    default:
      return vscode.DiagnosticSeverity.Warning;
  }
}

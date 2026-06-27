import * as vscode from "vscode";
import { EditorRange } from "./model";

export function toVsRange(range: EditorRange): vscode.Range {
  return new vscode.Range(
    new vscode.Position(range.startLine, range.startColumn),
    new vscode.Position(range.endLine, range.endColumn),
  );
}

export function contains(range: EditorRange, position: vscode.Position): boolean {
  return toVsRange(range).contains(position);
}

export function rangeWidth(range: EditorRange): number {
  if (range.startLine !== range.endLine) {
    return Number.MAX_SAFE_INTEGER;
  }
  return Math.max(0, range.endColumn - range.startColumn);
}

export function clampRangeToDocument(range: EditorRange, document: { lineAt(line: number): { text: string }; lineCount: number }): EditorRange | undefined {
  if (document.lineCount === 0) {
    return undefined;
  }
  const startLine = clamp(range.startLine, 0, document.lineCount - 1);
  const endLine = clamp(range.endLine, startLine, document.lineCount - 1);
  const startLength = document.lineAt(startLine).text.length;
  const endLength = document.lineAt(endLine).text.length;
  return {
    startLine,
    startColumn: clamp(range.startColumn, 0, startLength),
    endLine,
    endColumn: clamp(range.endColumn, 0, endLength),
  };
}

function clamp(value: number, min: number, max: number): number {
  if (value < min) {
    return min;
  }
  if (value > max) {
    return max;
  }
  return value;
}

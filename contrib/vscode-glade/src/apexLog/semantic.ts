import * as vscode from "vscode";

export const semanticTokenTypes = [
  "timestamp",
  "event",
  "class",
  "method",
  "variable",
  "type",
  "string",
  "number",
  "keyword",
  "property",
  "enumMember",
  "function",
  "comment",
];

export const semanticTokenModifiers = ["lowConfidence"];

export const semanticLegend = new vscode.SemanticTokensLegend(semanticTokenTypes, semanticTokenModifiers);

import * as vscode from "vscode";

export interface GladeBreakpointSummary {
  file: string;
  line: number;
  enabled: boolean;
}

export function apexBreakpoints(): GladeBreakpointSummary[] {
  return vscode.debug.breakpoints
    .filter((breakpoint): breakpoint is vscode.SourceBreakpoint => breakpoint instanceof vscode.SourceBreakpoint)
    .filter((breakpoint) => /\.(cls|trigger)$/i.test(breakpoint.location.uri.fsPath))
    .map((breakpoint) => ({
      file: breakpoint.location.uri.fsPath,
      line: breakpoint.location.range.start.line + 1,
      enabled: breakpoint.enabled,
    }));
}

import * as fs from "fs";
import * as vscode from "vscode";
import { GladeRunOptions, runGladeJSON } from "../gladeCli";
import { findProjectContext } from "../projectContext";
import { EditorAnalysis, EditorDiagnostic } from "./model";

type AnalysisState =
  | { kind: "missing"; reason: string; diagnostics: EditorDiagnostic[] }
  | { kind: "loading"; promise: Promise<EditorAnalysis | undefined> }
  | { kind: "ready"; analysis: EditorAnalysis }
  | { kind: "failed"; message: string; atVersion: number; diagnostics: EditorDiagnostic[] };

export interface ApexLogAnalysisCacheDeps {
  findProjectContext?: typeof findProjectContext;
  runGladeJSON?: typeof runGladeJSON;
  maxAnalysisBytes?: () => number;
  smartFeaturesEnabled?: () => boolean;
}

export class ApexLogAnalysisCache {
  private readonly states = new Map<string, AnalysisState>();
  private readonly findContext: typeof findProjectContext;
  private readonly runJSON: typeof runGladeJSON;
  private readonly maxBytes: () => number;
  private readonly enabled: () => boolean;

  constructor(deps: ApexLogAnalysisCacheDeps = {}) {
    this.findContext = deps.findProjectContext || findProjectContext;
    this.runJSON = deps.runGladeJSON || runGladeJSON;
    this.maxBytes = deps.maxAnalysisBytes || (() => vscode.workspace.getConfiguration("glade").get<number>("apexLog.maxAnalysisBytes", 10 * 1024 * 1024));
    this.enabled = deps.smartFeaturesEnabled || (() => vscode.workspace.getConfiguration("glade").get<boolean>("apexLog.smartFeatures.enabled", true));
  }

  async getAnalysis(document: vscode.TextDocument): Promise<EditorAnalysis | undefined> {
    if (!this.enabled()) {
      this.clear(document);
      return undefined;
    }
    const size = fileSize(document.uri.fsPath);
    if (size > this.maxBytes()) {
      const key = this.cacheKey(document, "");
      const diagnostic = missingDiagnostic(`Apex log is too large for smart analysis (${size} bytes).`);
      this.states.set(key, { kind: "missing", reason: diagnostic.message, diagnostics: [diagnostic] });
      return undefined;
    }
    let projectRoot: string | undefined;
    try {
      const project = await this.findContext();
      projectRoot = project?.projectRoot;
    } catch (error) {
      const key = this.cacheKey(document, "");
      const message = error instanceof Error ? error.message : String(error);
      this.states.set(key, { kind: "failed", message, atVersion: document.version, diagnostics: [missingDiagnostic(message)] });
      return undefined;
    }
    const key = this.cacheKey(document, projectRoot || "");
    const existing = this.states.get(key);
    if (existing?.kind === "ready") {
      return existing.analysis;
    }
    if (existing?.kind === "loading") {
      return existing.promise;
    }
    const promise = this.load(document, key, projectRoot);
    this.states.set(key, { kind: "loading", promise });
    return promise;
  }

  clear(document?: vscode.TextDocument): void {
    if (!document) {
      this.states.clear();
      return;
    }
    const prefix = `${document.uri.toString()}|`;
    for (const key of [...this.states.keys()]) {
      if (key.startsWith(prefix)) {
        this.states.delete(key);
      }
    }
  }

  diagnosticsFor(document: vscode.TextDocument): EditorDiagnostic[] {
    const prefix = `${document.uri.toString()}|`;
    for (const [key, state] of this.states) {
      if (!key.startsWith(prefix)) {
        continue;
      }
      if (state.kind === "missing" || state.kind === "failed") {
        return state.diagnostics;
      }
      if (state.kind === "ready") {
        return state.analysis.diagnostics || [];
      }
    }
    return [];
  }

  private async load(document: vscode.TextDocument, key: string, projectRoot?: string): Promise<EditorAnalysis | undefined> {
    try {
      const args = ["debug", "editor", "--log", document.uri.fsPath, "--json"];
      if (projectRoot) {
        args.splice(4, 0, "--project", projectRoot);
      }
      const analysis = await this.runJSON<EditorAnalysis>(
        args,
        { cwd: projectRoot || process.cwd() } as GladeRunOptions,
        "glade debug editor",
      );
      if (!this.enabled()) {
        this.clear(document);
        return undefined;
      }
      this.states.set(key, { kind: "ready", analysis });
      return analysis;
    } catch (error) {
      if (!this.enabled()) {
        this.clear(document);
        return undefined;
      }
      const message = error instanceof Error ? error.message : String(error);
      this.states.set(key, { kind: "failed", message, atVersion: document.version, diagnostics: [missingDiagnostic(message)] });
      return undefined;
    }
  }

  private cacheKey(document: vscode.TextDocument, projectRoot: string): string {
    return `${document.uri.toString()}|${document.version}|${projectRoot}|${this.maxBytes()}`;
  }
}

function fileSize(path: string): number {
  try {
    return fs.statSync(path).size;
  } catch {
    return 0;
  }
}

function missingDiagnostic(message: string): EditorDiagnostic {
  return {
    severity: "warning",
    code: "apexlog.analysis",
    message,
    range: { startLine: 0, startColumn: 0, endLine: 0, endColumn: 1 },
  };
}

export interface EditorAnalysis {
  version: number;
  logFile?: string;
  projectRoot?: string;
  language: "apexlog";
  generatedAt: string;
  entries: EditorEntry[];
  symbols: EditorSymbol[];
  folds: EditorFold[];
  links: EditorLink[];
  hovers: EditorHover[];
  codeLenses: EditorCodeLens[];
  semanticTokens: EditorToken[];
  diagnostics: EditorDiagnostic[];
  variables: EditorVariable[];
  replayFrames: EditorReplayFrame[];
  coverage: EditorCoverage;
}

export interface EditorRange {
  startLine: number;
  startColumn: number;
  endLine: number;
  endColumn: number;
}

export interface EditorLocation {
  file?: string;
  line?: number;
  column?: number;
  symbol?: string;
  reason?: string;
  confidence: number;
}

export interface EditorEntry {
  index: number;
  kind: string;
  raw: string;
  range: EditorRange;
  depth: number;
  frameId?: string;
  parentId?: string;
  fields?: Record<string, unknown>;
  source?: EditorLocation;
  lowDetail?: boolean;
}

export interface EditorSymbol {
  name: string;
  kind: string;
  range: EditorRange;
  selectionRange: EditorRange;
  detail?: string;
  source?: EditorLocation;
  children?: EditorSymbol[];
}

export interface EditorFold {
  kind: string;
  range: EditorRange;
  collapsedText?: string;
  depth: number;
}

export type ApexLogLinkKind =
  | "source"
  | "method"
  | "variableSource"
  | "variableLog"
  | "schemaObject"
  | "schemaField"
  | "replayFrame";

export interface EditorLink {
  kind: ApexLogLinkKind | string;
  range: EditorRange;
  target?: EditorLocation;
  command?: string;
  title?: string;
}

export interface EditorHover {
  range: EditorRange;
  markdown: string;
}

export interface EditorCodeLens {
  range: EditorRange;
  command: string;
  title: string;
  arguments?: string[];
}

export interface EditorToken {
  range: EditorRange;
  tokenType: string;
  modifiers?: string[];
}

export interface EditorDiagnostic {
  range: EditorRange;
  severity: "error" | "warning" | "information" | "hint" | string;
  code: string;
  message: string;
}

export interface EditorVariable {
  name: string;
  type?: string;
  value?: string;
  scopeId?: string;
  range: EditorRange;
  logDefinition?: EditorLocation;
  sourceDefinition?: EditorLocation;
  assignment?: EditorLocation;
}

export interface EditorReplayFrame {
  frameId: string;
  entryIndex: number;
  range: EditorRange;
  canReplay: boolean;
  reason?: string;
}

export interface EditorCoverage {
  totalEntries: number;
  resolvedSources: number;
  resolvedVariables: number;
  resolvedSchemaRefs: number;
  parserWarnings: number;
}

import * as vscode from "vscode";
import {
  EditorAnalysis,
  EditorCodeLens,
  EditorEntry,
  EditorFold,
  EditorHover,
  EditorLink,
  EditorLocation,
  EditorSymbol,
  EditorToken,
} from "./model";
import { clampRangeToDocument, contains, rangeWidth, toVsRange } from "./ranges";
import { semanticLegend } from "./semantic";

export interface ApexLogAnalysisCacheLike {
  getAnalysis(document: vscode.TextDocument): Promise<EditorAnalysis | undefined>;
}

export interface ApexLogProviderDeps {
  cache: ApexLogAnalysisCacheLike;
}

export function registerApexLogProviders(context: vscode.ExtensionContext, deps: ApexLogProviderDeps): vscode.Disposable[] {
  const selector: vscode.DocumentSelector = [
    { language: "apexlog", scheme: "file" },
    { language: "apexlog", scheme: "untitled" },
  ];
  const disposables = [
    vscode.languages.registerDefinitionProvider(selector, new ApexLogDefinitionProvider(deps.cache)),
    vscode.languages.registerDocumentLinkProvider(selector, new ApexLogDocumentLinkProvider(deps.cache)),
    vscode.languages.registerFoldingRangeProvider(selector, new ApexLogFoldingRangeProvider(deps.cache)),
    vscode.languages.registerDocumentSymbolProvider(selector, new ApexLogDocumentSymbolProvider(deps.cache)),
    vscode.languages.registerHoverProvider(selector, new ApexLogHoverProvider(deps.cache)),
    vscode.languages.registerCodeLensProvider(selector, new ApexLogCodeLensProvider(deps.cache)),
    vscode.languages.registerDocumentSemanticTokensProvider(selector, new ApexLogSemanticTokensProvider(deps.cache), semanticLegend),
  ];
  context.subscriptions.push(...disposables);
  return disposables;
}

export class ApexLogDefinitionProvider implements vscode.DefinitionProvider {
  constructor(private readonly cache: ApexLogAnalysisCacheLike) {}

  async provideDefinition(document: vscode.TextDocument, position: vscode.Position): Promise<vscode.LocationLink[] | undefined> {
    const analysis = await this.cache.getAnalysis(document);
    if (!analysis) {
      return undefined;
    }
    const link = bestLinkAt(analysis.links, position);
    if (link?.target?.file) {
      return [locationLink(link.target, link.range)];
    }
    const entry = analysis.entries.find((candidate) => contains(candidate.range, position) && candidate.source?.file);
    if (entry?.source?.file) {
      return [locationLink(entry.source, entry.range)];
    }
    return undefined;
  }
}

export class ApexLogDocumentLinkProvider implements vscode.DocumentLinkProvider {
  constructor(private readonly cache: ApexLogAnalysisCacheLike) {}

  async provideDocumentLinks(document: vscode.TextDocument): Promise<vscode.DocumentLink[]> {
    const analysis = await this.cache.getAnalysis(document);
    if (!analysis) {
      return [];
    }
    return analysis.links
      .map((link) => documentLink(link))
      .filter((link): link is vscode.DocumentLink => Boolean(link));
  }
}

export class ApexLogFoldingRangeProvider implements vscode.FoldingRangeProvider {
  constructor(private readonly cache: ApexLogAnalysisCacheLike) {}

  async provideFoldingRanges(document: vscode.TextDocument): Promise<vscode.FoldingRange[]> {
    const analysis = await this.cache.getAnalysis(document);
    if (!analysis) {
      return [];
    }
    return nonCrossingFolds(analysis.folds)
      .filter((fold) => fold.range.endLine > fold.range.startLine)
      .map((fold) => new vscode.FoldingRange(fold.range.startLine, fold.range.endLine, foldingKind(fold.kind)));
  }
}

export class ApexLogDocumentSymbolProvider implements vscode.DocumentSymbolProvider {
  constructor(private readonly cache: ApexLogAnalysisCacheLike) {}

  async provideDocumentSymbols(document: vscode.TextDocument): Promise<vscode.DocumentSymbol[]> {
    const analysis = await this.cache.getAnalysis(document);
    if (!analysis) {
      return [];
    }
    return analysis.symbols.map(toDocumentSymbol);
  }
}

export class ApexLogHoverProvider implements vscode.HoverProvider {
  constructor(private readonly cache: ApexLogAnalysisCacheLike) {}

  async provideHover(document: vscode.TextDocument, position: vscode.Position): Promise<vscode.Hover | undefined> {
    const analysis = await this.cache.getAnalysis(document);
    if (!analysis) {
      return undefined;
    }
    const hover = bestHoverAt(analysis.hovers, position);
    if (!hover) {
      return undefined;
    }
    const markdown = new vscode.MarkdownString(hover.markdown);
    markdown.isTrusted = false;
    const link = bestLinkAt(analysis.links, position);
    if (link?.target?.file) {
      markdown.appendMarkdown(`\n\nSource: ${link.target.file}:${link.target.line ?? 0}`);
    }
    return new vscode.Hover(markdown, toVsRange(hover.range));
  }
}

export class ApexLogCodeLensProvider implements vscode.CodeLensProvider {
  constructor(private readonly cache: ApexLogAnalysisCacheLike) {}

  async provideCodeLenses(document: vscode.TextDocument): Promise<vscode.CodeLens[]> {
    if (!vscode.workspace.getConfiguration("glade").get<boolean>("apexLog.codeLens.enabled", true)) {
      return [];
    }
    const analysis = await this.cache.getAnalysis(document);
    if (!analysis) {
      return [];
    }
    return codeLensesFromAnalysis(analysis, document);
  }
}

export class ApexLogSemanticTokensProvider implements vscode.DocumentSemanticTokensProvider {
  constructor(private readonly cache: ApexLogAnalysisCacheLike) {}

  async provideDocumentSemanticTokens(document: vscode.TextDocument): Promise<vscode.SemanticTokens> {
    const analysis = await this.cache.getAnalysis(document);
    const builder = new vscode.SemanticTokensBuilder(semanticLegend);
    if (!analysis) {
      return builder.build();
    }
    for (const token of analysis.semanticTokens) {
      pushToken(builder, document, token);
    }
    return builder.build();
  }
}

function bestLinkAt(links: EditorLink[], position: vscode.Position): EditorLink | undefined {
  const matches = links.filter((link) => contains(link.range, position));
  matches.sort((a, b) => {
    const priority = linkPriority(b) - linkPriority(a);
    if (priority !== 0) {
      return priority;
    }
    return rangeWidth(a.range) - rangeWidth(b.range);
  });
  return matches[0];
}

function linkPriority(link: EditorLink): number {
  if (link.kind === "variableSource") {
    return 100;
  }
  if (link.kind === "variableLog") {
    return 90;
  }
  if (link.kind === "method" || link.kind === "source") {
    return 80;
  }
  return 10;
}

function bestHoverAt(hovers: EditorHover[], position: vscode.Position): EditorHover | undefined {
  const matches = hovers.filter((hover) => contains(hover.range, position));
  matches.sort((a, b) => rangeWidth(a.range) - rangeWidth(b.range));
  return matches[0];
}

function locationLink(target: EditorLocation, origin: { startLine: number; startColumn: number; endLine: number; endColumn: number }): vscode.LocationLink {
  const targetLine = Math.max(0, (target.line ?? 1) - 1);
  const targetColumn = Math.max(0, target.column ?? 0);
  const targetRange = new vscode.Range(targetLine, targetColumn, targetLine, targetColumn);
  return {
    originSelectionRange: toVsRange(origin),
    targetUri: vscode.Uri.file(target.file || ""),
    targetRange,
    targetSelectionRange: targetRange,
  };
}

function documentLink(link: EditorLink): vscode.DocumentLink | undefined {
  let target: vscode.Uri | undefined;
  if (link.target?.file) {
    target = vscode.Uri.file(link.target.file);
  } else if (link.command && link.command.startsWith("glade.")) {
    target = vscode.Uri.parse(`command:${link.command}`);
  }
  if (!target) {
    return undefined;
  }
  const out = new vscode.DocumentLink(toVsRange(link.range), target);
  out.tooltip = linkTooltip(link);
  return out;
}

function linkTooltip(link: EditorLink): string {
  switch (link.kind) {
    case "schemaObject":
      return "Open schema object";
    case "schemaField":
      return "Open schema field";
    case "variableSource":
    case "variableLog":
      return "Open variable definition";
    case "replayFrame":
      return "Replay from this frame";
    default:
      return "Open Apex source";
  }
}

function foldingKind(kind: string): vscode.FoldingRangeKind | undefined {
  if (kind === "exception") {
    return vscode.FoldingRangeKind.Comment;
  }
  return vscode.FoldingRangeKind.Region;
}

function nonCrossingFolds(folds: EditorFold[]): EditorFold[] {
  const sorted = [...folds].sort((a, b) => a.range.startLine - b.range.startLine || b.range.endLine - a.range.endLine);
  const out: EditorFold[] = [];
  for (const fold of sorted) {
    const crosses = out.some((existing) =>
      existing.range.startLine < fold.range.startLine
      && fold.range.startLine <= existing.range.endLine
      && existing.range.endLine < fold.range.endLine,
    );
    if (!crosses) {
      out.push(fold);
    }
  }
  return out;
}

function toDocumentSymbol(symbol: EditorSymbol): vscode.DocumentSymbol {
  const out = new vscode.DocumentSymbol(
    symbol.name,
    symbol.detail || "",
    symbolKind(symbol.kind),
    toVsRange(symbol.range),
    toVsRange(symbol.selectionRange),
  );
  out.children = (symbol.children || []).map(toDocumentSymbol);
  return out;
}

function symbolKind(kind: string): vscode.SymbolKind {
  switch (kind) {
    case "execution":
      return vscode.SymbolKind.Namespace;
    case "codeUnit":
      return vscode.SymbolKind.Class;
    case "method":
      return vscode.SymbolKind.Method;
    case "constructor":
      return vscode.SymbolKind.Constructor;
    case "soql":
      return vscode.SymbolKind.Object;
    case "limits":
      return vscode.SymbolKind.Number;
    case "dml":
    case "exception":
    case "fatalError":
      return vscode.SymbolKind.Event;
    default:
      return vscode.SymbolKind.Event;
  }
}

function codeLensesFromAnalysis(analysis: EditorAnalysis, document: vscode.TextDocument): vscode.CodeLens[] {
  const out: vscode.CodeLens[] = [];
  const seenSource = new Set<string>();
  for (const link of analysis.links) {
    if (!isSourceLensLink(link) || !link.target?.file) {
      continue;
    }
    const key = `${link.range.startLine}:${link.target.file}:${link.target.line ?? 0}`;
    if (seenSource.has(key)) {
      continue;
    }
    seenSource.add(key);
    const targetUri = vscode.Uri.file(link.target.file);
    const targetPosition = new vscode.Position(Math.max(0, (link.target.line ?? 1) - 1), Math.max(0, link.target.column ?? 0));
    const originRange = toVsRange(link.range);
    out.push(new vscode.CodeLens(originRange, {
      command: "vscode.open",
      title: "Open Source",
      arguments: [targetUri, { selection: new vscode.Range(targetPosition, targetPosition) }],
    }));
    out.push(new vscode.CodeLens(originRange, {
      command: "editor.action.peekLocations",
      title: "Peek Source",
      arguments: [document.uri, originRange.start, [locationLink(link.target, link.range)], "peek"],
    }));
  }
  for (const frame of analysis.replayFrames || []) {
    if (!frame.canReplay) {
      continue;
    }
    out.push(new vscode.CodeLens(toVsRange(frame.range), {
      command: "glade.apexLog.replayFromFrame",
      title: "Replay From This Frame",
      arguments: [frame.entryIndex],
    }));
  }
  for (const lens of analysis.codeLenses || []) {
    const safe = toSafeBackendCodeLens(lens);
    if (safe) {
      out.push(safe);
    }
  }
  return out;
}

function isSourceLensLink(link: EditorLink): boolean {
  return link.kind === "source" || link.kind === "method" || link.kind === "variableSource";
}

function toSafeBackendCodeLens(lens: EditorCodeLens): vscode.CodeLens | undefined {
  if (!lens.command.startsWith("glade.apexLog.") && !lens.command.startsWith("vscode.")) {
    return undefined;
  }
  return new vscode.CodeLens(toVsRange(lens.range), {
    command: lens.command,
    title: lens.title,
    arguments: lens.arguments,
  });
}

function pushToken(builder: vscode.SemanticTokensBuilder, document: vscode.TextDocument, token: EditorToken): void {
  const range = clampRangeToDocument(token.range, document);
  if (!range) {
    return;
  }
  if (range.startLine !== range.endLine) {
    for (let line = range.startLine; line <= range.endLine; line++) {
      const text = document.lineAt(line).text;
      if (text.length > 0) {
        builder.push(new vscode.Range(line, 0, line, text.length), token.tokenType, token.modifiers || []);
      }
    }
    return;
  }
  if (range.endColumn > range.startColumn) {
    builder.push(toVsRange(range), token.tokenType, token.modifiers || []);
  }
}

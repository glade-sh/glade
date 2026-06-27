const assert = require("assert");
const Module = require("module");

const originalLoad = Module._load;
Module._load = function patchedLoad(request, parent, isMain) {
  if (request === "vscode") {
    class Position {
      constructor(line, character) {
        this.line = line;
        this.character = character;
      }
    }
    class Range {
      constructor(start, endOrStartColumn, endLine, endColumn) {
        if (start instanceof Position) {
          this.start = start;
          this.end = endOrStartColumn;
        } else {
          this.start = new Position(start, endOrStartColumn);
          this.end = new Position(endLine, endColumn);
        }
      }
      contains(position) {
        if (position.line < this.start.line || position.line > this.end.line) return false;
        if (position.line === this.start.line && position.character < this.start.character) return false;
        if (position.line === this.end.line && position.character > this.end.character) return false;
        return true;
      }
    }
    class Uri {
      static file(fsPath) {
        return { fsPath, scheme: "file", toString: () => `file://${fsPath}` };
      }
      static parse(value) {
        return { value, toString: () => value };
      }
    }
    class MarkdownString {
      constructor(value) {
        this.value = value;
      }
      appendMarkdown(value) {
        this.value += value;
      }
    }
    class Hover {
      constructor(contents, range) {
        this.contents = contents;
        this.range = range;
      }
    }
    class DocumentLink {
      constructor(range, target) {
        this.range = range;
        this.target = target;
      }
    }
    class CodeLens {
      constructor(range, command) {
        this.range = range;
        this.command = command;
      }
    }
    class DocumentSymbol {
      constructor(name, detail, kind, range, selectionRange) {
        this.name = name;
        this.detail = detail;
        this.kind = kind;
        this.range = range;
        this.selectionRange = selectionRange;
        this.children = [];
      }
    }
    class FoldingRange {
      constructor(start, end, kind) {
        this.start = start;
        this.end = end;
        this.kind = kind;
      }
    }
    class SemanticTokensBuilder {
      constructor() {
        this.tokens = [];
      }
      push(range, tokenType, tokenModifiers) {
        this.tokens.push({ range, tokenType, tokenModifiers });
      }
      build() {
        return { tokens: this.tokens };
      }
    }
    return {
      Position,
      Range,
      Uri,
      MarkdownString,
      Hover,
      DocumentLink,
      CodeLens,
      DocumentSymbol,
      FoldingRange,
      FoldingRangeKind: { Region: "region", Comment: "comment" },
      SymbolKind: { Namespace: 1, Class: 2, Method: 3, Constructor: 4, Object: 5, Event: 6, Number: 7 },
      SemanticTokensBuilder,
      SemanticTokensLegend: class SemanticTokensLegend {
        constructor(types, modifiers) {
          this.types = types;
          this.modifiers = modifiers;
        }
      },
      LocationLink: undefined,
      languages: {},
      workspace: { getConfiguration: () => ({ get: (_key, fallback) => fallback }) },
    };
  }
  return originalLoad.call(this, request, parent, isMain);
};

const {
  ApexLogDefinitionProvider,
  ApexLogDocumentLinkProvider,
  ApexLogFoldingRangeProvider,
  ApexLogDocumentSymbolProvider,
  ApexLogHoverProvider,
  ApexLogCodeLensProvider,
  ApexLogSemanticTokensProvider,
} = require("../out/apexLog/providers");

const analysis = {
  links: [
    {
      kind: "variableLog",
      range: { startLine: 1, startColumn: 0, endLine: 1, endColumn: 30 },
      target: { file: "/repo/apex.log", line: 2, column: 0, confidence: 1 },
      title: "Open variable scope",
    },
    {
      kind: "variableSource",
      range: { startLine: 1, startColumn: 5, endLine: 1, endColumn: 10 },
      target: { file: "/repo/force-app/main/default/classes/Test.cls", line: 4, column: 0, confidence: 0.8 },
      title: "Open variable definition",
    },
  ],
  entries: [
    {
      range: { startLine: 2, startColumn: 0, endLine: 2, endColumn: 20 },
      source: { file: "/repo/force-app/main/default/classes/Test.cls", line: 8, column: 0, confidence: 0.5 },
    },
  ],
  folds: [{ kind: "method", range: { startLine: 0, startColumn: 0, endLine: 4, endColumn: 0 }, depth: 1 }],
  symbols: [
    {
      name: "Test.run",
      kind: "method",
      detail: "frame",
      range: { startLine: 0, startColumn: 0, endLine: 4, endColumn: 0 },
      selectionRange: { startLine: 0, startColumn: 0, endLine: 0, endColumn: 10 },
      children: [{ name: "SOQL", kind: "soql", range: { startLine: 2, startColumn: 0, endLine: 3, endColumn: 0 }, selectionRange: { startLine: 2, startColumn: 0, endLine: 2, endColumn: 4 } }],
    },
  ],
  hovers: [{ range: { startLine: 1, startColumn: 5, endLine: 1, endColumn: 10 }, markdown: "**VARIABLE** `a`" }],
  codeLenses: [],
  semanticTokens: [{ range: { startLine: 1, startColumn: 0, endLine: 1, endColumn: 10 }, tokenType: "variable", modifiers: [] }],
  replayFrames: [{ frameId: "frame:0:test-run", entryIndex: 0, range: { startLine: 0, startColumn: 0, endLine: 4, endColumn: 0 }, canReplay: true }],
};

const cache = {
  async getAnalysis() {
    return analysis;
  },
};
const document = {
  uri: { fsPath: "/repo/apex.log" },
  lineCount: 5,
  lineAt(line) {
    return { text: ["METHOD_ENTRY", "     a = 1", "USER_DEBUG", "METHOD_EXIT", ""][line] || "" };
  },
};

(async () => {
  const definition = await new ApexLogDefinitionProvider(cache).provideDefinition(document, { line: 1, character: 6 });
  assert.strictEqual(definition[0].targetUri.fsPath, "/repo/force-app/main/default/classes/Test.cls");

  const logDefinition = await new ApexLogDefinitionProvider(cache).provideDefinition(document, { line: 1, character: 20 });
  assert.strictEqual(logDefinition[0].targetUri.fsPath, "/repo/apex.log");
  assert.strictEqual(logDefinition[0].targetRange.start.line, 1);

  const fallback = await new ApexLogDefinitionProvider(cache).provideDefinition(document, { line: 2, character: 1 });
  assert.strictEqual(fallback[0].targetUri.fsPath, "/repo/force-app/main/default/classes/Test.cls");

  const links = await new ApexLogDocumentLinkProvider(cache).provideDocumentLinks(document);
  assert.strictEqual(links.length, 2);
  assert.strictEqual(links[1].tooltip, "Open variable definition");

  const folds = await new ApexLogFoldingRangeProvider(cache).provideFoldingRanges(document);
  assert.deepStrictEqual(folds.map((fold) => [fold.start, fold.end, fold.kind]), [[0, 4, "region"]]);

  const symbols = await new ApexLogDocumentSymbolProvider(cache).provideDocumentSymbols(document);
  assert.strictEqual(symbols[0].name, "Test.run");
  assert.strictEqual(symbols[0].children[0].name, "SOQL");

  const hover = await new ApexLogHoverProvider(cache).provideHover(document, { line: 1, character: 6 });
  assert(hover.contents.value.includes("VARIABLE"));

  const lenses = await new ApexLogCodeLensProvider(cache).provideCodeLenses(document);
  assert.strictEqual(lenses[0].command.title, "Open Source");
  assert(lenses.some((lens) => lens.command.title === "Replay From This Frame" && lens.command.arguments[0] === 0));

  const tokens = await new ApexLogSemanticTokensProvider(cache).provideDocumentSemanticTokens(document);
  assert.strictEqual(tokens.tokens.length, 1);

  Module._load = originalLoad;
})().catch((error) => {
  Module._load = originalLoad;
  throw error;
});

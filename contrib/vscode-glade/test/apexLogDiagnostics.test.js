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
      constructor(start, end) {
        this.start = start;
        this.end = end;
      }
    }
    class Diagnostic {
      constructor(range, message, severity) {
        this.range = range;
        this.message = message;
        this.severity = severity;
      }
    }
    return {
      Position,
      Range,
      Diagnostic,
      DiagnosticSeverity: { Error: 0, Warning: 1, Information: 2, Hint: 3 },
      languages: {
        createDiagnosticCollection: () => ({
          sets: [],
          deletes: [],
          set(uri, diagnostics) {
            this.sets.push({ uri, diagnostics });
          },
          delete(uri) {
            this.deletes.push(uri);
          },
          dispose() {},
        }),
      },
    };
  }
  return originalLoad.call(this, request, parent, isMain);
};

const { ApexLogDiagnostics } = require("../out/apexLog/diagnostics");

const diagnostics = new ApexLogDiagnostics();
const document = { uri: { fsPath: "/repo/apex.log" } };
diagnostics.update(document, [
  { severity: "error", code: "apexlog.exception", message: "Boom", range: { startLine: 1, startColumn: 0, endLine: 1, endColumn: 10 } },
  { severity: "warning", code: "apexlog.lowDetail", message: "Low detail", range: { startLine: 2, startColumn: 0, endLine: 2, endColumn: 10 } },
]);
assert.strictEqual(diagnostics.collection.sets[0].diagnostics.length, 2);
assert.strictEqual(diagnostics.collection.sets[0].diagnostics[0].code, "apexlog.exception");
diagnostics.clear(document);
assert.strictEqual(diagnostics.collection.deletes[0], document.uri);

Module._load = originalLoad;

const assert = require("assert");
const fs = require("fs");
const os = require("os");
const path = require("path");
const Module = require("module");

const originalLoad = Module._load;
Module._load = function patchedLoad(request, parent, isMain) {
  if (request === "vscode") {
    return {
      workspace: {
        getConfiguration: () => ({
          get: (key, fallback) => fallback,
        }),
      },
      Uri: { file: (fsPath) => ({ fsPath }) },
    };
  }
  return originalLoad.call(this, request, parent, isMain);
};

const { ApexLogAnalysisCache } = require("../out/apexLog/cache");

(async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "glade-apexlog-cache-"));
  const logPath = path.join(dir, "apex.log");
  fs.writeFileSync(logPath, "00:00:00.001 (1)|METHOD_ENTRY|[1]|01p|Test.run()\n");
  let calls = 0;
  const cache = new ApexLogAnalysisCache({
    findProjectContext: async () => ({ projectRoot: dir }),
    runGladeJSON: async (args, options, label) => {
      calls++;
      assert.deepStrictEqual(args, ["debug", "editor", "--log", logPath, "--project", dir, "--json"]);
      assert.strictEqual(options.cwd, dir);
      assert.strictEqual(label, "glade debug editor");
      return { version: 1, language: "apexlog", generatedAt: "now", entries: [], symbols: [], folds: [], links: [], hovers: [], codeLenses: [], semanticTokens: [], diagnostics: [], variables: [], replayFrames: [], coverage: {} };
    },
    maxAnalysisBytes: () => 1024 * 1024,
    smartFeaturesEnabled: () => true,
  });
  const doc = { uri: { fsPath: logPath, toString: () => logPath }, version: 1 };
  await cache.getAnalysis(doc);
  await cache.getAnalysis(doc);
  assert.strictEqual(calls, 1, "same document version should reuse one CLI call");
  await cache.getAnalysis({ ...doc, version: 2 });
  assert.strictEqual(calls, 2, "new document version should re-run analysis");

  const noProject = new ApexLogAnalysisCache({
    findProjectContext: async () => undefined,
    runGladeJSON: async (args, _options, label) => {
      assert.deepStrictEqual(args, ["debug", "editor", "--log", logPath, "--json"]);
      assert.strictEqual(label, "glade debug editor");
      return { version: 1, language: "apexlog", generatedAt: "now", entries: [], symbols: [], folds: [], links: [], hovers: [], codeLenses: [], semanticTokens: [], diagnostics: [], variables: [], replayFrames: [], coverage: {} };
    },
    maxAnalysisBytes: () => 1024 * 1024,
    smartFeaturesEnabled: () => true,
  });
  await noProject.getAnalysis(doc);

  let enabled = true;
  const disabled = new ApexLogAnalysisCache({
    findProjectContext: async () => ({ projectRoot: dir }),
    runGladeJSON: async () => {
      throw new Error("bad log");
    },
    maxAnalysisBytes: () => 1024 * 1024,
    smartFeaturesEnabled: () => enabled,
  });
  await disabled.getAnalysis(doc);
  assert(disabled.diagnosticsFor(doc)[0].message.includes("bad log"));
  enabled = false;
  await disabled.getAnalysis(doc);
  assert.deepStrictEqual(disabled.diagnosticsFor(doc), [], "disabled smart features should clear stale diagnostics");

  let raceEnabled = true;
  let resolveRace;
  const race = new ApexLogAnalysisCache({
    findProjectContext: async () => ({ projectRoot: dir }),
    runGladeJSON: async () => new Promise((resolve) => {
      resolveRace = () => resolve({
        version: 1,
        language: "apexlog",
        generatedAt: "now",
        entries: [],
        symbols: [],
        folds: [],
        links: [],
        hovers: [],
        codeLenses: [],
        semanticTokens: [],
        diagnostics: [{ severity: "warning", code: "late", message: "late result", range: { startLine: 0, startColumn: 0, endLine: 0, endColumn: 1 } }],
        variables: [],
        replayFrames: [],
        coverage: {},
      });
    }),
    maxAnalysisBytes: () => 1024 * 1024,
    smartFeaturesEnabled: () => raceEnabled,
  });
  const pending = race.getAnalysis(doc);
  await Promise.resolve();
  assert(resolveRace, "race command should be waiting");
  raceEnabled = false;
  resolveRace();
  const late = await pending;
  assert.strictEqual(late, undefined);
  assert.deepStrictEqual(race.diagnosticsFor(doc), [], "late disabled analysis should not repopulate diagnostics");

  const bigPath = path.join(dir, "big.log");
  fs.writeFileSync(bigPath, "x".repeat(10));
  const limited = new ApexLogAnalysisCache({
    findProjectContext: async () => ({ projectRoot: dir }),
    runGladeJSON: async () => {
      throw new Error("should not run");
    },
    maxAnalysisBytes: () => 1,
    smartFeaturesEnabled: () => true,
  });
  const skipped = await limited.getAnalysis({ uri: { fsPath: bigPath, toString: () => bigPath }, version: 1 });
  assert.strictEqual(skipped, undefined);
  assert(limited.diagnosticsFor({ uri: { fsPath: bigPath, toString: () => bigPath }, version: 1 })[0].message.includes("too large"));

  Module._load = originalLoad;
})().catch((error) => {
  Module._load = originalLoad;
  throw error;
});

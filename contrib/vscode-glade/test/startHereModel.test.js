const assert = require("assert");
const model = require("../out/startHereModel");

const snapshot = {
  project: {
    workspaceFolder: "/repo",
    projectRoot: "/repo",
    configFound: true,
    namespace: "acme",
    sourceApiVersion: "63.0",
    packageDirs: ["force-app"],
    salesforceExtensions: { apex: true, apexTesting: true, apexLanguageServerTypescript: true },
  },
  activeEnvironment: { name: "dev", dbPath: "/repo/.glade/envs/dev.sqlite" },
  localOrgSummary: { objects: 3, records: 48, users: 1, profiles: 1, permissions: 2 },
  watchRunning: false,
  lastRun: { label: "Changed tests", passed: 8, failed: 1, durationMs: 1420 },
  changedSince: "origin/main",
};

const rows = model.buildStartHereRows(snapshot);
assert.deepStrictEqual(rows.map((row) => row.id), [
  "ready",
  "project",
  "environment",
  "local-proof",
  "last-run",
  "watch",
  "salesforce",
]);
assert.strictEqual(rows[0].label, "Ready for local Apex");
assert.strictEqual(rows[2].label, "Data env: dev");
assert.strictEqual(rows[2].description, "48 records");
assert.strictEqual(rows[3].label, "Run local proof");
assert.strictEqual(rows[3].command, "glade.runLocalProof");
assert.strictEqual(rows[4].description, "8 passed, 1 failed");
assert(rows.length <= 7, "Start Here must stay compact");

const missingRows = model.buildStartHereRows({ project: undefined, changedSince: "origin/main" });
assert.strictEqual(missingRows[0].label, "Open an SFDX project");
assert.strictEqual(missingRows[0].command, "vscode.openFolder");

const noDbRows = model.buildStartHereRows({
  project: snapshot.project,
  changedSince: "origin/main",
  activeEnvironment: { name: "empty", dbPath: "/repo/.glade/envs/empty.sqlite" },
  missingDb: true,
});
assert.strictEqual(noDbRows[2].description, "no DB");
assert.strictEqual(noDbRows[2].tooltip, "/repo/.glade/envs/empty.sqlite");

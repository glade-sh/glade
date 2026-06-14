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
  },
  activeEnvironment: { name: "dev", dbPath: "/repo/.glade/envs/dev.sqlite" },
  localOrgSummary: { objects: 3, records: 48, users: 1, profiles: 1, permissions: 2 },
  watchRunning: false,
  lastRun: { label: "Changed tests", passed: 8, failed: 1, durationMs: 1420 },
  changedSince: "origin/main",
  toolchainReady: true,
  toolchainDetail: "Node and Chromium ready",
  lwcRouteCount: 5,
  vfRouteCount: 2,
  pluginActionCount: 3,
};

const rows = model.buildStartHereRows(snapshot);
assert.deepStrictEqual(rows.map((row) => row.id), [
  "ready",
  "project",
  "toolchain",
  "lwc-preview",
  "vf-preview",
  "plugin-actions",
  "environment",
  "local-proof",
  "last-run",
  "watch",
]);
assert.strictEqual(rows[0].label, "Ready for local Apex");
assert.strictEqual(rows[2].label, "Toolchain ready");
assert.strictEqual(rows[2].description, "Node and Chromium ready");
assert.strictEqual(rows[3].label, "LWC preview");
assert.strictEqual(rows[3].description, "5 routes");
assert.strictEqual(rows[4].label, "Visualforce preview");
assert.strictEqual(rows[4].description, "2 pages");
assert.strictEqual(rows[5].label, "Plugin actions");
assert.strictEqual(rows[5].description, "3 actions");
assert.strictEqual(rows[5].tooltip, "3 plugin actions ready.");
assert.strictEqual(rows[6].label, "Data env: dev");
assert.strictEqual(rows[6].description, "48 records");
assert.strictEqual(rows[7].label, "Run local proof");
assert.strictEqual(rows[7].command, "glade.runLocalProof");
assert.strictEqual(rows[8].description, "8 passed, 1 failed");
assert(
  !rows.some((row) => /salesforce/i.test(`${row.id} ${row.label} ${row.description || ""} ${row.tooltip || ""}`)),
  "Start Here must only surface Glade local workflow state"
);

const missingRows = model.buildStartHereRows({ project: undefined, changedSince: "origin/main" });
assert.strictEqual(missingRows[0].label, "Open an SFDX project");
assert.strictEqual(missingRows[0].command, "vscode.openFolder");

const noDbRows = model.buildStartHereRows({
  project: snapshot.project,
  changedSince: "origin/main",
  activeEnvironment: { name: "empty", dbPath: "/repo/.glade/envs/empty.sqlite" },
  missingDb: true,
});
assert.strictEqual(noDbRows.find((row) => row.id === "environment").description, "no DB");
assert.strictEqual(noDbRows.find((row) => row.id === "environment").tooltip, "/repo/.glade/envs/empty.sqlite");

const stoppedRows = model.buildStartHereRows({
  project: snapshot.project,
  changedSince: "origin/main",
  toolchainReady: false,
  pluginActionCount: 0,
});
assert.strictEqual(stoppedRows.find((row) => row.id === "toolchain").label, "Toolchain install required");
assert.strictEqual(stoppedRows.find((row) => row.id === "lwc-preview").description, "stopped");
assert.strictEqual(stoppedRows.find((row) => row.id === "vf-preview").description, "stopped");
assert.strictEqual(stoppedRows.find((row) => row.id === "plugin-actions").description, "absent");

const unknownRows = model.buildStartHereRows({
  project: snapshot.project,
  changedSince: "origin/main",
});
assert.strictEqual(unknownRows.find((row) => row.id === "toolchain").label, "Toolchain unknown");

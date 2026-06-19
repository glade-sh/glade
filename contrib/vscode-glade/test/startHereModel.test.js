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
  pluginActionCount: 3,
};

const rows = model.buildStartHereRows(snapshot);
assert.deepStrictEqual(rows.map((row) => row.id), [
  "home",
  "ready",
  "project",
  "plugin-actions",
  "environment",
  "local-proof",
  "last-run",
  "watch",
]);
assert.strictEqual(rows[0].label, "Open Glade Home");
assert.strictEqual(rows[0].command, "glade.openHome");
assert.strictEqual(rows[1].label, "Ready for local Apex");
assert.strictEqual(rows[3].label, "Plugin actions");
assert.strictEqual(rows[3].description, "3 actions");
assert.strictEqual(rows[3].tooltip, "3 plugin actions ready.");
assert.strictEqual(rows[4].label, "Data env: dev");
assert.strictEqual(rows[4].description, "48 records");
assert.strictEqual(rows[5].label, "Run local proof");
assert.strictEqual(rows[5].command, "glade.runLocalProof");
assert.strictEqual(rows[6].description, "8 passed, 1 failed");
assert(
  !rows.some((row) => /salesforce/i.test(`${row.id} ${row.label} ${row.description || ""} ${row.tooltip || ""}`)),
  "Start Here must only surface Glade local workflow state"
);
assert(
  !rows.some((row) => /preview|toolchain|visualforce|lwc/i.test(`${row.id} ${row.label} ${row.description || ""} ${row.tooltip || ""}`)),
  "Start Here must not surface parked preview features"
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
  pluginActionCount: 0,
});
assert.strictEqual(stoppedRows.find((row) => row.id === "plugin-actions").description, "absent");

const unknownRows = model.buildStartHereRows({
  project: snapshot.project,
  changedSince: "origin/main",
});
assert(!unknownRows.some((row) => row.id === "toolchain"));

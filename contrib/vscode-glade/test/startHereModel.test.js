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
  "environment",
  "local-proof",
  "last-run",
]);
assert.strictEqual(rows[0].label, "Glade Home");
assert.strictEqual(rows[0].command, "glade.openHome");
assert.strictEqual(rows[1].label, "Data environment");
assert.strictEqual(rows[1].description, "dev - 48 records");
assert.strictEqual(rows[2].label, "Run changed tests");
assert.strictEqual(rows[2].command, "glade.runLocalProof");
assert.strictEqual(rows[3].description, "8 passed, 1 failed");
assert(
  !rows.some((row) => /salesforce/i.test(`${row.id} ${row.label} ${row.description || ""} ${row.tooltip || ""}`)),
  "Start Here must only surface Glade local workflow state"
);
assert(
  !rows.some((row) => /preview|toolchain|visualforce|lwc/i.test(`${row.id} ${row.label} ${row.description || ""} ${row.tooltip || ""}`)),
  "Start Here must not surface parked preview features"
);

const missingRows = model.buildStartHereRows({ project: undefined, changedSince: "origin/main" });
assert.strictEqual(missingRows[0].label, "Open a Salesforce DX project");
assert.strictEqual(missingRows[0].command, "vscode.openFolder");

const noDbRows = model.buildStartHereRows({
  project: snapshot.project,
  changedSince: "origin/main",
  activeEnvironment: { name: "empty", dbPath: "/repo/.glade/envs/empty.sqlite" },
  missingDb: true,
});
assert.strictEqual(noDbRows.find((row) => row.id === "environment").description, "empty - no DB");
assert(noDbRows.find((row) => row.id === "environment").tooltip.includes("/repo/.glade/envs/empty.sqlite"));

const stoppedRows = model.buildStartHereRows({
  project: snapshot.project,
  changedSince: "origin/main",
  pluginActionCount: 0,
});
assert(!stoppedRows.some((row) => row.id === "plugin-actions"));
assert(!stoppedRows.some((row) => row.id === "last-run"));
assert(!stoppedRows.some((row) => row.id === "watch"));

const unknownRows = model.buildStartHereRows({
  project: snapshot.project,
  changedSince: "origin/main",
});
assert(!unknownRows.some((row) => row.id === "toolchain"));
assert.deepStrictEqual(unknownRows.map((row) => row.id), ["home", "environment", "local-proof"]);

const watchingRows = model.buildStartHereRows({
  project: snapshot.project,
  changedSince: "origin/main",
  watchRunning: true,
});
assert.strictEqual(watchingRows.find((row) => row.id === "watch").label, "Watch running");

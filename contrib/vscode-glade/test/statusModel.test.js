const assert = require("assert");
const status = require("../out/statusModel");

assert.strictEqual(status.buildStatusText({ projectReady: false }), "Glade: no Salesforce DX project");
assert.strictEqual(status.buildStatusText({ projectReady: true, activeEnvironment: "dev" }), "Glade: dev");
assert.strictEqual(
  status.buildStatusText({ projectReady: true, activeEnvironment: "dev", lastRun: { failed: 0, durationMs: 18 } }),
  "Glade: dev 18ms",
);
assert.strictEqual(
  status.buildStatusText({ projectReady: true, activeEnvironment: "billing-case", lastRun: { failed: 1, durationMs: 42 } }),
  "Glade: billing-case 1 fail",
);
assert.strictEqual(
  status.buildStatusText({ projectReady: true, activeEnvironment: "dev", changedRecords: 47 }),
  "Glade: dev 47 changed",
);
assert.strictEqual(
  status.buildStatusText({ projectReady: true, activeEnvironment: "dev", missingDb: true }),
  "Glade: dev no DB",
);
assert.strictEqual(
  status.buildStatusText({ projectReady: true, activeEnvironment: "dev", pluginActionCount: 2 }),
  "Glade: plugin 2 reports",
);
assert.strictEqual(
  status.buildStatusTooltip({
    projectReady: true,
    projectRoot: "/repo",
    activeEnvironment: "dev",
    dbPath: "/repo/.glade/envs/dev.sqlite",
    lastCommand: "glade test changed --project . --since origin/main --json --env dev",
    pluginActionCount: 2,
  }),
  "Project: /repo\nEnvironment: dev\nDB: /repo/.glade/envs/dev.sqlite\nPlugin reports: 2 reports\nLast command: glade test changed --project . --since origin/main --json --env dev",
);

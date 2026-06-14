const assert = require("assert");
const status = require("../out/statusModel");

assert.strictEqual(status.buildStatusText({ projectReady: false }), "Glade: no SFDX root");
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
  status.buildStatusText({ projectReady: true, activeEnvironment: "dev", lwcRouteCount: 5 }),
  "Glade: preview 5 routes",
);
assert.strictEqual(
  status.buildStatusText({ projectReady: true, activeEnvironment: "dev", pluginActionCount: 2 }),
  "Glade: plugin 2 findings",
);
assert.strictEqual(
  status.buildStatusText({ projectReady: true, activeEnvironment: "dev", toolchainReady: false }),
  "Glade: toolchain needed",
);
assert.strictEqual(
  status.buildStatusTooltip({
    projectReady: true,
    projectRoot: "/repo",
    activeEnvironment: "dev",
    dbPath: "/repo/.glade/envs/dev.sqlite",
    lastCommand: "glade test changed --project . --since origin/main --json --env dev",
    toolchainReady: false,
    toolchainDetail: "Chromium is not installed",
    lwcRouteCount: 5,
    vfRouteCount: 2,
    pluginActionCount: 2,
  }),
  "Project: /repo\nEnvironment: dev\nDB: /repo/.glade/envs/dev.sqlite\nToolchain: install required\nToolchain detail: Chromium is not installed\nLWC preview: 5 routes\nVisualforce preview: 2 pages\nPlugin actions: 2 findings\nLast command: glade test changed --project . --since origin/main --json --env dev",
);

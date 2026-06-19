const assert = require("assert");
const hub = require("../out/hub/model");

const snapshot = {
  project: {
    workspaceFolder: "/repo",
    projectRoot: "/repo",
    configFound: true,
    namespace: "acme",
    sourceApiVersion: "63.0",
    packageDirs: ["force-app", "unpackaged"],
  },
  activeEnvironment: { name: "dev", dbPath: "/repo/.glade/envs/dev.sqlite" },
  localOrgSummary: { objects: 61, records: 1284, users: 18, profiles: 4, permissions: 11 },
  missingDb: false,
  watchRunning: true,
  lastRun: { label: "Changed tests", passed: 31, failed: 0, durationMs: 18 },
  changedSince: "origin/main",
  pluginActionCount: 3,
  pluginFindingCount: 2,
  salesforceTarget: { label: "core-scratch", state: "stale", detail: "3 days old" },
};

const home = hub.buildHubHome(snapshot);
assert.deepStrictEqual(home.map((group) => group.id), ["run", "data", "debug", "salesforce", "ship"]);
assert.strictEqual(home[0].title, "Run");
assert.strictEqual(home[0].primary.command, "glade.runLocalProof");
assert(home[0].actions.some((action) => action.command === "glade.runChangedTests"));
assert(home[1].actions.some((action) => action.command === "glade.seedLocalOrg"));
assert(home[1].actions.some((action) => action.command === "glade.workbench.newSoql"));
assert.strictEqual(home[2].primary.command, "glade.debugCurrentTest");
assert.strictEqual(home[2].actions.some((action) => action.command === "glade.debugCurrentTest"), false);
assert(home[3].actions.some((action) => action.command === "glade.schemaImportDescribe"));
assert(home[3].actions.some((action) => action.command === "glade.runPluginAction"));
assert(home[4].actions.some((action) => action.command === "glade.openOutput"));
assert.strictEqual(home[4].status.tone, "ok");

const state = hub.buildHubState(snapshot);
assert.deepStrictEqual(state.map((section) => section.id), [
  "project",
  "local-org",
  "data",
  "salesforce",
  "tests",
  "plugins",
]);
assert.strictEqual(state.find((section) => section.id === "project").rows[0].value, "/repo");
assert.strictEqual(state.find((section) => section.id === "data").rows.find((row) => row.label === "Records").value, "1284");
assert.strictEqual(state.find((section) => section.id === "salesforce").tone, "warn");
assert.strictEqual(state.find((section) => section.id === "plugins").rows.find((row) => row.label === "Findings").value, "2");

const missing = hub.buildHubHome({ project: undefined, changedSince: "origin/main" });
assert.strictEqual(missing[0].primary.command, "vscode.openFolder");
assert.strictEqual(missing[0].status.tone, "warn");

const noDb = hub.buildHubState({
  project: snapshot.project,
  activeEnvironment: { name: "empty", dbPath: "/repo/.glade/envs/empty.sqlite" },
  missingDb: true,
  changedSince: "origin/main",
});
assert.strictEqual(noDb.find((section) => section.id === "data").tone, "warn");
assert.strictEqual(noDb.find((section) => section.id === "data").rows.find((row) => row.label === "Records").value, "no DB");

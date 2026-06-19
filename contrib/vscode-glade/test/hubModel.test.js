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
  projectOrg: { alias: "my-glade-org", state: "running", detail: "http://127.0.0.1:17911" },
};

const home = hub.buildHubHome(snapshot);
assert.deepStrictEqual(home.map((group) => group.id), ["org", "run", "data"]);
assert.deepStrictEqual(home.map((group) => group.phase), ["setup", "daily", "data"]);
const orgGroup = home.find((group) => group.id === "org");
const runGroup = home.find((group) => group.id === "run");
const dataGroup = home.find((group) => group.id === "data");
assert.strictEqual(orgGroup.title, "First project setup");
assert.strictEqual(orgGroup.primary.command, "glade.stopProjectOrg");
assert.strictEqual(orgGroup.primary.label, "Stop org");
assert(orgGroup.actions.some((action) => action.command === "glade.projectOrgStatus"));
assert.strictEqual(runGroup.title, "Daily work");
assert.strictEqual(runGroup.primary.command, "glade.runLocalProof");
assert.strictEqual(runGroup.primary.label, "Run changed tests");
assert.strictEqual(runGroup.actions.some((action) => action.command === "glade.runFailedTests"), false);
assert(runGroup.actions.some((action) => action.command === "glade.workbench.newAnonymousApex"));
assert(runGroup.actions.some((action) => action.command === "glade.workbench.newSoql"));
assert.strictEqual(dataGroup.title, "Database inspection");
assert.strictEqual(dataGroup.primary.command, "glade.inspectLocalOrg");
assert(dataGroup.actions.some((action) => action.command === "glade.switchEnvironment"));
assert.strictEqual(dataGroup.actions.some((action) => action.command === "glade.createEnvironment"), false);
for (const group of home) {
  assert(group.actions.length <= 2, `${group.id} should stay quiet on Home`);
}
for (const command of [
  "glade.seedLocalOrg",
  "glade.resetLocalOrg",
  "glade.exportLocalOrg",
  "glade.cloneEnvironment",
  "glade.schemaImportDescribe",
  "glade.runPluginAction",
  "glade.managePlugins",
]) {
  assert.strictEqual(home.flatMap((group) => group.actions).some((action) => action.command === command), false);
}

const apexScratchActions = home.flatMap((group) => group.actions).filter((action) => action.command === "glade.workbench.newAnonymousApex");
assert.strictEqual(apexScratchActions.length, 1);

const failedHome = hub.buildHubHome({
  ...snapshot,
  lastRun: { label: "Changed tests", passed: 28, failed: 3, durationMs: 21 },
});
assert.strictEqual(failedHome.find((group) => group.id === "run").status.tone, "error");
assert.strictEqual(failedHome.find((group) => group.id === "run").actions.some((action) => action.command === "glade.runFailedTests"), false);

const noDbHome = hub.buildHubHome({
  ...snapshot,
  missingDb: true,
  localOrgSummary: undefined,
});
assert(noDbHome.find((group) => group.id === "data").actions.some((action) => action.command === "glade.createEnvironment"));

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
assert.strictEqual(state.find((section) => section.id === "local-org").rows.find((row) => row.label === "Alias").value, "my-glade-org");
assert.strictEqual(state.find((section) => section.id === "local-org").rows.find((row) => row.label === "State").value, "running");
assert.strictEqual(state.find((section) => section.id === "data").rows.find((row) => row.label === "Records").value, "1284");
assert.strictEqual(state.find((section) => section.id === "salesforce").tone, "warn");
assert.strictEqual(state.find((section) => section.id === "plugins").rows.find((row) => row.label === "Reports").value, "2");

const missing = hub.buildHubHome({ project: undefined, changedSince: "origin/main" });
assert.strictEqual(missing[0].primary.command, "vscode.openFolder");
assert.strictEqual(missing[0].phase, "setup");
assert.strictEqual(missing[0].status.tone, "warn");

const noDb = hub.buildHubState({
  project: snapshot.project,
  activeEnvironment: { name: "empty", dbPath: "/repo/.glade/envs/empty.sqlite" },
  missingDb: true,
  changedSince: "origin/main",
});
assert.strictEqual(noDb.find((section) => section.id === "data").tone, "warn");
assert.strictEqual(noDb.find((section) => section.id === "data").rows.find((row) => row.label === "Records").value, "no DB");

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
assert.deepStrictEqual(home.map((group) => group.id), ["run", "org", "data", "debug", "salesforce", "ship"]);
assert.strictEqual(home[0].title, "Run");
assert.strictEqual(home[0].primary.command, "glade.runLocalProof");
assert.strictEqual(home[0].primary.label, "Run changed tests");
assert(home[0].actions.some((action) => action.command === "glade.runChangedTests"));
assert.strictEqual(home[1].title, "Glade org");
assert.strictEqual(home[1].primary.command, "glade.stopProjectOrg");
assert.strictEqual(home[1].primary.label, "Stop org");
assert(home[1].actions.some((action) => action.command === "glade.projectOrgStatus"));
assert(home[2].actions.some((action) => action.command === "glade.seedLocalOrg"));
assert(home[2].actions.some((action) => action.command === "glade.workbench.newSoql"));
assert.strictEqual(home[3].primary.command, "glade.debugCurrentTest");
assert.strictEqual(home[3].actions.some((action) => action.command === "glade.debugCurrentTest"), false);
assert(home[4].actions.some((action) => action.command === "glade.schemaImportDescribe"));
assert(home[4].actions.some((action) => action.command === "glade.runPluginAction"));
assert(home[5].actions.some((action) => action.command === "glade.openOutput"));
assert.strictEqual(home[5].actions.find((action) => action.command === "glade.managePlugins").label, "Manage plugins");
assert.strictEqual(home[5].status.tone, "ok");

const apexScratchActions = home.flatMap((group) => group.actions).filter((action) => action.command === "glade.workbench.newAnonymousApex");
assert.strictEqual(apexScratchActions.length, 1);

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
assert.strictEqual(missing[0].status.tone, "warn");

const noDb = hub.buildHubState({
  project: snapshot.project,
  activeEnvironment: { name: "empty", dbPath: "/repo/.glade/envs/empty.sqlite" },
  missingDb: true,
  changedSince: "origin/main",
});
assert.strictEqual(noDb.find((section) => section.id === "data").tone, "warn");
assert.strictEqual(noDb.find((section) => section.id === "data").rows.find((row) => row.label === "Records").value, "no DB");

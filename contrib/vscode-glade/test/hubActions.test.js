const assert = require("assert");
const actions = require("../out/hub/actions");
const model = require("../out/hub/model");

function collectHomeCommands(snapshot) {
  const commands = new Set();
  for (const group of model.buildHubHome(snapshot)) {
    commands.add(group.primary.command);
    for (const action of group.actions) {
      commands.add(action.command);
    }
  }
  return commands;
}

const project = {
  workspaceFolder: "/repo",
  projectRoot: "/repo",
  configFound: true,
  namespace: "",
  sourceApiVersion: "63.0",
  packageDirs: ["force-app"],
};

const renderedCommands = new Set([
  ...collectHomeCommands({ changedSince: "origin/main" }),
  ...collectHomeCommands({ project, changedSince: "origin/main", watchRunning: false }),
  ...collectHomeCommands({ project, changedSince: "origin/main", watchRunning: true }),
  ...collectHomeCommands({
    project,
    changedSince: "origin/main",
    projectOrg: { alias: "my-glade-org", state: "running" },
  }),
  ...collectHomeCommands({
    project,
    changedSince: "origin/main",
    projectOrg: { alias: "my-glade-org", state: "missing" },
  }),
  ...collectHomeCommands({
    project,
    changedSince: "origin/main",
    missingDb: true,
  }),
  ...collectHomeCommands({
    project,
    changedSince: "origin/main",
    lastRun: { label: "Changed tests", passed: 1, failed: 2 },
  }),
]);

assert.deepStrictEqual(
  [...actions.allowedHubCommands].sort(),
  [...renderedCommands].sort(),
  "hub command allowlist must match the commands rendered by Home",
);

assert.strictEqual(actions.isHubCommand("glade.runLocalProof"), true);
assert.strictEqual(actions.isHubCommand("glade.salesforceTargetStatus"), true);
assert.strictEqual(actions.isHubCommand("glade.schemaImportDescribe"), true);
assert.strictEqual(actions.isHubCommand("glade.seedLocalOrg"), true);
assert.strictEqual(actions.isHubCommand("glade.resetLocalOrg"), true);
assert.strictEqual(actions.isHubCommand("glade.exportLocalOrg"), true);
assert.strictEqual(actions.isHubCommand("glade.runFailedTests"), true);
assert.strictEqual(actions.isHubCommand("glade.runPluginAction"), false);
assert.strictEqual(actions.isHubCommand("glade.managePlugins"), false);
assert.strictEqual(actions.isHubCommand("glade.cloneEnvironment"), false);
assert.strictEqual(actions.isHubCommand("workbench.action.files.delete"), false);
assert.strictEqual(actions.isHubCommand("glade.preview.start"), false);

assert.deepStrictEqual(actions.parseHubMessage({ type: "ready" }), { type: "ready" });
assert.deepStrictEqual(actions.parseHubMessage({ type: "runCommand", command: "glade.runLocalProof" }), {
  type: "runCommand",
  command: "glade.runLocalProof",
});
assert.deepStrictEqual(actions.parseHubMessage({ type: "selectLane", scope: "home", lane: "scratch" }), {
  type: "selectLane",
  scope: "home",
  lane: "scratch",
});
assert.deepStrictEqual(actions.parseHubMessage({ type: "selectLane", scope: "state", lane: "plugins" }), {
  type: "selectLane",
  scope: "state",
  lane: "plugins",
});
assert.throws(
  () => actions.parseHubMessage({ type: "selectLane", scope: "home", lane: "../scratch" }),
  /unsupported hub message/,
);
assert.throws(
  () => actions.parseHubMessage({ type: "runCommand", command: "workbench.action.files.delete" }),
  /command is not allowed/,
);
assert.throws(
  () => actions.parseHubMessage({ type: "unknown" }),
  /unsupported hub message/,
);

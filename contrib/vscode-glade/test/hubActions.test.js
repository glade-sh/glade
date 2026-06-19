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
]);

assert.deepStrictEqual(
  [...actions.allowedHubCommands].sort(),
  [...renderedCommands].sort(),
  "hub command allowlist must match the commands rendered by Home",
);

assert.strictEqual(actions.isHubCommand("glade.runLocalProof"), true);
assert.strictEqual(actions.isHubCommand("glade.salesforceTargetStatus"), true);
assert.strictEqual(actions.isHubCommand("glade.schemaImportDescribe"), false);
assert.strictEqual(actions.isHubCommand("glade.runPluginAction"), false);
assert.strictEqual(actions.isHubCommand("glade.seedLocalOrg"), false);
assert.strictEqual(actions.isHubCommand("glade.resetLocalOrg"), false);
assert.strictEqual(actions.isHubCommand("glade.exportLocalOrg"), false);
assert.strictEqual(actions.isHubCommand("workbench.action.files.delete"), false);
assert.strictEqual(actions.isHubCommand("glade.preview.start"), false);

assert.deepStrictEqual(actions.parseHubMessage({ type: "ready" }), { type: "ready" });
assert.deepStrictEqual(actions.parseHubMessage({ type: "runCommand", command: "glade.runLocalProof" }), {
  type: "runCommand",
  command: "glade.runLocalProof",
});
assert.throws(
  () => actions.parseHubMessage({ type: "runCommand", command: "workbench.action.files.delete" }),
  /command is not allowed/,
);
assert.throws(
  () => actions.parseHubMessage({ type: "unknown" }),
  /unsupported hub message/,
);

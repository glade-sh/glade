const assert = require("assert");
const Module = require("module");

const originalLoad = Module._load;
Module._load = function patchedLoad(request, parent, isMain) {
  if (request === "vscode") {
    return { workspace: { getConfiguration: () => ({ get: () => undefined }) } };
  }
  return originalLoad.call(this, request, parent, isMain);
};

const localOrg = require("../out/localOrg");

assert.deepStrictEqual(
  localOrg.schemaImportDescribeArgs({ projectRoot: "/repo" }, "/repo/reports/org-describe.json"),
  ["schema", "import", "describe", "--input", "/repo/reports/org-describe.json", "--project-cache", "/repo"],
);

assert.deepStrictEqual(
  localOrg.salesforceTargetStatusArgs(),
  ["org", "display", "--json"],
);

assert.deepStrictEqual(
  localOrg.orgCreateArgs({ projectRoot: "/repo" }, "my-glade-org"),
  ["org", "create", "my-glade-org", "--project", "/repo"],
);

assert.deepStrictEqual(
  localOrg.orgStartArgs({ projectRoot: "/repo" }, "my-glade-org"),
  ["org", "start", "my-glade-org", "--project", "/repo"],
);

assert.deepStrictEqual(
  localOrg.orgStatusArgs({ projectRoot: "/repo" }, "my-glade-org"),
  ["org", "status", "my-glade-org", "--project", "/repo", "--json"],
);

assert.deepStrictEqual(
  localOrg.tuiArgs({ projectRoot: "/repo" }, "project"),
  ["tui", "--project", "/repo", "--view", "project"],
);

assert.deepStrictEqual(
  localOrg.tuiArgs({ projectRoot: "/repo" }, "tests"),
  ["tui", "--project", "/repo", "--view", "tests"],
);

assert.deepStrictEqual(
  localOrg.tuiArgs({ projectRoot: "/repo" }, "data", "/repo/.glade/envs/dev.sqlite"),
  ["tui", "--project", "/repo", "--view", "data", "--db", "/repo/.glade/envs/dev.sqlite"],
);

assert.deepStrictEqual(
  localOrg.projectOrgStateFromStatus({
    alias: "my-glade-org",
    status: "running",
    instanceUrl: "http://127.0.0.1:17911",
    db: ".glade/orgs/my-glade-org.sqlite",
  }),
  {
    alias: "my-glade-org",
    state: "running",
    detail: "http://127.0.0.1:17911",
  },
);

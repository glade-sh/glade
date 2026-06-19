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

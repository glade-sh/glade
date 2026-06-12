const assert = require("assert");
const project = require("../out/projectModel");

assert.strictEqual(
  project.nearestSfdxRoot("/repo/force-app/main/default/classes/Foo.cls", [
    "/repo/sfdx-project.json",
  ]),
  "/repo",
);

assert.deepStrictEqual(
  project.parseConfigShowInfo(
    {
      projectRoot: "/repo",
      configFound: true,
      configPath: "/repo/glade.yml",
      namespace: "namz",
      sourceApiVersion: "63.0",
      packageDirs: ["force-app", "unpackaged"],
    },
    "/repo",
    { apex: false, apexTesting: false, apexLanguageServerTypescript: false },
  ),
  {
    workspaceFolder: "/repo",
    projectRoot: "/repo",
    configFound: true,
    configPath: "/repo/glade.yml",
    namespace: "namz",
    sourceApiVersion: "63.0",
    packageDirs: ["force-app", "unpackaged"],
    salesforceExtensions: { apex: false, apexTesting: false, apexLanguageServerTypescript: false },
  },
);

assert.deepStrictEqual(
  project.detectSalesforceExtensions([
    "salesforce.salesforcedx-vscode-apex",
    "salesforce.salesforcedx-vscode-apex-testing",
  ]),
  { apex: true, apexTesting: true, apexLanguageServerTypescript: false },
);

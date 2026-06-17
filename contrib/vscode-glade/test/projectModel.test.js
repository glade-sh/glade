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
      namespace: "otherpkg",
      sourceApiVersion: "63.0",
      packageDirs: ["force-app", "unpackaged"],
    },
    "/repo",
  ),
  {
    workspaceFolder: "/repo",
    projectRoot: "/repo",
    configFound: true,
    configPath: "/repo/glade.yml",
    namespace: "otherpkg",
    sourceApiVersion: "63.0",
    packageDirs: ["force-app", "unpackaged"],
  },
);

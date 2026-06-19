const assert = require("assert");
const fs = require("fs");
const path = require("path");

const prototype = fs.readFileSync(path.resolve(__dirname, "../prototypes/local-org-dashboard.html"), "utf8");

for (const staleCommand of [
  "glade.org.create",
  "glade.schema.importDescribe",
  "glade.plugin.captureFixture",
]) {
  assert(!prototype.includes(staleCommand), `${staleCommand} must not appear in the prototype`);
}

for (const command of [
  "glade.createEnvironment",
  "glade.schemaImportDescribe",
  "glade.salesforceTargetStatus",
  "glade.runPluginAction",
]) {
  assert(prototype.includes(command), `${command} must appear in the prototype`);
}

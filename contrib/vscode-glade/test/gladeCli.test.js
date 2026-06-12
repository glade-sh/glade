const assert = require("assert");
const glade = require("../out/gladeCli");

assert.deepStrictEqual(
  glade.buildGladeArgs("config", ["show", "--project", "/repo", "--json"]),
  ["config", "show", "--project", "/repo", "--json"],
);

assert.deepStrictEqual(glade.parseJSONOutput('{"ok":true}\n', "glade test"), { ok: true });

assert.throws(
  () => glade.parseJSONOutput("not json", "glade test"),
  /glade test produced invalid JSON/,
);

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

assert.deepStrictEqual(
  glade.parseJSONRunResult({ code: 1, stdout: '{"summary":{"failed":1}}\n', stderr: "" }, "glade test", [0, 1]),
  { summary: { failed: 1 } },
);

assert.throws(
  () => glade.parseJSONRunResult({ code: 2, stdout: '{"summary":{"failed":1}}\n', stderr: "bad args" }, "glade test", [0, 1]),
  /glade test failed: bad args/,
);

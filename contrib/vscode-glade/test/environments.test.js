const assert = require("assert");
const envs = require("../out/environments");

assert.deepStrictEqual(envs.normalizeEnvironments([], "/repo"), [
  { name: "dev", dbPath: "/repo/.glade/envs/dev.sqlite" },
]);

assert.deepStrictEqual(
  envs.normalizeEnvironments([{ name: "qa", dbPath: ".glade/envs/qa.sqlite", fixturePath: "data/qa.json" }], "/repo"),
  [{ name: "qa", dbPath: "/repo/.glade/envs/qa.sqlite", fixturePath: "/repo/data/qa.json" }],
);

assert.deepStrictEqual(
  envs.activeEnvironment("qa", [
    { name: "dev", dbPath: "/repo/.glade/envs/dev.sqlite" },
    { name: "qa", dbPath: "/repo/.glade/envs/qa.sqlite" },
  ]),
  { name: "qa", dbPath: "/repo/.glade/envs/qa.sqlite" },
);

assert.strictEqual(envs.environmentNameFromInput("  feature/foo  "), "feature-foo");
assert.throws(() => envs.environmentNameFromInput(""), /environment name is required/);
